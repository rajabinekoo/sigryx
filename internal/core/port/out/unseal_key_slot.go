package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrAlreadyInitialized = errors.New(
		"unseal repository: vault is already initialized",
	)

	ErrUnsealKeySlotNotFound = errors.New(
		"unseal repository: slot not found",
	)
)

type UnsealKeySlotRepository interface {
	Count(ctx context.Context) (int, error)

	// CreateInitial must persist every slot atomically.
	//
	// Either all slots are created or none of them are.
	// It must return ErrAlreadyInitialized when slots
	// already exist.
	CreateInitial(
		ctx context.Context,
		slots []domain.UnsealKeySlot,
	) error

	GetByID(
		ctx context.Context,
		id domain.UnsealSlotID,
	) (*domain.UnsealKeySlot, error)
}
