package out

import (
	"context"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type AuditWriter interface {
	Append(context.Context, domain.AuditEvent) error
}

type AuditRepository interface {
	AuditWriter
	List(context.Context, int, int) ([]domain.AuditEvent, int, error)
}

// AuditRetentionRepository exposes the only sanctioned deletion path for
// expired audit events. Ordinary audit storage remains append-only.
type AuditRetentionRepository interface {
	PurgeExpired(context.Context, domain.AuditRetentionClass, time.Time, int) (int, error)
}
