package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type ServiceAccount struct{ ent.Schema }

func (ServiceAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.String("name").NotEmpty().Unique(),
		field.String("client_id").NotEmpty().Unique(),
		field.Bytes("client_secret_hash").Immutable(),
		field.Bool("active").Default(true),
		field.String("role_id").SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Strings("allowed_cidrs").Default([]string{}),
	}
}

func (ServiceAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", Role.Type).Ref("service_accounts").Field("role_id").Unique().Required(),
	}
}

func (ServiceAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"service_account_secret_hash_size": "octet_length(client_secret_hash) = 32",
		}),
	}
}
