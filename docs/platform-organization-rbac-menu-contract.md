# Organization, RBAC And Menu Contract

> Status: design frozen; runtime and Admin UI implementation remain pending in the five downstream organization/RBAC/menu nodes.

This document is the implementation contract for organization role pools, tenant derivation, role ownership, menu visibility, permission boundaries and migration. The machine-readable source is `resources/platform-organization-rbac-menu-contract.json`.

## Decisions

- `org-units` remains the only default organization resource. Any enabled org unit may bind role groups directly; bindings do not inherit from parent org units in v1.
- A role group is either `platform` scoped or owned by one tenant. Role groups are not nested.
- Every role belongs to exactly one role group. Role scope and tenant ownership derive from that group.
- A tenant organization binds multiple enabled tenant role groups from the same tenant. Platform role groups cannot be bound to organizations.
- The effective organization role pool is the distinct set of enabled roles in enabled directly bound groups.
- A tenant user has exactly one primary organization. The server derives and redundantly persists `tenantCode`; Admin forms display it read-only and clients cannot change it directly.
- A platform user is an explicit scope, not the accidental result of a missing organization. Platform users have no persisted organization or tenant and may receive only platform roles. Their trusted runtime context may use the reserved platform scope.
- User roles must remain a subset of the effective pool for every create, update, bulk, import, service-command, delete, restore and purge path.
- Role groups classify roles. Roles continue to own allow permissions, deny permissions and data scope.
- Menus own navigation only. `role_menu` owns role-to-page visibility, page-button permissions own UI action visibility, and API permissions remain Casbin's authoritative backend boundary.
- Authorization decisions stay on one tenant and one datasource. Federation and XA are forbidden in this path.

Stable role, role-group, organization and menu codes remain globally unique during this migration. The existing generic resource layer, normalized projections and public contracts resolve relations by code; changing to tenant-local duplicate codes would require a separate identifier-contract migration and is not smuggled into this node.

## Relational Target

The backend migration node must introduce native, transactionally managed relations instead of placing the new authorization state only in `ValuesJSON` or the current full-snapshot delete-and-rebuild path.

| Logical model | Physical target | Required behavior |
| --- | --- | --- |
| role groups | `platform_admin_role_groups` | add `scope_type`, `tenant_code`, `revision`; remove target use of `parent_code`; tenant required exactly for tenant groups |
| roles | `platform_admin_roles` | normalize required `group_code`; derive scope and tenant from the group |
| organization role groups | `platform_admin_org_unit_role_groups` | unique organization/group binding; same enabled tenant; actor, revision and timestamps |
| users | `platform_admin_users` | normalize user scope, primary organization and derived tenant invariant |
| user roles | `platform_admin_user_roles` | validate every row against the tenant organization pool or platform role pool |
| menus | `platform_admin_menus` | add `node_type` and normalized parent; preserve legacy `permission` read-only during migration |
| role menus | `platform_admin_role_menus` | unique role/page relation with an independent revision and audit stream |
| page buttons | `platform_admin_page_buttons` | stable page/button key, localized display metadata and one page-button permission code |
| permissions | `platform_admin_permissions` | classify `api` and `page-button`; retain API enforcement metadata |

The file, memory and legacy snapshot repositories may remain local/test compatibility adapters. They cannot claim production organization authorization until they implement the same revisions, constraints, atomic mutations and impact queries.

## Organization And User Rules

The organization role pool contains only roles whose role group, role and direct organization binding are all enabled. Parent organization bindings do not flow to children. This avoids implicit grants and makes impact analysis deterministic.

Organization and user forms use dedicated governance behavior on top of platform wrappers:

