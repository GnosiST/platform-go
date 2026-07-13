# Sensitive Data Historical Migration Implementation Plan

> **Status:** Completed.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Every behavior change uses test-first RED/GREEN checkpoints and each task receives an independent review.

**Goal:** Ship an offline, tenant-batched and auditable migration command that encrypts historical manifest-declared admin-resource values, verifies them, resumes safely and rolls back through encrypted escrow.

**Architecture:** A pure `sensitivemigration` package owns manifest planning, classification, orchestration and sanitized reports. The admin-resource package owns a row-level GORM maintenance store because it already owns the physical normalized table map. `platform-admin` exposes the runner as a maintenance-only subcommand. Ordinary Store and HTTP runtime behavior remain unchanged and fail closed on plaintext.

**Tech Stack:** Go 1.26, GORM, AES-256-GCM data-protection runtime, MySQL/PostgreSQL production targets, SQLite local rehearsal, Node governance validators.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-12-sensitive-data-historical-migration-design.md`.
- Use test-first RED/GREEN cycles for every behavior change.
- Do not add an HTTP endpoint, background migration, plaintext dual-read or automatic startup mutation.
- Do not infer sensitive fields from names; use enabled capability manifests only.
- Do not log or report plaintext, ciphertext, keys, nonces, AAD, blind indexes, DSNs, record IDs or PII.
- Only `Source="values"` encrypted fields are migration-compatible.
- Inventory and dry-run must not create journal tables or change data.
- MySQL/PostgreSQL are the production targets; SQLite is local rehearsal only.
- Reject legacy generic SQL and file mutation modes.
- `prepare` is the only mode allowed to create migration journal tables; apply never AutoMigrates.
- Encrypted fields duplicated into normalized dedicated columns or relation tables fail physical-layout validation.
- Mutating batches atomically couple row updates, encrypted escrow, append-only events, checkpoints and global revision CAS.
- Prefix shell commands with `rtk` and refresh CodeGraph after structural edits.

---

### Task 1: Lock The Migration Contract And Pure Planner

**Files:**
- Modify: `internal/platform/capability/admin_contract.go`
- Modify: `internal/platform/capability/admin_contract_test.go`
- Create: `internal/platform/sensitivemigration/plan.go`
- Create: `internal/platform/sensitivemigration/plan_test.go`

**Interfaces:**

```go
type Plan struct { Resources []ResourcePlan }
type ResourcePlan struct {
    Resource string
    Scope string
    TenantField string
    SchemaVersion uint32
    Fields []FieldPlan
}
type FieldPlan struct {
    Key string
    Policy dataprotection.FieldPolicy
}
func PlanFromManifests([]capability.Manifest) (Plan, error)
```

- [ ] Write failing tests proving encrypted record-source fields are rejected, arbitrary values-source fields are accepted, ordering is deterministic, duplicates fail, tenant metadata is retained and hashed fields are excluded.
- [ ] Run `rtk go test ./internal/platform/capability ./internal/platform/sensitivemigration -run 'Test.*(EncryptedSource|MigrationPlan)' -count=1` and confirm the new tests fail for the missing contract/planner.
- [ ] Implement the minimum contract check and pure planner without field-name heuristics.
- [ ] Re-run the focused tests and `rtk go test ./internal/platform/capability ./internal/platform/sensitivemigration -count=1`.
- [ ] Commit as `feat: define sensitive data migration plan`.

### Task 2: Implement Classification And Read-Only Runner Modes

**Files:**
- Create: `internal/platform/sensitivemigration/model.go`
- Create: `internal/platform/sensitivemigration/classify.go`
- Create: `internal/platform/sensitivemigration/runner.go`
- Create: `internal/platform/sensitivemigration/runner_test.go`

**Interfaces:**

```go
type Mode string
const (
    ModeInventory Mode = "inventory"
    ModeDryRun Mode = "dry-run"
    ModePrepare Mode = "prepare"
    ModeApply Mode = "apply"
    ModeVerify Mode = "verify"
    ModeRehearseRestore Mode = "rehearse-restore"
    ModeRollback Mode = "rollback"
)
type Cursor struct { TenantID string; RecordID string }
type Row struct { Resource string; RecordID string; ValuesJSON string }
type ReadStore interface {
    TenantScopes(context.Context, ResourcePlan) ([]string, error)
    Rows(context.Context, ResourcePlan, string, string, int) ([]Row, error)
}
type Report struct {
    RunID string `json:"runId,omitempty"`
    Mode Mode `json:"mode"`
    Status string `json:"status"`
    Counts Counts `json:"counts"`
    Checkpoints int `json:"checkpoints"`
    EventChainHead string `json:"eventChainHead,omitempty"`
}
func NewRunner(Plan, dataprotection.Runtime, ReadStore) *Runner
func (r *Runner) Run(context.Context, Options) (Report, error)
```

- [ ] Write failing tests for missing/plaintext/target/foreign/malformed classification, tenant traversal, bounded batches, deterministic counts, no reveal during classification, idempotent target envelopes and sanitized errors/reports.
- [ ] Run `rtk go test ./internal/platform/sensitivemigration -run 'Test.*(Classif|Inventory|DryRun|Saniti)' -count=1` and confirm RED.
- [ ] Implement classification through `IsEnvelope` plus `Runtime.Validate`, and inventory/dry-run traversal through `ReadStore`.
- [ ] Re-run focused and package tests; scan serialized reports for fixture secrets and envelope prefixes.
- [ ] Commit as `feat: add sensitive data migration inventory`.

### Task 3: Add The GORM Maintenance Store, Physical Layout Gate And Journal

**Files:**
- Create: `internal/platform/adminresource/sensitive_migration_gorm.go`
- Create: `internal/platform/adminresource/sensitive_migration_gorm_test.go`
- Modify: `internal/platform/adminresource/gorm_store.go`

**Interfaces:**

```go
func NewGORMProtectedValueMigrationStore(*gorm.DB, string) (*GORMProtectedValueMigrationStore, error)
func (s *GORMProtectedValueMigrationStore) TenantScopes(context.Context, sensitivemigration.ResourcePlan) ([]string, error)
func (s *GORMProtectedValueMigrationStore) Rows(context.Context, sensitivemigration.ResourcePlan, string, string, int) ([]sensitivemigration.Row, error)
func (s *GORMProtectedValueMigrationStore) Prepare(context.Context, sensitivemigration.RunRequest) (sensitivemigration.RunState, error)
func (s *GORMProtectedValueMigrationStore) ApplyBatch(context.Context, sensitivemigration.BatchMutation) (sensitivemigration.BatchCommit, error)
```

- [ ] Write failing SQLite tests proving read-only methods do not AutoMigrate, generic and normalized tables use whitelisted mappings, duplicated dedicated-column/relation keys fail closed, prepare creates only migration tables and value-free targets, row snapshots and global revision use CAS, and journal/checkpoint/event updates roll back with a failed row update.
- [ ] Run `rtk go test ./internal/platform/adminresource -run 'TestGORMProtectedValueMigration' -count=1` and confirm RED.
- [ ] Extract the normalized resource table descriptor map from `gorm_store.go` so ordinary persistence and migration share one whitelist.
- [ ] Implement read-only traversal and explicit prepare-only journal/target creation. Mutation must update `values_json` with the original JSON predicate and update the global revision in the same transaction.
- [ ] Re-run focused tests plus `rtk go test ./internal/platform/adminresource -count=1`.
- [ ] Commit as `feat: add transactional sensitive migration store`.

### Task 4: Implement Apply, Verify, Resume And Event Chaining

**Files:**
- Modify: `internal/platform/sensitivemigration/model.go`
- Modify: `internal/platform/sensitivemigration/runner.go`
- Modify: `internal/platform/sensitivemigration/runner_test.go`
- Modify: `internal/platform/adminresource/sensitive_migration_gorm.go`
- Modify: `internal/platform/adminresource/sensitive_migration_gorm_test.go`

**Interfaces:**

```go
type MutatingStore interface {
    ReadStore
    Prepare(context.Context, RunRequest) (RunState, error)
    StartOrResume(context.Context, RunRequest) (RunState, error)
    ApplyBatch(context.Context, BatchMutation) (BatchCommit, error)
    FinishRun(context.Context, string, string) error
}
```

- [ ] Write failing runner/store tests for required prepare/apply approvals, sealed plan identity, apply refusal without prepare, batch resume, target-envelope no-op, malformed/foreign fail-closed behavior, event hash chaining, committed cursor durability, revision conflict and verification reconciliation.
- [ ] Run `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource -run 'Test.*(Apply|Resume|Verify|EventChain|RevisionConflict)' -count=1` and confirm RED.
- [ ] Implement apply and verify orchestration. Use `Runtime.Protect` only for plaintext and never call `Reveal`.
- [ ] Re-run focused tests, both package suites and `rtk go test ./internal/platform/dataprotection -count=1`.
- [ ] Commit as `feat: migrate historical sensitive values`.

### Task 5: Add Encrypted Escrow, Restore Rehearsal And Rollback

**Files:**
- Create: `internal/platform/sensitivemigration/escrow.go`
- Modify: `internal/platform/sensitivemigration/runner.go`
- Modify: `internal/platform/sensitivemigration/runner_test.go`
- Modify: `internal/platform/adminresource/sensitive_migration_gorm.go`
- Modify: `internal/platform/adminresource/sensitive_migration_gorm_test.go`

**Interfaces:**

```go
func EscrowContext(runID, tenantID, resource, recordID, fieldKey string) (dataprotection.FieldPolicy, dataprotection.FieldContext)
type EscrowEntry struct {
    RunID string
    Resource string
    RecordID string
    FieldKey string
    TenantID string
    ProtectedOriginal string
    MigratedValueHash string
}
```

- [ ] Write failing tests proving escrow uses reserved AAD, cannot validate as a target field, rehearsal decrypts without output, rollback refuses post-migration edits, successful rollback restores exactly the original field and all rollback mutations are journal-atomic.
- [ ] Run `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource -run 'Test.*(Escrow|Rehearse|Rollback)' -count=1` and confirm RED.
- [ ] Implement encrypted escrow, reveal-and-discard rehearsal and hash-guarded rollback.
- [ ] Re-run focused/package tests and scan output/error fixtures for secret values and envelope prefixes.
- [ ] Commit as `feat: add sensitive migration rollback`.

### Task 6: Integrate The Offline CLI And Driver Gates

**Files:**
- Modify: `cmd/platform-admin/main.go`
- Modify: `cmd/platform-admin/main_test.go`
- Create: `internal/platform/bootstrap/sensitive_migration.go`
- Create: `internal/platform/bootstrap/sensitive_migration_test.go`

**Interfaces:**

```text
platform-admin sensitive-data-migrate --mode <mode> [flags]
```

- [ ] Write failing CLI/bootstrap tests for exact modes including explicit prepare, required approval flags, batch bounds, JSON-only success output, sanitized errors, unsupported drivers, no file/legacy-SQL mutation and no journal creation during inventory/dry-run/verify.
- [ ] Run `rtk go test ./cmd/platform-admin ./internal/platform/bootstrap -run 'Test.*SensitiveDataMigrat' -count=1` and confirm RED.
- [ ] Add command dispatch without changing `bind-admin-oidc`. Open GORM directly through `storage.OpenGORM`; do not call the ordinary repository constructor or its `AutoMigrate` path.
- [ ] Re-run focused tests plus full command/bootstrap package tests.
- [ ] Commit as `feat: add offline sensitive migration command`.

### Task 7: Close Governance, Operations Documentation And Evidence

**Files:**
- Create: `docs/platform-sensitive-data-migration.md`
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/platform-deployment.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: matching `scripts/*.test.mjs` governance tests
- Create: `scripts/validate-platform-sensitive-data-migration.mjs`
- Create: `scripts/platform-sensitive-data-migration.test.mjs`

**Interfaces:**

- Produces an implemented `sensitive-data-historical-migration` node, `41/45/4` official graph counts and a neat-freak closeout.
- Keeps MySQL/PostgreSQL production certification as an explicit promotion evidence requirement when external integration environments are unavailable.

- [ ] Write failing Node tests for required command modes, driver policy, approval fields, redaction rules, evidence paths, `41/45/4` graph projections and closeout consistency.
- [ ] Run the focused Node tests/validators and confirm RED.
- [ ] Implement the validator, runbook and governance updates. Document exact backup, prepare, apply, verify, rehearsal, rollback and incident-stop steps without sample secrets.
- [ ] Run focused Node tests/validators, regenerate affected operations artifacts, and refresh CodeGraph.
- [ ] Commit as `docs: close sensitive data migration governance`.

### Task 8: Independent Review And Full Verification

**Files:**
- Modify only files required by review findings.

- [ ] Generate a whole-node review package from the pre-node base commit and dispatch an independent code reviewer.
- [ ] Fix every Critical/Important finding with a focused RED/GREEN cycle and re-review.
- [ ] Run:

```bash
rtk go test ./...
rtk go vet ./...
rtk node --test scripts/*.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-sensitive-data-migration.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
rtk git status --short
```

- [ ] Confirm no HTTP migration route exists and no tracked report/evidence contains fixture plaintext or `pgo:enc:` values.
- [ ] Commit any review fixes, mark this plan completed and leave the worktree clean.
