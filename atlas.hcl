variable "schema_name" {
  type    = string
  default = "vault"
}

# Runtime migrations are rendered with the deployment schema selected through
# POSTGRES_SCHEMA. Historical migrations remain untouched; the schema move is
# performed by the templated application_schema migration.
data "template_dir" "runtime_migrations" {
  path = "migrations"
  vars = {
    schema_name = var.schema_name
  }
}

# Ent currently describes the application in PostgreSQL's default schema. For
# migration authoring we therefore replay the history with the schema-move
# migration rendered as a no-op. With a schema-scoped dev database Atlas emits
# future table changes without hard-coded schema qualifiers, so the same files
# can later be applied to any POSTGRES_SCHEMA by the runtime environment.
data "template_dir" "authoring_migrations" {
  path = "migrations"
  vars = {
    schema_name = "public"
  }
}

env "local" {
  src = "ent://internal/ent/schema?dialect=postgres"
  dev = "docker://postgres/16/test?search_path=public"
  url = urlqueryset(getenv("POSTGRES_DSN"), "search_path", "public")
  migration {
    dir              = data.template_dir.authoring_migrations.url
    revisions_schema = "atlas_schema_revisions"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

# Used by the published Docker image. The target connection is explicitly
# scoped to the configured application schema. Atlas revision metadata stays in
# its own schema and is not mixed with Sigryx application tables.
env "runtime" {
  url = urlqueryset(getenv("POSTGRES_DSN"), "search_path", var.schema_name)
  migration {
    dir              = data.template_dir.runtime_migrations.url
    revisions_schema = "atlas_schema_revisions"
  }
}
