package postgres

import "testing"

func TestValidateSchemaName(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{"vault", "sigryx_vault", "tenant_01", "public"} {
		if err := ValidateSchemaName(schema); err != nil {
			t.Fatalf("ValidateSchemaName(%q): %v", schema, err)
		}
	}

	for _, schema := range []string{"", "Vault", "vault-prod", "1vault", "public;drop schema public", "pg_catalog", "information_schema"} {
		if err := ValidateSchemaName(schema); err == nil {
			t.Fatalf("ValidateSchemaName(%q) unexpectedly succeeded", schema)
		}
	}
}
