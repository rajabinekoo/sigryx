---
title: Security model
description: Assets, trust boundaries, guarantees, non-guarantees, and deployment assumptions for Sigryx.
---

Sigryx is security-sensitive infrastructure. Its design reduces secret exposure, but it does not make an already-compromised host magically trustworthy.

This page defines the practical security model for the current implementation.

## Assets Sigryx protects

The highest-value assets are:

- owner-held unseal credentials;
- real unseal keys reconstructed during unseal;
- the runtime Vault Encryption Key;
- plaintext HD master seeds;
- derived child private keys;
- recovery keys and recovery backup contents;
- authentication credentials and service-account secrets;
- integrity Signing Records and audit history.

## Primary trust boundaries

```text
Unseal owner
    │ owner_key + wrapped payload
    ▼
Network boundary
    ▼
Sigryx process
    │
    ├─ protected-memory boundary
    │
    ├─ PostgreSQL boundary
    │
    ├─ Ethereum/RPC consumer boundary (caller broadcasts today)
    │
    └─ optional alert-webhook boundary
```

The operating system, process privileges, database identity, TLS termination, reverse proxy, and secret-custody process all contribute to the effective security level.

## What Sigryx tries to guarantee

### No persisted child private keys

Wallet private keys are derived from an HD seed at the persisted derivation path when needed. They are not stored as wallet rows or returned from REST APIs.

### Encrypted durable root material

HD root seeds are persisted only as AES-256-GCM sealed payloads under the runtime Vault Encryption Key with root-specific AAD.

### Sealed process does not retain the Vault key

Sealing and process restart remove runtime access to the Vault Encryption Key and cached plaintext roots.

### Owner unseal secret is not persisted

During initialization, `owner_key` is returned to the operator and is not written to Sigryx storage.

### Authentication secrets are hashed where appropriate

- user passwords: Argon2id hashes;
- service-account client secrets: SHA-256 hashes;
- refresh tokens: hashes in session storage.

The JWT signing secret remains an operator-provided process secret and is not derived from the Vault.

### Append-only security records

Normal mutation of `signing_records` and `audit_events` is rejected by PostgreSQL triggers. Audit retention has a narrowly scoped sanctioned purge function; Signing Records do not have a normal purge path.

## What Sigryx does not guarantee

### A fully compromised privileged host can still be dangerous

An attacker with sufficient kernel/root/debugger/runtime control may observe process memory while the Vault is unsealed, alter execution, intercept credentials, or modify binaries.

Protected memory makes accidental leakage and several memory-handling classes harder; it is not a substitute for host isolation.

### No hardware root of trust in the current code

The current implementation does not require an HSM, TPM, enclave, or external KMS. The security boundary is software + host OS + operator process.

### No quorum signing in the current API

N-of-N unseal is not distributed signing. Once the process is unsealed, the current signer derives and uses the key locally inside the Sigryx process.

### No application intent policy engine in the current API

RBAC controls **who may call a signing route**. It does not currently evaluate arbitrary business intent such as transaction limits, address allowlists per treasury, or multi-approver workflows.

### API authentication is not transport encryption

Bearer tokens, owner keys, recovery material, and signed payloads must be protected in transit with TLS at the deployment boundary.

## Secret classes must remain separate

Do not reuse one secret for multiple roles.

Keep these independent:

```text
SETUP_TOKEN
AUTH_JWT_SECRET
unseal owner keys
Vault Encryption Key (derived internally)
HD root seeds (generated internally)
service account client secrets
recovery key
```

The current Signing Record design intentionally reuses the already-established Vault Encryption Key because Signing Records are part of the Vault-protected durable secret domain. That does not mean unrelated application/authentication secrets should share this key.

## Database compromise model

A database-only attacker can read persisted encrypted material and public metadata. Without the required owner-held unseal secrets, the attacker should not have the runtime Vault Encryption Key merely from database rows.

However, database integrity is still critical. An attacker who can mutate tables may cause denial of service, tamper with public metadata, alter authorization data, or attack surrounding invariants. Database credentials and network access remain security-critical.

Append-only triggers add a defense for audit/Signing Record mutation, but PostgreSQL superusers or owners with sufficient privileges can bypass many database-level controls. Do not treat SQL triggers as protection from a fully privileged database administrator.

## Network model

Prefer:

```text
application services
       │ private network / mTLS or TLS
       ▼
reverse proxy / ingress
       │
       ▼
Sigryx
       │
       ▼
PostgreSQL on private network
```

Avoid exposing port 8080 directly to the public internet unless you have explicitly designed controls around it.

The current `/docs` and `/openapi.json` routes are unauthenticated. Restrict them at the edge if API metadata should not be public.

## Recommended hardening checklist

- TLS for every non-loopback Sigryx connection.
- Private network placement for Sigryx and PostgreSQL.
- Run Sigryx as a dedicated non-root user.
- Minimize Linux capabilities.
- Disable core dumps where possible.
- Ensure swap/locking policy is compatible with secure-memory requirements.
- Restrict ptrace/debugging.
- Keep `AUTH_JWT_SECRET` outside source control.
- Remove or disable `SETUP_TOKEN` after successful bootstrap when operationally appropriate.
- Give applications service accounts, not Root Admin credentials.
- Use CIDR allowlists where stable network identity exists.
- Back up PostgreSQL and separately manage Sigryx recovery exports.
- Test recovery before production dependency.
- Monitor critical audit actions and integrity alerts.
- Patch the host, Go runtime, PostgreSQL, and dependencies on a defined cadence.

## Disclosure

Security issues should not be discussed in public issue threads before a coordinated fix process exists. Add a repository `SECURITY.md` with the project's preferred private reporting channel before broad public adoption.
