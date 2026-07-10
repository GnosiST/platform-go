# Platform Production Persistence Correctness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GORM-backed sessions and admin resources database-authoritative across independent API processes without changing existing HTTP payloads.

**Architecture:** Session repositories replace whole-snapshot writes with record-scoped create, resolve, renew and revoke operations. Admin resource snapshots gain a monotonic revision; GORM saves claim the expected revision with an atomic conditional update before replacing tables, while Store mutations reload first and return a typed conflict instead of overwriting peer data. Invalidation subscribers reload independent admin Stores before clearing derived caches.

**Tech Stack:** Go 1.26, Gin, GORM, database/sql, SQLite test databases, existing memory invalidation bus, Node contract validators.

## Global Constraints

- Keep existing HTTP paths and successful JSON payloads unchanged.
- Preserve memory and file-backed local development modes.
- Production multi-process correctness is required for GORM-backed sessions and admin resources.
- Do not add Redis-dependent integration tests.
- Do not redesign capability lifecycle, public capability packages, frontend code or source-writing generation.
- Session invalid, expired or revoked conditions remain non-errors; repository I/O failures must not mutate local state.
- Admin stale writes return a typed revision conflict and map to HTTP 409 without rewriting any resource table.
- Peer admin invalidation must reload Store state before clearing policy, principal, menu, branding or schema caches.
- File and legacy SQL adapters preserve revision/record semantics for compatibility but are not promoted as multi-process production stores.
- Use TDD: every behavior change starts with a focused failing test that is observed failing for the expected reason.
- Work in the current dirty feature branch without reverting unrelated user changes or creating commits automatically.

---

### Task 1: Record-Scoped Session Persistence

**Files:**
- Modify: `internal/platform/session/store.go`
- Modify: `internal/platform/session/gorm_repository.go`
- Modify: `internal/platform/session/sql_repository.go`
- Modify: `internal/platform/session/file_repository.go`
- Modify: `internal/platform/session/store_test.go`
- Modify: `internal/platform/session/gorm_repository_test.go`
- Modify: `internal/platform/session/sql_repository_test.go`
- Create: `internal/platform/session/file_repository_test.go`

**Interfaces:**
- Produces:

```go
type Repository interface {
	Load(context.Context) (Snapshot, error)
	Create(context.Context, Session) error
	Resolve(context.Context, string, time.Time) (Session, bool, error)
	Renew(context.Context, string, time.Time, time.Time) (Session, bool, error)
	Revoke(context.Context, string, time.Time) (Session, bool, error)
}

func (s *Store) ResolveContext(context.Context, string) (Session, bool, error)
func (s *Store) RenewContext(context.Context, string) (Session, bool, error)
func (s *Store) RevokeContext(context.Context, string) (bool, error)
```

- `Renew` repository arguments are token, current time and new expiry time.
- Repository activity predicate is `token = ? AND revoked_at IS NULL AND expires_at > now`.
- Existing `Resolve`, `Renew` and `Revoke` remain compatibility wrappers that call the context methods and fold repository errors into the existing false result.

- [ ] **Step 1: Add failing independent-store GORM tests**

Add tests that construct two Stores before either writes, using separate GORM connections to the same SQLite file:

```go
func TestIndependentGORMStoresIssueWithoutLosingSessions(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	writer := openSharedGORMStore(t, "sessions.db", now)
	reader := openSharedGORMStore(t, "sessions.db", now)

	first, err := writer.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	second, err := reader.Issue("ops")
	if err != nil {
		t.Fatalf("Issue(ops) error = %v", err)
	}

	verifier := openSharedGORMStore(t, "sessions.db", now)
	for _, token := range []string{first.Token, second.Token} {
		if _, ok := verifier.Resolve(token); !ok {
			t.Fatalf("Resolve(%q) = false, want both independent sessions", token)
		}
	}
}
```

Also add peer resolve, renew and revoke tests that do not call `Reload`, plus a SQLite delete-blocking trigger test proving normal operations never issue `DELETE FROM platform_sessions`.

- [ ] **Step 2: Run the GORM session tests and verify RED**

Run: `rtk go test ./internal/platform/session -run 'TestIndependentGORMStores|TestGORMStore|TestGORMRepositoryOperationsNeverExecuteGlobalDelete' -count=1`

Expected: FAIL because the second snapshot save removes the first session, peer operations read stale local maps, or the delete-blocking trigger aborts a normal operation.

- [ ] **Step 3: Add repository failure and file/SQL lifecycle tests**

Add Store tests using a repository fake whose `Create`, `Renew` or `Revoke` returns a sentinel error. Assert the Store does not add or replace local records after the error. Add file and SQL tests proving unrelated sessions survive record-scoped operations and inactive sessions reject conditional renew/revoke.

- [ ] **Step 4: Run the expanded package tests and verify RED**

Run: `rtk go test ./internal/platform/session -count=1`

Expected: FAIL to compile until every adapter implements the new interface, then fail behaviorally while Store still uses whole-snapshot `Save`.

