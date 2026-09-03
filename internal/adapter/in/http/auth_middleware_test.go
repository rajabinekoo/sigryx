package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type middlewareAuthStub struct {
	permission domain.Permission
	clientIP   string
	token      string
	principal  domain.Principal
}

func (s *middlewareAuthStub) Setup(context.Context, portin.SetupInput) (*portin.SetupResult, error) {
	return nil, nil
}
func (s *middlewareAuthStub) Login(context.Context, portin.LoginInput) (*portin.TokenPair, error) {
	return nil, nil
}
func (s *middlewareAuthStub) ServiceToken(context.Context, portin.ServiceTokenInput) (*portin.TokenPair, error) {
	return nil, nil
}
func (s *middlewareAuthStub) Refresh(context.Context, portin.RefreshInput) (*portin.TokenPair, error) {
	return nil, nil
}
func (s *middlewareAuthStub) Logout(context.Context, domain.Principal) error       { return nil }
func (s *middlewareAuthStub) UpdateMe(context.Context, portin.UpdateMeInput) error { return nil }
func (s *middlewareAuthStub) Authorize(_ context.Context, token, clientIP string, permission domain.Permission) (domain.Principal, error) {
	s.token = token
	s.clientIP = clientIP
	s.permission = permission
	if s.principal.ID != "" {
		return s.principal, nil
	}
	return domain.Principal{ID: "user-1", Kind: domain.PrincipalUser}, nil
}

func TestAuthMiddlewareUsesRoutePermissionAndRemoteIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &middlewareAuthStub{}
	router := gin.New()
	router.Use(authMiddleware(auth, nil))
	router.POST("/v1/vault/seal", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/vault/seal", nil)
	req.RemoteAddr = "203.0.113.9:4444"
	req.Header.Set("Authorization", "Bearer abc")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status %d", recorder.Code)
	}
	if auth.token != "abc" {
		t.Fatalf("unexpected token %q", auth.token)
	}
	if auth.clientIP != "203.0.113.9" {
		t.Fatalf("untrusted forwarded IP was accepted: %q", auth.clientIP)
	}
	if auth.permission != domain.PermissionVaultSeal {
		t.Fatalf("unexpected permission %q", auth.permission)
	}
}

func TestAuthMiddlewareTrustsForwardedIPOnlyFromConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &middlewareAuthStub{}
	router := gin.New()
	router.Use(authMiddleware(auth, []string{"10.10.0.0/16"}))
	router.POST("/v1/sign/data", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/sign/data", nil)
	req.RemoteAddr = "10.10.0.5:443"
	req.Header.Set("Authorization", "Bearer abc")
	req.Header.Set("X-Forwarded-For", "198.51.100.25")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status %d", recorder.Code)
	}
	if auth.clientIP != "198.51.100.25" {
		t.Fatalf("unexpected client IP %q", auth.clientIP)
	}
	if auth.permission != domain.PermissionSignGeneric {
		t.Fatalf("unexpected permission %q", auth.permission)
	}
}

func TestAuthMiddlewareRejectsMissingBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authMiddleware(&middlewareAuthStub{}, nil))
	router.POST("/v1/wallets", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/wallets", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareFailsClosedWhenRouteHasNoPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(authMiddleware(&middlewareAuthStub{}, nil))
	router.POST("/v1/new-sensitive-route", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/new-sensitive-route", nil)
	req.Header.Set("Authorization", "Bearer abc")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected fail-closed 403, got %d", recorder.Code)
	}
}

func TestResolveClientIPWalksTrustedProxyChainFromRight(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.10.0.5:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.200, 198.51.100.25, 10.10.0.6")

	got := resolveClientIP(req, parseTrustedPrefixes([]string{"10.10.0.0/16"}))
	if got != "198.51.100.25" {
		t.Fatalf("expected nearest untrusted client IP, got %q", got)
	}
}

func TestAuthMiddlewareAllowsRecoveryOnlyForRootAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("root admin", func(t *testing.T) {
		auth := &middlewareAuthStub{principal: domain.Principal{
			ID: "root-1", Kind: domain.PrincipalUser, RootAdmin: true,
		}}
		router := gin.New()
		router.Use(authMiddleware(auth, nil))
		router.POST("/v1/recovery/export", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		req := httptest.NewRequest(http.MethodPost, "/v1/recovery/export", nil)
		req.Header.Set("Authorization", "Bearer abc")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", recorder.Code)
		}
		if auth.permission != "" {
			t.Fatalf("recovery route must not use an assignable permission, got %q", auth.permission)
		}
	})

	t.Run("regular user", func(t *testing.T) {
		auth := &middlewareAuthStub{principal: domain.Principal{
			ID: "user-1", Kind: domain.PrincipalUser,
		}}
		router := gin.New()
		router.Use(authMiddleware(auth, nil))
		router.POST("/v1/recovery/export", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		req := httptest.NewRequest(http.MethodPost, "/v1/recovery/export", nil)
		req.Header.Set("Authorization", "Bearer abc")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", recorder.Code)
		}
	})
}
