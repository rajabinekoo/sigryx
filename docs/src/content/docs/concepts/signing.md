---
title: Signing
description: Ethereum transaction, EIP-712, generic JSON/RAW signing, verification, and context separation.
---

Sigryx currently exposes three general signing families plus integrity signing:

1. Ethereum transactions;
2. EIP-712 typed data;
3. generic JSON or RAW data;
4. integrity-protected JSON fields, documented separately.

All signing operations require the Vault to be unsealed because they derive a child private key from an HD key root.

## Ethereum transaction signing

Endpoint:

```text
POST /v1/sign/transaction
```

Supported envelope types:

```text
LEGACY
EIP1559
```

Example EIP-1559 request:

```json
{
  "wallet_id": "<wallet-id>",
  "transaction": {
    "type": "EIP1559",
    "chain_id": 1,
    "nonce": 3,
    "gas_limit": 65000,
    "max_priority_fee_per_gas": "1500000000",
    "max_fee_per_gas": "30000000000",
    "to": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
    "value": "0",
    "data": "0xa9059cbb..."
  }
}
```

Numeric fee/value fields that may exceed safe JSON integer ranges are accepted as decimal strings or supported hex quantities as documented by the live schema.

Response:

```json
{
  "raw_transaction": "0x...",
  "transaction_hash": "0x...",
  "r": "0x...",
  "s": "0x...",
  "y_parity": 0
}
```

The caller is responsible for broadcasting `raw_transaction` to an Ethereum JSON-RPC provider. Sigryx signs; it does not broadcast the transaction in the current implementation.

### Verify transaction

```text
POST /v1/verify/transaction
```

```json
{
  "wallet_id": "<wallet-id>",
  "raw_transaction": "0x..."
}
```

This path verifies against the persisted wallet public key and does not require the child private key. It can therefore operate while the Vault is sealed.

## EIP-712 typed data

Sign:

```text
POST /v1/sign/typed-data
```

Request:

```json
{
  "wallet_id": "<wallet-id>",
  "typed_data": {
    "types": {
      "EIP712Domain": [
        {"name":"name","type":"string"},
        {"name":"version","type":"string"},
        {"name":"chainId","type":"uint256"}
      ],
      "Order": [
        {"name":"id","type":"string"},
        {"name":"amount","type":"uint256"}
      ]
    },
    "primaryType": "Order",
    "domain": {
      "name": "Example",
      "version": "1",
      "chainId": 1
    },
    "message": {
      "id": "order-123",
      "amount": "1000000"
    }
  }
}
```

Response:

```json
{
  "signature": "0x<65-byte-r-s-v>",
  "digest": "0x<32-byte-eip712-digest>"
}
```

Verification uses `/v1/verify/typed-data` with the same typed data, wallet ID, and signature. Typed-data verification can operate while sealed.

## Generic data signing

Generic signing is useful when the caller wants a Sigryx-specific cryptographic envelope rather than Ethereum transaction/EIP-712 semantics.

Endpoint:

```text
POST /v1/sign/data
```

The request includes a mandatory application-level `context` and one of two formats:

```text
JSON
RAW
```

### JSON

```json
{
  "wallet_id": "<wallet-id>",
  "context": "ledger:journal-entry:v1",
  "format": "JSON",
  "payload": {
    "id": "journal-123",
    "asset": "USDT",
    "amount": "1250000"
  }
}
```

Sigryx canonicalizes the JSON before hashing. Semantically identical object-key ordering therefore produces the same canonical bytes.

### RAW

For RAW mode, the HTTP `payload` is a base64url string containing the exact bytes:

```json
{
  "wallet_id": "<wallet-id>",
  "context": "artifact:blob:v1",
  "format": "RAW",
  "payload": "SGVsbG8gU2lncnl4"
}
```

### Generic digest construction

Sigryx computes:

```text
SHA256(
  "sigryx:generic-sign:v1" ||
  frame(context) ||
  frame(format) ||
  frame(canonical-or-raw-payload)
)
```

`frame(x)` is an unsigned 64-bit big-endian length followed by the bytes of `x`.

The explicit context is a domain separator. Use a stable, versioned value owned by the calling application, for example:

```text
ledger:journal-entry:v1
payments:payout-approval:v2
inventory:reservation:v1
```

Do not reuse a generic context across unrelated object types simply because the payload JSON looks similar.

### Generic signature format

The current Ethereum/secp256k1 adapter returns a compact 64-byte signature encoded as:

```text
r || s
```

The HTTP API renders it as `0x`-prefixed hex.

## Verification and sealed state

Generic verification recomputes the digest and checks it against the persisted wallet public key. It does not require the Vault Encryption Key or child private key, so it can work while sealed.

Integrity verification is different because it must decrypt a historical Signing Record. See [Integrity signing](/concepts/integrity-signing/).

## What Sigryx does not decide

Current signing APIs do not independently decide whether a business operation is semantically authorized. For example, Sigryx does not currently know whether:

- a transfer amount exceeds a treasury limit;
- a destination address is approved;
- an invoice has passed an application workflow;
- a transaction nonce is current on-chain.

The caller remains responsible for business-policy validation unless and until a dedicated policy layer is implemented in the running release.
