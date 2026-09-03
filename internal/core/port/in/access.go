package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type CreateRoleInput struct {
	Name        string
	Permissions []domain.Permission
}

type UpdateRoleInput struct {
	ID          string
	Name        string
	Permissions []domain.Permission
}

type CreateUserInput struct {
	Username     string
	RoleID       string
	AllowedCIDRs []string
}

type CreatedUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateUserInput struct {
	ID           string
	RoleID       string
	Active       bool
	AllowedCIDRs []string
}

type CreateServiceAccountInput struct {
	Name         string
	RoleID       string
	AllowedCIDRs []string
}

type CreatedServiceAccount struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type UpdateServiceAccountInput struct {
	ID           string
	RoleID       string
	Active       bool
	AllowedCIDRs []string
}

type AccessUseCase interface {
	Permissions(ctx context.Context) []domain.PermissionDefinition
	CreateRole(ctx context.Context, input CreateRoleInput) (*domain.Role, error)
	ListRoles(ctx context.Context) ([]domain.Role, error)
	UpdateRole(ctx context.Context, input UpdateRoleInput) (*domain.Role, error)
	CreateUser(ctx context.Context, input CreateUserInput) (*CreatedUser, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	UpdateUser(ctx context.Context, input UpdateUserInput) error
	CreateServiceAccount(ctx context.Context, input CreateServiceAccountInput) (*CreatedServiceAccount, error)
	ListServiceAccounts(ctx context.Context) ([]domain.ServiceAccount, error)
	UpdateServiceAccount(ctx context.Context, input UpdateServiceAccountInput) error
}
