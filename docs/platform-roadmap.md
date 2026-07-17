# Platform Foundation Roadmap

Date: 2026-07-04
Last updated: 2026-07-16

## Goal

`platform-go` should become a reusable operations-platform foundation. Business projects should be able to enable or disable platform capabilities, add business capabilities, and consume common services through stable APIs without coupling to a specific business domain.

The platform is currently a compiled modular monolith, not a runtime plugin marketplace. Capabilities are Go packages plus frontend contracts that are registered through manifests and enabled by configuration.

## Current Baseline

Recorded task-graph release lane: v0.1.0 release blockers: `github-release-publication`.

Active v0.1.0 release blockers: none.
Post-release optional deferred nodes: `multi-datasource-contract-and-runtime`, `tenant-placement-and-request-routing`, `datasource-read-write-routing`, `sharding-and-tenant-migration`, `federated-read-query`, `xa-optional-adapter`, `database-certification-matrix`, `transactional-outbox-and-one-mq-adapter`, `asynchronous-search-projection`.
Current Remaining Node Order

`github-release-publication` is `implemented`. v0.1.0 is published and verified: the formal Git tag, GitHub Release, candidate CI and Pages evidence all resolve to the release commit.

There is no whole-node parallel batch; each release gate follows the declared
dependency and evidence order.

v0.1.0 manuals and compatibility claims must state the current one-datasource, one-native-transaction boundary.

> **Status:** Completed.

- [x] **Step 1** Publish the public contracts and release documentation.

Task 7: complete. Task 4: complete. The organization-user-admin-experience contract is implemented.


Implemented foundation slices:

