---
title: Troubleshooting
description: Common startup, authentication, sealed-state, wallet, signing, audit, and documentation failures.
---

## `vault is sealed`

A signing/key-creation/recovery operation returned a conflict mentioning sealed state.

Check:

```bash
curl -sS "$SIGRYX/v1/vault/status" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" | jq
```

If `SEALED`, submit every N-of-N unseal credential.

A process restart intentionally loses runtime Vault key material.

## `vault is not initialized`

Initialize once with:

```text
POST /v1/vault/init
```

You must already be authenticated/authorized.

## setup fails

### Setup disabled

`SETUP_TOKEN` is empty.

### Invalid setup token

Ensure `X-Sigryx-Setup-Token` exactly matches the configured token.

### Already setup

The database already has a Root Admin. Do not reinitialize the authentication system; use the existing Root Admin credentials or your operational recovery process.

## `authentication required`

Protected endpoints need:

```text
Authorization: Bearer <access_token>
```

Do not send the refresh token as an access token.

## `permission denied`

The principal authenticated successfully but its role lacks the route's permission.

Inspect:

```text
GET /v1/access/permissions
GET /v1/access/roles
```

with an identity allowed to manage roles.

## `IP not allowed`

The resolved source IP is outside the principal's CIDR list.

If running behind a proxy, verify `TRUSTED_PROXY_CIDRS`. Do not "fix" the issue by trusting every proxy/source globally; forwarded headers are security-sensitive.

## Wallet creation works while sealed sometimes

This can happen when the exact `(key_root_id, adapter, user_id)` wallet already exists. Sigryx returns persisted public metadata before needing derivation.

A **new** wallet requires unsealed secret access.

## Verification works while sealed but signing does not

Expected for transaction, EIP-712, and generic verification: they need only the persisted public key.

Integrity verification is different and requires unsealing because it must decrypt the historical Signing Record.

## Integrity signing returns conflict

For the same `(context, object_id)`, the first successful record freezes wallet/schema/protected values.

Typical causes:

```text
protected field changed
integrity_fields changed
wallet_id changed
stored record failed cryptographic validation
```

Do not delete/rewrite the Signing Record to make a legitimate conflict disappear. Correct the application object identity/versioning model.

## Audit retention is running but deletes zero rows

Check the actual class and age:

```sql
SELECT
  retention_class,
  count(*)
FROM audit_events
GROUP BY retention_class;
```

Then inspect candidate rows:

```sql
SELECT id, action, retention_class, occurred_at, now() - occurred_at AS age
FROM audit_events
ORDER BY occurred_at ASC
LIMIT 100;
```

Remember that `sign.*`, `security.*`, `recovery.*`, and other sensitive families are `CRITICAL` and may use a much longer retention period.

The worker runs once at startup, then every `AUDIT_CLEANUP_INTERVAL`.

## Direct `DELETE FROM audit_events` fails

Expected. The table is append-only during ordinary operation. Retention uses the dedicated PostgreSQL function.

## `/docs` opens blank

The runtime API client currently loads Scalar JavaScript from jsDelivr. Check browser network access to:

```text
https://cdn.jsdelivr.net/npm/@scalar/api-reference
```

Even if the UI cannot load, the OpenAPI JSON should still be available:

```bash
curl -sS "$SIGRYX/openapi.json" | jq
```

## Documentation site paths look wrong on GitHub Pages

The Astro config defaults to:

```text
site = https://rajabinekoo.github.io
base = /sigryx
```

when running in GitHub Actions.

For a custom domain, set build-time variables such as:

```text
SIGRYX_DOCS_SITE=https://docs.example.com
SIGRYX_DOCS_BASE=/
```

and configure the GitHub Pages custom domain/DNS separately.

## PostgreSQL migration mismatch

Run:

```bash
make migrate-status
make migrate-lint
```

Do not manually edit a migration that has already been applied to shared environments. Create a new migration and update `atlas.sum` through the normal Atlas workflow.

## Go toolchain mismatch

The current module declares:

```text
go 1.26.5
```

Use a compatible Go toolchain before assuming a compile/test failure is caused by Sigryx code.
