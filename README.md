<div align="center">
  <img src="./assets/sigryx-logo.png" alt="Sigryx" width="760" />
</div>

<p align="center">
  <strong>Self-hosted key and signing infrastructure for application backends.</strong>
</p>

Sigryx keeps deterministic key-root material encrypted at rest, reconstructs sensitive runtime state only after an explicit N-of-N unseal ceremony, derives application wallets without persisting child private keys, and exposes authenticated signing/verification APIs with auditability and recovery tooling.

> Sigryx is security-sensitive infrastructure. Read the security model and production guidance before using it with valuable keys.

## Use Sigryx without cloning the repository

The normal product distribution is a **single Docker image**:

```bash
docker pull rajabinekoo/sigryx:latest
```

That image already contains:

- the Sigryx server;
- Atlas;
- the exact versioned migrations for the release;
- the libsodium runtime required by secure memory.

To consume Sigryx you only need Docker and PostgreSQL. You do not need Go, Atlas, libsodium development packages, or the Sigryx source tree.

### Run directly with an existing PostgreSQL database

Generate independent application secrets:

```bash
export SIGRYX_SETUP_TOKEN="$(openssl rand -hex 32)"
export SIGRYX_AUTH_JWT_SECRET="$(openssl rand -hex 32)"
```

Then run the image. This example expects PostgreSQL on the Docker host:

```bash
docker run -d \
  --name sigryx \
  --restart unless-stopped \
  --add-host host.docker.internal:host-gateway \
  --cap-drop ALL \
  --cap-add IPC_LOCK \
  --security-opt no-new-privileges:true \
  --ulimit memlock=-1:-1 \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=32m \
  -p 8080:8080 \
  -e HTTP_ADDR=0.0.0.0:8080 \
  -e 'POSTGRES_DSN=postgres://sigryx:sigryx@host.docker.internal:5432/sigryx?sslmode=disable' \
  -e POSTGRES_SCHEMA=sigryx_vault \
  -e POSTGRES_AUTO_MIGRATE=true \
  -e SETUP_TOKEN="$SIGRYX_SETUP_TOKEN" \
  -e AUTH_JWT_SECRET="$SIGRYX_AUTH_JWT_SECRET" \
  rajabinekoo/sigryx:latest
```

Check it:

```bash
curl http://localhost:8080/v1/health
```

REST API client:

```text
http://localhost:8080/docs
```

### Put PostgreSQL beside Sigryx with Docker Compose

For a new project, the easiest topology is:

```text
Docker Compose
├── postgres
└── sigryx
```

A minimal copy/paste Compose file is available in the [Install & run guide](./docs/src/content/docs/getting-started/installation.md). Sigryx may also use an existing PostgreSQL service in your application's Compose stack; set a dedicated schema such as:

```dotenv
POSTGRES_SCHEMA=sigryx_vault
```

and point `POSTGRES_DSN` at the existing PostgreSQL service name.

Inside the same Docker network, applications call Sigryx using:

```text
http://sigryx:8080
```

The image automatically creates the configured application schema and applies pending Atlas migrations before starting the HTTP server.

## Configuration that normally matters first

```dotenv
POSTGRES_DSN=postgres://user:password@postgres:5432/database?sslmode=disable
POSTGRES_SCHEMA=sigryx_vault
POSTGRES_AUTO_MIGRATE=true

HTTP_ADDR=0.0.0.0:8080

SETUP_TOKEN=<random-secret-at-least-32-characters>
AUTH_JWT_SECRET=<independent-random-secret-at-least-32-bytes>
```

Generate the two application secrets independently:

```bash
openssl rand -hex 32
openssl rand -hex 32
```

`POSTGRES_SCHEMA` is configurable so Sigryx can share a PostgreSQL database with another backend while keeping its own objects isolated. Runtime connections use `<POSTGRES_SCHEMA>,pg_catalog` as `search_path` and do not depend on `public` as an application fallback.

`HTTP_ADDR` is the real server bind address inside the container. Docker host-port mapping is independent, for example:

```text
-p 9090:8080
```

means `localhost:9090` reaches Sigryx listening on container port `8080`.

See [Install & run](./docs/src/content/docs/getting-started/installation.md) for complete `docker run` and Docker Compose examples, and [Configuration](./docs/src/content/docs/operations/configuration.md) for every environment variable.

