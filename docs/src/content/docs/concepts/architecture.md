---
title: Architecture
description: Sigryx runtime boundaries, ports/adapters structure, secret ownership, persistence, and request flow.
---

Sigryx is organized as a modular monolith with a ports-and-adapters structure. The HTTP API, PostgreSQL persistence, Ethereum adapter, alerting, and secure-memory implementation are replaceable boundaries around a small set of core services.

## Runtime overview

```text
                       ┌───────────────────────────────┐
                       │         API consumers          │
                       │ humans / services / backends  │
                       └───────────────┬───────────────┘
                                       │ HTTPS/HTTP
                                       ▼
┌──────────────────────────────────────────────────────────────────┐
│                         Sigryx process                            │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Gin + Huma HTTP adapter                                   │  │
│  │ request metadata → audit → auth/RBAC → route handler      │  │
│  └──────────────────────────┬─────────────────────────────────┘  │
│                             │                                    │
│  ┌──────────────────────────▼─────────────────────────────────┐  │
│  │ Core services                                             │  │
│  │ Auth · Access · Seal · KeyRoot · Wallet · Signing         │  │
│  │ Recovery · Audit · AuditRetention                         │  │
│  └──────────────┬──────────────────────────┬──────────────────┘  │
│                 │                          │                     │
│       ┌─────────▼─────────┐      ┌─────────▼────────────────┐   │
│       │ SecretStore       │      │ Outbound adapters        │   │
│       │ secure runtime    │      │ PostgreSQL · Ethereum    │   │
│       │ secret ownership  │      │ webhook alert sink       │   │
│       └─────────┬─────────┘      └─────────┬────────────────┘   │
│                 │                          │                     │
│       ┌─────────▼─────────┐                │                     │
│       │ securemem.Secret  │                │                     │
│       │ libsodium-backed  │                │                     │
│       │ protected pages   │                │                     │
│       └───────────────────┘                │                     │
└────────────────────────────────────────────┼─────────────────────┘
                                             │
                                    ┌────────▼────────┐
                                    │   PostgreSQL    │
                                    │ encrypted roots │
                                    │ wallets/public  │
                                    │ auth/audit/etc. │
                                    └─────────────────┘
```

## Repository layout

The most important boundaries are:

```text
cmd/                         process composition and lifecycle
internal/adapter/in/http/    REST adapter, middleware, OpenAPI
internal/adapter/out/        PostgreSQL, Ethereum, alert adapters
internal/core/domain/        domain types and policies
internal/core/port/in/       use-case interfaces
internal/core/port/out/      repository/adapter interfaces
internal/core/service/       application/domain services
internal/ent/                Ent schemas/generated persistence types
pkg/securemem/               protected-memory abstraction
pkg/secretstore/             runtime secret ownership
pkg/cryptox/                 AES-256-GCM and randomness helpers
pkg/hdseed/                  protected HD master-seed generation
pkg/hdwallet/                BIP32 derivation primitives
pkg/canonicaljson/           canonical JSON + JSON Pointer selection
pkg/signingrecord/           encrypted integrity record format
pkg/recovery/                independent recovery bundle format
migrations/                  Atlas migration history
docs/                        public engineering documentation site
```

## Process composition

`cmd/main.go` composes the application in this order:

1. initialize secure-memory support;
2. load environment configuration;
3. initialize structured logging;
4. create signal-aware root context;
5. connect PostgreSQL;
6. create repositories;
7. create the process-local `SecretStore`;
8. create core services;
9. create the HTTP adapter;
10. start the HTTP server and audit-retention worker under an `errgroup`;
11. on shutdown, stop services and clear secret material.

The important architectural property is that the `SecretStore` is created **once** and shared only with services that require secret access.

## Persistent vs. runtime data

### Persisted

PostgreSQL stores durable state such as:

- wrapped unseal-key material and server-side key material;
- encrypted HD key-root seeds;
- wallet IDs, user IDs, derivation paths, public keys, addresses;
- users, roles, service accounts, session hashes;
- encrypted Signing Records;
- audit events.

### Never intentionally persisted as plaintext

Sigryx does not intentionally persist:

- owner-held unseal keys;
- recovered real unseal keys;
- the runtime Vault Encryption Key;
- plaintext HD root seeds;
- child private keys;
- plaintext passwords;
- plaintext service-account secrets;
- plaintext refresh tokens;
- recovery keys or exported recovery backups after the response is returned.

## SecretStore boundary

`pkg/secretstore.Store` owns runtime secret state.

When sealed, the store does not contain a Vault Encryption Key. When unsealed, it owns a `securemem.Secret` containing that key and may also cache plaintext key-root seeds as `securemem.Secret` values.

Access is callback based:

```go
secrets.WithVaultEncryptionKey(func(key []byte) error {
    // key is valid only for this callback
    return nil
})
```

and:

```go
secrets.WithKeyRootSeed(rootID, func(seed []byte) error {
    // seed is valid only for this callback
    return nil
})
```

This design discourages long-lived heap copies of high-value secrets.

## HTTP request path

A protected request generally travels through:

```text
request
  │
  ├─ requestMetadataMiddleware
  │    ├─ generate X-Request-ID
  │    └─ resolve client IP
  │
  ├─ auditMiddleware
  │    └─ records result after downstream handlers complete
  │
  ├─ gin.Recovery
  │
  ├─ authMiddleware
  │    ├─ public-route bypass OR bearer validation
  │    ├─ session/service validation
  │    ├─ CIDR policy
  │    └─ route permission / root-only policy
  │
  └─ Huma handler
       └─ core service
```

The audit middleware records the final status and principal metadata after the request has been processed. Dedicated integrity incidents can also append critical audit events directly from the signing service.

## Persistence model

The code intentionally separates repositories from services. Some repositories use Ent and others use pgx directly where transaction/control requirements are easier to express in SQL.

Examples:

- key-root and unseal-slot persistence use Ent-backed repositories;
- wallet allocation uses explicit PostgreSQL logic for deterministic counters and concurrency;
- audit retention uses a dedicated PostgreSQL function because `audit_events` is otherwise append-only.

## Blockchain adapter boundary

The current composition wires one Ethereum adapter. Core wallet/signing services depend on interfaces rather than directly on Ethereum package internals.

The Ethereum adapter is responsible for:

- BIP44 Ethereum derivation path selection;
- secp256k1 public/private key operations;
- EIP-55 address generation;
- legacy and EIP-1559 transaction signing/verification;
- EIP-712 typed-data hashing/signing/verification;
- signing a precomputed digest.

This boundary is why the key-root concept itself remains a derivation-scheme object rather than an Ethereum-specific database secret.

## Failure boundaries

A few important failure behaviors:

- PostgreSQL failure prevents durable state operations.
- Vault sealing prevents operations that require secret material, but public metadata and some verification paths can remain usable.
- Alert-webhook failure does not invalidate the integrity result; Sigryx records `security.alert_delivery_failed` as a critical audit event when possible.
- Audit-retention failure is logged and retried on the next interval; it does not intentionally shut down the HTTP service.
- Process shutdown clears runtime secret ownership through `SecretStore.Clear()`.

## Architecture principle for contributors

Before adding a new feature, decide which boundary owns it:

- domain invariant → `internal/core/domain` or a core service;
- use-case contract → `internal/core/port/in`;
- infrastructure dependency → `internal/core/port/out` + adapter;
- HTTP representation → `internal/adapter/in/http`;
- sensitive byte ownership → `securemem.Secret` / `SecretStore`, not a global `[]byte`.

Avoid bypassing these boundaries simply because a value is convenient to reach from `cmd/main.go`.
