package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var ErrRecoveryKeyRootConflict = errors.New("recovery repository: key root derivation scheme conflict")

type RecoveryRepository interface {
	GetKeyRootsForRecovery(ctx context.Context) ([]*domain.KeyRoot, error)
	RestoreKeyRoots(ctx context.Context, roots []domain.KeyRoot) error
}
