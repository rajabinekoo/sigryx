package ethereum

import (
	"encoding/hex"
	"testing"
)

func TestKeccak256EmptyVector(t *testing.T) {
	digest := keccak256(nil)
	const expected = "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	if got := hex.EncodeToString(digest[:]); got != expected {
		t.Fatalf("digest = %s, want %s", got, expected)
	}
}
