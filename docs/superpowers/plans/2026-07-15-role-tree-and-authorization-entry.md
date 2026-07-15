# Role Tree And Authorization Entry Implementation Plan

## Objective

Close `role-tree-and-authorization-entry` with a production-usable strict role tree, reviewed role move/disable flows, atomic role policy assignment, a controlled separate menu entry and synchronized governance evidence.

## Steps

1. Correct the blocking role governance backend gaps.
   - Add target-mode role and role-group snapshot writers for permitted metadata mutations.
   - Add role-state conflict query results required for explicit remediation.
   - Extend role policy prepare/apply to cover allow, deny and data scope atomically.
   - Regenerate Admin OpenAPI and TypeScript service-object clients.

2. Add shared platform UI components.
   - Implement `AdminTreeWorkbench` for strict two-level navigation and detail layout.
   - Implement `PlatformTreeTransfer` with accessible keyboard, responsive and large-data behavior.
   - Export both through the platform UI wrapper index.

3. Build the role governance console.
   - Route both role paths to one console after existing read-access checks.
   - Load role groups, roles, permissions and menus through platform APIs.
   - Implement role/group metadata forms, move/disable previews and remediation.
   - Implement functional permission assignment and controlled read-only menu assignment entry.

4. Add i18n, CSS and contract tests.
   - Add Chinese and English labels in the same change.
   - Add responsive, focus, ARIA, wrapper-usage and boundary validators.
   - Keep `design-taste-frontend` deferred for public surfaces.

5. Verify and close out.
   - Run focused Go and Admin tests, shared validators, production build and `git diff --check`.
   - Run the local browser acceptance set and record an evidence manifest without overstating the in-app browser result.
   - Update task graph, organization contract boundary, docs and node closeout evidence.
   - Refresh CodeGraph, independently review the diff, commit and leave the workspace clean.

## Verification

- `rtk go test ./...`
- `rtk node scripts/validate-platform-organization-rbac-menu-contract.mjs`
- `rtk node scripts/validate-platform-foundation-task-graph.mjs`
- `rtk node scripts/validate-platform-node-closeout-audit.mjs`
- `rtk node scripts/validate-admin-i18n.mjs`
- `rtk node scripts/validate-admin-ui-contracts.mjs`
- `rtk node --test scripts/admin-ui-contracts.test.mjs`
- `rtk npm --prefix admin run build`
- focused browser acceptance at the frozen viewports
- `rtk git diff --check`
- `rtk codegraph sync .`
- `rtk codegraph status`
