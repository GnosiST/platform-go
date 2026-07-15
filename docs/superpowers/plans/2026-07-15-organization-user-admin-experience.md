# Organization And User Admin Experience Implementation Plan

## Goal

Close `organization-user-admin-experience` with complete organization role-group administration, derived user tenant and role constraints, accessible responsive interaction, generated service-object clients and synchronized governance evidence.

## Completed Steps

- [x] Project organization role-group counts, effective role counts and role-pool names through Admin resource snapshots.
- [x] Add generated service-object client support for organization role pools, binding previews, impacts, conflict pages, remediation apply and user assignment.
- [x] Load complete organization, role-group, role-pool and conflict result sets across server pagination limits.
- [x] Add organization role-group selection, impact confirmation, explicit conflict remediation and role-pool provenance detail.
- [x] Derive user tenant from organization, constrain roles to the organization role pool and retain invalid existing roles until explicit removal.
- [x] Separate metadata saves from authorization commands and avoid misleading retry after authorization-only success.
- [x] Prevent async option loading from auto-selecting a new user's organization or resetting entered form values.
- [x] Add localized read-only, disabled, live-region, invalid-state and cancellation behavior.
- [x] Disable Refine third-party telemetry by default.
- [x] Add static and mutation gates for routing, pagination, stale-response invalidation, partial-success boundaries, form initialization, telemetry, accessibility and responsive UI behavior.
- [x] Capture and inspect reduced-motion browser evidence at 375, 390, 768, 1024, 1280 and 1440 widths.
- [x] Synchronize organization RBAC, Admin resource, UI assessment, roadmap and task-governance documents.

## Verification

Focused verification:

```bash
rtk go test ./...
rtk node scripts/validate-platform-organization-rbac-menu-contract.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
```

Browser acceptance uses a local Playwright 1.55 fallback because the installed in-app Browser module failed to initialize with `Cannot redefine property: process`. The fallback is not added to project dependencies and is recorded honestly in the evidence manifest.

## Closeout Evidence

- Design gates: `superpowers:brainstorming`, Product Design audit, `ui-ux-pro-max`.
- Browser manifest: `resources/evidence/organization-user-admin-experience-20260715.json`.
- Screenshot root: `.superpowers/product-design-audit/organization-user-admin-experience/2026-07-15/`.
- Contract evidence: generated Admin OpenAPI/client, Go tests, Admin UI validator/mutation tests, typecheck and production build.
- Cleanup: one phase-level `neat-freak` invocation completed after documentation and governance synchronization.
