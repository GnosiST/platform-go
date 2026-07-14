# Data Lifecycle And Retention Implementation Plan

> **Status:** Implemented. Runtime, maintenance CLI, disabled-by-default scheduler, generated contracts, deployment guidance and governance closeout are complete; the post-commit remaining-task topology review stays separate.

**Goal:** Close `data-lifecycle-retention` with fail-closed resource policy, recoverable record and file deletion, independent final purge, retention-policy promotion and a disabled-by-default resumable maintenance runner.

**Architecture:** Capability manifests own deletion intent. `adminresource.Record` owns lifecycle state while SQL/GORM repositories persist it in a sidecar table. The generic Store enforces visibility and policy-aware mutations. HTTP exposes delete and restore only; final purge is maintenance-only. A dedicated lifecycle package owns impact, promotion, leases, checkpoints and bounded cleanup.

**Tech Stack:** Go 1.26, Gin, GORM, existing Admin resource snapshot repositories, object-storage port, Casbin permission checks, generated Admin contracts/OpenAPI and Node governance validators.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-14-data-lifecycle-retention-design.md`.
- Use test-first RED/GREEN cycles for every behavior change.
- Every enabled Admin resource must declare a valid lifecycle mode; missing policy fails closed.
- Lifecycle fields are top-level record state and cannot be written through generic values/forms.
- Normal reads never expose an `includeDeleted` override.
- Restore uses a dedicated Admin permission and audit action. Purge is not an Admin HTTP action and is available only through the maintenance CLI/runner.
- File restore is possible only before external cleanup begins and while the object exists.
- API token and session invalidation remain authoritative revocation operations.
- The runner is disabled by default and cannot mutate without persistent lease/checkpoint state.
- Shorter retention requires an impact report and explicit promotion of the exact policy fingerprint.
- SQL snapshot persistence must commit record rows, lifecycle sidecars, audit rows and revision compare-and-swap atomically before the runner is enabled.
- Deleted users, roles, menus and policy records must be excluded from authentication, Casbin policy construction, menu resolution and data-scope evaluation, not only list/query output.
- Runner lease, cursor, checkpoint and reports carry `datasourceID`; the current node implements only the default primary datasource and does not introduce XA.
- Do not claim universal soft delete, legal compliance, external archive coverage or unsupported database certification.
- Prefix shell commands with `rtk`, use `apply_patch` for edits and refresh CodeGraph after structural changes.

---

### Task 1: Lock The Manifest Lifecycle Contract

**Files:**
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/capability/admin_contract.go`
- Modify: `internal/platform/capability/admin_contract_test.go`
- Modify: capability manifests under `internal/apps/` and platform capability declarations
- Modify: generated Admin resource contract tests

- [x] Write failing tests for missing declarations, every supported mode, illegal restore/purge combinations, required `append-only` audit policy, required `revoke` token policy, required `tombstone` file policy and explicit `hard-delete` use.
- [x] Run focused capability and contract-generator tests and confirm RED.
- [x] Add `AdminResourceDeletionPolicy`, mode constants, version/retention/reference validation, deterministic JSON projection and fail-closed validation.
- [x] Explicitly classify every enabled resource; do not use a defaulting helper that hides omissions.
- [x] Re-run focused Go and Node tests and confirm generated contract determinism.

### Task 2: Add Top-Level Lifecycle State And Active-Record Semantics

