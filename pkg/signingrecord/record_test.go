package signingrecord

import (
	"bytes"
	"errors"
	"testing"
)

func TestRecordRoundTripAndAADBinding(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	record := Record{
		WalletID: "wallet-1", IntegrityFields: []string{"/id", "/amount"},
		CanonicalData: []byte(`{"/amount":"10","/id":"j-1"}`),
		Digest:        bytes.Repeat([]byte{0x22}, 32), Signature: bytes.Repeat([]byte{0x33}, 64),
	}

	sealed, err := Seal(key, "ledger:journal:v1", "j-1", record)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(key, "ledger:journal:v1", "j-1", sealed)
	if err != nil {
		t.Fatal(err)
	}
	if opened.WalletID != record.WalletID || !bytes.Equal(opened.Digest, record.Digest) {
		t.Fatalf("opened record = %+v", opened)
	}

	_, err = Open(key, "ledger:journal:v1", "j-2", sealed)
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("expected ErrInvalidRecord, got %v", err)
	}
}

func TestRecordRejectsTampering(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	sealed, err := Seal(key, "ctx", "obj", Record{
		WalletID: "wallet-1", IntegrityFields: []string{"/id"}, CanonicalData: []byte(`{"/id":"obj"}`),
		Digest: bytes.Repeat([]byte{0x22}, 32), Signature: bytes.Repeat([]byte{0x33}, 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := Open(key, "ctx", "obj", sealed); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("expected ErrInvalidRecord, got %v", err)
	}
}
