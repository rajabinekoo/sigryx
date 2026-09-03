package recovery

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/rajabinekoo/sigryx/pkg/cryptox"
)

func TestRecoveryKeyRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)

	encoded, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(decoded)

	if !bytes.Equal(decoded, key) {
		t.Fatal("decoded recovery key does not match generated key")
	}
}

func TestRecoveryBackupRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, cryptox.KeySize)
	seed := bytes.Repeat([]byte{0x42}, 32)

	encryptedSeed, err := EncryptSeed(key, "root-1", "BIP32_SECP256K1", seed)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeBackup(key, Manifest{
		Version: Version,
		KeyRoots: []Entry{{
			ID:               "root-1",
			DerivationScheme: "BIP32_SECP256K1",
			EncryptedSeed:    encryptedSeed,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := DecodeBackup(key, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.KeyRoots) != 1 {
		t.Fatalf("key root count = %d, want 1", len(manifest.KeyRoots))
	}

	opened, err := DecryptSeed(key, manifest.KeyRoots[0])
	if err != nil {
		t.Fatal(err)
	}
	defer clear(opened)

	if !bytes.Equal(opened, seed) {
		t.Fatal("decrypted seed does not match original seed")
	}
}

func TestRecoveryRejectsWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, cryptox.KeySize)
	wrong := bytes.Repeat([]byte{0x22}, cryptox.KeySize)

	encryptedSeed, err := EncryptSeed(key, "root-1", "BIP32_SECP256K1", bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	backup, err := EncodeBackup(key, Manifest{
		Version:  Version,
		KeyRoots: []Entry{{ID: "root-1", DerivationScheme: "BIP32_SECP256K1", EncryptedSeed: encryptedSeed}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeBackup(wrong, backup); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("expected ErrInvalidBackup, got %v", err)
	}
}

func TestRecoverySeedAADBindsMetadata(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, cryptox.KeySize)
	encrypted, err := EncryptSeed(key, "root-1", "BIP32_SECP256K1", bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptSeed(key, Entry{
		ID:               "root-2",
		DerivationScheme: "BIP32_SECP256K1",
		EncryptedSeed:    encrypted,
	})
	if !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("expected ErrInvalidBackup, got %v", err)
	}
}

func TestRecoveryRejectsTamperedBackup(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, cryptox.KeySize)

	encryptedSeed, err := EncryptSeed(
		key,
		"root-1",
		"BIP32_SECP256K1",
		bytes.Repeat([]byte{0x42}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	backup, err := EncodeBackup(key, Manifest{
		Version: Version,
		KeyRoots: []Entry{
			{
				ID:               "root-1",
				DerivationScheme: "BIP32_SECP256K1",
				EncryptedSeed:    encryptedSeed,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := base64.RawURLEncoding.DecodeString(backup)
	if err != nil {
		t.Fatal(err)
	}

	// Change an actual byte of the authenticated payload,
	// not a Base64 representation bit.
	sealed[len(sealed)-1] ^= 1

	tampered := base64.RawURLEncoding.EncodeToString(sealed)

	if _, err := DecodeBackup(key, tampered); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf(
			"expected ErrInvalidBackup, got %v",
			err,
		)
	}
}
