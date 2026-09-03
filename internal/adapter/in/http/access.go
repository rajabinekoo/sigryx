package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type permissionResponse struct {
	Permission domain.Permission `json:"permission"`
	Category   string            `json:"category"`
	Label      string            `json:"label"`
}

type permissionsOutput struct {
	Body struct {
		Permissions []permissionResponse `json:"permissions"`
	}
}

type roleBody struct {
	Name        string              `json:"name"`
	Permissions []domain.Permission `json:"permissions"`
}

type roleResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Permissions []domain.Permission `json:"permissions"`
}

type createRoleInput struct{ Body roleBody }
type updateRoleInput struct {
	ID   string `path:"id"`
	Body roleBody
}
type roleOutput struct{ Body roleResponse }
type rolesOutput struct {
	Body struct {
		Roles []roleResponse `json:"roles"`
	}
}

type createUserInput struct {
	Body struct {
		Username     string   `json:"username"`
		RoleID       string   `json:"role_id"`
		AllowedCIDRs []string `json:"allowed_cidrs,omitempty" doc:"Optional per-user IP/CIDR allowlist. Empty allows any IP."`
	}
}

type createdUserOutput struct{ Body portin.CreatedUser }

type updateUserInput struct {
	ID   string `path:"id"`
	Body struct {
		RoleID       string   `json:"role_id"`
		Active       bool     `json:"active"`
		AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`
	}
}

type userResponse struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	IsRootAdmin  bool     `json:"is_root_admin"`
	Active       bool     `json:"active"`
	RoleID       string   `json:"role_id,omitempty"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
}

type usersOutput struct {
	Body struct {
		Users []userResponse `json:"users"`
	}
}

type createServiceAccountInput struct {
	Body struct {
		Name         string   `json:"name"`
		RoleID       string   `json:"role_id"`
		AllowedCIDRs []string `json:"allowed_cidrs,omitempty" doc:"Recommended for machine identities. Empty allows any IP."`
	}
}

type createdServiceAccountOutput struct{ Body portin.CreatedServiceAccount }

type updateServiceAccountInput struct {
	ID   string `path:"id"`
	Body struct {
		RoleID       string   `json:"role_id"`
		Active       bool     `json:"active"`
		AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`
	}
}

type serviceAccountResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ClientID     string   `json:"client_id"`
	Active       bool     `json:"active"`
	RoleID       string   `json:"role_id"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
}

type serviceAccountsOutput struct {
	Body struct {
		ServiceAccounts []serviceAccountResponse `json:"service_accounts"`
	}
}

func registerAccessRoutes(api huma.API, access portin.AccessUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "list_permissions", Method: http.MethodGet, Path: "/v1/access/permissions",
		Summary: "List available permissions", Tags: []string{"access"},
	}, func(ctx context.Context, _ *emptyInput) (*permissionsOutput, error) {
		definitions := access.Permissions(ctx)
		out := &permissionsOutput{}
		out.Body.Permissions = make([]permissionResponse, len(definitions))
		for i, item := range definitions {
			out.Body.Permissions[i] = permissionResponse{Permission: item.Permission, Category: item.Category, Label: item.Label}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create_role", Method: http.MethodPost, Path: "/v1/access/roles",
		Summary: "Create role", Tags: []string{"access"},
	}, func(ctx context.Context, in *createRoleInput) (*roleOutput, error) {
		role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: in.Body.Name, Permissions: in.Body.Permissions})
		if err != nil {
			return nil, translate(err)
		}
		return &roleOutput{Body: roleHTTP(*role)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list_roles", Method: http.MethodGet, Path: "/v1/access/roles",
		Summary: "List roles", Tags: []string{"access"},
	}, func(ctx context.Context, _ *emptyInput) (*rolesOutput, error) {
		roles, err := access.ListRoles(ctx)
		if err != nil {
			return nil, translate(err)
		}
		out := &rolesOutput{}
		out.Body.Roles = make([]roleResponse, len(roles))
		for i, role := range roles {
			out.Body.Roles[i] = roleHTTP(role)
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update_role", Method: http.MethodPatch, Path: "/v1/access/roles/{id}",
		Summary: "Update role", Tags: []string{"access"},
	}, func(ctx context.Context, in *updateRoleInput) (*roleOutput, error) {
		role, err := access.UpdateRole(ctx, portin.UpdateRoleInput{ID: in.ID, Name: in.Body.Name, Permissions: in.Body.Permissions})
		if err != nil {
			return nil, translate(err)
		}
		return &roleOutput{Body: roleHTTP(*role)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create_user", Method: http.MethodPost, Path: "/v1/access/users",
		Summary: "Create user", Description: "Returns the generated initial password once. Sigryx stores only its Argon2id hash.", Tags: []string{"access"},
	}, func(ctx context.Context, in *createUserInput) (*createdUserOutput, error) {
		result, err := access.CreateUser(ctx, portin.CreateUserInput{Username: in.Body.Username, RoleID: in.Body.RoleID, AllowedCIDRs: in.Body.AllowedCIDRs})
		if err != nil {
			return nil, translate(err)
		}
		return &createdUserOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list_users", Method: http.MethodGet, Path: "/v1/access/users",
		Summary: "List users", Tags: []string{"access"},
	}, func(ctx context.Context, _ *emptyInput) (*usersOutput, error) {
		users, err := access.ListUsers(ctx)
		if err != nil {
			return nil, translate(err)
		}
		out := &usersOutput{}
		out.Body.Users = make([]userResponse, len(users))
		for i, user := range users {
			out.Body.Users[i] = userResponse{ID: user.ID, Username: user.Username, IsRootAdmin: user.IsRootAdmin, Active: user.Active, RoleID: user.RoleID, AllowedCIDRs: user.AllowedCIDRs}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update_user", Method: http.MethodPatch, Path: "/v1/access/users/{id}",
		Summary: "Update user access", Tags: []string{"access"},
	}, func(ctx context.Context, in *updateUserInput) (*messageOutput, error) {
		if err := access.UpdateUser(ctx, portin.UpdateUserInput{ID: in.ID, RoleID: in.Body.RoleID, Active: in.Body.Active, AllowedCIDRs: in.Body.AllowedCIDRs}); err != nil {
			return nil, translate(err)
		}
		out := &messageOutput{}
		out.Body.Message = "user updated"
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create_service_account", Method: http.MethodPost, Path: "/v1/access/service-accounts",
		Summary: "Create service account", Description: "Returns client_secret once. Only its SHA-256 hash is stored.", Tags: []string{"access"},
	}, func(ctx context.Context, in *createServiceAccountInput) (*createdServiceAccountOutput, error) {
		result, err := access.CreateServiceAccount(ctx, portin.CreateServiceAccountInput{Name: in.Body.Name, RoleID: in.Body.RoleID, AllowedCIDRs: in.Body.AllowedCIDRs})
		if err != nil {
			return nil, translate(err)
		}
		return &createdServiceAccountOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list_service_accounts", Method: http.MethodGet, Path: "/v1/access/service-accounts",
		Summary: "List service accounts", Tags: []string{"access"},
	}, func(ctx context.Context, _ *emptyInput) (*serviceAccountsOutput, error) {
		accounts, err := access.ListServiceAccounts(ctx)
		if err != nil {
			return nil, translate(err)
		}
		out := &serviceAccountsOutput{}
		out.Body.ServiceAccounts = make([]serviceAccountResponse, len(accounts))
		for i, account := range accounts {
			out.Body.ServiceAccounts[i] = serviceAccountResponse{ID: account.ID, Name: account.Name, ClientID: account.ClientID, Active: account.Active, RoleID: account.RoleID, AllowedCIDRs: account.AllowedCIDRs}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update_service_account", Method: http.MethodPatch, Path: "/v1/access/service-accounts/{id}",
		Summary: "Update service account access", Tags: []string{"access"},
	}, func(ctx context.Context, in *updateServiceAccountInput) (*messageOutput, error) {
		if err := access.UpdateServiceAccount(ctx, portin.UpdateServiceAccountInput{ID: in.ID, RoleID: in.Body.RoleID, Active: in.Body.Active, AllowedCIDRs: in.Body.AllowedCIDRs}); err != nil {
			return nil, translate(err)
		}
		out := &messageOutput{}
		out.Body.Message = "service account updated"
		return out, nil
	})
}

func roleHTTP(role domain.Role) roleResponse {
	return roleResponse{ID: role.ID, Name: role.Name, Permissions: role.Permissions}
}
