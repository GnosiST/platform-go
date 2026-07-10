# platform-go Capability Architecture Design

Date: 2026-07-04

## Purpose

`platform-go` is a reusable operations platform foundation informed by reusable management-backend patterns observed in the current `zshenmez` platform work. It should preserve common management-backend strength while making platform capabilities selectable, replaceable and extensible for future business projects. Concrete `zshenmez` business workflows are reference evidence only and must not ship inside the default foundation.

The platform is not a runtime plugin marketplace and not a low-code-only admin generator. The first version is a single deployable service with engineering-level capability packages compiled into the project and enabled by configuration.

## Current Context

The source system in `zshenmez` already proves several reusable ideas:

- Gin + GORM + Casbin backend with admin and app token separation.
- Refine + React + Ant Design management backend.
- Tenant, user, RBAC, menu, API resource, dictionary, parameter, audit, file and session governance.
- A base resource manifest in `resources/admin-resources.json`, plus capability-declared admin resources, keeping menus, permissions, API routes, Refine resources and generated previews aligned.
- Complex business admin resources for role applications, tasks, transfer chains, fulfillment, public profiles, portfolio works, favorites and support tickets.

The same source system also exposes the main extraction risks:

- `admin/src/pages/ResourceTablePage.tsx` has grown into a large page containing standard resources, business resources, drawers, modals, row actions and special workflows.
- `api/internal/store/store.go` mixes platform governance, app identity, demo data and business persistence in one broad Store.
- `api/internal/httpapi/server.go` registers `/api/app` and `/api/admin` routes in one server method.
- Seed data and framework menus include both platform system resources and `zshenmez` business menus.

The architecture must reduce those coupling points without losing reference coverage for reusable management-backend capabilities.

## Goals

- Provide a reusable platform foundation for future business projects.
- Keep first-party core governance available by default: tenant, identity, session, RBAC, menu, API resource, audit, dictionary, parameter, admin shell and system admin resources.
- Allow optional common capability packages such as WeChat login, file storage, demo seed, notifications, jobs, OpenAPI, code generation and AI, while keeping branding configuration under the parameter capability.
- Allow downstream business capability packages to register routes, permissions, menus, admin resources, custom actions and data models without editing platform core.
- Provide one internal Go calling model and one external adapter model so business code does not depend on platform internals.
- Prove the architecture through reference coverage gates: reusable platform capabilities cover common management scenarios, while business-only `zshenmez` resources remain outside `platform-go`.

## Non-goals

- No runtime hot-plug plugin installation in v1.
- No plugin marketplace.
- No automatic destructive uninstall or data cleanup when a capability is disabled.
- No broad low-code engine that tries to express every business workflow in JSON.
- No automatic source-code overwrite from generation scripts in v1.
- No microservice split in the first implementation.
- No migration of concrete `zshenmez` business entities, flows, menus, routes, stores, demo fixtures or state machines into `platform-go`.

## Architecture Summary

```text
Platform Kernel
  config / registry / lifecycle / dependency graph / unit of work / DB / HTTP / error contract

Core Governance
  tenant / identity / session / RBAC / menu / api-resource
  audit / parameter / dictionary / admin-shell / system-admin

Capability Runtime
  manifest registry / service registry / route registry
  admin resource registry / migration registry / seed registry

First-party Optional Capabilities
  wechat-login / phone-binding / file-storage / branding / demo-seed
  notification / job / openapi / codegen / AI

Application Capabilities
  downstream business packages / future business modules
```

The first implementation should stay monolithic at deployment time. Modularity is expressed through capability packages, typed interfaces and registries, not through runtime-loaded binaries or separate services.

## Layer Responsibilities

### Platform Kernel

The kernel is the smallest mandatory runtime. It owns:

- configuration loading and capability enablement;
- capability registry and dependency graph validation;
- lifecycle orchestration;
- DB connection and migration execution;
- Unit of Work transaction entrypoints;
- HTTP runtime and middleware mounting;
- common error contract;
- actor, tenant and request context propagation.

The kernel must not know `zshenmez` business concepts.

### Core Governance

Core governance ships with the platform and is enabled by default:

- tenant;
- identity and users;
- login identities;
- sessions;
- RBAC and Casbin policy refresh;
- menus;
- API resources;
- audit logs;
- dictionary and dictionary items;
- parameters and sensitive-value masking;
- admin shell;
- system admin resources.

These capabilities are treated as first-party platform capabilities, not as business modules. They form the governance surface that optional capabilities use.

### Optional Capabilities

Optional capabilities are common platform features that can be enabled per project:

