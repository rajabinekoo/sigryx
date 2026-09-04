package out

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type AuditWriter interface {
	Append(context.Context, domain.AuditEvent) error
}

type AuditRepository interface {
	AuditWriter
	List(context.Context, int, int) ([]domain.AuditEvent, int, error)
}
