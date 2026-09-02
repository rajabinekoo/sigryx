package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrWalletNotFound           = errors.New("wallet repository: wallet not found")
	ErrWalletAlreadyExists      = errors.New("wallet repository: wallet already exists")
	ErrDerivationIndexExhausted = errors.New("wallet repository: derivation index exhausted")
)

type WalletRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Wallet, error)

	GetByUser(
		ctx context.Context,
		keyRootID string,
		adapter string,
		userID string,
	) (*domain.Wallet, error)

	NextIndex(
		ctx context.Context,
		keyRootID string,
		adapter string,
	) (uint32, error)

	Create(ctx context.Context, wallet domain.Wallet) error
}
