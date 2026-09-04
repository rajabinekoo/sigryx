---
title: Seal & unseal
description: N-of-N unseal credentials, Vault Encryption Key derivation, lifecycle, and operational consequences.
---

The sealed/unsealed lifecycle is the central runtime security boundary in Sigryx.

## States

The API exposes three states:

```text
UNINITIALIZED
SEALED
UNSEALED
```

### `UNINITIALIZED`

No durable unseal slots exist yet. The Vault must be initialized once using `POST /v1/vault/init`.

### `SEALED`

Unseal slots exist, but the process does not currently possess the Vault Encryption Key.

### `UNSEALED`

All required N-of-N credentials have been submitted and the process has constructed the Vault Encryption Key in protected memory.

## Credential construction

For each slot, initialization creates three pieces of material:

```text
ownerSecret          32 random bytes; returned once, not persisted
serverKeyMaterial    32 random bytes; persisted by Sigryx
realUnsealKey         32 random bytes; generated directly in protected memory
```

The wrapping key is:

```text
SHA256(ownerSecret || serverKeyMaterial)
```

The real unseal key is encrypted with AES-256-GCM using slot-specific authenticated data:

```text
AAD = "sigryx:unseal-key:v1:<slotID>"
```

The durable slot contains:

```text
slot_id
wrapped_key
server_key_material
```

The owner receives:

```text
slot_id
wrapped_key / unseal_payload
owner_key
```

Neither side alone has enough material to recover the real unseal key.

## Why the payload is sent back during unseal

`POST /v1/vault/unseal` supplies:

```json
{
  "slot_id": 1,
  "unseal_payload": "...",
  "owner_key": "..."
}
```

Sigryx loads the persisted slot, verifies that the submitted wrapped payload matches the stored wrapped key, derives the wrapping key from the owner/server material, and authenticates/decrypts the real unseal key.

Invalid credential material is surfaced as an authentication failure rather than revealing which internal comparison failed.

## N-of-N behavior

The current implementation requires **every configured slot**.

If initialized with `N=3`:

```text
slot 1 accepted -> submitted=1 required=3 state=SEALED
slot 2 accepted -> submitted=2 required=3 state=SEALED
slot 3 accepted -> submitted=3 required=3 state=UNSEALED
```

This is not Shamir M-of-N threshold unsealing. Losing one required owner credential can make normal unsealing impossible; use recovery backups and a deliberate credential-custody process.

## Vault Encryption Key

When all unseal keys are present, `SecretStore` constructs the runtime Vault Encryption Key from the real unseal keys in deterministic slot order:

```text
SHA256(
    unsealKey[1] ||
    unsealKey[2] ||
    ... ||
    unsealKey[N]
)
```

The resulting 32-byte value exists only as a `securemem.Secret` while the Vault is unsealed.

After construction, the individual real unseal keys have completed their job and are destroyed.

## What the Vault Encryption Key protects

The current implementation uses the Vault Encryption Key to encrypt/decrypt high-value durable records including:

- HD key-root seeds;
- encrypted integrity Signing Records.

AES-256-GCM is used with record-specific authenticated data so ciphertext is bound to the identity/context for which it was created.

## Sealing

`POST /v1/vault/seal` clears runtime secret ownership. This includes:

- the Vault Encryption Key;
- any partial unseal attempt;
- cached plaintext HD key-root seeds.

Sealing is intentionally idempotent once the Vault has been initialized.

## Restart

Runtime secret material is not reconstructed automatically from the database. A restart therefore produces:

```text
persisted slots exist + no runtime Vault key = SEALED
```

Operators must unseal again.

## Operation behavior while sealed

Not every endpoint needs secret material.

### Works while sealed

Subject to authentication/authorization:

- health;
- authentication and token flows;
- Vault status;
- listing key-root metadata;
- returning an already-existing wallet from `POST /v1/wallets`;
- transaction verification;
- EIP-712 verification;
- generic data verification;
- audit reads and access-management operations.

### Requires unsealed state

- creating a new key root;
- creating a new wallet that requires derivation;
- transaction signing;
- EIP-712 signing;
- generic signing;
- integrity signing;
- integrity verification, because its Signing Record is encrypted with the Vault Encryption Key;
- recovery export/import.

## Operational guidance

For production:

- assign unseal credentials to distinct owners or independently controlled secret stores;
- never place every owner credential in the same application `.env` file;
- do not persist initialization response bodies in ordinary logs;
- use TLS for all unseal submissions;
- keep the Sigryx HTTP plane private whenever possible;
- create and test recovery backups before depending on the service;
- define a restart/unseal runbook before rollout.