**Files:**
- Modify: `internal/platform/adminresource/store.go`
- Modify: `internal/platform/adminresource/query.go`
- Modify: `internal/platform/adminresource/audit.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: focused Store/query/security tests

- [x] Write failing tests proving external writes cannot set lifecycle metadata, active reads remain unchanged, deleted records are hidden from list/query/relation/detail/export, internal lifecycle lookup can find them, delete retry is idempotent and restore clears metadata.
- [x] Run focused Store/query tests and confirm RED.
- [x] Add the top-level lifecycle fields and one shared active-record filter used by ordinary reads, authentication principals, Casbin policy construction, menu resolution and data-scope evaluation.
- [x] Implement policy-aware delete/restore/purge mutations with rollback-on-audit-failure and reference guards for `restrict`.
- [x] Re-run package tests and verify audit records contain no business values or free-form dependency errors.

### Task 3: Persist Lifecycle State Through Snapshot Repositories

**Files:**
- Modify: `internal/platform/adminresource/repository.go`
- Modify: `internal/platform/adminresource/file_store.go`
- Modify: `internal/platform/adminresource/sql_store.go`
- Modify: `internal/platform/adminresource/gorm_store.go`
- Add or modify matching repository tests

- [x] Write failing file, SQL and GORM reload tests for active/deleted state, restore, purge, normalized resources, generic resources, revision conflict and transaction rollback.
- [x] Add the SQL/GORM sidecar migration keyed by `(resource, record_id)` with lifecycle-only columns.
- [x] Join sidecar rows into snapshot load and update record plus sidecar state in the same revision transaction.
- [x] Make the compatibility SQL repository use one transaction and revision compare-and-swap; reject partial snapshot, sidecar or audit commits.
- [x] Keep `ValuesJSON` and normalized business tables free of platform lifecycle metadata.
- [x] Prove existing databases with no sidecar rows load records as active and that ordinary startup does not silently rewrite business values.
- [x] Run repository package tests for all available local drivers; record that these tests do not certify production database versions.

### Task 4: Separate HTTP Delete And Restore From Maintenance Purge

**Files:**
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: generated Admin OpenAPI and contract generators/tests

- [x] Write failing HTTP tests for each mode, reason validation, idempotent delete, restore deadline, dedicated restore permission, absence of an online purge route, hidden normal reads and cache invalidation.
- [x] Preserve `DELETE /api/admin/resources/:resource/:id` while routing it through the manifest policy.
- [x] Add `POST /api/admin/resources/:resource/:id/restore`; keep final purge exclusively behind the maintenance command/runner.
- [x] Return stable conflict/forbidden/not-found codes without revealing whether an unauthorized deleted record exists.
- [x] Generate and test schema/OpenAPI contracts. Confirm no normal query accepts `includeDeleted`.

### Task 5: Give Files A Real Recovery Window

**Files:**
- Modify: `internal/platform/adminresource/audit.go`
- Modify: `internal/platform/adminresource/file_store.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: file lifecycle and HTTP tests

- [x] Write failing tests for tombstone-with-object-retained, hidden content, restore before cleanup, restore refusal after claim, object-not-found cleanup, dependency retry and metadata purge only after durable cleanup completion.
- [x] Replace immediate HTTP object deletion with a persistent tombstone and scheduled cleanup state.
- [x] Add retry-safe cleanup claim/completion transitions; never resurrect metadata after object cleanup starts.
- [x] Make restore verify the object still exists through the storage port before clearing the tombstone.
- [x] Re-run file, storage and HTTP tests including audit/persistence failure injection.

### Task 6: Preserve Authoritative Token And Session Revocation

**Files:**
- Modify only the token/session manifest declarations and routing needed by the lifecycle contract
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: session repository tests if retention cleanup is added there

- [x] Write failing tests that generic delete dispatches API tokens to revoke, revoked tokens fail authentication immediately, session invalidation uses the session repository, and lifecycle cleanup never substitutes for revocation.
- [x] Keep revoked token metadata retained according to its domain policy.
- [x] If expired/revoked session cleanup is included, accept only already-terminal records and keep it independent from active-session authorization.
- [x] Re-run token, Admin auth, App auth and session repository suites.

### Task 7: Implement Policy Impact, Promotion And Resumable Cleanup

**Files:**
- Create: `internal/platform/datalifecycle/model.go`
- Create: `internal/platform/datalifecycle/runner.go`
- Create: `internal/platform/datalifecycle/runner_test.go`
- Add persistent lifecycle-run repository files and tests under the owning platform package

**Interfaces:**

```go
type Mode string // impact, dry-run, apply
type Cursor struct { DatasourceID string; Resource string; EligibleAt string; RecordID string }
type Options struct { Mode Mode; BatchSize int; MaxRetries int; PolicyFingerprint string }
type Report struct { Mode Mode; Status string; Counts Counts; Cursor Cursor; PolicyFingerprint string }
```

