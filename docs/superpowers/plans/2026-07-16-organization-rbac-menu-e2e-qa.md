# Organization RBAC And Menu End-To-End QA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close `organization-rbac-menu-e2e-qa` with all-principal legacy/target authorization equivalence, guarded cutover and verified rollback, complete organization authorization write-path coverage, a 10,000-node Tree Transfer, and browser evidence across organization, user, role, permission and menu workflows.

**Architecture:** Keep the existing GORM organization authorization repository and persisted Query/Domain Command runtime as the source of truth. Add a canonical, value-minimized principal authorization snapshot inside `organizationrbac`, persist comparison and promotion evidence next to the migration run, and make runtime serving/write gates consult that evidence. Extend the existing platform Tree Transfer with indexed linear-time derivation and domain service-object search/hydration; retain Ant Design and platform wrappers rather than creating a parallel component system.

**Tech Stack:** Go, Gin, GORM, Casbin, persisted Query/Domain Command objects, React, TypeScript, Refine, Ant Design, Node 22 built-in tests with TypeScript stripping, platform UI wrappers, i18n validators and the in-app Browser.

## Global Constraints

- Work only in `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/.worktrees/platform-completion`.
- Prefix every shell command with `rtk`.
- Use CodeGraph before changing `rbac.Principal`, HTTP serving, GORM persistence, service-object definitions, generated clients or shared UI primitives.
- Preserve globally unique organization, role-group, role, menu and permission codes; do not introduce tenant-local duplicate codes.
- Menus are visibility hints. Casbin-backed API permission checks remain the authoritative backend boundary.
- `role_menu` persists page leaves only; directory ancestors are derived and directory selection never grants future descendants.
- Platform principals are created only from the explicit reviewed migration allowlist. Empty organization or tenant fields never infer platform scope.
- Do not add bulk or import product APIs. Prove that the currently shipped entry points are guarded and that nonexistent bulk/import routes remain unavailable.
- Do not add datasource routing, read/write splitting, sharding, federation, XA, MQ, search projection or workload identity work. Those capabilities remain deferred and outside this node.
- One authorization decision and one assignment save stay inside one datasource and one native transaction. The client must not chunk role-menu or role-permission apply operations.
- All UI changes must use existing platform page, table, modal, drawer, button, alert, dropdown, pagination and settings wrappers plus the existing i18n dictionaries.
- Apply `ui-ux-pro-max` to the Admin tasks: keyboard-first operation, visible focus, semantic roles/states, 44px targets, no hover-only actions, mobile/tablet adaptation, reduced motion and bounded main-thread work. Do not perform a marketing-style redesign.
- Use the in-app Browser first for UI verification. Use the existing scoped Playwright fallback only when the Browser is blocked, and record the blocker and fallback version in the evidence manifest.
- Coordinate with `unified-error-code-governance`: do not concurrently edit or regenerate service-object response types, Admin OpenAPI, generated clients, HTTP error construction or audit correlation fields.

## File And Ownership Map

- `internal/platform/organizationrbac/principal_equivalence.go`: canonical legacy/candidate/persisted target principal snapshots and all-principal comparison.
- `internal/platform/organizationrbac/migration.go`: migration orchestration, immutable execution evidence and role-menu candidate application.
- `internal/platform/organizationrbac/role_menu_repository.go`: transaction-bound page-leaf replacement used by normal CAS writes and migration backfill without nested transactions or per-role global revision bumps.
- `internal/platform/organizationrbac/migration_cutover.go`: promotion-state validation, dual-read observation aggregation and rollback verification.
- `internal/platform/httpapi/menu_serving.go`: request-time legacy/target menu comparison and comparison sink contract.
- `internal/platform/config/config.go`: syntactic mode compatibility; database-backed promotion evidence stays in bootstrap/repository validation.
- Existing organization snapshot writers: generic Admin create/update/delete/restore protection for domain-owned authorization state.
- `internal/platform/datalifecycle/adminresource_gorm_applier.go`: scheduled lifecycle fail-closed boundary for organization authorization resources.
- `internal/platform/organizationrbac/assignment_tree_service_objects.go`: server-backed menu and permission search/hydration queries.
- `admin/src/platform/ui/treeTransferModel.ts`: framework-independent indexed tree model and selection derivation.
- `admin/src/platform/ui/PlatformTreeTransfer.tsx`: accessible rendering, async search/hydration and responsive interaction.
- `admin/src/platform/resources/RoleGovernanceConsole.tsx`: role permission/menu integration without client-side chunks.
- `resources/evidence/organization-rbac-menu-e2e-qa-20260716.json`: tracked, redacted browser/performance/cutover/rollback evidence manifest.
- Organization contract and governance resources: executable completion gates and closeout truth.

---

### Task 1: Implement All-Principal Authorization Equivalence

**Files:**
- Create: `internal/platform/organizationrbac/principal_equivalence.go`
- Create: `internal/platform/organizationrbac/principal_equivalence_test.go`
- Modify: `internal/platform/organizationrbac/migration.go`
- Modify: `internal/platform/organizationrbac/navigation_repository.go`
- Modify: `internal/platform/organizationrbac/role_menu_repository.go`
- Modify: `internal/platform/organizationrbac/migration_test.go`

**Interfaces:**
- Consumes: `MigrationManifest`, normalized GORM organization/role/user/menu relations, trusted user/org/area scope columns and hierarchy used by current Admin data-scope enforcement, legacy `ValuesJSON`, legacy `menus.permission`, native `role_menu`, role allow/deny policies and data-scope fields.
- Produces:

