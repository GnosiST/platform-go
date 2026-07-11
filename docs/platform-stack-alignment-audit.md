# Platform Stack Alignment Audit

Date: 2026-07-04
Last updated: 2026-07-06

## Verdict

The original `platform-go` implementation had drifted from the intended source system. The current implementation has corrected the main stack mismatch and should now be treated as a stack-aligned foundation, with remaining work focused on production hardening and deeper Refine/business extension points.

The intended reference is `jiedanshi/platform`, whose baseline is:

- backend: Gin + GORM + Casbin + JWT;
- frontend: Refine + React + Ant Design;
- admin resource contract: `platform/resources/admin-resources.json`;
- generation loop: resource contract, codegen preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft and manifest validation;
- system modules: users, tenants, org units, roles, role groups, permissions, menus, API resources, dictionaries, parameters, audit logs, login logs, error logs, files, sessions, versions and API tokens.

`platform-go` originally had:

- Gin;
- hand-rolled admin resource store with memory/file/`database/sql` snapshot adapters;
- hand-rolled RBAC permission checks;
- opaque session token store instead of JWT;
- React + Ant Design without Refine;
- schema-driven generic resources, but not the `jiedanshi` resource manifest and generation loop;
- partial system management resources.

This was a material architecture mismatch. The core correction is now in place:

- GORM dependencies, storage opener seams, the admin resource repository, session repository and lifecycle history repository are in place; tenants, org units, users, user-role bindings, role groups, roles, role-permission bindings, permissions, menus, area codes, audit logs, login logs, error logs and versions now persist through normalized GORM tables.
- Casbin is now the backend authorization engine for HTTP permission enforcement, with policies generated from the dynamic `users` and `roles` admin resources.
- JWT admin bearer tokens now wrap revocable server-side sessions for HTTP auth.
- Refine dependencies and runtime providers are wired around the admin app. URL state now runs through React Router with Refine `syncWithLocation`; `AdminShell` still owns the visible layout, while `PlatformRoutePages` owns custom page and schema-driven resource page route elements. Schema-driven list/create/update/delete flows use Refine CRUD hooks and the shared data provider.
- The admin resource manifest, generated contract, codegen preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft and validator loop exist.

Further feature work should preserve these correction seams instead of reintroducing a divergent stack.

## Reference Evidence

Observed in `jiedanshi/platform/api/go.mod`:

- `github.com/gin-gonic/gin`
- `gorm.io/gorm`
- `gorm.io/driver/mysql`
- `gorm.io/driver/postgres`
- `gorm.io/driver/sqlite`
- `github.com/casbin/casbin/v2`
- `github.com/golang-jwt/jwt/v5`

Observed in `jiedanshi/platform/admin/package.json`:

- `@refinedev/core`
- `@refinedev/antd`
- `@refinedev/react-router`
- `react`
- `antd`

Observed in `jiedanshi/platform/resources/admin-resources.json`:

- menus, permissions, API routes, Refine resources and codegen modes share one manifest.

Observed in `jiedanshi/platform/scripts`:

- `generate-admin-resource-contract.mjs`
- `generate-admin-codegen-preview.mjs`
- `generate-admin-scaffold-plan.mjs`
- `generate-admin-scaffold-files.mjs`
- `generate-admin-scaffold-draft.mjs`
- `validate-admin-resources.mjs`

## Capability Gap Matrix

