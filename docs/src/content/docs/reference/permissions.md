---
title: Permissions
description: Complete RBAC permission catalog and route mapping for the current Sigryx API.
---

Permissions are exact strings stored on roles.

The current catalog is:

| Permission | Category | Meaning |
| --- | --- | --- |
| `vault.status.read` | vault | Read Vault status |
| `vault.initialize` | vault | Initialize Vault |
| `vault.unseal` | vault | Submit unseal credentials |
| `vault.seal` | vault | Seal Vault |
| `keyroot.read` | key-root | List key roots |
| `keyroot.create` | key-root | Create key root |
| `wallet.create` | wallet | Create or resolve wallet |
| `sign.transaction` | signing | Sign Ethereum transaction |
| `sign.typed_data` | signing | Sign EIP-712 typed data |
| `sign.generic` | signing | Sign generic data |
| `sign.integrity` | signing | Sign integrity-protected JSON fields |
| `verify.transaction` | verification | Verify Ethereum transaction |
| `verify.typed_data` | verification | Verify EIP-712 typed data |
| `verify.generic` | verification | Verify generic data |
| `verify.integrity` | verification | Verify integrity-protected JSON fields |
| `audit.read` | audit | Read audit log |
| `access.users.manage` | access | Manage users |
| `access.roles.manage` | access | Manage roles and list permission definitions |
| `access.service_accounts.manage` | access | Manage service accounts |

## Route mapping

```text
GET    /v1/vault/status                  -> vault.status.read
POST   /v1/vault/init                    -> vault.initialize
POST   /v1/vault/unseal                  -> vault.unseal
POST   /v1/vault/seal                    -> vault.seal
GET    /v1/key-roots                     -> keyroot.read
POST   /v1/key-roots                     -> keyroot.create
POST   /v1/wallets                       -> wallet.create
POST   /v1/sign/transaction              -> sign.transaction
POST   /v1/sign/typed-data               -> sign.typed_data
POST   /v1/sign/data                     -> sign.generic
POST   /v1/sign/integrity                -> sign.integrity
POST   /v1/verify/transaction            -> verify.transaction
POST   /v1/verify/typed-data             -> verify.typed_data
POST   /v1/verify/data                   -> verify.generic
POST   /v1/verify/integrity              -> verify.integrity
GET    /v1/audit/events                  -> audit.read
GET    /v1/access/permissions            -> access.roles.manage
GET    /v1/access/roles                  -> access.roles.manage
POST   /v1/access/roles                  -> access.roles.manage
PATCH  /v1/access/roles/:id              -> access.roles.manage
GET    /v1/access/users                  -> access.users.manage
POST   /v1/access/users                  -> access.users.manage
PATCH  /v1/access/users/:id              -> access.users.manage
GET    /v1/access/service-accounts       -> access.service_accounts.manage
POST   /v1/access/service-accounts       -> access.service_accounts.manage
PATCH  /v1/access/service-accounts/:id   -> access.service_accounts.manage
```

## Authenticated-only routes

These require a valid authenticated USER but do not use a named role permission:

```text
POST  /v1/auth/logout
PATCH /v1/auth/me
```

Service principals cannot use user-only session/account operations.

## Root-only routes

```text
POST /v1/recovery/export
POST /v1/recovery/import
```

Even a role containing all named permissions is not a substitute for Root Admin on these routes.

## Fail-closed mapping

The HTTP authorization middleware checks whether each protected route has one of:

- an explicit permission;
- an authenticated-only rule;
- a root-only rule.

If a new protected route is added without policy configuration, Sigryx returns forbidden rather than silently permitting it.

Contributors must update authorization policy and tests whenever they add a route.

## Suggested roles

### Basic application signer

```json
{
  "name":"application-signer",
  "permissions":[
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

Only include `wallet.create` if the application truly provisions wallets.

### Integrity service

```json
{
  "name":"integrity-service",
  "permissions":[
    "sign.integrity",
    "verify.integrity"
  ]
}
```

### Auditor

```json
{
  "name":"auditor",
  "permissions":["audit.read"]
}
```

### Vault operator

```json
{
  "name":"vault-operator",
  "permissions":[
    "vault.status.read",
    "vault.unseal",
    "vault.seal"
  ]
}
```

Do not grant `vault.initialize` after normal initialization unless your operating process has a concrete need for it on a fresh environment.
