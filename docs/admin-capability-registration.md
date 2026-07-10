# Admin Capability Registration

Date: 2026-07-04

## Purpose

Admin resources and menus are now registered through enabled capability manifests.

This is the first code-level step toward plugin-like platform capabilities: a capability can declare its admin surface, and disabled capabilities do not register admin resources or menus.

## Manifest Contract

Each `capability.Manifest` can expose an admin surface:

```go
Admin: capability.AdminSurface{
    Resources: []capability.AdminResource{
        {
            Resource:         "feedback-tickets",
            Title:            capability.Text("反馈工单", "Feedback Tickets"),
            Description:      capability.Text("用户反馈与处理记录。", "User feedback and handling records."),
            PermissionPrefix: "admin:feedback-ticket",
            Menu: capability.AdminMenu{
                Route:    "/feedback-tickets",
                Parent:   "support/workbench",
                Group:    "operations",
                Icon:     "audit",
                Order:    250,
                External: false,
                Cache:    true,
            },
        },
    },
}
```

It can also expose lifecycle declarations:

```go
Migrations: []capability.Migration{
    {
        ID:          "20260704_create_feedback_tables",
        Description: "Create feedback ticket tables.",
        Up:          migrateFeedbackTables,
    },
},
Seeds: []capability.Seed{
    {
        ID:          "feedback-default-dictionaries",
        Description: "Seed feedback dictionary values.",
        Run:         seedFeedbackDictionaries,
    },
},
```

The Store turns each enabled resource declaration into:

- a generic admin resource schema;
- action permission codes from `PermissionPrefix`;
- permission catalog records for `read`, `create`, `update`, and `delete`;
- a menu record using `Menu`;
- seed rows for known core resources.

If a capability is not enabled or not passed to the Store, its admin resources are not registered.

