package cryptox

import (
	"crypto/rand"
	"fmt"
)

const KeySize = 32

// RandomKey returns 256 bits of cryptographically secure random key material.
func RandomKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}
	return key, nil
}