| Capability | Depends on | Scope |
| --- | --- | --- |
| `wechat-login` | identity, session, audit | WeChat Mini Program login, OpenID binding, guest session creation |
| `phone-binding` | identity, audit | Phone verification and user formalization |
| `file-storage` | tenant, identity, parameter, audit | Upload, preview, download, delete, local storage, S3-compatible storage |
| `branding` | parameter, admin-shell | Product name, logo, theme, favicon, login copy |
| `demo-seed` | seed registry | Demo seed packs, reset behavior and fixture slots |
| `notification` | identity, audit | In-app notifications and future external delivery adapters |
| `job` | registry, audit | Scheduled jobs and execution logs |
| `openapi` | route registry, api-resource | OpenAPI export and API docs |
| `codegen` | resource contract | Generated previews, scaffold dry-run safety plans, generated scaffold file packages and scaffold drafts, without overwriting source by default |
| `AI` | identity, audit, parameter | Provider configuration, permissioned calls and call audit |

### Application Capabilities

Business modules live here and attach from downstream repositories or independent packages. `platform-go` may keep reference coverage records for the following `zshenmez` workflows, but it must not ship their concrete models, stores, menus, routes or state machines:

- role applications and review actions;
- task ledger, detail drawer and status update;
- transfer applications and transfer chain tracking;
- task check-ins and completion confirmations;
- public profiles, portfolio works and favorites;
- support tickets and handler notes;
- business demo seed data.

Business capabilities may depend on core and optional capabilities, but platform core must not depend on business capabilities. The `external-business-capability` name is only a reference-classification owner in the current governance records, not an executable profile bundled by the foundation.

## Capability Manifest

Each capability package provides a manifest object, not arbitrary global mutations.

```text
CapabilityManifest
  ID
  Name
  Version
  Dependencies
  Config schema
  Services
  DB models
  Migrations
  Routes
  Permissions
  Menus
  API resources
  Admin resources
  Audit actions
  Seed fixtures
  Contract tests
```

The manifest is code-backed. JSON resource manifests can describe stable contracts, but they are not the only source of behavior. Complex behavior must be represented through Go interfaces, typed services and tests.

## Lifecycle

All capabilities follow the same lifecycle:

```text
Declare -> Configure -> Migrate -> Seed -> RegisterServices -> RegisterRoutes -> RegisterAdmin -> Start
```

Rules:

- `Declare` exposes metadata and dependencies.
- `Configure` validates capability-specific configuration.
- `Migrate` registers or runs non-destructive schema changes.
- `Seed` inserts core rows, demo rows or fixture packs when enabled.
- `RegisterServices` exposes internal Go interfaces.
- `RegisterRoutes` mounts HTTP routes and route metadata.
- `RegisterAdmin` mounts admin resources, pages, menus and actions.
- `Start` starts background work such as jobs.

Capabilities must not mutate global state outside this lifecycle.

## Dependency Graph

The registry validates dependencies before runtime starts.

Examples:

- `wechat-login` requires `identity`, `session` and `audit`.
- `file-storage` requires `tenant`, `identity`, `parameter` and `audit`.
- `branding` requires `parameter` and `admin-shell`.
- `demo-seed` depends on the seed registry and any target capabilities whose fixtures it uses.
- Downstream business capabilities require core governance plus any optional platform capabilities they consume, such as `dictionary` for area ownership, `file-storage` for attachments, `wechat-login` for app identity, admin resource contracts for management pages and `demo-data` for fixtures.

Missing dependencies are startup failures. Cycles are startup failures.
Admin resource relations are also startup-validated: a declared `Relation.Resource` must be provided by the enabled manifest set, and relation value, label, filter, sort, parent and path fields must be exposed by the target resource. The current core graph keeps `dictionary` before `tenant` because `tenant.areaCode` consumes the shared `area-codes` resource.

## Unified Calling Model

The platform exposes two calling layers.

Internal business code uses Go capability interfaces:

```text
platform.Identity.Users()
platform.Auth.Sessions()
platform.RBAC.Can()
platform.Files.Store()
platform.Audit.Record()
platform.Wechat.Login()
platform.DemoSeed.Reset()
```

External clients use adapters:

```text
/admin/*
/api/admin/*
/api/auth/*
/api/files/*
/api/capabilities/*
```

All internal calls must carry:

```text
context + actor + tenant scope + permission intent + transaction
```

Business code must not directly access platform stores, platform tables or admin page internals.

## Actor And Tenant Context

Every permissioned capability call receives an execution context:

```text
ExecutionContext
  context.Context
  Actor
  TenantScope
  PermissionIntent
  UnitOfWork
```