- capability registry, dependency resolution, manifest validation and lifecycle declarations;
- capability-declared admin resources and menus;
- capability-declared app API route contracts with `/api/app/` domain, auth mode, `app:` permission validation, runtime `AppRouteRegistration` handler binding and generated `app-route-contract.json` export for generators;
- schema-driven generic admin resources with fields, search fields, form rules and permission codes;
- dynamic RBAC menu filtering through `user -> roles -> permissions -> menus/resources/actions`;
- role allow and deny permissions editable through the generic `roles` resource, with deny rules taking precedence over wildcard or exact allow rules;
- permission catalog generated from enabled capability admin declarations;
- backend permission checks for generic resource schema, list, create, update and delete actions;
- auth provider discovery with demo login, a configurable WeChat miniapp adapter, app identity resolver seam and generic app identity binding resource;
- JWT admin bearer tokens backed by server-issued sessions with TTL, sliding renewal and revoke support;
- session persistence modes: memory, JSON file and GORM-backed MySQL/PostgreSQL/SQLite;
- admin resource persistence modes: memory, JSON file and GORM-backed MySQL/PostgreSQL/SQLite;
- production persistence correctness: GORM sessions use record-scoped operations, admin resource snapshots use revision CAS, and peer admin invalidation reloads independent Stores before clearing derived caches;
- lifecycle history modes: memory, JSON file and GORM-backed MySQL/PostgreSQL/SQLite;
- optional cache modes: noop, memory and Redis;
- Redis-ready caching for branding, auth provider discovery, resource schemas, permission catalog, dynamic admin menus and principal permission expansion with write invalidation;
- Redis pub/sub invalidation bus for cross-instance policy, principal and menu cache refresh when Redis cache mode is enabled;
- distributed session issue, renewal and revoke convergence for repository-backed session stores through the same invalidation bus;
- cache stats endpoint for hit, miss, write, delete and error counters;
- GORM storage opener seam with MySQL, PostgreSQL and SQLite drivers available;
- GORM-backed admin resource repository wired into startup for supported drivers;
- normalized GORM tables for standard platform and operations resources: tenants, org units, users, user-role bindings, role groups, roles, role-permission bindings, permissions, menus, area codes, audit logs, login logs, error logs and versions;
- generic admin resource create, update and delete handlers record audit events with actor, resource and target metadata while skipping audit resources to avoid recursion;
- Casbin-backed HTTP authorization generated from dynamic user/role resources, with a local policy authorizer cache invalidated after role, user and permission writes;
- JWT admin HTTP auth with session-backed TTL and revocation;
- Refine runtime wrapper with data, auth, access-control, router and notification providers; schema-driven resource list/create/update/delete now run through Refine CRUD hooks while keeping the platform schema UI adapter;
- admin resource generation loop that merges `resources/admin-resources.json` with enabled `Manifest.Admin.Resources`, then emits contract, OpenAPI output, codegen preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft, scaffold promotion review packet and validation scripts;
- engineering capability coverage matrix in `resources/platform-engineering-capabilities.json`, with a validator tying dynamic resources, dynamic menus, permission codes, organization/role-group/area governance, capability contract governance, schema/form generation, OpenAPI, safe scaffold generation, source-writing readiness, system management, App routes, App client API boundaries, production runtime gating, production auth hardening, cache invalidation and deployment topology to real source and generated artifacts;
- cache invalidation contract in `resources/platform-cache-invalidation.json`, with a validator tying cache targets, resource invalidation rules, Redis pub/sub channel, session reload behavior and no-cache policy to runtime code, docs and tests;
- task execution audit in `resources/platform-task-execution-audit.json`, with a validator that keeps task evidence, dependency/resource-lock conflicts, visual gates and future runtime-promotion blockers visible after foundation closeout;
- goal completion audit in `resources/platform-goal-completion-audit.json`, with a validator that keeps the active foundation goal `complete` only when all task nodes are implemented while still requiring non-mutating production/source-writing promotion evidence before runtime mutation;
- node closeout audit in `resources/platform-node-closeout-audit.json`, with a validator that requires every implemented task node to carry focused cleanup evidence, docs/tests/validator evidence, resource-lock review, objective-conflict review and visual design-gate evidence where required; `neat-freak` is reserved for phase, major-task and release closeout;
- objective conformance gate in `resources/platform-objective-conformance.json`, with a validator tying approved stack, reference-only `zshenmez` usage, capability-contract governance, blocker nodes, visual gate order, Vercel boundaries, production preflight and node closeout cleanup to the persistent platform foundation objective;
- draft-only promotion evidence templates in `resources/generated/platform-promotion-evidence-templates.json`, with validators that keep production-auth and source-writing approval evidence packages aligned to their schemas and validate external submitted packages without turning templates or package checks into approval evidence;
- capability profile gate for `minimal-admin`, `platform-default`, `platform-app-ready`, `platform-personnel-ready`, `platform-notification-ready`, `platform-job-ready` and `enterprise-governance`, including default-profile drift checks, optional personnel/notification/job isolation and business capability leakage prevention;
- app route manifest loop with `cmd/platform-contracts app-routes`, generated `resources/generated/app-route-contract.json`, `openapi.app.json`, `app-codegen-preview.json` and `resources/platform-app-client-api-boundary.json` so downstream clients consume App routes through generated clients or request/upload ports;
- Gin-served `GET /api/openapi.json` backed by the generated admin OpenAPI file and `admin:api-docs:read`;
- admin i18n validation gate for shared component text;
- branding config API plus capability-owned dictionaries, dictionary parameters, parameters, branding and settings resources;
- capability-declared demo data APIs and admin console;
- optional `personnel` capability with `personnel-profiles`, `positions`, `position-assignments`, shared tenant/org/area/user relations and runtime readiness validation through the `platform-personnel-ready` profile;
- optional `notification` capability with `notification-templates`, `notifications`, `notification-deliveries` and shared tenant/user/audit relations through the `platform-notification-ready` profile;
- optional `job` capability with `job-definitions`, `job-runs`, `job-run-attempts` and shared tenant/user/audit relations through the `platform-job-ready` profile;
- Ant Design based admin shell with themes, layouts, tabs, i18n and reusable platform UI primitives;
- implemented Admin UI system-quality hardening with a compact two-tier shell below 1024px, 44px compact-shell/resource-toolbar/search/pagination/settings targets at mobile widths, skip/route focus, settled modal first-field focus with Escape/trigger restoration, localized icon labels, schema-order table priority, localized stale-session recovery and computed reduced-motion handling;
- implemented the Admin-only OIDC runtime, binding, provisioning and login contracts, then closed `production-admin-oidc-auth` with Task 8 production-like provider, six-viewport browser and neat-freak cleanup evidence;
- reusable dashboard home, data table filters, date/numeric range filters, active filter state, column settings, compact centered integrated pagination, settings drawer, exact `1/4/9/16` screen watermark counts, independent screen/export scopes, visual-aid switch, sidebar collapse and top-layout navigation.
- isolated app security-domain runtime with guest-first `POST /api/app/auth/login`, optional configured provider resolution through `httpapi.AppIdentityResolver`, generic `app-identities` provider-subject-hash binding, optional `app-phone` verification/binding routes, `GET /api/app/session/current`, `POST /api/app/auth/logout`, `tokenType=app` JWT enforcement and route-level `app:` permission checks for registered business handlers.

