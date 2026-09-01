package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type CreateKeyRootInput struct {
	WalletType domain.WalletType
}

type CreateKeyRootResult struct {
	ID               string
	WalletType       domain.WalletType
	DerivationScheme domain.DerivationScheme
}

type KeyRootUseCase interface {
	Create(
		ctx context.Context,
		input CreateKeyRootInput,
	) (*CreateKeyRootResult, error)

	GetAll(ctx context.Context) ([]*CreateKeyRootResult, error)
}
