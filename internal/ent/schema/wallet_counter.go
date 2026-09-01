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

type WalletCounter struct {
	ent.Schema
}

func (WalletCounter) Fields() []ent.Field {
	return []ent.Field{
		// HD root whose derivation index is being tracked.
		field.String("key_root_id").
			SchemaType(map[string]string{
				dialect.Postgres: "uuid",
			}).
			Immutable(),

		// Adapter owns an independent logical derivation sequence.
		field.String("adapter").
			NotEmpty().
			Immutable(),

		// Next logical child index available for allocation.
		//
		// Valid BIP32 non-hardened indexes are:
		// 0 .. 2^31-1
		//
		// 2^31 represents the exhausted terminal state.
		field.Int64("next_index").
			Default(0).
			Min(0).
			Max(1 << 31),
	}
}

func (WalletCounter) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("key_root", KeyRoot.Type).
			Ref("wallet_counters").
			Field("key_root_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (WalletCounter) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields(
			"key_root_id",
			"adapter",
		).
			Unique(),
	}
}

func (WalletCounter) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"wallet_counter_index_range": "next_index >= 0 AND next_index <= 2147483648",
		}),
	}
}
