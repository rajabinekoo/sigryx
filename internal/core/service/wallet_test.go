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
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func TestWalletServiceReturnsExistingWalletForSameUser(t *testing.T) {
	wallets := &memoryWalletRepository{
		wallet: &domain.Wallet{
			ID:             "wallet-1",
			KeyRootID:      "root-1",
			UserID:         "user-1",
			Adapter:        "evm",
			DerivationPath: "m/44'/60'/0'/0/7",
			PublicKey:      []byte{1, 2, 3},
			Address:        "0xabc",
		},
	}

	svc := NewWalletService(
		wallets,
		&walletKeyRootRepository{},
		secretstore.New(),
		&fakeWalletAdapter{},
	)

	result, err := svc.Create(context.Background(), portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.ID != "wallet-1" {
		t.Fatalf("wallet ID = %q, want wallet-1", result.ID)
	}
	if wallets.nextIndexCalls != 0 {
		t.Fatal("existing wallet must not allocate a new derivation index")
	}
}

func TestWalletServiceCreatesWalletOncePerUserAndRoot(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	seedBytes := bytes.Repeat([]byte{0x33}, 32)
	seed, err := securemem.New(append([]byte(nil), seedBytes...))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	wallets := &memoryWalletRepository{nextIndex: 12}
	roots := &walletKeyRootRepository{root: &domain.KeyRoot{
		ID:               "root-1",
		DerivationScheme: domain.DerivationSchemeBIP32Secp256k1,
	}}
	adapter := &fakeWalletAdapter{}
	svc := NewWalletService(wallets, roots, secrets, adapter)

	input := portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	}

	first, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf("same user returned different wallets: %q != %q", first.ID, second.ID)
	}
	if wallets.nextIndexCalls != 1 {
		t.Fatalf("next index called %d times, want 1", wallets.nextIndexCalls)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter called %d times, want 1", adapter.calls)
	}
	if adapter.index != 12 {
		t.Fatalf("adapter index = %d, want 12", adapter.index)
	}
	if !bytes.Equal(adapter.seed, seedBytes) {
		t.Fatal("adapter received wrong key root seed")
	}
	if wallets.wallet.UserID != "user-1" {
		t.Fatalf("persisted user ID = %q", wallets.wallet.UserID)
	}
}

func TestWalletServiceLoadsSeedFromSealedKeyRoot(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	root := &domain.KeyRoot{
		ID:               "root-1",
		DerivationScheme: domain.DerivationSchemeBIP32Secp256k1,
	}
	seedBytes := bytes.Repeat([]byte{0x5a}, 32)

	if err := secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		sealed, err := cryptox.Seal(
			vaultKey,
			seedBytes,
			keyRootAAD(root.ID, root.DerivationScheme),
		)
		root.SealedSeed = sealed
		return err
	}); err != nil {
		t.Fatal(err)
	}

	wallets := &memoryWalletRepository{}
	adapter := &fakeWalletAdapter{}
	svc := NewWalletService(
		wallets,
		&walletKeyRootRepository{root: root},
		secrets,
		adapter,
	)

	_, err := svc.Create(context.Background(), portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(adapter.seed, seedBytes) {
		t.Fatal("adapter did not receive decrypted persisted seed")
	}
}

func TestWalletServiceRequiresUnsealedVaultForNewWallet(t *testing.T) {
	wallets := &memoryWalletRepository{}
	svc := NewWalletService(
		wallets,
		&walletKeyRootRepository{},
		secretstore.New(),
		&fakeWalletAdapter{},
	)

	_, err := svc.Create(context.Background(), portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	})
	if !errors.Is(err, secretstore.ErrVaultSealed) {
		t.Fatalf("expected ErrVaultSealed, got %v", err)
	}
}

func TestWalletServiceRejectsMismatchedKeyRootScheme(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	svc := NewWalletService(
		&memoryWalletRepository{},
		&walletKeyRootRepository{root: &domain.KeyRoot{
			ID:               "root-1",
			DerivationScheme: domain.DerivationScheme("OTHER"),
		}},
		secrets,
		&fakeWalletAdapter{},
	)

	_, err := svc.Create(context.Background(), portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	})
	if !errors.Is(err, ErrWalletSchemeMismatch) {
		t.Fatalf("expected ErrWalletSchemeMismatch, got %v", err)
	}
}

