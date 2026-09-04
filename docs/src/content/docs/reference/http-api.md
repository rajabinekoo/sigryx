---
title: HTTP API
description: Complete route inventory and practical request examples for the current Sigryx REST API.
---

The authoritative machine-readable contract for a running build is always:

```text
GET /openapi.json
```

and the interactive client is:

```text
GET /docs
```

This page documents the current route semantics and gives copyable examples.

Assume:

```bash
export SIGRYX=http://localhost:8080
export TOKEN='<access-token>'
```

For protected endpoints:

```bash
-H "Authorization: Bearer $TOKEN"
```

## Route matrix

| Method | Path | Authentication | Secret-state requirement |
| --- | --- | --- | --- |
| GET | `/v1/health` | public | none |
| POST | `/v1/setup` | setup token header | none |
| POST | `/v1/auth/login` | public credentials | none |
| POST | `/v1/auth/service-token` | public credentials | none |
| POST | `/v1/auth/refresh` | public refresh token | none |
| POST | `/v1/auth/logout` | authenticated USER | none |
| PATCH | `/v1/auth/me` | authenticated USER | none |
| GET | `/v1/vault/status` | `vault.status.read` | none |
| POST | `/v1/vault/init` | `vault.initialize` | must be uninitialized |
| POST | `/v1/vault/unseal` | `vault.unseal` | currently sealed |
| POST | `/v1/vault/seal` | `vault.seal` | initialized |
| GET | `/v1/key-roots` | `keyroot.read` | works sealed |
| POST | `/v1/key-roots` | `keyroot.create` | **unsealed** |
| POST | `/v1/wallets` | `wallet.create` | unsealed only when new derivation is needed |
| POST | `/v1/sign/transaction` | `sign.transaction` | **unsealed** |
| POST | `/v1/verify/transaction` | `verify.transaction` | works sealed |
| POST | `/v1/sign/typed-data` | `sign.typed_data` | **unsealed** |
| POST | `/v1/verify/typed-data` | `verify.typed_data` | works sealed |
| POST | `/v1/sign/data` | `sign.generic` | **unsealed** |
| POST | `/v1/verify/data` | `verify.generic` | works sealed |
| POST | `/v1/sign/integrity` | `sign.integrity` | **unsealed** |
| POST | `/v1/verify/integrity` | `verify.integrity` | **unsealed** |
| GET | `/v1/audit/events` | `audit.read` | none |
| POST | `/v1/recovery/export` | Root Admin only | **unsealed** |
| POST | `/v1/recovery/import` | Root Admin only | **unsealed** |
| GET | `/v1/access/permissions` | `access.roles.manage` | none |
| GET | `/v1/access/roles` | `access.roles.manage` | none |
| POST | `/v1/access/roles` | `access.roles.manage` | none |
| PATCH | `/v1/access/roles/{id}` | `access.roles.manage` | none |
| GET | `/v1/access/users` | `access.users.manage` | none |
| POST | `/v1/access/users` | `access.users.manage` | none |
| PATCH | `/v1/access/users/{id}` | `access.users.manage` | none |
| GET | `/v1/access/service-accounts` | `access.service_accounts.manage` | none |
| POST | `/v1/access/service-accounts` | `access.service_accounts.manage` | none |
| PATCH | `/v1/access/service-accounts/{id}` | `access.service_accounts.manage` | none |
| GET | `/docs` | public | none |
| GET | `/openapi.json` | public | none |

Root Admin bypasses ordinary role-permission membership checks.

---

## Health

### `GET /v1/health`

```bash
curl -sS "$SIGRYX/v1/health" | jq
```

```json
{"message":"it's healthy"}
```

This is process/HTTP health, not unsealed readiness.

---

## Bootstrap and auth

### `POST /v1/setup`

```bash
curl -sS -X POST "$SIGRYX/v1/setup" \
  -H "X-Sigryx-Setup-Token: $SETUP_TOKEN" | jq
```

Response:

```json
{
  "username": "admin_ab12cd...",
  "password": "<one-time-generated-password>"
}
```

### `POST /v1/auth/login`

```bash
curl -sS -X POST "$SIGRYX/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin_x","password":"..."}' | jq
```

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 600
}
```

### `POST /v1/auth/service-token`

```bash
curl -sS -X POST "$SIGRYX/v1/auth/service-token" \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"...","client_secret":"..."}' | jq
```

Service accounts receive an access token but no refresh token in the current implementation.

### `POST /v1/auth/refresh`

```bash
curl -sS -X POST "$SIGRYX/v1/auth/refresh" \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"..."}' | jq
```

The returned refresh token replaces the old one.

### `POST /v1/auth/logout`

```bash
curl -sS -X POST "$SIGRYX/v1/auth/logout" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Response:

```json
{"message":"logged out"}
```

This is a user-session operation; service principals do not have user sessions.

### `PATCH /v1/auth/me`

Example password rotation:

