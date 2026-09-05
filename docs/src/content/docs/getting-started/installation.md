---
title: Install & run
description: Run the published Sigryx image directly or place it beside PostgreSQL in Docker Compose.
---

Sigryx is distributed as a **single Docker image**. The image contains:

- the Sigryx server;
- Atlas;
- the exact versioned migrations for that release;
- the libsodium runtime used by secure memory.

For normal product usage you do **not** need Go, Atlas, libsodium, or the Sigryx source tree on the host. You only need Docker and a reachable PostgreSQL database.

The examples below use the pinned image:

```text
rajabinekoo/sigryx:latest
```

Prefer a release tag in real deployments instead of `latest`.

## Option 1: run the image directly

Use this when PostgreSQL already exists and is reachable from Docker.

First pull the image and create two independent application secrets:

```bash
docker pull rajabinekoo/sigryx:latest

export SIGRYX_SETUP_TOKEN="$(openssl rand -hex 32)"
export SIGRYX_AUTH_JWT_SECRET="$(openssl rand -hex 32)"
```

If PostgreSQL is running on the Docker host, the following command works on Linux by mapping `host.docker.internal` to the Docker host gateway. Docker Desktop already understands this hostname as well.

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

Change the PostgreSQL username, password, database, host, and SSL mode for your environment.

The container starts in this order:

```text
container starts
      │
      ├─ ensure POSTGRES_SCHEMA exists
      ├─ replay/apply pending Atlas migrations
      └─ start Sigryx only after migrations succeed
```

Check it:

```bash
curl http://localhost:8080/v1/health
```

Open the live REST API client:

```text
http://localhost:8080/docs
```

OpenAPI is available at:

```text
http://localhost:8080/openapi.json
```

:::tip
If PostgreSQL is another container, put both containers on the same Docker network and use that container/service name in `POSTGRES_DSN` instead of `host.docker.internal`.
:::

## Option 2: Docker Compose with PostgreSQL

This is the easiest option for a new application or local environment. Put Sigryx **next to PostgreSQL in your own Compose file**. No Sigryx repository checkout is required.

Save the following as `compose.yml`:

```yaml
name: sigryx

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: sigryx
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: sigryx
    volumes:
      - sigryx-postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sigryx -d sigryx"]
      interval: 5s
      timeout: 5s
      retries: 20
      start_period: 5s

  sigryx:
    image: rajabinekoo/sigryx:latest
    restart: unless-stopped
    init: true
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      HTTP_ADDR: 0.0.0.0:8080

      POSTGRES_DSN: postgres://sigryx:${POSTGRES_PASSWORD}@postgres:5432/sigryx?sslmode=disable
      POSTGRES_SCHEMA: ${POSTGRES_SCHEMA:-sigryx_vault}
      POSTGRES_AUTO_MIGRATE: "true"

      SETUP_TOKEN: ${SIGRYX_SETUP_TOKEN}
      AUTH_JWT_SECRET: ${SIGRYX_AUTH_JWT_SECRET}
    ports:
      - "${SIGRYX_PORT:-8080}:8080"
    cap_drop:
      - ALL
    cap_add:
      - IPC_LOCK
    security_opt:
      - no-new-privileges:true
    ulimits:
      memlock:
        soft: -1
        hard: -1
    read_only: true
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=32m

volumes:
  sigryx-postgres-data:
```

Create a `.env` file beside it:

```bash
cat > .env <<'ENV'
POSTGRES_PASSWORD=replace-with-a-random-password
SIGRYX_SETUP_TOKEN=replace-with-a-random-setup-token
SIGRYX_AUTH_JWT_SECRET=replace-with-a-random-jwt-secret
POSTGRES_SCHEMA=sigryx_vault
SIGRYX_PORT=8080
ENV
```

Generate values with `openssl rand -hex 32` and replace the three placeholders before starting.

Start both services:

```bash
docker compose up -d
```

Verify:

```bash
docker compose ps
curl http://localhost:8080/v1/health
```

Then open:

```text
http://localhost:8080/docs
```

That Compose file is also a good starting point for an existing backend repository: merge the `sigryx` service into your existing Compose file and point it at your existing PostgreSQL service.

## Use an existing PostgreSQL service in your Compose project

If your application already has a PostgreSQL service, you do not need a second database container.