type memoryWalletRepository struct {
	wallet         *domain.Wallet
	nextIndex      uint32
	nextIndexCalls int
	createErr      error
	conflictWallet *domain.Wallet
}

func (r *memoryWalletRepository) GetByUser(
	_ context.Context,
	keyRootID string,
	adapter string,
	userID string,
) (*domain.Wallet, error) {
	if r.wallet == nil ||
		r.wallet.KeyRootID != keyRootID ||
		r.wallet.Adapter != adapter ||
		r.wallet.UserID != userID {
		return nil, portout.ErrWalletNotFound
	}
	copyWallet := *r.wallet
	copyWallet.PublicKey = bytes.Clone(r.wallet.PublicKey)
	return &copyWallet, nil
}

func (r *memoryWalletRepository) NextIndex(
	_ context.Context,
	_ string,
	_ string,
) (uint32, error) {
	r.nextIndexCalls++
	return r.nextIndex, nil
}

func (r *memoryWalletRepository) Create(
	_ context.Context,
	wallet domain.Wallet,
) error {
	if r.createErr != nil {
		if r.conflictWallet != nil {
			copyWallet := *r.conflictWallet
			copyWallet.PublicKey = bytes.Clone(r.conflictWallet.PublicKey)
			r.wallet = &copyWallet
		}
		return r.createErr
	}
	copyWallet := wallet
	copyWallet.PublicKey = bytes.Clone(wallet.PublicKey)
	r.wallet = &copyWallet
	return nil
}

type walletKeyRootRepository struct {
	root *domain.KeyRoot
}

func (r *walletKeyRootRepository) Create(context.Context, domain.KeyRoot) error {
	return nil
}

func (r *walletKeyRootRepository) GetByID(_ context.Context, id string) (*domain.KeyRoot, error) {
	if r.root == nil || r.root.ID != id {
		return nil, portout.ErrKeyRootNotFound
	}
	copyRoot := *r.root
	copyRoot.SealedSeed = bytes.Clone(r.root.SealedSeed)
	return &copyRoot, nil
}

func (r *walletKeyRootRepository) GetAll(context.Context) ([]*domain.KeyRoot, error) {
	return nil, nil
}

type fakeWalletAdapter struct {
	calls int
	seed  []byte
	index uint32
}

func (*fakeWalletAdapter) Name() string {
	return "evm"
}

func (*fakeWalletAdapter) WalletType() domain.WalletType {
	return domain.WalletTypeEthereum
}

func (*fakeWalletAdapter) DerivationScheme() domain.DerivationScheme {
	return domain.DerivationSchemeBIP32Secp256k1
}

func (a *fakeWalletAdapter) Derive(seed []byte, index uint32) (portout.DerivedWallet, error) {
	a.calls++
	a.seed = bytes.Clone(seed)
	a.index = index
	return portout.DerivedWallet{
		DerivationPath: "m/44'/60'/0'/0/12",
		PublicKey:      []byte{4, 1, 2, 3},
		Address:        "0xabc",
	}, nil
}

func TestWalletServiceReturnsConcurrentWalletOnUniqueConflict(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	seed, err := securemem.New(bytes.Repeat([]byte{0x44}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	concurrent := &domain.Wallet{
		ID:             "wallet-existing",
		KeyRootID:      "root-1",
		UserID:         "user-1",
		Adapter:        "evm",
		DerivationPath: "m/44'/60'/0'/0/9",
		PublicKey:      []byte{4, 9},
		Address:        "0xexisting",
	}
	wallets := &memoryWalletRepository{
		createErr:      portout.ErrWalletAlreadyExists,
		conflictWallet: concurrent,
	}

	svc := NewWalletService(
		wallets,
		&walletKeyRootRepository{root: &domain.KeyRoot{
			ID:               "root-1",
			DerivationScheme: domain.DerivationSchemeBIP32Secp256k1,
		}},
		secrets,
		&fakeWalletAdapter{},
	)

	result, err := svc.Create(context.Background(), portin.CreateWalletInput{
		KeyRootID:  "root-1",
		UserID:     "user-1",
		WalletType: domain.WalletTypeEthereum,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "wallet-existing" {
		t.Fatalf("wallet ID = %q, want wallet-existing", result.ID)
	}
}