```go
type PrincipalComparisonMode string

const (
	PrincipalComparisonCandidate PrincipalComparisonMode = "candidate"
	PrincipalComparisonPersisted PrincipalComparisonMode = "persisted"
)

type PrincipalAuthorizationSnapshot struct {
	UserCode           string
	ScopeType          ScopeType
	TenantCode         string
	OrgUnitCode        string
	RoleCodes          []string
	AllowPermissions   []string
	DeniedPermissions  []string
	DataScopeAll       bool
	DataScopeOrgCodes  []string
	DataScopeAreaCodes []string
	DataScopeSelf      bool
	MenuCodes          []string
}

type PrincipalAuthorizationDifference struct {
	UserCode        string
	LegacyHash      string
	TargetHash      string
	ChangedFields   []string
	BlockingReasons []string
	AddedMenus      int
	RemovedMenus    int
}

type PrincipalEquivalenceReport struct {
	ActivePrincipals int
	Equivalent       int
	Differences      []PrincipalAuthorizationDifference
	ComparisonHash   string
}

func (r *GORMRepository) CompareAllActivePrincipals(
	ctx context.Context,
	manifest MigrationManifest,
	mode PrincipalComparisonMode,
) (PrincipalEquivalenceReport, error)

type principalComparisonState struct {
	Manifest                MigrationManifest
	Users                   []gormUser
	AssignmentsByUserID     map[string][]string
	RolesByCode             map[string]gormRole
	RoleValuesByCode        map[string]map[string]string
	AllowByRoleCode         map[string][]string
	EnabledPermissions      []string
	EnabledPages            []gormMenu
	PersistedLeavesByRole   map[string][]string
	RoleMenuRevisionByRole  map[string]uint64
	OrganizationsByCode     map[string]gormOrganization
	OrgParentByCode         map[string]string
	OrgAreaByCode           map[string]string
	AreaParentByCode        map[string]string
	UserAreaByID            map[string]string
}

type intendedPrincipalIdentity struct {
	ScopeType   ScopeType
	TenantCode  string
	OrgUnitCode string
	AreaCode    string
}

func loadPrincipalComparisonState(
	db *gorm.DB,
	manifest MigrationManifest,
	mode PrincipalComparisonMode,
) (principalComparisonState, error)

func intendedIdentityForPrincipal(
	state principalComparisonState,
	user gormUser,
) (intendedPrincipalIdentity, string)

func legacyPrincipalSnapshot(
	state principalComparisonState,
	user gormUser,
) (PrincipalAuthorizationSnapshot, error)

func targetPrincipalSnapshot(
	state principalComparisonState,
	user gormUser,
	mode PrincipalComparisonMode,
) (PrincipalAuthorizationSnapshot, error)

func aggregateLegacyPrincipalPageLeaves(
	state principalComparisonState,
	roleCodes []string,
) []string

func candidatePageLeavesForRoles(
	state principalComparisonState,
	roleCodes []string,
) []string

func persistedPageLeavesForRoles(
	state principalComparisonState,
	roleCodes []string,
) []string

func remediatedRoleCodes(
	user gormUser,
	assignedRoleCodes []string,
	manifest MigrationManifest,
) ([]string, error)

func replaceRoleMenusTx(
	tx *gorm.DB,
	request ReplaceRoleMenusRequest,
	bumpGlobal bool,
) (RoleMenuSet, error)
```

- `loadPrincipalComparisonState` performs one manifest-aware bulk load. It includes every enabled, non-logically-deleted user, sorted by code, even when the user has no effective enabled permission; disabled or logically deleted users are not active principals.
- Parse trusted user area data and organization/area parent relationships from the same persisted schema fields used by current Admin data-scope enforcement. Expand `current_and_children` and `current_and_children_areas`, union organization/area scopes across roles, preserve `self`, and let any `all` role set `DataScopeAll`.
- `intendedIdentityForPrincipal` applies the explicit platform allowlist or the manifest's tenant-user organization mapping once and supplies the same intended scope/tenant/org/area identity to both legacy and target snapshots. Raw pre-migration identity integrity remains an `inventoryMigration` validation responsibility and must not create a false equivalence hash difference.
- A non-allowlisted user without an explicit valid tenant organization produces a deterministic difference with blocking reason `target-organization-required`; it is never inferred as platform scope.
- Allow and deny snapshots contain canonical enabled exact permission codes after wildcard expansion. A zero-permission active user contributes one comparison with empty allow, deny and menu sets.
- Snapshot hashes use canonical sorted sets and stable field ordering. Do not hash display names, arbitrary `ValuesJSON`, PII or adapter errors.
- Legacy menu visibility uses one aggregate deny-first principal policy across the user's currently assigned enabled roles. Do not union `legacyRoleMenuCandidate` results. Fix its wildcard branch so permissionless pages remain invisible, matching current Admin menu semantics.
- Candidate mode applies manifest role-assignment remediations in memory, then unions page-leaf candidates derived independently from each resulting enabled role's legacy allow/deny policy. Persisted mode uses the assignments already stored in the transaction/database and reads native `role_menu` page leaves for those enabled roles; it must not apply the manifest remediations a second time.
- Equivalence hashes page leaves only on all three paths. Directory ancestors remain runtime rendering behavior in `ResolveRoleMenuNodes` and never enter the equivalence hash.
- `replaceRoleMenusTx` contains the existing role-menu validation, CAS, relation and per-role revision work. `ReplaceRoleMenus` keeps its current public transaction and calls it with `bumpGlobal=true`; migration calls it on the existing apply transaction with `bumpGlobal=false` and performs one existing global revision bump after persisted verification.

- [ ] **Step 1: Write the failing cross-role and platform-principal tests**

Add table-driven tests over a shared seed containing enabled `users` and `reports` pages, one disabled page, one logically deleted page, enabled exact user/report permissions, enabled wildcard `*`, one disabled permission, enabled and disabled roles, enabled and disabled users, and one logically deleted user:

```go
tests := []struct {
	name                  string
	platformAllowlist     []string
	wantActivePrincipals  int
	wantDifferences       int
	wantBlockingReason    string
}{
	{
		name: "aggregate legacy cross-role deny differs from post-role candidate union",
		wantActivePrincipals: 1,
		wantDifferences: 1,
	},
	{
		name: "explicit wildcard platform principal equals every enabled permission-backed page leaf",
		platformAllowlist: []string{"platform-admin"},
		wantActivePrincipals: 1,
	},
	{
		name: "missing tenant organization without allowlist is blocking and never platform",
		wantActivePrincipals: 1,
		wantDifferences: 1,
		wantBlockingReason: "target-organization-required",
	},
	{
		name: "enabled zero-permission user contributes an empty snapshot",
		wantActivePrincipals: 1,
	},
}
```

For the cross-role case, assign role A `allow=admin:user:read, deny=admin:report:read` and role B `allow=admin:report:read`; assert legacy leaves are `users`, candidate leaves are `reports,users`, and `AddedMenus == 1`. Add a remediation fixture that replaces an assigned role and assert candidate roles, permissions, data scope and page leaves use the replacement role.

Add `TestCompareAllActivePrincipalsNormalizesEffectiveDataScope` with two active roles whose scopes are `custom_orgs + self`, and another pair whose scopes are `current_org + custom_areas`. Assert canonical `DataScopeAll`, expanded/sorted org codes, expanded/sorted area codes and `DataScopeSelf`; include current-and-children organization/area descendants from persisted hierarchy rows.

