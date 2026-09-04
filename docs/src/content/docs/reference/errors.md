---
title: Error behavior
description: HTTP status mapping, common conflict states, and safe client retry guidance.
---

Sigryx translates domain/infrastructure errors into HTTP semantics at the inbound adapter boundary.

The live Huma/OpenAPI client at `/docs` is the best place to inspect the concrete response schema for the running release.

## Status categories

### `400 Bad Request`

Used for malformed or invalid caller input, including examples such as:

- invalid unseal count;
- unsupported wallet type;
- missing/invalid wallet or key-root input;
- invalid transaction/typed data/signature encoding;
- missing signing context;
- invalid generic signing format;
- invalid JSON/integrity field selection;
- invalid pagination;
- invalid role/permission/CIDR/name;
- malformed recovery key/backup.

Client action: fix the request; blind retry is usually not useful.

### `401 Unauthorized`

Used for authentication failures, for example:

- invalid unseal credential;
- invalid setup token;
- invalid username/password;
- invalid/expired/revoked refresh session;
- invalid current password.

Client action: reauthenticate or correct credentials. Do not turn repeated credential failures into high-frequency retry loops.

### `403 Forbidden`

Used when identity is known but policy blocks the action, for example:

- missing RBAC permission;
- source IP outside allowlist;
- inactive principal;
- root-admin-only constraint;
- user-only constraint;
- attempts to mutate protected Root Admin state through ordinary access APIs.

Client action: change authorization/configuration, not payload formatting.

### `404 Not Found`

Used for missing resources such as:

- key root;
- wallet;
- user;
- role;
- service account;
- session.

Client action: verify identifier and provisioning lifecycle.

### `409 Conflict`

Used for state/invariant conflicts, including:

- Vault already initialized;
- Vault sealed when secret material is required;
- duplicate unseal slot submission;
- Vault already unsealed / configuration locked;
- wallet/key-root conflicts;
- recovery root conflict;
- no key roots available for recovery export;
- integrity Signing Record schema/value/wallet mismatch;
- Signing Record tamper detection during a reuse path.

A conflict often means the request is syntactically valid but incompatible with established durable state.

### `429 Too Many Requests`

Reserved by the shared error taxonomy for rate-limited failures. Whether a specific route emits it depends on the implementation in the running release.

### `503 Service Unavailable`

Used for optional/unavailable service capabilities, such as:

- setup disabled by configuration;
- integrity signing dependencies unavailable.

### `500 Internal Server Error`

Used for unexpected internal errors or intentionally redacted corruption failures. Sigryx avoids returning certain sensitive internal details directly.

## Request correlation

Every HTTP response is assigned:

```text
X-Request-ID
```

Log this ID in the caller. It is the safest primary key for correlating client failures with Sigryx logs and audit events without copying sensitive request bodies into observability systems.

## Retry guidance

### Safe retry candidates

With normal exponential backoff, retry transient transport/5xx failures where the business operation is safe to repeat.

Wallet creation is designed to resolve the existing wallet for the same stable `(key_root_id, adapter, user_id)` tuple.

Integrity signing also has deliberate replay behavior: an exact repeat for the same immutable object returns the original signature with `reused=true`.

### Do not blindly retry

Avoid blind retries for:

```text
400
401
403
409 caused by integrity mismatch
```

These represent deterministic caller/policy/state problems.

## Security-sensitive failures

Integrity verification can return HTTP success with a structured negative verification result, for example:

```json
{
  "valid":false,
  "signature_valid":true,
  "record_match":false,
  "digest":"0x...",
  "reason":"INTEGRITY_VALUE_MISMATCH"
}
```

This is not a transport/API error. It is a cryptographic/application verification result and should be handled as a security-relevant business outcome.
