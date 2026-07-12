# Platform Foundation Task Map

Date: 2026-07-05
Last reviewed: 2026-07-10

## Purpose

This map is the execution control layer for `platform-go`. It keeps the reusable platform foundation aligned with the approved stack and prevents independent tasks from pulling the project into conflicting directions.

The target is not to migrate or clone `zshenmez` business behavior into the foundation. The target is to cover reusable base capabilities that a management backend needs, using `zshenmez` only as one reference sample, then let independent business capabilities attach or detach through manifests and stable APIs.

## Technology Route

Authoritative stack:

- backend: Gin, GORM, Casbin, JWT;
- frontend: Refine, React, Ant Design;
- admin contracts: capability manifests plus `resources/admin-resources.json`;
- generated artifacts and gates: admin resource contract, app route contract, platform capability audit, capability contract gate, capability profile gate, reference coverage gate, task dependency graph gate, OpenAPI, codegen preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft and scaffold promotion review packet;
- optional infrastructure: Redis through the platform cache port.

Conflicts are resolved by this route:

- do not expand a divergent custom stack;
- keep generic resource behavior schema-driven;
- keep source-writing code generation deferred until scaffold previews and rules are stable;
- keep business tables, workflows and UI outside platform core unless they are common platform capabilities.

## Capability Coverage Target

Reusable base capability coverage should be at least broad enough for the common `zshenmez` management backend scenarios:

| Area | Foundation capability | Status | Next evidence |
| --- | --- | --- | --- |
| Authentication | provider discovery, demo login, JWT admin bearer token, sliding renewal, logout, session revocation | implemented | auth tests and browser login |
| WeChat Login | provider boundary, manifest, configurable miniapp code2session adapter, app identity resolver seam and generic hash binding persistence | implemented base | production account-linking and binding hardening |
| App Phone Binding | optional `app-phone` capability, app-session phone verification, local rolling-window abuse guard, phone hash binding and masked admin records | implemented base | production SMS adapter and distributed rate-limit policy |
| Organization And Area Model | tenants, org units, role groups, area codes and user/role linkage fields | implemented base | `TestCurrentSessionEndpointReturnsRoleBackedPermissions` covers current-session tenant/org/area context; store tests cover org/area data scopes; browser QA remains for admin forms |
| Personnel Extension | optional `personnel` capability with personnel profiles, positions and position assignments | implemented optional | `platform-personnel-ready` profile plus `scripts/validate-platform-personnel-runtime-readiness.mjs` prove default exclusion and enabled admin resource, permission, frontend and OpenAPI coverage |
| Authorization | users, roles, permissions, menus, Casbin HTTP checks, local policy refresh and Redis invalidation events | implemented base | production policy migration tooling when deployments need it |
| Policy Review | optional `policy-review` capability, `policy-reviews` ledger, request/reject/approve/export APIs, `enterprise-governance` profile and implemented `PolicyReviewConsole` | implemented optional | production migration or rollout evidence workflows when target deployments need them |
| Dynamic Menus | backend menu API, parent hierarchy, external/cache flags | implemented base | `TestMenuItemsForPrincipalKeepsDeepParentsAndAppliesRoleVisibility` covers deep parents, cache flags and role/deny menu visibility |
| Resource Schema | fields, search/filter/sort/localizable flags, form groups, field help, validation metadata, permissions, safe query | implemented base | resource validator and query tests |
| Engineering Capability Matrix | dynamic resources, menus, permission codes, organization/role-group/area governance, capability contract governance, optional personnel runtime readiness, schema forms, OpenAPI, admin API boundary/query security, App client API boundary, scaffold safety, source-writing readiness, system management, app route contracts, cache invalidation, deployment topology, task dependency governance, reference discovery and coverage boundary gates, foundation alignment/objective-conflict audit, goal completion audit, node closeout audit, objective conformance gate, promotion evidence templates, submitted evidence package validation and production readiness preflight | implemented gate | `scripts/validate-platform-engineering-capabilities.mjs`, `scripts/validate-platform-foundation-task-graph.mjs`, `scripts/validate-platform-capability-contracts.mjs`, `scripts/validate-platform-cache-invalidation.mjs`, `scripts/validate-platform-deployment-topology.mjs`, `scripts/validate-platform-admin-api-boundary.mjs`, `scripts/validate-platform-app-client-api-boundary.mjs`, `scripts/validate-platform-reference-discovery.mjs`, `scripts/validate-platform-reference-coverage.mjs`, `scripts/validate-platform-personnel-runtime-readiness.mjs`, `scripts/validate-platform-foundation-alignment.mjs`, `scripts/validate-platform-goal-completion-audit.mjs`, `scripts/validate-platform-node-closeout-audit.mjs`, `scripts/validate-platform-objective-conformance.mjs`, `scripts/validate-platform-promotion-evidence-templates.mjs`, `scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>`, `scripts/validate-platform-production-readiness.mjs` and drift tests |
| Capability Contracts | built-in, optional, local-demo and external-business capability classifications | implemented gate | `resources/platform-capability-contracts.json` plus `scripts/validate-platform-capability-contracts.mjs` prove every profile/default capability is classified, default-enabled capabilities match runtime defaults, profile-only capabilities stay out of defaults, external business stays outside non-business profiles and declared resources/routes/providers match audited Go manifests |
| Capability Profiles | minimal, default, app-ready, personnel-ready, notification-ready, job-ready and enterprise-governance capability compositions | implemented gate | `resources/platform-capability-profiles.json` plus `scripts/validate-platform-capability-profiles.mjs` prove dependency resolution, default-profile drift checks, optional policy-review/personnel/notification/job exclusion from defaults, explicit additional Go manifest paths, app composition-root registration for business profiles and business-module exclusion from default runtime |
| Admin UI Shell | themes, layouts, work tabs, dashboard, settings drawer, i18n | implemented base | desktop/mobile browser QA |
| List Components | sorting, column visibility, filters, range query, selection, batch slots, compact centered unified pagination, mobile cards before pagination, overflow tooltips | implemented base | shared table visual and query QA |
| Branding | API plus settings resource | implemented base | `TestBrandingEndpointReflectsAdminSettingsUpdate` covers API output, branding cache fill, settings invalidation and cache reload |
| Demo Data | manifest datasets, target-resource validation, duplicate record-key validation and apply API | implemented base | `TestAdminDemoDataApplyWritesDeclaredRecords` covers repeated apply without duplicate records |
| Cache/Redis | noop, memory, Redis, stats, invalidation targets and Redis pub/sub invalidation bus | implemented base | broader cache coverage only where invalidation remains explicit |
| API Docs | generated OpenAPI and admin API docs page | implemented base | OpenAPI generation and admin page build |
| Code Generator | contract, platform audit, reference coverage gate, preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft, scaffold promotion review packet and source-writing readiness gate | implemented safe preview | source-writing stays disabled until the explicit source-writing promotion node, spec and review gates are satisfied |
| Form Generator | schema-driven `PlatformResourceForm` with groups, help text, validation metadata, controlled source-level slots, controlled runtime slot descriptors, `two-column-density`, `side-detail-preview` and a layout/slot contract gate | implemented base | keep registry allowlists, i18n, permission filtering, UI contracts, build and browser evidence current |
| File Storage | optional `file-storage` capability, local/S3-compatible object adapters, admin upload/content/delete API, session-scoped App upload/content API, metadata resource schema, backend operation audit contract and implemented generic resource-console experience | implemented base + admin experience | upload, authenticated preview, download, delete, metadata drawer, audit fallback, App `appUpload` route coverage and mobile file cards are guarded by reference coverage, App client boundary and `resources/platform-file-storage-experience.json` evidence |
| Audit/Login/Error Logs | `audit-logs`, `login-logs` and `error-logs` resources with normalized GORM base tables and audit capability ownership; generic admin resource writes record runtime audit events; `audit-logs` has a structured read-only query schema | implemented base | production retention, export and audit review flows |
| API/App Tokens | one-time admin API token issue/update/revoke plus scoped Bearer authorization exists; app auth runtime issues isolated `tokenType=app` sessions and rejects admin/API tokens | implemented base | refresh/revocation production policy |
| App API Contracts | capability manifest can declare `/api/app/*` routes with auth mode, `app:` permission validation, `AppRouteRegistration` runtime handler binding, `app-route-contract.json`, `openapi.app.json`, app codegen preview and an App client API boundary requiring generated clients or request/upload ports | implemented base | business identity adoption, generated client adoption and route-level integration tests |
| External Business Boundary | business-only reference candidates classified under `external-business-capability` | implemented reference gate | downstream business packages own concrete workflows and persistence |

