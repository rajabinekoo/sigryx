---
title: Configuration
description: Environment-variable reference for the Sigryx image and server process.
---

Sigryx is configured entirely through environment variables. The published Docker image and the native Go process use the same application variables.

For repository development, start from:

```bash
cp .env.example .env
```

Do not commit real secrets.

## Docker Compose convenience variables

The repository's root `compose.yml` also understands a few variables that are only used by Docker Compose itself:

| Variable | Default | Purpose |
| --- | --- | --- |
| `SIGRYX_IMAGE` | `sigryx:dev` | Image name/tag used by the repository Compose stack. |
| `SIGRYX_PORT` | `8080` | Host port mapped to the bundled Compose stack's Sigryx container port `8080`. |
| `POSTGRES_PORT` | `5432` | Host port mapped to the local PostgreSQL service. |
| `POSTGRES_USER` | `sigryx` | Local Compose PostgreSQL user. |
| `POSTGRES_PASSWORD` | `sigryx` | Local Compose PostgreSQL password. |
| `POSTGRES_DB` | `sigryx` | Local Compose PostgreSQL database. |

Changing:

```dotenv
SIGRYX_PORT=9090
```

changes the browser/host URL to:

```text
http://localhost:9090
```

It does not change Sigryx's internal listener. The repository Compose stack intentionally keeps the container listener at `0.0.0.0:8080` so changing the host port cannot accidentally desynchronize Docker port publishing from the application listener.

## Service and logging

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `SERVICE_NAME` | `sigryx` | no | Service name included in logs. |
| `LOG_LEVEL` | `info` | no | Application log level. |
| `LOG_FORMAT` | `json` | no | Logger output format. |

## PostgreSQL

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `POSTGRES_DSN` | none | **yes** | PostgreSQL connection string. |
| `POSTGRES_SCHEMA` | `vault` | no | Schema containing Sigryx application objects. |
| `POSTGRES_AUTO_MIGRATE` | `true` in the Docker image | no | Apply pending Atlas migrations before starting Sigryx. |
| `POSTGRES_MAX_CONNS` | `10` | no | Maximum pool connections. |
| `POSTGRES_MIN_CONNS` | `2` | no | Minimum pool connections. |
| `POSTGRES_CONNECT_TIMEOUT` | `5s` | no | Connection establishment timeout. |
| `POSTGRES_MAX_CONN_LIFETIME` | `30m` | no | Maximum connection lifetime. |
| `POSTGRES_MAX_CONN_IDLE_TIME` | `5m` | no | Maximum idle duration. |

Example:

```dotenv
POSTGRES_DSN=postgres://sigryx:strong-password@postgres.internal:5432/sigryx?sslmode=require
POSTGRES_SCHEMA=sigryx_vault
POSTGRES_AUTO_MIGRATE=true
```

### Application schema

The default is:

```text
vault
```

Valid custom names include:

```text
sigryx_vault
security_keys
vault_prod
```

The name must match:

```text
^[a-z_][a-z0-9_]{0,62}$
```

Sigryx rejects PostgreSQL-reserved schema names such as `pg_catalog`, any `pg_*` name, and `information_schema`.

The runtime pool explicitly sets:

```text
search_path=<POSTGRES_SCHEMA>,pg_catalog
```

so unqualified Sigryx queries do not silently fall back to `public`. Atlas applies migrations with `search_path=<POSTGRES_SCHEMA>` as well, while its revision table is stored separately in `atlas_schema_revisions.atlas_schema_revisions`.

The historical migration chain remains intact for compatibility. A later migration moves all Sigryx application tables and database functions from `public` into `POSTGRES_SCHEMA`. Existing Atlas-managed installations therefore upgrade in place.

:::caution
Do not casually change `POSTGRES_SCHEMA` after a production database has been initialized. The schema name should be treated as persistent deployment configuration.
:::

### Automatic migration behavior

The Docker image contains Atlas and the exact migration directory for that image version.

With:

```dotenv
POSTGRES_AUTO_MIGRATE=true
```

startup is:

```text
container entrypoint
      ↓
atlas migrate apply
      ↓
only on success
      ↓
Sigryx process
```

If migration fails, the server does not start.

Use:

```dotenv
POSTGRES_AUTO_MIGRATE=false
```

when a separate deployment job owns DDL privileges and migrations.

`POSTGRES_AUTO_MIGRATE` is interpreted by the Docker entrypoint. A native `go run ./cmd` invocation does not automatically execute Atlas.

## HTTP server

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `HTTP_ADDR` | `0.0.0.0:8080` in the image | no | Real Sigryx bind address for direct image, Kubernetes/systemd, or native execution. |
| `HTTP_READ_TIMEOUT` | `15s` | no | HTTP read timeout. |
| `HTTP_WRITE_TIMEOUT` | `15s` | no | HTTP write timeout. |
| `HTTP_IDLE_TIMEOUT` | `60s` | no | Keep-alive idle timeout. |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | no | Graceful shutdown timeout. |

The same listener serves:

```text
/v1/*
/docs
/openapi.json
```

In the repository's bundled `compose.yml`, `HTTP_ADDR` is intentionally set to `0.0.0.0:8080` and the host-facing port is controlled with `SIGRYX_PORT`. This prevents accidental mismatches such as publishing host `9090` to container `8080` while Sigryx itself listens on container `9091`.

