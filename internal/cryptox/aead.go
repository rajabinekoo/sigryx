package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const sealedPayloadVersion byte = 1

var (
	ErrInvalidKeySize            = errors.New("key must be 32 bytes")
	ErrMalformedSealedPayload    = errors.New("malformed sealed payload")
	ErrUnsupportedPayloadVersion = errors.New("unsupported sealed payload version")
)

// SealedPayload is a self-contained authenticated encrypted blob:
//
//	version || nonce || ciphertext+authentication-tag
//
// It is safe to persist as binary data. It does not contain plaintext key
// material.
type SealedPayload []byte

// Seal encrypts plaintext using AES-256-GCM and authenticates aad.
func Seal(key, plaintext, aad []byte) (SealedPayload, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	payload := make([]byte, 1+len(nonce), 1+len(nonce)+len(plaintext)+gcm.Overhead())
	payload[0] = sealedPayloadVersion
	copy(payload[1:], nonce)
	payload = gcm.Seal(payload, nonce, plaintext, aad)

	return SealedPayload(payload), nil
}

// Open authenticates and decrypts a payload produced by Seal.
func Open(key []byte, payload SealedPayload, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	minimumSize := 1 + gcm.NonceSize() + gcm.Overhead()
	if len(payload) < minimumSize {
		return nil, ErrMalformedSealedPayload
	}
	if payload[0] != sealedPayloadVersion {
		return nil, ErrUnsupportedPayloadVersion
	}

	nonceStart := 1
	nonceEnd := nonceStart + gcm.NonceSize()
	nonce := payload[nonceStart:nonceEnd]
	ciphertext := payload[nonceEnd:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("open sealed payload: %w", err)
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return gcm, nil
}