## Governance Model Assessment

The current governance model is intentionally broader than tenant-only management:

- `org-units` is the shared institution, department and team tree. It should remain the default organization model for the foundation because one tenant-owned tree supports multi-level departments, parent-child filtering and generic tree selectors without introducing parallel organization resources. `org-units.tenantCode` is required; user assignment to an org unit remains optional.
- `role-groups` is useful and should stay, but only as role catalog classification and governance metadata through a tree-capable `roles.groupCode` selector. It can be a tree catalog through `role-groups.parentCode`, but it must not grant permissions, inherit policies, bind users, own membership or carry data scopes. This keeps permission resolution predictable and avoids hidden access through group membership.
- `area-codes` is the shared address-code and region master-data resource. Tenants, org units, users and optional personnel resources should carry `areaCode` when regional ownership matters. Detailed address fields, employment locations or regional operation workflows should stay in the owning capability until at least two reusable platform capabilities need the same contract.
- Authorization still comes from `user -> roles -> permissions / denyPermissions`, followed by optional row-level data scopes. Address-code references are not permissions; area-restricted access must use explicit `roles.dataScopeAreaCodes`.

Implementation conclusion from the 2026-07-06 governance review, tightened on 2026-07-07: do not add separate `organizations`, `departments`, role-inheritance models or role-group membership models now. Keep one required-tenant `org-units` tree for institutions, departments and teams; keep `role-groups` as a tree-capable role catalog and review dimension, not an inheritance or membership model; keep `area-codes` as reusable regional master data, optional ownership metadata and explicit role area-scope input. HR files, positions and position assignments live in the optional `personnel` capability and the `platform-personnel-ready` profile; `scripts/validate-platform-governance-topology.mjs` now verifies that the generated personnel profile contract keeps personnel profiles linked to tenant, org unit, address code and user, while positions and assignments reuse the shared tenant/org boundaries. `scripts/validate-platform-personnel-runtime-readiness.mjs` adds the runtime evidence gate: default contracts and audits must not expose `personnel`, while the personnel-ready profile must generate personnel resources, read permissions, frontend entries and OpenAPI query paths/schemas together. The same validator also checks generated Go manifest output, so `users.tenantCode`, `org-units.tenantCode`, role-group linkage, role data-scope fields, org data-scope relations, area data-scope relations and permission/deny-permission relations cannot exist only in the static JSON contract. The 2026-07-07 hardening also makes `tenants`, `org-units`, `users`, `roles`, `role-groups` and `area-codes` non-droppable default governance primitives, and rejects any topology that promotes `personnel-profiles`, `positions` or `position-assignments` into the default foundation. Detailed street addresses or regional operation workflows should still stay in the owning capability until at least two reusable platform capabilities need the same contract.

Additional 2026-07-07 assessment: tenant-only support is explicitly rejected in `resources/platform-governance-topology.json` because it is too narrow for a reusable foundation. The supported default is tenant plus organization tree plus optional regional metadata. Role groups are explicitly supported because they improve role catalog management, but the topology contract keeps them classification-only and cross-checks that policy owners remain `roles` and membership ownership remains `users.roles`. Address-code support is explicitly supported as shared regional master data and explicit area-scope input; address-code references on tenants, org units, users or personnel do not grant access by themselves. Production readiness now includes the `governance-topology` preflight so release checks cannot skip this architecture boundary.

## Reference Project Coverage Audit

This audit uses `/Users/irainbow/Documents/DevelopmentSpace/myProject/zshenmez` as the reference management backend. The point is coverage, not copying: reusable system capabilities stay in platform foundation, while business workflows stay in optional application capabilities.

