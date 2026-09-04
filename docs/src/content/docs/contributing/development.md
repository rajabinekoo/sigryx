---
title: Development guide
description: Repository layout, local workflow, tests, migrations, and contribution rules for Sigryx developers.
---

This page is for engineers modifying Sigryx itself. If you only want to run the service, start with [5-minute usage](/usage/) and [Install & run](/getting-started/installation/).

## Repository map

```text
cmd/                         process entry point and dependency composition
internal/
  adapter/
    in/http/                 Gin + Huma HTTP adapter, auth/audit middleware
    out/blockchain/          blockchain adapters such as the current EVM adapter
    out/persistence/         PostgreSQL repositories
  config/                    environment-backed configuration
  core/
    domain/                  business/security domain types and invariants
    port/in/                 inbound use-case interfaces
    port/out/                outbound repository/adapter interfaces
    service/                 application/core services
  ent/                       Ent schemas/generated persistence helpers
migrations/                  Atlas migration history
pkg/
  cryptox/                   encryption/signature helpers
  hdwallet/                  HD derivation primitives
  securemem/                 protected-memory secret abstraction
deployments/                 local infrastructure manifests
docs/                        this Starlight documentation site
.github/workflows/           CI/deployment workflows
```

The intended dependency direction is:

```text
HTTP / external adapters
          |
          v
      port/in
          |
          v
    core services
      /       \
 domain     port/out
              |
              v
        persistence / chain adapters
```

Core services should not import Gin, Huma, pgx, PostgreSQL-specific types, or concrete blockchain adapters.

## Local prerequisites

The current `go.mod` declares Go `1.26.5`. Use a matching toolchain when running the full repository checks.

You will also need:

- PostgreSQL;
- Atlas for the migration Make targets;
- libsodium/runtime dependencies required by the protected-memory implementation;
- Node.js 24+ only when working on the static documentation site.

For a local database:

```bash
make infra-up
```

The checked-in Compose file is intended to provide PostgreSQL for development. It is not a complete production Sigryx topology.

## Configure the process

```bash
cp .example.env .env
```

At minimum, set the required database/auth/vault values described in [Configuration](/operations/configuration/).

Do not commit real values for:

```text
AUTH_JWT_SECRET
SETUP_TOKEN
POSTGRES_DSN credentials
recovery keys
owner-held unseal credentials
service-account client secrets
```

## Database migrations

Inspect migration status:

```bash
make migrate-status
```

Apply migrations:

```bash
make migrate-up
```

Create a new migration through the repository's existing Atlas workflow rather than hand-editing a previously released migration.

Migration rules for security-sensitive tables:

1. preserve append-only constraints unless the design explicitly requires a controlled exception;
2. keep migration and Ent schema changes consistent;
3. make destructive behavior bounded and observable;
4. avoid silently dropping encrypted or audit data;
5. test upgrade behavior from the previous released schema.

After changing an Ent schema:

```bash
make ent-generate
```

Review generated changes before committing them.

## Run Sigryx

```bash
make run-server
```

The default HTTP address is:

```text
http://localhost:8080
```

Useful development endpoints:

```text
GET /v1/health
GET /docs
GET /openapi.json
```

`/docs` is the live interactive REST API client served by the same Sigryx process. When adding or changing HTTP operations, inspect this page after starting the server to make sure the generated contract says what you intended.

## Run checks

The Makefile exposes the primary quality gates:

```bash
make fmt
make vet
make test
make test-race
make check
```

For a focused package during development:

```bash
go test ./internal/core/service/...
```

For a focused test:

```bash
go test ./internal/core/service/... -run TestName -count=1
```

Do not use `time.Sleep` to test retention or expiry behavior when the component exposes/injects a clock. Freeze time in the test and assert the exact cutoff instead.

## Security-sensitive coding rules

### Secrets are owned values

Do not turn a long-lived secret into an ordinary `[]byte` field merely because an API accepts bytes.

Prefer the repository's protected secret abstraction:

```go
secret.WithBytes(func(raw []byte) error {
    // Use raw only inside this callback.
    return nil
})
```

Never:

- log secret bytes;
- serialize protected secret objects;
- put secret material in `%+v` debugging;
- retain callback slices after the callback;
- copy the Vault Encryption Key into a service field;
- return private keys from an HTTP response.