This prevents business modules from bypassing tenant isolation, audit identity or permission intent.

The same actor and tenant context is used by:

- HTTP handlers;
- internal Go services;
- admin resource actions;
- seed or demo reset routines when they need ownership attribution;
- background jobs where the actor is a system actor.

## Unit Of Work

Cross-capability writes must share a transaction boundary.

Examples:

- WeChat login may create a user, create a login identity, create a session and record a login log.
- File upload may create a file asset, write storage metadata and record audit.
- A downstream business task-status update may update a task, write audit and enqueue notifications through public platform ports.

The platform provides Unit of Work entrypoints so capabilities can compose writes without exposing raw stores to business code.

## Admin Resource Engine

The admin backend must not regress from the current `zshenmez` management capability. The current large `ResourceTablePage` should be replaced by an admin resource engine.

```text
AdminShell
  authentication
  menu
  top bar
  tabs
  route mounting
  branding

AdminResourceEngine
  standard list
  filters
  toolbar actions
  row actions
  forms
  drawers
  modals
  detail panels
  custom pages

AdminResourceDefinition
  name
  route
  title
  description
  permission
  list
  customPage
```

`AdminResourceDefinition.list` supports:

- columns;
- filters;
- status preset buttons;
- toolbar actions;
- row actions;
- create and edit forms;
- custom drawers;
- custom modals;
- detail panels.

This is required to preserve current `zshenmez` features such as task detail drawers, role application review modals, support ticket handling modals, file preview drawers and role permission drawers.

## Resource Contract

The existing resource contract idea remains valuable. It should evolve into a platform contract that can validate:

- resource names;
- menu codes and paths;
- permission codes;
- HTTP routes;
- API resource rows;
- admin resource registrations;
- audit action declarations;
- codegen preview consistency.

The contract supports consistency checks and scaffold drafts. It must not be the only way to express behavior, and v1 generation must not overwrite source files automatically.

## Data And Migration Strategy

Core governance models and optional capability models are registered separately.

Rules:

- v1 uses non-destructive migrations only.
- Disabling a capability stops registering routes, menus, admin pages, seed and jobs.
- Disabling a capability does not drop tables or erase data.
- Explicit uninstall or cleanup tooling is future work.
- SQLite is used for tests; MySQL and PostgreSQL remain runtime targets.

The current `AllModels()` pattern should be replaced with model registration by capability.

## Seed And Demo Strategy

Seed is also capability-driven:

- core seed creates platform tenant, default roles, permissions, menus, API resources and initial admin user;
- optional capability seed creates capability-specific baseline rows;
- demo seed packs are explicit and resettable;
- business demo data lives in downstream application packages when a product chooses to provide it.

`zshenmez` demo data must not be migrated into platform core. If a future downstream `zshenmez` package needs demo fixtures, that package owns them through public platform contracts.

## Branding Strategy

Branding is a first-party optional capability:

- product name;
- short name;
- logo;
- favicon;
- primary theme;
- login page text;
- admin shell title;
- default emails or support copy when needed.

Core code can have neutral defaults, but `zshenmez` branding must be provided through configuration or a branding fixture.

## WeChat Login Strategy

WeChat login is a reusable capability, not a `zshenmez` business rule.

The reusable capability owns:

- provider configuration;
- code exchange adapter;
- OpenID or union identity binding;
- guest session creation;
- session issuance;
- login audit.

Downstream business capabilities own role-specific outcomes. For example, a product-specific capability owns whether a user becomes customer, worker or merchant and which review workflow is required.

## Error Contract

All routes should use a shared response shape:

```text
success data error
```

Errors include:

- stable code;
- user-safe message;
- HTTP status;
- optional field errors;
- optional trace ID.

Sensitive information must not leak through query strings, logs or error messages.

## Development Standards

The platform must define one code-writing and calling style before implementation starts. The goal is to make every new capability look and behave like it belongs to the same system.

### Package Layout

Backend packages use a capability-first layout:

```text
internal/platform/kernel
internal/platform/capability
internal/platform/core/tenant
internal/platform/core/identity
internal/platform/core/session
internal/platform/core/rbac
internal/platform/core/audit
internal/platform/core/menu
internal/platform/core/parameter
internal/platform/capabilities/wechatlogin
internal/platform/capabilities/filestorage
internal/platform/capabilities/branding
internal/platform/capabilities/demoseed
external application packages
```

Frontend packages use the same split:

```text
admin/src/platform/shell
admin/src/platform/api
admin/src/platform/resources
admin/src/platform/capabilities
admin/src/capabilities/fileStorage
admin/src/capabilities/branding
admin/src/apps/<business>
```