Assert disabled and logically deleted users are excluded from `ActivePrincipals`; disabled roles and disabled permissions contribute nothing; logically deleted pages are absent; every enabled non-logically-deleted user contributes exactly one result even when all effective sets are empty.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
rtk go test ./internal/platform/organizationrbac -run 'TestCompareAllActivePrincipals|TestOrganizationRBACMigration' -count=1
```

Expected: FAIL because `CompareAllActivePrincipals`, the manifest-aware comparison state and effective data-scope snapshot fields do not exist.

- [ ] **Step 3: Implement canonical legacy, candidate and persisted snapshots**

Implement the manifest-aware bulk loader and canonical derivation functions declared in the interface block. Use these exact derivation rules:

```go
func canonicalPrincipalSnapshot(snapshot PrincipalAuthorizationSnapshot) PrincipalAuthorizationSnapshot
func principalAuthorizationHash(snapshot PrincipalAuthorizationSnapshot) string

func (r *GORMRepository) CompareAllActivePrincipals(
	ctx context.Context,
	manifest MigrationManifest,
	mode PrincipalComparisonMode,
) (PrincipalEquivalenceReport, error) {
	if !r.ready(ctx) {
		return PrincipalEquivalenceReport{}, ErrRepositoryFailed
	}
	state, err := loadPrincipalComparisonState(r.db.WithContext(ctx), manifest, mode)
	if err != nil {
		return PrincipalEquivalenceReport{}, err
	}
	return compareLoadedPrincipals(state, mode)
}
```

`compareLoadedPrincipals` derives the intended identity once per user and uses it in both snapshots. If identity derivation returns `target-organization-required`, append one blocking difference rather than constructing platform scope. For valid principals, legacy uses current assigned roles and aggregate deny-first menu evaluation; candidate target uses `remediatedRoleCodes`, while persisted target uses the assignments already loaded from the database. Compare canonical page-leaf sets and all authorization fields, append the changed field names, and compute the report hash from sorted per-principal hashes and blocking reasons.

- [ ] **Step 4: Extract the transaction-bound role-menu writer**

Move the existing `ReplaceRoleMenus` transaction body into `replaceRoleMenusTx`. Preserve public CAS/no-op/revision behavior:

```go
func (r *GORMRepository) ReplaceRoleMenus(ctx context.Context, request ReplaceRoleMenusRequest) (RoleMenuSet, error) {
	if !r.ready(ctx) {
		return RoleMenuSet{}, ErrInvalid
	}
	var committed RoleMenuSet
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		committed, err = replaceRoleMenusTx(tx, request, true)
		return err
	})
	return committed, repositoryError(err)
}
```

Inside `replaceRoleMenusTx`, call `bumpGlobalRevision(tx)` only when `bumpGlobal` is true. Migration passes `false`, so all role candidate backfills share the outer transaction and the existing migration-level global revision bump.

- [ ] **Step 5: Make migration verify and apply consume the comparison**

Update `RunMigration` so:

```go
case MigrationVerify:
	comparison, compareErr := r.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonCandidate)
	if compareErr != nil || len(comparison.Differences) != 0 {
		report.Status = "blocked"
		if compareErr != nil {
			return report, compareErr
		}
		return report, ErrRolePoolViolation
	}
	report.Status = "verified"
	return report, nil
```

Inside the existing `applyMigration` transaction, bind comparison and writes to `tx`:

```go
txRepository := &GORMRepository{db: tx}
candidate, err := txRepository.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonCandidate)
if err != nil {
	return err
}
if len(candidate.Differences) != 0 {
	return ErrRolePoolViolation
}

// Apply intended identity, organization bindings and role remediations first.
// Then persist page-leaf candidates for every enabled, non-deleted role.
state, err := loadPrincipalComparisonState(tx, manifest, PrincipalComparisonCandidate)
if err != nil {
	return err
}
for _, roleCode := range sortedComparisonRoleCodes(state) {
	_, err = replaceRoleMenusTx(tx, ReplaceRoleMenusRequest{
		RoleCode: roleCode,
		MenuCodes: candidatePageLeavesForRoles(state, []string{roleCode}),
		ExpectedRevision: state.RoleMenuRevisionByRole[roleCode],
		ActorID: evidence.ActorID,
		ChangedAt: evidence.AppliedAt,
	}, false)
	if err != nil {
		return err
	}
}

persisted, err := txRepository.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonPersisted)
if err != nil {
	return err
}
if len(persisted.Differences) != 0 {
	return ErrRolePoolViolation
}
// Continue with ValidateCutover, one global revision bump and applied run state.
```

Populate `RoleMenuRevisionByRole` in candidate mode so the transaction-bound writer uses the actual per-role CAS revision. Candidate comparison must occur before any mutation; persisted comparison must occur after identity/remediation/menu writes and before `ValidateCutover`, the single global revision bump and commit. Do not silently approve added/removed menus, permission/data-scope drift or blocking identity reasons. The manifest or source relations must be corrected first.

- [ ] **Step 6: Re-run focused tests and verify GREEN**

Run:

```bash
rtk go test ./internal/platform/organizationrbac -run 'TestCompareAllActivePrincipals|TestOrganizationRBACMigration|TestGORMRepositoryComparesDeterministicLegacyRoleMenuCandidate' -count=1
```

Expected: PASS, including cross-role deny, post-remediation target roles, explicit platform allowlist, missing-organization blocking, zero-permission active users, effective mixed data-scope union, page-leaf-only equality and transaction rollback on persisted mismatch.

- [ ] **Step 7: Commit Task 1**

```bash
rtk git add internal/platform/organizationrbac/principal_equivalence.go internal/platform/organizationrbac/principal_equivalence_test.go internal/platform/organizationrbac/migration.go internal/platform/organizationrbac/navigation_repository.go internal/platform/organizationrbac/role_menu_repository.go internal/platform/organizationrbac/migration_test.go
rtk git commit -m "feat: compare organization authorization principals"
```

---

### Task 2: Add Dual-Read Promotion, Cutover And Verified Rollback

**Files:**
- Create: `internal/platform/organizationrbac/migration_cutover.go`
- Create: `internal/platform/organizationrbac/migration_cutover_test.go`
- Modify: `internal/platform/organizationrbac/migration.go`
- Modify: `internal/platform/httpapi/menu_serving.go`
- Modify: `internal/platform/httpapi/menu_serving_test.go`
- Modify: `internal/platform/bootstrap/organization_rbac.go`
- Modify: `internal/platform/bootstrap/organization_rbac_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `cmd/platform-admin/main.go`
- Modify: `cmd/platform-admin/main_test.go`

**Interfaces:**
- Consumes: `PrincipalEquivalenceReport` from Task 1, current Admin resource global revision, migration evidence, configured menu serving mode and role-menu write flag.
- Produces:

