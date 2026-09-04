---
title: Audit & retention
description: Audit event generation, append-only controls, NORMAL/CRITICAL classification, retention worker, and purge behavior.
---

Sigryx records security and control-plane activity into `audit_events`.

The audit system is designed around two goals that normally conflict:

1. make audit rows append-only during ordinary operation;
2. prevent unbounded database growth through controlled retention.

## What gets audited

The HTTP middleware records most application routes after the request completes. It captures fields such as:

```text
occurred_at
actor_type
actor_id
session_id
action
outcome
source_ip
request_id
method
path
status_code
details
retention_class
```

The following read-only infrastructure routes are intentionally skipped:

```text
GET /v1/health
GET /docs
GET /openapi.json
```

Integrity incidents can also append dedicated security events directly from the signing service.

## Outcomes

HTTP results are mapped to audit outcomes:

```text
2xx/3xx     -> SUCCESS
401/403     -> DENIED
other >=400 -> FAILED
```

Special integrity violations use `BLOCKED`.

## Retention classes

Every event is classified as:

```text
NORMAL
CRITICAL
```

The default policy marks security-sensitive action families critical, including:

```text
audit.retention_cleanup
security.*
sign.*
recovery.*
vault.* except vault.status
keyroot.* except keyroot.list
access.* except *.list
auth.setup
auth.service_token
auth.update_me
wallet.create
```

Routine actions such as health-independent reads, normal login/refresh activity, verification, and audit listing default to `NORMAL` unless explicitly overridden.

## Retention configuration

```dotenv
AUDIT_NORMAL_RETENTION_DAYS=30
AUDIT_CRITICAL_RETENTION_DAYS=365
AUDIT_CLEANUP_INTERVAL=6h
AUDIT_CLEANUP_BATCH_SIZE=5000
```

`0` days means retain that class forever.

Examples:

```dotenv
# Keep everything forever.
AUDIT_NORMAL_RETENTION_DAYS=0
AUDIT_CRITICAL_RETENTION_DAYS=0
```

```dotenv
# Short routine history, two years for critical events.
AUDIT_NORMAL_RETENTION_DAYS=14
AUDIT_CRITICAL_RETENTION_DAYS=730
```

## Worker lifecycle

When at least one retention class has a positive retention period, the process starts an internal retention worker.

It runs:

```text
once at startup
then every AUDIT_CLEANUP_INTERVAL
```

For each enabled class, the cutoff is computed from the worker's current UTC time:

```text
NORMAL cutoff   = now - normal retention days
CRITICAL cutoff = now - critical retention days
```

## Bounded purge batches

The service repeatedly asks the repository to delete at most `AUDIT_CLEANUP_BATCH_SIZE` rows until a partial batch is returned.

The PostgreSQL purge function selects candidates using:

```text
retention_class
occurred_at
id
```

with ordered batching and `FOR UPDATE SKIP LOCKED`.

This avoids one giant unbounded `DELETE` transaction when a deployment has accumulated a large backlog.

The hard validation range for a batch is:

```text
1..50000
```

## Append-only enforcement

Normal database mutations are rejected by a trigger:

```sql
UPDATE audit_events ...;   -- rejected
DELETE FROM audit_events;  -- rejected
TRUNCATE audit_events;     -- rejected
```

The retention worker uses the dedicated database function:

```text
public.sigryx_purge_audit_events(...)
```

That function temporarily sets a transaction-local guard used by the trigger, performs only the bounded eligible delete, and turns the guard back off even on failure.

This is a sanctioned lifecycle operation, not a general mutation bypass.

## Retention cleanup is itself audited

When a cleanup cycle deletes at least one row, Sigryx appends a new critical event:

```text
action: audit.retention_cleanup
actor:  SYSTEM
outcome: SUCCESS
```

with details including deleted counts and configured retention periods.

This provides evidence that historical data disappeared because of the configured lifecycle policy.

## Reading audit events

Endpoint:

```text
GET /v1/audit/events?page=1&limit=50
```

Pagination rules:

- `page` starts at 1;
- default `limit` is 50;
- maximum `limit` is 200.

Response shape:

```json
{
  "items": [
    {
      "id": "...",
      "occurred_at": "2026-09-04T08:00:00Z",
      "actor_type": "USER",
      "actor_id": "...",
      "action": "sign.transaction",
      "outcome": "SUCCESS",
      "source_ip": "10.0.0.10",
      "request_id": "...",
      "method": "POST",
      "path": "/v1/sign/transaction",
      "status_code": 200,
      "retention_class": "CRITICAL"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 50
}
```

## Capacity planning

Retention controls row age, not instantaneous write volume. Estimate event rate separately:

```text
rows/day = requests/day that produce audit events + internal security events
```

Monitor:

- total audit rows;
- table/index size;
- cleanup deleted counts;
- cleanup failures;
- PostgreSQL WAL and autovacuum behavior;
- growth rate by retention class.

For much larger deployments, time partitioning may eventually be more efficient than row deletion, but the current V1 design uses bounded deletes and indexed cutoffs.

## Compliance note

Retention periods are operational policy, not legal advice. A deployment subject to regulatory retention requirements should configure values accordingly and may choose `0` for indefinite retention or export audit events to an external immutable system before local expiry.
