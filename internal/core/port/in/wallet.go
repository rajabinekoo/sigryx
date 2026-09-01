package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type CreateWalletInput struct {
	KeyRootID  string
	UserID     string
	WalletType domain.WalletType
}

type WalletResult struct {
	ID             string            `json:"id"`
	KeyRootID      string            `json:"key_root_id"`
	UserID         string            `json:"user_id"`
	WalletType     domain.WalletType `json:"wallet_type"`
	Adapter        string            `json:"adapter"`
	DerivationPath string            `json:"derivation_path"`
	PublicKey      []byte            `json:"-"`
	Address        string            `json:"address"`
}

type WalletUseCase interface {
	Create(ctx context.Context, input CreateWalletInput) (*WalletResult, error)
}