- [x] Write failing tests for disabled default, persistent-repository requirement, deterministic planning, bounded batches, cursor resume, lease contention/expiry, retry exhaustion, idempotent replay, policy drift and sanitized reports.
- [x] Write failing promotion tests proving a shorter window cannot apply without a matching impact hash, actor, reason, approval reference and promoted fingerprint.
- [x] Implement value-free impact reports, durable promotions, leases, heartbeats and checkpoints.
- [x] Atomically commit database-only purge plus target-local audit plus cursor on one datasource. Route file cleanup through its persistent state machine and do not introduce XA.
- [x] Re-run datalifecycle, adminresource and storage tests, including cancellation and process-restart simulations.

### Task 8: Add Maintenance Command And Disabled-By-Default Bootstrap

**Files:**
- Modify: `cmd/platform-admin/main.go`
- Modify: `cmd/platform-admin/main_test.go`
- Create: `internal/platform/bootstrap/data_lifecycle.go`
- Create: `internal/platform/bootstrap/data_lifecycle_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`

- [x] Write failing tests for exact modes, batch/retry bounds, explicit apply approval, invalid policy fingerprints, disabled runtime and production rejection of memory-only state.
- [x] Add `platform-admin data-lifecycle` commands for impact, promotion, dry-run and one-shot apply.
- [x] Keep any scheduled runner behind `PLATFORM_RETENTION_RUNNER_ENABLED=false`; startup with it enabled requires persistent storage and valid lease configuration.
- [x] Emit one sanitized JSON report per command and keep DSNs, object keys, record IDs, values and dependency error strings out of output.
- [x] Re-run command/bootstrap/config tests.

### Task 9: Generate Contracts And Close Governance

**Files:**
- Create: `docs/platform-data-lifecycle-retention.md`
- Modify: relevant schema, deployment, roadmap and task-map documentation
- Modify: generated Admin contracts/OpenAPI
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: engineering capability, execution, alignment, goal, objective and node-closeout resources
- Add focused validator/tests for lifecycle policy and closeout evidence

- [x] Write failing Node tests for complete resource classification, generated lifecycle schema, route/permission separation, runner-disabled default, promotion evidence and honest support boundaries.
- [x] Regenerate Admin contracts and OpenAPI.
- [x] Update the node from pending to implemented only after code, tests, operations docs and closeout evidence pass.
- [x] Keep the next node `multi-datasource-contract-and-runtime` pending and do not claim database certification.
- [ ] After this node is verified, committed and its resource locks are released, run one unified remaining-task topology review using the authoritative SaaS data-plane, persisted Query Object and Platform Service Contract input. Mark earlier cross-session variants as superseded drafts and publish exactly one graph adjustment proposal before changing later nodes.
- [x] Run one phase-level `neat-freak` cleanup after all lifecycle code and documentation are synchronized.

### Task 10: Independent Review And Full Verification

**Files:**
- Modify only files required by review findings.

- [x] Dispatch independent reviews for lifecycle correctness and verification/governance coverage.
- [x] Fix every Critical/Important finding with focused RED/GREEN evidence and re-review.
- [x] Run:

```bash
rtk go test ./...
rtk go vet ./...
rtk node --test scripts/*.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
rtk git status --short
```

- [x] Confirm all manifests explicitly declare lifecycle policy, normal routes never expose deleted records, the runner remains disabled by default and the worktree is clean after the final commit.

## Remaining Program Coordination

This node stays independent. It shares implementation primitives with later tasks but does not absorb them:

- The future data-plane program remains separate; this node only reserves `datasourceID`, same-datasource lifecycle state and one-datasource transaction semantics.
- The authoritative future input covers SaaS tenant placement/routing, persisted Query Objects, Platform Service Contracts, database certification, optional integrations and their interaction with organization/RBAC/menu governance. Earlier related cross-session drafts are superseded and must not be accumulated as separate requirements or nodes.
- No future node split, dependency change or parallel batch is approved inside this lifecycle plan. Those decisions belong to the single post-closeout topology review after current code, tests, governance and commit are complete.
