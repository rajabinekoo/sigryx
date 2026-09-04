---
title: Recovery
description: Export/import of encrypted HD key-root backups independent from the current unseal credentials.
---

Sigryx recovery provides a way to export HD key roots into an encrypted backup that is cryptographically independent from the current Vault Encryption Key and current unseal credentials.

Recovery endpoints are **Root Admin only** and require an **unsealed Vault**.

## Why recovery is separate from unseal

The N-of-N unseal scheme has an availability consequence: losing one required owner credential can prevent reconstruction of the current Vault Encryption Key.

A recovery export creates a separate recovery key and re-encrypts every root seed into a backup bundle.

Conceptually:

```text
current key root seed
      │
      ├─ currently decryptable because Vault is unsealed
      │
      ▼
fresh recovery key
      │
      ├─ encrypt each root independently
      └─ encrypt backup manifest
             │
             ▼
       opaque backup blob
```

The recovery key and backup should be stored separately.

## Export

```text
POST /v1/recovery/export
```

No request body is required.

Example:

```bash
curl -sS -X POST "$SIGRYX/v1/recovery/export" \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN" \
  | tee /secure/location/recovery-export.json | jq
```

Response:

```json
{
  "recovery_key": "rec_...",
  "backup": "...",
  "key_roots": 2
}
```

Sigryx persists neither returned value.

The recovery key is a newly generated 256-bit key encoded with the `rec_` prefix.

## Backup format

Internally the recovery package uses version `1` and builds a manifest containing key-root entries:

```text
id
derivation_scheme
encrypted_seed
```

Each seed is encrypted under the recovery key with AAD bound to its root ID and derivation scheme. The manifest itself is then encrypted under the same recovery key with a distinct bundle AAD.

The REST `backup` field is opaque. Do not parse or edit it in an application workflow.

## Storage model

Do **not** store these together in the same database or the same ordinary secret file:

```text
recovery_key + backup
```

A reasonable operating model is:

```text
backup       -> encrypted backup storage / disaster recovery system
recovery key -> independent secret manager / offline escrow
```

Protect both from accidental deletion.

## Import

```text
POST /v1/recovery/import
```

```json
{
  "recovery_key": "rec_...",
  "backup": "..."
}
```

During import Sigryx:

1. validates/decodes the recovery key;
2. decrypts and validates the backup manifest;
3. decrypts each recovery-protected HD seed;
4. re-encrypts each seed under the **current** Vault Encryption Key using the normal key-root AAD;
5. transactionally restores the key roots;
6. drops any stale cached plaintext root entries so future operations load restored state.

Old unseal credentials are not needed to decrypt the recovery backup. What is required is an already initialized/unsealed target Vault plus the recovery pair.

## Conflict behavior

Import validates root IDs, supported derivation schemes, seed sizes, duplicate entries, and database conflicts. It is not a "merge anything" endpoint.

Test restoration procedures in an isolated environment before relying on them for a real incident.

## Recovery runbook

A production runbook should answer:

- who may trigger export;
- where the backup is written;
- where the `rec_...` key is written;
- how many independent copies exist;
- how often exports are refreshed after new roots are created;
- who may authorize an import;
- how the target database is prepared;
- how restored wallets/addresses are validated;
- how recovery actions are audited and reviewed.

## Important limitation

Recovery exports key roots, not every PostgreSQL table. Database backups are still required for identities, roles, wallet mappings, audit history, and other durable application metadata.

Think of Sigryx recovery and PostgreSQL backup as complementary:

```text
PostgreSQL backup -> operational/durable state
Recovery export   -> cryptographic root survivability
```
