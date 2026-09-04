package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Append(ctx context.Context, event domain.AuditEvent) error {
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.RetentionClass = event.EffectiveRetentionClass()
	details, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_events (
    id, occurred_at, actor_type, actor_id, session_id, action, outcome,
    source_ip, request_id, method, path, status_code, details, retention_class
) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7,
          NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), $12, $13::jsonb, $14)
`, event.ID, event.OccurredAt, event.ActorType, event.ActorID, event.SessionID, event.Action,
		string(event.Outcome), event.SourceIP, event.RequestID, event.Method, event.Path, event.StatusCode, details, string(event.RetentionClass))
	if err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

func (r *AuditRepository) List(ctx context.Context, page, limit int) ([]domain.AuditEvent, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
SELECT id, occurred_at, COALESCE(actor_type, ''), COALESCE(actor_id, ''), COALESCE(session_id, ''),
       action, outcome, COALESCE(source_ip, ''), COALESCE(request_id, ''), COALESCE(method, ''),
       COALESCE(path, ''), status_code, details, retention_class
FROM audit_events
ORDER BY occurred_at DESC, id DESC
LIMIT $1 OFFSET $2
`, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	items := make([]domain.AuditEvent, 0, limit)
	for rows.Next() {
		var event domain.AuditEvent
		var outcome string
		var details []byte
		if err := rows.Scan(
			&event.ID, &event.OccurredAt, &event.ActorType, &event.ActorID, &event.SessionID,
			&event.Action, &outcome, &event.SourceIP, &event.RequestID, &event.Method,
			&event.Path, &event.StatusCode, &details, &event.RetentionClass,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit event: %w", err)
		}
		event.Outcome = domain.AuditOutcome(outcome)
		if len(details) > 0 {
			if err := json.Unmarshal(details, &event.Details); err != nil {
				return nil, 0, fmt.Errorf("decode audit details: %w", err)
			}
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit events: %w", err)
	}
	return items, total, nil
}

func (r *AuditRepository) PurgeExpired(
	ctx context.Context,
	retentionClass domain.AuditRetentionClass,
	before time.Time,
	limit int,
) (int, error) {
	var deleted int
	if err := r.pool.QueryRow(ctx, `
SELECT public.sigryx_purge_audit_events($1, $2, $3)
`, string(retentionClass), before.UTC(), limit).Scan(&deleted); err != nil {
		return 0, fmt.Errorf("purge expired audit events: %w", err)
	}
	return deleted, nil
}

var _ interface {
	Append(context.Context, domain.AuditEvent) error
	List(context.Context, int, int) ([]domain.AuditEvent, int, error)
	PurgeExpired(context.Context, domain.AuditRetentionClass, time.Time, int) (int, error)
} = (*AuditRepository)(nil)
