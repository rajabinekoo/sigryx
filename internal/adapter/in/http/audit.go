package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type auditListInput struct {
	Page  int `query:"page" default:"1" minimum:"1" doc:"1-based page number."`
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"200" doc:"Events per page."`
}

type auditEventBody struct {
	ID         string              `json:"id"`
	OccurredAt time.Time           `json:"occurred_at"`
	ActorType  string              `json:"actor_type,omitempty"`
	ActorID    string              `json:"actor_id,omitempty"`
	SessionID  string              `json:"session_id,omitempty"`
	Action     string              `json:"action"`
	Outcome    domain.AuditOutcome `json:"outcome"`
	SourceIP   string              `json:"source_ip,omitempty"`
	RequestID  string              `json:"request_id,omitempty"`
	Method     string              `json:"method,omitempty"`
	Path       string              `json:"path,omitempty"`
	StatusCode int                 `json:"status_code"`
	Details    map[string]any      `json:"details,omitempty"`
}

type auditListOutput struct {
	Body struct {
		Items []auditEventBody `json:"items"`
		Total int              `json:"total"`
		Page  int              `json:"page"`
		Limit int              `json:"limit"`
	}
}

func registerAuditRoutes(api huma.API, audit portin.AuditUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "list_audit_events", Method: http.MethodGet, Path: "/v1/audit/events",
		Summary: "List system audit events", Tags: []string{"audit"},
	}, func(ctx context.Context, in *auditListInput) (*auditListOutput, error) {
		result, err := audit.List(ctx, portin.AuditListInput{Page: in.Page, Limit: in.Limit})
		if err != nil {
			return nil, translate(err)
		}
		out := &auditListOutput{}
		out.Body.Total, out.Body.Page, out.Body.Limit = result.Total, result.Page, result.Limit
		out.Body.Items = make([]auditEventBody, len(result.Items))
		for i, event := range result.Items {
			out.Body.Items[i] = auditEventBody{
				ID: event.ID, OccurredAt: event.OccurredAt.UTC(),
				ActorType: event.ActorType, ActorID: event.ActorID, SessionID: event.SessionID,
				Action: event.Action, Outcome: event.Outcome, SourceIP: event.SourceIP,
				RequestID: event.RequestID, Method: event.Method, Path: event.Path,
				StatusCode: event.StatusCode, Details: event.Details,
			}
		}
		return out, nil
	})
}
