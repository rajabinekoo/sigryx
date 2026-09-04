package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
)

type SigningRecordRepository struct {
	pool *pgxpool.Pool
}

func NewSigningRecordRepository(pool *pgxpool.Pool) *SigningRecordRepository {
	return &SigningRecordRepository{pool: pool}
}

func (r *SigningRecordRepository) Get(ctx context.Context, contextName, objectID string) (*domain.SigningRecord, error) {
	var record domain.SigningRecord
	err := r.pool.QueryRow(ctx, `
SELECT id, context, object_id, encrypted_record, created_at
FROM signing_records
WHERE context = $1 AND object_id = $2
`, contextName, objectID).Scan(&record.ID, &record.Context, &record.ObjectID, &record.EncryptedRecord, &record.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrSigningRecordNotFound
		}
		return nil, fmt.Errorf("get signing record: %w", err)
	}
	return &record, nil
}

func (r *SigningRecordRepository) Create(ctx context.Context, record domain.SigningRecord) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO signing_records (id, context, object_id, encrypted_record, created_at)
VALUES ($1, $2, $3, $4, $5)
`, record.ID, record.Context, record.ObjectID, record.EncryptedRecord, record.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return portout.ErrSigningRecordExists
		}
		return fmt.Errorf("create signing record: %w", err)
	}
	return nil
}

var _ portout.SigningRecordRepository = (*SigningRecordRepository)(nil)
