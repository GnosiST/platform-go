---
sidebar_position: 4
title: Operations and troubleshooting
---

# Operations and troubleshooting

Start with generated artifacts and read-only preflight, then enter the reviewed change process. Production recovery must not rely on temporary configuration edits, data deletion, or skipped validation.

## Routine checks

```bash
node scripts/validate-platform-foundation-alignment.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/run-platform-production-preflight.mjs --list
```

Use `request_id` to correlate API, application logs, and audit events. Status output exposes only safe aggregates such as `disabled`, `ready`, or `unavailable`; it never returns adapter endpoints, credentials, or upstream error details.

## Diagnosis path

| Symptom | Check first | Safe recovery |
| --- | --- | --- |
| API is unhealthy | Process, bind address, `/api/health` | Restore configuration or process; do not publish internal readiness detail |
| API is not ready | `GET /api/ready`, store/Redis settings, startup logs | Repair verified dependency configuration, restart, then retest |
| Admin has no data | API proxy, session, `request_id` | Confirm authentication and authorization; do not bypass policy with frontend hiding |
| Login fails or OTP is not delivered | Provider discovery, challenge, notification configuration, redacted audit | Create a new challenge/OTP; never render or refill a debug code |
| Message/search unavailable | Capability/profile, named adapter, status | Keep default-off and recover or roll back through a separate change packet |
| Migration/retention failure | Change packet, backup, checkpoint, audit | Stop and use the approved rollback/restore rehearsal |

## Change discipline

Database migration, key rotation, configuration import/restore, retention purge, message replay, and external SMS delivery each require an owner, impact analysis, stop condition, rollback, observability, and redacted evidence. Preflight does not write production and does not replace that evidence.

Start production work with the [production checklist](./production-checklist). See the [support matrix](./support-matrix) for default availability.
