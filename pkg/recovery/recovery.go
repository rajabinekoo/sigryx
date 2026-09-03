package recovery

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rajabinekoo/sigryx/pkg/cryptox"
)

const (
	Version   = 1
	keyPrefix = "rec_"
)

var (
	ErrInvalidKey         = errors.New("recovery: invalid recovery key")
	ErrInvalidBackup      = errors.New("recovery: invalid backup")
	ErrUnsupportedVersion = errors.New("recovery: unsupported backup version")
)

type Entry struct {
	ID               string `json:"id"`
	DerivationScheme string `json:"derivation_scheme"`
	EncryptedSeed    string `json:"encrypted_seed"`
}

type Manifest struct {
	Version  int     `json:"version"`
	KeyRoots []Entry `json:"key_roots"`
}

func GenerateKey() ([]byte, error) {
	return cryptox.RandomKey()
}

func EncodeKey(key []byte) (string, error) {
	if len(key) != cryptox.KeySize {
		return "", ErrInvalidKey
	}
	return keyPrefix + base64.RawURLEncoding.EncodeToString(key), nil
}

func DecodeKey(value string) ([]byte, error) {
	if !strings.HasPrefix(value, keyPrefix) {
		return nil, ErrInvalidKey
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, keyPrefix))
	if err != nil || len(key) != cryptox.KeySize {
		clear(key)
		return nil, ErrInvalidKey
	}
	return key, nil
}

func EncryptSeed(key []byte, id, derivationScheme string, seed []byte) (string, error) {
	sealed, err := cryptox.Seal(key, seed, seedAAD(id, derivationScheme))
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func DecryptSeed(key []byte, entry Entry) ([]byte, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(entry.EncryptedSeed)
	if err != nil {
		return nil, ErrInvalidBackup
	}
	plaintext, err := cryptox.Open(
		key,
		cryptox.SealedPayload(sealed),
		seedAAD(entry.ID, entry.DerivationScheme),
	)
	if err != nil {
		return nil, ErrInvalidBackup
	}
	return plaintext, nil
}

func EncodeBackup(key []byte, manifest Manifest) (string, error) {
	if manifest.Version != Version {
		return "", ErrUnsupportedVersion
	}

	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal recovery manifest: %w", err)
	}
	defer clear(raw)

	sealed, err := cryptox.Seal(key, raw, bundleAAD())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func DecodeBackup(key []byte, value string) (Manifest, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Manifest{}, ErrInvalidBackup
	}

	raw, err := cryptox.Open(key, cryptox.SealedPayload(sealed), bundleAAD())
	if err != nil {
		return Manifest{}, ErrInvalidBackup
	}
	defer clear(raw)

	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, ErrInvalidBackup
	}
	if manifest.Version != Version {
		return Manifest{}, ErrUnsupportedVersion
	}
	if len(manifest.KeyRoots) == 0 {
		return Manifest{}, ErrInvalidBackup
	}

	seen := make(map[string]struct{}, len(manifest.KeyRoots))
	for _, entry := range manifest.KeyRoots {
		if entry.ID == "" || entry.DerivationScheme == "" || entry.EncryptedSeed == "" {
			return Manifest{}, ErrInvalidBackup
		}
		if _, exists := seen[entry.ID]; exists {
			return Manifest{}, ErrInvalidBackup
		}
		seen[entry.ID] = struct{}{}
	}

	return manifest, nil
}

func seedAAD(id, derivationScheme string) []byte {
	return fmt.Appendf(nil, "sigryx:recovery:key-root:v1:%s:%s", id, derivationScheme)
}

func bundleAAD() []byte {
	return []byte("sigryx:recovery:bundle:v1")
}