- Organization lists show bound-group and effective-role counts. Details show the group source for every role.
- Tenant is selectable when an organization is created and becomes read-only once referenced by users or bindings. Cross-tenant moves are a migration operation, not a normal form edit.
- Tenant users choose an organization before roles. Until then, tenant is empty/read-only and role selection is disabled.
- Role options show role name, code and the one owning role group. The strict single-group model has no multi-source role ambiguity.
- Changing a user's organization keeps current values visible while impact is calculated. Invalid roles are never silently retained or removed.
- Platform users have an explicit scope control, no organization field, no tenant field and only platform-role options.

## Conflict Protocol

Organization change, role-group unbinding, role movement, role disablement and role-group disablement are specialized commands, not generic CRUD updates.

Every high-risk change begins with an idempotent domain prepare command. It accepts the typed target set and remediation plan, authorizes them, computes the full change set and stores an owner-scoped short-lived preview. A read-only persisted impact query then loads that preview by scalar `previewId` and returns its `expectedRevision`, `impactHash`, severity, counts, conflicts, allowed remediations and expiry. Apply sends the same preview, revision, hash and an idempotency key. Changed or expired state returns `409` and requires a new prepare operation.

The default is `reject`. A migration applies only an explicit per-user removal or replacement plan. A request never accepts `force=true`, a client-supplied tenant, a datasource or an implicit "remove everything invalid" instruction.

Emergency role disable is a separate high-permission incident command. It may immediately make a compromised role ineffective, but it records the resulting assignments as unresolved conflicts and blocks re-enable until remediation completes. This exception only removes access; it never creates or restores authorization.

Audit events record actor type, trusted tenant/organization context, before and after revision, request/trace id, conflict count and a change-set hash. They do not store user PII, role display names, complete record snapshots or arbitrary dependency errors. Identity binding, role-menu assignment and role-permission assignment use separate audit actions.

## Authorization Lifecycle

An authorization entity is effective only while it is not logically deleted and is enabled. Lifecycle-managed entities are organizations, role groups, roles, users, menus and permissions. The same impact preview and remediation protocol applies to logical delete as to disable; generic lifecycle handlers and the scheduled retention runner cannot bypass the domain validator or native authorization transaction.

Restore does not reactivate access. It must restore into a disabled state, revalidate tenant ownership, organization role pools and dependent relations, then require a separate reviewed enable command. Purge is rejected until retention has expired and no live or unresolved authorization references remain.

Organization-group bindings, user-role bindings, role-menu bindings and page-button rows are dependent authorization relations, not independently restorable lifecycle entities. They participate in impact previews, purge reference blocking and audit, and change only through an explicit server-side atomic diff. Entity delete, restore and purge never silently cascade these relations or resurrect prior grants.

## Service Objects

High-risk reads use the persisted Query runtime completed by the preceding node. IDs and versions are stable registry entries and every definition version is a numeric SemVer string; clients submit typed arguments, never fields, operators, SQL, tenant routing or physical schema identifiers.

Read and impact queries:

- `platform.identity.organization-role-pool.get@1.0.0`
- `platform.identity.organization-role-group-change.impact@1.0.0`
- `platform.identity.user-organization-change.impact@1.0.0`
- `platform.identity.role-state-or-group-change.impact@1.0.0`
- `platform.authorization.resource-lifecycle.impact@1.0.0`
- `platform.navigation.role-menu-change.impact@1.0.0`
- `platform.authorization.role-permission-change.impact@1.0.0`
- `platform.navigation.role-menu-migration.compare@1.0.0`

Prepare commands:

- `platform.identity.organization-role-group-change.prepare@1.0.0`
- `platform.identity.user-organization-change.prepare@1.0.0`
- `platform.identity.role-state-or-group-change.prepare@1.0.0`
- `platform.authorization.resource-lifecycle.prepare@1.0.0`
- `platform.navigation.role-menu-change.prepare@1.0.0`
- `platform.authorization.role-permission-change.prepare@1.0.0`

Apply commands:

