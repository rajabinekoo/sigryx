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
		IntegrityDependencies{},
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
		IntegrityDependencies{},
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
		IntegrityDependencies{},
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
		IntegrityDependencies{},
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
	if len(digest) != 32 {
		return nil, errors.New("unexpected digest size")
	}
	signature := make([]byte, 64)
	copy(signature[:32], digest)
	copy(signature[32:], digest)
	return signature, nil
}
func (*fakeSigningAdapter) VerifyDigest(_ []byte, digest, signature []byte) bool {
	if len(signature) == 64 && signature[0] == 0xaa {
		return true
	}
	if len(digest) != 32 || len(signature) != 64 {
		return false
	}
	return bytes.Equal(signature[:32], digest) && bytes.Equal(signature[32:], digest)
}

func TestSigningServiceIntegrityFreezesNestedFieldSchemaAndValues(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()
	seed, err := securemem.New(bytes.Repeat([]byte{0x44}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	records := &memorySigningRecordRepository{}
	audit := &memoryAuditWriter{}
	alerts := &memoryAlertSink{}
	wallet := signingWallet()
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet},
		&walletKeyRootRepository{root: signingRoot()},
		secrets,
		&fakeSigningAdapter{},
		IntegrityDependencies{
			Records: records, Audit: audit, Alerts: alerts,
		},
	)

	first, err := svc.SignIntegrity(context.Background(), portin.SignIntegrityInput{
		WalletID: wallet.ID, Context: "ledger:journal-entry:v1", ObjectID: "journal-1",
		Payload:         []byte(`{"id":"journal-1","lines":[{"amount":10}],"note":"first"}`),
		IntegrityFields: []string{"/lines/0/amount", "/id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused {
		t.Fatal("first signature must not be reused")
	}
	if records.created != 1 {
		t.Fatalf("created records = %d", records.created)
	}

	// Field order and fields outside the protected subset do not matter.
	second, err := svc.SignIntegrity(context.Background(), portin.SignIntegrityInput{
		WalletID: wallet.ID, Context: "ledger:journal-entry:v1", ObjectID: "journal-1",
		Payload:         []byte(`{"note":"changed","lines":[{"amount":10.0}],"id":"journal-1"}`),
		IntegrityFields: []string{"/id", "/lines/0/amount"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || !bytes.Equal(first.Signature, second.Signature) || !bytes.Equal(first.Digest, second.Digest) {
		t.Fatal("same protected values must reuse the original signature")
	}
	if records.created != 1 {
		t.Fatalf("created records after reuse = %d", records.created)
	}

	_, err = svc.SignIntegrity(context.Background(), portin.SignIntegrityInput{
		WalletID: wallet.ID, Context: "ledger:journal-entry:v1", ObjectID: "journal-1",
		Payload:         []byte(`{"id":"journal-1","lines":[{"amount":99}]}`),
		IntegrityFields: []string{"/id", "/lines/0/amount"},
	})
	if !errors.Is(err, ErrIntegrityValueMismatch) {
		t.Fatalf("expected ErrIntegrityValueMismatch, got %v", err)
	}
	if len(audit.events) != 1 || len(alerts.alerts) != 1 || alerts.alerts[0].Code != "INTEGRITY_VALUE_MISMATCH" {
		t.Fatalf("expected audit+alert for value mismatch: audit=%d alerts=%+v", len(audit.events), alerts.alerts)
	}
	if audit.events[0].RetentionClass != domain.AuditRetentionCritical {
		t.Fatalf("integrity incident retention class = %q", audit.events[0].RetentionClass)
	}

	_, err = svc.SignIntegrity(context.Background(), portin.SignIntegrityInput{
		WalletID: wallet.ID, Context: "ledger:journal-entry:v1", ObjectID: "journal-1",
		Payload:         []byte(`{"id":"journal-1","lines":[{"amount":10}]}`),
		IntegrityFields: []string{"/id"},
	})
	if !errors.Is(err, ErrIntegritySchemaMismatch) {
		t.Fatalf("expected ErrIntegritySchemaMismatch, got %v", err)
	}
}

func TestSigningServiceIntegrityVerifyRequiresUnsealedVault(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	seed, err := securemem.New(bytes.Repeat([]byte{0x55}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}

	records := &memorySigningRecordRepository{}
	wallet := signingWallet()
	adapter := &fakeSigningAdapter{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: wallet}, &walletKeyRootRepository{root: signingRoot()},
		secrets, adapter, IntegrityDependencies{Records: records},
	)
	input := portin.SignIntegrityInput{
		WalletID: wallet.ID, Context: "ledger:test:v1", ObjectID: "object-1",
		Payload:         []byte(`{"id":"object-1","nested":{"amount":"10"}}`),
		IntegrityFields: []string{"/id", "/nested/amount"},
	}
	signed, err := svc.SignIntegrity(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	secrets.Clear()

	_, err = svc.VerifyIntegrity(context.Background(), portin.VerifyIntegrityInput{
		WalletID: input.WalletID, Context: input.Context, ObjectID: input.ObjectID,
		Payload: input.Payload, IntegrityFields: input.IntegrityFields, Signature: signed.Signature,
	})
	if !errors.Is(err, secretstore.ErrVaultSealed) {
		t.Fatalf("expected ErrVaultSealed, got %v", err)
	}
}

func TestSigningServiceGenericDoesNotCreateSigningRecord(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()
	seed, err := securemem.New(bytes.Repeat([]byte{0x22}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}
	records := &memorySigningRecordRepository{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: signingWallet()}, &walletKeyRootRepository{root: signingRoot()},
		secrets, &fakeSigningAdapter{}, IntegrityDependencies{Records: records},
	)
	if _, err := svc.SignData(context.Background(), portin.SignDataInput{
		WalletID: "wallet-1", Context: "ledger:journal-entry:v1", Format: domain.DataFormatJSON,
		Payload: []byte(`{"id":"j-1"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if records.created != 0 || records.record != nil {
		t.Fatal("generic signing must remain stateless")
	}
}

func TestSigningServiceIntegrityRejectsTamperedRecord(t *testing.T) {
	secrets := newUnsealedSecretStore(t)
	defer secrets.Clear()
	seed, err := securemem.New(bytes.Repeat([]byte{0x25}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.StoreKeyRootSeed("root-1", seed); err != nil {
		t.Fatal(err)
	}
	records := &memorySigningRecordRepository{}
	svc := NewSigningService(
		&memoryWalletRepository{wallet: signingWallet()}, &walletKeyRootRepository{root: signingRoot()},
		secrets, &fakeSigningAdapter{}, IntegrityDependencies{Records: records},
	)
	input := portin.SignIntegrityInput{
		WalletID: "wallet-1", Context: "ledger:v1", ObjectID: "j-1",
		Payload: []byte(`{"id":"j-1"}`), IntegrityFields: []string{"/id"},
	}
	signed, err := svc.SignIntegrity(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	records.record.EncryptedRecord[len(records.record.EncryptedRecord)-1] ^= 1

	_, err = svc.SignIntegrity(context.Background(), input)
	if !errors.Is(err, ErrSigningRecordTampered) {
		t.Fatalf("expected ErrSigningRecordTampered, got %v", err)
	}
	verified, err := svc.VerifyIntegrity(context.Background(), portin.VerifyIntegrityInput{
		WalletID: input.WalletID, Context: input.Context, ObjectID: input.ObjectID,
		Payload: input.Payload, IntegrityFields: input.IntegrityFields, Signature: signed.Signature,
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.Valid || verified.Reason != "SIGNING_RECORD_TAMPERED" {
		t.Fatalf("unexpected verify result: %+v", verified)
	}
}

type memorySigningRecordRepository struct {
	record  *domain.SigningRecord
	created int
}

func (r *memorySigningRecordRepository) Get(_ context.Context, contextName, objectID string) (*domain.SigningRecord, error) {
	if r.record == nil || r.record.Context != contextName || r.record.ObjectID != objectID {
		return nil, portout.ErrSigningRecordNotFound
	}
	copyRecord := *r.record
	copyRecord.EncryptedRecord = bytes.Clone(r.record.EncryptedRecord)
	return &copyRecord, nil
}

func (r *memorySigningRecordRepository) Create(_ context.Context, record domain.SigningRecord) error {
	if r.record != nil && r.record.Context == record.Context && r.record.ObjectID == record.ObjectID {
		return portout.ErrSigningRecordExists
	}
	copyRecord := record
	copyRecord.EncryptedRecord = bytes.Clone(record.EncryptedRecord)
	r.record = &copyRecord
	r.created++
	return nil
}

type memoryAuditWriter struct{ events []domain.AuditEvent }

func (m *memoryAuditWriter) Append(_ context.Context, event domain.AuditEvent) error {
	m.events = append(m.events, event)
	return nil
}

type memoryAlertSink struct{ alerts []domain.Alert }

func (m *memoryAlertSink) Send(_ context.Context, alert domain.Alert) error {
	m.alerts = append(m.alerts, alert)
	return nil
}