| Reference capability in `zshenmez` | Platform placement | Current `platform-go` coverage | Boundary decision |
| --- | --- | --- | --- |
| Admin login, JWT session, current user and login logs | Platform foundation | `auth` provider discovery, JWT admin bearer tokens, revocable sessions, `login-logs` and session resources | Keep provider adapters swappable; product identity mapping stays behind capability services. |
| Tenants, users, org units, user-role bindings and role groups | Platform foundation | `tenants`, `users`, `org-units`, `roles`, `role-groups`, data scopes, menu entries, seed data, validator gates and GORM normalized tables | Role groups classify and govern roles; they do not own role membership, permissions, inheritance or data scopes. |
| Permissions, menus and API resources | Platform foundation | Manifest-derived permission catalog, dynamic menus, Casbin checks and `api-resources` | Menus remain visibility hints; backend permission checks are authoritative. |
| Dictionaries, parameters, storage settings and branding | Platform foundation | `dictionaries`, `dictionary-parameters`, `parameters`, `settings`, `branding`, branding API and config resources; capability audit maps these through `dictionary` and `parameter`, while non-resource reference coverage maps storage settings through `parameter` plus `file-storage` | Treat project branding and operational parameters as data/config, not hard-coded shell behavior. |
| Files, upload/content/delete, sessions, versions, audit/error/login logs and API tokens | Platform foundation or optional platform capability | `file-storage`, `sessions`, `versions`, `audit-logs`, `login-logs`, `error-logs`, `api-tokens`; capability audit now maps sessions to `session`, logs to `audit`, and versions/API tokens to `system-admin` | Keep operational modules detachable where possible; add preview/export/review workflows only when target deployments need them. |
| Dashboard counts and operations overview | Platform foundation extension point | `/overview` common dashboard and dashboard UI surface | Base dashboard owns shell metrics; business dashboards should contribute data through capability-owned providers instead of forking the shell. |
| App phone verification/binding and detailed user addresses | Optional platform capability or owning business capability | `app-phone` provides app-session phone verification and hash-backed bindings through `platform-app-ready`; `user-addresses` is tracked as a detailed-address boundary outside `platform-default` | Phone verification is reusable but optional; full address records should not be confused with `area-codes` regional master data. |
| Role applications, public profiles, portfolio works, favorites, task ledger, transfers, check-ins, completion confirmations and support tickets | External business capability | Reference discovery and coverage classify these as business-only under `external-business-capability`; no concrete business resource ships in `platform-go` | Do not move these into platform core or the default platform backlog. Business write cutover and richer custom panels belong to downstream business packages when they explicitly opt in. |
| App/mini-program routes for role applications, public profiles, task creation/listing, transfer applications, check-ins, completion confirmations and support tickets | External business capability plus reusable app auth | Platform app session, app phone, WeChat resolver seam and app route contracts exist; reference coverage keeps product-specific app routes outside platform defaults | Reusable auth and route contracts stay in platform; product-specific app route handlers stay in the downstream business package. |

Coverage conclusion on 2026-07-06, tightened with discovery evidence on 2026-07-07 and default manifest drift checking on 2026-07-08: the current foundation covers the reference project's reusable management backend surface without enabling concrete business workflows by default. `resources/platform-reference-discovery.json` records the inspected `zshenmez` source sets and classifies each candidate before coverage mapping; `scripts/validate-platform-reference-discovery.mjs` rejects missing source signals, candidate `evidenceSignals` that are absent from the candidate's declared source sets, missing boundary explanations, default-profile leakage and attempts to promote business candidates into platform capabilities. `scripts/validate-platform-reference-coverage.mjs` reads the current `zshenmez/resources/admin-resources.json` through the discovery record by default and compares business-boundary declarations with `external-business-capability`, so newly added reference resources, business resources and business app-route additions cannot silently drift into the default platform contract.

## Dependency Graph

```text
Stack alignment
  -> capability manifest contract
    -> resource schema contract
      -> generated contracts and OpenAPI
      -> generic admin resource API
      -> Refine data provider
      -> PlatformDataTable and form surfaces

Stack alignment
  -> GORM storage opener
    -> admin resource repository
    -> session repository
    -> lifecycle history repository
    -> normalized RBAC and operations tables
      -> Casbin policy generation
      -> menu filtering
      -> resource/action authorization

Auth provider boundary
  -> JWT admin bearer token
  -> sliding renewal
  -> session revocation
    -> current-session API
    -> frontend auth/access-control providers
    -> role-aware menus and resource actions
  -> app identity resolver
    -> provider code exchange adapters
    -> app identity hash binding
    -> app phone verification and hash binding
    -> app guest identity restoration

Capability manifests
  -> auth providers
  -> app API route contracts
  -> demo data
  -> admin menus/resources
  -> permission catalog
  -> codegen/OpenAPI

Capability profiles
  -> capability contracts
  -> minimal-admin
  -> platform-default
  -> platform-app-ready
  -> enterprise-governance
  -> external reference classification
  -> composition gate
  -> platform capability audit gate

Reference coverage
  -> reusable foundation coverage
  -> default-platform exclusion checks
  -> external business boundary checks
  -> external reference drift checks

Task dependency graph
  -> approved stack route
  -> phase order
  -> task dependencies
  -> resource locks
  -> parallel batch conflict checks
  -> visual design gate checks
```

## Current P0 Work

Implemented P0: task node `production-persistence-correctness` is closed. GORM
session operations are record-scoped; GORM admin resource saves use revision
CAS; peer admin invalidation reloads an independent repository-backed Store
before clearing derived caches. File and legacy SQL adapters remain
compatibility modes, and Redis Pub/Sub is best-effort convergence rather than a
durable consistency log. Evidence is in the production persistence design,
Task 1-4 reports, focused Go tests and the cache invalidation validator.

1. Runtime consistency
   - Keep the default `9200/9202` dev runtime on current source.
   - Evidence: `/api/admin/resources/:resource/query` returns 200 through the default API and the admin list loads without query errors.

2. Documentation consistency
   - Keep README, AGENTS and docs aligned with JWT, GORM, Casbin, Refine and resource query facts.
   - Evidence: keyword checks have no stale opaque-session or pre-GORM lifecycle claims.

