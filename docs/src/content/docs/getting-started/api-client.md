---
title: Live REST API client
description: Use the Scalar client and OpenAPI document served by every running Sigryx instance.
---

Every running Sigryx HTTP server exposes a live API-documentation surface on the **same host and port as the REST API**.

With the default local configuration:

```text
REST API:        http://localhost:8080/v1/...
API client:      http://localhost:8080/docs
OpenAPI JSON:    http://localhost:8080/openapi.json
```

## `/docs`

`GET /docs` serves a small HTML shell that loads the Scalar API Reference client and points it at:

```text
/openapi.json
```

Open it in a browser:

```text
http://localhost:8080/docs
```

You can inspect the exact request/response schemas generated from the Huma route definitions and use the client to exercise the API against that Sigryx instance.

For protected endpoints, supply a bearer token in the client just as you would in any REST client.

## `/openapi.json`

The OpenAPI document is generated from the actual registered Huma operations. This makes it preferable to manually maintained endpoint tables when you need the precise schema for the build you are running.

Fetch it directly:

```bash
curl -sS http://localhost:8080/openapi.json | jq
```

You can feed this document to code generators, REST clients, contract tests, or security tooling.

## Authentication status of the documentation routes

The current Sigryx auth middleware treats both routes as public:

```text
GET /docs
GET /openapi.json
```

They are also skipped by the normal HTTP audit middleware.

This is convenient for developer experience, but it means the deployment boundary matters. If publishing your Sigryx management plane would reveal API shape that you do not want exposed, restrict these paths at your reverse proxy, private network, service mesh, VPN, or ingress layer.

## CDN dependency

The current `/docs` HTML loads Scalar from jsDelivr:

```text
https://cdn.jsdelivr.net/npm/@scalar/api-reference
```

Therefore an isolated browser with no outbound access to that CDN may see an empty/non-functional API client even though `/openapi.json` is available locally. The Sigryx API itself does not depend on the CDN.

For disconnected environments, either use `/openapi.json` with an offline OpenAPI client or vendor/self-host the Scalar frontend in a future deployment customization.

## Static docs site vs. live API client

Do not confuse these two surfaces:

| Surface | Purpose | Source of truth |
| --- | --- | --- |
| Public documentation site | architecture, concepts, operations, tutorials | Markdown in `docs/src/content/docs/` |
| Sigryx `/docs` | interactive REST schema/client | running HTTP handlers |
| Sigryx `/openapi.json` | machine-readable API contract | running Huma API |

Use both. The static docs tell engineers how the system is intended to be used; the live OpenAPI client tells them exactly what a particular build accepts.