- `platform.identity.organization-role-groups.replace@1.0.0`
- `platform.identity.user-organization.change@1.0.0`
- `platform.identity.role.move@1.0.0`
- `platform.identity.role.disable@1.0.0`
- `platform.authorization.resource-lifecycle.apply@1.0.0`
- `platform.navigation.role-menus.replace@1.0.0`
- `platform.authorization.role-permissions.replace@1.0.0`

The existing generic Command AST supports scalar arguments, one-resource insert/update and at most 1,000 affected rows. It cannot implement atomic relationship replacement, role movement, disablement or lifecycle changes. The backend node therefore adds a domain command executor built from a versioned domain handler registry, typed `string-set` and remediation-list arguments, and a `DomainCommandPlan` independent of the generic `CommandAST`. Domain handlers retain the shared authentication, authorization, trusted tenant/data-scope, cost, timeout, idempotency and audit guards.

The idempotent domain prepare command receives the `string-set` and typed remediation-list arguments and stores the normalized full change set server-side with owner scope, expiry, revision and impact hash. The generic QueryExecutor remains read-only; its impact query accepts only a scalar `previewId` and displays the stored result. Apply sends only `previewId`, `expectedRevision`, `impactHash` and `idempotencyKey`; the handler reloads the reviewed set, recomputes its guards and applies the complete diff in one native transaction. The UI may hydrate 2,000 selected leaves, but it must not split one authorization decision into client-side chunks that can partially succeed. The generic runtime's 1,000-row limit remains unchanged.

All write paths call one shared domain validator. Generic Admin CRUD remains suitable for low-risk metadata but cannot bypass these commands for organization bindings, user organization/roles, role moves/status, lifecycle operations, menu assignments or permission assignments.

## Menu And Permission Model

Directory menus are containers. They have no route or component, do not navigate and are visible only when at least one assigned descendant page is visible. Directories may nest.

Page menus are leaves. An internal page has a registered route and component/resource key. An external page has a validated external URL and open mode. Parameters are typed static key/value data; React paths, scripts, expressions, SQL and arbitrary executable configuration are forbidden.

`role_menu` persists page leaves only. The backend derives directory ancestors. Selecting a directory in the UI is a bulk selection shortcut over eligible descendant pages, not a durable grant that would automatically expose pages added later.

Effective menus are the union of enabled page bindings from enabled user roles. Effective API and button permissions continue to use role allow/deny semantics, with deny taking precedence. A page-button permission never authorizes its API. A consistency linter may warn that an assigned page lacks useful API permissions, but it must not silently grant permissions or hide the page.

## Identity And RBAC Migration

The backend migration requires a versioned migration manifest. It maps every legacy role group to an explicit scope and tenant, every ungrouped role to exactly one existing or new group, every tenant user to one primary organization, each organization's approved role-group bindings, every approved platform principal to an explicit reviewed allowlist, and every role-pool conflict to an explicit remediation.

Nested legacy role groups flatten into top-level groups while retaining stable group identity and existing role membership; the migration does not guess a new role owner. An orphan role blocks cutover until mapped. A user without an organization blocks cutover unless assigned one organization or explicitly approved as a platform principal. Tenant mismatches and roles outside the derived organization pool also block rather than being silently rewritten.

Because the source model has no organization-to-role-group relation, the tool derives only a review candidate: for each organization, take the distinct owner groups of the existing roles held by users explicitly mapped to that organization. No candidate binding is applied without its exact versioned manifest entry. The migration records every role that would become newly assignable by binding the whole group, requires explicit approval of that expansion and rejects cross-tenant groups.

The migration adds target columns and relations without serving them, backfills from the approved manifest, compares each principal's roles, allow/deny permissions, data scope and menus, then freezes legacy authorization writes for the final diff. Cutover requires zero unresolved identity/RBAC conflicts and zero unapproved principal authorization differences. Before target writes begin, the additive schema and legacy read path remain the rollback route; afterward, rollback requires the reviewed database checkpoint.

## Legacy Menu Migration

