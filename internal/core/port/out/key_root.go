package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrKeyRootAlreadyExists = errors.New("key root repository: key root already exists")
	ErrKeyRootNotFound      = errors.New("key root repository: key root not found")
)

type KeyRootRepository interface {
	Create(ctx context.Context, root domain.KeyRoot) error
	GetByID(ctx context.Context, id string) (*domain.KeyRoot, error)
	GetAll(ctx context.Context) ([]*domain.KeyRoot, error)
}
