package postgres

import (
	"context"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/ent"
	"github.com/rajabinekoo/sigryx/internal/ent/keyroot"
)

type KeyRootRepository struct {
	client *ent.Client
}

func NewKeyRootRepository(client *ent.Client) *KeyRootRepository {
	return &KeyRootRepository{client: client}
}

func (r *KeyRootRepository) GetByID(
	ctx context.Context,
	id string,
) (*domain.KeyRoot, error) {
	item, err := r.client.KeyRoot.Query().
		Where(keyroot.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, portout.ErrKeyRootNotFound
		}
		return nil, fmt.Errorf("get key root: %w", err)
	}

	return &domain.KeyRoot{
		ID:               item.ID,
		DerivationScheme: domain.DerivationScheme(item.DerivationScheme),
		SealedSeed:       item.SealedSeed,
	}, nil
}

func (r *KeyRootRepository) GetAll(ctx context.Context) ([]*domain.KeyRoot, error) {
	list, err := r.client.KeyRoot.Query().
		Select(keyroot.FieldID, keyroot.FieldDerivationScheme).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.KeyRoot, len(list))
	for i, k := range list {
		result[i] = &domain.KeyRoot{
			ID:               k.ID,
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

func (r *KeyRootRepository) GetKeyRootsForRecovery(
	ctx context.Context,
) ([]*domain.KeyRoot, error) {
	list, err := r.client.KeyRoot.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list key roots for recovery: %w", err)
	}

	result := make([]*domain.KeyRoot, len(list))
	for i, item := range list {
		result[i] = &domain.KeyRoot{
			ID:               item.ID,
			DerivationScheme: domain.DerivationScheme(item.DerivationScheme),
			SealedSeed:       item.SealedSeed,
		}
	}
	return result, nil
}

func (r *KeyRootRepository) RestoreKeyRoots(
	ctx context.Context,
	roots []domain.KeyRoot,
) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin key root recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, root := range roots {
		existing, err := tx.KeyRoot.Query().
			Where(keyroot.IDEQ(root.ID)).
			Only(ctx)
		switch {
		case ent.IsNotFound(err):
			if _, err := tx.KeyRoot.Create().
				SetID(root.ID).
				SetDerivationScheme(string(root.DerivationScheme)).
				SetSealedSeed(root.SealedSeed).
				Save(ctx); err != nil {
				return fmt.Errorf("create recovered key root %s: %w", root.ID, err)
			}

		case err != nil:
			return fmt.Errorf("read key root %s during recovery: %w", root.ID, err)

		default:
			if existing.DerivationScheme != string(root.DerivationScheme) {
				return portout.ErrRecoveryKeyRootConflict
			}

			// sealed_seed is immutable in normal application flows. Recovery is
			// the intentional exception: keep the Ent schema immutable and update
			// only this field through the upsert update set inside the transaction.
			if err := tx.KeyRoot.Create().
				SetID(root.ID).
				SetDerivationScheme(string(root.DerivationScheme)).
				SetSealedSeed(root.SealedSeed).
				OnConflictColumns(keyroot.FieldID).
				Update(func(update *ent.KeyRootUpsert) {
					update.Set(keyroot.FieldSealedSeed, root.SealedSeed)
				}).
				Exec(ctx); err != nil {
				return fmt.Errorf("update recovered key root %s: %w", root.ID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit key root recovery: %w", err)
	}
	return nil
}

var _ portout.RecoveryRepository = (*KeyRootRepository)(nil)
