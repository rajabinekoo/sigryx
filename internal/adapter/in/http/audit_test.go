package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	"github.com/rajabinekoo/sigryx/internal/core/service"
)

func TestAuditListHTTP(t *testing.T) {
	fake := &fakeAuditUseCase{listResult: &portin.AuditListResult{
		Items: []domain.AuditEvent{{
			ID: "01900000-0000-7000-8000-000000000001", OccurredAt: time.Unix(10, 0).UTC(),
			ActorType: "USER", ActorID: "user-1", Action: "wallet.create",
			Outcome: domain.AuditOutcomeSuccess, SourceIP: "192.0.2.10", StatusCode: 200,
		}},
		Total: 1, Page: 1, Limit: 25,
	}}
	response := serveJSON(t, New(Deps{Audit: fake}), http.MethodGet, "/v1/audit/events?page=1&limit=25", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.listInput.Page != 1 || fake.listInput.Limit != 25 {
		t.Fatalf("unexpected input: %+v", fake.listInput)
	}
}

func TestAuditMiddlewareRecordsDeniedRequestMetadata(t *testing.T) {
	audit := &fakeAuditUseCase{}
	auth := &middlewareAuthStub{authorizeErr: service.ErrPermissionDenied}
	handler := New(Deps{Auth: auth, Audit: audit})

	request := httptest.NewRequest(http.MethodPost, "/v1/sign/data", bytes.NewBufferString(`{"wallet_id":"w","context":"c","format":"JSON","payload":{}}`))
	request.RemoteAddr = "203.0.113.10:4444"
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit events = %d", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "sign.generic" || event.Outcome != domain.AuditOutcomeDenied || event.RequestID == "" || event.SourceIP == "" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}

type fakeAuditUseCase struct {
	events     []domain.AuditEvent
	listInput  portin.AuditListInput
	listResult *portin.AuditListResult
}

func (f *fakeAuditUseCase) Record(_ context.Context, event domain.AuditEvent) error {
	f.events = append(f.events, event)
	return nil
}
func (f *fakeAuditUseCase) List(_ context.Context, input portin.AuditListInput) (*portin.AuditListResult, error) {
	f.listInput = input
	return f.listResult, nil
}
