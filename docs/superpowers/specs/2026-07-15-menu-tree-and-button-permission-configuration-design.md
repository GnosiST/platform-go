# Menu Tree And Button Permission Configuration Design

> Status: approved contract baseline; implementation in progress on 2026-07-15.

## Goal

Add a production-shaped menu governance workbench that distinguishes non-navigating directories from routable page leaves, manages page-button permission metadata, and implements independent page-only role-menu assignment behind the frozen migration cutover gate without weakening API authorization.

## Chosen Architecture

The existing `organizationrbac` GORM repository and Domain Command executor remain the single authorization mutation boundary. Menu records stay in the normalized Admin `menus` resource, while the new `platform_admin_role_menus`, `platform_admin_role_menu_revisions` and `platform_admin_page_buttons` tables hold independent relations and revisions. Menu and permission ownership is explicitly injected into the Admin resource composition root so generic CRUD cannot bypass menu validation, relation revisions, lifecycle references or audit. This avoids a second policy framework and keeps role-menu writes in the same reviewed preview/impact/apply pattern already used by role permissions.

Rejected alternatives are a new navigation authorization package, which would duplicate composition and audit infrastructure, and generic `ValuesJSON` relations, which would violate the frozen native transactional repository contract.

## Menu Model

- Every menu has `nodeType=directory|page`, a stable globally unique code, normalized `parentCode`, localized title and description, status, icon and sort order.
- A directory has no route, component key, resource code, external URL or page buttons. It expands or collapses only and is visible when at least one effective descendant page is visible.
- A page is always a leaf. An internal page has a normalized absolute route and a registered component/resource key. An external page has an HTTPS URL and `openMode=same-tab|new-tab`.
- Page parameters are a bounded JSON array of typed static `{key,type,value}` entries where type is `string|number|boolean`. Scripts, expressions, SQL, route templates and physical routing inputs are rejected.
- Advanced page metadata includes cache, hidden, active-menu and breadcrumb flags. Legacy `menus.permission` remains readable for migration comparison but is read-only in target authoring.
- Parent changes reject cycles, self-parenting, page parents and converting a directory with children into a page.

## Page Buttons

Page buttons are edited inside the selected page detail, not as a separate top-level Admin resource. Each row has a stable `buttonKey`, localized label, action, sort order, status and exactly one `page-button` permission code whose metadata points back to the same menu and button. `platform.navigation.menu-definition.get` returns the complete menu/button definition and revision, while `platform.navigation.menu-definition.replace` creates, updates or removes the menu, buttons and managed page-button permission relations in one native transaction. The Admin must not compose this operation from generic menu and permission CRUD calls. Button visibility never authorizes an API; the UI shows a coherence warning when the role lacks useful API permissions but never grants or hides access implicitly.

## Role Menu Assignment

`role_menu` stores enabled page leaves only. Directory selection in `PlatformTreeTransfer` is a bulk shortcut over currently eligible descendants, and adding a future page never grants it implicitly. A persisted `platform.navigation.role-menus.get` query returns the current page-leaf set and per-role revision needed to initialize the editor. The prepare command stores the complete normalized target set, expected role-menu revision and impact hash; apply revalidates role state, page state and the reviewed set before one atomic diff. No client chunking is allowed.

The existing role workbench menu entry is wired to the target mutation boundary but remains unavailable while the migration cutover gate is closed. It becomes functional only after `organization-rbac-menu-e2e-qa` proves principal-level equivalence, freezes legacy writes, completes the bounded target observation and explicitly enables new role-menu writes; the principal must also have role-update plus menu-read permissions. The default serving mode remains legacy. Explicit compare and dual-read seams may calculate candidate diffs in this node, but target serving and writes reject while their independent cutover gates are closed.

## Runtime Navigation

Legacy mode preserves permission-derived pages. Target resolution computes the union of enabled page bindings from enabled roles, derives enabled directory ancestors, and returns directory/page metadata without consulting page-button permissions. Serving mode and write mode are independently gated: legacy is the only enabled serving mode until the E2E cutover completes, while compare/dual-read can record value-free candidate differences without returning the target set. API access continues through Casbin and allow/deny permissions. Cache keys include menu serving mode and relation revision, and menu mutations invalidate the Admin menu cache.

## Admin Experience

`/menus` renders a dedicated dense `MenuGovernanceConsole` using `AdminTreeWorkbench`. The left tree supports search, expand/collapse and stable selection; the right pane shows metadata, route settings, typed parameters and page buttons. Creation uses explicit directory/page commands. Destructive or structural changes require confirmation and explain conflicts instead of silently moving children or deleting bindings.

The role menu assignment reuses `PlatformTreeTransfer`. Desktop is two-pane; mobile uses Available/Selected tabs. Keyboard tree behavior, focus restoration, polite count announcements, disabled reasons, reduced motion, 44px targets and no horizontal page overflow remain hard gates. The existing Ant Design tokens and quiet operational density are authoritative; the generated marketing palette and typography are rejected, and `design-taste-frontend` remains deferred to public surfaces.

## Migration Boundary

This node adds target columns/tables, deterministic candidate backfill helpers, the migration compare query, separate role-menu revisions/audit and disabled-by-default serving/write seams. It does not enable target serving or new role-menu writes and does not claim final principal-level legacy/candidate equivalence, the write-freeze cutover window, bounded target observation, rollback rehearsal or 10,000-node certification. Those remain mandatory completion gates for `organization-rbac-menu-e2e-qa`.

## Verification

Public seams are menu validation and native repository methods, ownership-aware Admin writes, Domain Query/Command execution, `/api/admin/menus`, generated Admin service-object contracts and the menu/role Admin workflows. Tests must prove directory/page invariants, page-button permission integrity, page-only role-menu storage, no-op revision behavior, stale preview rejection, closed write/target-serving gates, legacy default compatibility, candidate comparison, target ancestor derivation, minimum permissions, async search race protection and focused menu-workbench keyboard/focus behavior across the frozen six responsive viewports.

## Boundary

Owned here: menu schema/runtime metadata, ownership and lifecycle integration, menu workbench, native page buttons, native role-menu assignment implementation behind a closed gate, deterministic candidate backfill and compare service objects, disabled target navigation seam, i18n, generated contracts, focused menu-workbench browser evidence and governance closeout.

Not owned here: enabling target serving or role-menu writes, final migration cutover certification, all-principal dual-read equivalence, bounded observation, rollback rehearsal, 10,000-node/2,000-selected Tree Transfer certification, organization-wide E2E, datasource routing, federation, XA, Outbox/MQ, search projection or publication.
