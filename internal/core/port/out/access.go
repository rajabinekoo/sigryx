package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrAccessAlreadyExists    = errors.New("access repository: entity already exists")
	ErrUserNotFound           = errors.New("access repository: user not found")
	ErrRoleNotFound           = errors.New("access repository: role not found")
	ErrSessionNotFound        = errors.New("access repository: session not found")
	ErrServiceAccountNotFound = errors.New("access repository: service account not found")
)

type AccessRepository interface {
	CountRootAdmins(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, user domain.User) error
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	UpdateUserAccess(ctx context.Context, id string, roleID string, active bool, allowedCIDRs []string) error
	UpdateUserCredentials(ctx context.Context, id, username, passwordHash string, allowedCIDRs []string) error

	CreateRole(ctx context.Context, role domain.Role) error
	GetRoleByID(ctx context.Context, id string) (*domain.Role, error)
	ListRoles(ctx context.Context) ([]domain.Role, error)
	UpdateRole(ctx context.Context, role domain.Role) error

	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionByID(ctx context.Context, id string) (*domain.Session, error)
	GetSessionByRefreshHash(ctx context.Context, hash []byte) (*domain.Session, error)
	RotateSession(ctx context.Context, id string, currentHash, refreshHash []byte) error
	RevokeSession(ctx context.Context, id string) error
	RevokeUserSessions(ctx context.Context, userID string) error

	CreateServiceAccount(ctx context.Context, account domain.ServiceAccount) error
	GetServiceAccountByID(ctx context.Context, id string) (*domain.ServiceAccount, error)
	GetServiceAccountByClientID(ctx context.Context, clientID string) (*domain.ServiceAccount, error)
	ListServiceAccounts(ctx context.Context) ([]domain.ServiceAccount, error)
	UpdateServiceAccount(ctx context.Context, id, roleID string, active bool, allowedCIDRs []string) error
}
