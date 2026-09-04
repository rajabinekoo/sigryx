package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

func TestAuditRetentionServiceCleanupUsesIndependentCutoffsAndBatches(t *testing.T) {
	now := time.Date(2026, time.September, 4, 8, 30, 0, 0, time.UTC)
	repository := &memoryAuditRetentionRepository{
		responses: map[domain.AuditRetentionClass][]int{
			domain.AuditRetentionNormal:   {2, 1},
			domain.AuditRetentionCritical: {2, 0},
		},
	}
	service, err := NewAuditRetentionService(repository, AuditRetentionConfig{
		NormalRetentionDays:   30,
		CriticalRetentionDays: 365,
		CleanupInterval:       6 * time.Hour,
		BatchSize:             2,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }

	result, err := service.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.NormalDeleted != 3 || result.CriticalDeleted != 2 || result.TotalDeleted() != 5 {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	if len(repository.calls) != 4 {
		t.Fatalf("purge calls = %d, want 4", len(repository.calls))
	}

	normalCutoff := now.AddDate(0, 0, -30)
	criticalCutoff := now.AddDate(0, 0, -365)
	if repository.calls[0].retentionClass != domain.AuditRetentionNormal || !repository.calls[0].before.Equal(normalCutoff) {
		t.Fatalf("unexpected normal purge call: %+v", repository.calls[0])
	}
	if repository.calls[2].retentionClass != domain.AuditRetentionCritical || !repository.calls[2].before.Equal(criticalCutoff) {
		t.Fatalf("unexpected critical purge call: %+v", repository.calls[2])
	}
	for _, call := range repository.calls {
		if call.limit != 2 {
			t.Fatalf("batch limit = %d, want 2", call.limit)
		}
	}
}

func TestAuditRetentionServiceZeroDaysMeansRetainForever(t *testing.T) {
	repository := &memoryAuditRetentionRepository{}
	service, err := NewAuditRetentionService(repository, AuditRetentionConfig{
		NormalRetentionDays:   0,
		CriticalRetentionDays: 365,
		CleanupInterval:       time.Hour,
		BatchSize:             100,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time {
		return time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC)
	}

	if _, err := service.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repository.calls) != 1 || repository.calls[0].retentionClass != domain.AuditRetentionCritical {
		t.Fatalf("unexpected purge calls: %+v", repository.calls)
	}
}

func TestAuditRetentionServiceDisabledWhenBothClassesAreForever(t *testing.T) {
	service, err := NewAuditRetentionService(&memoryAuditRetentionRepository{}, AuditRetentionConfig{
		CleanupInterval: time.Hour,
		BatchSize:       100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if service.Enabled() {
		t.Fatal("retention worker must be disabled when both retention values are zero")
	}
}

func TestAuditRetentionServiceRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config AuditRetentionConfig
		want   error
	}{
		{
			name: "negative retention",
			config: AuditRetentionConfig{
				NormalRetentionDays: -1, CleanupInterval: time.Hour, BatchSize: 100,
			},
			want: ErrInvalidAuditRetentionDays,
		},
		{
			name: "zero interval",
			config: AuditRetentionConfig{
				NormalRetentionDays: 30, BatchSize: 100,
			},
			want: ErrInvalidAuditCleanupInterval,
		},
		{
			name: "zero batch",
			config: AuditRetentionConfig{
				NormalRetentionDays: 30, CleanupInterval: time.Hour,
			},
			want: ErrInvalidAuditCleanupBatch,
		},
		{
			name: "oversized batch",
			config: AuditRetentionConfig{
				NormalRetentionDays: 30, CleanupInterval: time.Hour, BatchSize: maxAuditCleanupBatchSize + 1,
			},
			want: ErrInvalidAuditCleanupBatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAuditRetentionService(&memoryAuditRetentionRepository{}, tt.config)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDefaultAuditRetentionClass(t *testing.T) {
	critical := []string{
		"audit.retention_cleanup",
		"vault.unseal",
		"sign.transaction",
		"recovery.export",
		"access.service_accounts.update",
		"security.integrity_violation",
	}
	for _, action := range critical {
		if got := domain.DefaultAuditRetentionClass(action); got != domain.AuditRetentionCritical {
			t.Fatalf("%s retention class = %q", action, got)
		}
	}

	normal := []string{"auth.login", "vault.status", "verify.transaction", "audit.list", "http.request"}
	for _, action := range normal {
		if got := domain.DefaultAuditRetentionClass(action); got != domain.AuditRetentionNormal {
			t.Fatalf("%s retention class = %q", action, got)
		}
	}
}

type auditRetentionCall struct {
	retentionClass domain.AuditRetentionClass
	before         time.Time
	limit          int
}

type memoryAuditRetentionRepository struct {
	calls     []auditRetentionCall
	responses map[domain.AuditRetentionClass][]int
}

func (r *memoryAuditRetentionRepository) PurgeExpired(
	_ context.Context,
	retentionClass domain.AuditRetentionClass,
	before time.Time,
	limit int,
) (int, error) {
	r.calls = append(r.calls, auditRetentionCall{retentionClass: retentionClass, before: before, limit: limit})
	if len(r.responses[retentionClass]) == 0 {
		return 0, nil
	}
	deleted := r.responses[retentionClass][0]
	r.responses[retentionClass] = r.responses[retentionClass][1:]
	return deleted, nil
}
