package ethereum

import (
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/hdwallet"
)

var (
	ErrInvalidTransaction = portout.ErrInvalidTransaction
	ErrInvalidTypedData   = portout.ErrInvalidTypedData
	ErrInvalidSignature   = portout.ErrInvalidSignature
)

func (*Adapter) WithPrivateKey(seed []byte, derivationPath string, fn func([]byte) error) error {
	path, err := hdwallet.ParsePath(derivationPath)
	if err != nil {
		return fmt.Errorf("parse wallet derivation path: %w", err)
	}
	return hdwallet.DerivePrivateKey(seed, path, fn)
}

func (*Adapter) SignDigest(privateKey, digest []byte) ([]byte, error) {
	sig, err := hdwallet.SignDigest(privateKey, digest)
	if err != nil {
		return nil, err
	}
	return sig.Compact(), nil
}

func (*Adapter) VerifyDigest(publicKey, digest, signature []byte) bool {
	return hdwallet.VerifyDigest(publicKey, digest, signature)
}

func (a *Adapter) SignTransaction(
	privateKey []byte,
	tx domain.EthereumTransaction,
) (portout.TransactionSignature, error) {
	return signTransaction(privateKey, tx)
}

func (a *Adapter) VerifyTransaction(
	publicKey []byte,
	rawTransaction []byte,
) (bool, error) {
	return verifyTransaction(publicKey, rawTransaction)
}

func (a *Adapter) SignTypedData(
	privateKey []byte,
	typedData []byte,
) ([]byte, []byte, error) {
	digest, err := typedDataDigest(typedData)
	if err != nil {
		return nil, nil, err
	}

	sig, err := hdwallet.SignDigest(privateKey, digest[:])
	if err != nil {
		return nil, nil, err
	}
	return sig.Ethereum(), append([]byte(nil), digest[:]...), nil
}

func (a *Adapter) VerifyTypedData(
	publicKey []byte,
	typedData []byte,
	signature []byte,
) (bool, []byte, error) {
	digest, err := typedDataDigest(typedData)
	if err != nil {
		return false, nil, err
	}

	if _, err := hdwallet.ParseEthereumSignature(signature); err != nil {
		return false, nil, ErrInvalidSignature
	}

	return hdwallet.VerifyEthereumDigest(publicKey, digest[:], signature), append([]byte(nil), digest[:]...), nil
}

var _ portout.SigningAdapter = (*Adapter)(nil)