```go
const (
	PromotionPrepared         = "prepared"
	PromotionDualRead         = "dual-read"
	PromotionTargetRead       = "target-read"
	PromotionTargetWrite      = "target-write"
	PromotionRollbackRequired = "rollback-required"
	PromotionRolledBack       = "rolled-back-verified"
)

type MenuPromotionState struct {
	RunID              string
	ManifestHash       string
	Phase              string
	FrozenRevision     uint64
	ActivePrincipals   int
	Equivalent         int
	ComparisonHash     string
	LegacySnapshotHash string
	CheckpointRef      string
	ObservedAt         time.Time
}

func (r *GORMRepository) ValidateMenuPromotion(
	ctx context.Context,
	mode string,
	roleMenuWriteEnabled bool,
) (MenuPromotionState, error)

func (r *GORMRepository) RecordMenuDualReadComparison(
	ctx context.Context,
	principalKey string,
	comparison httpapi.AdminMenuComparison,
) error
```

- `AdminMenuComparisonSink` changes to `Record(context.Context, rbac.Principal, AdminMenuComparison)` so the persistent adapter can account for distinct principals. The default log sink must remain value-free and must not log username, role codes or menu codes.
- Config validation checks only mode compatibility. Bootstrap/repository validation checks the database-backed promotion evidence.

- [ ] **Step 1: Write failing promotion-state tests**

Cover this exact transition table:

| Current evidence | Requested mode | Writes | Result |
| --- | --- | --- | --- |
| none | legacy | false | allow |
| prepared | dual-read | false | reject |
| all principals equal, frozen revision | dual-read | false | allow |
| incomplete observation | target | false | reject |
| completed observation, unchanged revision | target | false | allow |
| target read only | target | true | reject |
| target write approved | target | true | allow |
| target writes occurred | legacy | false | reject until restored checkpoint verifies |

Also assert that revision drift after comparison invalidates promotion and requires a new all-principal comparison.

- [ ] **Step 2: Run RED tests**

```bash
rtk go test ./internal/platform/organizationrbac ./internal/platform/httpapi ./internal/platform/bootstrap ./internal/platform/config -run 'TestMenuPromotion|TestAdminMenusDualRead|TestPrepareAndOpenOrganizationRBAC' -count=1
```

Expected: FAIL because promotion evidence and principal-aware comparison recording do not exist.

- [ ] **Step 3: Persist immutable promotion evidence and value-free observations**

Extend the additive migration schema with promotion state and observation tables. Store only a stable principal key digest, equality, added/removed counts, global revision and timestamp. Reject duplicate principal records with mismatched results for the same revision.

Implement the sink wiring in `bootstrap/organization_rbac.go`; retain `adminMenuComparisonLogSink` only as a secondary value-free operational log.

- [ ] **Step 4: Open config syntax gates but keep repository gates authoritative**

Change `Config.ValidateRuntime` to allow:

```go
if menuServingMode != AdminMenuServingModeLegacy && organizationRBACMode != OrganizationRBACModeTarget {
	errs = append(errs, errors.New("target or dual-read menu serving requires organization RBAC target mode"))
}
if c.AdminRoleMenuWriteEnabled && menuServingMode != AdminMenuServingModeTarget {
	errs = append(errs, errors.New("role menu writes require target menu serving"))
}
```

Do not let this syntactic check authorize promotion. `OpenOrganizationRBAC` must call `ValidateMenuPromotion` before constructing the runtime.

- [ ] **Step 5: Make rollback verification executable without adding a new migration mode**

During apply, store a canonical `LegacySnapshotHash` before the first target mutation. `MigrationRollback` must be run after the operator restores the reviewed checkpoint and must verify:

```go
currentHash == appliedRun.LegacySnapshotHash
evidence.CheckpointRef == appliedRun.CheckpointRef
evidence.BackupSHA256 == appliedRun.BackupSHA256
```

Only then record `rolled-back-verified`. A pre-target-write rollback may switch serving to legacy without restoring data; a post-target-write rollback must fail until the restored database hash matches.

- [ ] **Step 6: Add a bounded SQLite checkpoint rehearsal test**

The test must:

1. create a temporary SQLite database;
2. seed legacy authorization records;
3. copy the database as the reviewed checkpoint;
4. apply migration and one target role-menu write;
5. prove config-only legacy rollback is rejected;
6. close the DB, restore the checkpoint file, reopen it;
7. run `MigrationRollback` and assert `rolled-back-verified` plus the original snapshot hash.

- [ ] **Step 7: Run GREEN tests**

```bash
rtk go test ./internal/platform/organizationrbac ./internal/platform/httpapi ./internal/platform/bootstrap ./internal/platform/config ./cmd/platform-admin -run 'TestMenuPromotion|TestAdminMenusDualRead|TestOrganizationRBACMigrationRollback|TestPrepareAndOpenOrganizationRBAC' -count=1
```

Expected: PASS with no principal values or menu codes in the comparison log output.

- [ ] **Step 8: Commit Task 2**

```bash
rtk git add internal/platform/organizationrbac/migration_cutover.go internal/platform/organizationrbac/migration_cutover_test.go internal/platform/organizationrbac/migration.go internal/platform/httpapi/menu_serving.go internal/platform/httpapi/menu_serving_test.go internal/platform/bootstrap/organization_rbac.go internal/platform/bootstrap/organization_rbac_test.go internal/platform/config/config.go internal/platform/config/config_test.go cmd/platform-api/main.go cmd/platform-admin/main.go cmd/platform-admin/main_test.go
rtk git commit -m "feat: gate organization menu cutover"
```

---

### Task 3: Close Every Shipped Authorization Write Entry

**Files:**
- Modify: `internal/platform/organizationrbac/admin_user_snapshot.go`
- Modify: `internal/platform/organizationrbac/admin_user_snapshot_test.go`
- Modify: `internal/platform/organizationrbac/admin_org_unit_snapshot.go`
- Modify: `internal/platform/organizationrbac/admin_org_unit_snapshot_test.go`
- Modify: `internal/platform/organizationrbac/admin_role_snapshot.go`
- Modify: `internal/platform/organizationrbac/admin_role_snapshot_test.go`
- Modify: `internal/platform/organizationrbac/menu_repository.go`
- Modify: `internal/platform/organizationrbac/menu_repository_test.go`
- Modify: `internal/platform/adminresource/gorm_store.go`
- Modify: `internal/platform/adminresource/gorm_store_test.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/datalifecycle/adminresource_gorm_applier.go`
- Modify: `internal/platform/datalifecycle/adminresource_gorm_applier_test.go`
- Modify: `internal/platform/organizationrbac/lifecycle_service_objects_test.go`
- Modify: `internal/platform/organizationrbac/service_objects_test.go`

