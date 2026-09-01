package hdwallet

import (
	"encoding/hex"
	"testing"
)

func TestBIP32Vector1(t *testing.T) {
	seed, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	if err != nil {
		t.Fatal(err)
	}

	path, err := NewPath(
		Segment{Index: 0, Hardened: true},
		Segment{Index: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	const expected = "3c6cb8d0f6a264c91ea8b5030fadaa8e538b020f0a387421a12de9319dc93368"

	err = DerivePrivateKey(seed, path, func(privateKey []byte) error {
		if got := hex.EncodeToString(privateKey); got != expected {
			t.Fatalf("private key = %s, want %s", got, expected)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBIP44Path(t *testing.T) {
	path, err := BIP44(60, 0, 0, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := path.String(), "m/44'/60'/0'/0/7"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestUncompressedPublicKeyForPrivateKeyOne(t *testing.T) {
	privateKey := make([]byte, 32)
	privateKey[31] = 1

	publicKey, err := UncompressedPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	const expected = "0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8"
	if got := hex.EncodeToString(publicKey); got != expected {
		t.Fatalf("public key = %s, want %s", got, expected)
	}
}
