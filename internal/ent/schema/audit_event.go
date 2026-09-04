package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AuditEvent struct{ ent.Schema }

func (AuditEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.Time("occurred_at").Default(time.Now).Immutable(),
		field.String("actor_type").Optional().Nillable().Immutable(),
		field.String("actor_id").Optional().Nillable().Immutable(),
		field.String("session_id").Optional().Nillable().Immutable(),
		field.String("action").NotEmpty().Immutable(),
		field.String("outcome").NotEmpty().Immutable(),
		field.String("source_ip").Optional().Nillable().Immutable(),
		field.String("request_id").Optional().Nillable().Immutable(),
		field.String("method").Optional().Nillable().Immutable(),
		field.String("path").Optional().Nillable().Immutable(),
		field.Int("status_code").Default(0).Immutable(),
		field.JSON("details", map[string]any{}).Default(map[string]any{}).Immutable(),
		field.String("retention_class").Default("NORMAL").Immutable(),
	}
}

func (AuditEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("occurred_at"),
		index.Fields("retention_class", "occurred_at", "id").StorageKey("audit_events_retention_cleanup"),
	}
}

func (AuditEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Checks(map[string]string{
		"audit_action_not_empty":      "length(action) > 0",
		"audit_outcome_not_empty":     "length(outcome) > 0",
		"audit_retention_class_valid": "retention_class IN ('NORMAL', 'CRITICAL')",
	})}
}
