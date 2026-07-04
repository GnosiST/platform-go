# platform-go Capability Architecture Design

Date: 2026-07-04

## Purpose

`platform-go` is a reusable operations platform foundation extracted from the current `zshenmez` platform work. It should preserve the existing management-backend strength while making common capabilities selectable, replaceable and extensible for future business projects.

The platform is not a runtime plugin marketplace and not a low-code-only admin generator. The first version is a single deployable service with engineering-level capability packages compiled into the project and enabled by configuration.

## Current Context

The source system in `zshenmez` already proves several reusable ideas:

- Gin + GORM + Casbin backend with admin and app token separation.
- Refine + React + Ant Design management backend.
- Tenant, user, RBAC, menu, API resource, dictionary, parameter, audit, file and session governance.
- A resource manifest in `resources/admin-resources.json` that keeps menus, permissions, API routes, Refine resources and generated previews aligned.
- Complex business admin resources for role applications, tasks, transfer chains, fulfillment, public profiles, portfolio works, favorites and support tickets.

The same source system also exposes the main extraction risks:

- `admin/src/pages/ResourceTablePage.tsx` has grown into a large page containing standard resources, business resources, drawers, modals, row actions and special workflows.
- `api/internal/store/store.go` mixes platform governance, app identity, demo data and business persistence in one broad Store.
- `api/internal/httpapi/server.go` registers `/api/app` and `/api/admin` routes in one server method.
- Seed data and framework menus include both platform system resources and `zshenmez` business menus.

The architecture must reduce those coupling points without reducing the current `zshenmez` admin capability.

## Goals

- Provide a reusable platform foundation for future business projects.
- Keep first-party core governance available by default: tenant, identity, session, RBAC, menu, API resource, audit, dictionary, parameter, admin shell and system admin resources.
- Allow optional common capability packages such as WeChat login, file storage, branding, demo seed, notifications, jobs, OpenAPI, code generation and AI.
- Allow business capability packages such as `zshenmez-business` to register routes, permissions, menus, admin resources, custom actions and data models without editing platform core.
- Provide one internal Go calling model and one external adapter model so business code does not depend on platform internals.
- Prove the architecture with `zshenmez-business` parity: the current management-backend capability must not regress.

## Non-goals

- No runtime hot-plug plugin installation in v1.
- No plugin marketplace.
- No automatic destructive uninstall or data cleanup when a capability is disabled.
- No broad low-code engine that tries to express every business workflow in JSON.
- No automatic source-code overwrite from generation scripts in v1.
- No microservice split in the first implementation.

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
  zshenmez-business / future business modules
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
| `codegen` | resource contract | Generated previews and scaffold drafts, without overwriting source by default |
| `AI` | identity, audit, parameter | Provider configuration, permissioned calls and call audit |

### Application Capabilities

Business modules live here. For the first proof, `zshenmez-business` is the parity capability:

- role applications and review actions;
- task ledger, detail drawer and status update;
- transfer applications and transfer chain tracking;
- task check-ins and completion confirmations;
- public profiles, portfolio works and favorites;
- support tickets and handler notes;
- business demo seed data.

Business capabilities may depend on core and optional capabilities, but platform core must not depend on business capabilities.

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
- `zshenmez-business` requires core governance and may require `file-storage`, `wechat-login`, `admin-resource-engine` and `demo-seed`.

Missing dependencies are startup failures. Cycles are startup failures.

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
- Task status update in `zshenmez-business` may update a task, write audit and enqueue notifications.

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
- business demo data lives in business capability packages.

`zshenmez` demo data must move into `zshenmez-business` fixtures, not platform core.

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

Business capabilities own role-specific outcomes. For example, `zshenmez-business` owns whether a user becomes customer, worker or merchant and which review workflow is required.

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
internal/apps/zshenmez
```

Frontend packages use the same split:

```text
admin/src/platform/shell
admin/src/platform/api
admin/src/platform/resources
admin/src/platform/capabilities
admin/src/capabilities/fileStorage
admin/src/capabilities/branding
admin/src/apps/zshenmez
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

- audit action names use `capability.resource.action`, for example `identity.user.create` or `zshenmez.task.status.update`;
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
platformApi.zshenmez.tasks.updateStatus(...)
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
PLATFORM_CAPABILITIES=wechat-login,file-storage,branding,zshenmez-business
PLATFORM_BRANDING_PRODUCT_NAME=...
PLATFORM_FILE_STORAGE_DRIVER=...
PLATFORM_WECHAT_MINIAPP_APP_ID=...
```

Rules:

- capability config keys are prefixed by the capability domain;
- secrets are never stored in frontend code or seed files;
- config validation runs during `Configure`;
- invalid enabled capability config is a startup failure.

### Unified Codegen Style

The codegen capability is preview-first in v1.

Rules:

- manifests generate contracts, previews and scaffold drafts;
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

4. Parity tests:
   - `zshenmez-business` preserves existing management backend resources, menus and workflows;
   - admin build passes;
   - backend tests pass;
   - resource contract validation passes.

## zshenmez Parity Acceptance

`zshenmez-business` is the first real validation target. The platform is not accepted until it proves no regression for:

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

The parity target is functional equivalence, not identical source structure.

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

### Slice 4: zshenmez Business Capability

- Move business models, routes, menus, permissions and admin definitions into `zshenmez-business`.
- Preserve current business backend capability through parity tests.

### Slice 5: Resource Contract And Codegen Preview

- Evolve the existing admin resource manifest into platform capability contracts.
- Keep generation as preview and scaffold draft only.
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
| Existing `zshenmez` admin capability regresses | Require `zshenmez-business` parity acceptance |
| First version overbuilds plugin infrastructure | No runtime hot-plug, no marketplace, no external plugin process |

## Decision Summary

- Use a monolithic deployable with engineering-level capability packages.
- Keep core governance and admin shell as first-party platform capabilities enabled by default.
- Use typed internal Go interfaces plus external HTTP/Admin adapters.
- Require actor, tenant, permission and transaction context for capability calls.
- Replace the giant admin resource page with an admin resource engine.
- Treat `zshenmez-business` as the first parity proof.
- Keep code generation as validation and scaffold preview in v1.