- [ ] **Step 5: Implement the minimal record-scoped repository interface**

Replace `Save` in the public internal interface with the five methods above. Implement GORM `Create` with one insert, `Resolve` with one active-row query, and `Renew`/`Revoke` with conditional updates and a read in the same transaction. Implement equivalent SQL statements. Give `FileRepository` an instance mutex and use locked load/modify/atomic-rename helpers.

The Store must hold its mutex across repository operations, update its local map only after repository success, and hold the same mutex for the complete `Reload` load-and-swap operation so an old reload cannot overwrite a successful concurrent mutation.

- [ ] **Step 6: Verify GREEN and run race coverage**

Run: `rtk go test ./internal/platform/session -count=1`

Run: `rtk go test -race ./internal/platform/session -count=1`

Expected: all Session tests pass with no race reports.

---

### Task 2: Revision-Checked Admin Resource Snapshots

**Files:**
- Modify: `internal/platform/adminresource/repository.go`
- Modify: `internal/platform/adminresource/store.go`
- Modify: `internal/platform/adminresource/gorm_store.go`
- Modify: `internal/platform/adminresource/file_store.go`
- Modify: `internal/platform/adminresource/sql_store.go`
- Modify: `internal/platform/adminresource/demo_data.go`
- Modify: `internal/platform/adminresource/policy_review.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `internal/platform/adminresource/gorm_store_test.go`
- Modify: `internal/platform/adminresource/file_store_test.go`
- Modify: `internal/platform/adminresource/sql_store_test.go`

**Interfaces:**
- Produces:

```go
var ErrRevisionConflict = errors.New("admin resource revision conflict")

type RevisionConflictError struct {
	Expected uint64
	Actual   uint64
}

func (e *RevisionConflictError) Error() string
func (e *RevisionConflictError) Unwrap() error

type ResourceSnapshot struct {
	Revision  uint64
	NextID    int
	Resources map[string][]Record
}

type AdminResourceRepository interface {
	Load(context.Context) (ResourceSnapshot, error)
	Save(context.Context, ResourceSnapshot) (uint64, error)
}

func (s *Store) Reload() error
```

- `errors.Is(err, ErrRevisionConflict)` must be true for stale saves.
- GORM state key is exactly `revision`; its value is an unsigned base-10 integer string.

- [ ] **Step 1: Add failing GORM stale-snapshot tests**

Add a direct repository test:

```go
func TestGORMAdminResourceRepositoryRejectsStaleRevision(t *testing.T) {
	repository := openAdminResourceGORMRepository(t)
	first, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}
	stale := first
	first.Resources["tenants"] = []Record{{ID: "tenant-a", Code: "a", Name: "A", Status: "enabled"}}
	committed, err := repository.Save(context.Background(), first)
	if err != nil || committed != 1 {
		t.Fatalf("Save(first) = %d, %v; want revision 1", committed, err)
	}
	stale.Resources["tenants"] = []Record{{ID: "tenant-b", Code: "b", Name: "B", Status: "enabled"}}
	if _, err := repository.Save(context.Background(), stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("Save(stale) error = %v, want ErrRevisionConflict", err)
	}
}
```

Assert the database retains tenant A and never writes tenant B. Add Store tests for full rollback of resources, `nextID` and revision after a conflict.

- [ ] **Step 2: Run focused admin repository tests and verify RED**

Run: `rtk go test ./internal/platform/adminresource -run 'TestGORMAdminResourceRepositoryRejectsStaleRevision|TestRepositoryBackedStore.*Conflict' -count=1`

Expected: FAIL because snapshots have no revision, Save returns no committed revision, and stale writes overwrite the first commit.

- [ ] **Step 3: Add adapter round-trip and mutation-reload tests**

Add file tests proving old JSON without `revision` loads as zero and new saves round-trip a nonzero revision. Add legacy SQL revision round-trip coverage. Add a two-Store mutation test proving the second Store reloads and preserves an unrelated record committed by the first Store.

- [ ] **Step 4: Run the expanded adminresource tests and verify RED**

Run: `rtk go test ./internal/platform/adminresource -count=1`

Expected: FAIL until every adapter and every mutation path uses revision-aware snapshots.

- [ ] **Step 5: Implement revision CAS and Store reload boundaries**

In GORM Save, lazily create `revision=0` with `clause.OnConflict{DoNothing: true}`, then perform this conditional update before any table delete:

```go
expected := strconv.FormatUint(snapshot.Revision, 10)
nextRevision := snapshot.Revision + 1
result := tx.Model(&gormAdminResourceState{}).
	Where("key = ? AND value = ?", "revision", expected).
	Update("value", strconv.FormatUint(nextRevision, 10))
