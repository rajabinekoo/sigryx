package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
)

type AccessRepository struct {
	pool *pgxpool.Pool
}

func NewAccessRepository(pool *pgxpool.Pool) *AccessRepository {
	return &AccessRepository{pool: pool}
}

func (r *AccessRepository) CountRootAdmins(ctx context.Context) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_root_admin = true`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count root admins: %w", err)
	}
	return count, nil
}

func (r *AccessRepository) CreateUser(ctx context.Context, user domain.User) error {
	cidrs, err := json.Marshal(user.AllowedCIDRs)
	if err != nil {
		return fmt.Errorf("marshal user CIDRs: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO users (id, username, password_hash, is_root_admin, active, role_id, allowed_cidrs)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7::jsonb)
`, user.ID, user.Username, user.PasswordHash, user.IsRootAdmin, user.Active, user.RoleID, cidrs)
	return mapAccessWriteError("create user", err)
}

func (r *AccessRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return r.getUser(ctx, `
SELECT id, username, password_hash, is_root_admin, active, COALESCE(role_id::text, ''), allowed_cidrs
FROM users WHERE id = $1
`, id)
}

func (r *AccessRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.getUser(ctx, `
SELECT id, username, password_hash, is_root_admin, active, COALESCE(role_id::text, ''), allowed_cidrs
FROM users WHERE username = $1
`, username)
}

func (r *AccessRepository) getUser(ctx context.Context, query string, arg any) (*domain.User, error) {
	var user domain.User
	var cidrs []byte
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsRootAdmin,
		&user.Active,
		&user.RoleID,
		&cidrs,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if err := json.Unmarshal(cidrs, &user.AllowedCIDRs); err != nil {
		return nil, fmt.Errorf("decode user CIDRs: %w", err)
	}
	return &user, nil
}

func (r *AccessRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, username, password_hash, is_root_admin, active, COALESCE(role_id::text, ''), allowed_cidrs
FROM users ORDER BY username
`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var result []domain.User
	for rows.Next() {
		var user domain.User
		var cidrs []byte
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsRootAdmin, &user.Active, &user.RoleID, &cidrs); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if err := json.Unmarshal(cidrs, &user.AllowedCIDRs); err != nil {
			return nil, fmt.Errorf("decode user CIDRs: %w", err)
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (r *AccessRepository) UpdateUserAccess(ctx context.Context, id, roleID string, active bool, allowedCIDRs []string) error {
	cidrs, err := json.Marshal(allowedCIDRs)
	if err != nil {
		return fmt.Errorf("marshal user CIDRs: %w", err)
	}
	command, err := r.pool.Exec(ctx, `
UPDATE users
SET role_id = NULLIF($2, '')::uuid, active = $3, allowed_cidrs = $4::jsonb
WHERE id = $1 AND is_root_admin = false
`, id, roleID, active, cidrs)
	if err != nil {
		return fmt.Errorf("update user access: %w", err)
	}
	if command.RowsAffected() == 0 {
		return portout.ErrUserNotFound
	}
	return nil
}

func (r *AccessRepository) UpdateUserCredentials(ctx context.Context, id, username, passwordHash string, allowedCIDRs []string) error {
	cidrs, err := json.Marshal(allowedCIDRs)
	if err != nil {
		return fmt.Errorf("marshal user CIDRs: %w", err)
	}
	command, err := r.pool.Exec(ctx, `UPDATE users SET username = $2, password_hash = $3, allowed_cidrs = $4::jsonb WHERE id = $1`, id, username, passwordHash, cidrs)
	if err != nil {
		return mapAccessWriteError("update user credentials", err)
	}
	if command.RowsAffected() == 0 {
		return portout.ErrUserNotFound
	}
	return nil
}

func (r *AccessRepository) CreateRole(ctx context.Context, role domain.Role) error {
	permissions := permissionStrings(role.Permissions)
	data, _ := json.Marshal(permissions)
	_, err := r.pool.Exec(ctx, `INSERT INTO roles (id, name, permissions) VALUES ($1, $2, $3::jsonb)`, role.ID, role.Name, data)
	return mapAccessWriteError("create role", err)
}

func (r *AccessRepository) GetRoleByID(ctx context.Context, id string) (*domain.Role, error) {
	var role domain.Role
	var data []byte
	err := r.pool.QueryRow(ctx, `SELECT id, name, permissions FROM roles WHERE id = $1`, id).Scan(&role.ID, &role.Name, &data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrRoleNotFound
		}
		return nil, fmt.Errorf("get role: %w", err)
	}
	role.Permissions, err = decodePermissions(data)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *AccessRepository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, permissions FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	var result []domain.Role
	for rows.Next() {
		var role domain.Role
		var data []byte
		if err := rows.Scan(&role.ID, &role.Name, &data); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		role.Permissions, err = decodePermissions(data)
		if err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, rows.Err()
}

func (r *AccessRepository) UpdateRole(ctx context.Context, role domain.Role) error {
	data, _ := json.Marshal(permissionStrings(role.Permissions))
	command, err := r.pool.Exec(ctx, `UPDATE roles SET name = $2, permissions = $3::jsonb WHERE id = $1`, role.ID, role.Name, data)
	if err != nil {
		return mapAccessWriteError("update role", err)
	}
	if command.RowsAffected() == 0 {
		return portout.ErrRoleNotFound
	}
	return nil
}

func (r *AccessRepository) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO sessions (id, user_id, refresh_token_hash, expires_at)
VALUES ($1, $2, $3, $4)
`, session.ID, session.UserID, session.RefreshTokenHash, session.ExpiresAt)
	return mapAccessWriteError("create session", err)
}

