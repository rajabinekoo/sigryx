package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/ent"
)

type UnsealKeySlotRepository struct {
	client *ent.Client
}

func NewUnsealKeySlotRepository(client *ent.Client) *UnsealKeySlotRepository {
	return &UnsealKeySlotRepository{client: client}
}

func (r *UnsealKeySlotRepository) Count(ctx context.Context) (int, error) {
	count, err := r.client.UnsealKeySlot.Query().Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unseal key slots: %w", err)
	}

	return count, nil
}

func (r *UnsealKeySlotRepository) CreateInitial(
	ctx context.Context,
	slots []domain.UnsealKeySlot,
) (err error) {
	if len(slots) == 0 {
		return nil
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start unseal initialization transaction: %w", err)
	}

	defer func() {
		if err == nil {
			return
		}

		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback unseal initialization: %w", rollbackErr))
		}
	}()

	exists, err := tx.UnsealKeySlot.Query().Exist(ctx)
	if err != nil {
		return fmt.Errorf("check existing unseal key slots: %w", err)
	}
	if exists {
		return portout.ErrAlreadyInitialized
	}

	builders := make([]*ent.UnsealKeySlotCreate, 0, len(slots))
	for _, slot := range slots {
		builders = append(
			builders,
			tx.UnsealKeySlot.
				Create().
				SetID(int(slot.ID)).
				SetWrappedKey(slot.WrappedKey).
				SetServerKeyMaterial(slot.ServerKeyMaterial),
		)
	}

	if _, err = tx.UnsealKeySlot.CreateBulk(builders...).Save(ctx); err != nil {
		if ent.IsConstraintError(err) {
			return portout.ErrAlreadyInitialized
		}

		return fmt.Errorf("create initial unseal key slots: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit unseal initialization: %w", err)
	}

	return nil
}

func (r *UnsealKeySlotRepository) GetByID(
	ctx context.Context,
	id domain.UnsealSlotID,
) (*domain.UnsealKeySlot, error) {
	slot, err := r.client.UnsealKeySlot.Get(ctx, int(id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, portout.ErrUnsealKeySlotNotFound
		}

		return nil, fmt.Errorf("get unseal key slot %d: %w", id, err)
	}

	return &domain.UnsealKeySlot{
		ID:                domain.UnsealSlotID(slot.ID),
		WrappedKey:        domain.WrappedUnsealKey(slot.WrappedKey),
		ServerKeyMaterial: domain.ServerKeyMaterial(slot.ServerKeyMaterial),
	}, nil
}

var _ portout.UnsealKeySlotRepository = (*UnsealKeySlotRepository)(nil)