**Interfaces:**
- Consumes: existing organization snapshot writer interfaces and lifecycle prepare/impact/apply service objects.
- Produces: fail-closed generic persistence semantics for domain-owned authorization changes. No new public bulk or import API is produced.

- [ ] **Step 1: Add failing generic REST and snapshot tests**

Test the currently shipped entry points:

```text
POST   /api/admin/resources/:resource
PUT    /api/admin/resources/:resource/:id
DELETE /api/admin/resources/:resource/:id
POST   /api/admin/resources/:resource/:id/restore
POST   /api/admin/service-objects/command
```

For `org-units`, `role-groups`, `roles`, `users`, `menus` and `permissions`, assert:

- ordinary metadata create/update follows the existing snapshot validator;
- authorization projection changes through generic PUT return conflict/fail closed;
- generic delete/restore of a live authorization entity cannot bypass lifecycle preview/apply;
- specialized service-object apply succeeds with matching preview/revision/hash;
- stale, expired, cross-owner or mismatched-hash apply returns conflict and does not mutate rows.

- [ ] **Step 2: Add failing platform-principal exception tests**

Assert all of the following:

```go
// Generic Admin create cannot create a platform principal.
// Empty org/tenant never infers platform scope.
// A reviewed migration allowlist can create a platform principal.
// Platform principals accept only enabled platform-group roles.
// Tenant roles, tenantCode and orgUnitCode are rejected for platform principals.
```

- [ ] **Step 3: Run RED tests**

```bash
rtk go test ./internal/platform/organizationrbac ./internal/platform/adminresource ./internal/platform/httpapi ./internal/platform/datalifecycle -run 'TestAdmin.*Snapshot|TestAdminResource.*Organization|TestResourceLifecycle|TestPlatformPrincipal|TestGORMAdminResourceApplier' -count=1
```

Expected: at least the generic delete/restore and scheduled lifecycle bypass assertions fail.

- [ ] **Step 4: Reject domain lifecycle changes in generic snapshot persistence**

Update each organization snapshot writer to distinguish unchanged lifecycle metadata from a new delete/restore transition. Return `adminresource.ErrDomainOwnedMutation` when the generic snapshot attempts to change `DeletedAt`, restore a deleted entity, change an authorization status, rebind organizations/roles, move a role or mutate menu/permission ownership.

The specialized organization lifecycle repository continues to mutate its normalized tables directly inside one transaction after preview validation.

- [ ] **Step 5: Make scheduled lifecycle fail closed for organization authorization resources**

In `GORMAdminResourceApplier`, reject the six authorization resources before direct delete/restore/purge mutation. Record the runner result as blocked for manual reviewed domain lifecycle handling; do not invent an automatic bulk remediation plan.

Add a validator assertion that no `/bulk`, `/import` or equivalent Admin resource mutation route exists. A future feature must explicitly integrate the organization domain guard before such a route can be registered.

- [ ] **Step 6: Run GREEN tests**

```bash
rtk go test ./internal/platform/organizationrbac ./internal/platform/adminresource ./internal/platform/httpapi ./internal/platform/datalifecycle -run 'TestAdmin.*Snapshot|TestAdminResource.*Organization|TestResourceLifecycle|TestPlatformPrincipal|TestGORMAdminResourceApplier' -count=1
```

Expected: PASS; no generic or scheduled path mutates domain-owned authorization state without the specialized protocol.

- [ ] **Step 7: Commit Task 3**

```bash
rtk git add internal/platform/organizationrbac/admin_user_snapshot.go internal/platform/organizationrbac/admin_user_snapshot_test.go internal/platform/organizationrbac/admin_org_unit_snapshot.go internal/platform/organizationrbac/admin_org_unit_snapshot_test.go internal/platform/organizationrbac/admin_role_snapshot.go internal/platform/organizationrbac/admin_role_snapshot_test.go internal/platform/organizationrbac/menu_repository.go internal/platform/organizationrbac/menu_repository_test.go internal/platform/adminresource/gorm_store.go internal/platform/adminresource/gorm_store_test.go internal/platform/httpapi/server_test.go internal/platform/datalifecycle/adminresource_gorm_applier.go internal/platform/datalifecycle/adminresource_gorm_applier_test.go internal/platform/organizationrbac/lifecycle_service_objects_test.go internal/platform/organizationrbac/service_objects_test.go
rtk git commit -m "fix: guard organization authorization writes"
```

---

### Task 4: Add Assignment Tree Search And Hydration Contracts

**Serialization Gate:** This task owns files also required by `unified-error-code-governance`. Before editing, confirm that the canonical error response, request/trace correlation fields and generator output have been merged or frozen. Do not run this task concurrently with error-code codegen work.

**Files:**
- Create: `internal/platform/organizationrbac/assignment_tree_service_objects.go`
- Create: `internal/platform/organizationrbac/assignment_tree_service_objects_test.go`
- Modify: `internal/platform/organizationrbac/service_objects.go`
- Modify: `scripts/admin-service-object-definitions.mjs`
- Modify: `scripts/admin-resource-contract-generators.test.mjs`
- Modify: `scripts/generate-admin-codegen-preview.mjs`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `resources/generated/admin-codegen-preview.json`
- Modify: `resources/generated/admin-service-object-client.ts`
- Modify: `resources/generated/openapi.admin.json`
- Modify: `admin/src/platform/api/organizationRBAC.ts`

**Interfaces:**
- Produces four stable queries:

```text
platform.navigation.menu-assignment-tree.search@1.0.0
platform.navigation.menu-assignment-tree.hydrate@1.0.0
platform.authorization.permission-assignment-tree.search@1.0.0
platform.authorization.permission-assignment-tree.hydrate@1.0.0
```

- Search arguments: scalar `roleCode`, scalar `query`; standard page/pageSize pagination. Results contain matched leaves plus their ancestor path nodes.
- Hydration arguments: scalar `roleCode`. The server reads the role's current assignment and returns every selected leaf definition, including disabled/missing historical selections with a removable disabled reason.
- Neither query accepts fields, operators, SQL, datasource, physical schema, tenant routing or client-provided selected-code arrays.

- [ ] **Step 1: Write failing definition and executor tests**

Assert exact IDs, numeric SemVer, permission separation, typed fields and result ordering. Add 10,000 menu/permission definitions and a role with 2,000 selected leaves. Verify:

```go
// search returns matched leaves and all ancestor paths without unrelated branches;
// hydrate returns exactly the 2,000 assigned leaves plus ancestors;
// tenant/platform role boundaries are enforced server-side;
// disabled historical selections remain visible and removable;
// result order is stable across repeated reads.
```

- [ ] **Step 2: Run RED tests**

