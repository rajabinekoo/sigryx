---
title: Secure memory
description: How securemem.Secret owns high-value bytes and why callback-scoped access matters.
---

Sigryx uses `pkg/securemem` for high-value runtime secret material.

The central type is:

```go
type Secret struct { ... }
```

Its purpose is ownership, lifetime control, page protection, and explicit destruction.

## Ownership transfer

Creating a secret from a Go slice transfers ownership:

```go
secret, err := securemem.New(data)
```

The input slice is wiped before `New` returns, including failure paths after ownership transfer begins.

For newly generated secrets, Sigryx prefers:

```go
securemem.Random(size)
```

because it creates random bytes directly in the protected region instead of generating a plaintext Go-heap slice first.

This is used for values such as HD master seeds and real unseal keys.

## Callback-only access

Secret bytes are exposed using:

```go
secret.WithBytes(func(value []byte) error {
    // use value here
    return nil
})
```

The callback slice:

- must not escape;
- must not be stored;
- must not be passed to another goroutine;
- must not be modified.

The protected memory region becomes inaccessible again after the callback returns.

## Synchronization

`Secret.WithBytes` uses an exclusive mutex. This is intentional because the implementation changes memory-page protections around access. Concurrent readers would otherwise race on page permissions.

Treat a `Secret` as an exclusive cryptographic resource rather than a normal immutable byte slice.

## Destruction

```go
secret.Destroy()
```

permanently invalidates the protected region. Destruction is idempotent.

`SecretStore.Clear()` is the larger lifecycle operation used during Vault sealing and process shutdown to destroy all secrets owned by the store.

## SecretStore responsibilities

The current store owns:

- partial real unseal keys during an unseal attempt;
- the Vault Encryption Key after successful unseal;
- cached plaintext HD root seeds.

This prevents each service from inventing its own secret lifetime rules.

## Loading an encrypted root

A typical path looks like:

```text
PostgreSQL encrypted root
        │
        ▼
WithVaultEncryptionKey(callback)
        │ AES-256-GCM Open
        ▼
plaintext []byte (short-lived transition)
        │ ownership transfer
        ▼
securemem.Secret
        │
        ▼
WithKeyRootSeed(callback)
```

The design tries to keep plaintext transitional buffers short-lived and explicitly wiped.

## Derived child private key

The HD wallet package derives a child private key inside a callback:

```go
hdwallet.DerivePrivateKey(seed, path, func(privateKey []byte) error {
    return sign(privateKey)
})
```

Intermediate BIP32 private-key and chain-code buffers are cleared during derivation.

The child private key is not inserted into a wallet database row or returned to the HTTP caller.

## Why `[]byte` fields are dangerous

A long-lived field such as:

```go
recordKey []byte
```

would create a normal Go-heap copy with a lifetime tied to the service object rather than Vault state.

For this reason, Vault-protected operations access the current key through `SecretStore.WithVaultEncryptionKey` instead of caching it as a service field.

The Signing Record implementation follows this rule: it encrypts/decrypts records inside Vault-key callbacks.

## Limits

Secure memory improves control over accidental copies, swapping/page access, and explicit destruction, but you must still assume:

- compiler/runtime behavior outside the protected region matters;
- temporary data may exist in CPU registers/stacks during cryptographic operations;
- a privileged attacker can potentially defeat process-local memory defenses;
- serialized plaintext in logs, request dumps, panic output, or tracing would bypass secure-memory protections entirely.

Never log secret-bearing request bodies.

## Contributor rules

When touching secret-bearing code:

1. prefer `securemem.Random` for fresh secret generation;
2. transfer ownership explicitly;
3. use callback-scoped access;
4. wipe unavoidable transitional Go slices;
5. never format secrets with `%v`, `%s`, JSON, or structured logging;
6. never return private key or seed bytes from an API;
7. bind encrypted records with meaningful AAD;
8. make failure paths destroy material too;
9. add tests for destruction/ownership when changing lifecycle logic.