| Capability | jiedanshi/platform | platform-go now | Required correction |
| --- | --- | --- | --- |
| Gin HTTP runtime | Present | Present | Keep Gin. |
| GORM persistence | Present | Admin resource, session and lifecycle repositories are GORM-backed for mysql/postgres/sqlite; tenants, org units, users, user-role bindings, role groups, roles, role-permission bindings, permissions, menus, area codes, audit logs, login logs, error logs and versions are normalized; file/memory modes remain for local development | Add migration tooling, backup/export/import and production rollout runbooks when deployment targets require them. |
| Casbin authorization | Present | Backend enforcement routes through Casbin generated from normalized user-role and role-permission records; explicit deny rules and tenant/org/area/self data scopes are enforced through the platform resource layer; local policy refresh and Redis invalidation clear policy, principal and menu caches after writes; optional `policy-review` contributes request/reject/approve/export ledgers and approve write-through through `enterprise-governance` | Add production policy migration, rollout rehearsal and rollback workflows only when deployments need controlled policy migration beyond the current review ledger. |
| JWT auth | Present | Admin HTTP auth returns JWT bearer tokens with session/revocation checks plus sliding renewal; optional Admin OIDC authenticates existing bound Admin users only; app HTTP auth and generic app identity binding use an isolated `tokenType=app` runtime | Keep the independent refresh-token-family slice disabled until external promotion approval; add product-specific identity adoption only in downstream capabilities. |
| Refine admin runtime | Present | Refine providers/runtime wired; React Router URL state, route elements and Refine `syncWithLocation` are enabled; schema-driven resource routes enter `platform/refine/ResourceRoutePage` and guard read access through `useCan`; schema-driven resource list/create/update/delete run through Refine CRUD hooks and `dataProvider` meta for safe query; shared form layout presets, controlled source/runtime slots, schema row actions and platform-governance custom panels stay on the Refine-compatible shell | Add new custom panels only when a platform-governance workflow cannot stay generic, and keep business panels in downstream capabilities. |
| Dynamic routes | Present through backend menus to Refine resources | Backend menus feed Refine resource metadata, app API route contracts are declared through `Manifest.App.Routes`, exported through `AppRouteContracts`, `resources/generated/app-route-contract.json`, `openapi.app.json` and `app-codegen-preview.json`, direct URL/menu/history navigation syncs to route elements, resource route pages use Refine metadata/access-control hooks, and `/api/app/*` handlers are exposed through neutral `approute.Registration` handlers only when the enabled manifest declares the same method/path. | Add business identity adoption tests for app routes when a target product enables those app capabilities. |
| Dynamic menus | Present with backend `menus` table and `/api/admin/navigation/menus` | Backend `GET /api/admin/menus` exposes role-filtered multi-level menus from capability/admin resource metadata, including parent, external-link and cache flags | Add browser-level role visibility and multi-level navigation regression coverage as the menu editor matures. |
| Permission codes | Present, role-permission managed and Casbin enforced | Present as generated records, normalized role-permission rows in the GORM adapter, Casbin-enforced HTTP actions, deny overrides, tenant/org/area/self data scopes and cache invalidation after policy writes | Add production policy export/import and migration guardrails only when deployment operations need them. |
| Code generator | Preview/scaffold/validation present | Manifest, contract, preview, dry-run safety plan, generated scaffold files, scaffold draft and validator present | Keep source-writing generator deferred until generated-file boundaries and scaffold output are reviewed. |
| Form generator | Refine/AntD resource page patterns present | Schema-driven Ant Design forms support groups, help text, validation metadata, relation fields, edit hydration, `two-column-density`, `side-detail-preview`, controlled source-level React slots, controlled runtime slot descriptors and browser-verified desktop/mobile layouts through the generic resource console | Keep arbitrary backend component names, component paths, raw scripts and source-writing generators forbidden; business renderers must be registered outside platform core. |
| Swagger/API docs | Not fully implemented in reference; API docs exist as docs and generated candidates | Admin OpenAPI JSON generation, `GET /api/openapi.json` runtime serving, an admin API Docs page, and separate App OpenAPI generation are present | Add a Swagger UI wrapper later only if interactive try-it-out docs are needed; expose App docs runtime only when an app-facing docs surface is needed. |
| Basic system management | Broadly present | Manifest-backed resources cover tenants, org units, users, role groups, roles, permissions, menus, API resources, dictionaries/parameters, settings, audit/login/error logs, files, sessions/versions, API tokens, demo data and monitoring placeholders | Keep optional modules detachable and add production-specific workflows such as retention, export, review and monitoring integrations when required. |
| File/storage | Present local/S3 | Local adapter, S3-compatible adapter, `file-storage` manifest, upload/content/delete API, generic metadata resource, normalized audit records and generic resource-console preview/audit experience are present | Add production retention, object lifecycle and external storage operations workflows only when deployments require them. |
| Audit/logs | Present | Audit, login and error resources have normalized GORM base tables behind the generic resource API; generic admin writes emit structured `audit-logs` records with actor/resource/target metadata | Add retention, export, review workflows and production telemetry sinks. |

## Required Architecture Correction

`platform-go` must stay on the cleaned extraction path from `jiedanshi/platform`, preserving the same technology choices while improving modularity.

### Backend baseline

- Keep Gin.
- Keep GORM and supported drivers: MySQL runtime default, PostgreSQL supported, SQLite for tests.
- Keep Casbin as the authorization engine and move policy persistence behind the normalized RBAC model.
- Keep JWT for admin and app token types. Admin and app HTTP tokens preserve revocation through the session store; generic app identity binding exists behind `httpapi.AppIdentityResolver` and `app-identities`; optional Admin OIDC uses an isolated Admin resolver plus hash-only `admin-identities` bindings to existing enabled Admin users. OIDC must not auto-provision users or authorization relationships. Default refresh-token-family enablement, provider-specific token refresh and product-specific identity adoption remain gated hardening.
- Keep Redis/cache as an infrastructure adapter.
- Keep capability manifests, but make them feed GORM models, Casbin policies, admin route metadata, generated app API route contracts and admin resource manifests.
- Serve the generated admin OpenAPI document through Gin at `GET /api/openapi.json` with `admin:api-docs:read`.
- Keep API token management as a generic security resource: one-time `pgo_` secret issue, prefix/hash storage, scope validation against permission codes, scoped Bearer authorization and revoke-in-place semantics. Do not expose or replace raw token material in admin list/query/update APIs.

