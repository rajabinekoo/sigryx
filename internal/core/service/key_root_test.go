package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/cryptox"
	"github.com/rajabinekoo/sigryx/pkg/hdseed"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func TestKeyRootServiceCreatesEthereumRoot(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	repo := &memoryKeyRootRepository{}
	svc := NewKeyRootService(repo, secrets)

	result, err := svc.Create(context.Background(), portin.CreateKeyRootInput{
		WalletType: domain.WalletTypeEthereum,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := idgen.Parse(result.ID); err != nil {
		t.Fatalf("result ID is not a UUID: %v", err)
	}
	if result.WalletType != domain.WalletTypeEthereum {
		t.Fatalf("wallet type = %q, want %q", result.WalletType, domain.WalletTypeEthereum)
	}
	if result.DerivationScheme != domain.DerivationSchemeBIP32Secp256k1 {
		t.Fatalf(
			"derivation scheme = %q, want %q",
			result.DerivationScheme,
			domain.DerivationSchemeBIP32Secp256k1,
		)
	}

	root := repo.root
	if root == nil {
		t.Fatal("key root was not persisted")
	}
	if root.ID != result.ID {
		t.Fatalf("persisted root ID = %q, want %q", root.ID, result.ID)
	}
	if root.DerivationScheme != domain.DerivationSchemeBIP32Secp256k1 {
		t.Fatalf("unexpected persisted derivation scheme: %q", root.DerivationScheme)
	}
	if len(root.SealedSeed) == 0 {
		t.Fatal("sealed seed is empty")
	}

	var openedSeed []byte
	if err := secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var openErr error
		openedSeed, openErr = cryptox.Open(
			vaultKey,
			cryptox.SealedPayload(root.SealedSeed),
			keyRootAAD(root.ID, root.DerivationScheme),
		)
		return openErr
	}); err != nil {
		t.Fatal(err)
	}
	defer clear(openedSeed)

	if len(openedSeed) != hdseed.Size {
		t.Fatalf("opened seed size = %d, want %d", len(openedSeed), hdseed.Size)
	}

	if err := secrets.WithKeyRootSeed(root.ID, func(cachedSeed []byte) error {
		if !bytes.Equal(cachedSeed, openedSeed) {
			t.Fatal("cached key root seed does not match persisted sealed seed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRootServiceRejectsUnsupportedWalletType(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	repo := &memoryKeyRootRepository{}
	svc := NewKeyRootService(repo, secrets)

	_, err := svc.Create(context.Background(), portin.CreateKeyRootInput{
		WalletType: domain.WalletType("UNKNOWN"),
	})
	if !errors.Is(err, ErrUnsupportedWalletType) {
		t.Fatalf("expected ErrUnsupportedWalletType, got %v", err)
	}
	if repo.root != nil {
		t.Fatal("repository must not be called for an unsupported wallet type")
	}
}

func TestKeyRootServiceRequiresUnsealedVault(t *testing.T) {
	secrets := secretstore.New()
	defer secrets.Clear()

	repo := &memoryKeyRootRepository{}
	svc := NewKeyRootService(repo, secrets)

	_, err := svc.Create(context.Background(), portin.CreateKeyRootInput{
		WalletType: domain.WalletTypeEthereum,
	})
	if !errors.Is(err, secretstore.ErrVaultSealed) {
		t.Fatalf("expected ErrVaultSealed, got %v", err)
	}
	if repo.root != nil {
		t.Fatal("repository must not be called while the Vault is sealed")
	}
}

func TestKeyRootServiceRemovesRuntimeSeedWhenPersistenceFails(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	expected := errors.New("database unavailable")
	repo := &memoryKeyRootRepository{err: expected}
	svc := NewKeyRootService(repo, secrets)

	_, err := svc.Create(context.Background(), portin.CreateKeyRootInput{
		WalletType: domain.WalletTypeEthereum,
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected repository error, got %v", err)
	}
	if repo.root == nil {
		t.Fatal("repository did not receive the key root")
	}

	err = secrets.WithKeyRootSeed(repo.root.ID, func([]byte) error {
		return nil
	})
	if !errors.Is(err, secretstore.ErrKeyRootSeedNotFound) {
		t.Fatalf("expected runtime seed rollback, got %v", err)
	}
}

func TestKeyRootServiceListsRootsWhileVaultIsSealed(t *testing.T) {
	repo := &memoryKeyRootRepository{list: []*domain.KeyRoot{
		{ID: "root-1", DerivationScheme: domain.DerivationSchemeBIP32Secp256k1},
	}}
	svc := NewKeyRootService(repo, secretstore.New())

	result, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("root count = %d, want 1", len(result))
	}
	if result[0].ID != "root-1" {
		t.Fatalf("root ID = %q, want root-1", result[0].ID)
	}
	if result[0].DerivationScheme != domain.DerivationSchemeBIP32Secp256k1 {
		t.Fatalf("unexpected derivation scheme: %q", result[0].DerivationScheme)
	}
}

func newUnsealedSecretStore(t *testing.T) *secretstore.Store {
	t.Helper()

	store := secretstore.New()
	if err := store.ConfigureUnsealKeyCount(1); err != nil {
		t.Fatal(err)
	}

	raw := bytes.Repeat([]byte{0x42}, cryptox.KeySize)
	key, err := securemem.New(raw)
	if err != nil {
		t.Fatal(err)
	}

	progress, err := store.SubmitUnsealKey(1, key)
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Unsealed {
		t.Fatal("secret store did not become unsealed")
	}

	return store
}

type memoryKeyRootRepository struct {
	root *domain.KeyRoot
	list []*domain.KeyRoot
	err  error
}

func (r *memoryKeyRootRepository) GetByID(_ context.Context, id string) (*domain.KeyRoot, error) {
	if r.root == nil || r.root.ID != id {
		return nil, portout.ErrKeyRootNotFound
	}
	copyRoot := *r.root
	copyRoot.SealedSeed = bytes.Clone(r.root.SealedSeed)
	return &copyRoot, nil
}

func (r *memoryKeyRootRepository) GetAll(context.Context) ([]*domain.KeyRoot, error) {
	return r.list, r.err
}

func (r *memoryKeyRootRepository) Create(
	_ context.Context,
	root domain.KeyRoot,
) error {
	copyRoot := root
	copyRoot.SealedSeed = bytes.Clone(root.SealedSeed)
	r.root = &copyRoot
	return r.err
}
