---
title: First boot
description: The state transitions and one-time actions required on a fresh Sigryx database.
---

A new Sigryx database begins with two independent bootstrap concerns:

1. **authentication bootstrap** — create the single Root Admin;
2. **Vault bootstrap** — create the N-of-N unseal credentials.

The Root Admin should be created first because Vault initialization is an authenticated, permission-protected endpoint.

## Initial state

On a fresh database:

```text
Auth:  no Root Admin
Vault: UNINITIALIZED
```

The process itself can still start because authentication secrets and Vault secrets are separate concerns.

## Step 1: bootstrap Root Admin

`POST /v1/setup` requires:

```text
X-Sigryx-Setup-Token: <SETUP_TOKEN>
```

If `SETUP_TOKEN` is empty, setup is disabled. If a Root Admin already exists, setup cannot be repeated.

The endpoint returns a random username and password exactly as response data. Sigryx stores the password as an Argon2id hash, not plaintext.

After bootstrap, authenticate with `/v1/auth/login` and keep the access token for the remaining steps.

## Step 2: initialize the Vault

Call:

```http
POST /v1/vault/init
Authorization: Bearer <root-admin-access-token>
Content-Type: application/json

{"unseal_key_count":3}
```

The count must be at least 1 and cannot exceed `MAX_UNSEAL_SIZE` when that limit is configured.

Initialization is one-time. Sigryx refuses a second initialization once unseal slots exist.

## Step 3: distribute owner credentials

Each returned credential contains:

- `slot_id` — stable unseal slot number;
- `unseal_payload` — wrapped unseal-key payload, base64url encoded;
- `owner_key` — owner-held secret, base64url encoded.

The `owner_key` is **never persisted by Sigryx**.

A production operator should treat initialization output as a high-sensitivity ceremony. Avoid shell history, terminal recording, centralized logs, chat, tickets, CI output, and plaintext shared storage.

## Step 4: unseal

Submit each credential to `/v1/vault/unseal`.

The implementation is N-of-N:

```text
required = N
submitted < N  -> SEALED
submitted = N  -> UNSEALED
```

The Vault Encryption Key is derived only after all unique configured slots have been accepted.

## Step 5: create key roots

Key-root creation requires the Vault to be unsealed because a new plaintext HD master seed is generated directly in protected memory and immediately encrypted for persistence.

The built-in wallet profile currently accepts:

```json
{"wallet_type":"ETHEREUM"}
```

## Step 6: create wallets

A wallet is resolved by the tuple:

```text
key_root_id + adapter + user_id
```

The same tuple returns the same persisted wallet. A new user under the same root receives the next deterministic derivation index.

## Step 7: create application identities

For machine-to-machine use, avoid sharing the Root Admin token with application workloads. Create a restricted role and a service account with only the permissions that workload needs.

See [Auth, RBAC & network policy](/security/auth-rbac-network/) and [Permissions](/reference/permissions/).

## Restart behavior

A process restart clears all in-memory secret state. Persisted encrypted data remains, but the Vault starts sealed again:

```text
process restart
   │
   ├─ persisted unseal slots still exist
   ├─ encrypted key roots still exist
   ├─ wallets/public keys still exist
   └─ runtime Vault Encryption Key does not exist
             │
             └─ submit all N credentials again
```

This is expected behavior, not data loss.
