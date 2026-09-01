package ethereum

import (
	"encoding/hex"
	"testing"
)

func TestEthereumAddressKnownVector(t *testing.T) {
	publicKey, err := hex.DecodeString("0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := ethereumAddress(publicKey), "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"; got != want {
		t.Fatalf("address = %s, want %s", got, want)
	}
}

func TestAdapterDeriveIsDeterministic(t *testing.T) {
	adapter := New()
	seed := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}

	first, err := adapter.Derive(seed, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Derive(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	if first.Address != second.Address {
		t.Fatalf("same seed and index returned different addresses: %s != %s", first.Address, second.Address)
	}
	if first.DerivationPath != "m/44'/60'/0'/0/0" {
		t.Fatalf("path = %q, want m/44'/60'/0'/0/0", first.DerivationPath)
	}
}
