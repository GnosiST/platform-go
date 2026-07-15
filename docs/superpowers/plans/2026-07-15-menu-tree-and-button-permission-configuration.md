# Menu Tree And Button Permission Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Close `menu-tree-and-button-permission-configuration` with strict directory/page authoring, native page-button and role-menu relations, a cutover-gated role-menu assignment implementation and controlled legacy/candidate navigation seams.

**Architecture:** Extend the existing normalized Admin menu record and `organizationrbac` GORM/Domain Command runtime, inject menu/permission ownership into the Admin composition root, and keep legacy menu serving as the only enabled mode. Add deterministic candidate backfill, compare and target resolution behind independent closed serving/write gates; the next E2E node owns principal-equivalence proof, cutover, observation and gate enablement.

**Tech Stack:** Go, Gin, GORM, persisted Query/Domain Command objects, React, TypeScript, Refine, Ant Design, platform UI wrappers, Node governance validators and the in-app Browser.

## Global Constraints

- Directory menus never navigate; page menus are leaves.
- `role_menu` persists page leaves only and derives directory ancestors.
- Page-button permissions control UI visibility only; Casbin API permissions remain authoritative.
- Legacy `menus.permission` is read-only during migration and default serving remains compatible.
- Target serving and new role-menu writes remain disabled until the later E2E cutover explicitly enables each gate.
- All relation mutations use one native transaction, optimistic revision, idempotency and sanitized audit.
- No datasource, shard, SQL, script or expression input is accepted from clients.
- Chinese and English i18n, platform wrappers, 44px targets, keyboard/focus, reduced motion and frozen viewport evidence are mandatory.

---

### Task 1: Freeze Executable Menu Contracts

**Files:**
- Modify: `resources/platform-organization-rbac-menu-contract.json`
- Modify: `scripts/validate-platform-organization-rbac-menu-contract.mjs`
- Modify: `scripts/platform-organization-rbac-menu-contract.test.mjs`
- Create: `internal/platform/organizationrbac/menu_model_test.go`

- [ ] Add failing contract and Go tests for directory/page invariants, typed parameters, page-button metadata, page-only role-menu storage and explicit serving modes.
- [ ] Run the focused Node and Go tests and confirm the new assertions fail against the current implementation.
- [ ] Add the minimum shared model constants/types required by later repository work.
- [ ] Re-run the focused tests and keep the contract validator green.

### Task 2: Add Native Menu Relations And Validation

**Files:**
- Modify: `internal/platform/adminresource/gorm_store.go`
- Modify: `internal/platform/adminresource/menu.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: `internal/platform/bootstrap/admin_resources.go`
- Modify: `internal/platform/bootstrap/admin_resources_test.go`
- Modify: `internal/platform/organizationrbac/lifecycle_mutations.go`
- Create: `internal/platform/organizationrbac/menu_repository.go`
- Create: `internal/platform/organizationrbac/menu_repository_test.go`

- [ ] Add failing repository tests for schema migration, directory/page validation, cycle rejection, page-button permission integrity, page-only role-menu writes, stale revision and no-op revision behavior.
- [ ] Add normalized menu columns plus native role-menu, revision and page-button tables.
- [ ] Implement repository reads and atomic diffs without snapshot delete-and-recreate for the new relations.
- [ ] Inject ownership-aware menu and permission writers through bootstrap so generic CRUD cannot bypass validation, revision, audit or lifecycle reference checks.
- [ ] Preserve legacy menu projections and verify SQLite reload behavior.

### Task 3: Add Role Menu Service Objects And Runtime Resolution

**Files:**
- Modify: `internal/platform/organizationrbac/service_objects.go`
- Create: `internal/platform/organizationrbac/navigation_service_objects.go`
- Create: `internal/platform/organizationrbac/navigation_service_objects_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `scripts/admin-service-object-definitions.mjs`

- [ ] Add failing tests for atomic menu-definition get/replace, prepare/impact/apply, idempotent replay, stale preview, disabled role/page rejection, complete-set atomicity and target ancestor derivation.
- [ ] Register `platform.navigation.menu-definition.get`, `platform.navigation.menu-definition.create`, `platform.navigation.menu-definition.replace`, `platform.navigation.role-menus.get`, `platform.navigation.role-menu-change.impact`, `platform.navigation.role-menu-change.prepare` and `platform.navigation.role-menus.replace` with distinct create/update permission gates.
- [ ] Add deterministic legacy-policy candidate backfill and `platform.navigation.role-menu-migration.compare` without claiming all-principal equivalence.
- [ ] Add explicit legacy/compare/dual-read/target resolution seams while rejecting target serving and role-menu writes when their independent cutover gates are closed.
- [ ] Regenerate OpenAPI and TypeScript service-object clients and validate Go/JS definition consistency.