Important correction:

- The earlier implementation had drifted from the intended `jiedanshi/platform` reference stack.
- The target stack is Gin + GORM + Casbin + JWT on the backend and Refine + React + Ant Design on the admin frontend.
- Main stack alignment has been corrected in thin slices; production token-rotation policy is contract-gated, and the reusable foundation now marks `production-auth-provider-hardening` plus the source-writing generator skeleton gate as implemented.
- The original foundation baseline is closed at 37 implemented nodes. The completed continuation nodes, including `platform-service-contract-standard`, `persisted-query-command-object-runtime`, `integration-ports-disabled-default`, unified error-code governance, the organization/RBAC/menu contract gate, the organization role-pool backend/migration runtime, `organization-user-admin-experience`, `role-tree-and-authorization-entry`, `menu-tree-and-button-permission-configuration`, the organization E2E gate and `github-release-publication`, are implemented, so the active completion program is now `67 total / 58 implemented / 9 controlled unfinished` and the current goal remains `not-complete-controlled`.
- `runtime-security-containment`, `sensitive-data-protection-runtime` and `sensitive-data-historical-migration` remain `implemented`; the Query/Command closeout reuses their authorization, protected-storage and maintenance-only boundaries without reopening them.
- The persistent full-scope unfinished inventory contains only the nine post-release optional deferred datasource/runtime nodes in task-graph order. `open-source-portability`, `public-docs-community`, `public-docs-site` and `github-release-publication` are `implemented`; v0.1.0 Tag, Release, CI and Pages evidence are verified.
- `mask-strategy-runtime` is closed with manifest-driven versioned strategies, fail-closed encrypted projection, single-projection HTTP boundaries, Admin encrypted-edit safety and contract/OpenAPI/TypeScript propagation. `sensitive-data-reveal-step-up` is closed with manifest policy, dedicated permission, OIDC/SMS factor orchestration, short-lived single-use grants, rate limits, append-only audit and an accessible expiring modal.
- Historical migration and lifecycle retention keep their maintenance-only boundaries. The service contract standard, persisted Query/Command runtime, unified error-code governance, organization/RBAC/menu design and runtime, organization E2E, portability, public documentation and formal GitHub publication nodes are closed with machine gates. The nine deferred datasource/routing/sharding/federation/XA/certification/outbox/search nodes have no implementation closeout. MySQL/PostgreSQL promotion still requires real integration certification evidence, SQLite remains local development/test rehearsal only, Oracle and KingbaseES remain uncertified, federation is read-only and controlled, XA is optional and default-off, and messaging/search integrations remain disabled by default. Production-auth and source-writing promotion remain intentionally non-mutating and `not-approved`.

## Menu And Permission Decision

The legacy serving model remains dynamic and permission-derived. The target runtime now has independent page-only role-menu bindings, revision-aware resolution, the dedicated menu UI and completed organization/menu E2E evidence, but production serving and write cutover gates remain closed pending explicit rollout approval.

```text
user -> roles -> permissions / denyPermissions -> menus/resources/actions
```

Reasons:

- menus, buttons, resource actions and future APIs can share one permission-code model;
- capabilities can declare permission codes without coupling to role names;
- business modules can add menus without editing platform core;
- frontend consumes `GET /api/admin/menus` and current-session permissions / denied permissions;
- the persistence implementation can later move to normalized user-role, role-permission and menu tables without changing the admin shell contract.

