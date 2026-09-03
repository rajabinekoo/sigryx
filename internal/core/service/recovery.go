package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/cryptox"
	"github.com/rajabinekoo/sigryx/pkg/hdseed"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
	"github.com/rajabinekoo/sigryx/pkg/recovery"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

var (
	ErrRecoveryRootAdminRequired = errors.New("recovery: root admin required")
	ErrRecoveryNoKeyRoots        = errors.New("recovery: no key roots to export")
	ErrRecoveryInvalidKeyRoot    = errors.New("recovery: invalid key root")
)

type RecoveryService struct {
	repository portout.RecoveryRepository
	secrets    *secretstore.Store
}

func NewRecoveryService(
	repository portout.RecoveryRepository,
	secrets *secretstore.Store,
) *RecoveryService {
	return &RecoveryService{
		repository: repository,
		secrets:    secrets,
	}
}

func (s *RecoveryService) Export(
	ctx context.Context,
	input portin.ExportRecoveryInput,
) (*portin.ExportRecoveryResult, error) {
	if !input.Principal.RootAdmin {
		return nil, ErrRecoveryRootAdminRequired
	}
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}

	roots, err := s.repository.GetKeyRootsForRecovery(ctx)
	if err != nil {
		return nil, fmt.Errorf("list key roots for recovery: %w", err)
	}
	if len(roots) == 0 {
		return nil, ErrRecoveryNoKeyRoots
	}

	key, err := recovery.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate recovery key: %w", err)
	}
	defer clear(key)

	manifest := recovery.Manifest{
		Version:  recovery.Version,
		KeyRoots: make([]recovery.Entry, 0, len(roots)),
	}

	for _, root := range roots {
		if root == nil || len(root.SealedSeed) == 0 {
			return nil, ErrRecoveryInvalidKeyRoot
		}
		if _, err := idgen.Parse(root.ID); err != nil || !supportedRecoveryDerivationScheme(root.DerivationScheme) {
			return nil, ErrRecoveryInvalidKeyRoot
		}

		var encryptedSeed string
		err := withRecoveryKeyRootSeed(s.secrets, root, func(seed []byte) error {
			if len(seed) != hdseed.Size {
				return ErrRecoveryInvalidKeyRoot
			}
			var encryptErr error
			encryptedSeed, encryptErr = recovery.EncryptSeed(
				key,
				root.ID,
				string(root.DerivationScheme),
				seed,
			)
			return encryptErr
		})
		if err != nil {
			return nil, fmt.Errorf("encrypt key root %s for recovery: %w", root.ID, err)
		}

		manifest.KeyRoots = append(manifest.KeyRoots, recovery.Entry{
			ID:               root.ID,
			DerivationScheme: string(root.DerivationScheme),
			EncryptedSeed:    encryptedSeed,
		})
	}

	backup, err := recovery.EncodeBackup(key, manifest)
	if err != nil {
		return nil, fmt.Errorf("encode recovery backup: %w", err)
	}

	encodedKey, err := recovery.EncodeKey(key)
	if err != nil {
		return nil, fmt.Errorf("encode recovery key: %w", err)
	}

	return &portin.ExportRecoveryResult{
		RecoveryKey: encodedKey,
		Backup:      backup,
		KeyRoots:    len(manifest.KeyRoots),
	}, nil
}

func (s *RecoveryService) Import(
	ctx context.Context,
	input portin.ImportRecoveryInput,
) (*portin.ImportRecoveryResult, error) {
	if !input.Principal.RootAdmin {
		return nil, ErrRecoveryRootAdminRequired
	}
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}

	key, err := recovery.DecodeKey(input.RecoveryKey)
	if err != nil {
		return nil, err
	}
	defer clear(key)

	manifest, err := recovery.DecodeBackup(key, input.Backup)
	if err != nil {
		return nil, err
	}

	roots := make([]domain.KeyRoot, 0, len(manifest.KeyRoots))

	err = s.secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		for _, entry := range manifest.KeyRoots {
			if _, parseErr := idgen.Parse(entry.ID); parseErr != nil {
				return ErrRecoveryInvalidKeyRoot
			}

			scheme := domain.DerivationScheme(entry.DerivationScheme)
			if !supportedRecoveryDerivationScheme(scheme) {
				return ErrRecoveryInvalidKeyRoot
			}

			seed, openErr := recovery.DecryptSeed(key, entry)
			if openErr != nil {
				return openErr
			}

			if len(seed) != hdseed.Size {
				clear(seed)
				return ErrRecoveryInvalidKeyRoot
			}

			sealedSeed, sealErr := cryptox.Seal(
				vaultKey,
				seed,
				keyRootAAD(entry.ID, scheme),
			)
			clear(seed)
			if sealErr != nil {
				return sealErr
			}

			roots = append(roots, domain.KeyRoot{
				ID:               entry.ID,
				DerivationScheme: scheme,
				SealedSeed:       sealedSeed,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("prepare recovered key roots: %w", err)
	}

	if err := s.repository.RestoreKeyRoots(ctx, roots); err != nil {
		return nil, fmt.Errorf("restore key roots: %w", err)
	}

	// Drop any cached plaintext seeds so subsequent wallet operations reload
	// the just-restored encrypted seeds from durable storage.
	for _, root := range roots {
		s.secrets.RemoveKeyRootSeed(root.ID)
	}

	return &portin.ImportRecoveryResult{KeyRoots: len(roots)}, nil
}

func withRecoveryKeyRootSeed(
	secrets *secretstore.Store,
	root *domain.KeyRoot,
	fn func([]byte) error,
) error {
	err := secrets.WithKeyRootSeed(root.ID, fn)
	if err == nil {
		return nil
	}
	if !errors.Is(err, secretstore.ErrKeyRootSeedNotFound) {
		return err
	}

	var plaintext []byte
	err = secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var openErr error
		plaintext, openErr = cryptox.Open(
			vaultKey,
			cryptox.SealedPayload(root.SealedSeed),
			keyRootAAD(root.ID, root.DerivationScheme),
		)
		return openErr
	})
	if err != nil {
		return err
	}

	seed, err := securemem.New(plaintext)
	if err != nil {
		return err
	}
	defer seed.Destroy()

	return seed.WithBytes(fn)
}

func supportedRecoveryDerivationScheme(scheme domain.DerivationScheme) bool {
	switch scheme {
	case domain.DerivationSchemeBIP32Secp256k1:
		return true
	default:
		return false
	}
}

var _ portin.RecoveryUseCase = (*RecoveryService)(nil)