### Task 4: Build The Menu Governance Console

**Files:**
- Create: `admin/src/platform/resources/MenuGovernanceConsole.tsx`
- Modify: `admin/src/platform/refine/ResourceRoutePage.tsx`
- Modify: `admin/src/platform/api/organizationRBAC.ts`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

- [ ] Add failing UI contract tests for dedicated routing, directory/page forms, typed parameter validation, page-button permission integrity, minimum read/write permissions and stale search protection.
- [ ] Implement the tree/detail workbench with explicit directory/page creation and structural conflict feedback.
- [ ] Implement page metadata and nested page-button editing with platform wrappers and localized copy.
- [ ] Preserve stable selection, focus return, keyboard operation, reduced motion, 44px targets and responsive density.

### Task 5: Wire Cutover-Gated Role Menu Assignment

**Files:**
- Modify: `admin/src/platform/resources/RoleGovernanceConsole.tsx`
- Modify: `admin/src/platform/ui/PlatformTreeTransfer.tsx`
- Modify: `admin/src/platform/api/organizationRBAC.ts`
- Modify: `scripts/admin-ui-contracts.test.mjs`

- [ ] Add failing tests that require page-only values, derived directory half-selection, disabled historical selection preservation, minimum permissions and the closed migration-write gate.
- [ ] Wire the service-object prepare/impact/apply flow without enabling it while the migration-write gate is closed.
- [ ] Keep legacy visibility context and explicit unavailable messaging until the later E2E node enables target mutation.
- [ ] Verify no client-side chunking or implicit future descendant grants.

### Task 6: Governance, Browser Evidence And Closeout

**Files:**
- Modify: `README.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-organization-rbac-menu-contract.md`
- Modify: `docs/platform-ui-optimization-assessment.md`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: corresponding governance validators and tests
- Create: `resources/evidence/menu-tree-and-button-permission-configuration-20260715.json`

- [ ] Run focused Go, Node and Admin tests, then `rtk go test ./...` and the Admin production build.
- [ ] Verify directory expansion, page navigation, menu editing, page buttons and the explicitly gated role-menu entry at `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720` and `1440x1024`.
- [ ] Record focused menu-workbench console/request/focus/overflow/accessibility evidence without claiming full Tree Transfer, 10,000-node, dual-read-equivalence or migration-cutover gates.
- [ ] Run phase-level documentation synchronization, CodeGraph refresh, independent diff review, commit and confirm a clean worktree.

## Verification Commands

- `rtk go test ./...`
- `rtk node scripts/validate-platform-organization-rbac-menu-contract.mjs`
- `rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json`
- `rtk node scripts/generate-admin-resource-contract.mjs`
- `rtk node scripts/generate-admin-openapi.mjs`
- `rtk node scripts/generate-admin-codegen-preview.mjs`
- `rtk node scripts/validate-admin-service-object-definitions.mjs`
- `rtk node scripts/validate-platform-service-object-runtime.mjs`
- `rtk node scripts/validate-admin-resources.mjs`
- `rtk node scripts/validate-platform-admin-api-boundary.mjs`
- `rtk node scripts/validate-platform-engineering-capabilities.mjs`
- `rtk node scripts/validate-platform-foundation-task-graph.mjs`
- `rtk node scripts/validate-platform-task-execution-audit.mjs`
- `rtk node scripts/validate-platform-foundation-alignment.mjs`
- `rtk node scripts/validate-platform-goal-completion-audit.mjs`
- `rtk node scripts/validate-platform-objective-conformance.mjs`
- `rtk node scripts/validate-platform-node-closeout-audit.mjs`
- `rtk node scripts/validate-admin-i18n.mjs`
- `rtk node scripts/validate-admin-ui-contracts.mjs`
- `rtk node --test scripts/admin-ui-contracts.test.mjs`
- `rtk npm --prefix admin run build`
- focused Browser acceptance at the frozen six viewports
- `rtk git diff --check`
- `rtk codegraph sync .`
- `rtk codegraph status`
