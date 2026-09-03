package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

func TestAccessRejectsUnknownPermissionAndNormalizesCIDRs(t *testing.T) {
	repo := newMemoryAccessRepository()
	access := NewAccessService(repo)
	ctx := context.Background()

	if _, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Bad", Permissions: []domain.Permission{"does.not.exist"}}); !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("expected invalid permission, got %v", err)
	}
	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Signer", Permissions: []domain.Permission{domain.PermissionSignGeneric}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := access.CreateUser(ctx, portin.CreateUserInput{Username: "alice", RoleID: role.ID, AllowedCIDRs: []string{"10.1.2.99/24", "192.0.2.5"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Password == "" {
		t.Fatal("generated password is empty")
	}
	users, err := access.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || len(users[0].AllowedCIDRs) != 2 || users[0].AllowedCIDRs[0] != "10.1.2.0/24" || users[0].AllowedCIDRs[1] != "192.0.2.5/32" {
		t.Fatalf("unexpected user CIDRs: %+v", users)
	}
	if users[0].PasswordHash != "" {
		t.Fatal("password hash leaked through list")
	}
}

func TestAccessCannotChangeRootAdminAccess(t *testing.T) {
	repo := newMemoryAccessRepository()
	auth := newTestAuth(t, repo)
	access := NewAccessService(repo)
	ctx := context.Background()

	setup, err := auth.Setup(ctx, portin.SetupInput{SetupToken: "setup-token-abcdefghijklmnopqrstuvwxyz"})
	if err != nil {
		t.Fatal(err)
	}
	root, err := repo.GetUserByUsername(ctx, setup.Username)
	if err != nil {
		t.Fatal(err)
	}
	role, err := access.CreateRole(ctx, portin.CreateRoleInput{Name: "Admin", Permissions: nil})
	if err != nil {
		t.Fatal(err)
	}

	err = access.UpdateUser(ctx, portin.UpdateUserInput{ID: root.ID, RoleID: role.ID, Active: false})
	if !errors.Is(err, ErrRootAdminImmutable) {
		t.Fatalf("expected root admin to be immutable, got %v", err)
	}
}
