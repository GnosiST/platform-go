# Role Tree And Authorization Entry Design

> Status: implemented and closed on 2026-07-15.

## Goal

Replace the split role-group and role resource experience with one strict two-level governance workbench, then provide separate role menu and permission authorization entry points without pulling the later menu-schema migration into this node.

## Product Outcome

An administrator can locate a role through its one owning group, inspect its scope and authorization state, move or disable it through a reviewed impact flow, and edit its allow, deny and data-scope policy through one dedicated permission workflow. The same workbench is reached from both legacy role routes so existing deep links remain valid.

## Dependency Correction

Implementation review found three backend gaps that would otherwise make the UI misleading:

- target-mode generic snapshots reject every role and role-group mutation because only users and organizations have domain-owned writers;
- role move and disable previews expose conflict counts but not the affected user-role rows required for explicit remediation;
- role permission replacement persists only allow permissions while deny permissions and data scope remain outside the reviewed command.

This node therefore owns the smallest correction required for a usable role workbench: target-safe role and role-group metadata writers, role-state conflict hydration, and one atomic role-policy prepare/apply command covering allow permissions, deny permissions and data scope. It does not add role-menu persistence, menu schema migration or menu runtime cutover.

## Workbench Direction

- Both `/roles` and `/role-groups` render one `RoleGovernanceConsole` after the existing route access gate.
- `AdminTreeWorkbench` renders role groups as level one and roles as level two. Nested role groups are surfaced as invalid data, never rendered as a third level.
- Selecting a group shows scope, tenant, status and role count. Selecting a role shows its owning group, status, data scope and authorization counts.
- Role movement and disablement always use prepare, impact, optional explicit remediation and apply. Conflict rows are listed with minimum identifiers only.
- Role groups remain classification boundaries and never grant permissions.

## Authorization Entry Direction

- `Assign Permissions` is functional in this node. One reviewed atomic change owns allow permissions, deny permissions and data scope. Allow and deny selections are mutually exclusive.
- Permissions are grouped by API and page-button resource type, then by capability/resource when metadata is available.
- `Assign Menus` is a real separate entry point using the same `PlatformTreeTransfer`, but remains read-only with an explicit migration notice until `menu-tree-and-button-permission-configuration` adds page-only `role_menu` persistence and menu revisions.
- The menu entry may display the current legacy permission-derived visibility as context. It must not imply that saving is available or durable.

## Shared Components

`AdminTreeWorkbench` provides the responsive tree/detail shell. The implemented `PlatformTreeTransfer` owns leaf-only values plus revision metadata, local filtered-result selection over loaded nodes, half-selected parents, disabled reasons, counts, selected replay and virtual rendering. It exposes an optional child-loading seam. Server-backed search, hydration of unloaded selections and the 10,000-node/2,000-selected certification remain full-E2E gates after menu runtime support exists.

At 1024px and above, the Transfer uses two panes. From 768px to 1023px it uses a compact two-pane layout. Below 768px it uses Available/Selected tabs with sticky actions. Keyboard interaction follows the treeview pattern, mixed parents expose `aria-checked="mixed"`, count changes use `aria-live="polite"`, targets are at least 44px and closing returns focus to the trigger.

## Visual Direction

The existing Refine, React, Ant Design and platform-wrapper system is the visual source of truth. The workbench is quiet, dense and operational. `ui-ux-pro-max` guidance is limited to accessibility, keyboard behavior, responsive density, stable interaction targets and form feedback. The generated exaggerated-minimalist purple marketing direction is rejected. `design-taste-frontend` is not applicable to this Admin surface.

The Product Design brief is the existing platform Admin system plus the frozen organization/RBAC contract. Saved Product Design context is unrelated `zshenmez` material and is intentionally not reused.

## Browser Acceptance

This node records focused role-workbench evidence at `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720` and `1440x1024` for strict two-level rendering, selection, zero-impact and conflicting role changes, permission search/half-selection/save/reload, controlled menu entry, keyboard flow, focus return, reduced motion and no horizontal page overflow. Full organization/menu migration equivalence remains owned by `organization-rbac-menu-e2e-qa`.

## Boundary

Owned here: role tree UI, role/group metadata maintenance needed by the tree, role move/disable remediation, atomic role policy assignment, shared workbench and Tree Transfer components, i18n, generated service-object contracts, focused browser evidence and governance closeout.

Not owned here: directory/page menu schema, page-button authoring, `role_menu` persistence, legacy menu dual-read cutover, full organization E2E, datasource routing, federation, XA, Outbox/MQ, search or publication.
