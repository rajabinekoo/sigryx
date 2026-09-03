package http

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	"github.com/rajabinekoo/sigryx/internal/core/service"
)

type authContextKey string

const (
	principalContextKey authContextKey = "sigryx-principal"
	clientIPContextKey  authContextKey = "sigryx-client-ip"
)

func authMiddleware(auth portin.AuthUseCase, trustedProxyCIDRs []string) gin.HandlerFunc {
	trusted := parseTrustedPrefixes(trustedProxyCIDRs)

	return func(c *gin.Context) {
		clientIP := resolveClientIP(c.Request, trusted)
		ctx := context.WithValue(c.Request.Context(), clientIPContextKey, clientIP)
		c.Request = c.Request.WithContext(ctx)

		if isPublicRoute(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		permission, configured := permissionFor(c.Request.Method, path)
		authenticatedOnly := isAuthenticatedOnlyRoute(c.Request.Method, path)
		rootOnly := isRootOnlyRoute(c.Request.Method, path)
		if !configured && !authenticatedOnly && !rootOnly {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "authorization policy is not configured for this route"})
			return
		}

		principal, err := auth.Authorize(c.Request.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), clientIP, permission)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, service.ErrPermissionDenied) || errors.Is(err, service.ErrIPNotAllowed) {
				status = http.StatusForbidden
			}
			c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}

		if rootOnly && !principal.RootAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "root admin required"})
			return
		}

		ctx = context.WithValue(c.Request.Context(), principalContextKey, principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func principalFromContext(ctx context.Context) (domain.Principal, bool) {
	value, ok := ctx.Value(principalContextKey).(domain.Principal)
	return value, ok
}

func clientIPFromContext(ctx context.Context) string {
	value, _ := ctx.Value(clientIPContextKey).(string)
	return value
}

func isPublicRoute(method, path string) bool {
	if method == http.MethodGet && (path == "/v1/health" || path == "/docs" || path == "/openapi.json") {
		return true
	}
	if method == http.MethodPost {
		switch path {
		case "/v1/setup", "/v1/auth/login", "/v1/auth/refresh", "/v1/auth/service-token":
			return true
		}
	}
	return false
}

func permissionFor(method, path string) (domain.Permission, bool) {
	permission, ok := routePermissions[method+" "+path]
	return permission, ok
}

func isAuthenticatedOnlyRoute(method, path string) bool {
	if method == http.MethodPost && path == "/v1/auth/logout" {
		return true
	}
	return method == http.MethodPatch && path == "/v1/auth/me"
}

func isRootOnlyRoute(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	return path == "/v1/recovery/export" || path == "/v1/recovery/import"
}

var routePermissions = map[string]domain.Permission{
	"GET /v1/vault/status":                  domain.PermissionVaultStatusRead,
	"POST /v1/vault/init":                   domain.PermissionVaultInitialize,
	"POST /v1/vault/unseal":                 domain.PermissionVaultUnseal,
	"POST /v1/vault/seal":                   domain.PermissionVaultSeal,
	"GET /v1/key-roots":                     domain.PermissionKeyRootRead,
	"POST /v1/key-roots":                    domain.PermissionKeyRootCreate,
	"POST /v1/wallets":                      domain.PermissionWalletCreate,
	"POST /v1/sign/transaction":             domain.PermissionSignTransaction,
	"POST /v1/sign/typed-data":              domain.PermissionSignTypedData,
	"POST /v1/sign/data":                    domain.PermissionSignGeneric,
	"POST /v1/verify/transaction":           domain.PermissionVerifyTransaction,
	"POST /v1/verify/typed-data":            domain.PermissionVerifyTypedData,
	"POST /v1/verify/data":                  domain.PermissionVerifyGeneric,
	"GET /v1/access/permissions":            domain.PermissionAccessRolesManage,
	"GET /v1/access/roles":                  domain.PermissionAccessRolesManage,
	"POST /v1/access/roles":                 domain.PermissionAccessRolesManage,
	"PATCH /v1/access/roles/:id":            domain.PermissionAccessRolesManage,
	"GET /v1/access/users":                  domain.PermissionAccessUsersManage,
	"POST /v1/access/users":                 domain.PermissionAccessUsersManage,
	"PATCH /v1/access/users/:id":            domain.PermissionAccessUsersManage,
	"GET /v1/access/service-accounts":       domain.PermissionAccessServiceManage,
	"POST /v1/access/service-accounts":      domain.PermissionAccessServiceManage,
	"PATCH /v1/access/service-accounts/:id": domain.PermissionAccessServiceManage,
}

func parseTrustedPrefixes(values []string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		if prefix, err := netip.ParsePrefix(strings.TrimSpace(value)); err == nil {
			result = append(result, prefix)
		}
	}
	return result
}

func resolveClientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return strings.TrimSpace(host)
	}
	peer = peer.Unmap()
	if !addressInPrefixes(peer, trusted) {
		return peer.String()
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				continue
			}
			addr = addr.Unmap()
			if !addressInPrefixes(addr, trusted) {
				return addr.String()
			}
		}
	}
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		if addr, err := netip.ParseAddr(value); err == nil {
			return addr.Unmap().String()
		}
	}
	return peer.String()
}

func addressInPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