Current tradeoff:

- this is enough for reusable platform RBAC, dynamic menus, explicit deny rules and data-scope filtering;
- it intentionally avoids role inheritance in the base model, while the optional `policy-review` capability provides an isolated review ledger plus request, reject, approve and export APIs through the `enterprise-governance` profile for deployments that need controlled policy changes.

## Next Implementation Phases

### Phase 1: Stabilize Foundation Contracts

Priority: high.

P0 stack alignment:

- Done: add GORM dependencies and storage opener seam for MySQL, PostgreSQL and SQLite.
- Done: add GORM-backed admin resource repository behind the existing repository port.
- Done: add GORM-backed session repository behind the existing session repository port.
- Done: replace lifecycle snapshot runtime with a GORM-backed repository.
- Done: normalize tenants, org units, users, role groups, roles, permissions, menus and area codes behind the GORM admin resource repository while preserving the resource snapshot API.
- Done: keep dictionaries, dictionary parameters, parameters, branding and settings as capability-owned configuration resources behind the generic admin resource API; GORM stores them through the generic records table unless a future deployment needs dedicated normalized tables.
- Done: add machine-validated capability profiles so capability composition changes are checked against dependency resolution, default runtime capabilities and business-boundary rules.
- Done: add machine-validated capability contracts so built-in, optional, local-demo and external-business capabilities are classified separately from runtime `capability.Manifest` and checked against profile/default policy plus audited manifest surfaces.
- Done: normalize audit logs, login logs, error logs and versions behind the GORM admin resource repository while preserving the resource snapshot API.
- Done: record generic admin resource create/update/delete operations into the `audit-logs` resource after successful writes, and expose `audit-logs` through a structured read-only query schema.
- Done: add Casbin and route backend permission enforcement through dynamic role policies.
- Done: persist role-permission and user-role data through normalized GORM tables consumed by Casbin policy generation.
- Done: add role `denyPermissions` with deny-overrides-allow precedence in Casbin checks, backend menu filtering and frontend access controls.
- Done: cache the default Casbin authorizer locally and refresh it after role, user and permission writes, while also clearing principal and dynamic-menu caches.
- Done: add Redis-backed distributed policy/cache invalidation events so peer API instances refresh local policy, principal and menu caches after writes.
- Done: implement `production-persistence-correctness`: GORM session writes are record-scoped, GORM admin resource saves reject stale revisions through CAS, and peer admin events reload independent Stores before derived-cache invalidation. File and legacy SQL adapters remain compatibility modes, while Redis Pub/Sub remains best-effort convergence rather than a durable consistency log.
- Done: add optional `policy-review` capability, `enterprise-governance` profile and controlled `request`, `reject`, `approve` and `export` APIs so approval ledgers stay outside default RBAC and role groups. Approve write-through applies role permission, deny or data-scope changes and records policy-review audit events.
- Done: add the product-designed `PolicyReviewConsole` for deployments that enable `/policy-reviews`. It covers queue review, status filtering, request, approve, reject, evidence export, change summary and audit trail through shared platform UI primitives; business approval workflows still belong in downstream capabilities.
- Done: add JWT admin/app token service seam.
- Done: replace opaque HTTP session tokens with JWT admin bearer tokens while preserving server-side revocation semantics.
- Done: add admin sliding session renewal through `POST /api/auth/refresh`, reusing the authoritative server-side session and returning a newly signed JWT.
- Done: close `production-auth-provider-hardening` as an implemented foundation gate backed by `resources/platform-production-auth-hardening.json`, `resources/platform-refresh-token-family-promotion.json`, `resources/generated/production-auth-promotion-review.json`, `scripts/validate-platform-production-auth-hardening.mjs`, `scripts/validate-platform-refresh-token-family-promotion.mjs` and `scripts/generate-production-auth-promotion-review.mjs`; it covers JWT/session credentials, provider adapter controls, provider runtime policy, Provider Promotion Matrix with manifest-provider coverage, token-rotation evidence, structured promotion approval evidence schema, operations-plan schema propagation, a non-mutating not-approved promotion review packet and implemented-disabled refresh-token-family runtime artifacts.
- Done: add the production session-policy specification and disabled refresh-token-family runtime slice. The current sliding-renewal credential is authoritative through the server-side session. It is not a refresh-token-family model. The optional runtime slice provides hashed token-family storage, rotation/reuse detection and audit redaction without binding that behavior to the default refresh endpoint.
- Done: add `pgo_` API token issue/update/revoke plus scoped Bearer authorization for protected platform APIs.
- Done: add isolated app auth runtime with `tokenType=app` JWTs, app current-session and app logout.
- Done: add app identity resolver seam and generic app identity binding persistence for configured app login providers without exposing raw OpenID/UnionID in responses, audit records or generic resource rows.
- Done: add optional `app-phone` capability with app-session-only phone verification and phone binding APIs, `app-phone-verifications` and `app-phone-bindings` admin resources, phone hashing, phone masking, verification-code hashing and local rolling-window verification abuse guard.
- Done: add capability manifest contract validation, stable contract export, generated artifact and freshness gate for app API route declarations.
- Done: add configurable WeChat miniapp `jscode2session` adapter behind `httpapi.AppIdentityResolver`.
- Done: harden provider runtime policy gates: adapter registration remains manifest-declared and composition-root injected, unconfigured providers are rejected by default, provider subjects stay hash-and-mask only, and provider promotion requires tests for unconfigured-provider rejection, subject redaction, configured-provider-only login and provider error normalization.
- Next: enable or bind independent refresh-token rotation, business API adoption, additional provider adapters and normalized binding storage hardening only when production auth requirements exceed the current gate and the production approval package is complete.
- Done: add Refine runtime wrapper with `dataProvider`, `authProvider`, `accessControlProvider`, router provider and dynamic resource metadata.
- Done: move schema-driven resource route pages into `platform/refine/ResourceRoutePage` with `useResourceParams`, `useCan` and a build-time Refine runtime validator.
- Done: move schema-driven resource list/create/update/delete through Refine `useList`, `useCreate`, `useUpdate` and `useDelete`, with safe query `keywords` and structured `conditions` carried through `dataProvider` meta.
- Done: promote the reference admin API boundary and query-security checks into a platform foundation gate. `resources/platform-admin-api-boundary.json` and `scripts/validate-platform-admin-api-boundary.mjs` keep admin source behind the shared API client, block App API calls from admin code, forbid query-string collection filters and verify structured OpenAPI query allowlists.
- Done: promote the reference App client API boundary into a platform foundation gate. `resources/platform-app-client-api-boundary.json` and `scripts/validate-platform-app-client-api-boundary.mjs` keep downstream App/H5/mini-program clients behind generated App clients or request/upload ports, forbid page-level request/upload/Authorization wiring and keep generated app clients tied to `app-route-contract.json`, `openapi.app.json` and `app-codegen-preview.json`. The base `file-storage` capability now contributes session-scoped App file upload/content routes (`POST /api/app/files`, `GET /api/app/files/:id/content`) through the same contract, so C-end clients use `appUpload` without page-level upload calls.
- Done: keep richer form layout and bespoke custom panels on platform wrappers and Refine page/form hooks only where they reduce custom code without weakening the platform schema contract. `PlatformResourceForm` now owns the common schema form surface with controlled source-level slots, controlled runtime slot descriptors and `side-detail-preview`; schema-declared row actions execute through the shared admin action executor.
- Done: port the `jiedanshi/platform/resources` manifest loop and extend it with enabled capability admin resources: `admin-resources.json`, `admin-capability-resource-contract.json`, merged generated contract, codegen preview, scaffold dry-run safety plan, generated scaffold files, scaffold draft, scaffold promotion review packet and validator.
- Keep source-writing code generation and arbitrary form slots deferred. Controlled runtime form slot descriptors are implemented through schema metadata and frontend registries, but arbitrary backend component names, component paths, raw scripts and source-writing generators remain forbidden. Safe preview/scaffold generation is implemented and non-mutating; any future generated runtime root must appear in `runtimeTargetPolicy`; proposed roots require a separate architecture/source-writing spec before promotion.

