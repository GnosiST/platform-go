# Menu Task 2 Report

## Status

Task 2 complete: native menu validation, menu/page-button/permission transactions, role-menu relations and revisions, Admin ownership wiring, lifecycle references, and SQLite projection reload are implemented. Service objects, HTTP serving modes, cutover gates, and frontend work remain outside this task.

## TDD Evidence

RED was observed before implementation:

- Repository tests failed to compile for missing native menu tables, `ValidateMenuSnapshot`, role-menu APIs, and the ownership writer.
- Atomic menu-definition tests failed for missing `ReplaceMenuDefinition` and rollback behavior.
- Lifecycle tests failed because menus and permissions were not valid lifecycle resources and deep parent restoration loaded only one ancestor.
- No-op tests exposed a persisted boolean default mismatch before the explicit cache-column write was added.

GREEN evidence:

- `rtk go test ./internal/platform/organizationrbac`: 94 passed.
- `rtk go test ./internal/platform/adminresource ./internal/platform/bootstrap`: 314 passed.
- `rtk go test ./...`: 1674 passed across 34 packages.
- `rtk git diff --check`: passed.
- `rtk codegraph sync .` and `rtk codegraph status`: index up to date.

## Implemented Files

- `internal/platform/organizationrbac/menu_repository.go`
- `internal/platform/organizationrbac/role_menu_repository.go`
- `internal/platform/organizationrbac/menu_repository_test.go`
- `internal/platform/organizationrbac/gorm_repository.go`
- `internal/platform/organizationrbac/lifecycle_mutations.go`
- `internal/platform/adminresource/gorm_store.go`
- `internal/platform/adminresource/menu.go`
- `internal/platform/bootstrap/admin_resources.go`
- `internal/platform/bootstrap/admin_resources_test.go`

## Behavior Closed

- Additive native schema and Open checks for normalized menus, page buttons, role menus, and per-role revisions.
- Directory/page validation, cycle rejection, fixed-route and static-parameter restrictions.
- Atomic menu definition replacement that derives page-button permissions in the same transaction, preserves API permissions, rolls back fully on failure, and does not bump revision on no-op.
- Stable menu ID/code identity guards and immutable legacy permission handling.
- Page-only role-menu preview/load/replace with independent per-role CAS, incremental diffs, and no-op revision behavior.
- Admin target ownership writer and bootstrap injection without snapshot delete/recreate of native relations.
- Target menu governance schema activation in bootstrap.
- Menu/permission delete, restore, and purge reference checks, including role-menu, page-button, child, active-menu, and deep ancestor closure.
- Legacy `MenuItem` and SQLite `Record` projections remain readable while exposing normalized target metadata.

## Commit

This report is included in `feat: add native menu governance repository`.

## Self Review

- Scope is limited to Task 2 repository, ownership, projection, bootstrap, and lifecycle boundaries.
- Role-menu audit/idempotency and write/serving gates are intentionally left for Task 3.
- Legacy unresolved parent labels remain readable during migration only when backed by the frozen legacy permission projection; target authoring uses normalized `parentCode`.
- No destructive commands or unrelated file changes were used.
