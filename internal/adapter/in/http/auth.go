package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type setupInput struct {
	SetupToken string `header:"X-Sigryx-Setup-Token" required:"true" doc:"One-time bootstrap token configured through SETUP_TOKEN."`
}

type setupOutput struct {
	Body portin.SetupResult
}

type loginInput struct {
	Body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
}

type serviceTokenInput struct {
	Body struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
}

type refreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token"`
	}
}

type tokenOutput struct {
	Body portin.TokenPair
}

type updateMeInput struct {
	Body struct {
		CurrentPassword string    `json:"current_password"`
		Username        string    `json:"username,omitempty" doc:"New username. Empty keeps the current username."`
		NewPassword     string    `json:"new_password,omitempty" doc:"New password, minimum 12 characters. Changing it revokes all sessions."`
		AllowedCIDRs    *[]string `json:"allowed_cidrs,omitempty" doc:"Root admin only. Replaces the root admin IP/CIDR allowlist when provided."`
	}
}

type messageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func registerAuthRoutes(api huma.API, auth portin.AuthUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "setup", Method: http.MethodPost, Path: "/v1/setup",
		Summary: "Bootstrap the single root admin", Tags: []string{"auth"},
	}, func(ctx context.Context, in *setupInput) (*setupOutput, error) {
		result, err := auth.Setup(ctx, portin.SetupInput{SetupToken: in.SetupToken})
		if err != nil {
			return nil, translate(err)
		}
		return &setupOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login", Method: http.MethodPost, Path: "/v1/auth/login",
		Summary: "Login with username and password", Tags: []string{"auth"},
	}, func(ctx context.Context, in *loginInput) (*tokenOutput, error) {
		result, err := auth.Login(ctx, portin.LoginInput{
			Username: in.Body.Username, Password: in.Body.Password, ClientIP: clientIPFromContext(ctx),
		})
		if err != nil {
			return nil, translate(err)
		}
		return &tokenOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "service_token", Method: http.MethodPost, Path: "/v1/auth/service-token",
		Summary: "Issue a short-lived token for a service account", Tags: []string{"auth"},
	}, func(ctx context.Context, in *serviceTokenInput) (*tokenOutput, error) {
		result, err := auth.ServiceToken(ctx, portin.ServiceTokenInput{
			ClientID: in.Body.ClientID, ClientSecret: in.Body.ClientSecret, ClientIP: clientIPFromContext(ctx),
		})
		if err != nil {
			return nil, translate(err)
		}
		return &tokenOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "refresh_token", Method: http.MethodPost, Path: "/v1/auth/refresh",
		Summary: "Rotate a refresh token and issue a new access token", Tags: []string{"auth"},
	}, func(ctx context.Context, in *refreshInput) (*tokenOutput, error) {
		result, err := auth.Refresh(ctx, portin.RefreshInput{RefreshToken: in.Body.RefreshToken, ClientIP: clientIPFromContext(ctx)})
		if err != nil {
			return nil, translate(err)
		}
		return &tokenOutput{Body: *result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "logout", Method: http.MethodPost, Path: "/v1/auth/logout",
		Summary: "Revoke the current user session", Tags: []string{"auth"},
	}, func(ctx context.Context, _ *emptyInput) (*messageOutput, error) {
		principal, ok := principalFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := auth.Logout(ctx, principal); err != nil {
			return nil, translate(err)
		}
		out := &messageOutput{}
		out.Body.Message = "logged out"
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update_me", Method: http.MethodPatch, Path: "/v1/auth/me",
		Summary: "Change the current user's username or password", Tags: []string{"auth"},
	}, func(ctx context.Context, in *updateMeInput) (*messageOutput, error) {
		principal, ok := principalFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		if err := auth.UpdateMe(ctx, portin.UpdateMeInput{
			Principal: principal, CurrentPassword: in.Body.CurrentPassword, Username: in.Body.Username, NewPassword: in.Body.NewPassword, AllowedCIDRs: in.Body.AllowedCIDRs,
		}); err != nil {
			return nil, translate(err)
		}
		out := &messageOutput{}
		out.Body.Message = "credentials updated"
		return out, nil
	})
}
