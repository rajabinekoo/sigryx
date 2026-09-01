package out

import (
	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type DerivedWallet struct {
	DerivationPath string
	PublicKey      []byte
	Address        string
}

type WalletAdapter interface {
	Name() string
	WalletType() domain.WalletType
	DerivationScheme() domain.DerivationScheme
	Derive(seed []byte, index uint32) (DerivedWallet, error)
}