```bash
rtk go test ./internal/platform/organizationrbac -run 'TestAssignmentTree' -count=1
rtk node --test scripts/admin-resource-contract-generators.test.mjs
```

Expected: FAIL because the four query definitions and generated methods do not exist.

- [ ] **Step 3: Implement repository-backed search and hydration**

Use parameterized GORM predicates over whitelisted code/name fields. Search must cap page size through the existing Query runtime, include ancestor closure and never return raw `ValuesJSON`. Hydration derives selected codes from native role relations rather than accepting a client list.

- [ ] **Step 4: Regenerate contracts and wire the Admin API**

Add these exact TypeScript functions:

```ts
export async function searchMenuAssignmentTree(roleCode: string, query: string, page: number, pageSize: number)
export async function hydrateMenuAssignmentTree(roleCode: string)
export async function searchPermissionAssignmentTree(roleCode: string, query: string, page: number, pageSize: number)
export async function hydratePermissionAssignmentTree(roleCode: string)
```

Run:

```bash
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-openapi.mjs
```

- [ ] **Step 5: Verify generated contract GREEN**

```bash
rtk go test ./internal/platform/organizationrbac -run 'TestAssignmentTree' -count=1
rtk node --test scripts/admin-resource-contract-generators.test.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
```

Expected: PASS with fresh generated client/OpenAPI artifacts and no public physical routing inputs.

- [ ] **Step 6: Commit Task 4**

```bash
rtk git add internal/platform/organizationrbac/assignment_tree_service_objects.go internal/platform/organizationrbac/assignment_tree_service_objects_test.go internal/platform/organizationrbac/service_objects.go scripts/admin-service-object-definitions.mjs scripts/admin-resource-contract-generators.test.mjs scripts/generate-admin-codegen-preview.mjs scripts/generate-admin-openapi.mjs resources/generated/admin-codegen-preview.json resources/generated/admin-service-object-client.ts resources/generated/openapi.admin.json admin/src/platform/api/organizationRBAC.ts
rtk git commit -m "feat: add authorization tree queries"
```

---

### Task 5: Certify Tree Transfer At 10,000 Nodes

**Required Skill:** Use `ui-ux-pro-max` for accessibility, touch, responsive behavior and performance. Preserve the current Admin visual direction and platform wrappers.

**Files:**
- Create: `admin/src/platform/ui/treeTransferModel.ts`
- Create: `scripts/platform-tree-transfer-model.test.mjs`
- Modify: `admin/src/platform/ui/PlatformTreeTransfer.tsx`
- Modify: `admin/src/platform/resources/RoleGovernanceConsole.tsx`
- Modify: `admin/src/platform/api/organizationRBAC.ts`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Consumes: search/hydration functions from Task 4.
- Produces:

```ts
export type TreeTransferIndex = {
  readonly byKey: ReadonlyMap<string, PlatformTreeTransferNode>;
  readonly childrenByParent: ReadonlyMap<string, readonly string[]>;
  readonly leafDescendantsByBranch: ReadonlyMap<string, readonly string[]>;
  readonly leafKeys: ReadonlySet<string>;
};

export function buildTreeTransferIndex(nodes: readonly PlatformTreeTransferNode[]): TreeTransferIndex;
export function deriveTreeTransferSelection(
  index: TreeTransferIndex,
  selectedLeafKeys: readonly string[],
  visibleKeys: ReadonlySet<string>,
  eligibleLeafKeys: ReadonlySet<string>,
): { checkedKeys: string[]; halfCheckedKeys: string[] };

export type PlatformTreeTransferDataSource = {
  search(query: string): Promise<readonly PlatformTreeTransferNode[]>;
  hydrateSelected(): Promise<readonly PlatformTreeTransferNode[]>;
  loadChildren?(node: PlatformTreeTransferNode): Promise<readonly PlatformTreeTransferNode[]>;
};
```

- Replace the current branch-by-branch full-tree scan. Build indexes once per node set and derive branch state from the index.
- Search requests use a monotonically increasing request ID or `AbortController`; stale responses cannot replace newer results.

- [ ] **Step 1: Write the failing pure TypeScript model tests**

Use Node 22's existing TypeScript-strip pattern:

```js
const result = spawnSync(process.execPath, [
  "--experimental-strip-types",
  "--input-type=module",
  "--eval",
  body(moduleURL),
]);
```

Generate a deterministic fixture with 100 branches, 10,000 leaves and 2,000 selected leaves. Assert exact full/half branch state, hidden-selection preservation and operation counts bounded by a constant multiple of nodes plus selected leaves. Do not use a brittle millisecond-only unit assertion.

- [ ] **Step 2: Run RED model tests**

```bash
rtk node --test scripts/platform-tree-transfer-model.test.mjs
```

Expected: FAIL because `treeTransferModel.ts` and its indexed functions do not exist.

- [ ] **Step 3: Implement the indexed model and async data source**

Move pure filtering, leaf normalization, descendant lookup and checked/half-checked derivation out of the React component. Keep `PlatformTreeTransfer` responsible for rendering, async state, focus and responsive panes only.

Hydrate the selected pane before enabling Save. Server search returns result leaves plus ancestors; selection outside the current result remains intact. Search loading over 300ms shows the existing platform loading state and does not resize the panes.

- [ ] **Step 4: Wire permission and menu assignment without client chunks**

`RoleGovernanceConsole` must:

1. hydrate the current assignment by role;
2. search on the server after a bounded debounce;
3. merge hydrated and searched nodes by stable key;
4. keep 2,000 selected leaves in one local value set;
5. call the existing prepare/impact/apply command exactly once for Save.

Do not split 2,000 selections into multiple apply requests.

- [ ] **Step 5: Add keyboard, ARIA and responsive contract tests**

Require:

```text
ArrowUp / ArrowDown: move within the current tree
ArrowLeft / ArrowRight: collapse and expand
Home / End: first and last visible node
Space: toggle the focused eligible leaf or branch shortcut
Enter: activate the focused command
aria-checked="mixed": partial branches
aria-live="polite": selected-count changes
focus return: connected trigger after modal close
minimum target: 44px
drag-only interaction: absent
```

At `<768px`, use Available/Selected single-pane tabs with sticky actions. At `768-1023px`, use compact two-pane layout. At `>=1024px`, use the standard two-pane layout. No viewport may gain document horizontal overflow at 200% zoom.

- [ ] **Step 6: Run GREEN UI tests and build**

```bash
rtk node --test scripts/platform-tree-transfer-model.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
```

Expected: PASS; the 10,000/2,000 model fixture completes with linear operation bounds and the Admin production build succeeds.