Rules:

- core packages must not import application packages;
- optional capability packages must not import application packages;
- application packages may import platform core and optional capabilities through public interfaces only;
- cross-capability imports must follow declared manifest dependencies;
- package names are lowercase and stable; capability IDs are kebab-case in manifests and config.

### Unified Internal Call Style

Every permissioned service method uses a request/response style:

```go
type Service interface {
    DoThing(exec platform.ExecutionContext, req DoThingRequest) (DoThingResult, error)
}
```

`ExecutionContext` carries:

```go
type ExecutionContext struct {
    Context          context.Context
    Actor            Actor
    TenantScope      TenantScope
    PermissionIntent PermissionIntent
    UoW              UnitOfWork
}
```

Rules:

- no service method accepts loose `userID`, `tenantID` and `db` parameters when the operation is permissioned;
- request structs contain business input only;
- result structs contain caller-facing output only;
- services return typed platform errors, not raw HTTP responses;
- services do not read HTTP headers, route params or frontend-specific field names;
- handlers adapt HTTP to service calls, then adapt service results to HTTP responses.

### Unified HTTP Handler Style

HTTP handlers are adapters. They should follow this shape:

```text
bind request
build ExecutionContext
call capability service
write platform response
```

Rules:

- handlers must not contain business state transitions;
- handlers must not call GORM directly;
- handlers must not assemble audit rows manually when the capability service owns the action;
- admin handlers must enforce permission intent through middleware or service-level checks;
- collection queries must use request bodies for filters and must reject sensitive filter fields.

### Unified Transaction Style

Write paths use Unit of Work:

```go
result, err := platform.WithUnitOfWork(exec, func(txExec platform.ExecutionContext) (Result, error) {
    return service.DoThing(txExec, request)
})
```

Rules:

- cross-capability writes must share the same Unit of Work;
- services must not create hidden nested transactions unless explicitly documented;
- audit, session and notification writes that belong to the same user action should be in the same Unit of Work when consistency matters;
- reads may run without Unit of Work unless they must participate in a larger write flow.

### Unified Error Style

Capability services return platform errors:

```text
code
message
safe detail
field errors
cause
```

Rules:

- user-facing messages are safe and localized at the adapter edge when needed;
- causes are logged but not exposed to clients;
- permission errors, validation errors, not-found errors and conflict errors use stable error codes;
- HTTP adapters map platform errors to status codes consistently.

### Unified Audit Style

Write actions declare audit metadata in the capability manifest and record through the audit capability.

Rules:

- audit action names use `capability.resource.action`, for example `identity.user.create` or `business.task.status.update`;
- services record audit with actor, tenant, target type, target ID and note;
- handlers must not invent audit action names;
- seed and system jobs use a system actor.

### Unified Admin Resource Style

Admin resources are registered through `AdminResourceDefinition`.

Rules:

- no resource-specific `resource === "name"` branching in the core resource engine;
- columns, filters, toolbar actions, row actions, forms, drawers, modals and detail panels live in the capability or app package that owns the resource;
- standard list resources use the shared engine;
- complex screens register `customPage`;
- resources declare permission, route, menu and API resource metadata through the capability manifest;
- disabled capabilities must not register admin resources or menus.

### Unified Frontend API Style

Frontend calls use typed capability clients:

```text
platformApi.admin.request(...)
platformApi.identity.users.query(...)
platformApi.files.upload(...)
businessApi.tasks.updateStatus(...)
```

Rules:

- UI code must not construct absolute backend URLs;
- admin requests must stay on relative admin or capability paths accepted by the request boundary;
- UI code must not set `Authorization` manually;
- query filters must pass through the shared query safety layer;
- capability clients own endpoint paths and request/response types.

### Unified Config Style

Configuration is capability-scoped:

```text
PLATFORM_CAPABILITIES=dictionary,tenant,identity,session,rbac,menu,audit,admin-shell,wechat-login,parameter,file-storage
PLATFORM_BRANDING_PRODUCT_NAME=...
PLATFORM_FILE_STORAGE_DRIVER=...
PLATFORM_WECHAT_MINIAPP_APP_ID=...
```

Downstream business packages may add their own capability IDs in their own composition root; `platform-go` must not require or ship those IDs by default.

Rules:

- capability config keys are prefixed by the capability domain;
- secrets are never stored in frontend code or seed files;
- config validation runs during `Configure`;
- invalid enabled capability config is a startup failure.

### Unified Codegen Style

The codegen capability is preview-first in v1.

Rules:

