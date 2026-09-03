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

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.String("username").NotEmpty().Unique(),
		field.String("password_hash").NotEmpty(),
		field.Bool("is_root_admin").Default(false).Immutable(),
		field.Bool("active").Default(true),
		field.String("role_id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Optional().Nillable(),
		field.Strings("allowed_cidrs").Default([]string{}),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_root_admin").Unique().Annotations(entsql.IndexWhere("is_root_admin = true")),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", Role.Type).Ref("users").Field("role_id").Unique(),
		edge.To("sessions", Session.Type),
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"user_root_role_consistency": "(is_root_admin = true AND role_id IS NULL) OR (is_root_admin = false AND role_id IS NOT NULL)",
		}),
	}
}