- [ ] **Step 7: Commit Task 5**

```bash
rtk git add admin/src/platform/ui/treeTransferModel.ts scripts/platform-tree-transfer-model.test.mjs admin/src/platform/ui/PlatformTreeTransfer.tsx admin/src/platform/resources/RoleGovernanceConsole.tsx admin/src/platform/api/organizationRBAC.ts admin/src/platform/i18n.ts admin/src/styles.css scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "feat: certify large authorization trees"
```

---

### Task 6: Run Cross-Surface Browser And Accessibility Acceptance

**Required Skills And Tools:** Use `ui-ux-pro-max`; use the in-app Browser first. Use Product Design evidence only for acceptance review, not for visual redesign.

**Files:**
- Create: `resources/evidence/organization-rbac-menu-e2e-qa-20260716.json`
- Create: `scripts/validate-platform-organization-rbac-menu-e2e-evidence.mjs`
- Create: `scripts/platform-organization-rbac-menu-e2e-evidence.test.mjs`
- Modify only when a browser defect is reproduced: `admin/src/platform/resources/organizationUserExperience.tsx`
- Modify only when a browser defect is reproduced: `admin/src/platform/resources/RoleGovernanceConsole.tsx`
- Modify only when a browser defect is reproduced: `admin/src/platform/resources/MenuGovernanceConsole.tsx`
- Modify only when a browser defect is reproduced: `admin/src/platform/ui/PlatformTreeTransfer.tsx`
- Modify only when a browser defect is reproduced: `admin/src/styles.css`
- Modify only when localized recovery copy is required: `admin/src/platform/i18n.ts`

**Evidence Contract:**

```json
{
  "taskId": "organization-rbac-menu-e2e-qa",
  "tool": "in-app-browser",
  "viewports": ["375x812", "390x844", "768x1024", "1024x768", "1280x720", "1440x1024"],
  "largeDataset": {"nodes": 10000, "selected": 2000},
  "consoleErrors": 0,
  "failedFirstPartyRequests": 0,
  "documentHorizontalOverflow": false,
  "zoom200PercentOverflow": false,
  "unapprovedPrincipalDifferences": 0,
  "rollbackVerified": true
}
```

The tracked manifest stores hashes, counts, viewport results and sanitized descriptions. Screenshots remain in the established Product Design evidence directory and must be redaction-scanned before their hashes are recorded.

- [ ] **Step 1: Write the failing evidence validator and negative tests**

Reject manifests that omit any viewport, any required scenario, 10,000/2,000 counts, all eight keyboard keys, `aria-checked=mixed`, count announcement, focus return, 44px targets, reduced motion, 200% zoom, zero unapproved principal differences, cutover checkpoint or rollback verification.

Run:

```bash
rtk node --test scripts/platform-organization-rbac-menu-e2e-evidence.test.mjs
```

Expected: FAIL because the evidence manifest and validator do not exist.

- [ ] **Step 2: Start a bounded local acceptance runtime**

Use a temporary SQLite database and temporary environment file. Seed only synthetic organizations, role groups, roles, users, menus and permissions. Start the API and Admin dev server as bounded background processes, record their PIDs, and terminate both after acceptance. Do not reuse production data or credentials.

Required runtime states during the run:

```text
legacy serving / target writes false
dual-read serving / target writes false
target serving / target writes false
target serving / target writes true
restored checkpoint / legacy serving / target writes false
```

- [ ] **Step 3: Verify organization, user and conflict workflows**

At desktop and mobile widths:

1. bind role groups to an organization and inspect group provenance and effective-role counts;
2. select an organization for a tenant user, verify server-derived read-only tenant and then load roles;
3. change organization with invalid current roles, reject once, then apply explicit removal/replacement remediation;
4. prove a platform principal has no organization/tenant controls and rejects tenant roles;
5. cover zero-impact and conflicting group unbind, role move and role disable;
6. confirm nested role groups and roles without exactly one group are rejected;
7. verify stale preview produces a recoverable conflict and focuses the recovery action.

- [ ] **Step 4: Verify menu, permission and 10k Tree Transfer workflows**

1. directory click expands/collapses without navigation;
2. page click navigates to the registered route;
3. menu save/reload and permission save/reload produce separate audit actions;
4. search the 10,000-node fixture and retain selected values outside the result;
5. scroll both panes, verify virtualization markers and half selection;
6. save all 2,000 selected leaves in one prepare/impact/apply flow;
7. verify tap/input feedback appears within 100ms and no visible main-thread freeze occurs;
8. confirm loading states reserve pane dimensions and do not create layout shift.

- [ ] **Step 5: Verify keyboard, screen reader semantics and responsive behavior**

Run the eight-key matrix in both Available and Selected panes. Inspect the accessibility tree for tree/treeitem roles, expanded/selected/disabled states and mixed checkbox state. Confirm count changes announce politely without stealing focus. Close each modal with Save, Cancel and Escape and confirm focus returns to the connected trigger.

Repeat at all six viewports, reduced motion and 200% zoom. Verify no document or modal horizontal overflow and every primary touch/control target is at least 44px.

- [ ] **Step 6: Verify dual-read, cutover and rollback in the Browser/API session**

Confirm dual-read serves legacy menus while the persisted observation count reaches every synthetic active principal. Promote target read only after zero differences, then enable target writes after the observation gate. Make one reviewed role-menu write, prove config-only legacy rollback is rejected, restore the checkpoint, run rollback verification and prove legacy authorization/menu hashes match the pre-cutover evidence.

- [ ] **Step 7: Record and validate evidence**

```bash
rtk node scripts/validate-platform-organization-rbac-menu-e2e-evidence.mjs
rtk node --test scripts/platform-organization-rbac-menu-e2e-evidence.test.mjs
rtk git diff --check
```

Expected: PASS with zero console errors, zero failed first-party requests, zero unapproved principal differences, no overflow and verified rollback.

- [ ] **Step 8: Commit Task 6**

Stage only the evidence validator, test, manifest and any files changed for reproduced defects:

```bash
rtk git add resources/evidence/organization-rbac-menu-e2e-qa-20260716.json scripts/validate-platform-organization-rbac-menu-e2e-evidence.mjs scripts/platform-organization-rbac-menu-e2e-evidence.test.mjs
rtk git add admin/src/platform/resources/organizationUserExperience.tsx admin/src/platform/resources/RoleGovernanceConsole.tsx admin/src/platform/resources/MenuGovernanceConsole.tsx admin/src/platform/ui/PlatformTreeTransfer.tsx admin/src/styles.css admin/src/platform/i18n.ts
rtk git commit -m "test: verify organization authorization e2e"
```

