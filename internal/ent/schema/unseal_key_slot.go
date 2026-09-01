package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UnsealKeySlot struct {
	ent.Schema
}

func (UnsealKeySlot) Fields() []ent.Field {
	return []ent.Field{
		// Stable position of this unseal key in the N-of-N set.
		//
		// Root encryption key derivation always consumes recovered
		// unseal keys ordered by this value.
		field.Int("id").
			Min(1).
			Immutable(),

		// AES-GCM encrypted real unseal key.
		//
		// Self-contained binary payload:
		// version || nonce || ciphertext || authentication tag
		field.Bytes("wrapped_key").
			Immutable(),

		// Random server-side material used together with the
		// owner-held secret to derive the key that unwraps wrapped_key.
		//
		// This value never leaves Sigryx.
		field.Bytes("server_key_material").
			Immutable(),
	}
}

func (UnsealKeySlot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("wrapped_key").
			Unique(),
	}
}

func (UnsealKeySlot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"unseal_wrapped_key_not_empty": "octet_length(wrapped_key) > 0",

			"unseal_server_key_material_size": "octet_length(server_key_material) = 32",
		}),
	}
}
