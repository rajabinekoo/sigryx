---
title: Install & run
description: Local prerequisites, database startup, migrations, and starting the Sigryx server.
---

Sigryx is a Go service backed by PostgreSQL. The repository also includes a standalone documentation site under `docs/`.

## Prerequisites

For the current source tree you need:

- Go **1.26.5** or a compatible newer Go toolchain;
- PostgreSQL 16 or compatible PostgreSQL;
- Atlas CLI for migrations;
- libsodium development/runtime libraries on platforms where the secure-memory implementation requires them;
- Docker + Docker Compose if you want to use the included PostgreSQL development service;
- Node.js for building the public documentation site only.

## Clone

```bash
git clone https://github.com/rajabinekoo/sigryx.git
cd sigryx
```

## Configure environment

Start from the example file:

```bash
cp .example.env .env
```

At minimum, review these values before starting:

```dotenv
POSTGRES_DSN=postgres://sigryx:sigryx@localhost:5432/sigryx?sslmode=disable
HTTP_ADDR=:8080
SETUP_TOKEN=replace-with-at-least-32-random-characters
AUTH_JWT_SECRET=replace-with-at-least-32-random-characters
MAX_UNSEAL_SIZE=10
```

Do not reuse the sample secrets outside local development.

Generate strong random values, for example:

```bash
openssl rand -hex 32
```

`SETUP_TOKEN` is used only by the one-time bootstrap endpoint. `AUTH_JWT_SECRET` is independent of the Vault Encryption Key and HD key material.

## Start PostgreSQL

The current `deployments/docker-compose.yml` provisions PostgreSQL for development:

```bash
make infra-up
```

Check it:

```bash
docker compose -f deployments/docker-compose.yml ps
```

The compose file currently starts **PostgreSQL only**. It is not a complete production deployment of Sigryx.

## Apply migrations

```bash
make migrate-up
```

Useful migration commands:

```bash
make migrate-status
make migrate-lint
```

Sigryx uses Atlas-managed SQL migrations under `migrations/`.

## Run the server

```bash
make run-server
```

With the default configuration the server listens on:

```text
http://localhost:8080
```

Check health:

```bash
curl http://localhost:8080/v1/health
```

## Live HTTP documentation

Once the server is running, browse:

```text
http://localhost:8080/docs
```

This is not the static documentation site in the repository. It is an interactive REST API client served by the **running Sigryx process** and backed by the process's generated OpenAPI document at `/openapi.json`.

## Run the documentation site locally

In another terminal:

```bash
make docs-install
make docs-dev
```

Then browse:

```text
http://localhost:4321
```

The documentation site explains architecture and operating behavior; the server-side `/docs` page is the authoritative interactive view of the actual HTTP schema in the running build.
