# Platform Production Persistence Correctness Design

Date: 2026-07-10
Status: implemented

## Purpose

Make repository-backed admin resources and sessions correct when multiple API
processes share the same database. The database must be authoritative, writes
must not replace unrelated state, and cross-process invalidation must refresh
process-local read state.

This is the first optimization stage after the platform foundation assessment.
It addresses the highest-risk production correctness gap before public
capability contracts, lifecycle migration ownership, large-module deepening, or
delivery automation.

## Pre-Implementation Failure Mode (Historical)

Before this slice, the repository interfaces persisted whole in-memory
snapshots:

- `adminresource.AdminResourceRepository` exposed `Load` and `Save` for every
  resource at once.
- `session.Repository` exposed `Load` and `Save` for every session at once.
- GORM `Save` implementations deleted complete tables and recreated them from
  the calling process snapshot.
- Redis invalidation reloaded peer session stores, but admin resource events
  only cleared derived caches.
- The distributed admin invalidation test gave writer and reader servers the
  same in-memory `adminresource.Store`, so it did not represent two production
  processes.

Two processes could therefore load the same initial state, apply different
changes, and overwrite each other. Admin resource readers could also keep stale
process-local records after a peer write.

## Goals

- Prevent unrelated session records from being deleted by issue, renew, or
  revoke operations.
- Prevent stale admin resource snapshots from overwriting newer database state.
- Make peer admin resource writes visible to separately constructed stores.
- Preserve memory and file-backed local development modes.
- Keep existing HTTP and frontend contracts unchanged.
- Add regression tests that use independent stores sharing one GORM database.
- Keep the change focused on correctness; avoid a broad persistence rewrite.

## Non-Goals

- No public capability package extraction in this stage.
- No capability lifecycle or migration framework redesign in this stage.
- No HTTP handler or admin console decomposition in this stage.
- No Redis integration test requiring an external Redis process.
- No source-writing code generation promotion.

## Approaches Considered

### 1. Hybrid database-authoritative writes (recommended)

Use incremental repository operations for sessions, because session writes are
frequent and naturally record-scoped. Add revision-checked snapshots for admin
resources, because their current schema-driven store and policy-review workflow
perform coordinated mutations across several resources.

This removes session table replacement immediately while adding a compare-and-
swap guard around the existing admin resource implementation. It is the
smallest change that prevents data loss without designing a wide persistence
interface for every admin resource workflow.

### 2. Incremental operations for every admin resource

Replace the admin snapshot repository with create, update, delete, query, audit,
policy-review, and demo-data operations.

This is the eventual deeper persistence Module, but doing it now would combine
correctness work with a large domain persistence redesign. It has higher review
risk and would touch most admin resource tests and adapters.

### 3. Serialize all writes with a global database lock

Keep snapshot replacement and acquire a database-wide advisory lock before
every save.

This prevents concurrent writers but retains full-table rewrites, limits
throughput, and does not solve stale peer reads by itself. It is not selected.

## Selected Architecture

### Session Module

Repository-backed session stores treat the repository as the source of truth.
The session repository Interface supports record-scoped create, resolve, renew,
and revoke behavior in addition to loading a snapshot for startup and explicit
reload.

The GORM Adapter:

- inserts one session row on issue;
- resolves one active row by token;
- renews with a conditional update that requires an active, unexpired session;
- revokes with a conditional update that requires an active session;
- never deletes all session rows during a normal session operation.

The file Adapter uses a process-local lock and retains its atomic temporary file
rename behavior. The legacy SQL Adapter uses equivalent record-scoped
statements so its tests describe the same Interface. Production
multi-process guarantees apply to the GORM Adapter; file mode remains a local
development option.

The in-memory implementation remains repository-free and keeps its current
fast path. `Reload` remains available for invalidation convergence, but normal
repository-backed resolve, renew, and revoke operations no longer depend on a
fresh process-local snapshot for correctness.

### Admin Resource Module

`ResourceSnapshot` carries a monotonically increasing revision. Repository
`Save` accepts the revision loaded by the caller and returns the committed
revision.

