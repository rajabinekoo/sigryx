package ethereum

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/hdwallet"
)

const (
	adapterName = "evm"
	coinType    = 60
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (*Adapter) Name() string {
	return adapterName
}

func (*Adapter) WalletType() domain.WalletType {
	return domain.WalletTypeEthereum
}

func (*Adapter) DerivationScheme() domain.DerivationScheme {
	return domain.DerivationSchemeBIP32Secp256k1
}

func (*Adapter) Derive(seed []byte, index uint32) (portout.DerivedWallet, error) {
	path, err := hdwallet.BIP44(coinType, 0, 0, index)
	if err != nil {
		return portout.DerivedWallet{}, fmt.Errorf("build ethereum derivation path: %w", err)
	}

	var publicKey []byte
	var address string

	err = hdwallet.DerivePrivateKey(seed, path, func(privateKey []byte) error {
		derivedPublicKey, err := hdwallet.UncompressedPublicKey(privateKey)
		if err != nil {
			return err
		}

		publicKey = derivedPublicKey
		address = ethereumAddress(derivedPublicKey)
		return nil
	})
	if err != nil {
		return portout.DerivedWallet{}, fmt.Errorf("derive ethereum wallet: %w", err)
	}

	return portout.DerivedWallet{
		DerivationPath: path.String(),
		PublicKey:      publicKey,
		Address:        address,
	}, nil
}

func ethereumAddress(publicKey []byte) string {
	digest := keccak256(publicKey[1:])
	lower := hex.EncodeToString(digest[len(digest)-20:])

	checksum := keccak256([]byte(lower))

	address := []byte(lower)
	for i := range address {
		if address[i] >= '0' && address[i] <= '9' {
			continue
		}

		nibble := checksum[i/2]
		if i%2 == 0 {
			nibble >>= 4
		} else {
			nibble &= 0x0f
		}

		if nibble >= 8 {
			address[i] = byte(strings.ToUpper(string(address[i]))[0])
		}
	}

	return "0x" + string(address)
}

var _ portout.WalletAdapter = (*Adapter)(nil)