func (r *AccessRepository) GetSessionByID(ctx context.Context, id string) (*domain.Session, error) {
	return r.getSession(ctx, `SELECT id, user_id, refresh_token_hash, expires_at, revoked_at FROM sessions WHERE id = $1`, id)
}

func (r *AccessRepository) GetSessionByRefreshHash(ctx context.Context, hash []byte) (*domain.Session, error) {
	return r.getSession(ctx, `SELECT id, user_id, refresh_token_hash, expires_at, revoked_at FROM sessions WHERE refresh_token_hash = $1`, hash)
}

func (r *AccessRepository) getSession(ctx context.Context, query string, arg any) (*domain.Session, error) {
	var session domain.Session
	var revokedAt *time.Time
	err := r.pool.QueryRow(ctx, query, arg).Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &session.ExpiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	session.RevokedAt = revokedAt
	return &session, nil
}

func (r *AccessRepository) RotateSession(ctx context.Context, id string, currentHash, refreshHash []byte) error {
	command, err := r.pool.Exec(ctx, `UPDATE sessions SET refresh_token_hash = $3 WHERE id = $1 AND refresh_token_hash = $2 AND revoked_at IS NULL`, id, currentHash, refreshHash)
	if err != nil {
		return mapAccessWriteError("rotate session", err)
	}
	if command.RowsAffected() == 0 {
		return portout.ErrSessionNotFound
	}
	return nil
}

func (r *AccessRepository) RevokeSession(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE id = $1`, id)
	return err
}

func (r *AccessRepository) RevokeUserSessions(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE user_id = $1`, userID)
	return err
}

func (r *AccessRepository) CreateServiceAccount(ctx context.Context, account domain.ServiceAccount) error {
	cidrs, _ := json.Marshal(account.AllowedCIDRs)
	_, err := r.pool.Exec(ctx, `
INSERT INTO service_accounts (id, name, client_id, client_secret_hash, active, role_id, allowed_cidrs)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
`, account.ID, account.Name, account.ClientID, account.ClientSecretHash, account.Active, account.RoleID, cidrs)
	return mapAccessWriteError("create service account", err)
}

func (r *AccessRepository) GetServiceAccountByID(ctx context.Context, id string) (*domain.ServiceAccount, error) {
	return r.getServiceAccount(ctx, `SELECT id, name, client_id, client_secret_hash, active, role_id::text, allowed_cidrs FROM service_accounts WHERE id = $1`, id)
}

func (r *AccessRepository) GetServiceAccountByClientID(ctx context.Context, clientID string) (*domain.ServiceAccount, error) {
	return r.getServiceAccount(ctx, `SELECT id, name, client_id, client_secret_hash, active, role_id::text, allowed_cidrs FROM service_accounts WHERE client_id = $1`, clientID)
}

func (r *AccessRepository) getServiceAccount(ctx context.Context, query string, arg any) (*domain.ServiceAccount, error) {
	var account domain.ServiceAccount
	var cidrs []byte
	err := r.pool.QueryRow(ctx, query, arg).Scan(&account.ID, &account.Name, &account.ClientID, &account.ClientSecretHash, &account.Active, &account.RoleID, &cidrs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, portout.ErrServiceAccountNotFound
		}
		return nil, fmt.Errorf("get service account: %w", err)
	}
	if err := json.Unmarshal(cidrs, &account.AllowedCIDRs); err != nil {
		return nil, fmt.Errorf("decode service account CIDRs: %w", err)
	}
	return &account, nil
}

func (r *AccessRepository) ListServiceAccounts(ctx context.Context) ([]domain.ServiceAccount, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, client_id, client_secret_hash, active, role_id::text, allowed_cidrs FROM service_accounts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list service accounts: %w", err)
	}
	defer rows.Close()
	var result []domain.ServiceAccount
	for rows.Next() {
		var account domain.ServiceAccount
		var cidrs []byte
		if err := rows.Scan(&account.ID, &account.Name, &account.ClientID, &account.ClientSecretHash, &account.Active, &account.RoleID, &cidrs); err != nil {
			return nil, fmt.Errorf("scan service account: %w", err)
		}
		if err := json.Unmarshal(cidrs, &account.AllowedCIDRs); err != nil {
			return nil, fmt.Errorf("decode service account CIDRs: %w", err)
		}
		result = append(result, account)
	}
	return result, rows.Err()
}

func (r *AccessRepository) UpdateServiceAccount(ctx context.Context, id, roleID string, active bool, allowedCIDRs []string) error {
	cidrs, _ := json.Marshal(allowedCIDRs)
	command, err := r.pool.Exec(ctx, `
UPDATE service_accounts SET role_id = $2, active = $3, allowed_cidrs = $4::jsonb WHERE id = $1
`, id, roleID, active, cidrs)
	if err != nil {
		return fmt.Errorf("update service account: %w", err)
	}
	if command.RowsAffected() == 0 {
		return portout.ErrServiceAccountNotFound
	}
	return nil
}

func mapAccessWriteError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return portout.ErrAccessAlreadyExists
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func permissionStrings(values []domain.Permission) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result
}

func decodePermissions(data []byte) ([]domain.Permission, error) {
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("decode role permissions: %w", err)
	}
	result := make([]domain.Permission, len(values))
	for i, value := range values {
		result[i] = domain.Permission(value)
	}
	return result, nil
}

var _ portout.AccessRepository = (*AccessRepository)(nil)
