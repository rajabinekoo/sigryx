package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

func newTestAuth(t *testing.T, repo *memoryAccessRepository) *AuthService {
	t.Helper()
	service, err := NewAuthService(repo, AuthConfig{
		SetupToken: "setup-token-abcdefghijklmnopqrstuvwxyz",
		JWTSecret:  []byte(strings.Repeat("j", 32)),
		Issuer:     "sigryx", Audience: "sigryx-api", AccessTTL: 10 * time.Minute, RefreshTTL: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestAuthSetupLoginAuthorizeAndRefreshRotation(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	auth := newTestAuth(t, repo)

	setup, err := auth.Setup(ctx, portin.SetupInput{SetupToken: "setup-token-abcdefghijklmnopqrstuvwxyz"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.Username == "" || setup.Password == "" {
		t.Fatal("setup credentials are empty")
	}
	if _, err := auth.Setup(ctx, portin.SetupInput{SetupToken: "setup-token-abcdefghijklmnopqrstuvwxyz"}); !errors.Is(err, ErrAlreadySetup) {
		t.Fatalf("expected already setup, got %v", err)
	}

	pair, err := auth.Login(ctx, portin.LoginInput{Username: setup.Username, Password: setup.Password, ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionVaultSeal)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.RootAdmin {
		t.Fatal("setup user is not root admin")
	}

	rotated, err := auth.Refresh(ctx, portin.RefreshInput{RefreshToken: pair.RefreshToken, ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := auth.Refresh(ctx, portin.RefreshInput{RefreshToken: pair.RefreshToken, ClientIP: "127.0.0.1"}); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("old refresh token should be invalid, got %v", err)
	}
}

func TestAuthRolePermissionAndIPAllowlist(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	access := NewAccessService(repo)
	auth := newTestAuth(t, repo)

	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Signer", Permissions: []domain.Permission{domain.PermissionSignTransaction}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := access.CreateUser(ctx, portin.CreateUserInput{Username: "alice", RoleID: role.ID, AllowedCIDRs: []string{"10.0.0.0/24"}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := auth.Login(ctx, portin.LoginInput{Username: "alice", Password: created.Password, ClientIP: "10.0.1.3"}); !errors.Is(err, ErrIPNotAllowed) {
		t.Fatalf("expected IP rejection, got %v", err)
	}
	pair, err := auth.Login(ctx, portin.LoginInput{Username: "alice", Password: created.Password, ClientIP: "10.0.0.3"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "10.0.0.3", domain.PermissionSignTransaction); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "10.0.0.3", domain.PermissionVaultSeal); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestServiceAccountTokenUsesRolePermissions(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	access := NewAccessService(repo)
	auth := newTestAuth(t, repo)

	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Verifier", Permissions: []domain.Permission{domain.PermissionVerifyGeneric}})
	if err != nil {
		t.Fatal(err)
	}
	account, err := access.CreateServiceAccount(ctx, portin.CreateServiceAccountInput{Name: "ledger", RoleID: role.ID, AllowedCIDRs: []string{"192.0.2.5/32"}})
	if err != nil {
		t.Fatal(err)
	}

	pair, err := auth.ServiceToken(ctx, portin.ServiceTokenInput{ClientID: account.ClientID, ClientSecret: account.ClientSecret, ClientIP: "192.0.2.5"})
	if err != nil {
		t.Fatal(err)
	}
	if pair.RefreshToken != "" {
		t.Fatal("service account must not receive refresh token")
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "192.0.2.5", domain.PermissionVerifyGeneric); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "192.0.2.5", domain.PermissionSignGeneric); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestAuthorizationUsesCurrentRolePermissions(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	access := NewAccessService(repo)
	auth := newTestAuth(t, repo)

	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Operator", Permissions: []domain.Permission{domain.PermissionVerifyGeneric}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := access.CreateUser(ctx, portin.CreateUserInput{Username: "bob", RoleID: role.ID})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := auth.Login(ctx, portin.LoginInput{Username: created.Username, Password: created.Password, ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionSignGeneric); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected permission denied before role update, got %v", err)
	}
	if _, err := access.UpdateRole(ctx, portin.UpdateRoleInput{ID: role.ID, Name: role.Name, Permissions: []domain.Permission{domain.PermissionSignGeneric}}); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionSignGeneric); err != nil {
		t.Fatalf("updated role was not applied immediately: %v", err)
	}
}

func TestChangingPasswordRevokesCurrentSession(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	auth := newTestAuth(t, repo)

	setup, err := auth.Setup(ctx, portin.SetupInput{SetupToken: "setup-token-abcdefghijklmnopqrstuvwxyz"})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := auth.Login(ctx, portin.LoginInput{Username: setup.Username, Password: setup.Password, ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionVaultStatusRead)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.UpdateMe(ctx, portin.UpdateMeInput{Principal: principal, CurrentPassword: setup.Password, NewPassword: "a-new-strong-password"}); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionVaultStatusRead); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected revoked access token, got %v", err)
	}
}

func TestDeactivatingUserRevokesExistingSession(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryAccessRepository()
	access := NewAccessService(repo)
	auth := newTestAuth(t, repo)

	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Reader", Permissions: []domain.Permission{domain.PermissionVaultStatusRead}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := access.CreateUser(ctx, portin.CreateUserInput{Username: "carol", RoleID: role.ID})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := auth.Login(ctx, portin.LoginInput{Username: created.Username, Password: created.Password, ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.GetUserByUsername(ctx, created.Username)
	if err != nil {
		t.Fatal(err)
	}
	if err := access.UpdateUser(ctx, portin.UpdateUserInput{ID: user.ID, RoleID: role.ID, Active: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authorize(ctx, pair.AccessToken, "127.0.0.1", domain.PermissionVaultStatusRead); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected deactivated user's session to be revoked, got %v", err)
	}
}