3. Refine routing convergence
   - Keep Refine providers active and migrate custom page rendering toward Refine resource routing.
   - Current state: `BrowserRouter` owns URL state, Refine `syncWithLocation` is enabled, backend menu resources open routes through React Router, and `PlatformRoutePages` renders custom pages plus schema-driven resource pages through `<Routes>`. Schema-driven resource routes enter `platform/refine/ResourceRoutePage`, which reads Refine resource metadata and guards read access through `useCan` before delegating to the generic resource console. The generic console still owns schema UI, but list/create/update/delete now flow through Refine CRUD hooks and `dataProvider` meta instead of direct API calls.
   - Constraint: `AdminShell` may remain the layout, but resource pages, guards and CRUD flow should increasingly use backend resource metadata and Refine route/data semantics.
   - Evidence: `scripts/validate-admin-refine-runtime.mjs`, `scripts/validate-admin-refine-crud.mjs`, direct `/tenants`, sidebar `/roles`, work tabs and browser back navigation pass in browser QA with no console warnings after the route-page adapter slice.

4. RBAC end-to-end proof
   - Prove role permission edits affect backend menus and protected resource actions.
   - Constraint: menu visibility is not security; backend checks remain authoritative.
   - Role groups are a governance/classification dimension through `roles.groupCode`; they do not grant permissions or inherit policies in the base model.
   - Users prefer `roles` for role bindings while the backend still accepts legacy `role` values for persisted snapshots; RBAC must be enabled when that relation-backed user role binding surface is exposed.
   - Roles can declare `denyPermissions`; deny matches override wildcard or exact allow matches in Casbin checks, backend menu filtering and frontend access controls.
   - The default Casbin authorizer is cached per API server and invalidated after `roles`, `users` or `permissions` writes, alongside principal and dynamic-menu caches. Redis cache mode publishes the same resource invalidation event to peer API instances.
   - Roles declare `dataScope` values and optional `dataScopeOrgCodes` / `dataScopeAreaCodes`; Casbin action checks still use role permission codes, while generic list/query calls apply row-level tenant/org/area/self filtering after the read action is allowed.
   - Organization, role group and area code references use schema-level `relation` metadata, so generic forms, filters and future generators can render dynamic options without business-specific code. Organization and area references use tree relation metadata for shared `PlatformTreeSelect` rendering; tenant/org relations are also consumed by the data-scope filter.
   - Tenants, org units, role groups and area codes are normalized in the GORM admin resource adapter, while keeping the same generic resource API. Address codes are regional master data and can also be used by explicit role area-scope values; address-code references still do not imply permissions by themselves.
   - `users` is the platform account/principal resource. Personnel files, employees, staff profiles, positions and position assignments are reserved for an optional `personnel` capability and must reuse shared tenant/org/area ownership fields when those dimensions apply.
   - Current-session principals expose the user's tenant, org unit and area codes. Tenant/org/area values are active data-scope inputs when the role declares the corresponding scope dimension.
   - Evidence: `TestCurrentSessionEndpointReturnsRoleBackedPermissions` verifies the current-session principal returns the user's tenant, org unit and area codes alongside role-backed permissions. `TestRolePermissionUpdateRefreshesSessionMenusAndResourceActions` logs in with a JWT admin token, updates the operator role permissions, verifies current-session permissions, menu route filtering and resource query 200/403 behavior. `TestRoleDenyPermissionsOverrideWildcardAllows` verifies that `denyPermissions` overrides `admin:*` for session state, menu visibility and resource queries. `TestAdminResourceQueryAppliesRoleDataScope` verifies that `current_org` lets `ops` read the users resource but only returns users in its own org.

5. Admin UI hardening
   - Keep i18n as a build gate for every admin component, including `admin/src/App.tsx`.
   - Tighten shared list surfaces: solid dropdown plugins, safe multi-condition filters, time/number ranges, integrated compact centered `PlatformPaginationBar`, overflow tooltips and batch slots.
   - Keep settings as a theme-synced drawer with appearance, layout, watermark, visual-aid, transition controls and persisted user preferences.
   - Keep sidebar collapse available in side, mixed and split layouts with desktop and mobile affordances.
   - Keep global environment and tenant context controls near work tabs until real switching is wired; their dropdowns must explain scope and remain lighter than resource filters. `production` is the runtime environment placeholder; `platform` is the platform-level tenant placeholder.
   - Keep `/overview` as a common platform dashboard with role-aware metrics and a theme-synced trend panel. Business dashboards should replace data through a project data source, not fork the shell.
   - Keep SQL-like search as a structured query DSL only. Values that look like SQL must remain literal query values and cannot be concatenated into database SQL.
   - Keep admin API calls behind `admin/src/platform/api/client.ts`; generic list pages must use Refine `dataProvider` meta and `POST /api/admin/resources/:resource/query`, not direct `fetch`, App API paths or query-string collection filters.
   - Keep downstream App/H5/mini-program calls behind generated App clients or platform request/upload ports. Business pages must not hand-write request/upload calls or `Authorization`; `resources/platform-app-client-api-boundary.json` and `scripts/validate-platform-app-client-api-boundary.mjs` tie this rule to `app-route-contract.json`, `openapi.app.json` and `app-codegen-preview.json`. The reusable file-storage App routes are `POST /api/app/files` and `GET /api/app/files/:id/content`; they are generic file-object plumbing and must not absorb business attachment workflows.
   - Evidence: i18n validator, `scripts/validate-admin-ui-contracts.mjs`, `scripts/admin-ui-contracts.test.mjs`, admin build and browser QA on dashboard, resource list, settings drawer, pagination and sidebar collapse. The UI contract gate must also protect relation-field behavior: generic resource forms keep AntD Form control-prop passthrough, edit modals hydrate selected records after opening, tree relations use `PlatformTreeSelect`, and relation options load through the Refine data provider rather than page-local resource API calls. Pagination QA must include desktop and narrow-screen geometry checks for centered page buttons, 24px-class pager controls, 36px-class footer density, no horizontal page overflow and mobile cards before pagination.