## Automatic Atlas migrations

The official image owns the normal migration path:

```text
container starts
      │
      ├─ ensure configured PostgreSQL schema exists
      ├─ Atlas applies pending migrations
      └─ Sigryx starts only after migration succeeds
```

Disable that behavior with:

```dotenv
POSTGRES_AUTO_MIGRATE=false
```

when a production deployment pipeline owns DDL separately.

## Current capabilities

- `UNINITIALIZED -> SEALED -> UNSEALED` Vault lifecycle.
- N-of-N owner-held unseal credentials.
- Runtime Vault Encryption Key derived only after successful unseal.
- Protected-memory secret handling and explicit secret destruction.
- Encrypted HD key roots; plaintext root seeds are never returned by the API.
- Deterministic Ethereum-compatible wallets using BIP32/secp256k1 and BIP44 paths.
- No persisted or returned child private keys.
- Ethereum legacy and EIP-1559 transaction signing/verification.
- EIP-712 typed-data signing/verification.
- Domain-separated generic JSON/RAW signing/verification.
- Integrity-field signing backed by encrypted append-only Signing Records.
- JWT user authentication, rotating refresh tokens, service accounts, RBAC and CIDR restrictions.
- Audit events with configurable normal/critical retention.
- Root-admin key-root recovery export/import.
- Generated OpenAPI and a live interactive REST API client.

The current implementation should not be assumed to provide HSM-backed roots, distributed/quorum signing, or an intent-aware policy engine unless those capabilities are present in the release you are running.

## Documentation

Engineering documentation is published at:

**https://rajabinekoo.github.io/sigryx/**

Useful starting points in the repository:

- [5-minute usage](./docs/src/content/docs/usage.md)
- [Install & run](./docs/src/content/docs/getting-started/installation.md)
- [Architecture](./docs/src/content/docs/concepts/architecture.md)
- [Security model](./docs/src/content/docs/security/security-model.md)
- [Configuration](./docs/src/content/docs/operations/configuration.md)
- [Production deployment](./docs/src/content/docs/operations/production.md)
- [HTTP API reference](./docs/src/content/docs/reference/http-api.md)

### Live REST API client

Every running Sigryx server also exposes:

```text
GET /docs
GET /openapi.json
```

For the default local Compose stack:

```text
http://localhost:8080/docs
http://localhost:8080/openapi.json
```

The public documentation site explains architecture and operations. `/docs` reflects the actual Huma REST contract generated by the running Sigryx build.

## High-level architecture

```text
                    Application / Operator
                             │
                             │ HTTP + JWT
                             ▼
                  ┌───────────────────────┐
                  │ Gin + Huma HTTP API   │
                  │ /docs + OpenAPI       │
                  └───────────┬───────────┘
                              │
                 auth / RBAC / CIDR / audit
                              │
                              ▼
                  ┌───────────────────────┐
                  │ Core application      │
                  │ services + domain     │
                  └────┬─────────────┬────┘
                       │             │
              metadata│             │ secret access
                       ▼             ▼
                  PostgreSQL    SecretStore / securemem
                       │             │
                       │             ▼
                       │        HD derivation
                       │             │
                       └─────────────┴───► signing adapters
```

The default database schema is:

```text
vault
```

but operators may choose another safe PostgreSQL identifier, such as:

```text
sigryx_vault
security_keys
```

## Build and publish the image

Build locally:

```bash
docker build -t rajabinekoo/sigryx:v1.0.0 .
```

Push to Docker Hub:

```bash
docker login
docker push rajabinekoo/sigryx:v1.0.0
```

Optionally tag the same release as `latest`:

```bash
docker tag rajabinekoo/sigryx:v1.0.0 rajabinekoo/sigryx:latest
docker push rajabinekoo/sigryx:latest
```

For production consumers, prefer an explicit version tag instead of relying on `latest`.

## Development without Docker

Native development is still supported for contributors who want it. It requires Go, libsodium, PostgreSQL and Atlas on the host. See the [development guide](./docs/src/content/docs/contributing/development.md).

## Security

Please do not report vulnerabilities through public GitHub issues. Follow [SECURITY.md](./SECURITY.md) for responsible disclosure.

## License

See [LICENSE](./LICENSE).
