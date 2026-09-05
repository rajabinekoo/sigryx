#!/bin/sh
set -eu

# Keep the image useful for operational/debug commands as well. Migration is
# only automatic for the normal Sigryx startup command.
if [ "${1:-}" != "sigryx" ]; then
  exec "$@"
fi

: "${POSTGRES_DSN:?POSTGRES_DSN is required}"

POSTGRES_SCHEMA="${POSTGRES_SCHEMA:-vault}"
POSTGRES_AUTO_MIGRATE="${POSTGRES_AUTO_MIGRATE:-true}"

validate_schema_name() {
  case "$POSTGRES_SCHEMA" in
    information_schema|pg_*)
      echo "sigryx: POSTGRES_SCHEMA '$POSTGRES_SCHEMA' is reserved" >&2
      exit 1
      ;;
  esac

  if ! printf '%s' "$POSTGRES_SCHEMA" | grep -Eq '^[a-z_][a-z0-9_]{0,62}$'; then
    echo "sigryx: POSTGRES_SCHEMA must match ^[a-z_][a-z0-9_]{0,62}$" >&2
    exit 1
  fi
}

is_true() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    0|false|no|off) return 1 ;;
    *)
      echo "sigryx: POSTGRES_AUTO_MIGRATE must be true or false" >&2
      exit 1
      ;;
  esac
}

validate_schema_name

if is_true "$POSTGRES_AUTO_MIGRATE"; then
  # Atlas opens the runtime connection with search_path set to the configured
  # application schema. PostgreSQL requires that schema to exist before Atlas
  # can open the scoped connection, so bootstrap only the empty namespace here.
  # All application objects are still created/moved exclusively by versioned
  # Atlas migrations. POSTGRES_SCHEMA is validated above before interpolation.
  echo "sigryx: ensuring database schema exists (schema=$POSTGRES_SCHEMA)"
  psql "$POSTGRES_DSN" \
    --no-psqlrc \
    --set=ON_ERROR_STOP=1 \
    --quiet \
    --command="CREATE SCHEMA IF NOT EXISTS \"$POSTGRES_SCHEMA\";"

  # Sigryx shipped its first five migrations before application schemas became
  # configurable, so those immutable historical migrations explicitly qualify
  # objects with "public". Some PostgreSQL installations intentionally drop
  # the public schema. Bootstrap it temporarily so an empty database can replay
  # the historical migration chain, then the application_schema migration moves
  # all Sigryx objects into POSTGRES_SCHEMA. We never use CASCADE when cleaning
  # up, so a pre-existing/non-empty public schema is never destroyed.
  if [ "$POSTGRES_SCHEMA" != "public" ]; then
    echo "sigryx: ensuring legacy migration schema exists (schema=public)"
    psql "$POSTGRES_DSN" \
      --no-psqlrc \
      --set=ON_ERROR_STOP=1 \
      --quiet \
      --command='CREATE SCHEMA IF NOT EXISTS "public";'
  fi

  echo "sigryx: applying database migrations (schema=$POSTGRES_SCHEMA)"
  atlas migrate apply \
    --config file:///app/atlas.hcl \
    --env runtime \
    --var "schema_name=$POSTGRES_SCHEMA"
  echo "sigryx: database migrations are current"

  # If public was absent before startup, or was left behind by a previously
  # interrupted migration attempt, it should now be empty because the final
  # migration moved Sigryx objects into POSTGRES_SCHEMA. DROP without CASCADE is
  # intentionally best-effort: if another application owns anything in public,
  # PostgreSQL refuses the drop and Sigryx leaves the schema untouched.
  if [ "$POSTGRES_SCHEMA" != "public" ]; then
    if psql "$POSTGRES_DSN" \
      --no-psqlrc \
      --set=ON_ERROR_STOP=1 \
      --quiet \
      --command='DROP SCHEMA IF EXISTS "public";' >/dev/null 2>&1; then
      :
    else
      echo "sigryx: public schema is non-empty; leaving it in place"
    fi
  fi
else
  echo "sigryx: automatic database migrations are disabled"
fi

exec /usr/local/bin/sigryx