- Lock capability manifest conventions for admin resources, app API route contracts, auth providers, demo data, lifecycle steps and future service registrations.
- Keep app API route declarations under `Manifest.App.Routes`; handlers attach through the neutral `approute.Registration` contract and are passed into `httpapi.ServerOptions`, but routes are exposed only when the enabled manifest declares the same method/path. The route contract must stay in the manifest and flow through `AppRouteContracts` plus `cmd/platform-contracts app-routes` for generators.
- Keep the public capability development guide current with naming, permission-code, route, field and schema rules.
- Keep contract tests for disabled capabilities: disabled resources, menus, demo data and auth providers must not register.
- Keep storage adapters behind repository ports; avoid leaking GORM or `database/sql` details into HTTP handlers or frontend code.

Acceptance:

- a new capability can register one admin resource, menu, permissions and demo data without editing platform core;
- disabling that capability removes its admin surface from APIs and UI;
- tests prove the contract.

### Phase 2: Resource Schema And Admin Component Standard

Priority: high.

- Extend resource schema for list columns, form layout, field widgets, validation, search presets, batch actions and row actions.
- Keep AntD as the base UI framework and expose platform wrappers for high-frequency UI: page, toolbar, table, form, modal, drawer, button, tag, alert, empty state and confirm.
- Support slots and prop overrides so business pages can customize without forking common components.
- Make permission checks consistent across backend actions and frontend controls.
- Keep i18n as a hard gate: platform components use dictionary keys, and resource/plugin/dashboard data uses localized data contracts.
- Keep SQL-like list search as a safe structured DSL or JSON query model; never turn user text into raw SQL.
- Use `POST /api/admin/resources/:resource/query` for generic resource pagination, filtering and sorting. The current store evaluates the structured query in memory; future GORM pushdown must reuse the same query model with parameterized predicates.
- Keep data-entry resources ready for content localization through localized fields or translation tables; platform resource names and descriptions default to localized values.
- Keep dropdown panels, pagination, drawers, buttons, prompts and table cells as reusable platform primitives so style changes stay centralized.
- Keep table pagination, column settings, advanced filters, active filter counts, date/numeric range filters, batch actions, row actions and overflow tooltips inside `PlatformDataTable` unless a resource explicitly opts out. Pagination must stay visually centered, compact and content-after-list on mobile.
- Keep account/system settings in `SystemSettingsDrawer`, including theme, layout, density, work tabs, sidebar collapse, watermark, visual aid, page transition, import/export and reset.
- Keep environment and tenant selectors as global context controls in the shell; they are read-only placeholders until multi-environment or multi-tenant switching is wired.
- Keep generated admin OpenAPI and codegen previews normalized to the Gin generic resource route shape, for example `/api/admin/resources/tenants/query`. Keep App OpenAPI and preview artifacts generated from `Manifest.App.Routes` and separated from the admin security domain.

