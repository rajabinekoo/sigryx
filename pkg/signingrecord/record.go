package signingrecord

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/pkg/cryptox"
)

var ErrInvalidRecord = errors.New("signingrecord: invalid encrypted record")

type Record struct {
	WalletID        string   `json:"wallet_id"`
	IntegrityFields []string `json:"integrity_fields"`
	CanonicalData   []byte   `json:"canonical_data"`
	Digest          []byte   `json:"digest"`
	Signature       []byte   `json:"signature"`
}

func Seal(key []byte, contextName, objectID string, record Record) ([]byte, error) {
	plain, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode integrity record: %w", err)
	}
	defer clear(plain)
	sealed, err := cryptox.Seal(key, plain, aad(contextName, objectID))
	if err != nil {
		return nil, err
	}
	return []byte(sealed), nil
}

func Open(key []byte, contextName, objectID string, encrypted []byte) (*Record, error) {
	plain, err := cryptox.Open(key, cryptox.SealedPayload(encrypted), aad(contextName, objectID))
	if err != nil {
		return nil, ErrInvalidRecord
	}
	defer clear(plain)

	var record Record
	if err := json.Unmarshal(plain, &record); err != nil {
		return nil, ErrInvalidRecord
	}
	if record.WalletID == "" || len(record.IntegrityFields) == 0 || len(record.CanonicalData) == 0 || len(record.Digest) != 32 || len(record.Signature) != 64 {
		return nil, ErrInvalidRecord
	}
	return &record, nil
}

func aad(contextName, objectID string) []byte {
	result := []byte("sigryx:signing-record:v1")
	result = appendFramed(result, []byte(contextName))
	result = appendFramed(result, []byte(objectID))
	return result
}

func appendFramed(dst, value []byte) []byte {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	dst = append(dst, size[:]...)
	dst = append(dst, value...)
	return dst
}
