package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func TestSigningServiceGenericJSONIsCanonical(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	seed, err := securemem.New(bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	wallet := signingWallet()
	adapter := &fakeSigningAdapter{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet},
		&walletKeyRootRepository{root: signingRoot()},
		secrets,
		adapter,
	)

	first, err := svc.SignData(context.Background(), portin.SignDataInput{
		WalletID: wallet.ID,
		Context:  "ledger:journal-entry:v1",
		Format:   domain.DataFormatJSON,
		Payload:  []byte(`{"b":1.0,"a":{"y":1e0,"x":true}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.SignData(context.Background(), portin.SignDataInput{
		WalletID: wallet.ID,
		Context:  "ledger:journal-entry:v1",
		Format:   domain.DataFormatJSON,
		Payload:  []byte(`{"a":{"x":true,"y":1},"b":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first.Digest, second.Digest) {
		t.Fatal("equivalent JSON produced different digests")
	}
	if !bytes.Equal(first.Signature, second.Signature) {
		t.Fatal("equivalent JSON produced different signatures")
	}
}

func TestSigningServiceVerifyDoesNotRequireUnsealedVault(t *testing.T) {
	wallet := signingWallet()
	adapter := &fakeSigningAdapter{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet},
		&walletKeyRootRepository{root: signingRoot()},
		secretstore.New(),
		adapter,
	)

	result, err := svc.VerifyData(context.Background(), portin.VerifyDataInput{
		WalletID:  wallet.ID,
		Context:   "ledger:journal-entry:v1",
		Format:    domain.DataFormatRaw,
		Payload:   []byte("entry"),
		Signature: bytes.Repeat([]byte{0xaa}, 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatal("verification should use the persisted public key while sealed")
	}
}

func TestSigningServiceSignRequiresUnsealedVault(t *testing.T) {
	wallet := signingWallet()
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet},
		&walletKeyRootRepository{root: signingRoot()},
		secretstore.New(),
		&fakeSigningAdapter{},
	)

	_, err := svc.SignData(context.Background(), portin.SignDataInput{
		WalletID: wallet.ID,
		Context:  "ledger:journal-entry:v1",
		Format:   domain.DataFormatRaw,
		Payload:  []byte("entry"),
	})
	if !errors.Is(err, secretstore.ErrVaultSealed) {
		t.Fatalf("expected ErrVaultSealed, got %v", err)
	}
}

func TestSigningServiceSignsTransactionWithWalletPrivateKey(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()

	seedBytes := bytes.Repeat([]byte{0x55}, 32)
	seed, err := securemem.New(append([]byte(nil), seedBytes...))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	wallet := signingWallet()
	adapter := &fakeSigningAdapter{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet},
		&walletKeyRootRepository{root: signingRoot()},
		secrets,
		adapter,
	)

	result, err := svc.SignTransaction(context.Background(), portin.SignTransactionInput{
		WalletID:    wallet.ID,
		Transaction: domain.EthereumTransaction{Type: domain.TransactionTypeEIP1559, ChainID: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.RawTransaction) != "signed" {
		t.Fatalf("raw transaction = %q", result.RawTransaction)
	}
	if adapter.path != wallet.DerivationPath {
		t.Fatalf("derivation path = %q", adapter.path)
	}
	if !bytes.Equal(adapter.seed, seedBytes) {
		t.Fatal("adapter received wrong key root seed")
	}
}

func signingWallet() *domain.Wallet {
	return &domain.Wallet{
		ID: "wallet-1", KeyRootID: "root-1", UserID: "user-1", Adapter: "evm",
		DerivationPath: "m/44'/60'/0'/0/3", PublicKey: []byte{0x04, 1, 2, 3}, Address: "0xabc",
	}
}

func signingRoot() *domain.KeyRoot {
	return &domain.KeyRoot{ID: "root-1", DerivationScheme: domain.DerivationSchemeBIP32Secp256k1}
}

type fakeSigningAdapter struct {
	seed []byte
	path string
}

func (*fakeSigningAdapter) Name() string { return "evm" }
func (*fakeSigningAdapter) DerivationScheme() domain.DerivationScheme {
	return domain.DerivationSchemeBIP32Secp256k1
}
func (a *fakeSigningAdapter) WithPrivateKey(seed []byte, path string, fn func([]byte) error) error {
	a.seed = bytes.Clone(seed)
	a.path = path
	return fn([]byte("private-key"))
}
func (*fakeSigningAdapter) SignTransaction([]byte, domain.EthereumTransaction) (portout.TransactionSignature, error) {
	return portout.TransactionSignature{RawTransaction: []byte("signed"), Hash: []byte("hash"), R: make([]byte, 32), S: make([]byte, 32)}, nil
}
func (*fakeSigningAdapter) VerifyTransaction([]byte, []byte) (bool, error) { return true, nil }
func (*fakeSigningAdapter) SignTypedData([]byte, []byte) ([]byte, []byte, error) {
	return []byte("typed-signature"), []byte("typed-digest"), nil
}
func (*fakeSigningAdapter) VerifyTypedData([]byte, []byte, []byte) (bool, []byte, error) {
	return true, []byte("typed-digest"), nil
}
func (*fakeSigningAdapter) SignDigest(_ []byte, digest []byte) ([]byte, error) {
	return bytes.Clone(digest), nil
}
func (*fakeSigningAdapter) VerifyDigest(_ []byte, digest, signature []byte) bool {
	if len(signature) == 64 && signature[0] == 0xaa {
		return true
	}
	return bytes.Equal(digest, signature)
}
