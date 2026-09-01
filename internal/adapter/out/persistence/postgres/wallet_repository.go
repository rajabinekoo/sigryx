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

type WalletRepository struct {
	pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

func (r *WalletRepository) GetByUser(
	ctx context.Context,
	keyRootID string,
	adapter string,
	userID string,
) (*domain.Wallet, error) {
	const query = `
SELECT id, key_root_id, user_id, adapter, derivation_path, public_key, address
FROM wallets
WHERE key_root_id = $1 AND adapter = $2 AND user_id = $3
`

	wallet := &domain.Wallet{}
	err := r.pool.QueryRow(ctx, query, keyRootID, adapter, userID).Scan(
		&wallet.ID,
		&wallet.KeyRootID,
		&wallet.UserID,
		&wallet.Adapter,
		&wallet.DerivationPath,
		&wallet.PublicKey,
		&wallet.Address,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrWalletNotFound
		}
		return nil, fmt.Errorf("get wallet by user: %w", err)
	}

	return wallet, nil
}

func (r *WalletRepository) NextIndex(
	ctx context.Context,
	keyRootID string,
	adapter string,
) (uint32, error) {
	const query = `
INSERT INTO wallet_counters (key_root_id, adapter, next_index)
VALUES ($1, $2, 1)
ON CONFLICT (key_root_id, adapter)
DO UPDATE SET next_index = wallet_counters.next_index + 1
WHERE wallet_counters.next_index < 2147483648
RETURNING next_index - 1
`

	var index int64
	if err := r.pool.QueryRow(ctx, query, keyRootID, adapter).Scan(&index); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, portout.ErrDerivationIndexExhausted
		}
		return 0, fmt.Errorf("allocate wallet derivation index: %w", err)
	}

	return uint32(index), nil
}

func (r *WalletRepository) Create(
	ctx context.Context,
	wallet domain.Wallet,
) error {
	const query = `
INSERT INTO wallets (
    id,
    key_root_id,
    user_id,
    adapter,
    derivation_path,
    public_key,
    address
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

	_, err := r.pool.Exec(
		ctx,
		query,
		wallet.ID,
		wallet.KeyRootID,
		wallet.UserID,
		wallet.Adapter,
		wallet.DerivationPath,
		wallet.PublicKey,
		wallet.Address,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return portout.ErrWalletAlreadyExists
		}
		return fmt.Errorf("create wallet: %w", err)
	}

	return nil
}

var _ portout.WalletRepository = (*WalletRepository)(nil)
