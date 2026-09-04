package service

import (
	"context"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

func TestAuditServiceRecordAndList(t *testing.T) {
	repository := &memoryAuditRepository{}
	service := NewAuditService(repository)

	if err := service.Record(context.Background(), domain.AuditEvent{
		Action: "wallet.create", Outcome: domain.AuditOutcomeSuccess,
	}); err != nil {
		t.Fatal(err)
	}
	if len(repository.items) != 1 || repository.items[0].ID == "" {
		t.Fatalf("unexpected audit repository state: %+v", repository.items)
	}

	result, err := service.List(context.Background(), portin.AuditListInput{Page: 1, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Page != 1 || result.Limit != 50 {
		t.Fatalf("unexpected list result: %+v", result)
	}
}

func TestAuditServiceRejectsInvalidPagination(t *testing.T) {
	service := NewAuditService(&memoryAuditRepository{})
	if _, err := service.List(context.Background(), portin.AuditListInput{Page: 0, Limit: 50}); err != ErrInvalidAuditPagination {
		t.Fatalf("expected ErrInvalidAuditPagination, got %v", err)
	}
	if _, err := service.List(context.Background(), portin.AuditListInput{Page: 1, Limit: 201}); err != ErrInvalidAuditPagination {
		t.Fatalf("expected ErrInvalidAuditPagination, got %v", err)
	}
}

type memoryAuditRepository struct{ items []domain.AuditEvent }

func (r *memoryAuditRepository) Append(_ context.Context, event domain.AuditEvent) error {
	r.items = append(r.items, event)
	return nil
}

func (r *memoryAuditRepository) List(_ context.Context, page, limit int) ([]domain.AuditEvent, int, error) {
	start := (page - 1) * limit
	if start >= len(r.items) {
		return nil, len(r.items), nil
	}
	end := start + limit
	if end > len(r.items) {
		end = len(r.items)
	}
	return append([]domain.AuditEvent(nil), r.items[start:end]...), len(r.items), nil
}
