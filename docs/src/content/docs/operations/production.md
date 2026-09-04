---
title: Production deployment
description: Recommended topology, TLS, database controls, startup/unseal workflow, observability, backup, and rollout checks.
---

The repository's development compose file is not a complete production topology. Production deployment should treat Sigryx as a sensitive control-plane service.

## Recommended topology

```text
application workloads
        │
        │ TLS / private network
        ▼
load balancer / reverse proxy
        │
        │ trusted proxy CIDR configured
        ▼
┌──────────────────────┐
│ Sigryx instance      │
│ sealed at process    │
│ start/restart        │
└──────────┬───────────┘
           │ TLS/private network
           ▼
┌──────────────────────┐
│ PostgreSQL           │
│ encrypted roots     │
│ auth/audit metadata │
└──────────────────────┘

optional:
Sigryx -> HTTPS security alert webhook
```

## TLS

The current Go HTTP server is composed as a plain HTTP listener. Terminate TLS at a trusted reverse proxy/ingress or extend the server configuration before exposing it across an untrusted network.

Never send these over plaintext networks:

- bearer/refresh tokens;
- `owner_key` unseal material;
- service-account credentials;
- recovery key/backup;
- sensitive signing payloads.

## Process identity

Run Sigryx with:

- a dedicated non-root OS/container identity;
- a read-only filesystem where practical;
- a writable area only where actually required;
- minimal Linux capabilities;
- restricted ptrace/debug permissions;
- core dumps disabled;
- resource limits appropriate for the workload.

The secure-memory subsystem may require OS support and libsodium. Validate the actual production image/runtime, not only local Go tests.

## PostgreSQL identity

Use a dedicated database role with only the privileges Sigryx needs.

The audit-retention design depends on the `sigryx_purge_audit_events` function and append-only triggers. Avoid granting application identities PostgreSQL superuser/bypass privileges that make these controls meaningless.

## Startup runbook

A normal restart should look like:

```text
1. start PostgreSQL
2. apply/verify migrations
3. start Sigryx
4. health becomes available
5. authenticate operators
6. confirm Vault state = SEALED
7. submit all N unseal credentials
8. confirm state = UNSEALED
9. enable/confirm dependent signing workloads
```

Do not configure the service to silently embed all owner credentials into the same runtime environment just to automate step 7; that collapses the unseal custody boundary.

## Readiness semantics

`GET /v1/health` currently reports process HTTP health, not "Vault is unsealed and ready to sign".

A workload that requires signing should separately inspect:

```text
GET /v1/vault/status
```

and treat `UNSEALED` as the cryptographic readiness condition.

## Authentication bootstrap

Before production exposure:

1. set a high-entropy `SETUP_TOKEN`;
2. bootstrap the Root Admin once;
3. store returned credentials securely;
4. optionally rotate password and configure a Root Admin CIDR allowlist;
5. remove/empty `SETUP_TOKEN` after bootstrap if your secret-delivery process permits it;
6. create role-scoped users/service accounts;
7. stop using Root Admin for normal application traffic.

## Service-account design

Prefer one service account per workload and environment:

```text
payments-prod-signer
ledger-prod-integrity
operations-prod-auditor
```

Avoid one global client secret shared across all services.

## Audit configuration

Choose retention before production, not after the table becomes large.

A common baseline:

```dotenv
AUDIT_NORMAL_RETENTION_DAYS=30
AUDIT_CRITICAL_RETENTION_DAYS=365
AUDIT_CLEANUP_INTERVAL=6h
AUDIT_CLEANUP_BATCH_SIZE=5000
```

If organizational policy requires indefinite critical evidence:

```dotenv
AUDIT_CRITICAL_RETENTION_DAYS=0
```

Export audit events externally if centralized retention/search is required.

## Alerting

Configure `ALERT_WEBHOOK_URL` if integrity incidents must immediately reach a SIEM/security automation endpoint.

Monitor both:

```text
security.integrity_violation
security.alert_delivery_failed
```

The second event means Sigryx detected something worth alerting on but could not deliver the external notification.

## Backups

Use both:

1. regular PostgreSQL backups;
2. periodic tested Sigryx recovery exports after key-root changes.

Store recovery backup and recovery key separately.

## Upgrade procedure

Before upgrading:

- read migrations;
- back up PostgreSQL;
- make a fresh recovery export if key roots exist;
- stage the upgrade against a copy of production data where possible;
- run tests and migration lint;
- verify API/OpenAPI changes;
- verify audit classification/retention behavior;
- verify seal/unseal and signing after restart.

## Documentation exposure

The runtime routes:

```text
/docs
/openapi.json
```

are public to Sigryx's own auth middleware. If they should not be visible to all clients that can reach the service, deny/restrict them at your ingress layer.

## Release gate checklist

Before calling a deployment production-ready, verify at least:

- migrations applied cleanly;
- Root Admin access tested and stored;
- Root Admin not used by application workloads;
- unseal ceremony documented and rehearsed;
- recovery export/import tested;
- TLS/private-network controls active;
- trusted proxy CIDRs correct;
- service-account CIDRs/roles correct;
- backup restore tested;
- audit retention configured;
- alert webhook tested if enabled;
- restart returns to SEALED;
- seal destroys signing ability;
- unseal restores signing ability;
- no secrets appear in logs/traces.
