# Platform Business Neutrality Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep `platform-go` as a reusable business-neutral foundation and ensure no built-in `zshenmez` business implementation ships in the repository.

**Architecture:** The platform keeps generic capability manifests, admin resources, route registration seams, permission codes and validation gates. Concrete business manifests belong to downstream business repositories or independent plugin packages, not `platform-go`.

**Tech Stack:** Gin, GORM, Casbin, JWT, Refine, React, Ant Design.

---

### Task 1: Runtime Composition Root

**Files:**
- Modify: `internal/apps/manifests.go`
- Modify: `internal/apps/admin_routes.go`
- Modify: `internal/apps/app_routes.go`
- Confirm/remove any historical inline business package files if present.

- [x] Make `apps.DefaultManifests`, `apps.DefaultAdminRoutes` and `apps.DefaultAppRoutes` return empty slices.
- [x] Keep the default composition root free of built-in business manifests, admin routes and app routes.
- [x] Verify `rtk go test ./internal/apps ./internal/platform/bootstrap ./internal/platform/httpapi`.

### Task 2: Tests And Contracts

**Files:**
- Modify: `internal/apps/admin_routes_test.go`
- Modify: `internal/apps/app_routes_test.go`
- Modify: `internal/platform/bootstrap/capabilities_test.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `cmd/platform-contracts/main_test.go`

- [x] Replace enabled `external-business-capability` assertions with business-neutral assertions.
- [x] Keep tests proving unknown business capability IDs do not resolve without external manifests.
- [x] Verify `rtk go test ./...`.

### Task 3: Governance Resources

**Files:**
- Modify: `resources/platform-capability-profiles.json`
- Modify: `resources/platform-reference-discovery.json`
- Modify: `resources/platform-reference-coverage.json`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Delete: `external business package hardening contract`
- Delete: `external business hardening validator`
- Delete: `external business hardening tests`

- [x] Remove `external reference classification` as an executable profile.
- [x] Keep reference discovery/coverage as external classification evidence only.
- [x] Remove production preflight dependency on business hardening.
- [x] Verify `rtk node --test scripts/*.test.mjs` and platform validators.

### Task 4: Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `docs/admin-ui-foundation.md`

- [x] Replace stale business-migration and business-hardening wording with external business integration guidance.
- [x] State that `platform-go` does not ship concrete `zshenmez` business resources, routes or stores.
- [x] Verify the old concrete business identifiers no longer appear in README, docs, resources, scripts, internal packages or commands.

### Task 5: Final Verification

- [x] Run `rtk go test ./...`.
- [x] Run `rtk node --test scripts/*.test.mjs`.
- [x] Run `rtk node scripts/validate-admin-resources.mjs`.
- [x] Run `rtk node scripts/validate-admin-i18n.mjs`.
- [x] Run `rtk node scripts/validate-admin-ui-contracts.mjs`.
- [x] Run `rtk npm --prefix admin run build`.
- [x] Run `rtk git diff --check`.
- [x] Run `rtk codegraph sync . && rtk codegraph status`.

## Closeout Notes

- 2026-07-08: The default static admin manifest no longer carries optional `app-phone` resources. `app-phone-verifications` and `app-phone-bindings` are contributed only by the optional `app-phone` capability when an app-ready composition enables it. `scripts/validate-admin-resources.mjs` and `scripts/admin-resource-contract-generators.test.mjs` now guard this boundary.
