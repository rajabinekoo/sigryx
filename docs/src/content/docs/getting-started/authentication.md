---
title: Authentication bootstrap
description: Root Admin setup, user sessions, service accounts, token rotation, and bootstrap safety.
---

Sigryx authentication is intentionally independent from the sealed Vault. Operators must be able to authenticate while the Vault is sealed in order to submit unseal credentials or inspect status.

## Root Admin

The Root Admin is unique. The database enforces a single root-admin row.

Create it once:

```bash
curl -X POST http://localhost:8080/v1/setup \
  -H "X-Sigryx-Setup-Token: $SETUP_TOKEN"
```

Response:

```json
{
  "username": "admin_<random>",
  "password": "<random-secret>"
}
```

The Root Admin bypasses role permission checks, but is still subject to its optional IP/CIDR allowlist.

## User login

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin_xxx","password":"..."}'
```

Response shape:

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 600
}
```

The default access TTL is 10 minutes and the default refresh-session TTL is 168 hours.

## Refresh-token rotation

Refresh tokens are opaque random values. Sigryx stores only their hash in the session record. Refreshing rotates the hash:

```bash
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"..."}'
```

Always replace the previous refresh token with the newly returned value after a successful rotation.

## Logout

User sessions can be revoked with:

```bash
curl -X POST http://localhost:8080/v1/auth/logout \
  -H "Authorization: Bearer $SIGRYX_ACCESS_TOKEN"
```

Service-account access tokens are short-lived bearer tokens and do not have user refresh sessions.

## Change current user credentials

Authenticated users may call:

```http
PATCH /v1/auth/me
```

with the current password. A new password must be at least 12 characters. Changing the password revokes all user sessions.

Only the Root Admin may change its own `allowed_cidrs` through this endpoint.

## Application workloads: use service accounts

Create an application-specific role, then a service account. The creation response returns `client_secret` once; only its SHA-256 hash is persisted.

The service exchanges credentials for an access token using:

```http
POST /v1/auth/service-token
```

```json
{
  "client_id": "...",
  "client_secret": "..."
}
```

Service accounts should normally have:

- the smallest possible role;
- an IP/CIDR allowlist where the network topology permits it;
- separate credentials per workload/environment;
- no access-management permissions unless the workload is itself an administrative control plane.

## Public endpoints

The current middleware deliberately allows the following without a bearer token:

```text
GET  /v1/health
GET  /docs
GET  /openapi.json
POST /v1/setup
POST /v1/auth/login
POST /v1/auth/refresh
POST /v1/auth/service-token
```

`/v1/setup` still requires the setup header. The other public authentication endpoints validate their own credentials.

Treat the Sigryx network endpoint as a management/security plane. Public in middleware does not mean it should be internet-exposed without an intentional network policy.
