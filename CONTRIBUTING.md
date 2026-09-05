# Contributing to Sigryx

Thank you for improving Sigryx.

Sigryx handles high-value secret material, so changes are reviewed not only for functional correctness but also for secret lifecycle, sealed-state behavior, authorization, auditability, migrations, and recovery implications.

## Start here

Read the full contributor documentation:

- [Development guide](./docs/src/content/docs/contributing/development.md)
- [Documentation guide](./docs/src/content/docs/contributing/documentation.md)
- [Release guide](./RELEASING.md)
- [Architecture](./docs/src/content/docs/concepts/architecture.md)
- [Security model](./docs/src/content/docs/security/security-model.md)

## Local checks

Before opening a pull request:

```bash
make fmt
make vet
make test
make test-race
```

For documentation changes:

```bash
make docs-install
make docs-build
```

## Security-sensitive changes

Do not introduce long-lived ordinary heap copies of Vault keys, plaintext key-root seeds, or derived private keys. New protected routes must have an explicit permission policy. New critical operations must be evaluated for audit classification and retention behavior.

Do not put real credentials, recovery keys, owner-held unseal credentials, JWT secrets, service-account secrets, or private keys in tests, issues, logs, examples, commits, or screenshots.

## API changes

If REST behavior changes:

1. update the Huma request/response documentation tags;
2. add/update tests;
3. inspect the generated `/openapi.json` and `/docs` client locally;
4. update the static HTTP reference and relevant concept/operations pages.

## Database changes

Use the repository's Atlas migration workflow. Do not rewrite an already-released migration. Keep Ent schema and migrations aligned where applicable.

## Pull requests

Keep PRs focused. Explain:

- the problem being solved;
- security/lifecycle implications;
- migration implications;
- sealed/unsealed behavior;
- tests added;
- documentation updated.

By contributing, you agree that your contribution is provided under the repository's GNU Affero General Public License v3.0 unless another contribution agreement is explicitly stated by the project maintainers.