When running the image directly, `HTTP_ADDR` remains fully configurable. For example:

```bash
docker run --rm \
  -e HTTP_ADDR=0.0.0.0:9091 \
  -p 9091:9091 \
  ... \
  rajabinekoo/sigryx:v1.0.0
```

The published container port must match the port portion of `HTTP_ADDR` when you override the internal listener.

## Authentication

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `SETUP_TOKEN` | empty | no | One-time Root Admin bootstrap token. Empty disables setup. If set, at least 32 characters. |
| `AUTH_JWT_SECRET` | none | **yes** | HMAC secret used by the JWT token manager. Minimum 32 bytes. |
| `AUTH_ISSUER` | `sigryx` | no | JWT issuer. |
| `AUTH_AUDIENCE` | `sigryx-api` | no | JWT audience. |
| `AUTH_ACCESS_TTL` | `10m` | no | Access-token lifetime. |
| `AUTH_REFRESH_TTL` | `168h` | no | User refresh-session lifetime. |
| `TRUSTED_PROXY_CIDRS` | empty | no | Comma-separated proxies allowed to supply forwarded client-IP headers. |

Generate `SETUP_TOKEN` and `AUTH_JWT_SECRET` independently:

```bash
openssl rand -hex 32
openssl rand -hex 32
```

Example:

```dotenv
SETUP_TOKEN=0d94...random-value...
AUTH_JWT_SECRET=12af...different-random-value...
AUTH_ISSUER=sigryx
AUTH_AUDIENCE=sigryx-api
```

These values are unrelated to:

- unseal owner keys;
- the Vault Encryption Key;
- HD key-root seeds;
- recovery keys.

After the initial Root Admin has been created, an operator may remove `SETUP_TOKEN` to disable bootstrap by configuration as well as by database state.

## Audit retention

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `AUDIT_NORMAL_RETENTION_DAYS` | `30` | no | Days to retain `NORMAL` audit rows. `0` means forever. |
| `AUDIT_CRITICAL_RETENTION_DAYS` | `365` | no | Days to retain `CRITICAL` audit rows. `0` means forever. |
| `AUDIT_CLEANUP_INTERVAL` | `6h` | no | Retention worker interval. |
| `AUDIT_CLEANUP_BATCH_SIZE` | `5000` | no | Rows per purge call. Valid range `1..50000`. |

If both retention-day values are `0`, cleanup is disabled.

## Integrity alerting

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `ALERT_WEBHOOK_URL` | empty | no | Optional target for critical integrity incident alerts. |
| `ALERT_WEBHOOK_TIMEOUT` | `2s` | no | Alert delivery timeout. |

## Vault limits

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `MAX_UNSEAL_SIZE` | `10` | no | Maximum number of N-of-N unseal slots accepted at initialization. |

## Minimal Docker configuration

A consuming project usually needs only this much to get started:

```yaml
sigryx:
  image: rajabinekoo/sigryx:v1.0.0
  environment:
    POSTGRES_DSN: postgres://app:password@postgres:5432/backend?sslmode=disable
    POSTGRES_SCHEMA: sigryx_vault
    POSTGRES_AUTO_MIGRATE: "true"
    SETUP_TOKEN: ${SIGRYX_SETUP_TOKEN}
    AUTH_JWT_SECRET: ${SIGRYX_AUTH_JWT_SECRET}
  ports:
    - "8086:8080"
```

For secure-memory locking, also grant `IPC_LOCK` and an appropriate `memlock` ulimit as shown in [Install & run](/getting-started/installation/).

## Production example

```dotenv
SERVICE_NAME=sigryx
LOG_LEVEL=info
LOG_FORMAT=json

POSTGRES_DSN=postgres://sigryx:${DB_PASSWORD}@postgres.internal:5432/sigryx?sslmode=require
POSTGRES_SCHEMA=sigryx_vault
POSTGRES_AUTO_MIGRATE=true
POSTGRES_MAX_CONNS=20
POSTGRES_MIN_CONNS=4
POSTGRES_CONNECT_TIMEOUT=5s
POSTGRES_MAX_CONN_LIFETIME=30m
POSTGRES_MAX_CONN_IDLE_TIME=5m

HTTP_ADDR=0.0.0.0:8080
HTTP_READ_TIMEOUT=15s
HTTP_WRITE_TIMEOUT=15s
HTTP_IDLE_TIMEOUT=60s
HTTP_SHUTDOWN_TIMEOUT=15s

SETUP_TOKEN=<secret-manager-injected-bootstrap-token>
AUTH_JWT_SECRET=<independent-secret-manager-injected-jwt-secret>
AUTH_ISSUER=sigryx
AUTH_AUDIENCE=sigryx-api
AUTH_ACCESS_TTL=10m
AUTH_REFRESH_TTL=168h
TRUSTED_PROXY_CIDRS=10.42.0.0/16

AUDIT_NORMAL_RETENTION_DAYS=30
AUDIT_CRITICAL_RETENTION_DAYS=365
AUDIT_CLEANUP_INTERVAL=6h
AUDIT_CLEANUP_BATCH_SIZE=5000

ALERT_WEBHOOK_URL=https://security.internal.example/sigryx
ALERT_WEBHOOK_TIMEOUT=2s
MAX_UNSEAL_SIZE=10
```
