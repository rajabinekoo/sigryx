package domain

// WalletType is the wallet profile requested by an API consumer.
//
// It is not persisted on KeyRoot. A wallet type only selects the cryptographic
// derivation scheme required to create the root. Blockchain-specific behavior
// remains an adapter concern.
type WalletType string

const (
	WalletTypeEthereum WalletType = "ETHEREUM"
)

// DerivationScheme describes how child private keys are derived from a KeyRoot.
// It intentionally contains no blockchain identity.
type DerivationScheme string

const (
	DerivationSchemeBIP32Secp256k1 DerivationScheme = "BIP32_SECP256K1"
)

func (w WalletType) DerivationScheme() (DerivationScheme, bool) {
	switch w {
	case WalletTypeEthereum:
		return DerivationSchemeBIP32Secp256k1, true
	default:
		return "", false
	}
}

func (w DerivationScheme) WalletType() (WalletType, bool) {
	switch w {
	case DerivationSchemeBIP32Secp256k1:
		return WalletTypeEthereum, true
	default:
		return "", false
	}
}

// KeyRoot is durable metadata for one encrypted HD master seed.
// Plaintext seed material is never represented by this entity.
type KeyRoot struct {
	ID               string
	DerivationScheme DerivationScheme
	SealedSeed       []byte
}
