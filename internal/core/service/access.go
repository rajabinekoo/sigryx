package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/authn"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
)

var (
	ErrInvalidUsername    = errors.New("access: invalid username")
	ErrInvalidRoleName    = errors.New("access: invalid role name")
	ErrInvalidPermission  = errors.New("access: invalid permission")
	ErrInvalidCIDR        = errors.New("access: invalid CIDR")
	ErrRootAdminImmutable = errors.New("access: root admin access cannot be changed")
	ErrInvalidServiceName = errors.New("access: invalid service account name")
	ErrInvalidAccessID    = errors.New("access: invalid identifier")
)

type AccessService struct {
	repository portout.AccessRepository
}

func NewAccessService(repository portout.AccessRepository) *AccessService {
	return &AccessService{repository: repository}
}

func (s *AccessService) Permissions(context.Context) []domain.PermissionDefinition {
	return append([]domain.PermissionDefinition(nil), domain.PermissionDefinitions...)
}

func (s *AccessService) CreateRole(ctx context.Context, input portin.CreateRoleInput) (*domain.Role, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return nil, ErrInvalidRoleName
	}
	permissions, err := normalizePermissions(input.Permissions)
	if err != nil {
		return nil, err
	}
	role := domain.Role{ID: idgen.New(), Name: name, Permissions: permissions}
	if err := s.repository.CreateRole(ctx, role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *AccessService) ListRoles(ctx context.Context) ([]domain.Role, error) {
	return s.repository.ListRoles(ctx)
}

func (s *AccessService) UpdateRole(ctx context.Context, input portin.UpdateRoleInput) (*domain.Role, error) {
	name := strings.TrimSpace(input.Name)
	if !validAccessID(input.ID) {
		return nil, ErrInvalidAccessID
	}
	if name == "" || len(name) > 80 {
		return nil, ErrInvalidRoleName
	}
	permissions, err := normalizePermissions(input.Permissions)
	if err != nil {
		return nil, err
	}
	role := domain.Role{ID: input.ID, Name: name, Permissions: permissions}
	if err := s.repository.UpdateRole(ctx, role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *AccessService) CreateUser(ctx context.Context, input portin.CreateUserInput) (*portin.CreatedUser, error) {
	username := strings.TrimSpace(input.Username)
	if !validUsername(username) {
		return nil, ErrInvalidUsername
	}
	if !validAccessID(input.RoleID) {
		return nil, ErrInvalidAccessID
	}
	if _, err := s.repository.GetRoleByID(ctx, input.RoleID); err != nil {
		return nil, err
	}
	cidrs, err := normalizeCIDRs(input.AllowedCIDRs)
	if err != nil {
		return nil, err
	}
	password, err := authn.RandomToken(24)
	if err != nil {
		return nil, err
	}
	passwordHash, err := authn.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := domain.User{
		ID:           idgen.New(),
		Username:     username,
		PasswordHash: passwordHash,
		Active:       true,
		RoleID:       input.RoleID,
		AllowedCIDRs: cidrs,
	}
	if err := s.repository.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return &portin.CreatedUser{ID: user.ID, Username: username, Password: password}, nil
}

func (s *AccessService) ListUsers(ctx context.Context) ([]domain.User, error) {
	users, err := s.repository.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].PasswordHash = ""
	}
	return users, nil
}

func (s *AccessService) UpdateUser(ctx context.Context, input portin.UpdateUserInput) error {
	if !validAccessID(input.ID) || !validAccessID(input.RoleID) {
		return ErrInvalidAccessID
	}
	user, err := s.repository.GetUserByID(ctx, input.ID)
	if err != nil {
		return err
	}
	if user.IsRootAdmin {
		return ErrRootAdminImmutable
	}
	if _, err := s.repository.GetRoleByID(ctx, input.RoleID); err != nil {
		return err
	}
	cidrs, err := normalizeCIDRs(input.AllowedCIDRs)
	if err != nil {
		return err
	}
	if err := s.repository.UpdateUserAccess(ctx, input.ID, input.RoleID, input.Active, cidrs); err != nil {
		return err
	}
	if !input.Active {
		return s.repository.RevokeUserSessions(ctx, input.ID)
	}
	return nil
}

func (s *AccessService) CreateServiceAccount(ctx context.Context, input portin.CreateServiceAccountInput) (*portin.CreatedServiceAccount, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return nil, ErrInvalidServiceName
	}
	if !validAccessID(input.RoleID) {
		return nil, ErrInvalidAccessID
	}
	if _, err := s.repository.GetRoleByID(ctx, input.RoleID); err != nil {
		return nil, err
	}
	cidrs, err := normalizeCIDRs(input.AllowedCIDRs)
	if err != nil {
		return nil, err
	}
	randomID, err := authn.RandomHex(10)
	if err != nil {
		return nil, err
	}
	secret, err := authn.RandomToken(32)
	if err != nil {
		return nil, err
	}
	hash := authn.TokenHash(secret)
	account := domain.ServiceAccount{
		ID:               idgen.New(),
		Name:             name,
		ClientID:         "svc_" + randomID,
		ClientSecretHash: hash[:],
		Active:           true,
		RoleID:           input.RoleID,
		AllowedCIDRs:     cidrs,
	}
	if err := s.repository.CreateServiceAccount(ctx, account); err != nil {
		return nil, err
	}
	return &portin.CreatedServiceAccount{
		ID: account.ID, Name: account.Name, ClientID: account.ClientID, ClientSecret: secret,
	}, nil
}

func (s *AccessService) ListServiceAccounts(ctx context.Context) ([]domain.ServiceAccount, error) {
	accounts, err := s.repository.ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		accounts[i].ClientSecretHash = nil
	}
	return accounts, nil
}

func (s *AccessService) UpdateServiceAccount(ctx context.Context, input portin.UpdateServiceAccountInput) error {
	if !validAccessID(input.ID) || !validAccessID(input.RoleID) {
		return ErrInvalidAccessID
	}
	if _, err := s.repository.GetServiceAccountByID(ctx, input.ID); err != nil {
		return err
	}
	if _, err := s.repository.GetRoleByID(ctx, input.RoleID); err != nil {
		return err
	}
	cidrs, err := normalizeCIDRs(input.AllowedCIDRs)
	if err != nil {
		return err
	}
	return s.repository.UpdateServiceAccount(ctx, input.ID, input.RoleID, input.Active, cidrs)
}

func normalizePermissions(values []domain.Permission) ([]domain.Permission, error) {
	known := make(map[domain.Permission]struct{}, len(domain.PermissionDefinitions))
	for _, item := range domain.PermissionDefinitions {
		known[item.Permission] = struct{}{}
	}
	seen := make(map[domain.Permission]struct{}, len(values))
	result := make([]domain.Permission, 0, len(values))
	for _, permission := range values {
		if _, ok := known[permission]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidPermission, permission)
		}
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func normalizeCIDRs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr, addrErr := netip.ParseAddr(value)
			if addrErr != nil {
				return nil, fmt.Errorf("%w: %s", ErrInvalidCIDR, value)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		canonical := prefix.Masked().String()
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result, nil
}

func validUsername(value string) bool {
	if len(value) < 3 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func hasPermission(values []domain.Permission, target domain.Permission) bool {
	if target == "" {
		return true
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func ipAllowed(clientIP string, allowedCIDRs []string) bool {
	if len(allowedCIDRs) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, value := range allowedCIDRs {
		prefix, err := netip.ParsePrefix(value)
		if err == nil && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func validAccessID(value string) bool {
	_, err := idgen.Parse(value)
	return err == nil
}
