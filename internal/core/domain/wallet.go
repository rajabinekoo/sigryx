package domain

type Wallet struct {
	ID             string
	KeyRootID      string
	UserID         string
	Adapter        string
	DerivationPath string
	PublicKey      []byte
	Address        string
}