```bash
curl -sS -X PATCH "$SIGRYX/v1/auth/me" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "current_password":"old-password",
    "new_password":"a-new-password-with-12-plus-chars"
  }' | jq
```

Changing password revokes user sessions. A Root Admin may also replace its own `allowed_cidrs` by providing that field.

---

## Vault

### `GET /v1/vault/status`

```bash
curl -sS "$SIGRYX/v1/vault/status" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Possible states:

```text
UNINITIALIZED
SEALED
UNSEALED
```

Response shape:

```json
{
  "state":"SEALED",
  "submitted":1,
  "required":3
}
```

### `POST /v1/vault/init`

```bash
curl -sS -X POST "$SIGRYX/v1/vault/init" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"unseal_key_count":3}' | jq
```

Response contains one-time credentials:

```json
{
  "state":"SEALED",
  "credentials":[
    {
      "slot_id":1,
      "unseal_payload":"<base64url>",
      "owner_key":"<base64url-secret>"
    }
  ]
}
```

### `POST /v1/vault/unseal`

```bash
curl -sS -X POST "$SIGRYX/v1/vault/unseal" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "slot_id":1,
    "unseal_payload":"...",
    "owner_key":"..."
  }' | jq
```

### `POST /v1/vault/seal`

```bash
curl -sS -X POST "$SIGRYX/v1/vault/seal" \
  -H "Authorization: Bearer $TOKEN" | jq
```

```json
{"state":"SEALED"}
```

---

## Key roots

### `POST /v1/key-roots`

```bash
curl -sS -X POST "$SIGRYX/v1/key-roots" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"wallet_type":"ETHEREUM"}' | jq
```

Response shape:

```json
{
  "id":"<root-id>",
  "wallet_type":"ETHEREUM",
  "derivation_scheme":"BIP32_SECP256K1"
}
```

Use the live `/docs` schema for the exact enum string emitted by the release you are running.

### `GET /v1/key-roots`

```bash
curl -sS "$SIGRYX/v1/key-roots" \
  -H "Authorization: Bearer $TOKEN" | jq
```

```json
{
  "key_roots":[
    {
      "id":"<root-id>",
      "derivation_scheme":"BIP32_SECP256K1"
    }
  ]
}
```

---

## Wallets

### `POST /v1/wallets`

```bash
curl -sS -X POST "$SIGRYX/v1/wallets" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "key_root_id":"<root-id>",
    "user_id":"customer-123",
    "wallet_type":"ETHEREUM"
  }' | jq
```

Response:

```json
{
  "id":"<wallet-id>",
  "key_root_id":"<root-id>",
  "user_id":"customer-123",
  "wallet_type":"ETHEREUM",
  "adapter":"evm",
  "derivation_path":"m/44'/60'/0'/0/0",
  "public_key":"0x04...",
  "address":"0x..."
}
```

Private keys are not returned.

---

## Ethereum transaction signing

### `POST /v1/sign/transaction`

Legacy transaction example:

```bash
curl -sS -X POST "$SIGRYX/v1/sign/transaction" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id":"<wallet-id>",
    "transaction":{
      "type":"LEGACY",
      "chain_id":1,
      "nonce":0,
      "gas_limit":21000,
      "gas_price":"20000000000",
      "to":"0x1111111111111111111111111111111111111111",
      "value":"1000000000000000",
      "data":"0x"
    }
  }' | jq
```

EIP-1559 replaces `gas_price` with:

```json
{
  "max_priority_fee_per_gas":"1500000000",
  "max_fee_per_gas":"30000000000"
}
```

and may include an EIP-2930-style `access_list`.

### `POST /v1/verify/transaction`

```bash
curl -sS -X POST "$SIGRYX/v1/verify/transaction" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"wallet_id":"<wallet-id>","raw_transaction":"0x..."}' | jq
```

```json
{"valid":true}
```

---

## EIP-712

### `POST /v1/sign/typed-data`

```bash
curl -sS -X POST "$SIGRYX/v1/sign/typed-data" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary @typed-data-request.json | jq
```

Request body shape:

```json
{
  "wallet_id":"<wallet-id>",
  "typed_data":{
    "types":{},
    "primaryType":"...",
    "domain":{},
    "message":{}
  }
}
```

Response:

```json
{
  "signature":"0x<65-byte-r-s-v>",
  "digest":"0x<32-byte-digest>"
}
```

### `POST /v1/verify/typed-data`

Supply the same `typed_data` plus `signature` and `wallet_id`.

---

## Generic signing

### `POST /v1/sign/data` — JSON

```bash
curl -sS -X POST "$SIGRYX/v1/sign/data" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id":"<wallet-id>",
    "context":"orders:approval:v1",
    "format":"JSON",
    "payload":{"order_id":"ord-1","amount":"100"}
  }' | jq