6. Task dependency and conflict control
   - Keep `resources/platform-foundation-task-graph.json` as the machine-checkable execution graph for the foundation roadmap.
   - Each task node declares phase, dependencies, resource locks, status, evidence and visual design gates when applicable.
   - Each resource lock has a policy entry with an exclusive/shared mode and localized rationale. A lock cannot be introduced as a raw string without explaining why it serializes work.
   - Resource-lock conflict groups describe cross-lock surfaces that must not be parallelized even when task nodes do not share the exact same lock. Current groups cover admin visual/i18n/browser QA, contract/codegen surfaces, auth/policy/runtime config and business/storage/app-contract cutover.
   - Declared evidence paths include docs, validators, tests and screenshots. Screenshot evidence is checked as a real path, and implemented or preview visual tasks must declare at least one screenshot, so visual QA cannot silently point at missing browser evidence.
   - Visual task design gates are intentionally allowlisted to `superpowers:brainstorming` and `product-design`. Unknown ad-hoc gate names are rejected so visual work keeps the same design-review standard.
   - A task cannot depend on a later phase unless it declares `phaseDependencyExceptions` with a localized rationale. This keeps necessary cross-phase contract inputs explicit instead of hiding execution-order conflicts.
   - Phase dependency exceptions are valid only for real later-phase dependencies; same-phase and earlier-phase dependencies must not use them.
   - Parallel batches are allowed only when their task nodes do not share exclusive resource locks, do not hit the same conflict group and do not depend on each other directly or transitively.
   - Capability profile composition is a first-class task node and resource lock. Changes to `PLATFORM_CAPABILITIES`, `resources/platform-capability-profiles.json`, default runtime capability lists or downstream composition-root registration must run the profile validator before claiming modular attach/detach safety.
   - Evidence: `scripts/validate-platform-foundation-task-graph.mjs` checks approved stack drift, unknown dependencies, dependency cycles, later-phase dependency conflicts, resource-lock policy coverage, exact resource-lock conflicts, resource-lock group conflicts, visual design-gate allowlists, missing product-design gates and missing screenshot evidence for visual tasks. `scripts/platform-production-readiness.test.mjs` keeps this task-graph validator in the production preflight catalog, and `scripts/platform-foundation-task-graph.test.mjs` covers the graph failure modes.

7. Reference coverage drift control
   - Keep `resources/platform-reference-discovery.json` aligned with the inspected `zshenmez` source sets before changing `resources/platform-reference-coverage.json`. Discovery is the evidence and classification gate; coverage is the platform mapping gate.
   - Keep `resources/platform-reference-coverage.json` aligned with default platform resources, optional extension boundaries and `external-business-capability` business-only classifications.
   - Required reusable reference areas are fixed as dashboard, identity and tenancy, RBAC and menu, API governance, dictionary/parameter/branding, audit and operations, file storage, auth providers and demo data. Removing one of these common areas now fails the reference coverage gate.
   - Reference resource parity is tracked per resource, not only by broad area. Each reference admin resource from `zshenmez` must be classified as foundation, business or extension: foundation resources must map to default platform admin resources and capabilities; business resources must stay out of the default platform contract and be owned outside `platform-go` through `external-business-capability`; extension resources such as personnel and positions must stay optional.
   - Non-resource reference capabilities are tracked separately. `storage-settings` maps to `parameter` plus `file-storage`; `admin-api-boundary-query-security` and `app-client-api-boundary` map to API governance as foundation gates; `app-phone-binding` must stay owned by optional `app-phone` and enabled through `platform-app-ready`; `user-addresses` must stay outside the default foundation as a detailed-address boundary until a reusable address module is deliberately promoted.
   - `scripts/validate-platform-reference-coverage.mjs` reads the default reference manifest from `resources/platform-reference-discovery.json` plus `resources/platform-reference-coverage.json.reference.resourceManifest`, then rejects newly added `zshenmez` admin resources that have not been classified. Use `--reference-manifest <path>` only for an alternate snapshot or review fixture.
   - Each required reusable area must map to at least one platform capability, not only to a loose resource name. This keeps coverage tied to capability composition and prevents accidental drift away from the modular foundation contract.
   - Default platform audits continue to prove reusable foundation resources exist and business/personnel resources stay out of core.
   - External business app routes are checked through the business boundary table when an external audit is supplied. This keeps app-facing task, transfer, check-in, completion and support actions out of platform core routes.
   - Capability profile `businessCapabilities` and reference `businessBoundary.expectedCapability` values must stay bidirectionally aligned, so a business capability cannot be declared without reference ownership or referenced without a profile contract.
   - Reference business boundaries from `zshenmez` must stay owned by abstract `external-business-capability`; resource-level parity owners must match their boundary owner so concrete reference business names cannot be smuggled into `platform-go`.
   - The top-level foundation alignment audit also treats `app-phone-identity`, `detailed-addresses` and `personnel-and-positions` as required optional boundaries. They can stay optional, but they cannot silently disappear from the platform goal audit after being identified from the reference project.
   - Evidence: `scripts/platform-reference-discovery.test.mjs` verifies source-set signals, foundation/extension/business classification, default-profile leakage and non-resource capability coverage. `scripts/platform-reference-coverage.test.mjs` verifies required reusable area presence, capability mappings, default reference manifest drift, default isolation, personnel extension exclusion, non-resource capability classification, app-phone optional-profile ownership, detailed-address exclusion, missing profile business ownership, reference business owner isolation, resource parity owner alignment, external business resource drift and missing business app route drift.

8. Production readiness preflight
   - Keep `resources/platform-production-readiness.json` aligned with runtime configuration, production environment variables, `ValidateRuntime()` snippets, docs and deployment preflight commands.
   - Generate `resources/generated/platform-operations-plan.json` from the same readiness contract as a dry-run review packet. The plan must keep `runtimeMutation=disabled` and `sourceWriting=disabled`.
   - The preflight is a contract gate, not a deployment script; it proves the production baseline is documented and enforced before a release process wires real secrets and infrastructure.
   - Evidence: `scripts/validate-platform-production-readiness.mjs` checks the contract and generated plan, `scripts/platform-production-readiness.test.mjs` covers missing env reads, missing docs, missing preflight scripts and runtime gate test gaps, and `scripts/platform-operations-plan.test.mjs` covers non-mutating plan generation.

## Current P1 Work

The P1/P2 foundation items below are represented in `resources/platform-foundation-task-graph.json`. The original 37-node foundation baseline remains implemented and closed, including Task 8 production Admin OIDC evidence. `runtime-security-containment` is `implemented` and `admin-watermark-export-governance` is `implemented`, producing `45 total / 39 implemented / 6 controlled unfinished`; `resources/platform-foundation-alignment-audit.json` projects the six remaining nodes through `requiredFutureTaskNodes` without moving the preserved baseline out of `requiredTaskNodes`.

Remaining nodes, in task-graph order: `sensitive-data-protection-runtime`, `sensitive-data-historical-migration`, `open-source-portability`, `public-docs-community`, `public-docs-site`, `github-release-publication`.

