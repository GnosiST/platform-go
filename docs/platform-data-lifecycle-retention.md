# Platform Data Lifecycle And Retention

> **Implementation status:** Implemented. The runtime, maintenance CLI, disabled-by-default scheduler and generated Admin contracts follow this document; database support remains limited by the separate certification matrix.

## Purpose

This document defines the platform's deletion, recovery, final purge and retention-maintenance contract. It is intentionally resource-specific: deleting a business record, revoking a token and cleaning up a file are different operations and must not be hidden behind a universal soft-delete switch.

## Resource Policy

Every enabled Admin resource must declare one deletion mode. Startup fails when the declaration is missing or inconsistent.

| Mode | Operational meaning |
| --- | --- |
| `disabled` | deletion is not supported |
| `append-only` | records are immutable evidence and ordinary deletion is rejected |
| `restrict` | terminal removal is allowed only when current reference guards pass |
| `soft-delete` | ordinary delete hides the record and opens a bounded restore window |
| `revoke` | delete delegates to the credential or session revocation authority |
| `tombstone` | delete hides metadata and schedules retry-safe external cleanup |
| `hard-delete` | explicitly approved low-risk records are physically removed immediately |

There is no implicit default and no runtime switch that converts all resources to soft delete. Audit records remain append-only, API tokens use authoritative revocation plus retained metadata, files remain tombstone-based, and session invalidation stays in the session store rather than the generic resource lifecycle.

## Lifecycle Metadata

Recoverable records carry platform-owned top-level metadata:

- `deletedAt`: canonical UTC deletion time;
- `deletedBy`: authenticated actor identifier;
- `deleteReason`: normalized reason code;
- `purgeAfter`: earliest final-purge time.

Generic forms and resource values cannot write these fields. In-memory and file snapshots serialize them directly. SQL and GORM persistence use a lifecycle sidecar keyed by resource and record ID, so business values and normalized tables do not acquire hidden deletion fields.

Persistent GORM storage uses `platform_admin_resource_lifecycle` for record lifecycle state and `platform_data_lifecycle_leases`, `platform_data_lifecycle_checkpoints`, `platform_data_lifecycle_impact_reports` and `platform_data_lifecycle_promotions` for maintenance coordination and approval evidence. `platform-admin data-lifecycle --operation prepare` creates or validates these tables before the runner can be enabled.

Normal list, structured query, relation lookup, detail and export paths exclude deleted or tombstoned records. There is no public `includeDeleted` option. Maintenance code uses explicit internal lookup and remains subject to permission and policy checks.

## Operations

### Delete

`DELETE /api/admin/resources/:resource/:id` applies the resource's declared mode:

- `soft-delete` records lifecycle metadata and hides the record;
- `tombstone` hides the record and schedules external cleanup;
- `revoke` calls the authoritative token/session revoker;
- `restrict` performs reference checks before an audited terminal removal;
- `hard-delete` performs an explicitly declared audited physical removal;
- `disabled` and `append-only` reject the request.

Retrying an already completed soft delete or tombstone is idempotent and does not extend the retention window.

### Restore

`POST /api/admin/resources/:resource/:id/restore` requires the resource-specific restore permission. Restore succeeds only for eligible `soft-delete` or `tombstone` records within the restore window.

File restore additionally requires the object to remain present and cleanup not to have started. Restore clears lifecycle state and appends a dedicated audit event in the same persistence transaction.

### Final Purge

Final purge is a separate maintenance operation available only through `platform-admin data-lifecycle` and the disabled-by-default retention runner. There is no generic HTTP purge endpoint. Apply requires the exact current and promoted policy fingerprints, a completed dry-run checkpoint named by `DryRunID`, matching persisted promotion evidence, persistent lease/checkpoint state and an eligible `purgeAfter` value; ordinary delete or restore permission never grants purge authority.

For files, the object must be deleted or confirmed missing before metadata can be purged. External object cleanup is retry-safe and its persistent state survives process failure.

## Retention Maintenance

The scheduled retention runner is disabled by default:

```text
PLATFORM_RETENTION_RUNNER_ENABLED=false
PLATFORM_RETENTION_RUNNER_INTERVAL=24h
PLATFORM_RETENTION_RUNNER_BATCH_SIZE=100
PLATFORM_RETENTION_RUNNER_MAX_RETRIES=3
```

