# Data Lifecycle And Retention Design

> **Status:** Implemented and verified.

## Goal

Replace generic physical deletion with explicit, fail-closed lifecycle semantics declared by every enabled Admin resource. Recoverable records must support audited soft delete and restore, final purge must remain a separate maintenance operation, files must keep a real recovery window, and retention maintenance must be disabled by default and resumable when enabled.

## Current Baseline

- Generic resources call `Store.Delete` or `DeleteWithAudit` and physically remove the record.
- Files use an internal tombstone but immediately delete the object and purge metadata in the same HTTP request, so there is no usable restore window.
- API tokens are retained and authoritatively revoked. Login sessions use the session store's revocation semantics.
- `Record` has no top-level lifecycle metadata. Normal list and query paths only hide file tombstones.
- The memory, file, SQL and GORM repositories persist full resource snapshots. GORM also projects normalized resources into dedicated tables.

These facts are migration inputs, not behavior to preserve unchanged.

## Manifest Contract

`capability.AdminResource` gains one required lifecycle declaration. Missing or invalid declarations fail capability validation and process startup; an undeclared resource never inherits physical deletion.

```go
type AdminResource struct {
    // existing fields omitted
    Deletion *AdminResourceDeletionPolicy
}

type AdminResourceDeletionPolicy struct {
    Mode               string
    PolicyVersion      uint32
    RetentionDays      int
    AutoPurge          bool
    RestrictReferences bool
}
```

Supported `DeleteMode` values are:

| Mode | Interactive delete | Restore | Maintenance purge | Intended use |
| --- | --- | --- | --- | --- |
| `disabled` | rejected | no | no | resources with no deletion contract |
| `append-only` | rejected | no | only a separately governed archive/compliance process | audit and immutable evidence |
| `restrict` | allowed only when reference guards pass, then audited physical removal | no | no | referenced master data explicitly approved for terminal removal |
| `soft-delete` | sets lifecycle metadata | within the declared window | after `purgeAfter` and approval | recoverable business and platform records |
| `revoke` | delegates to the authoritative credential/session revoker | no | domain retention only | API tokens and sessions |
| `tombstone` | hides the record and schedules external cleanup | before cleanup starts and within the window | after external cleanup succeeds | files and external objects |
| `hard-delete` | audited physical removal | no | no | explicitly reviewed low-risk resources only |

Validation rules:

- Every enabled resource declares exactly one mode, including non-deletable resources.
- `PolicyVersion` is positive and changes whenever lifecycle semantics change.
- `RetentionDays` is non-negative. For `soft-delete` and `tombstone`, it is both the recovery window and the earliest purge interval; for `revoke`, it applies only after authoritative revocation.
- `AutoPurge=true` requires a positive retention interval and is valid only for a mode with a defined retention clock.
- `disabled`, `restrict` and `hard-delete` reject retention and automatic-purge settings.
- `RestrictReferences=true` is valid only for `soft-delete` or `restrict`; it never enables cascade deletion.
- Audit resources must be `append-only`; `api-tokens` must be `revoke`; `files` must be `tombstone`.
- `hard-delete` is never a compatibility default. Its use requires an explicit manifest declaration and contract test.

An `append-only` resource may declare a separately governed retention window for immutable records, but the implementation must define its stable retention-clock field and archive/legal-hold boundary before enabling automatic purge. A missing clock or hold decision fails closed.

## Record And Persistence Model

Lifecycle metadata is first-class record state, not user-editable values:

```go
type Record struct {
    // existing fields omitted
    DeletedAt    string `json:"deletedAt,omitempty"`
    DeletedBy    string `json:"deletedBy,omitempty"`
    DeleteReason string `json:"deleteReason,omitempty"`
    PurgeAfter   string `json:"purgeAfter,omitempty"`
}
```

- External create and update inputs cannot set or clear lifecycle fields.
- Normal list, query, relation lookup, detail lookup and export paths exclude records with `DeletedAt != ""` before projection.
- Internal lifecycle and maintenance operations may load deleted records through explicit APIs; generic `InternalRecord` remains unsuitable as an authorization boundary.
- Restore clears all four fields atomically and records a separate audit event.
- Final purge physically removes the record only after eligibility, permission and policy checks pass.

The file and in-memory snapshot formats serialize these top-level fields. SQL and GORM repositories use a sidecar lifecycle table keyed by `(resource, record_id)` rather than placing platform metadata in `ValuesJSON` or adding lifecycle columns to every normalized business table. Snapshot load joins sidecar state into `Record`; snapshot save updates records and sidecar rows under the same revision transaction. A missing sidecar row means active, not deleted.

Sidecar rows contain lifecycle state only. They do not duplicate encrypted values, business fields or object-storage credentials.

## Delete, Restore And Purge Semantics

The store exposes policy-aware mutations rather than callers selecting physical deletion:

```go
DeleteWithAudit(resource, id, request, event)
RestoreWithAudit(resource, id, request, event)
PurgeWithAudit(resource, id, request, event)
```

- Delete is idempotent for an already soft-deleted or tombstoned record and does not move `purgeAfter` forward on retry.
- Delete requires a normalized actor and reason code. User-entered free text is not copied into audit payloads.
- `restrict` checks current declared relations before removal. It never cascades or silently edits referring records.
- Restore is rejected after the restore deadline, after file cleanup begins, or when the resource mode does not support it.
- Purge is never an alias of ordinary delete. It is an internal maintenance mutation that requires an eligible record and a reviewed runner policy snapshot.
- Mutation and append-only audit save together or roll back together for repository-backed resources.

