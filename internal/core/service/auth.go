package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/authn"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
)

var (
	ErrAlreadySetup        = errors.New("auth: setup already completed")
	ErrSetupDisabled       = errors.New("auth: setup is disabled")
	ErrInvalidSetupToken   = errors.New("auth: invalid setup token")
	ErrInvalidCredentials  = errors.New("auth: invalid credentials")
	ErrInactivePrincipal   = errors.New("auth: principal is inactive")
	ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")
	ErrSessionRevoked      = errors.New("auth: session revoked")
	ErrSessionExpired      = errors.New("auth: session expired")
	ErrPermissionDenied    = errors.New("auth: permission denied")
	ErrIPNotAllowed        = errors.New("auth: client IP is not allowed")
	ErrCurrentPassword     = errors.New("auth: current password is invalid")
	ErrInvalidNewPassword  = errors.New("auth: new password must be at least 12 characters")
	ErrUserPrincipalOnly   = errors.New("auth: operation requires a user principal")
	ErrRootNetworkOnly     = errors.New("auth: only the root admin can change its own IP allowlist")
)

type AuthConfig struct {
	SetupToken string
	JWTSecret  []byte
	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type AuthService struct {
	repository portout.AccessRepository
	tokens     *authn.TokenManager
	config     AuthConfig
	now        func() time.Time
}

func NewAuthService(repository portout.AccessRepository, config AuthConfig) (*AuthService, error) {
	tokens, err := authn.NewTokenManager(config.JWTSecret, config.Issuer, config.Audience)
	if err != nil {
		return nil, err
	}
	if config.SetupToken != "" && len(config.SetupToken) < 32 {
		return nil, errors.New("auth: setup token must be at least 32 characters")
	}
	if config.AccessTTL <= 0 || config.RefreshTTL <= 0 {
		return nil, errors.New("auth: token TTLs must be positive")
	}
	return &AuthService{repository: repository, tokens: tokens, config: config, now: time.Now}, nil
}

func (s *AuthService) Setup(ctx context.Context, input portin.SetupInput) (*portin.SetupResult, error) {
	if s.config.SetupToken == "" {
		return nil, ErrSetupDisabled
	}
	if subtle.ConstantTimeCompare([]byte(input.SetupToken), []byte(s.config.SetupToken)) != 1 {
		return nil, ErrInvalidSetupToken
	}
	count, err := s.repository.CountRootAdmins(ctx)
	if err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, ErrAlreadySetup
	}

	randomUsername, err := authn.RandomHex(6)
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

	username := "admin_" + randomUsername
	if err := s.repository.CreateUser(ctx, domain.User{
		ID:           idgen.New(),
		Username:     username,
		PasswordHash: passwordHash,
		IsRootAdmin:  true,
		Active:       true,
	}); err != nil {
		if errors.Is(err, portout.ErrAccessAlreadyExists) {
			return nil, ErrAlreadySetup
		}
		return nil, err
	}

	return &portin.SetupResult{Username: username, Password: password}, nil
}

