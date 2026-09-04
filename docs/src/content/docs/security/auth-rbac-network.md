---
title: Auth, RBAC & network policy
description: JWT principals, sessions, roles, route permissions, CIDR allowlists, and reverse-proxy trust.
---

Sigryx uses bearer authentication plus route-level permission mapping.

There are two authenticated principal kinds:

```text
USER
SERVICE
```

The Root Admin is a privileged USER with no role requirement.

## User authentication

Users authenticate with username/password and receive:

- a short-lived JWT access token;
- an opaque refresh token tied to a durable session.

Every protected request verifies:

- JWT signature/issuer/audience;
- user/session identity;
- session has not been revoked;
- session has not expired;
- user is active;
- client IP is allowed;
- route permission is granted unless Root Admin.

## Service authentication

Service accounts receive `client_id` and `client_secret` at creation. The secret is shown once.

`POST /v1/auth/service-token` exchanges them for a short-lived access token.

Service tokens do not create refresh sessions in the current implementation.

## Roles

Non-root users and service accounts reference a role containing explicit permission strings.

Example role:

```json
{
  "name": "application-signer",
  "permissions": [
    "wallet.create",
    "sign.transaction",
    "sign.typed_data",
    "sign.generic",
    "verify.transaction",
    "verify.typed_data",
    "verify.generic"
  ]
}
```

Avoid broad administrative permissions for application identities.

## Root Admin

The Root Admin bypasses normal permission membership checks and is required for recovery export/import.

The Root Admin cannot be treated like an ordinary role-backed user. Protect its credentials as break-glass/control-plane credentials rather than application credentials.

## CIDR allowlists

Users and service accounts may have `allowed_cidrs`.

An empty list means any source IP is accepted by Sigryx authentication.

Example:

```json
{
  "allowed_cidrs": [
    "10.20.0.0/16",
    "192.0.2.15/32"
  ]
}
```

CIDR checks happen during login/token authorization as appropriate.

## Trusted proxies

When Sigryx is behind a reverse proxy, configure:

```dotenv
TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.100.10/32
```

Sigryx trusts `X-Forwarded-For` / `X-Real-IP` only when the immediate peer belongs to a configured trusted proxy CIDR.

Without this boundary, accepting forwarded headers from arbitrary peers would let clients spoof source IPs and bypass IP policy.

The resolver walks `X-Forwarded-For` from right to left and selects the first address not recognized as a trusted proxy.

## Route authorization model

Protected routes are explicitly mapped to permissions in HTTP middleware. A protected route without a configured permission, authenticated-only rule, or root-only rule fails closed with `403`.

This is important for contributors: adding a new route without adding authorization policy does not silently make it available to authenticated principals.

## Public routes

The following bypass bearer-token middleware:

```text
GET  /v1/health
GET  /docs
GET  /openapi.json
POST /v1/setup
POST /v1/auth/login
POST /v1/auth/refresh
POST /v1/auth/service-token
```

They still have route-specific validation where applicable.

## Recommended role split

A small production deployment might define:

```text
root admin
  -> human break-glass / access management / recovery

vault operator
  -> vault.status.read
  -> vault.unseal
  -> vault.seal

wallet provisioner
  -> keyroot.read (if needed)
  -> wallet.create

transaction signer
  -> sign.transaction
  -> verify.transaction

integrity service
  -> sign.integrity
  -> verify.integrity

security auditor
  -> audit.read
```

Create roles around actual workloads rather than user titles when possible.

## Token handling

Bearer tokens are credentials. Do not:

- put them in query strings;
- log `Authorization` headers;
- persist access tokens unnecessarily;
- send them over plaintext networks;
- reuse Root Admin tokens in application containers.

Refresh tokens are also bearer credentials even though Sigryx stores only hashes.
