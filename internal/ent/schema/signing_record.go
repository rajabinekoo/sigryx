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

type SigningRecord struct{ ent.Schema }

func (SigningRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").SchemaType(map[string]string{dialect.Postgres: "uuid"}).Immutable(),
		field.String("context").NotEmpty().Immutable(),
		field.String("object_id").NotEmpty().Immutable(),
		field.Bytes("encrypted_record").Immutable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SigningRecord) Indexes() []ent.Index {
	return []ent.Index{index.Fields("context", "object_id").Unique()}
}

func (SigningRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Checks(map[string]string{
		"signing_record_context_not_empty":   "length(context) > 0",
		"signing_record_object_id_not_empty": "length(object_id) > 0",
		"signing_record_encrypted_not_empty": "octet_length(encrypted_record) > 0",
	})}
}
