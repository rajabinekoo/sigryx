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
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

var ErrUnsupportedWalletType = errors.New("key root: unsupported wallet type")

type KeyRootService struct {
	repository portout.KeyRootRepository
	secrets    *secretstore.Store
}

func NewKeyRootService(
	repository portout.KeyRootRepository,
	secrets *secretstore.Store,
) *KeyRootService {
	return &KeyRootService{
		repository: repository,
		secrets:    secrets,
	}
}

func (s *KeyRootService) GetAll(ctx context.Context) ([]*portin.CreateKeyRootResult, error) {
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}

	list, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*portin.CreateKeyRootResult, len(list))
	for i, item := range list {
		walletType, ok := item.DerivationScheme.WalletType()
		if !ok {
			return nil, ErrUnsupportedWalletType
		}
		result[i] = &portin.CreateKeyRootResult{
			ID:               item.ID,
			WalletType:       walletType,
			DerivationScheme: item.DerivationScheme,
		}
	}

	return result, nil
}

func (s *KeyRootService) Create(
	ctx context.Context,
	input portin.CreateKeyRootInput,
) (*portin.CreateKeyRootResult, error) {
	derivationScheme, ok := input.WalletType.DerivationScheme()
	if !ok {
		return nil, ErrUnsupportedWalletType
	}

	// Fail before generating secret material when the Vault is sealed.
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}

	rootID := idgen.New()

	seed, err := hdseed.Generate()
	if err != nil {
		return nil, err
	}
	seedOwnedByStore := false
	defer func() {
		if !seedOwnedByStore {
			seed.Destroy()
		}
	}()

	var sealedSeed cryptox.SealedPayload

	err = s.secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		return seed.WithBytes(func(masterSeed []byte) error {
			var sealErr error
			sealedSeed, sealErr = cryptox.Seal(
				vaultKey,
				masterSeed,
				keyRootAAD(rootID, derivationScheme),
			)
			return sealErr
		})
	})
	if err != nil {
		return nil, fmt.Errorf("seal HD master seed: %w", err)
	}

	// Keep the plaintext seed only in libsodium-backed memory while the Vault
	// remains unsealed. Ownership transfers to SecretStore here.
	if err := s.secrets.StoreKeyRootSeed(rootID, seed); err != nil {
		return nil, fmt.Errorf("store HD master seed in secure memory: %w", err)
	}
	seedOwnedByStore = true

	root := domain.KeyRoot{
		ID:               rootID,
		DerivationScheme: derivationScheme,
		SealedSeed:       sealedSeed,
	}

	if err := s.repository.Create(ctx, root); err != nil {
		// Persistence is the source of truth. Roll back the runtime copy if the
		// durable record could not be created.
		s.secrets.RemoveKeyRootSeed(rootID)
		return nil, fmt.Errorf("persist key root: %w", err)
	}

	return &portin.CreateKeyRootResult{
		ID:               rootID,
		WalletType:       input.WalletType,
		DerivationScheme: derivationScheme,
	}, nil
}

func keyRootAAD(
	rootID string,
	derivationScheme domain.DerivationScheme,
) []byte {
	return fmt.Appendf(
		nil,
		"sigryx:key-root:v1:%s:%s",
		rootID,
		derivationScheme,
	)
}

var _ portin.KeyRootUseCase = (*KeyRootService)(nil)