Acceptance:

- standard CRUD resources are mostly schema-driven;
- complex resources can still mount custom actions or custom panels through extension points;
- one style/token change applies consistently across platform pages.

### Post-Foundation UI Optimization State

Status: P1 and the public documentation surface are implemented. Full assessment: `docs/platform-ui-optimization-assessment.md`.

- Done, P1: `admin-ui-system-quality-hardening` implements default focus visibility, skip/route focus, explicit localized icon labels, 44px mobile shell/resource controls, a two-tier shell below 1024px, settled modal focus lifecycle, schema-order table prioritization, localized stale-session recovery, computed reduced-motion behavior and browser/contract coverage across 375, 390, 768, 1024, 1280 and 1440 widths with no stable-state page overflow or new application console errors.
- Done, completion program: `admin-watermark-export-governance` is `implemented`. The normalized settings contract preserves legacy boolean configuration, supports independent screen/export scopes, renders exactly `1/4/9/16` inert full-viewport screen marks over navigation, data surfaces and overlays, reflows narrow sixteen-mark layouts to `2x8`, and adds structured provenance only to Policy Review JSON exports. Product Design, `ui-ux-pro-max`, bilingual contract/build checks, responsive browser evidence, dark-mode coverage and a clean current-run console are recorded in the task graph and node closeout audit.
- Done, completion program: `sensitive-data-reveal-step-up` is `implemented`. Manifest-declared encrypted fields can require a dedicated permission and `anyOf`/`allOf` policy over OIDC reauthentication and Admin SMS OTP; grants are short-lived and single-use, audit is append-only, failed verification returns `422` without clearing the Admin session, and plaintext is confined to the expiring modal.
- Done, P2 public surface: `design-taste-frontend` informed the bilingual documentation landing page while Docusaurus owns shared navigation, locale switching, metadata and the footer. This art direction remains limited to public and brand surfaces; repeated CRUD and governance workflows stay on the Admin product UI system.
- Governance: the 37-node foundation baseline and completed continuation nodes remain closed, and the graph is `67 total / 58 implemented / 9 controlled unfinished` with status `not-complete-controlled`. Open-source portability, public docs/community, the documentation site and publication governance are `implemented`; the nine post-release optional nodes remain `deferred`. Production promotion remains `not-approved`; target menu serving, role-menu migration writes, runtime mutation and refresh-token-family default runtime remain disabled.

