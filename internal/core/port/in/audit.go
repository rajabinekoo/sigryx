package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type AuditListInput struct {
	Page  int
	Limit int
}

type AuditListResult struct {
	Items []domain.AuditEvent
	Total int
	Page  int
	Limit int
}

type AuditUseCase interface {
	Record(context.Context, domain.AuditEvent) error
	List(context.Context, AuditListInput) (*AuditListResult, error)
}
