package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	"github.com/rajabinekoo/sigryx/pkg/cryptox"
	"github.com/rajabinekoo/sigryx/pkg/hdseed"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func TestRecoveryExportImportReencryptsSeedWithCurrentVaultKey(t *testing.T) {
	sourceSecrets := recoveryUnsealedStore(t, 0x11)
	defer sourceSecrets.Clear()

	seed := bytes.Repeat([]byte{0x77}, hdseed.Size)
	sourceRoot := recoveryRoot(t, sourceSecrets, seed)
	sourceRepo := &memoryRecoveryRepository{roots: []domain.KeyRoot{sourceRoot}}
	sourceService := NewRecoveryService(sourceRepo, sourceSecrets)

	exported, err := sourceService.Export(context.Background(), portin.ExportRecoveryInput{
		Principal: domain.Principal{ID: "root-admin", Kind: domain.PrincipalUser, RootAdmin: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if exported.RecoveryKey == "" || exported.Backup == "" {
		t.Fatal("export did not return recovery material")
	}
	if exported.KeyRoots != 1 {
		t.Fatalf("exported key roots = %d, want 1", exported.KeyRoots)
	}

	if err := sourceSecrets.WithKeyRootSeed(sourceRoot.ID, func([]byte) error { return nil }); !errors.Is(err, secretstore.ErrKeyRootSeedNotFound) {
		t.Fatalf("recovery export should not leave uncached master seeds resident, got %v", err)
	}

	targetSecrets := recoveryUnsealedStore(t, 0x22)
	defer targetSecrets.Clear()
	targetRepo := &memoryRecoveryRepository{}
	targetService := NewRecoveryService(targetRepo, targetSecrets)

	imported, err := targetService.Import(context.Background(), portin.ImportRecoveryInput{
		Principal:   domain.Principal{ID: "root-admin", Kind: domain.PrincipalUser, RootAdmin: true},
		RecoveryKey: exported.RecoveryKey,
		Backup:      exported.Backup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if imported.KeyRoots != 1 {
		t.Fatalf("imported key roots = %d, want 1", imported.KeyRoots)
	}
	if len(targetRepo.roots) != 1 {
		t.Fatalf("restored roots = %d, want 1", len(targetRepo.roots))
	}

	restored := targetRepo.roots[0]
	var opened []byte
	if err := targetSecrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var openErr error
		opened, openErr = cryptox.Open(
			vaultKey,
			cryptox.SealedPayload(restored.SealedSeed),
			keyRootAAD(restored.ID, restored.DerivationScheme),
		)
		return openErr
	}); err != nil {
		t.Fatal(err)
	}
	defer clear(opened)

	if !bytes.Equal(opened, seed) {
		t.Fatal("restored master seed does not match exported master seed")
	}

	// The restored sealed_seed must belong to the current target Vault key,
	// not the old source Vault key.
	if err := sourceSecrets.WithVaultEncryptionKey(func(oldVaultKey []byte) error {
		_, openErr := cryptox.Open(
			oldVaultKey,
			cryptox.SealedPayload(restored.SealedSeed),
			keyRootAAD(restored.ID, restored.DerivationScheme),
		)
		return openErr
	}); err == nil {
		t.Fatal("restored seed unexpectedly opened with the old Vault key")
	}
}

func TestRecoveryRequiresRootAdmin(t *testing.T) {
	secrets := recoveryUnsealedStore(t, 0x11)
	defer secrets.Clear()

	repo := &memoryRecoveryRepository{roots: []domain.KeyRoot{recoveryRoot(t, secrets, bytes.Repeat([]byte{1}, hdseed.Size))}}
	svc := NewRecoveryService(repo, secrets)

	_, err := svc.Export(context.Background(), portin.ExportRecoveryInput{
		Principal: domain.Principal{ID: "user", Kind: domain.PrincipalUser},
	})
	if !errors.Is(err, ErrRecoveryRootAdminRequired) {
		t.Fatalf("expected ErrRecoveryRootAdminRequired, got %v", err)
	}
}

func TestRecoveryExportRequiresUnsealedVault(t *testing.T) {
	svc := NewRecoveryService(&memoryRecoveryRepository{}, secretstore.New())

	_, err := svc.Export(context.Background(), portin.ExportRecoveryInput{
		Principal: domain.Principal{RootAdmin: true},
	})
	if !errors.Is(err, secretstore.ErrVaultSealed) {
		t.Fatalf("expected ErrVaultSealed, got %v", err)
	}
}

func TestRecoveryImportRejectsWrongRecoveryKey(t *testing.T) {
	sourceSecrets := recoveryUnsealedStore(t, 0x11)
	defer sourceSecrets.Clear()

	sourceRepo := &memoryRecoveryRepository{roots: []domain.KeyRoot{
		recoveryRoot(t, sourceSecrets, bytes.Repeat([]byte{1}, hdseed.Size)),
	}}
	sourceService := NewRecoveryService(sourceRepo, sourceSecrets)

	exported, err := sourceService.Export(context.Background(), portin.ExportRecoveryInput{
		Principal: domain.Principal{RootAdmin: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	wrongKey, err := cryptox.RandomKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(wrongKey)

	// Valid format, wrong cryptographic key.
	wrongEncoded := "rec_" + base64.RawURLEncoding.EncodeToString(wrongKey)

	targetSecrets := recoveryUnsealedStore(t, 0x22)
	defer targetSecrets.Clear()
	targetRepo := &memoryRecoveryRepository{}
	targetService := NewRecoveryService(targetRepo, targetSecrets)

	if _, err := targetService.Import(context.Background(), portin.ImportRecoveryInput{
		Principal:   domain.Principal{RootAdmin: true},
		RecoveryKey: wrongEncoded,
		Backup:      exported.Backup,
	}); err == nil {
		t.Fatal("import succeeded with the wrong recovery key")
	}
	if len(targetRepo.roots) != 0 {
		t.Fatal("repository was modified after failed recovery decryption")
	}
}

func recoveryRoot(t *testing.T, secrets *secretstore.Store, seed []byte) domain.KeyRoot {
	t.Helper()

	root := domain.KeyRoot{
		ID:               idgen.New(),
		DerivationScheme: domain.DerivationSchemeBIP32Secp256k1,
	}

	if err := secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var err error
		root.SealedSeed, err = cryptox.Seal(vaultKey, seed, keyRootAAD(root.ID, root.DerivationScheme))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func recoveryUnsealedStore(t *testing.T, value byte) *secretstore.Store {
	t.Helper()

	store := secretstore.New()
	if err := store.ConfigureUnsealKeyCount(1); err != nil {
		t.Fatal(err)
	}

	secret, err := securemem.New(bytes.Repeat([]byte{value}, cryptox.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	progress, err := store.SubmitUnsealKey(1, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Unsealed {
		t.Fatal("secret store did not become unsealed")
	}
	return store
}

type memoryRecoveryRepository struct {
	roots []domain.KeyRoot
	err   error
}

func (r *memoryRecoveryRepository) GetKeyRootsForRecovery(context.Context) ([]*domain.KeyRoot, error) {
	if r.err != nil {
		return nil, r.err
	}
	result := make([]*domain.KeyRoot, len(r.roots))
	for i := range r.roots {
		root := r.roots[i]
		root.SealedSeed = bytes.Clone(root.SealedSeed)
		result[i] = &root
	}
	return result, nil
}

func (r *memoryRecoveryRepository) RestoreKeyRoots(_ context.Context, roots []domain.KeyRoot) error {
	if r.err != nil {
		return r.err
	}
	r.roots = make([]domain.KeyRoot, len(roots))
	for i := range roots {
		r.roots[i] = roots[i]
		r.roots[i].SealedSeed = bytes.Clone(roots[i].SealedSeed)
	}
	return nil
}

func TestRecoveryImportRequiresRootAdmin(t *testing.T) {
	secrets := recoveryUnsealedStore(t, 0x11)
	defer secrets.Clear()

	svc := NewRecoveryService(&memoryRecoveryRepository{}, secrets)
	_, err := svc.Import(context.Background(), portin.ImportRecoveryInput{
		Principal: domain.Principal{ID: "user", Kind: domain.PrincipalUser},
	})
	if !errors.Is(err, ErrRecoveryRootAdminRequired) {
		t.Fatalf("expected ErrRecoveryRootAdminRequired, got %v", err)
	}
}

func TestRecoveryExportRejectsEmptyKeyRootSet(t *testing.T) {
	secrets := recoveryUnsealedStore(t, 0x11)
	defer secrets.Clear()

	svc := NewRecoveryService(&memoryRecoveryRepository{}, secrets)
	_, err := svc.Export(context.Background(), portin.ExportRecoveryInput{
		Principal: domain.Principal{RootAdmin: true},
	})
	if !errors.Is(err, ErrRecoveryNoKeyRoots) {
		t.Fatalf("expected ErrRecoveryNoKeyRoots, got %v", err)
	}
}