if result.Error != nil {
	return result.Error
}
if result.RowsAffected != 1 {
	actual, err := loadGORMRevision(tx)
	if err != nil {
		return err
	}
	return &RevisionConflictError{Expected: snapshot.Revision, Actual: actual}
}
```

Load revision before and after GORM's multi-query snapshot load; reject the load if the monotonic values differ so mixed-revision snapshots are never installed.

Store must retain immutable capability seed resources, reload repository state before every repository-backed mutation, and use full-snapshot rollback helpers for CRUD, demo-data and policy-review mutations. File and SQL saves return `snapshot.Revision + 1` and persist that revision without claiming cross-process CAS guarantees.

- [ ] **Step 6: Verify GREEN and run race coverage**

Run: `rtk go test ./internal/platform/adminresource -count=1`

Run: `rtk go test -race ./internal/platform/adminresource -count=1`

Expected: all Admin Resource tests pass with no race reports.

---

### Task 3: HTTP Conflict Mapping And Independent-Store Invalidation

**Files:**
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `resources/platform-cache-invalidation.json`
- Modify: `scripts/validate-platform-cache-invalidation.mjs`
- Modify: `scripts/platform-cache-invalidation.test.mjs`

**Interfaces:**
- Consumes: `Store.Reload`, `ErrRevisionConflict`, and Session context methods from Tasks 1 and 2.
- Produces: HTTP conflict code `ADMIN_RESOURCE_REVISION_CONFLICT` with status 409.

- [ ] **Step 1: Add failing HTTP conflict and independent-store tests**

Add `TestWriteAdminResourceErrorMapsRevisionConflict` and `TestDistributedInvalidationReloadsIndependentGORMAdminResourceStore`. The distributed test must use two independent Stores backed by separate repositories sharing one SQLite file, two independent memory caches, and one memory invalidation bus. Prime reader authorization, update `role-operator` through writer, then assert reader tenant query changes from 200 to 403.

- [ ] **Step 2: Add failing cache contract tests**

Extend the Node drift test with a fixture that removes `s.resources.Reload()` or puts cache invalidation before reload. Both mutations must make `validate-platform-cache-invalidation.mjs` fail.

- [ ] **Step 3: Run focused tests and verify RED**

Run: `rtk go test ./internal/platform/httpapi -run 'TestWriteAdminResourceErrorMapsRevisionConflict|TestDistributedInvalidationReloadsIndependentGORMAdminResourceStore' -count=1`

Run: `rtk node --test scripts/platform-cache-invalidation.test.mjs`

Expected: Go test fails with stale reader authorization or missing 409 mapping; Node test fails because the validator does not require admin Store reload ordering.

- [ ] **Step 4: Implement reload-before-cache and context-aware session writes**

Update invalidation subscription to:

```go
if event.Resource == sessionInvalidationResource {
	_ = s.sessions.Reload()
	return
}
if err := s.resources.Reload(); err != nil {
	return
}
s.invalidateCachesForResourceLocal(ctx, event.Resource)
```

Map `ErrRevisionConflict` to 409 and code `ADMIN_RESOURCE_REVISION_CONFLICT`. Switch refresh/logout write handlers to Session context methods so repository I/O errors return existing internal auth errors while inactive sessions retain current invalid-session behavior.

Extend the cache invalidation contract and validator to require independent admin Store reload before derived cache deletion.

- [ ] **Step 5: Verify GREEN**

Run: `rtk go test ./internal/platform/httpapi -count=1`

Run: `rtk node scripts/validate-platform-cache-invalidation.mjs`

Run: `rtk node --test scripts/platform-cache-invalidation.test.mjs`

Expected: all focused Go and Node tests pass.

---

### Task 4: Governance Status And Full Verification

**Files:**
- Modify: `docs/superpowers/specs/2026-07-10-platform-production-persistence-correctness-design.md`
- Modify: `docs/platform-cache.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-node-closeout-audit.json`

**Interfaces:**
- Produces implemented task node `production-persistence-correctness`.
- Task node dependencies: `gorm-storage-runtime`, `cache-redis-invalidation`.
- Task node locks: `storage-runtime`, `cache-runtime`, `admin-resource-api`, `docs`.

- [ ] **Step 1: Add the implemented governance node and evidence**

Only after Tasks 1-3 pass, mark the spec implemented, add the task node as `implemented`, add it to alignment and closeout requirements, change goal totals from 34/34 to 35/35, and cite the exact focused tests, source files, cache validator and full verification commands.

- [ ] **Step 2: Update operational documentation**

Document that GORM session operations are record-scoped, admin saves use revision CAS, peer admin events reload independent Stores before cache clearing, file/legacy SQL modes are compatibility modes, and Redis Pub/Sub remains best-effort convergence rather than a durable consistency log.

- [ ] **Step 3: Run full verification**

Run:

```text
rtk go test ./...
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node --test scripts/platform-cache-invalidation.test.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync
```

Expected: every command exits zero, goal completion reports complete with 35 implemented nodes, and CodeGraph reports an up-to-date index.

- [ ] **Step 4: Review for scope and cleanliness**

Confirm no frontend files, capability public APIs, lifecycle migration ownership, production auth promotion or source-writing behavior changed. Confirm no unrelated dirty-worktree changes were reverted or included in generated output.