The approved program is defined by the [completion program](superpowers/specs/2026-07-12-platform-go-completion-program-design.md), [runtime security](superpowers/specs/2026-07-12-runtime-security-hardening-design.md), [watermark/export governance](superpowers/specs/2026-07-12-admin-watermark-export-design.md), [sensitive data encryption](superpowers/specs/2026-07-12-sensitive-data-encryption-design.md) and [open-source docs/site](superpowers/specs/2026-07-12-open-source-docs-site-design.md) specifications.

Watermark/export closeout:

- `admin-watermark-export-governance` is `implemented` with a normalized legacy-compatible configuration, independent screen/export scopes and exact `1/4/9/16` screen mark counts.
- Screen marks are inert DOM grid elements; export intent is passed explicitly to Policy Review JSON export, which adds structured `product`, `exportedBy` and `exportedAt` metadata and stores only the applied boolean in audit policy.
- Canonical OpenAPI and original file downloads remain unchanged. PDF, image, CSV, XLSX and arbitrary-file watermarking are outside this node.
- Evidence includes Product Design, `ui-ux-pro-max`, bilingual UI contracts, focused Go tests, Admin build, desktop and `390x844` browser checks, dark mode, every scope combination, exact DOM counts and clean console output.

1. Production auth hardening
   - Current state: the policy gate is implemented in `resources/platform-production-auth-hardening.json` and `scripts/validate-platform-production-auth-hardening.mjs`; it is also referenced by production readiness preflight and the engineering capability matrix.
   - Current state: admin sliding renewal is available through `POST /api/auth/refresh`; it renews the same server-side session and returns a newly signed JWT.
   - Current state: login, refresh and logout publish `sessions` invalidation events; peer API instances reload repository-backed session stores through the same invalidation bus used for policy/cache refresh.
   - Current state: provider runtime policy is machine-checked. Provider adapters must be manifest-declared and composition-root injected, unconfigured providers are denied by default, provider subjects stay hash-and-mask only, and production promotion requires tests for unconfigured-provider rejection, subject redaction, configured-provider-only login and provider error normalization.
   - Current state: the Provider Promotion Matrix now classifies built-in providers before promotion. `demo` stays local-harness-only; `wechat` is App-only and optional for production; `oidc` is Admin-only and requires the exact issuer/client/secret/redirect/scopes configuration contract, Admin resolver boundary, audience isolation, issuer/signature/audience/nonce/state/PKCE/redirect validation, explicit binding, disabled-user rejection, credential owner, rotation runbook, normalized errors, redaction and production-like rehearsal evidence.
   - Current state: `docs/superpowers/specs/2026-07-07-platform-production-session-policy-design.md` is the machine-checked production session-policy specification. It keeps current sliding renewal distinct from optional offline refresh-token-family behavior and requires hashed refresh-token storage, rotation lineage, reuse detection, revocation scope, Redis/session invalidation convergence and audit redaction before default runtime enablement.
   - Current state: `resources/generated/production-auth-promotion-review.json` is generated as a non-mutating production-auth review packet. It keeps the decision `not-approved`, lists active blockers and missing approval evidence, and is required by the `token-rotation` production preflight before any credential or provider promotion work.
   - Current state: `internal/platform/refreshtoken` now contains the disabled refresh-token-family store, GORM repository, rotation/reuse-detection service and audit-redaction adapter. It is not bound to the default `/api/auth/refresh` endpoint.
   - Enable independent refresh-token rotation only when product session policy requires offline renewal, token family reuse detection or stricter rotation semantics and the production approval package is complete.
   - Add stronger token-family revocation or centralized session policy only when multi-instance production requirements exceed shared repository reload semantics.
   - Task graph node: `production-auth-provider-hardening` is `implemented`: the contract gate and session-policy spec are active and machine-checkable, and the generated operations plan carries the production approval completed-evidence schema for artifact URI/hash, reviewed commit, target environment, rollback commands, `rtk` verification commands, audit sample refs, provider rotation runbook refs and refresh-token-family test refs. Runtime promotion remains blocked while the refresh-token-family slice is `implemented-disabled`, `defaultRuntime=disabled`, Redis/persistence convergence evidence, structured security/platform/operations approval evidence, rollback evidence and redacted audit samples are missing. Independent refresh-token family behavior is intentionally not enabled by default, and text-only approval is rejected by the production auth hardening validator.

2. Form schema expansion
	- Current state: layout groups, validation metadata and field help text are available in the resource schema and consumed by `PlatformResourceForm`.
	- Current state: business resources can opt into field-level localized data by declaring `localizable` and storing `<key>Zh` / `<key>En` values; generic list/detail/search paths honor active language with canonical fallback.
	- Next: add richer layout presets or translation-table adapters only after repeated use cases prove the extension shape.
	- `resources/platform-form-schema-layout-slots.json` is the current machine-checkable gate: `single-column`, `grouped-sections`, schema-driven `two-column-density`, `side-detail-preview`, controlled source-level React slots and controlled runtime slot descriptors are implemented. Browser QA covers 1440px desktop dense forms, 390px dense-form fallback, desktop side-preview rail, mobile collapsed preview stack, no horizontal overflow and `AdminFormModal` internal scrolling.
	- Keep arbitrary runtime component paths, raw script slots and backend-manifest React component names forbidden. Standard CRUD should use the generic resource console and `PlatformResourceForm` before adding business slots. Business packages may register renderer implementations only at their own composition boundary.
	- Task graph node: `form-schema-layout-and-slots` is `implemented` and `contractGateOnly=false`; follow-on changes must preserve registry allowlists, localized slot labels/descriptions, permission filtering, AntD Form prop passthrough and Refine hook integration.

