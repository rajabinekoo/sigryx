package postgres

import (
	"context"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/ent"
)

type KeyRootRepository struct {
	client *ent.Client
}

func NewKeyRootRepository(client *ent.Client) *KeyRootRepository {
	return &KeyRootRepository{client: client}
}

func (r *KeyRootRepository) GetAll(ctx context.Context) ([]*domain.KeyRoot, error) {
	list, err := r.client.KeyRoot.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.KeyRoot, len(list))
	for i, k := range list {
		result[i] = &domain.KeyRoot{
			ID:               k.ID,
			SealedSeed:       k.SealedSeed,
			DerivationScheme: domain.DerivationScheme(k.DerivationScheme),
		}
	}

	return result, nil
}

func (r *KeyRootRepository) Create(
	ctx context.Context,
	root domain.KeyRoot,
) error {
	_, err := r.client.KeyRoot.
		Create().
		SetID(root.ID).
		SetDerivationScheme(string(root.DerivationScheme)).
		SetSealedSeed(root.SealedSeed).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return portout.ErrKeyRootAlreadyExists
		}

		return fmt.Errorf("create key root: %w", err)
	}

	return nil
}

var _ portout.KeyRootRepository = (*KeyRootRepository)(nil)
