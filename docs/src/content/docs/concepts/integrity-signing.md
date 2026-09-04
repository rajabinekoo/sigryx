---
title: Integrity signing
description: Immutable field selection, canonical JSON, encrypted Signing Records, replay/reuse semantics, and incident handling.
---

Integrity signing is for application objects where selected fields must become permanently bound to the first accepted cryptographic record.

It combines:

- RFC 6901 JSON Pointer field selection;
- canonical JSON;
- a context + object ID identity;
- a secp256k1 signature;
- an append-only encrypted Signing Record;
- integrity incident audit/alert behavior.

## Sign selected fields

Endpoint:

```text
POST /v1/sign/integrity
```

Example:

```json
{
  "wallet_id": "<wallet-id>",
  "context": "ledger:journal-entry:v1",
  "object_id": "je_01J...",
  "payload": {
    "id": "je_01J...",
    "account_id": "acc_42",
    "amount": "1250000",
    "asset": "USDT",
    "status": "PENDING",
    "metadata": {
      "note": "can change without invalidating immutable fields"
    }
  },
  "integrity_fields": [
    "/id",
    "/account_id",
    "/amount",
    "/asset"
  ]
}
```

Only selected fields participate in the protected canonical representation.

## Stable identity

A Signing Record is uniquely identified by:

```text
(context, object_id)
```

The database enforces this identity as unique.

The first successful signature establishes the immutable truth for that object identity:

```text
wallet_id
integrity_fields
canonical selected values
digest
signature
```

## Digest

The digest is:

```text
SHA256(
  "sigryx:integrity-sign:v1" ||
  frame(context) ||
  frame(object_id) ||
  frame(canonical_selected_json)
)
```

This makes the same field values under a different context or object ID a different signed object.

## Signing Record encryption

The full Signing Record is serialized and encrypted using AES-256-GCM with the **runtime Vault Encryption Key**.

AAD binds the encrypted record to its external identity:

```text
"sigryx:signing-record:v1" + frame(context) + frame(object_id)
```

There is no independent `SIGNING_RECORD_SECRET` in the current design. The Signing Record is decryptable only while the Vault Encryption Key exists.

## Append-only persistence

`signing_records` is protected by database triggers that reject normal:

```text
UPDATE
DELETE
TRUNCATE
```

This makes the historical record intentionally difficult to rewrite through ordinary application/database paths.

## Reuse behavior

Calling `POST /v1/sign/integrity` a second time for the same `(context, object_id)` does **not** blindly sign again.

Sigryx decrypts the stored record and compares:

- wallet ID;
- normalized integrity-field schema;
- canonical protected values;
- stored digest;
- stored signature validity.

If all match, the original result is returned:

```json
{
  "signature": "0x...",
  "digest": "0x...",
  "reused": true
}
```

No new private-key signing operation is required for an exact replay.

If a caller tries to change the protected schema, protected values, or wallet, the request conflicts with the original immutable record.

## Verify against history

Endpoint:

```text
POST /v1/verify/integrity
```

Verification performs two different checks:

1. **signature check** — does the supplied signature match the current selected values and wallet public key?
2. **record check** — do the wallet, schema, selected values, digest, and signature match the immutable Signing Record?

Response:

```json
{
  "valid": false,
  "signature_valid": true,
  "record_match": false,
  "digest": "0x...",
  "reason": "INTEGRITY_VALUE_MISMATCH"
}
```

Possible mismatch/incident reasons include:

```text
SIGNING_RECORD_NOT_FOUND
SIGNING_RECORD_TAMPERED
WALLET_MISMATCH
INTEGRITY_SCHEMA_MISMATCH
INTEGRITY_VALUE_MISMATCH
DIGEST_MISMATCH
SIGNATURE_MISMATCH
STORED_SIGNATURE_INVALID
```

## Why verification requires an unsealed Vault

Ordinary signature verification only needs a public key. Integrity verification additionally needs to open the encrypted Signing Record, which is protected by the Vault Encryption Key.

Therefore `/v1/verify/integrity` requires the Vault to be unsealed in the current implementation.

## Incident handling

A detected historical mismatch/tamper path records a critical audit event:

```text
security.integrity_violation
```

with outcome `BLOCKED` and useful context such as incident code, object context, object ID, request ID, source IP, and digest details where applicable.

If `ALERT_WEBHOOK_URL` is configured, Sigryx also attempts to POST a CRITICAL alert. If delivery fails, it records:

```text
security.alert_delivery_failed
```

as another critical audit event when audit persistence is available.

## Recommended use

Integrity signing is appropriate for fields such as:

- ledger entry identity and economic amounts;
- settlement instructions;
- immutable payout destination and amount;
- signed configuration revisions;
- compliance decision artifacts.

Do not include mutable workflow fields in `integrity_fields` unless changing them is intended to be treated as an integrity violation.

For example, signing `/status` is usually a bad idea when `PENDING -> COMPLETED` is a legitimate state transition. Sign the immutable inputs and model state transitions separately.