The enabled manifest surface can also be exported for generators and review:

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
```

The output is generated from `capability.Manifest.Admin.Resources`. It is the plugin-side companion to the platform base manifest, not a replacement for `resources/admin-resources.json`. The main admin resource generator merges capability resources that are not already declared by the platform base, so enabled plugins become visible to OpenAPI/codegen/scaffold previews while core resources keep their curated platform definitions. Keep shared system resources such as tenants, org units, role groups, roles, users and area codes in the platform base; keep business resources in the optional capability manifest that owns them.

## Menu Metadata

`capability.AdminMenu` is the platform menu contract:

- `Route`: route path. Internal routes start with `/`; external routes may use a full URL when `External` is true.
- `Parent`: optional slash-separated parent path. The admin shell uses it to build second-level and deeper navigation.
- `Group`: top-level shell group, such as `foundation`, `governance`, `operations` or `security`.
- `Icon`: semantic icon key consumed by the admin shell.
- `Order`: ordering hint inside a group or parent branch.
- `External`: when true, the frontend opens the route in a new browser tab and does not create a work tab.
- `Cache`: frontend route/component cache hint. Internal pages default to cacheable; external links should normally disable it.

Do not encode role names, tenant IDs or user-specific state in menu declarations. Visibility is still derived from permission codes and the current principal.

## Development Rule

New common or business capabilities should not edit core menu registration directly.

Use this order:

1. Add or update the capability manifest.
2. Declare admin resources and menu metadata in `AdminSurface`.
3. Add field-level schema only when the default resource schema is not enough.
4. Add tests proving the enabled capability registers its resources and the disabled capability does not.
5. Add custom frontend pages only when the generic resource engine cannot express the workflow.

## Contract Validation

Enabled capability manifests are validated during dependency resolution. Startup fails fast when an enabled capability declares an invalid admin surface.

The validator rejects:

- missing resource key, localized title, localized description or permission prefix;
- malformed permission prefix. `PermissionPrefix` must use `admin:<resource>` with lowercase letters, numbers or hyphens after `admin:`;
- custom action, panel and runtime slot permissions must stay under the declaring resource prefix, such as `admin:feedback-ticket:approve`;
- unsupported form layout presets. `FormLayout` may be empty or one of `single-column`, `grouped-sections`, `two-column-density` and `side-detail-preview`;
- menu definitions without route, group or icon;
- internal menu routes that do not start with `/`;
- external menu routes that are not `http(s)` URLs;
- duplicate resource keys across enabled capabilities;
- duplicate menu routes across enabled capabilities;
- duplicate permission prefixes across enabled capabilities;
- duplicate custom field keys on one resource;
- invalid form group metadata. Form groups require a key, localized label, localized description when present and unique keys;
- field `Group` values that reference undeclared form groups when `FormGroups` is declared;
- custom field definitions without key, localized label, valid source or valid type;
- field help text without both `zh` and `en` when help text is present;
- supported custom field types are `text`, `textarea`, `select`, `multiselect`, `datetime`, `switch`, `number` and `color`;
- select or multiselect fields without either static `options` or a dynamic `relation`;
- static field options without a value or localized label;
- invalid relation metadata. Relations must declare target `resource`, `valueField` and `labelField`; multiple relations must use `multiselect`; relation filters may only use `contains`, `=`, `!=`, `>`, `>=`, `<` or `<=`;
- `SearchFields` or `DefaultSortKey` values that reference fields not declared by the resource schema or standard record fields;
- migration declarations without ID, description or `Up`;
- seed declarations without ID, description or `Run`;
- duplicate migration IDs across enabled capabilities;
- duplicate seed IDs across enabled capabilities.

Only enabled capabilities are validated. A disabled capability does not register its admin resources and does not block startup.

## Lifecycle Order

Enabled capabilities are resolved in dependency order. The registry runs lifecycle phases in this order:

```text
Configure -> Migrations -> Seed -> RegisterServices -> RegisterRoutes -> RegisterAdmin -> Start
```

Structured `Migrations` run during the migration phase before the legacy `Hooks.Migrate`. Structured `Seeds` run during the seed phase before the legacy `Hooks.Seed`.

Migration or seed failures are wrapped with the capability ID and declaration ID, so startup logs can identify the broken capability contract.

## Runtime Executors

`capability.Runtime` is the infrastructure boundary for lifecycle execution.

The zero-value runtime executes migration and seed functions directly. Production runtimes can inject:

```go
capability.Runtime{
    MigrationExecutor: myMigrationExecutor,
    SeedExecutor:      mySeedExecutor,
}
```

Executors receive:

- capability ID;
- migration or seed ID;
- description;
- the original lifecycle step.

This keeps capability packages declarative. Capability packages declare what must run; Runtime executors decide how to record migration state, enforce idempotency, attach transactions, write audit logs or emit startup telemetry.

## Lifecycle History Port

The capability package now defines a persistence port:

```go
type LifecycleHistory interface {
    HasMigration(ctx context.Context, capabilityID capability.ID, migrationID string) bool
    RecordMigration(ctx context.Context, record capability.LifecycleRecord) error
    HasSeed(ctx context.Context, capabilityID capability.ID, seedID string) bool
    RecordSeed(ctx context.Context, record capability.LifecycleRecord) error
}
```

`NewRecordedLifecycleExecutor(history)` uses this port to make migration and seed execution idempotent:

- if a migration is already recorded, it is skipped;
- if a seed is already recorded, it is skipped;
- successful steps are recorded after execution;
- failed steps are not recorded.

`NewMemoryLifecycleHistory()` is the in-memory implementation for tests. `NewFileLifecycleHistory(path)` persists records to a JSON file and is useful for local development or single-node validation. `NewGORMLifecycleHistory(ctx, db)` is the target durable database implementation.

The zero-value runtime does not create a history file. File or database history must be explicitly injected through `NewRecordedLifecycleExecutor(history)`.

The GORM implementation creates and uses a `platform_lifecycle_history` table with `(kind, capability_id, step_id)` as the primary key. It can later add durable timestamps, checksums, actor metadata and audit references without changing capability manifests.

`NewSQLLifecycleHistory(ctx, db)` remains as a compatibility adapter behind the same port and is covered by tests, but startup should use the GORM runtime path.

## Startup Integration

`platform-api` now resolves enabled capabilities, builds `capability.Runtime`, runs capability lifecycle, and then starts HTTP.

By default, lifecycle steps execute directly and do not create local state files. To enable local file-backed lifecycle history:

```bash
PLATFORM_LIFECYCLE_HISTORY_FILE=.platform/lifecycle-history.json
```

When the variable is set, startup uses `NewFileLifecycleHistory(path)` with `NewRecordedLifecycleExecutor(history)`, so migration and seed declarations are skipped after they have been recorded once.

To use a GORM lifecycle history backend:

```bash
PLATFORM_LIFECYCLE_HISTORY_DRIVER=mysql
PLATFORM_LIFECYCLE_HISTORY_DSN=user:pass@tcp(localhost:3306)/platform
```

Runtime selection rules:

- if `PLATFORM_LIFECYCLE_HISTORY_DRIVER` is set, GORM history is used and `PLATFORM_LIFECYCLE_HISTORY_DSN` is required;
- if driver is empty and `PLATFORM_LIFECYCLE_HISTORY_FILE` is set, file history is used;
- if both are empty, the zero-value runtime executes lifecycle steps directly without persisted history.

Core capabilities currently declare no-op migration and seed boundaries. These declarations keep startup lifecycle, history recording and idempotency exercised by the default platform runtime. They are not database DDL yet. Database-backed migrations will replace the no-op bodies while preserving the manifest IDs and execution contract.

## Admin Resource Persistence

Admin resources persist through the `AdminResourceRepository` snapshot port. The Store owns generic resource behavior, while repositories own loading and saving records.

`PLATFORM_ADMIN_RESOURCE_FILE` enables a JSON file-backed admin resource repository. The file persists records and generated IDs, while schemas continue to come from enabled capability manifests. This gives local deployments durable configuration without coupling capability packages to a specific database.

`PLATFORM_ADMIN_RESOURCE_DRIVER` and `PLATFORM_ADMIN_RESOURCE_DSN` enable the GORM-backed admin resource repository for `mysql`, `postgres` and `sqlite`. Startup chooses GORM driver first, then file, then memory.

The GORM-backed repository persists generic business resources through the snapshot contract and stores standard platform resources in normalized tables for tenants, org units, users, user-role bindings, role groups, roles, role-permission bindings, permissions, menus, area codes, audit logs, login logs, error logs and versions. This keeps RBAC, Casbin policy generation, menu filtering, organization metadata, regional metadata and operations logs auditable without changing capability manifests or the generic admin HTTP contract.

## Current Boundary

The current implementation uses an in-memory Store by default, an optional file-backed repository for local persistence, and optional GORM-backed admin resource, session and lifecycle repositories. Core capabilities have lifecycle boundary declarations. Standard platform resources such as tenants, org units, users, role groups, roles, permissions, menus and area codes are normalized in the GORM admin resource adapter; concrete business DDL and durable business seed writes remain later slices. The manifest contract, execution order, startup integration, Runtime executor boundary, lifecycle history port, local file-backed history adapter, GORM lifecycle-history adapter, compatibility SQL adapter and admin resource repository port are now in place.
