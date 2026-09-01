package hdseed

import (
	"bytes"
	"testing"
)

func TestGenerateSize(t *testing.T) {
	seed, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	defer seed.Destroy()

	if seed.Size() != Size {
		t.Fatalf("seed size = %d, want %d", seed.Size(), Size)
	}
}

func TestGenerateProducesDifferentSeeds(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Destroy()

	second, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Destroy()

	var firstCopy []byte
	if err := first.WithBytes(func(seed []byte) error {
		firstCopy = bytes.Clone(seed)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer clear(firstCopy)

	if err := second.WithBytes(func(seed []byte) error {
		if bytes.Equal(firstCopy, seed) {
			t.Fatal("generated HD master seeds are identical")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
