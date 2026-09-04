package service

import (
	"context"
	"errors"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
)

const maxAuditCleanupBatchSize = 50_000

var (
	ErrInvalidAuditRetentionDays   = errors.New("audit retention: retention days must be >= 0")
	ErrInvalidAuditCleanupBatch    = errors.New("audit retention: cleanup batch size must be between 1 and 50000")
	ErrInvalidAuditCleanupInterval = errors.New("audit retention: cleanup interval must be greater than zero")
)

type AuditRetentionConfig struct {
	NormalRetentionDays   int
	CriticalRetentionDays int
	CleanupInterval       time.Duration
	BatchSize             int
}

type AuditRetentionResult struct {
	NormalDeleted   int
	CriticalDeleted int
}

func (r AuditRetentionResult) TotalDeleted() int {
	return r.NormalDeleted + r.CriticalDeleted
}

type AuditRetentionService struct {
	repository portout.AuditRetentionRepository
	config     AuditRetentionConfig
	now        func() time.Time
}

func NewAuditRetentionService(
	repository portout.AuditRetentionRepository,
	config AuditRetentionConfig,
) (*AuditRetentionService, error) {
	if config.NormalRetentionDays < 0 || config.CriticalRetentionDays < 0 {
		return nil, ErrInvalidAuditRetentionDays
	}
	if config.CleanupInterval <= 0 {
		return nil, ErrInvalidAuditCleanupInterval
	}
	if config.BatchSize < 1 || config.BatchSize > maxAuditCleanupBatchSize {
		return nil, ErrInvalidAuditCleanupBatch
	}
	return &AuditRetentionService{
		repository: repository,
		config:     config,
		now:        func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *AuditRetentionService) Interval() time.Duration {
	return s.config.CleanupInterval
}

func (s *AuditRetentionService) Enabled() bool {
	return s.config.NormalRetentionDays > 0 || s.config.CriticalRetentionDays > 0
}

func (s *AuditRetentionService) Cleanup(ctx context.Context) (AuditRetentionResult, error) {
	var result AuditRetentionResult
	if !s.Enabled() {
		return result, nil
	}

	now := s.now().UTC()
	if s.config.NormalRetentionDays > 0 {
		deleted, err := s.purgeClass(
			ctx,
			domain.AuditRetentionNormal,
			now.AddDate(0, 0, -s.config.NormalRetentionDays),
		)
		if err != nil {
			return result, err
		}
		result.NormalDeleted = deleted
	}

	if s.config.CriticalRetentionDays > 0 {
		deleted, err := s.purgeClass(
			ctx,
			domain.AuditRetentionCritical,
			now.AddDate(0, 0, -s.config.CriticalRetentionDays),
		)
		if err != nil {
			return result, err
		}
		result.CriticalDeleted = deleted
	}

	return result, nil
}

func (s *AuditRetentionService) purgeClass(
	ctx context.Context,
	retentionClass domain.AuditRetentionClass,
	before time.Time,
) (int, error) {
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		deleted, err := s.repository.PurgeExpired(ctx, retentionClass, before, s.config.BatchSize)
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < s.config.BatchSize {
			return total, nil
		}
	}
}
