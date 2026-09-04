package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	"github.com/rajabinekoo/sigryx/internal/core/requestmeta"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
)

func requestMetadataMiddleware(trustedProxyCIDRs []string) gin.HandlerFunc {
	trusted := parseTrustedPrefixes(trustedProxyCIDRs)
	return func(c *gin.Context) {
		requestID := idgen.New()
		clientIP := resolveClientIP(c.Request, trusted)
		metadata := requestmeta.Metadata{RequestID: requestID, SourceIP: clientIP}
		ctx := requestmeta.With(c.Request.Context(), metadata)
		ctx = context.WithValue(ctx, clientIPContextKey, clientIP)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func auditMiddleware(audit portin.AuditUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if shouldSkipAudit(c.Request.Method, c.Request.URL.Path) {
			return
		}

		metadata := requestmeta.From(c.Request.Context())
		routePath := c.FullPath()
		if routePath == "" {
			routePath = c.Request.URL.Path
		}
		status := c.Writer.Status()
		outcome := domain.AuditOutcomeSuccess
		switch {
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			outcome = domain.AuditOutcomeDenied
		case status >= 400:
			outcome = domain.AuditOutcomeFailed
		}
		actorType := string(metadata.Principal.Kind)
		if actorType == "" {
			actorType = "ANONYMOUS"
		}
		event := domain.AuditEvent{
			OccurredAt: time.Now().UTC(), ActorType: actorType,
			ActorID: metadata.Principal.ID, SessionID: metadata.Principal.SessionID,
			Action: auditAction(c.Request.Method, routePath), Outcome: outcome,
			SourceIP: metadata.SourceIP, RequestID: metadata.RequestID,
			Method: c.Request.Method, Path: c.Request.URL.Path, StatusCode: status,
		}
		if err := audit.Record(context.WithoutCancel(c.Request.Context()), event); err != nil {
			slog.Error("append audit event", slog.Any("err", err), slog.String("request_id", metadata.RequestID))
		}
	}
}

func shouldSkipAudit(method, path string) bool {
	return method == http.MethodGet && (path == "/v1/health" || path == "/docs" || path == "/openapi.json")
}

func auditAction(method, path string) string {
	if action, ok := routeAuditActions[method+" "+path]; ok {
		return action
	}
	return "http.request"
}

var routeAuditActions = map[string]string{
	"POST /v1/setup":                        "auth.setup",
	"POST /v1/auth/login":                   "auth.login",
	"POST /v1/auth/refresh":                 "auth.refresh",
	"POST /v1/auth/service-token":           "auth.service_token",
	"POST /v1/auth/logout":                  "auth.logout",
	"PATCH /v1/auth/me":                     "auth.update_me",
	"GET /v1/vault/status":                  "vault.status",
	"POST /v1/vault/init":                   "vault.initialize",
	"POST /v1/vault/unseal":                 "vault.unseal",
	"POST /v1/vault/seal":                   "vault.seal",
	"GET /v1/key-roots":                     "keyroot.list",
	"POST /v1/key-roots":                    "keyroot.create",
	"POST /v1/wallets":                      "wallet.create",
	"POST /v1/sign/transaction":             "sign.transaction",
	"POST /v1/verify/transaction":           "verify.transaction",
	"POST /v1/sign/typed-data":              "sign.typed_data",
	"POST /v1/verify/typed-data":            "verify.typed_data",
	"POST /v1/sign/data":                    "sign.generic",
	"POST /v1/verify/data":                  "verify.generic",
	"POST /v1/sign/integrity":               "sign.integrity",
	"POST /v1/verify/integrity":             "verify.integrity",
	"GET /v1/audit/events":                  "audit.list",
	"POST /v1/recovery/export":              "recovery.export",
	"POST /v1/recovery/import":              "recovery.import",
	"GET /v1/access/permissions":            "access.permissions.list",
	"GET /v1/access/roles":                  "access.roles.list",
	"POST /v1/access/roles":                 "access.roles.create",
	"PATCH /v1/access/roles/:id":            "access.roles.update",
	"GET /v1/access/users":                  "access.users.list",
	"POST /v1/access/users":                 "access.users.create",
	"PATCH /v1/access/users/:id":            "access.users.update",
	"GET /v1/access/service-accounts":       "access.service_accounts.list",
	"POST /v1/access/service-accounts":      "access.service_accounts.create",
	"PATCH /v1/access/service-accounts/:id": "access.service_accounts.update",
}
