package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type KeyRoot struct {
	ent.Schema
}

func (KeyRoot) Fields() []ent.Field {
	return []ent.Field{
		// Application-generated UUIDv7.
		field.String("id").
			SchemaType(map[string]string{
				dialect.Postgres: "uuid",
			}).
			Immutable(),

		// Cryptographic derivation algorithm used by this root.
		//
		// Examples:
		// BIP32_SECP256K1
		// SLIP10_ED25519
		//
		// This is intentionally blockchain-agnostic.
		field.String("derivation_scheme").NotEmpty().Immutable(),

		// Encrypted HD master seed.
		//
		// Self-contained binary payload:
		// version || nonce || ciphertext || authentication tag
		field.Bytes("sealed_seed").Immutable(),
	}
}

func (KeyRoot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("wallets", Wallet.Type),

		edge.To("wallet_counters", WalletCounter.Type),
	}
}

func (KeyRoot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"key_root_sealed_seed_not_empty": "octet_length(sealed_seed) > 0",
		}),
	}
}