See [Secure memory](/security/secure-memory/).

### Sealed means unavailable

Any operation requiring the Vault Encryption Key or a plaintext root seed must fail while sealed. Public-metadata operations can intentionally work while sealed when the implementation does not need secret state.

When adding a new use case, explicitly decide which category it belongs to and document it.

### Authentication is fail-closed

A new protected HTTP route must have an explicit permission mapping. If a route is omitted from authorization policy, it should not accidentally become broadly callable.

When adding a new capability:

1. add the domain permission constant and definition;
2. map the route to that permission;
3. add tests for allowed and denied principals;
4. classify its audit retention behavior;
5. update [Permissions](/reference/permissions/) and [HTTP API](/reference/http-api/).

### Audit security-relevant actions

Authentication, access changes, vault lifecycle operations, signing, recovery, and integrity incidents are security-relevant. A new critical operation should not bypass the audit middleware/service without an explicit reason.

## Adding a blockchain adapter

The current built-in adapter is the EVM/Ethereum wallet profile. The core deliberately keeps `KeyRoot` blockchain-agnostic.

A new adapter should implement the outbound wallet adapter contract and own chain-specific concerns such as:

- wallet-type mapping;
- derivation scheme compatibility;
- BIP44 coin type/path construction;
- public key/address encoding;
- chain-specific signing/verification semantics.

Do not put chain-specific address logic in the key-root service.

When adding an adapter, add vectors/tests for:

```text
seed -> derivation path
seed/path -> public key
public key -> address
signing digest -> signature
signature/public key -> verification
```

Use deterministic test vectors where possible.

## Adding an HTTP operation

A typical operation lives in `internal/adapter/in/http` and delegates to a port/in interface.

Checklist:

1. define request/response structs with Huma tags;
2. give the operation a stable `OperationID`;
3. document fields with `doc` tags where semantics are non-obvious;
4. translate domain errors through the shared HTTP translator;
5. wire authorization;
6. verify auditing and retention classification;
7. add HTTP tests;
8. inspect `/openapi.json` and `/docs` locally;
9. update the static documentation when behavior is user-visible.

The generated OpenAPI is part of the developer contract. Treat incorrect schemas as a bug.

## Error handling

Core/domain errors should carry meaning without knowing about HTTP. The HTTP adapter converts them to status codes and redacted problem responses.

Do not return internal encryption/database errors directly to clients. If a corruption or cryptographic invariant fails, log enough non-secret context for operators and return the sanitized external error selected by the translator.

See [Error behavior](/reference/errors/).

## Concurrency and idempotency

Security infrastructure is routinely called concurrently. Prefer database constraints and transaction semantics over check-then-insert assumptions.

For example, wallet creation is intentionally idempotent for the tuple represented by the current implementation, so concurrent attempts must converge on the same durable wallet rather than allocating multiple child indexes.

Tests for write paths should consider:

- exact retry of a successful request;
- two concurrent creators;
- transaction rollback after partial work;
- process restart;
- sealed transition during/around an operation;
- uniqueness conflicts.

## Updating docs with code

A feature PR is not complete when its external behavior changes but the docs still describe the previous behavior.

At minimum update:

- the relevant concept/operations page;
- [HTTP API](/reference/http-api/) for REST changes;
- [Permissions](/reference/permissions/) for authorization changes;
- [Configuration](/operations/configuration/) for env changes;
- [Audit & retention](/operations/audit-retention/) when adding security actions;
- [5-minute usage](/usage/) only when the golden path changes.

See [Documentation guide](/contributing/documentation/).

## Pull-request review checklist

Before opening a PR, ask:

```text
[ ] Does it compile with the repository Go toolchain?
[ ] Are unit/integration tests included?
[ ] Does race-sensitive code have race coverage where useful?
[ ] Are migrations forward-safe?
[ ] Are secrets protected and destroyed at the right lifecycle boundary?
[ ] Is sealed/unsealed behavior explicit?
[ ] Is authorization fail-closed?
[ ] Is the operation audited/classified correctly?
[ ] Is OpenAPI accurate?
[ ] Are static docs updated?
[ ] Are logs free of secret/private material?
```
