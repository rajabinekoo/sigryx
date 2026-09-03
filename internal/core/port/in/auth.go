package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type SetupInput struct {
	SetupToken string
}

type SetupResult struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginInput struct {
	Username string
	Password string
	ClientIP string
}

type ServiceTokenInput struct {
	ClientID     string
	ClientSecret string
	ClientIP     string
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RefreshInput struct {
	RefreshToken string
	ClientIP     string
}

type UpdateMeInput struct {
	Principal       domain.Principal
	CurrentPassword string
	Username        string
	NewPassword     string
	AllowedCIDRs    *[]string
}

type AuthUseCase interface {
	Setup(ctx context.Context, input SetupInput) (*SetupResult, error)
	Login(ctx context.Context, input LoginInput) (*TokenPair, error)
	ServiceToken(ctx context.Context, input ServiceTokenInput) (*TokenPair, error)
	Refresh(ctx context.Context, input RefreshInput) (*TokenPair, error)
	Logout(ctx context.Context, principal domain.Principal) error
	UpdateMe(ctx context.Context, input UpdateMeInput) error
	Authorize(ctx context.Context, accessToken, clientIP string, permission domain.Permission) (domain.Principal, error)
}
