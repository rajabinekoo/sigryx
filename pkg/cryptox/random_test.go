package cryptox

import (
	"bytes"
	"testing"
)

func TestRandomKeySize(t *testing.T) {
	key, err := RandomKey()
	if err != nil {
		t.Fatalf("RandomKey() error = %v", err)
	}

	if len(key) != KeySize {
		t.Fatalf("RandomKey() size = %d, want %d", len(key), KeySize)
	}
}

func TestRandomKeysAreDifferent(t *testing.T) {
	first, err := RandomKey()
	if err != nil {
		t.Fatalf("first RandomKey() error = %v", err)
	}

	second, err := RandomKey()
	if err != nil {
		t.Fatalf("second RandomKey() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("RandomKey() returned identical keys")
	}
}