Before committing, unstage unchanged UI files so the commit contains no metadata-only churn.

---

### Task 7: Synchronize Contracts, Resolve Locks And Close The Node

**Files:**
- Modify: `resources/platform-organization-rbac-menu-contract.json`
- Modify: `scripts/validate-platform-organization-rbac-menu-contract.mjs`
- Modify: `scripts/platform-organization-rbac-menu-contract.test.mjs`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: corresponding governance validators and negative tests
- Modify: `README.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-organization-rbac-menu-contract.md`
- Modify: `docs/admin-rbac-menu.md`
- Modify: `docs/platform-ui-optimization-assessment.md`
- Modify: `docs/superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md`

**Interfaces:**
- Consumes: committed implementation and evidence from Tasks 1-6.
- Produces: `organization-rbac-menu-e2e-qa.status=implemented`, node closeout evidence, updated release blocker projection and truthful lock topology.

- [ ] **Step 1: Write failing governance assertions before changing status**

Require the organization node to remain pending unless all of these are present and validated:

```text
all-principal candidate and persisted comparison tests
zero unapproved principal differences
all shipped write-entry tests
platform-principal allowlist tests
10,000 nodes / 2,000 selected evidence
eight-key and ARIA evidence
six viewports plus 200% zoom
target read and target write promotion evidence
verified post-write checkpoint rollback
independent audit and browser evidence manifest
```

Add negative tests that remove each gate independently and expect the validator to fail.

- [ ] **Step 2: Correct the real lock topology**

The complete organization node touches shared error-code surfaces through service objects, generated responses, HTTP integration, audit correlation and docs. Add these locks to `organization-rbac-menu-e2e-qa`:

```json
[
  "service-contract",
  "admin-resource-api",
  "codegen",
  "audit-policy",
  "docs"
]
```

Remove `release-blocker-contract-lanes` from `parallelBatches`, or replace it with a documented subtask-only execution note outside the machine parallel batch. Whole-node parallel execution with `unified-error-code-governance` is not lock-safe.

- [ ] **Step 3: Update the organization contract from declared gates to collected evidence**

Change target serving/write gates to implemented only after Tasks 1-6 pass. Remove these entries from menu evidence `unclaimedGates` only when their corresponding tracked evidence exists:

```text
tree-transfer-10000-node-performance
full-tree-transfer-acceptance
dual-read-principal-equivalence
migration-cutover
migration-rollback
organization-rbac-menu-e2e
```

Keep datasource routing, federation, XA, MQ, search projection and workload identity in `notOwned`.

- [ ] **Step 4: Close task, engineering capability and release blocker projections**

Mark only `organization-rbac-menu-e2e-qa` implemented. Remove it from `releaseBlockingNodes`, pending closeout evidence and controlled unfinished projections. Recalculate counts from the graph rather than hand-editing stale totals. Do not change the nine deferred post-release nodes.

- [ ] **Step 5: Run focused and broad verification**

```bash
rtk go test ./internal/platform/organizationrbac ./internal/platform/httpapi ./internal/platform/bootstrap ./internal/platform/adminresource ./internal/platform/datalifecycle ./cmd/platform-admin
rtk node --test scripts/platform-organization-rbac-menu-contract.test.mjs
rtk node --test scripts/platform-organization-rbac-menu-e2e-evidence.test.mjs
rtk node --test scripts/platform-tree-transfer-model.test.mjs
rtk node scripts/validate-platform-organization-rbac-menu-contract.mjs
rtk node scripts/validate-platform-organization-rbac-menu-e2e-evidence.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node --test scripts/platform-foundation-docs-drift.test.mjs
rtk go test ./...
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

Expected: every command exits 0; CodeGraph reports no pending files; the worktree contains only the intended Task 7 changes before commit.

- [ ] **Step 6: Perform closeout review**

Run one phase-level `neat-freak` synchronization because this is a cross-module release blocker closeout. Perform an independent diff review focused on accidental error-code registry churn, generated artifact freshness, secret/PII leakage, deferred capability drift and unsupported claims.

- [ ] **Step 7: Commit Task 7**

```bash
rtk git add resources/platform-organization-rbac-menu-contract.json scripts/validate-platform-organization-rbac-menu-contract.mjs scripts/platform-organization-rbac-menu-contract.test.mjs resources/platform-foundation-task-graph.json resources/platform-engineering-capabilities.json resources/platform-task-execution-audit.json resources/platform-foundation-alignment-audit.json resources/platform-goal-completion-audit.json resources/platform-objective-conformance.json resources/platform-node-closeout-audit.json README.md docs/platform-foundation-task-map.md docs/platform-roadmap.md docs/platform-organization-rbac-menu-contract.md docs/admin-rbac-menu.md docs/platform-ui-optimization-assessment.md docs/superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md
rtk git add scripts/validate-platform-foundation-task-graph.mjs scripts/platform-foundation-task-graph.test.mjs scripts/validate-platform-engineering-capabilities.mjs scripts/platform-engineering-capabilities.test.mjs scripts/validate-platform-task-execution-audit.mjs scripts/platform-task-execution-audit.test.mjs scripts/validate-platform-foundation-alignment.mjs scripts/platform-foundation-alignment.test.mjs scripts/validate-platform-goal-completion-audit.mjs scripts/platform-goal-completion-audit.test.mjs scripts/validate-platform-objective-conformance.mjs scripts/platform-objective-conformance.test.mjs scripts/validate-platform-node-closeout-audit.mjs scripts/platform-node-closeout-audit.test.mjs scripts/platform-foundation-docs-drift.test.mjs
rtk git commit -m "feat: close organization authorization e2e"
```

## Final Acceptance Summary

The node is complete only when all of the following are true at the same revision:

- every active principal has equal legacy and persisted target scope, roles, allow/deny permissions, data scope and effective menus;
- platform principals come only from the reviewed allowlist and cannot receive tenant ownership or tenant roles;
- generic Admin writes, specialized commands and scheduled lifecycle handling cannot bypass the organization domain validator;
- dual-read evidence covers every active principal, not only principals who happened to request `/api/admin/menus`;
- target read and target write gates are separately promoted from persisted evidence;
- post-write rollback is verified against the restored checkpoint and canonical legacy snapshot hash;
- Tree Transfer handles 10,000 nodes and 2,000 selected leaves with server search, deterministic hydration, linear indexed derivation and one atomic save;
- keyboard, ARIA, focus, reduced motion, 44px targets, six viewports and 200% zoom are proven in the Browser evidence;
- the organization node is removed from release blockers without changing deferred datasource/integration capabilities;
- `unified-error-code-governance` shared files and generators were serialized rather than edited concurrently.
