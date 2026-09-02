package hdwallet

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestParsePathRoundTrip(t *testing.T) {
	path, err := ParsePath("m/44'/60'/0'/0/17")
	if err != nil {
		t.Fatal(err)
	}
	if got := path.String(); got != "m/44'/60'/0'/0/17" {
		t.Fatalf("path = %q", got)
	}
}

func TestSignDigestIsDeterministicAndVerifiable(t *testing.T) {
	privateKey := make([]byte, 32)
	privateKey[31] = 1
	publicKey, err := UncompressedPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("sigryx signing test"))

	first, err := SignDigest(privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	second, err := SignDigest(privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Compact(), second.Compact()) || first.RecoveryID != second.RecoveryID {
		t.Fatal("RFC6979 signature is not deterministic")
	}
	if !VerifyDigest(publicKey, digest[:], first.Compact()) {
		t.Fatal("signature did not verify")
	}
	if !VerifyDigestWithRecoveryID(publicKey, digest[:], first.Compact(), first.RecoveryID) {
		t.Fatal("signature recovery id did not verify")
	}
	if VerifyDigestWithRecoveryID(publicKey, digest[:], first.Compact(), first.RecoveryID^1) {
		t.Fatal("signature verified with wrong recovery id")
	}

	tampered := digest
	tampered[0] ^= 0xff
	if VerifyDigest(publicKey, tampered[:], first.Compact()) {
		t.Fatal("signature verified for a different digest")
	}
}