3. Generator hardening
   - Keep preview/scaffold first. `codegen-preview-scaffold` is now implemented as a safe non-mutating scaffold capability; it is not evidence that source-writing generation is enabled.
   - Current state: `admin-scaffold-plan.json` gives a machine-readable dry-run plan with source writing disabled, generated-file boundaries under `resources/generated/scaffold/`, candidate files and conflict status. `admin-scaffold-files.json` plus files under `resources/generated/scaffold/` materialize those drafts with marker and hash validation while still leaving runtime source untouched.
   - Current state: `admin-scaffold-promotion-review.json` packages scaffold files, hashes, target families, eventual runtime targets, preflight commands, approval evidence package and manual review status into a non-mutating review packet. It remains `not-approved` until a separate human source-writing review exists.
   - Current state: `resources/platform-codegen-source-writing-readiness.json` records the explicit gates for any future source-writing generator: source writing disabled now, `sourceWritingApprovalPackage.status=blocked`, separate spec, platform-architect/codegen-owner/runtime-owner/operations-owner approvals, completed-evidence artifact schema with external absolute artifact URIs, `sha256:` plus 64 lowercase hex artifact hashes, reviewed diff, rollback plan, target-family test output, runtime target owner approval and safe runtime target allowlist.
   - Current state: target families bind scaffold roles to allowed runtime target roots and `rtk` test commands, so future promotion can be reviewed by backend model, repository, route, admin page or contract-test family.
   - Current state: `resources/platform-engineering-capabilities.json` records the reusable engineering capability matrix and `scripts/validate-platform-engineering-capabilities.mjs` checks required capability IDs plus each entry's target-stack dependencies, key backend/frontend wiring, source files, generated files, admin resources, OpenAPI paths, scaffold source-writing boundaries, source-writing readiness, foundation alignment, goal completion, node closeout, objective conformance and production-readiness preflight evidence.
   - Current state: `resources/platform-task-execution-audit.json` records the ordered seven-node remainder of the completion program as controlled unfinished work. Future runtime/source-writing promotion blockers, required validators and conflict-free parallel batches remain explicit; visual/UI work stays tied to both `superpowers:brainstorming` and `product-design` evidence.
   - Current state: `resources/platform-goal-completion-audit.json` records `not-complete-controlled` at 45 total, 39 implemented and 6 unfinished. `resources/platform-node-closeout-audit.json` records 39 implemented closeouts and rejects closeout evidence for pending nodes. `resources/platform-objective-conformance.json` projects the same six controlled blockers while keeping approved stack, reference-only `zshenmez` usage, capability-contract governance, future promotion gates, visual gate order, Vercel boundary and production preflight aligned.
   - Current state: `resources/generated/platform-promotion-evidence-templates.json` contains draft-only evidence package templates for production auth promotion and source-writing promotion. `scripts/validate-platform-promotion-evidence-templates.mjs` keeps those templates aligned with the approval schemas while rejecting submitted state, runtime mutation, completed approval values or sensitive forbidden fields. `scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>` validates external completed evidence packages, including external absolute `https`/`s3`/`gs` artifact URIs and strict `sha256:` plus 64 lowercase hex artifact hashes, against the same schemas without mutating platform contracts or marking blockers complete.
   - `resources/platform-foundation-alignment-audit.json` is the top-level objective and conflict audit. `scripts/validate-platform-foundation-alignment.mjs` cross-checks the approved stack, task graph, engineering matrix, reference discovery and coverage, admin API boundary/query security, required optional boundaries, governance topology, capability profiles, visual design gates, source-writing policy and production preflight commands so a later task cannot silently drift away from the platform-foundation objective.
   - Current state: `objectiveConflictPolicy` is fail-fast. It requires the alignment node to depend on task dependency governance, reference discovery, reference coverage, source-writing readiness, production readiness, governance topology and visual product-design QA; production preflight must include reference discovery, foundation alignment, task execution audit, goal completion audit, objective conformance, promotion evidence templates, admin i18n, admin UI contracts, admin UI contract drift tests, admin build and diff checks.
   - Foundation alignment also reads `resources/platform-deployment-topology.json` directly and keeps selected scheme A as `single-service-production`; Vercel remains optional admin-static hosting, not the selected default runtime topology.
   - Add real source-writing only after a separate explicit source-writing spec and the `source-writing-codegen-promotion` gate are approved.
   - Task graph node: `source-writing-codegen-promotion` is `implemented` as a skeleton gate; promotion to actual source writing requires an independent source-writing spec, approved promotion review packet, runtime target allowlist, conflict handling, rollback plan, reviewed diff, runtime-root owner approval, target-family test output, structured approval evidence package and `rtk` tests per target family. `resources/platform-task-execution-audit.json` keeps runtime mutation blocked while source writing is disabled, the approval package is blocked or `admin-scaffold-promotion-review.json` remains `manualReview.decision=not-approved`.

4. Production runtime gate
   - Current state: `cmd/platform-api` runs `config.Config.ValidateRuntime()` before capability resolution and store construction.
   - Current state: production runtime mode requires a non-default JWT secret, GORM-backed admin resources, GORM-backed sessions, GORM-backed lifecycle history, Redis cache/invalidation, a non-empty `PLATFORM_CAPABILITIES` list and no `demo-data` capability.
   - Current state: an empty capability list is not a supported way to disable the platform. `config.Load` preserves empty comma-separated entries so malformed capability lists fail validation, and `Registry.ResolveEnabled` rejects nil or empty enabled lists in downstream composition roots.
   - Evidence: `TestValidateRuntimeRejectsProductionWithoutPersistentRuntime`, `TestValidateRuntimeRejectsProductionShortJWTSecret`, `TestValidateRuntimeRejectsProductionNonGORMDrivers`, `TestValidateRuntimeRejectsProductionRedisWithoutAddress`, `TestValidateRuntimeRejectsProductionDemoDataCapability` and `TestValidateRuntimeAcceptsProductionBaseline` cover the production gate and accepted baseline.
   - Keep development mode lightweight for local demos; add stricter provider-specific checks only when a deployment target needs them.

5. Optional system modules
   - Login logs, error logs and versions have normalized GORM resource tables behind the generic resource API. API tokens should stay an optional security capability, and file storage should keep local and S3-compatible storage behind the same object-store port.
   - Task graph node: `file-storage-preview-and-audit-workflow` is `implemented`; `resources/platform-file-storage-experience.json` locks the UI to a generic resource-console extension, rejects standalone file-manager pages, keeps `storage.ObjectStore` unchanged, and requires the existing product-design approval, API/audit tests, i18n checks, build gate and browser screenshots to remain valid before future changes.

## Current P2 Work

