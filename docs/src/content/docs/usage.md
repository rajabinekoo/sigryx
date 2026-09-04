---
title: 5-minute usage
description: The shortest end-to-end Sigryx flow from first boot to a signature.
---

This page intentionally avoids architecture detail. It is the shortest complete path through Sigryx using `curl`.

Assumptions:

- Sigryx is listening on `http://localhost:8080`.
- PostgreSQL migrations have already been applied.
- `SETUP_TOKEN` and `AUTH_JWT_SECRET` are configured.
- `jq` is installed for shell examples.

Set the base URL:

```bash
export SIGRYX=http://localhost:8080
```

## 1. Check health

```bash
curl -sS "$SIGRYX/v1/health" | jq
```

Expected shape:

```json
{
  "message": "it's healthy"
}
```

## 2. Bootstrap the Root Admin once

`POST /v1/setup` is public, but it requires the one-time setup token configured in `SETUP_TOKEN`.

```bash
curl -sS -X POST "$SIGRYX/v1/setup" \
  -H "X-Sigryx-Setup-Token: $SETUP_TOKEN" | tee /tmp/sigryx-setup.json | jq
```

The response contains a randomly generated Root Admin username and password. They are returned once.

```bash
export SIGRYX_ADMIN_USERNAME="$(jq -r .username /tmp/sigryx-setup.json)"
export SIGRYX_ADMIN_PASSWORD="$(jq -r .password /tmp/sigryx-setup.json)"
```

Store the password in a secret manager, then rotate it using `PATCH /v1/auth/me` when your operational process requires it.

## 3. Login

```bash
curl -sS -X POST "$SIGRYX/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg username "$SIGRYX_ADMIN_USERNAME" \
    --arg password "$SIGRYX_ADMIN_PASSWORD" \
    '{username:$username,password:$password}')" \
  | tee /tmp/sigryx-token.json | jq
```

```bash
export SIGRYX_ACCESS_TOKEN="$(jq -r .access_token /tmp/sigryx-token.json)"
export SIGRYX_REFRESH_TOKEN="$(jq -r .refresh_token /tmp/sigryx-token.json)"
```

Authenticated requests use:

```text
Authorization: Bearer <access_token>
```

## 4. Initialize the Vault once

For a local demonstration, create three N-of-N unseal credentials:

```bash
curl -sS -X POST "$SIGRYX/v1/vault/init" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"unseal_key_count":3}' \
  | tee /tmp/sigryx-unseal.json | jq
```

The response resembles:

```json
{
  "state": "SEALED",
  "credentials": [
    {
      "slot_id": 1,
      "unseal_payload": "...",
      "owner_key": "..."
    }
  ]
}
```

**Do not treat `/tmp/sigryx-unseal.json` as a production storage strategy.** Each `owner_key` is owner-held secret material and is never persisted by Sigryx. In production, distribute credentials to separate trusted owners or independent secret stores according to your operating model.

## 5. Unseal

Every configured slot must be submitted successfully. The current implementation is N-of-N, not threshold M-of-N.

```bash
for slot in 0 1 2; do
  body="$(jq -c ".credentials[$slot] | {slot_id, unseal_payload, owner_key}" /tmp/sigryx-unseal.json)"

  curl -sS -X POST "$SIGRYX/v1/vault/unseal" \
    -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "$body" | jq
done
```

The final response should report:

```json
{
  "state": "UNSEALED",
  "submitted": 3,
  "required": 3
}
```

Confirm:

```bash
curl -sS "$SIGRYX/v1/vault/status" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" | jq
```

## 6. Create an Ethereum key root

```bash
curl -sS -X POST "$SIGRYX/v1/key-roots" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"wallet_type":"ETHEREUM"}' \
  | tee /tmp/sigryx-root.json | jq
```

```bash
export SIGRYX_KEY_ROOT_ID="$(jq -r .id /tmp/sigryx-root.json)"
```

The plaintext HD seed is not returned. Sigryx persists an AES-256-GCM encrypted key root and keeps the currently loaded plaintext seed only in protected runtime memory while the Vault remains unsealed.

## 7. Create a deterministic wallet

Use a stable application-owned user identifier:

```bash
export APP_USER_ID='user-42'

curl -sS -X POST "$SIGRYX/v1/wallets" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg root "$SIGRYX_KEY_ROOT_ID" \
    --arg user "$APP_USER_ID" \
    '{key_root_id:$root,user_id:$user,wallet_type:"ETHEREUM"}')" \
  | tee /tmp/sigryx-wallet.json | jq
```

```bash
export SIGRYX_WALLET_ID="$(jq -r .id /tmp/sigryx-wallet.json)"
```

Calling the same endpoint again with the same key root, adapter, and `user_id` returns the existing wallet instead of allocating a new derivation path.

## 8. Sign JSON data

```bash
curl -sS -X POST "$SIGRYX/v1/sign/data" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg wallet "$SIGRYX_WALLET_ID" \
    '{
      wallet_id:$wallet,
      context:"example:invoice:v1",
      format:"JSON",
      payload:{id:"inv_123",amount:"125000",currency:"IRR"}
    }')" \
  | tee /tmp/sigryx-signature.json | jq
```

The response contains:

```json
{
  "signature": "0x...",
  "digest": "0x..."
}
```

For JSON signing, Sigryx canonicalizes the JSON before hashing, frames the context and format, computes a SHA-256 digest, and signs that digest using the wallet's derived secp256k1 private key.

## 9. Verify the signature

```bash
export SIGRYX_SIGNATURE="$(jq -r .signature /tmp/sigryx-signature.json)"

curl -sS -X POST "$SIGRYX/v1/verify/data" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg wallet "$SIGRYX_WALLET_ID" \
    --arg signature "$SIGRYX_SIGNATURE" \
    '{
      wallet_id:$wallet,
      context:"example:invoice:v1",
      format:"JSON",
      payload:{currency:"IRR",amount:"125000",id:"inv_123"},
      signature:$signature
    }')" | jq
```

The key ordering in the JSON object may differ because Sigryx canonicalizes JSON. The context must match exactly.

## 10. Seal the Vault

```bash
curl -sS -X POST "$SIGRYX/v1/vault/seal" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" | jq
```

Sealing destroys the runtime Vault Encryption Key, partial unseal state, and cached plaintext key-root seeds.

Normal transaction/data/typed-data signing now fails until the Vault is unsealed again.

## 11. Explore the live REST API client

Open:

```text
http://localhost:8080/docs
```

Sigryx serves an interactive Scalar API client there. Its OpenAPI source is:

```text
http://localhost:8080/openapi.json
```

Both routes are intentionally unauthenticated in the current HTTP middleware. Protect network access to the Sigryx management plane accordingly.
