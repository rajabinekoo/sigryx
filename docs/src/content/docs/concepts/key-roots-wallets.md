---
title: Key roots & wallets
description: HD master seeds, encrypted roots, deterministic wallet allocation, derivation paths, and persisted wallet metadata.
---

Sigryx separates **key roots** from **wallets**.

A key root owns one HD master seed. A wallet is a deterministic child identity derived from that seed.

## Key root

Creating a key root currently accepts:

```json
{
  "wallet_type": "ETHEREUM"
}
```

The Ethereum wallet type selects the derivation scheme:

```text
BIP32 + secp256k1
```

Sigryx generates a 32-byte HD master seed directly inside protected memory. The seed is encrypted using AES-256-GCM under the current Vault Encryption Key before persistence.

The AAD binds ciphertext to the root identity and derivation scheme:

```text
sigryx:key-root:v1:<root-id>:<derivation-scheme>
```

The API never returns the plaintext master seed.

## Runtime seed cache

After a root is created, `SecretStore` may keep the plaintext seed as a protected `securemem.Secret` while the Vault remains unsealed.

For a persisted root not currently cached, wallet/signing operations can decrypt the seed under the Vault Encryption Key, place it into protected memory, and use it through callback-scoped access.

Sealing destroys cached roots.

## Wallet derivation

The current Ethereum adapter uses BIP44:

```text
m/44'/60'/0'/0/<index>
```

Where:

- `44'` is the BIP44 purpose;
- `60'` is the Ethereum coin type;
- account is `0'`;
- change is `0`;
- `<index>` is allocated by Sigryx.

Example:

```text
m/44'/60'/0'/0/0
m/44'/60'/0'/0/1
m/44'/60'/0'/0/2
```

## Deterministic application mapping

`POST /v1/wallets` accepts:

```json
{
  "key_root_id": "<root-id>",
  "user_id": "customer-123",
  "wallet_type": "ETHEREUM"
}
```

The durable identity mapping is effectively:

```text
(key_root_id, adapter, user_id) -> wallet
```

If the tuple already exists, Sigryx returns the same wallet. This makes the API safe for application-level retries around wallet resolution.

If it does not exist, Sigryx allocates the next derivation index for that key root and adapter.

## Concurrency

Wallet allocation uses a durable per-root/per-adapter counter. The persistence layer serializes index allocation and enforces uniqueness constraints on:

- key root + adapter + user ID;
- key root + adapter + derivation path;
- adapter + address.

If two requests race to create the same application mapping, the service handles the unique-conflict path by reading and returning the concurrently committed wallet.

## What is persisted for a wallet

A wallet record contains public/non-secret metadata:

```text
id
key_root_id
user_id
adapter
derivation_path
public_key
address
```

A child private key is not stored.

## How signing gets a private key

At signing time:

```text
wallet id
   │
   ├─ load wallet metadata
   ├─ load key-root metadata
   ├─ get/decrypt protected root seed
   ├─ derive private key at persisted path
   ├─ perform signing callback
   └─ wipe derived private-key material
```

This is the core deterministic-account property of the implementation.

## Public key and address

The Ethereum adapter returns an uncompressed 65-byte secp256k1 public key. The REST response renders it as `0x`-prefixed hex.

The address is the Ethereum address derived from the public key and formatted with EIP-55 checksum casing.

## Sealed-state nuance

`POST /v1/wallets` first checks whether the mapping already exists. Therefore:

- resolving an existing wallet can succeed while sealed;
- allocating a new wallet requires an unsealed Vault because secret derivation is needed.

Applications should not rely on this nuance as a readiness signal. If the workload needs signing or new deterministic accounts, treat `UNSEALED` as required.

## Multiple roots

Applications may use multiple roots to create administrative or cryptographic separation, for example:

```text
root A -> treasury wallets
root B -> customer deposit wallets
root C -> staging integration identities
```

A root ID should be treated as long-lived configuration in the calling application. Do not select roots from untrusted user input without an application-level policy.
