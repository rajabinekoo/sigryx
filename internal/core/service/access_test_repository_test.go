package service

import (
	"context"
	"sync"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
)

type memoryAccessRepository struct {
	mu       sync.Mutex
	users    map[string]domain.User
	roles    map[string]domain.Role
	sessions map[string]domain.Session
	services map[string]domain.ServiceAccount
}

func newMemoryAccessRepository() *memoryAccessRepository {
	return &memoryAccessRepository{
		users: make(map[string]domain.User), roles: make(map[string]domain.Role),
		sessions: make(map[string]domain.Session), services: make(map[string]domain.ServiceAccount),
	}
}

func (r *memoryAccessRepository) CountRootAdmins(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, user := range r.users {
		if user.IsRootAdmin {
			count++
		}
	}
	return count, nil
}
func (r *memoryAccessRepository) CreateUser(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Username == user.Username || (existing.IsRootAdmin && user.IsRootAdmin) {
			return portout.ErrAccessAlreadyExists
		}
	}
	r.users[user.ID] = cloneUser(user)
	return nil
}
func (r *memoryAccessRepository) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return nil, portout.ErrUserNotFound
	}
	copy := cloneUser(user)
	return &copy, nil
}
func (r *memoryAccessRepository) GetUserByUsername(_ context.Context, username string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range r.users {
		if user.Username == username {
			copy := cloneUser(user)
			return &copy, nil
		}
	}
	return nil, portout.ErrUserNotFound
}
func (r *memoryAccessRepository) ListUsers(context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		out = append(out, cloneUser(user))
	}
	return out, nil
}
func (r *memoryAccessRepository) UpdateUserAccess(_ context.Context, id, roleID string, active bool, cidrs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok || user.IsRootAdmin {
		return portout.ErrUserNotFound
	}
	user.RoleID = roleID
	user.Active = active
	user.AllowedCIDRs = append([]string(nil), cidrs...)
	r.users[id] = user
	return nil
}
func (r *memoryAccessRepository) UpdateUserCredentials(_ context.Context, id, username, passwordHash string, allowedCIDRs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return portout.ErrUserNotFound
	}
	user.Username = username
	user.PasswordHash = passwordHash
	user.AllowedCIDRs = append([]string(nil), allowedCIDRs...)
	r.users[id] = user
	return nil
}
func (r *memoryAccessRepository) CreateRole(_ context.Context, role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.roles {
		if item.Name == role.Name {
			return portout.ErrAccessAlreadyExists
		}
	}
	r.roles[role.ID] = cloneRole(role)
	return nil
}
func (r *memoryAccessRepository) GetRoleByID(_ context.Context, id string) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	role, ok := r.roles[id]
	if !ok {
		return nil, portout.ErrRoleNotFound
	}
	copy := cloneRole(role)
	return &copy, nil
}
func (r *memoryAccessRepository) ListRoles(context.Context) ([]domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Role, 0, len(r.roles))
	for _, role := range r.roles {
		out = append(out, cloneRole(role))
	}
	return out, nil
}
func (r *memoryAccessRepository) UpdateRole(_ context.Context, role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.roles[role.ID]; !ok {
		return portout.ErrRoleNotFound
	}
	r.roles[role.ID] = cloneRole(role)
	return nil
}
func (r *memoryAccessRepository) CreateSession(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = cloneSession(session)
	return nil
}
func (r *memoryAccessRepository) GetSessionByID(_ context.Context, id string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok {
		return nil, portout.ErrSessionNotFound
	}
	copy := cloneSession(session)
	return &copy, nil
}
func (r *memoryAccessRepository) GetSessionByRefreshHash(_ context.Context, hash []byte) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		if string(session.RefreshTokenHash) == string(hash) {
			copy := cloneSession(session)
			return &copy, nil
		}
	}
	return nil, portout.ErrSessionNotFound
}
func (r *memoryAccessRepository) RotateSession(_ context.Context, id string, currentHash, hash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok || session.RevokedAt != nil || string(session.RefreshTokenHash) != string(currentHash) {
		return portout.ErrSessionNotFound
	}
	session.RefreshTokenHash = append([]byte(nil), hash...)
	r.sessions[id] = session
	return nil
}
func (r *memoryAccessRepository) RevokeSession(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok {
		return portout.ErrSessionNotFound
	}
	now := time.Now()
	session.RevokedAt = &now
	r.sessions[id] = session
	return nil
}
func (r *memoryAccessRepository) RevokeUserSessions(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, session := range r.sessions {
		if session.UserID == userID {
			session.RevokedAt = &now
			r.sessions[id] = session
		}
	}
	return nil
}
func (r *memoryAccessRepository) CreateServiceAccount(_ context.Context, account domain.ServiceAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.services {
		if existing.Name == account.Name || existing.ClientID == account.ClientID {
			return portout.ErrAccessAlreadyExists
		}
	}
	r.services[account.ID] = cloneService(account)
	return nil
}
func (r *memoryAccessRepository) GetServiceAccountByID(_ context.Context, id string) (*domain.ServiceAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.services[id]
	if !ok {
		return nil, portout.ErrServiceAccountNotFound
	}
	copy := cloneService(account)
	return &copy, nil
}
func (r *memoryAccessRepository) GetServiceAccountByClientID(_ context.Context, clientID string) (*domain.ServiceAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, account := range r.services {
		if account.ClientID == clientID {
			copy := cloneService(account)
			return &copy, nil
		}
	}
	return nil, portout.ErrServiceAccountNotFound
}
func (r *memoryAccessRepository) ListServiceAccounts(context.Context) ([]domain.ServiceAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.ServiceAccount, 0, len(r.services))
	for _, account := range r.services {
		out = append(out, cloneService(account))
	}
	return out, nil
}
func (r *memoryAccessRepository) UpdateServiceAccount(_ context.Context, id, roleID string, active bool, cidrs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.services[id]
	if !ok {
		return portout.ErrServiceAccountNotFound
	}
	account.RoleID = roleID
	account.Active = active
	account.AllowedCIDRs = append([]string(nil), cidrs...)
	r.services[id] = account
	return nil
}

func cloneUser(value domain.User) domain.User {
	value.AllowedCIDRs = append([]string(nil), value.AllowedCIDRs...)
	return value
}
func cloneRole(value domain.Role) domain.Role {
	value.Permissions = append([]domain.Permission(nil), value.Permissions...)
	return value
}
func cloneSession(value domain.Session) domain.Session {
	value.RefreshTokenHash = append([]byte(nil), value.RefreshTokenHash...)
	return value
}
func cloneService(value domain.ServiceAccount) domain.ServiceAccount {
	value.ClientSecretHash = append([]byte(nil), value.ClientSecretHash...)
	value.AllowedCIDRs = append([]string(nil), value.AllowedCIDRs...)
	return value
}
