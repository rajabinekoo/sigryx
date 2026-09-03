package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Session struct{ ent.Schema }

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.String("user_id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.Bytes("refresh_token_hash"),
		field.Time("expires_at"),
		field.Time("revoked_at").Optional().Nillable(),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("refresh_token_hash").Unique(),
		index.Fields("user_id"),
	}
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Field("user_id").Unique().Required().Immutable(),
	}
}

func (Session) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"session_refresh_token_hash_size": "octet_length(refresh_token_hash) = 32",
		}),
	}
}