The GORM Adapter keeps a revision row in the existing state table. Within the
save transaction, a conditional update atomically claims the expected revision
and advances it to the next revision. If no row matches, Save reads the actual
revision and returns a typed conflict. This claim happens before any resource
table delete, so a stale save does not delete or rewrite resource tables.

The Store reloads repository state before a repository-backed mutation. If
another writer commits between reload and save, the Store returns the typed
conflict instead of overwriting the newer state. HTTP maps that conflict to
`409 Conflict` so callers can refresh and retry.

Admin resource invalidation events reload repository-backed peer stores before
clearing derived policy, principal, menu, branding, and schema caches. Reload
failure keeps the previous in-memory snapshot and derived caches; it does not
replace state with a partial snapshot.

The file and legacy SQL Adapters carry the same revision field for format and
Interface compatibility. Revision conflict enforcement is provided by the
GORM production Adapter. File mode remains process-local, and the legacy SQL
Adapter is not an approved production runtime.

### Data Flow

Session write:

```text
HTTP handler -> Session Store -> record-scoped Repository operation
             -> publish sessions invalidation -> peer Reload
```

Admin resource write:

```text
HTTP handler -> Admin Resource Store
             -> Load(revision N)
             -> apply validated mutation
             -> SaveIfRevision(N)
             -> revision N+1 or conflict
             -> publish resource invalidation
             -> peer Reload -> clear derived caches
```

## Error Handling

- Session conditional updates that find no active row retain the existing
  invalid-session behavior.
- Repository I/O errors remain operation failures and do not update local state.
- Admin revision conflicts use a dedicated sentinel and map to HTTP 409.
- Peer reload errors do not discard the last valid snapshot.
- No retry loop is added in this stage. Automatic retries could repeat
  policy-review or audit mutations and require idempotency rules that are
  outside this correctness change.

## Testing

Tests were written before production changes and observed failing against the
pre-implementation snapshot behavior.

Implemented regression coverage:

1. Two independent GORM session stores issue different sessions without losing
   either record.
2. A revoke from one session store is observed by a separately constructed
   store without table replacement.
3. GORM session operations do not execute global delete behavior.
4. Two independent GORM admin resource stores cannot commit stale snapshots;
   the second write returns the typed conflict and preserves the first write.
5. A peer admin resource invalidation reloads an independently constructed
   Store and updates authorization behavior.
6. File and legacy SQL adapters preserve revision values across reloads.
7. Existing Go, Node contract, production readiness, and admin build gates stay
   green.

## Rollout And Compatibility

- Existing HTTP paths and JSON payloads remain unchanged.
- Existing development files without a revision field load as revision zero.
- Existing GORM state tables create the revision row lazily with value zero.
- This slice did not enable production auth promotion or source writing.
- Documentation and machine-readable task status are recorded as implemented
  only with passing regression and full-verification evidence.

## Implementation Evidence

The implemented slice is tracked by task node
`production-persistence-correctness`. The source and regression evidence is:

- record-scoped session repositories and Store operations under
  `internal/platform/session`, with focused coverage in
  `store_test.go`, `gorm_repository_test.go`, `file_repository_test.go` and
  `sql_repository_test.go`;
- revision-aware admin resource snapshots and GORM compare-and-swap persistence
  under `internal/platform/adminresource`, with focused coverage in
  `store_test.go`, `gorm_store_test.go`, `file_store_test.go` and
  `sql_store_test.go`;
- HTTP 409 conflict mapping and independent-Store invalidation reload behavior
  in `internal/platform/httpapi/server.go` and `server_test.go`;
- cache invalidation contract checks through
  `scripts/validate-platform-cache-invalidation.mjs` and
  `scripts/platform-cache-invalidation.test.mjs`.

The complete Task 1-3 evidence and the Task 4 full verification commands and
results are recorded in `.superpowers/sdd/task-1-report.md` through
`.superpowers/sdd/task-4-report.md`. No external approval or commit is claimed
by this implementation evidence.

## Follow-On Stages

With this stage complete and verified:

1. Publish importable capability and route registration contracts with an
   external consumer compile test.
2. Move schema migration ownership from repository constructors into real
   capability lifecycle steps.
3. Deepen the HTTP, capability declaration, and admin resource console Modules.
4. Establish a committed baseline and repository-local CI workflow.