func (s *AuthService) Login(ctx context.Context, input portin.LoginInput) (*portin.TokenPair, error) {
	user, err := s.repository.GetUserByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		if errors.Is(err, portout.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	valid, err := authn.VerifyPassword(user.PasswordHash, input.Password)
	if err != nil || !valid {
		return nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, ErrInactivePrincipal
	}
	if !ipAllowed(input.ClientIP, user.AllowedCIDRs) {
		return nil, ErrIPNotAllowed
	}
	return s.createUserSession(ctx, user)
}

func (s *AuthService) ServiceToken(ctx context.Context, input portin.ServiceTokenInput) (*portin.TokenPair, error) {
	account, err := s.repository.GetServiceAccountByClientID(ctx, strings.TrimSpace(input.ClientID))
	if err != nil {
		if errors.Is(err, portout.ErrServiceAccountNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !authn.SecretMatchesHash(input.ClientSecret, account.ClientSecretHash) {
		return nil, ErrInvalidCredentials
	}
	if !account.Active {
		return nil, ErrInactivePrincipal
	}
	if !ipAllowed(input.ClientIP, account.AllowedCIDRs) {
		return nil, ErrIPNotAllowed
	}
	access, _, err := s.tokens.Issue(account.ID, string(domain.PrincipalService), "", s.config.AccessTTL)
	if err != nil {
		return nil, err
	}
	return &portin.TokenPair{AccessToken: access, ExpiresIn: int64(s.config.AccessTTL.Seconds())}, nil
}

func (s *AuthService) Refresh(ctx context.Context, input portin.RefreshInput) (*portin.TokenPair, error) {
	hash := authn.TokenHash(input.RefreshToken)
	session, err := s.repository.GetSessionByRefreshHash(ctx, hash[:])
	if err != nil {
		if errors.Is(err, portout.ErrSessionNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if session.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}
	if !session.ExpiresAt.After(s.now()) {
		return nil, ErrSessionExpired
	}
	user, err := s.repository.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	if !user.Active {
		return nil, ErrInactivePrincipal
	}
	if !ipAllowed(input.ClientIP, user.AllowedCIDRs) {
		return nil, ErrIPNotAllowed
	}

	refresh, err := authn.RandomToken(32)
	if err != nil {
		return nil, err
	}
	newHash := authn.TokenHash(refresh)
	access, _, err := s.tokens.Issue(user.ID, string(domain.PrincipalUser), session.ID, s.config.AccessTTL)
	if err != nil {
		return nil, err
	}
	if err := s.repository.RotateSession(ctx, session.ID, hash[:], newHash[:]); err != nil {
		if errors.Is(err, portout.ErrSessionNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	return &portin.TokenPair{
		AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(s.config.AccessTTL.Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, principal domain.Principal) error {
	if principal.Kind != domain.PrincipalUser || principal.SessionID == "" {
		return ErrUserPrincipalOnly
	}
	return s.repository.RevokeSession(ctx, principal.SessionID)
}

func (s *AuthService) UpdateMe(ctx context.Context, input portin.UpdateMeInput) error {
	if input.Principal.Kind != domain.PrincipalUser {
		return ErrUserPrincipalOnly
	}
	user, err := s.repository.GetUserByID(ctx, input.Principal.ID)
	if err != nil {
		return err
	}
	valid, err := authn.VerifyPassword(user.PasswordHash, input.CurrentPassword)
	if err != nil || !valid {
		return ErrCurrentPassword
	}

	username := strings.TrimSpace(input.Username)
	if username == "" {
		username = user.Username
	}
	if !validUsername(username) {
		return ErrInvalidUsername
	}
	passwordHash := user.PasswordHash
	allowedCIDRs := user.AllowedCIDRs
	if input.AllowedCIDRs != nil {
		if !user.IsRootAdmin {
			return ErrRootNetworkOnly
		}
		allowedCIDRs, err = normalizeCIDRs(*input.AllowedCIDRs)
		if err != nil {
			return err
		}
	}
	if input.NewPassword != "" {
		if len(input.NewPassword) < 12 {
			return ErrInvalidNewPassword
		}
		passwordHash, err = authn.HashPassword(input.NewPassword)
		if err != nil {
			return err
		}
	}
	if err := s.repository.UpdateUserCredentials(ctx, user.ID, username, passwordHash, allowedCIDRs); err != nil {
		return err
	}
	if input.NewPassword != "" {
		return s.repository.RevokeUserSessions(ctx, user.ID)
	}
	return nil
}

func (s *AuthService) Authorize(ctx context.Context, accessToken, clientIP string, permission domain.Permission) (domain.Principal, error) {
	claims, err := s.tokens.Verify(accessToken)
	if err != nil {
		return domain.Principal{}, ErrInvalidCredentials
	}

	switch domain.PrincipalKind(claims.Kind) {
	case domain.PrincipalUser:
		return s.authorizeUser(ctx, claims.Subject, claims.SessionID, clientIP, permission)
	case domain.PrincipalService:
		return s.authorizeService(ctx, claims.Subject, clientIP, permission)
	default:
		return domain.Principal{}, ErrInvalidCredentials
	}
}

func (s *AuthService) createUserSession(ctx context.Context, user *domain.User) (*portin.TokenPair, error) {
	refresh, err := authn.RandomToken(32)
	if err != nil {
		return nil, err
	}
	hash := authn.TokenHash(refresh)
	session := domain.Session{
		ID: idgen.New(), UserID: user.ID, RefreshTokenHash: hash[:], ExpiresAt: s.now().Add(s.config.RefreshTTL),
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	access, _, err := s.tokens.Issue(user.ID, string(domain.PrincipalUser), session.ID, s.config.AccessTTL)
	if err != nil {
		_ = s.repository.RevokeSession(ctx, session.ID)
		return nil, err
	}
	return &portin.TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(s.config.AccessTTL.Seconds())}, nil
}

func (s *AuthService) authorizeUser(ctx context.Context, userID, sessionID, clientIP string, permission domain.Permission) (domain.Principal, error) {
	if sessionID == "" {
		return domain.Principal{}, ErrInvalidCredentials
	}
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil || session.UserID != userID || session.RevokedAt != nil || !session.ExpiresAt.After(s.now()) {
		return domain.Principal{}, ErrInvalidCredentials
	}
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil || !user.Active {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if !ipAllowed(clientIP, user.AllowedCIDRs) {
		return domain.Principal{}, ErrIPNotAllowed
	}
	principal := domain.Principal{ID: user.ID, Kind: domain.PrincipalUser, SessionID: session.ID, RootAdmin: user.IsRootAdmin}
	if user.IsRootAdmin || permission == "" {
		return principal, nil
	}
	role, err := s.repository.GetRoleByID(ctx, user.RoleID)
	if err != nil {
		return domain.Principal{}, ErrPermissionDenied
	}
	principal.Permissions = role.Permissions
	if !hasPermission(role.Permissions, permission) {
		return domain.Principal{}, ErrPermissionDenied
	}
	return principal, nil
}

func (s *AuthService) authorizeService(ctx context.Context, accountID, clientIP string, permission domain.Permission) (domain.Principal, error) {
	account, err := s.repository.GetServiceAccountByID(ctx, accountID)
	if err != nil || !account.Active {
		return domain.Principal{}, ErrInvalidCredentials
	}
	if !ipAllowed(clientIP, account.AllowedCIDRs) {
		return domain.Principal{}, ErrIPNotAllowed
	}
	principal := domain.Principal{ID: account.ID, Kind: domain.PrincipalService}
	if permission == "" {
		return principal, nil
	}

	role, err := s.repository.GetRoleByID(ctx, account.RoleID)
	if err != nil {
		return domain.Principal{}, ErrPermissionDenied
	}
	if !hasPermission(role.Permissions, permission) {
		return domain.Principal{}, ErrPermissionDenied
	}
	principal.Permissions = role.Permissions
	return principal, nil
}