The current runtime requires `menus.permission` and evaluates it against the principal's aggregated allow/deny policy. The target cannot replace that behavior in place.

1. Inventory and classify existing menu, permission, role and principal records. Freeze writes to the legacy menu-permission field.
2. Add target columns and relations without serving them.
3. Backfill page-level role-menu candidates from each role's allow/deny policy. Give wildcard roles all enabled pages.
4. Compare legacy and candidate effective menu sets for every active principal. This principal-level gate is required because deny rules are aggregated across roles and per-role backfill alone is not equivalent.
5. Resolve every unapproved added or removed page. Do not guess through ambiguous cross-role cases.
6. Enter dual-read mode: continue returning the legacy set and record value-free diffs.
7. Freeze role/menu/permission mutations for a bounded cutover window, switch serving to `role_menu`, run the acceptance suite and retain a read-path rollback flag.
8. After the observation window passes, enable new role-menu writes. From that point, legacy-only rollback requires restoring the reviewed database checkpoint; a config switch alone is no longer sufficient.
9. Stop reading `menus.permission`, keep it read-only for a deprecation window, then remove it in a later compatible migration.

Cutover requires zero unapproved principal differences, a verified checkpoint, a rollback runbook and audit evidence.

## Tree Workbench And Transfer

Role and menu management use a shared `AdminTreeWorkbench`; authorization selection uses a new `PlatformTreeTransfer`. `PlatformTreeSelect` remains appropriate for ordinary bounded relation fields and is not expanded into this large-data control.

The Transfer value contains assignable leaf keys plus a revision. Parent nodes are navigation and bulk-selection controls. Search is server-backed, preserves ancestor paths and defines "select all" as the current filtered result only. Lazy loading, selected-value hydration and virtual rendering activate for large sets; 50 visible nodes is the virtualization threshold.

The control supports search, full/half selection, disabled reasons, selected replay, counts, optimistic concurrency and 10,000 nodes with 2,000 selected items. Permission assignment groups API and page-button permissions and keeps grant and explicit-deny sets mutually exclusive.

Keyboard behavior follows the treeview pattern: arrows navigate and expand/collapse, Home/End move within the current tree, Space toggles selection and Enter activates the focused command. Mixed nodes expose `aria-checked="mixed"`; count changes use `aria-live="polite"`; focus returns to the trigger when the dialog closes. Drag is optional on desktop and always has a button/command alternative. Interactive targets are at least 44px.

At 1024px and above the Transfer is two-pane. At 768-1023px it uses a compact two-pane layout. Below 768px it becomes one pane with Available/Selected tabs and sticky actions. The page must not gain horizontal overflow at 200% zoom.

## Browser Acceptance Contract

Later UI and E2E nodes must exercise the following at `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720` and `1440x1024`:

- bind organization groups and inspect role-pool provenance;
- derive tenant and load roles after selecting an organization;
- preview organization-change conflicts and explicitly remediate invalid roles;
- reject tenant roles for platform users;
- cover zero-impact and conflicting group unbind, role move and disable paths;
- reject nested role groups and roles without exactly one group;
- verify directory expansion does not navigate and page nodes do;
- save, reload and audit menus independently from permissions;
- search, scroll, half-select and save the large-data fixture without a visible main-thread freeze;
- complete selection and save by keyboard, restore focus, announce counts, honor reduced motion, remain usable at 200% zoom and avoid page overflow;
- prove legacy/candidate menu equivalence and the bounded rollback path.

## Deferred Implementation

This design node does not change runtime schema, handlers, repositories, Casbin policy, Admin forms or menu serving. Those changes remain owned by:

1. `organization-role-pool-backend-and-migration`
2. `organization-user-admin-experience`
3. `role-tree-and-authorization-entry`
4. `menu-tree-and-button-permission-configuration`
5. `organization-rbac-menu-e2e-qa`

Datasource routing, federation, XA, Outbox/MQ, search projection and workload identity remain outside this lane.
