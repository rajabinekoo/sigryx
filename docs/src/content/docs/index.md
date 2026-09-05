---
title: Sigryx
description: Self-hosted key and signing infrastructure for application backends.
template: splash
hero:
  title: Sigryx
  tagline: Self-hosted key and signing infrastructure for application backends.
  actions:
    - text: Start in 5 minutes
      link: ./usage/
      icon: right-arrow
      variant: primary
    - text: Run with Docker
      link: ./getting-started/installation/
      icon: right-arrow
      variant: secondary
    - text: GitHub
      link: https://github.com/rajabinekoo/sigryx
      icon: external
      variant: minimal
---

<p align="center">
  <img src="./sigryx-logo.png" alt="Sigryx" width="760" />
</p>

## Pull one image and run

Sigryx is distributed as a single image containing the server, Atlas, the matching migrations, and libsodium:

```bash
docker pull rajabinekoo/sigryx:latest
```

You can run that image against an existing PostgreSQL database with `docker run`, or place it beside PostgreSQL in your application's Docker Compose file.

Minimal topology:

```text
PostgreSQL
    │
    │ POSTGRES_DSN
    ▼
Sigryx image
    ├─ bootstrap configured PostgreSQL schema
    ├─ Atlas migrate apply
    └─ HTTP API + /docs
```

No Go installation, Atlas installation, or Sigryx source checkout is required to **use** the product.

See [Install & run](/getting-started/installation/) for a copy/paste `docker run` command and a complete PostgreSQL + Sigryx Compose example.

After startup, open the live REST API client exposed by the running server:

```text
http://localhost:8080/docs
```

Sigryx is a **self-hosted key and signing service** designed for backend systems that need deterministic accounts, controlled signing, sealed secret lifecycle, auditable access, and recoverable HD key roots without persisting end-user private keys.

The current implementation provides:

- a sealed/unsealed Vault lifecycle;
- N-of-N unseal credentials with split owner/server material;
- encrypted HD key roots;
- deterministic Ethereum wallets using BIP32/BIP44;
- Ethereum transaction signing and verification;
- EIP-712 typed-data signing and verification;
- context-bound generic JSON/RAW signing;
- integrity signing backed by immutable encrypted Signing Records;
- JWT authentication, users, service accounts, roles, permissions, and IP/CIDR allowlists;
- root-admin recovery export/import;
- append-only audit logging with configurable retention classes;
- an automatically generated OpenAPI specification and an interactive REST API client at `/docs` on every running Sigryx server.

## Docker deployment choices

Choose the model that matches your application:

| Situation | Recommended setup |
| --- | --- |
| PostgreSQL already exists | `docker run rajabinekoo/sigryx:<version>` with `POSTGRES_DSN` |
| New/local application | Docker Compose with `postgres` + `sigryx` |
| Existing Compose stack | Add only the `sigryx` service and point it to the existing PostgreSQL service |
| Sigryx contributor | Clone the repository and use the bundled development Compose stack |

Sigryx application tables live under configurable `POSTGRES_SCHEMA`, such as `sigryx_vault`, so the same PostgreSQL database can be shared with another backend while keeping Sigryx objects namespaced.

## Two kinds of documentation

Sigryx exposes two complementary documentation surfaces:

1. **This documentation site** explains architecture, security properties, lifecycle, deployment, and recommended usage.
2. **The live REST API client** is served by Sigryx itself at `http://<sigryx-host>:<port>/docs`. It is generated from the actual HTTP handlers and reads the live OpenAPI document from `/openapi.json`.

For a default Docker deployment:

```text
Live API client:      http://localhost:8080/docs
OpenAPI document:     http://localhost:8080/openapi.json
Sigryx REST API:      http://localhost:8080/v1/...
```

The live API client is the best place to inspect exact request/response schemas. This site is the best place to understand **why** the API behaves the way it does.

## Core lifecycle

A typical deployment follows this order:

```text
start container
    │
    ├─ apply pending database migrations
    ├─ health endpoint becomes available
    ├─ bootstrap Root Admin once
    ├─ authenticate
    ├─ initialize Vault once
    │      └─ receive N owner-held credentials exactly once
    ├─ submit all N credentials
    │      └─ Vault Encryption Key exists only in protected runtime memory
    ├─ create HD key root(s)
    ├─ create deterministic wallets
    ├─ sign application payloads / transactions
    └─ seal Vault
           └─ destroy runtime secret material
```

## Important mental model

Sigryx does **not** store one private key per wallet. A persisted wallet contains public metadata such as its derivation path, public key, and address. When signing is requested, Sigryx uses the appropriate protected HD root seed to derive the private key for that wallet for the duration of the signing callback, then discards the derived secret.

The durable secret boundary is therefore centered around encrypted **key roots**, not persisted child private keys.

## Current implementation scope

The documentation in this repository describes the code that exists in the current tree. At the moment, the built-in blockchain adapter is Ethereum-compatible and the exposed derivation scheme is BIP32/secp256k1 with Ethereum BIP44 paths. Do not infer support for additional chains, quorum/distributed signing, HSM-backed roots, or an intent-aware policy engine unless those features appear in the code and API of the release you are running.

## Where to go next

- Want a copy/paste Docker deployment? Read [Install & run](/getting-started/installation/).
- Want the shortest path from zero to a signature? Read [5-minute usage](/usage/).
- Want to understand how secrets are constructed and destroyed? Read [Seal & unseal](/concepts/seal-unseal/) and [Secure memory](/security/secure-memory/).
- Want to embed Sigryx in an application? Read [Signing](/concepts/signing/) and the [HTTP API reference](/reference/http-api/).
- Want to operate Sigryx in production? Read [Production deployment](/operations/production/), [Audit & retention](/operations/audit-retention/), and [Recovery](/operations/recovery/).