HTTP contracts remain separate:

- `DELETE /api/admin/resources/:resource/:id` applies the declared delete or revoke semantic.
- `POST /api/admin/resources/:resource/:id/restore` restores one eligible record.
- There is no generic online purge route. Final purge is available only through the maintenance CLI/runner so approval, lease, checkpoint and policy-fingerprint checks cannot be bypassed by a request handler.

Generated schemas expose the declared mode and dedicated `restore` permission string. Purge remains a maintenance capability rather than an Admin resource action. Backend permission checks remain authoritative. Normal query endpoints never accept an `includeDeleted` escape hatch.

## Files

File deletion becomes a recoverable state machine:

```text
active -> tombstoned -> cleanup-started -> object-deleted -> purged
            | restore
            +--------> active
```

- Interactive delete tombstones metadata, hides content and records `purgeAfter`; it does not immediately delete the object.
- Restore is allowed only while the object is still present, cleanup has not started and the restore window is open.
- The runner claims cleanup before calling object storage. Object-not-found is an idempotent cleanup success.
- External object deletion and metadata purge cannot share one database transaction. Persistent cleanup state and retry-safe transitions close that gap.
- Metadata remains tombstoned if object deletion fails. It is purged only after object cleanup is durably recorded.

## Tokens And Sessions

- API token delete continues to call the authoritative token revocation path and retains the revoked record according to its declared domain retention.
- Login and App session invalidation continues to call the session store. Generic Admin resource soft delete is never used to invalidate credentials.
- Retention cleanup may later remove already-expired or already-revoked session/token records, but it cannot be the security boundary for revocation.

## Retention Runner

The maintenance runtime is disabled by default. Enabling it requires explicit production configuration and a persistent repository; memory-only execution is limited to tests.

The runner supports:

- `impact`: value-free counts for a candidate policy change;
- `dry-run`: evaluate current approved policy without mutation;
- `apply`: bounded, audited cleanup using the approved policy snapshot;
- deterministic resource/eligibility/record cursors;
- one persistent lease with owner, expiry and heartbeat;
- bounded batch size and retry count;
- idempotent replay after process or dependency failure;
- a policy fingerprint on every report, lease and checkpoint.

The runner partition key includes `datasourceID`. The initial implementation uses only the default primary datasource, but lifecycle rows, target-local audit, leases and checkpoints must remain on that same datasource. This preserves the future named-datasource boundary without introducing XA or routing current records across databases.

Each database-only batch atomically writes lifecycle mutations, audit events and the committed cursor. File cleanup uses the persistent state machine above and advances the cursor only after the terminal metadata result is durable. Lease loss stops new work and never causes the runner to skip an uncommitted item.

Reports contain counts, policy fingerprint, cursor/checkpoint and sanitized failure categories. They exclude record values, encrypted envelopes, object keys, PII and arbitrary dependency errors.

## Retention Policy Promotion

Every policy revision is audited. A longer retention window or disabling automatic purge may be promoted through the normal reviewed configuration path. A shorter `RetentionDays` value is destructive and cannot become active merely because a new manifest or environment value was deployed.

Shortening retention requires:

1. generate an `impact` report against the current data and proposed fingerprint;
2. record actor, reason, approval reference and immutable report hash;
3. explicitly promote that exact fingerprint;
4. run `dry-run` against the promoted snapshot before `apply`;
5. reject apply when manifest, promotion, report or checkpoint fingerprints differ.

The impact report is not a legal hold decision. Downstream deployments remain responsible for organization-specific legal, contractual and regulatory retention requirements.

## Migration And Compatibility

- Existing records start active because they have no sidecar lifecycle row.
- Existing generic delete endpoints change behavior only after all manifests carry valid declarations and generated contracts are updated together.
- Existing file tombstones must be classified before enabling restore. A tombstone whose object was already deleted is cleanup-complete and cannot be presented as recoverable.
- Existing revoked API tokens remain revoked and visible according to the token resource contract.
- Sidecar creation and policy promotion require explicit migrations; ordinary query code must not lazily create lifecycle tables.

## Boundaries

- This design does not make every resource soft-deletable.
- This design does not claim database portability or certification beyond the repository behavior actually tested by this node.
- This design does not replace credential revocation, object-storage lifecycle rules, backups, legal holds or external archive systems.
- This design does not add a public `includeDeleted` query parameter or allow clients to edit lifecycle metadata.
- This design does not certify compliance with any law, industry regime or customer retention policy.

## Acceptance

- Capability validation fails when any enabled resource omits or misconfigures lifecycle policy.
- Normal reads, relations and exports hide soft-deleted and tombstoned records consistently.
- Soft delete, restore and purge are distinct, permissioned and atomically audited operations.
- GORM and SQL reload preserve top-level lifecycle state through the sidecar model.
- Files remain recoverable during the declared window and final cleanup is retry-safe.
- Token/session delete semantics remain authoritative revoke operations.
- The runner is disabled by default and resumes through bounded batches, durable cursors, a lease and retries.
- A shorter retention policy cannot apply without a matching impact report and explicit promotion.
- No completion claim implies universal soft delete, database certification, legal compliance or replacement of external backup/archive controls.
