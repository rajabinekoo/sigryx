---
title: Configuration
description: Complete environment-variable reference for the current Sigryx process.
---

Sigryx loads configuration from environment variables. The repository Makefile also sources `.env` for local commands.

Start from:

```bash
cp .example.env .env
```

Do not commit real secrets.

## Service and logging

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `SERVICE_NAME` | `sigryx` | no | Service name included in logging/identity. |
| `LOG_LEVEL` | `info` | no | Application log level. |
| `LOG_FORMAT` | `json` | no | Log encoding used by the logger package. |

## PostgreSQL

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `POSTGRES_DSN` | none | **yes** | PostgreSQL connection string. |
| `POSTGRES_MAX_CONNS` | `10` | no | Maximum pool connections. |
| `POSTGRES_MIN_CONNS` | `2` | no | Minimum pool connections. |
| `POSTGRES_CONNECT_TIMEOUT` | `5s` | no | Connection establishment timeout. |
| `POSTGRES_MAX_CONN_LIFETIME` | `30m` | no | Maximum connection lifetime. |
| `POSTGRES_MAX_CONN_IDLE_TIME` | `5m` | no | Maximum idle duration. |

Example:

```dotenv
POSTGRES_DSN=postgres://sigryx:strong-password@postgres.internal:5432/sigryx?sslmode=require
```

Use TLS and narrowly scoped database credentials in production.

## HTTP server

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `HTTP_ADDR` | `:8080` | no | Listen address. |
| `HTTP_READ_TIMEOUT` | `15s` | no | HTTP read timeout. |
| `HTTP_WRITE_TIMEOUT` | `15s` | no | HTTP write timeout. |
| `HTTP_IDLE_TIMEOUT` | `60s` | no | Keep-alive idle timeout. |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | no | Graceful shutdown timeout. |

The same HTTP listener serves:

```text
/v1/*
/docs
/openapi.json
```

## Authentication

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `SETUP_TOKEN` | empty | no | One-time Root Admin bootstrap token. Empty disables setup. If set, it must be at least 32 characters. |
| `AUTH_JWT_SECRET` | none | **yes** | Secret used by the JWT token manager. Keep independent from Vault keys. |
| `AUTH_ISSUER` | `sigryx` | no | JWT issuer. |
| `AUTH_AUDIENCE` | `sigryx-api` | no | JWT audience. |
| `AUTH_ACCESS_TTL` | `10m` | no | Access-token lifetime. Must be positive. |
| `AUTH_REFRESH_TTL` | `168h` | no | User refresh-session lifetime. Must be positive. |
| `TRUSTED_PROXY_CIDRS` | empty | no | Comma-separated proxies allowed to supply forwarded client-IP headers. |

Example:

```dotenv
AUTH_JWT_SECRET=2c0d...64-hex-or-other-high-entropy-secret...
TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.50.10/32
```

Generate secrets rather than choosing human passwords:

```bash
openssl rand -hex 32
```

## Audit retention

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `AUDIT_NORMAL_RETENTION_DAYS` | `30` | no | Days to retain `NORMAL` audit rows. `0` means forever. |
| `AUDIT_CRITICAL_RETENTION_DAYS` | `365` | no | Days to retain `CRITICAL` audit rows. `0` means forever. |
| `AUDIT_CLEANUP_INTERVAL` | `6h` | no | Worker interval. Must be greater than zero. |
| `AUDIT_CLEANUP_BATCH_SIZE` | `5000` | no | Rows per database purge call. Valid range: `1..50000`. |

If both retention-day values are `0`, the retention service is disabled even though its interval/batch configuration must still pass construction validation.

## Integrity alerting

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `ALERT_WEBHOOK_URL` | empty | no | Optional target for integrity incident alerts. Empty disables remote alert delivery. |
| `ALERT_WEBHOOK_TIMEOUT` | `2s` | no | HTTP timeout for alert delivery. |

The webhook receives critical integrity incident data. Treat the destination as part of the security boundary; do not send sensitive context to an untrusted collector.

## Vault limits

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `MAX_UNSEAL_SIZE` | none | **yes** | Maximum number of N-of-N unseal slots accepted during initialization. |

The current `.example.env` uses:

```dotenv
MAX_UNSEAL_SIZE=10
```

## Variables currently present but not part of the active runtime composition

The example environment contains historical gRPC-related settings:

```dotenv
GRPC_TIMEOUT
GPRC_MAX_TRIES
SERVICE_GRPC_ADDRESS
```

The current `internal/config.Config` and `cmd/main.go` HTTP composition do not consume these values. Treat them as inactive/legacy configuration until code in a release wires them into a runtime component.

## Production example

A conservative starting point might look like:

```dotenv
SERVICE_NAME=sigryx
LOG_LEVEL=info
LOG_FORMAT=json

POSTGRES_DSN=postgres://sigryx:${DB_PASSWORD}@postgres.internal:5432/sigryx?sslmode=require
POSTGRES_MAX_CONNS=20
POSTGRES_MIN_CONNS=4
POSTGRES_CONNECT_TIMEOUT=5s
POSTGRES_MAX_CONN_LIFETIME=30m
POSTGRES_MAX_CONN_IDLE_TIME=5m

HTTP_ADDR=:8080
HTTP_READ_TIMEOUT=15s
HTTP_WRITE_TIMEOUT=15s
HTTP_IDLE_TIMEOUT=60s
HTTP_SHUTDOWN_TIMEOUT=15s

SETUP_TOKEN=
AUTH_JWT_SECRET=<secret-manager-injected-value>
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

After bootstrap, many operators choose to remove `SETUP_TOKEN` from the runtime secret configuration so the setup endpoint is disabled by configuration as well as by the existing Root Admin record.
