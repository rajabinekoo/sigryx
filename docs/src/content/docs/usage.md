---
title: 5-minute usage
description: Run the published Sigryx image and go from first boot to a signature.
---

This page is intentionally practical. It starts with a running Docker image and then walks through the shortest complete Sigryx API flow.

## 0. Start Sigryx

If Sigryx is already running in your application stack, skip to [Check health](#1-check-health).

The easiest new installation is Docker Compose with PostgreSQL beside Sigryx. You do **not** need to clone the Sigryx repository.

Create `compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: sigryx
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: sigryx
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U sigryx -d sigryx"]
      interval: 5s
      timeout: 5s
      retries: 20

  sigryx:
    image: rajabinekoo/sigryx:latest
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      HTTP_ADDR: 0.0.0.0:8080
      POSTGRES_DSN: postgres://sigryx:${POSTGRES_PASSWORD}@postgres:5432/sigryx?sslmode=disable
      POSTGRES_SCHEMA: sigryx_vault
      POSTGRES_AUTO_MIGRATE: "true"
      SETUP_TOKEN: ${SIGRYX_SETUP_TOKEN}
      AUTH_JWT_SECRET: ${SIGRYX_AUTH_JWT_SECRET}
    ports:
      - "8080:8080"
    cap_drop: [ALL]
    cap_add: [IPC_LOCK]
    security_opt:
      - no-new-privileges:true
    ulimits:
      memlock:
        soft: -1
        hard: -1

volumes:
  postgres-data:
```

Create `.env` beside it:

```dotenv
POSTGRES_PASSWORD=replace-with-a-random-password
SIGRYX_SETUP_TOKEN=replace-with-a-random-setup-token
SIGRYX_AUTH_JWT_SECRET=replace-with-a-random-jwt-secret
```

Generate three random values with:

```bash
openssl rand -hex 32
```

Load the setup token into your shell for the API examples:

```bash
set -a
. ./.env
set +a
export SETUP_TOKEN="$SIGRYX_SETUP_TOKEN"
```

Start the stack:

```bash
docker compose up -d
```

The Sigryx image creates its configured PostgreSQL schema and applies pending Atlas migrations before starting the HTTP server.

If you already have PostgreSQL and prefer one direct container command, see [Install & run](/getting-started/installation/#option-1-run-the-image-directly).

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

For a non-default host port, replace `8080` with the port you published from Docker.
