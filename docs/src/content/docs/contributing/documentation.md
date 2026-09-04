---
title: Documentation guide
description: How Sigryx documentation is authored, previewed, reviewed, and published to GitHub Pages.
---

Sigryx has **two complementary documentation surfaces**:

1. this version-controlled engineering documentation site under `docs/`;
2. the live interactive REST API client exposed by every running Sigryx server at `/docs`.

Do not confuse them. The static site explains architecture, security, operations, and workflows. The runtime `/docs` client is generated from the actual Huma HTTP API and is the best interactive view of the concrete REST schema for the running binary.

## Technology

The static site uses:

```text
Astro
+ Starlight
+ Markdown
+ a small custom stylesheet
```

The source tree is:

```text
docs/
  astro.config.mjs
  package.json
  src/
    content.config.ts
    styles/custom.css
    content/docs/
      index.md
      usage.md
      getting-started/
      concepts/
      security/
      operations/
      reference/
      contributing/
```

## Local preview

Install dependencies:

```bash
make docs-install
```

Start the development site:

```bash
make docs-dev
```

or directly:

```bash
cd docs
npm install
npm run dev
```

Build the static site:

```bash
make docs-build
```

The generated files are written to `docs/dist/` and are intentionally ignored by Git.

## Public GitHub Pages deployment

The repository includes `.github/workflows/docs.yml`.

After GitHub Pages is configured to use **GitHub Actions** as its source, pushes to `main` that change the documentation or the deployment workflow build and publish the site.

For the canonical repository name used by the checked-in config, the default project-site shape is:

```text
https://rajabinekoo.github.io/sigryx/
```

The Astro config supports overrides for forks or a custom domain:

```text
SIGRYX_DOCS_SITE
SIGRYX_DOCS_BASE
```

For example, for a root custom domain:

```text
SIGRYX_DOCS_SITE=https://docs.sigryx.dev
SIGRYX_DOCS_BASE=/
```

If your GitHub owner/repository path differs, either update the defaults in `astro.config.mjs` or set the environment variables in the deployment environment.

## Runtime API documentation

The Go server separately exposes:

```text
GET /docs
GET /openapi.json
```

The `/docs` HTML points the Scalar API Reference client at `/openapi.json` on the same server. This makes it possible to inspect and exercise the REST API while running Sigryx locally or in an environment where the route is exposed.

Example:

```text
Sigryx:        https://vault.example.internal
API client:    https://vault.example.internal/docs
OpenAPI JSON:  https://vault.example.internal/openapi.json
```

Those routes are public in the current application middleware. Production operators may restrict them at an ingress/reverse proxy if exposing API metadata is undesirable.

The current `/docs` HTML loads the Scalar JavaScript frontend from jsDelivr. A browser in a fully disconnected network may therefore be unable to render the interactive client even though `/openapi.json` remains available from Sigryx.

## Writing principles

Sigryx is security infrastructure. Documentation must distinguish:

- **implemented behavior** from roadmap/design intent;
- **encrypted-at-rest** from protected-in-memory;
- **authentication** from authorization;
- **verification** from signing;
- **process health** from secret readiness;
- **backup/recovery of key roots** from full database backup;
- **append-only policy** from the controlled audit-retention purge path.

Avoid claims such as “private keys can never be stolen” or “memory cannot be read.” Describe the concrete mechanism and its boundary instead.

## Documentation for a new feature

For every externally visible feature, answer these questions:

### What does it do?

Give the developer a short mental model before implementation detail.

### What state does it require?

State explicitly whether it works while:

```text
UNINITIALIZED
SEALED
UNSEALED
```

### Who can call it?

Name the exact permission or say Root Admin/public.

### What secrets are involved?

Explain which values are returned once, persisted encrypted/hashed, held only in protected memory, or never persisted.

### How is it used?

Give a copyable `curl` example using the actual field names/enums.

### How does it fail?

Document important conflict/validation/authentication behavior and whether retry is sensible.

### How is it audited?

State whether it produces normal or critical audit history and whether retention applies.

### How is it operated?

Document new env variables, migration requirements, monitoring signals, and backup implications.

## Keep examples executable

Prefer examples that can be copied directly:

```bash
curl -sS -X POST "$SIGRYX/v1/example" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"example":"value"}' | jq
```

When an example includes generated IDs/tokens, make the placeholder explicit rather than inventing something that looks real.

Use the exact current enum spelling. For example, the current wallet profile is:

```text
ETHEREUM
```

and the current durable adapter identifier returned by wallet creation is:

```text
evm
```

## Validate against the running API

After changing a Huma route:

```bash
make run-server
```

Then inspect:

```bash
curl -sS http://localhost:8080/openapi.json | jq
```

and open:

```text
http://localhost:8080/docs
```

Check request fields, required flags, enum values, operation description, and response shape.

The static `reference/http-api.md` page should agree with the generated API. When in doubt about a particular binary, its own `/openapi.json` is authoritative for shape.

## Link style

Use site-root documentation links:

```md
[Seal & unseal](/concepts/seal-unseal/)
```

The project includes a small Remark transform in `astro.config.mjs` that prefixes root-relative Markdown documentation links with the configured Astro `base` during the build. This keeps authoring readable while allowing the same content to work at `/` locally and under `/sigryx` on the default GitHub Pages project site.

Avoid hardcoding `/sigryx` into content links. Runtime Sigryx URLs such as `/docs` should generally be shown as code/text rather than static-site links.

## Diagrams

Text/ASCII diagrams are deliberately used throughout the current docs because they:

- render in GitHub diffs;
- remain searchable;
- do not introduce binary assets;
- are easy to review when architecture changes.

If visual diagrams are added later, keep a text description next to them and commit the source format used to generate the image.

## Review checklist

```text
[ ] Is every claim true for the current code?
[ ] Are route names and JSON fields exact?
[ ] Are permissions exact?
[ ] Is sealed/unsealed behavior stated?
[ ] Are one-time secrets called out clearly?
[ ] Are examples safe (no real credentials)?
[ ] Does the static API reference agree with /openapi.json?
[ ] Do internal links resolve?
[ ] Does `make docs-build` succeed?
[ ] Did the PR avoid presenting roadmap features as implemented?
```

Good documentation is part of Sigryx's security model: operators and application developers must understand exactly which component owns a secret and which guarantees an API does—and does not—provide.
