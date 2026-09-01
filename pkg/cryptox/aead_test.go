package cryptox

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)
	plaintext := []byte("sigryx secret")
	aad := []byte("sigryx:test:v1")

	sealed, err := Seal(key, plaintext, aad)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	opened, err := Open(key, sealed, aad)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("Open() = %x, want %x", opened, plaintext)
	}
}

func TestSealRejectsInvalidKeySize(t *testing.T) {
	_, err := Seal([]byte("short"), []byte("secret"), nil)
	if !errors.Is(err, ErrInvalidKeySize) {
		t.Fatalf("Seal() error = %v, want %v", err, ErrInvalidKeySize)
	}
}

func TestOpenRejectsInvalidKeySize(t *testing.T) {
	_, err := Open([]byte("short"), SealedPayload{sealedPayloadVersion}, nil)
	if !errors.Is(err, ErrInvalidKeySize) {
		t.Fatalf("Open() error = %v, want %v", err, ErrInvalidKeySize)
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)
	wrongKey := bytes.Repeat([]byte{0x22}, KeySize)

	sealed, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	if _, err := Open(wrongKey, sealed, []byte("aad")); err == nil {
		t.Fatal("Open() succeeded with wrong key")
	}
}

func TestOpenRejectsWrongAAD(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)

	sealed, err := Seal(key, []byte("secret"), []byte("correct-aad"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	if _, err := Open(key, sealed, []byte("wrong-aad")); err == nil {
		t.Fatal("Open() succeeded with wrong AAD")
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)

	sealed, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	tampered := append(SealedPayload(nil), sealed...)
	tampered[len(tampered)-1] ^= 0xff

	if _, err := Open(key, tampered, []byte("aad")); err == nil {
		t.Fatal("Open() succeeded with tampered ciphertext")
	}
}

func TestSealUsesDifferentNonce(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)
	plaintext := []byte("same secret")
	aad := []byte("same aad")

	first, err := Seal(key, plaintext, aad)
	if err != nil {
		t.Fatalf("first Seal() error = %v", err)
	}

	second, err := Seal(key, plaintext, aad)
	if err != nil {
		t.Fatalf("second Seal() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("Seal() produced identical payloads")
	}
}

func TestOpenRejectsMalformedPayload(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)

	_, err := Open(key, SealedPayload{sealedPayloadVersion}, nil)
	if !errors.Is(err, ErrMalformedSealedPayload) {
		t.Fatalf("Open() error = %v, want %v", err, ErrMalformedSealedPayload)
	}
}

func TestOpenRejectsUnsupportedVersion(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)

	sealed, err := Seal(key, []byte("secret"), nil)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	sealed[0] = sealedPayloadVersion + 1

	_, err = Open(key, sealed, nil)
	if !errors.Is(err, ErrUnsupportedPayloadVersion) {
		t.Fatalf("Open() error = %v, want %v", err, ErrUnsupportedPayloadVersion)
	}
}

func TestOpenRejectsTamperedNonce(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)

	sealed, err := Seal(key, []byte("secret"), []byte("aad"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	tampered := append(SealedPayload(nil), sealed...)
	tampered[1] ^= 0xff

	if _, err := Open(key, tampered, []byte("aad")); err == nil {
		t.Fatal("Open() succeeded with tampered nonce")
	}
}
