package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type ExportRecoveryInput struct {
	Principal domain.Principal
}

type ExportRecoveryResult struct {
	RecoveryKey string
	Backup      string
	KeyRoots    int
}

type ImportRecoveryInput struct {
	Principal   domain.Principal
	RecoveryKey string
	Backup      string
}

type ImportRecoveryResult struct {
	KeyRoots int
}

type RecoveryUseCase interface {
	Export(ctx context.Context, input ExportRecoveryInput) (*ExportRecoveryResult, error)
	Import(ctx context.Context, input ImportRecoveryInput) (*ImportRecoveryResult, error)
}