- manifests generate contracts, previews, scaffold dry-run safety plans, generated scaffold file packages and scaffold drafts;
- generated previews are stable and reviewable;
- generation does not overwrite Go or React source by default;
- any future source-writing generator must have an explicit dry-run, diff review and test gate;
- hand-written complex actions remain first-class and must not be forced into generic CRUD generation.

### Prohibited Patterns

- importing an application package from platform core;
- directly editing core route registration for a business capability;
- directly editing core admin engine for a business resource;
- direct GORM access from HTTP handlers;
- bypassing `ExecutionContext` for permissioned operations;
- writing audit logs with ad hoc action names;
- adding query-string filters for sensitive fields;
- making capability disable destructive by default.

## Testing Strategy

The architecture requires tests at four levels:

1. Kernel tests:
   - lifecycle ordering;
   - dependency graph validation;
   - capability enable and disable behavior;
   - Unit of Work commit and rollback.

2. Core governance tests:
   - tenant isolation;
   - RBAC permission checks;
   - session token type separation;
   - audit log writes;
   - sensitive query rejection.

3. Capability contract tests:
   - manifest completeness;
   - route and permission registration;
   - admin resource registration;
   - seed behavior;
   - disabled capability behavior.

4. Reference coverage tests:
   - reusable foundation candidates from `zshenmez` are mapped to platform capabilities;
   - business-only resources, app routes and workflows stay classified outside `platform-go`;
   - admin build passes;
   - backend tests pass;
   - resource contract validation passes.

## zshenmez Reference Acceptance

`zshenmez` is a reference coverage target, not a migration target. The platform is acceptable when it proves the reusable foundation covers common management scenarios and keeps these business workflows outside the default contract:

- business menu grouping;
- role application review;
- task ledger list, details and status actions;
- transfer applications and transfer chains;
- task check-ins and completion confirmations;
- public profiles, portfolio works and favorites;
- support tickets and handling actions;
- file upload, preview, download, delete and storage settings;
- audit logging for write actions;
- permissions and menu filtering.

The acceptance target is boundary correctness, not functional migration. Business packages that want these workflows must implement them outside `platform-go` through public capability manifests, resource schemas and route registration contracts.

## Implementation Slices

### Slice 1: Kernel And Core Governance Skeleton

- Create kernel packages for config, registry, lifecycle, DB, HTTP and Unit of Work.
- Move core governance models and services behind interfaces.
- Register first-party core capabilities.
- Add dependency graph and lifecycle tests.

### Slice 2: Admin Shell And Resource Engine

- Bring over the admin shell.
- Introduce `AdminResourceDefinition`.
- Convert system resources from hardcoded branches to resource definitions.
- Preserve admin request boundary and query security.

### Slice 3: Optional Common Capabilities

- Add branding.
- Add file storage.
- Add WeChat login.
- Add demo seed framework.

### Slice 4: External Business Integration Guidance

- Keep `zshenmez` business models, routes, menus, permissions and admin definitions outside `platform-go`.
- Provide integration guidance and reference coverage gates so a downstream business package can attach through manifests without editing platform core.

### Slice 5: Resource Contract And Codegen Preview

- Evolve the existing admin resource manifest into platform capability contracts.
- Keep generation as preview, scaffold dry-run safety plan, generated scaffold file package and scaffold draft only.
- Add validators for manifests and registered resources.

## Risks And Controls

| Risk | Control |
| --- | --- |
| Capability packages mutate global state directly | Enforce lifecycle-only registration |
| Business code bypasses permissions | Require ExecutionContext with actor, tenant scope and permission intent |
| Store grows into another monolith | Split services by capability and expose interfaces |
| Admin resource engine only supports CRUD | Include actions, forms, drawers, modals, detail panels and custom pages in v1 |
| Disabling capabilities destroys data | v1 disable is non-destructive only |
| Resource manifest becomes fake single source of truth | Use manifest for contracts, Go interfaces for behavior |
| `zshenmez` business concepts leak into the default foundation | Require reference discovery, reference coverage and profile-leakage validators |
| First version overbuilds plugin infrastructure | No runtime hot-plug, no marketplace, no external plugin process |

## Decision Summary

- Use a monolithic deployable with engineering-level capability packages.
- Keep core governance and admin shell as first-party platform capabilities enabled by default.
- Use typed internal Go interfaces plus external HTTP/Admin adapters.
- Require actor, tenant, permission and transaction context for capability calls.
- Replace the giant admin resource page with an admin resource engine.
- Treat external business integration as a downstream package concern, with `zshenmez` used only as reference coverage evidence.
- Keep code generation as validation and scaffold preview in v1.