For example:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    # your existing PostgreSQL configuration

  sigryx:
    image: rajabinekoo/sigryx:latest
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      HTTP_ADDR: 0.0.0.0:8080
      POSTGRES_DSN: postgres://app:${POSTGRES_PASSWORD}@postgres:5432/backend?sslmode=disable
      POSTGRES_SCHEMA: sigryx_vault
      POSTGRES_AUTO_MIGRATE: "true"
      SETUP_TOKEN: ${SIGRYX_SETUP_TOKEN}
      AUTH_JWT_SECRET: ${SIGRYX_AUTH_JWT_SECRET}
    ports:
      - "9095:8080"
    cap_drop:
      - ALL
    cap_add:
      - IPC_LOCK
    security_opt:
      - no-new-privileges:true
    ulimits:
      memlock:
        soft: -1
        hard: -1
```

Sigryx keeps its application objects under `POSTGRES_SCHEMA`, so a shared database can remain cleanly namespaced:

```text
backend database
│
├── other application schemas
│
└── sigryx_vault
    ├── key_roots
    ├── wallets
    ├── signing_records
    ├── audit_events
    └── ...
```

Container-to-container calls should use Docker DNS:

```text
http://sigryx:8080
```

The published host port, such as `localhost:9095`, is only required for callers outside the Compose network.

## Required configuration

The minimum configuration for a useful Sigryx deployment is:

```dotenv
POSTGRES_DSN=postgres://user:password@host:5432/database?sslmode=disable
POSTGRES_SCHEMA=sigryx_vault
POSTGRES_AUTO_MIGRATE=true
HTTP_ADDR=0.0.0.0:8080
SETUP_TOKEN=<at-least-32-characters>
AUTH_JWT_SECRET=<independent-secret-at-least-32-bytes>
```

Generate the application secrets independently:

```bash
openssl rand -hex 32
openssl rand -hex 32
```

`SETUP_TOKEN` authorizes the one-time `POST /v1/setup` bootstrap request. `AUTH_JWT_SECRET` signs Sigryx JWTs. Neither value is the Vault Encryption Key or an unseal credential.

## PostgreSQL schema

Sigryx defaults to:

```text
vault
```

but a consuming project can choose another schema:

```dotenv
POSTGRES_SCHEMA=sigryx_vault
```

Runtime connections use:

```text
<POSTGRES_SCHEMA>,pg_catalog
```

as the PostgreSQL `search_path`. Sigryx application queries do not rely on `public` as a fallback.

The image automatically creates the configured schema when automatic migration is enabled, then lets Atlas apply the versioned migration history. Historical Sigryx migrations that predate configurable schemas are replayed compatibly and current application objects end up in `POSTGRES_SCHEMA`.

:::caution
Treat `POSTGRES_SCHEMA` as persistent deployment configuration. Do not casually rename it after a production database has been initialized.
:::

## Ports and bind addresses

`HTTP_ADDR` controls the real listener **inside the Sigryx container**:

```dotenv
HTTP_ADDR=0.0.0.0:8080
```

Docker decides which host port maps to that listener:

```yaml
ports:
  - "9090:8080"
```

That means:

```text
host localhost:9090 → container 0.0.0.0:8080
```

For direct `docker run`, if you change the internal listener to `0.0.0.0:9091`, publish the same container port:

```bash
docker run ... \
  -e HTTP_ADDR=0.0.0.0:9091 \
  -p 9091:9091 \
  rajabinekoo/sigryx:latest
```

## Automatic migrations

The image owns its migration lifecycle by default:

```dotenv
POSTGRES_AUTO_MIGRATE=true
```

On startup, the image runs Atlas and only starts Sigryx after migration succeeds.

Set:

```dotenv
POSTGRES_AUTO_MIGRATE=false
```

when your production deployment pipeline manages DDL separately.

## Upgrade the image

With Compose, upgrade deliberately to a newer release tag:

```yaml
sigryx:
  image: rajabinekoo/sigryx:v1.0.2
```

Then:

```bash
docker compose pull sigryx
docker compose up -d sigryx
```

The new image contains the migrations that belong to that release and applies pending ones before starting.

## Build from source

Cloning the Sigryx repository is only necessary when you want to develop Sigryx itself.

Contributor workflow:

```bash
git clone https://github.com/rajabinekoo/sigryx.git
cd sigryx
cp .env.example .env
docker compose up --build -d
```

See [Development guide](/contributing/development/) for native Go development.