```

### `POST /v1/sign/data` — RAW

RAW payload must be a base64url string:

```json
{
  "wallet_id":"<wallet-id>",
  "context":"artifact:v1",
  "format":"RAW",
  "payload":"SGVsbG8"
}
```

### `POST /v1/verify/data`

```json
{
  "wallet_id":"<wallet-id>",
  "context":"orders:approval:v1",
  "format":"JSON",
  "payload":{"amount":"100","order_id":"ord-1"},
  "signature":"0x..."
}
```

Response:

```json
{
  "valid":true,
  "digest":"0x..."
}
```

---

## Integrity signing

### `POST /v1/sign/integrity`

```json
{
  "wallet_id":"<wallet-id>",
  "context":"ledger:journal-entry:v1",
  "object_id":"je-123",
  "payload":{
    "id":"je-123",
    "amount":"100",
    "asset":"USDT",
    "status":"PENDING"
  },
  "integrity_fields":["/id","/amount","/asset"]
}
```

Response:

```json
{
  "signature":"0x...",
  "digest":"0x...",
  "reused":false
}
```

Exact replay returns `reused:true` and the original cryptographic record.

### `POST /v1/verify/integrity`

Add the signature:

```json
{
  "wallet_id":"<wallet-id>",
  "context":"ledger:journal-entry:v1",
  "object_id":"je-123",
  "payload":{
    "id":"je-123",
    "amount":"100",
    "asset":"USDT",
    "status":"COMPLETED"
  },
  "integrity_fields":["/id","/amount","/asset"],
  "signature":"0x..."
}
```

Because `/status` is not selected, changing it does not change protected canonical data.

Response shape:

```json
{
  "valid":true,
  "signature_valid":true,
  "record_match":true,
  "digest":"0x..."
}
```

---

## Audit

### `GET /v1/audit/events`

```bash
curl -sS "$SIGRYX/v1/audit/events?page=1&limit=50" \
  -H "Authorization: Bearer $TOKEN" | jq
```

`limit` is capped at 200.

---

## Recovery

Recovery endpoints are Root Admin only.

### `POST /v1/recovery/export`

```bash
curl -sS -X POST "$SIGRYX/v1/recovery/export" \
  -H "Authorization: Bearer $TOKEN" | jq
```

```json
{
  "recovery_key":"rec_...",
  "backup":"...",
  "key_roots":1
}
```

### `POST /v1/recovery/import`

```bash
curl -sS -X POST "$SIGRYX/v1/recovery/import" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"recovery_key":"rec_...","backup":"..."}' | jq
```

---

## Access management

### `GET /v1/access/permissions`

Returns all permission definitions:

```bash
curl -sS "$SIGRYX/v1/access/permissions" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### `POST /v1/access/roles`

```bash
curl -sS -X POST "$SIGRYX/v1/access/roles" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"generic-signer",
    "permissions":["sign.generic","verify.generic"]
  }' | jq
```

### `GET /v1/access/roles`

```bash
curl -sS "$SIGRYX/v1/access/roles" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### `PATCH /v1/access/roles/{id}`

The request replaces role name/permission state represented by the request body:

```json
{
  "name":"generic-and-integrity-signer",
  "permissions":[
    "sign.generic",
    "verify.generic",
    "sign.integrity",
    "verify.integrity"
  ]
}
```

### `POST /v1/access/users`

```bash
curl -sS -X POST "$SIGRYX/v1/access/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "username":"alice",
    "role_id":"<role-id>",
    "allowed_cidrs":["10.20.0.0/16"]
  }' | jq
```

Response returns the generated initial password once:

```json
{
  "id":"...",
  "username":"alice",
  "password":"<one-time-secret>"
}
```

### `GET /v1/access/users`

Returns public administrative metadata, not password hashes.

### `PATCH /v1/access/users/{id}`

```json
{
  "role_id":"<role-id>",
  "active":true,
  "allowed_cidrs":["10.20.0.0/16"]
}
```

The Root Admin has special immutability rules enforced by the service; use `/v1/auth/me` for its own credentials/network allowlist.

### `POST /v1/access/service-accounts`

```bash
curl -sS -X POST "$SIGRYX/v1/access/service-accounts" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"payments-signer",
    "role_id":"<role-id>",
    "allowed_cidrs":["10.50.10.0/24"]
  }' | jq
```

Response returns `client_secret` once:

```json
{
  "id":"...",
  "name":"payments-signer",
  "client_id":"...",
  "client_secret":"<one-time-secret>"
}
```

### `GET /v1/access/service-accounts`

Returns service-account metadata including client ID, role, active status, and CIDR list. It does not return the client secret.

### `PATCH /v1/access/service-accounts/{id}`

```json
{
  "role_id":"<role-id>",
  "active":true,
  "allowed_cidrs":["10.50.10.0/24"]
}
```

---

## Request IDs

Sigryx assigns an ID to every HTTP request and returns it as:

```text
X-Request-ID: <id>
```

The same ID is attached to audit metadata and security incidents. Preserve it in caller logs to correlate behavior without logging sensitive bodies.
