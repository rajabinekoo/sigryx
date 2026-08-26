package cryptox

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
)

var (
	ErrEmptyKDFSecret = errors.New("kdf secret is required")
	ErrEmptyKDFInfo   = errors.New("kdf info is required")
)

// DeriveKey derives a 256-bit key using HKDF-SHA256.
//
// secret is the primary input keying material, salt is independent key
// material, and info provides domain separation between cryptographic uses.
func DeriveKey(secret, salt []byte, info string) ([]byte, error) {
	if len(secret) == 0 {
		return nil, ErrEmptyKDFSecret
	}
	if info == "" {
		return nil, ErrEmptyKDFInfo
	}

	key, err := hkdf.Key(sha256.New, secret, salt, info, KeySize)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	return key, nil
}
