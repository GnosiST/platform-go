# Task 5: Product Design Role Workbench Redesign

## Outcome

Implemented the approved Role Management workbench redesign inside the existing platform design system. The role group -> role hierarchy, Task 4 permission write modes, backend contracts, routes, permission codes, and capability manifests remain unchanged.

## Audit Evidence Applied

The implementation was grounded in the supplied desktop and 390px audit captures under `/tmp/platform-go-role-governance-audit-20260717/`. The changes directly address the flat action hierarchy, unstable navigation/detail proportions, mobile label overflow, misleading menu assignment entry state, and inaccessible tree selection/level semantics visible in those captures.

## Implementation

- Projected explicit `aria-selected` and `aria-level` values into `AdminTreeWorkbench` data while preserving Ant Tree keyboard behavior, selected/active keys, and 44px expanders.
- Added full-label title affordances, one-line ellipsis, full-width tree rows, a stable tabular count column, and horizontal-overflow containment for workbench and Tree Transfer nodes.
- Bounded desktop workbench navigation to `clamp(264px, 28vw, 320px)` with a flexible detail track; detail sticking is desktop-only and tablet/mobile remain in normal flow.
- Added a stable, focus-visible Role detail target and 360px detail/empty minimum height using the existing Menu Governance focus pattern. Tree Transfer modal cleanup still restores focus to the opening command without scrolling.
- Reworked role detail to `Title level={4}`, localized status/scope/data-scope summaries, responsive two-column descriptions, and a full-width description row.
- Split role commands into unframed, divider-based `Access Control` and `Lifecycle` sections. All detail commands now use `AdminActionButton`; permission assignment remains the single primary authorization action and disable remains separately danger-styled.
- Enforced `canEditMenus = roleMenuMigrationWriteEnabled && canAssignMenus && role.status === "enabled"` at both entry and modal boundaries, plus a disabled-role save guard. Ineligible states use localized `View Menus` copy and distinct legacy, access, or disabled-role read-only explanations.
- Added the general `AdminModal` platform wrapper, based `AdminFormModal` on it, and removed direct Ant `Modal` use only from the role workspace.
- Removed the unused `visibleCheckedKeys` Tree Transfer variable and its false-positive validator requirement; deterministic selection behavior remains covered by the Tree Transfer model suite.
- Made the mobile Tree Transfer toolbar sticky inside modal scrolling, with full-width search and a stable two-column 44px bulk-action row.
- Added matching Chinese and English labels for Access Control, Lifecycle, View Menus, and role-menu read-only states.

## Tests And Contracts

Added `scripts/platform-role-workbench-redesign.test.mjs` with six focused source-contract checks. Strengthened the Admin UI validator and mutation suite for focus, ARIA selection/level, menu edit eligibility, View Menus copy, localized summaries, bounded tracks, minimum height, node ellipsis, mobile transfer controls, wrapper use, and dead-variable removal.

Regression-first evidence:

- Red: the focused Admin UI acceptance test failed with all new Task 5 contracts absent before product code changes.
- Green: `rtk node --test --test-reporter=dot scripts/platform-role-workbench-redesign.test.mjs` - 6/6 passed.
- `rtk node --test --test-reporter=dot scripts/platform-tree-transfer-model.test.mjs` - 3/3 passed.
- `rtk node --test --test-reporter=dot scripts/admin-ui-contracts.test.mjs` - 204/204 passed.
- `rtk node scripts/validate-admin-ui-contracts.mjs` - passed.
- `rtk node scripts/validate-admin-i18n.mjs` - passed.
- `rtk npm --prefix admin run typecheck` - passed.
- `rtk npm --prefix admin run build` - passed; 3780 modules transformed.
- `rtk codegraph sync .` and `rtk codegraph status` - passed; index is up to date.

## Scope And Review

- `admin/src/platform/resources/rolePermissionWorkflow.ts` has no diff.
- No backend API, capability manifest, generated resource contract, runtime schema logic, permission code, route, version, or tag changed.
- No new visual identity, palette, radius, typography system, icon dependency, nested card, or page-specific hard-coded color was introduced.
- Browser QA was intentionally not run in this implementation subtask. Consolidated desktop/mobile/dark/zoom/focus acceptance remains with the owning agent per the Task 5 brief.
