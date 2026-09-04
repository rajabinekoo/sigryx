package service

import (
	"context"
	"errors"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
)

var ErrInvalidAuditPagination = errors.New("audit: invalid pagination")

type AuditService struct {
	repository portout.AuditRepository
}

func NewAuditService(repository portout.AuditRepository) *AuditService {
	return &AuditService{repository: repository}
}

func (s *AuditService) Record(ctx context.Context, event domain.AuditEvent) error {
	if event.ID == "" {
		event.ID = idgen.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	event.RetentionClass = event.EffectiveRetentionClass()
	return s.repository.Append(ctx, event)
}

func (s *AuditService) List(ctx context.Context, input portin.AuditListInput) (*portin.AuditListResult, error) {
	if input.Page < 1 || input.Limit < 1 || input.Limit > 200 {
		return nil, ErrInvalidAuditPagination
	}
	items, total, err := s.repository.List(ctx, input.Page, input.Limit)
	if err != nil {
		return nil, err
	}
	return &portin.AuditListResult{Items: items, Total: total, Page: input.Page, Limit: input.Limit}, nil
}

var _ portin.AuditUseCase = (*AuditService)(nil)