### Phase 3: Production Auth And Identity

Priority: high.

- Done: implement WeChat miniapp code exchange behind `httpapi.AppIdentityResolver`.
- Harden login identity records separate from platform users; the generic `app-identities`, `app-phone-verifications` and `app-phone-bindings` resources already provide the reusable hash-binding baseline.
- Replace the local/demo phone `debugCode` delivery with a production SMS adapter when a target product needs real phone verification. The HTTP contract, rolling-window semantics and stored hash/masked records should remain stable; multi-instance deployments can move the guard behind Redis or another distributed adapter.
- Done: add admin sliding renewal for long admin sessions through `POST /api/auth/refresh`.
- Done: add a production auth hardening contract and validator so refresh-token-family production enablement cannot be claimed while the default runtime still uses sliding session renewal and approval evidence is missing.
- Enable independent refresh-token rotation only when a target product requires offline renewal, reuse detection or stricter token family policies and the approval package is complete.
- Done: add repository-backed session reload through the invalidation bus so multi-instance deployments converge session issue, renewal and revoke events when they share a session repository.
- Add stronger token-family revocation, reuse detection or centralized session policy only when a target product requires those production semantics.
- Keep demo provider and demo datasets as development/demo surfaces. Production runtime now rejects `demo-data` in `PLATFORM_CAPABILITIES` and requires `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`; target deployments should build production profiles without sample data loaders or demo login.

Acceptance:

- WeChat login can resolve or create an app identity through a provider adapter;
- login, logout, current-session, menu and resource APIs remain unchanged for the admin shell;
- audit logs do not expose secrets or raw tokens.

### Phase 4: Persistence Hardening

Priority: medium.

- Replace snapshot-oriented SQL repositories where necessary with normalized tables.
- Add migrations for session, lifecycle and normalized admin resource tables, including tenants, org units, users, role groups, roles, permissions, menus, area codes and operations resources.
- Extend the platform cache port beyond current stable admin reads only where invalidation is clear.
- Define invalidation on successful writes to any future capability-derived mutable resources.
- Data-scope, deny declarations, generic resource write audit, optional policy-review request/reject/approve/export APIs and the implemented `PolicyReviewConsole` exist. Add migration workflows only when production deployments need controlled policy migration or rollout tooling beyond the current approval ledger.
- Done: add machine-checkable production operation policies for `config-backup-export`, `config-import-restore`, `database-migration` and `token-rotation` in the production readiness contract. Default runtime import/restore remains intentionally non-destructive until a reviewed dry-run/diff/rollback workflow exists.
- Done: generate `resources/generated/platform-operations-plan.json` as a non-mutating dry-run plan from the production readiness policies, and generate `resources/generated/production-auth-promotion-review.json` as the narrower production-auth review packet for token-rotation readiness. These artifacts are review evidence only; real production restore, migration execution or token-rotation actions remain separate reviewed capabilities.
- Done: include `task-execution-audit` in production preflight so remaining preview/planned/deferred nodes stay visible during release checks instead of being silently promoted.
- Done: move policy-level required preflight gates into `resources/platform-production-readiness.json` and surface each policy's required and missing gates in the generated operations plan instead of keeping that logic hidden in validator constants.

