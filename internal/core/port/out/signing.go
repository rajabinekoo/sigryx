package out

import (
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrInvalidTransaction = errors.New("signing adapter: invalid transaction")
	ErrInvalidTypedData   = errors.New("signing adapter: invalid typed data")
	ErrInvalidSignature   = errors.New("signing adapter: invalid signature")
)

type TransactionSignature struct {
	RawTransaction []byte
	Hash           []byte
	R              []byte
	S              []byte
	YParity        uint8
}

type SigningAdapter interface {
	Name() string
	DerivationScheme() domain.DerivationScheme
	WithPrivateKey(seed []byte, derivationPath string, fn func([]byte) error) error

	SignTransaction(privateKey []byte, tx domain.EthereumTransaction) (TransactionSignature, error)
	VerifyTransaction(publicKey, rawTransaction []byte) (bool, error)

	SignTypedData(privateKey, typedData []byte) (signature, digest []byte, err error)
	VerifyTypedData(publicKey, typedData, signature []byte) (valid bool, digest []byte, err error)

	SignDigest(privateKey, digest []byte) ([]byte, error)
	VerifyDigest(publicKey, digest, signature []byte) bool
}
