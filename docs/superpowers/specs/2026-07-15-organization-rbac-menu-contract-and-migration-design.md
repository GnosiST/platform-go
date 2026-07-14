# Organization RBAC And Menu Contract And Migration Design

> **Status:** Implemented as a contract gate. Runtime implementation is intentionally deferred.

## Goal

Resolve the confirmed conflict between the current direct user-tenant model, nested role-group catalog and permission-derived menus, and the target organization role-pool model before changing persistence or Admin UI behavior.

## Product Outcome

Platform administrators can reason about who may receive which role and which pages they can navigate before applying a high-risk change. Tenant ownership is derived instead of manually duplicated, invalid authorization is never silently retained or removed, and menu visibility is separated from API and page-button permission enforcement.

## Chosen Direction

The implementation contract is `docs/platform-organization-rbac-menu-contract.md`; the machine gate is `resources/platform-organization-rbac-menu-contract.json`.

The design chooses:

- direct organization-to-role-group bindings with no implicit ancestor inheritance;
- strict non-nested role groups and exactly one group per role;
- explicit tenant and platform user scopes;
- server-derived tenant ownership for tenant users;
- persisted impact previews and reviewed remediation plans for conflicting changes;
- lifecycle-safe delete, disabled-state restore and reference-restricted purge;
- a versioned migration manifest for nested groups, orphan roles, tenant users and explicit platform principals;
- page-only role-menu persistence with derived directory ancestors;
- separate role-menu and role-permission commands, revisions and audit actions;
- numeric SemVer service-object versions and a dedicated domain command executor for atomic relation mutations;
- a dedicated tree workbench and virtualized Tree Transfer rather than overloading ordinary relation selectors.

## Rejected Alternatives

- **Role-to-role-group many-to-many:** rejected because the approved management model is a strict two-level tree and one owner makes move, order and disable semantics deterministic.
- **Nested role groups:** rejected because group ancestry would become an implicit authorization dimension and complicate organization role-pool impact.
- **Tenant selection on the user form:** rejected because it can diverge from organization ownership.
- **Treating a missing organization as a platform user:** rejected because accidental omissions must not create platform scope.
- **Silent cleanup on organization or group change:** rejected because it destroys authorization state without an explicit decision or audit trail.
- **Keeping `menu.permission` as the visibility owner:** rejected because menu assignment and API authorization are separate administrator intents.
- **Persisting directory grants:** rejected because assigning a directory could unintentionally expose future descendant pages.
- **Using the existing tree select as Tree Transfer:** rejected because static relation loading has no server search, selected hydration, impact revision or large-data selection contract.
- **Changing to tenant-local duplicate codes now:** rejected because all current generic and normalized relations use stable codes; it requires a separate identifier migration.
- **Client-side chunking of relationship replacement:** rejected because partial success would violate one authorization decision and its audit/revision boundary.
- **Restoring soft-deleted authorization directly to enabled:** rejected because it could silently resurrect grants that are no longer valid.

## UX System Notes

The existing Refine/React/Ant Design shell, platform wrappers, density rules and responsive breakpoints remain authoritative. This is a quiet, task-focused governance workflow, so marketing art direction is not applicable.

The implementation must preserve visible labels, inline recovery guidance, keyboard tree semantics, focus return, 44px interaction targets, screen-reader count announcements, reduced-motion support, virtual rendering for 50 or more visible nodes and a single-pane mobile Transfer below 768px.

The intended browser scenarios and viewports are frozen in the machine contract. This node does not claim those scenarios have passed; the final browser evidence belongs to `organization-rbac-menu-e2e-qa`.

## Migration Safety

The current principal menu algorithm aggregates deny rules across roles. Therefore, per-role menu backfill is only a candidate. Cutover is blocked until every active principal has zero unapproved old/new menu differences.

Role/menu/permission writes remain frozen during a bounded cutover observation window. Once new role-menu writes are enabled, returning to the legacy permission-derived model requires the reviewed database checkpoint; a feature flag alone cannot reconstruct divergent authorization state.

New authorization relations require native repository transactions, revisions and audit. The existing GORM snapshot save deletes and recreates normalized tables and is not the target write path for these relations.

The identity/RBAC migration is also explicit: nested groups flatten without changing group identity or role ownership, orphan roles and users without organizations block until a versioned manifest resolves them, and platform principals come only from a reviewed allowlist. Organization-group candidates are the minimal distinct owner groups of existing roles held by users mapped to that organization, but the manifest must explicitly approve each binding and every newly assignable-role expansion. Delete follows the same impact gate as disable; restore returns a lifecycle-managed entity disabled; purge waits for retention and reference gates. Authorization relation rows are changed by explicit atomic diff and are not independently restored.

The preceding generic Command runtime remains limited to scalar arguments, one-resource insert/update and 1,000 affected rows. The backend migration node adds a versioned domain handler registry, set/remediation argument types and an independent `DomainCommandPlan`. An idempotent domain prepare command accepts the set/remediation input and persists the normalized full change set; the generic impact query remains read-only and loads it by preview id; apply carries its id, revision, hash and idempotency key. A 2,000-leaf UI selection is applied as one server-side diff and transaction, never as independently committed client chunks.

## Boundary

This node delivers design docs, a machine contract, a validator, mutation tests, task-graph activation and closeout evidence only. It does not claim schema migration, backend enforcement, Admin UI, browser QA, datasource routing, federation, XA, Outbox/MQ or search implementation.
