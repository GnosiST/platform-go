# Platform Governance Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent the platform foundation from regressing to tenant-only governance or growing into business-specific personnel modeling.

**Architecture:** Keep tenants, org units, users, role groups, roles and area codes as schema-driven foundation resources. Enforce role groups as classification-only metadata, and reserve personnel, employee and position resources for an optional capability boundary.

**Tech Stack:** Node.js validation scripts, Go admin resource contracts, Gin + GORM + Casbin + JWT backend, Refine + React + Ant Design frontend contract.

---

### Task 1: Role Group Governance Gate

**Files:**
- Modify: `scripts/validate-admin-resources.mjs`
- Test: `scripts/validate-admin-resources.test.mjs`

- [x] **Step 1: Add a failing test for role-group policy fields**

Add a test that mutates `role-groups.schema.fields` with `permissions` and `inheritFrom`, runs `rtk node --test scripts/validate-admin-resources.test.mjs`, and expects the validator to reject role groups that try to grant permissions or inheritance.

- [x] **Step 2: Add the validator rule**

Extend `validatePlatformGovernanceContract` so `role-groups` fails when fields contain permission, data-scope or inheritance semantics. Keep `roles.groupCode` as the only role-group binding path.

- [x] **Step 3: Verify the gate**

Run `rtk node --test scripts/validate-admin-resources.test.mjs` and `rtk node scripts/validate-admin-resources.mjs`.

### Task 2: Personnel Extension Boundary

**Files:**
- Modify: `resources/platform-reference-coverage.json`
- Modify: `scripts/validate-platform-reference-coverage.mjs`
- Test: `scripts/platform-reference-coverage.test.mjs`

- [x] **Step 1: Declare the extension boundary**

Add `extensionBoundary` for `personnel-and-positions`, with `personnel`, `employees`, `staff`, `employeeProfiles`, `positions` and `positionAssignments` staying outside the default platform contract unless the optional `personnel` capability is enabled.

- [x] **Step 2: Validate extension boundaries**

Teach the reference coverage validator to check `extensionBoundary` the same way it checks business boundaries: every boundary must declare `expectedCapability`, must set `mustStayOutOfDefaultPlatform=true`, and must reject listed resources in the default admin contract while that capability is not enabled.

- [x] **Step 3: Verify the gate**

Run `rtk node --test scripts/platform-reference-coverage.test.mjs` and `rtk node scripts/validate-platform-reference-coverage.mjs`.

### Task 3: Documentation Alignment

**Files:**
- Modify: `README.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `docs/admin-rbac-menu.md`
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/platform-foundation-task-map.md`

- [x] **Step 1: Document the boundary**

State that users are platform accounts and principals, while employee files, positions and employment assignments belong to an optional `personnel` capability.

- [x] **Step 2: Document the governance rule**

State that role groups classify and govern roles through `roles.groupCode` only. Permission grants, denies and data scopes stay on roles.

- [x] **Step 3: Verify docs and generated contracts**

Run `rtk node scripts/validate-admin-resources.mjs`, `rtk node scripts/validate-platform-reference-coverage.mjs`, `rtk git diff --check`, and the relevant Go tests.

### Task 4: Governance Resource Menu Closure

**Files:**
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `internal/platform/adminresource/store.go`
- Modify: `docs/platform-foundation-task-map.md`

- [x] **Step 1: Add a failing fallback-menu test**

Extend the default store coverage so `NewStore()` proves the fallback seed menu exposes `/org-units`, `/role-groups` and `/area-codes`, matching the manifest-driven runtime path.

- [x] **Step 2: Add the missing fallback menu rows**

Add seed menu records for `org-units`, `role-groups` and `area-codes` beside the existing tenants/users/roles/configuration menus. Keep permissions, route names, group names and cache behavior aligned with the core capability manifests.

- [x] **Step 3: Document the closure**

Update the foundation task map to state that organization, role-group and area-code resources are platform governance resources with menu, schema, seed-data and validation coverage.

- [x] **Step 4: Verify the node**

Run `rtk go test ./internal/platform/adminresource`, `rtk node scripts/validate-admin-resources.mjs`, `rtk node scripts/validate-platform-capability-profiles.mjs`, `rtk git diff --check`, and refresh/check CodeGraph status.

### Task 5: Current Principal Governance Context Proof

**Files:**
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `docs/platform-foundation-task-map.md`

- [x] **Step 1: Prove current-session returns governance context**

Extend current-session HTTP coverage so the `ops` principal returns `tenantCode=platform`, `orgUnitCode=platform-ops` and `areaCode=110000` alongside role-backed permissions.

- [x] **Step 2: Document the evidence**

Update the foundation task map to cite the current-session test as the contract proving tenant, organization and area context is exposed through the unified principal API.

- [x] **Step 3: Verify the node**

Run `rtk go test ./internal/platform/httpapi`.