Acceptance:

- file mode remains useful for local demos;
- database-backed mode is suitable for multi-instance and production-like environments;
- `PLATFORM_RUNTIME_ENV=production` fails fast unless JWT secret, GORM-backed admin resources, GORM-backed sessions, GORM-backed lifecycle history, Redis cache/invalidation and disabled demo auth are configured;
- Redis can be enabled or disabled by configuration without changing business capability code;
- role permission changes survive restart and are independently auditable.

### Phase 5: Optional Platform Capabilities

Priority: medium.

- File storage: `file-storage` manifest, local and S3-compatible object adapters, admin upload/content/delete API, App session-scoped upload/content API, generic metadata resource, backend file-operation audit contract and generic admin resource-console experience are present. `resources/platform-file-storage-experience.json` keeps upload, authenticated preview, download, audit visualization, optional `/audit-logs` fallback and mobile cards guarded by product-design, i18n, UI contract, build and browser evidence; standalone file-manager pages, business attachment workflows and storage-port changes remain rejected until a separate approved design says otherwise.
- Notification: in-app notifications first, external delivery later.
- Jobs: `job-definitions`, `job-runs` and `job-run-attempts` are present as an optional platform capability for scheduler-ready contracts; worker execution, distributed locks and retry engines remain follow-up slices.
- OpenAPI: generated route metadata export, `GET /api/openapi.json` and the admin API Docs page are present. Generated admin resource paths are normalized to the generic Gin resource engine and include enabled capability admin resources that are not already provided by the platform base. App route contracts now generate `openapi.app.json` and `app-codegen-preview.json`; runtime serving remains admin-only until App API docs are intentionally exposed. The page uses `PlatformDataTable` for search, filters, sorting, column settings and integrated pagination.
- Code generation: schema-to-scaffold previews, a machine-readable dry-run safety plan, generated scaffold files under `resources/generated/scaffold/` and a non-mutating promotion review packet that do not overwrite runtime source by default.
- Engineering coverage: `rtk node scripts/validate-platform-engineering-capabilities.mjs` checks the reusable engineering-capability matrix and is included in the default admin resource validation gate.

Acceptance:

- optional capabilities can be removed without breaking the kernel;
- each capability owns its routes, admin resources, permissions and docs.

### Phase 6: External Business Boundary

Priority: medium.

- The `zshenmez` project remains an external reference source for capability coverage only; it is not a migration target for platform-go.
- `resources/platform-reference-discovery.json` records inspected reference source sets and classifies each candidate as foundation, extension, business or deferred before it can influence platform coverage.
- `resources/platform-reference-coverage.json` keeps role applications, public profiles, portfolio works, favorites, task ledgers, transfers, fulfillment confirmations and support tickets classified as business-only under the abstract `external-business-capability` owner.
- `resources/platform-capability-profiles.json` has no executable business profile; default and optional reusable profiles explicitly exclude `external-business-capability`.
- Platform core provides neutral seams: capability manifests, admin resources, permission codes, app/admin route registration, generated OpenAPI/resource contracts, generic action executor, audit hooks and ownership relation fields. Concrete business state machines, stores, UI panels and route handlers belong to downstream packages.

Acceptance:

- platform core remains business-neutral;
- reference business workflows are classified and blocked from `platform-default`;
- future projects can reuse the platform without inheriting `zshenmez` business code.

## Operating Rules

- Platform core owns contracts and shared runtime; business capabilities own business outcomes.
- Common capabilities expose stable APIs or repository/service ports, not raw implementation details.
- Menus are visibility hints, not security boundaries; backend permission checks remain authoritative.
- Generated code is preview/scaffold by default; source overwrite must be explicit, backed by the scaffold dry-run safety plan, source-writing readiness contract and a separate source-writing specification.
- UI common components should have sane defaults plus slots/props for customization.
- Every reusable capability must include docs, tests and a clear disable behavior.
