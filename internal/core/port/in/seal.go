package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type InitializeVaultInput struct {
	UnsealKeyCount int
}

type InitializeVaultResult struct {
	Credentials []domain.UnsealCredential
}

type SubmitUnsealCredentialInput struct {
	SlotID      domain.UnsealSlotID
	WrappedKey  []byte
	OwnerSecret []byte
}

type SealStatus struct {
	State     domain.SealState
	Submitted int
	Required  int
}

type SealUseCase interface {
	Initialize(
		ctx context.Context,
		input InitializeVaultInput,
	) (*InitializeVaultResult, error)

	SubmitUnsealCredential(
		ctx context.Context,
		input SubmitUnsealCredentialInput,
	) (SealStatus, error)

	Seal(ctx context.Context) error

	Status(ctx context.Context) (SealStatus, error)
}
