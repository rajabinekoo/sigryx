# Security Policy

Sigryx is key and signing infrastructure. Security reports may contain information that should not be disclosed publicly before a fix is available.

## Reporting a vulnerability

Please **do not open a public GitHub issue containing exploit details, credentials, private keys, recovery material, owner-held unseal credentials, or instructions that would expose a deployed Sigryx instance**.

Use GitHub's private vulnerability reporting/security-advisory mechanism for this repository when it is enabled. If the repository exposes another private security contact in its GitHub Security page, use that channel instead.

Include only the information needed to reproduce and assess the issue:

- affected version/commit;
- deployment assumptions;
- affected endpoint/component;
- reproduction steps or minimal proof of concept;
- expected vs. observed security behavior;
- impact;
- suggested mitigation, if known.

Sanitize all samples. Never send production secrets.

## Security scope

Examples of issues that are especially relevant include:

- recovery of plaintext Vault/key-root/child-key material outside its intended lifecycle;
- bypass of sealed-state enforcement;
- authentication or authorization bypass;
- CIDR/proxy trust bypass;
- signing with a wallet/root the caller is not authorized to use;
- integrity Signing Record tampering or replay weaknesses;
- cryptographic misuse that changes signed semantics;
- recovery bundle/key handling vulnerabilities;
- secret leakage through logs, serialization, errors, traces, core dumps, or API responses;
- audit bypass for security-sensitive operations;
- database paths that bypass append-only security controls unexpectedly.

## Deployment responsibility

Sigryx's security also depends on its deployment environment. PostgreSQL access control, TLS termination, host/container isolation, secret injection, backup protection, trusted proxy configuration, and access to `/docs`/`/openapi.json` are operator responsibilities described in the production documentation.

See:

- [Security model](./docs/src/content/docs/security/security-model.md)
- [Production deployment](./docs/src/content/docs/operations/production.md)
- [Recovery](./docs/src/content/docs/operations/recovery.md)