When enabled, production operation requires the GORM-backed Admin resource store so promotions, leases, checkpoints and audit evidence survive restart; memory and file-backed Admin stores fail closed. The runner uses bounded batches, deterministic cursors, one expiring lease, heartbeat renewal and bounded retry. The defaults are a 24-hour interval, 100-record batch and three retries; accepted bounds are `1m..720h`, `1..1000` and `0..5`. Its cursor, lease and report include a datasource identifier; the initial runtime accepts only the default primary datasource and keeps database target rows, lifecycle state, target-local audit and checkpoints in one datasource transaction. It resumes after the last committed cursor and stops on policy drift or lease loss.

Supported maintenance modes are:

- `impact`: report value-free counts for a proposed policy;
- `dry-run`: evaluate the approved active policy without mutation;
- `apply`: perform eligible cleanup only after the named dry-run completed under the approved policy fingerprint.

Reports may contain aggregate counts, timestamps, policy hashes and checkpoints. They must not contain resource values, encrypted envelopes, object keys, PII, DSNs or raw dependency errors.

## Changing Retention

All changes are audited. Lengthening retention or disabling automatic purge follows the reviewed configuration path. Shortening a restore or purge interval is destructive and requires an explicit promotion workflow:

1. generate an impact report for the proposed policy fingerprint;
2. review counts and affected lifecycle classes outside the platform;
3. record actor, reason, approval reference and the immutable impact-report hash;
4. promote exactly that policy fingerprint;
5. complete dry-run and record its immutable run ID;
6. apply with that `DryRunID`, the reviewed current fingerprint and the same promoted fingerprint.

The runner refuses manifest, report, promotion or checkpoint fingerprint drift. Deploying a shorter value alone cannot purge historical records.

## File Recovery

File deletion no longer means immediate object removal. The expected sequence is:

```text
delete request -> tombstoned and hidden -> restore window
                                     -> cleanup claimed -> object removed -> metadata purged
```

During the restore window, content remains inaccessible through normal Admin and App routes. A valid restore makes the metadata and content visible again. Once cleanup is claimed, restore is rejected even if the object-storage call has not yet completed.

Object-storage `not found` is treated as an idempotent cleanup result. Other storage errors keep the tombstone and retry state; they never cause metadata to be presented as active.

## Tokens And Sessions

Deletion policy does not weaken authentication:

- deleting an API token invokes immediate server-side revocation and retains the revoked metadata according to its domain policy;
- logging out or invalidating a session invokes the session repository's revocation path;
- later retention cleanup may remove already-terminal records but never serves as the revocation boundary.

## Failure And Incident Rules

Stop mutation and preserve sanitized reports when any of the following occurs:

- policy fingerprint, promotion or checkpoint drift;
- lease loss or concurrent runner ownership;
- sidecar/business-record revision conflict;
- audit persistence failure;
- file cleanup state conflict or repeated object-storage failure;
- a deleted record appears in a normal query, relation or export;
- a report or log exposes values, encrypted envelopes, object keys, identifiers beyond the approved report contract, DSNs or PII.

Do not bypass restore deadlines, edit checkpoints, delete sidecar state manually or use hard delete as an incident shortcut. Use the reviewed database backup and external object-storage recovery procedures when platform-level restore is no longer eligible.

## Migration Notes

- Existing records with no sidecar row are active.
- All manifests must be classified before the generic delete route changes behavior.
- Existing file tombstones require an inventory: records whose objects were already removed are not recoverable.
- Existing revoked API tokens remain revoked; migration must not reactivate them.
- Sidecar and runner-state tables are created by reviewed migrations, not lazily by ordinary read paths.

## Support Boundaries

This capability does not promise that every resource is recoverable. It provides no general-purpose archive tier or online purge API, and it does not replace backups, external archive systems, object-storage lifecycle configuration, legal holds or credential revocation. The current runner is deliberately single-datasource and cannot switch datasource during a transaction. It does not certify Oracle, KingbaseES or any other database, and passing local SQL/GORM tests is not production database certification.

Retention defaults are platform engineering defaults, not legal advice or evidence of regulatory compliance. Each downstream deployment must define and approve its own contractual and regulatory retention periods before enabling automatic purge.

## Release Evidence

The node may be marked implemented only when evidence proves:

- every enabled resource has an explicit valid lifecycle declaration;
- normal reads hide recoverable-deleted records consistently;
- delete and restore have separate Admin permissions, while final purge is isolated behind the maintenance command/runner and atomic audit behavior;
- file and SQL/GORM reload preserve lifecycle state;
- file recovery and retry-safe final cleanup pass failure-injection tests;
- token/session invalidation remains immediate and authoritative;
- the runner is disabled by default and resumes through lease/checkpoint state;
- shorter-retention promotion cannot be bypassed;
- generated contracts, OpenAPI, governance validators, full Go tests and CodeGraph checks pass.
