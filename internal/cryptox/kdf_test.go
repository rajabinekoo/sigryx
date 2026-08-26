package cryptox

import (
	"bytes"
	"errors"
	"testing"
)

func TestDeriveKeyIsDeterministic(t *testing.T) {
	secret := bytes.Repeat([]byte{0x11}, KeySize)
	salt := bytes.Repeat([]byte{0x22}, KeySize)

	first, err := DeriveKey(secret, salt, "sigryx:test:v1")
	if err != nil {
		t.Fatalf("first DeriveKey() error = %v", err)
	}

	second, err := DeriveKey(secret, salt, "sigryx:test:v1")
	if err != nil {
		t.Fatalf("second DeriveKey() error = %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("DeriveKey() is not deterministic")
	}
}

func TestDeriveKeySize(t *testing.T) {
	key, err := DeriveKey([]byte("owner-secret"), []byte("server-material"), "sigryx:test:v1")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	if len(key) != KeySize {
		t.Fatalf("DeriveKey() size = %d, want %d", len(key), KeySize)
	}
}

func TestDeriveKeyDifferentSecretProducesDifferentKey(t *testing.T) {
	salt := bytes.Repeat([]byte{0x22}, KeySize)

	first, err := DeriveKey(bytes.Repeat([]byte{0x11}, KeySize), salt, "sigryx:test:v1")
	if err != nil {
		t.Fatalf("first DeriveKey() error = %v", err)
	}

	second, err := DeriveKey(bytes.Repeat([]byte{0x12}, KeySize), salt, "sigryx:test:v1")
	if err != nil {
		t.Fatalf("second DeriveKey() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("different secrets produced identical derived keys")
	}
}

func TestDeriveKeyDifferentSaltProducesDifferentKey(t *testing.T) {
	secret := bytes.Repeat([]byte{0x11}, KeySize)

	first, err := DeriveKey(secret, bytes.Repeat([]byte{0x22}, KeySize), "sigryx:test:v1")
	if err != nil {
		t.Fatalf("first DeriveKey() error = %v", err)
	}

	second, err := DeriveKey(secret, bytes.Repeat([]byte{0x23}, KeySize), "sigryx:test:v1")
	if err != nil {
		t.Fatalf("second DeriveKey() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("different salts produced identical derived keys")
	}
}

func TestDeriveKeyDifferentInfoProducesDifferentKey(t *testing.T) {
	secret := bytes.Repeat([]byte{0x11}, KeySize)
	salt := bytes.Repeat([]byte{0x22}, KeySize)

	first, err := DeriveKey(secret, salt, "sigryx:test:a:v1")
	if err != nil {
		t.Fatalf("first DeriveKey() error = %v", err)
	}

	second, err := DeriveKey(secret, salt, "sigryx:test:b:v1")
	if err != nil {
		t.Fatalf("second DeriveKey() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("different info values produced identical derived keys")
	}
}

func TestDeriveKeyRejectsEmptySecret(t *testing.T) {
	_, err := DeriveKey(nil, []byte("server-material"), "sigryx:test:v1")
	if !errors.Is(err, ErrEmptyKDFSecret) {
		t.Fatalf("DeriveKey() error = %v, want %v", err, ErrEmptyKDFSecret)
	}
}

func TestDeriveKeyRejectsEmptyInfo(t *testing.T) {
	_, err := DeriveKey([]byte("owner-secret"), []byte("server-material"), "")
	if !errors.Is(err, ErrEmptyKDFInfo) {
		t.Fatalf("DeriveKey() error = %v, want %v", err, ErrEmptyKDFInfo)
	}
}