### Frontend baseline

- Keep Refine dependencies and providers wired into the runtime.
- Keep the schema-driven `ResourceRoutePage` + `GenericResourceConsole` adapter on Refine CRUD hooks. Form layout presets, controlled slots, schema row actions and platform-governance custom panels are implemented on shared wrappers; keep future extensions behind the same schema/action/panel boundaries instead of forking pages by resource name.
- Keep the approved UI componentization, but implement it as Refine-compatible components:
  - `ResourcePageHeader`;
  - `PlatformDataTable` with column settings, safe query hints, range filters, batch actions, compact centered pagination and mobile card ordering before pagination;
  - `SystemSettingsDrawer`;
  - `AdminThemeProvider`.

### Resource manifest baseline

Port the `jiedanshi/platform/resources` model:

```text
resources/admin-resources.json
  + enabled capability Manifest.Admin.Resources
  -> resources/generated/admin-resource-contract.json
  -> resources/generated/openapi.admin.json
  -> scripts/generate-admin-openapi.mjs
  -> resources/generated/admin-codegen-preview.json
  -> resources/generated/admin-scaffold-plan.json
  -> resources/generated/admin-scaffold-files.json + resources/generated/scaffold/
  -> resources/generated/admin-scaffold-draft.md
  -> scripts/validate-admin-resources.mjs
```

The merged admin resource contract should become the source for:

- resource code and labels;
- menus and route paths;
- permission codes;
- API routes;
- Refine resource names;
- codegen mode;
- docs and audit candidates.

## Revised Implementation Order

1. **Freeze custom-stack expansion.** Done as a standing rule.
   Do not add features that bypass Gin, GORM, Casbin, JWT, Refine, capability manifests, resource schemas or platform UI wrappers.

2. **Introduce stack parity dependencies and ports.** Done.
   GORM, Casbin, JWT and Refine dependencies/ports exist. Future work should harden these ports rather than fork another runtime path.

3. **Port resource manifest and validators.** Done for preview mode.
   Bring over the manifest/contract/preview/scaffold/validator loop from `jiedanshi/platform`, then adapt names to `platform-go`.

4. **Replace persistence with GORM repositories.** Admin resource, session, lifecycle, standard platform, operations, file-storage and API-token resource slices done.
   Continue with production retention, lifecycle, export and rollout workflows when deployments require them.

5. **Replace custom permission checks with Casbin.** Done for protected HTTP actions.
   Keep permission codes, deny rules, data scopes, policy-review write-through and local/Redis invalidation behind the current RBAC model; add production policy migration and rollback workflows only when deployment operations need them.

6. **Replace opaque sessions with JWT sessions.** Admin HTTP, Admin OIDC existing-user binding, admin sliding renewal and isolated app HTTP slices done.
   Keep server-side session revocation, repository-backed convergence and cache invalidation behavior. The production-like OIDC rehearsal is complete, but external production promotion remains `not-approved`; independent refresh-token-family default binding remains blocked by the production approval gate, and product-specific identity adoption belongs downstream.

7. **Migrate frontend to Refine runtime.** Provider wrapper, React Router URL convergence, guarded `ResourceRoutePage`, route-page adapter, Refine CRUD hooks, controlled form slots, schema actions and platform-governance panels are done.
   Keep future actions, form hooks and panels manifest/schema-driven, and keep business-specific panels outside `platform-go`.

8. **Apply approved UI componentization on Refine.** Base slice done.
   Keep `PlatformDataTable`, `ResourcePageHeader`, `SystemSettingsDrawer` and theme synchronization as the default shared surfaces; extend through props/slots rather than page forks.

9. **Add admin API Docs page.** Done.
   OpenAPI JSON generation, `GET /api/openapi.json` and the admin API Docs page are connected. A Swagger UI wrapper can remain optional.

10. **Rebuild optional capabilities.** Base slices done.
    WeChat login, Admin OIDC, branding, demo data, Redis cache and file storage sit on the corrected stack. Admin OIDC has local production-like protocol and browser evidence, while real production promotion still requires the external approval, credential ownership, rotation and rollback package.

## Decision

Future implementation plans must preserve the corrected stack before adding new UI or business capability work. UI componentization remains approved, and its implementation target is the Refine-based admin runtime.