1. External business boundary
   - Platform state: `platform-go` no longer ships any concrete `zshenmez` business capability implementation.
   - Current state: `resources/platform-reference-discovery.json` and `resources/platform-reference-coverage.json` keep reference business resources and app routes classified under `external-business-capability`, which means outside the platform foundation.
   - Current state: the default and optional reusable profiles explicitly exclude `external-business-capability`; `scripts/validate-platform-reference-discovery.mjs`, `scripts/validate-platform-reference-coverage.mjs` and `scripts/validate-platform-foundation-alignment.mjs` prevent business-only candidates from being promoted into platform defaults.

2. Data scopes and deny rules
   - Current state: roles declare `dataScope`, optional `dataScopeOrgCodes`, optional `dataScopeAreaCodes` and optional `denyPermissions`; human admin list/query calls enforce tenant/org/area/self data scopes through the generic store, while deny rules override allow rules for action permissions.
   - Current state: the default local Casbin authorizer refreshes after role, user and permission writes; Redis cache mode broadcasts resource invalidation events so peer API instances clear local policy, principal and menu caches.
   - Current state: generic admin resource create/update/delete handlers write `admin_resource.*` audit records with actor, resource and target metadata after successful writes.
   - Current state: resource validators reject role-group permission, membership, inheritance and data-scope fields, and reject personnel/position resources in the default platform contract unless an explicit optional capability owns them.
   - Current state: `policy-review` is available through `enterprise-governance` for policy-change review ledgers. It exposes request, reject, approve and export API contracts: `POST /api/admin/policy-reviews/:id/request`, `POST /api/admin/policy-reviews/:id/reject`, `POST /api/admin/policy-reviews/:id/approve` and `GET /api/admin/policy-reviews/export`. Approve applies role permission, deny or data-scope changes with policy-review audit records and cache invalidation; request/reject/export record policy-review audit events without putting approval semantics into role groups.
   - Current state: `PolicyReviewConsole` is now available as a platform-governance custom console when `/policy-reviews` is enabled. It covers queue review, status filtering, request, approve, reject, evidence export, change summary and audit trail through shared API client wrappers and platform UI primitives.
   - Task graph node: `policy-review-custom-ui` is `implemented`; browser evidence covers desktop queue, reject modal, audit trail and narrow responsive approval views while API behavior tests, i18n, UI contracts, build and task-graph validation stay green.
   - Do not introduce role inheritance or role templates without tests showing policy precedence.

3. Refine custom panels and actions
   - Current state: route ownership, read guards and generic CRUD are routed through Refine resource metadata while `GenericResourceConsole` keeps the schema-driven UI contract. Resource schemas support metadata-driven `actions` and `panels`: row actions render through an overflow menu, batch actions render in the selection command bar, and detail drawers render tabbed panels for fields, permissions and capability extensions. The `menus` resource declares a non-destructive sample action and audit panel, and validators reject duplicate action/panel keys, unsafe component paths, unsupported kinds and dangerous actions without confirmation.
   - Implemented proof: the shared frontend executor calls schema-declared routed actions through the admin API, honors confirmation metadata and preserves localized failure states. Concrete business actions must be provided by downstream capability routes; the platform UI does not ship product-specific action handlers.
   - Task graph node: `refine-custom-panels-and-actions` is `implemented`; richer approval/file/business panels remain follow-on capability-owned UI work and must still go through product design, i18n, browser evidence and capability-local tests.

## Post-Foundation UI Optimization State

The original foundation baseline remains closed at 37 implemented nodes. `runtime-security-containment` and `admin-watermark-export-governance` are `implemented`, expanding the active graph to `45 total / 39 implemented / 6 controlled unfinished`; the remaining work covers sensitive-data runtime and migration, open-source portability, public community/docs work and GitHub publication. External production promotion remains `not-approved`; runtime mutation, refresh-token-family default runtime and source writing remain disabled.

1. Admin UI system-quality hardening
   - Requested aid: `ui-ux-pro-max`.
   - Current state: `admin-ui-system-quality-hardening` is `implemented` after `production-persistence-correctness`, `admin-ui-shell-and-list-components` and `visual-product-design-qa`.
   - Behavior: widths below 1024px use the compact command/context shell; widths at or above 1024px preserve the desktop shell. Compact-shell controls use 44px minimum targets; at mobile resource widths the toolbar, search, pagination quick-jumper and settings Drawer controls also measure at least 44x44px. Route changes move focus to the stable main region; the create modal settles focus on its first enabled editable field, closes through Escape and restores the create trigger.
   - Data and recovery: mobile cards remain authoritative through 767px; 768/1024/1280/1440 tables expose schema-order priority tiers while keeping actions discoverable. Stored-session 401 responses clear the token once and return localized sign-in recovery instead of raw `unauthorized` copy.
   - Completion evidence: fresh 375/390/768/1024/1280/1440 captures, create-modal and settings-drawer states, keyboard/focus checks, target-size and stable-state overflow measurements, computed reduced-motion checks, clean browser console, admin UI contract tests, i18n and build. `tmp/product-design/p1-admin-ui-hardening-20260711/10-stale-session-390x844.png` records “会话已过期，请重新登录。” without raw backend copy.

2. Brand entry and public-surface visual redesign
   - Requested aid: `design-taste-frontend`.
   - Proposed priority: P2 deferred until the repository has a real landing, portfolio, marketing or approved login-brand brief.
   - Scope boundary: login, branding, public documentation and future marketing surfaces only. CRUD consoles, policy review, file management and repeated operational workflows retain the quiet, dense admin design language.
   - Activation gate: approved product positioning, brand assets, primary action and three bounded visual directions reviewed through `superpowers:brainstorming` and Product Design.
   - Completion evidence: real product/brand media where appropriate, mobile/desktop screenshots, contrast review and no regression to login completion speed.

Detailed scope and scheduling: `docs/platform-ui-optimization-assessment.md`.

## Execution Rules

- Work P0 before P1 unless a P1 task is required to unblock P0.
- Check `resources/platform-foundation-task-graph.json` before starting a new node; if dependencies, phase order or resource locks conflict, update the graph and validator evidence in the same node.
- Every shared component or API change must update i18n/docs/tests in the same node.
- Every task node ends with relevant tests, `git diff --check`, and CodeGraph sync when structure changed.
- Product-design work uses the confirmed componentized admin direction and requires browser evidence for visible UI changes.
- `preview`, `planned` and `deferred` task nodes must declare localized `statusReason`, localized `completionGate` and at least one `evidence.docs` path. A non-implemented node is not final delivery evidence; it is a controlled state with a documented reason and promotion gate.
