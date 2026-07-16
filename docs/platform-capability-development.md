# Platform Capability Development Guide

Date: 2026-07-04

## Purpose

This guide defines how platform and business capabilities should be added to `platform-go`.

The goal is a reusable foundation: a capability can be enabled, disabled, tested and documented without editing platform core or coupling generic services to one business domain.

## Capability Shape

Each capability is a code-backed package that exposes one `capability.Manifest`.

The manifest is the only registration entrypoint for common platform surface:

- dependencies;
- lifecycle migrations and seeds;
- admin resources;
- menu entries;
- permission prefixes;
- app API route declarations;
- auth providers;
- demo data sets;
- future service registrations.

Do not register menus, resources, providers or demo data through global init side effects.

Platform packages under `internal/platform/**` must not import a concrete business package. Concrete business capability packages live outside `platform-go` or in downstream composition roots, then attach through `capability.Manifest`, admin resource schemas, app/admin route registrations and generated contracts. This repository must not bundle a product-specific business implementation.

## Naming Rules

Capability IDs:

- use lowercase kebab-case: `wechat-login`, `file-storage`, `external-ordering`;
- keep platform capabilities business-neutral;
- use business prefixes only for application capabilities.

Resource keys:

- use lowercase kebab-case plural nouns: `feedback-tickets`, `role-applications`;
- stay stable after release because URLs, permission codes and persisted resource records depend on them.

Admin routes:

- start with `/`;
- usually match the resource key: `/feedback-tickets`;
- do not include tenant, user or environment values in the route.

App API routes:

- start with `/api/app/`;
- keep query strings and fragments out of the route path; put filters and route-specific input in the request body or business payload;
- declare an explicit auth mode: `public` or `session`;
- use `app:<domain>:<action>` permission codes when a route needs business app authorization;
- never use `admin:` permission codes in app routes.

Permission prefixes:

- use `admin:<singular-resource>` for admin resources;
- generated actions are `read`, `create`, `update`, `delete`;
- examples: `admin:tenant:read`, `admin:feedback-ticket:update`.

Provider IDs:

- use stable lowercase IDs such as `demo`, `wechat`, `oidc`, `password`;
- provider-specific secrets must stay in configuration, not in manifests.

## Manifest Example

```go
func Manifest() capability.Manifest {
    return capability.Manifest{
        ID:           "feedback",
        Name:         "Feedback",
        Version:      "0.1.0",
        Dependencies: []capability.ID{"dictionary", "tenant", "identity", "rbac", "menu", "audit"},
        Admin: capability.AdminSurface{
            Resources: []capability.AdminResource{
                {
                    Resource:         "feedback-tickets",
                    Title:            capability.Text("反馈工单", "Feedback Tickets"),
                    Description:      capability.Text("用户反馈与处理记录。", "User feedback and handling records."),
                    PermissionPrefix: "admin:feedback-ticket",
                    Menu: capability.AdminMenu{
                        Route:  "/feedback-tickets",
                        Parent: "support/workbench",
                        Group:  "operations",
                        Icon:   "audit",
                        Order:  250,
                        Cache:  true,
                    },
                },
            },
        },
        App: capability.AppSurface{
            Routes: []capability.AppRoute{
                {
                    Method:      "GET",
                    Path:        "/api/app/feedback/tickets",
                    Auth:        capability.AppRouteAuthSession,
                    Permission:  "app:feedback-ticket:read",
                    Description: capability.Text("读取反馈工单。", "Read feedback tickets."),
                },
            },
        },
        Migrations: []capability.Migration{
            {
                ID:          "feedback-0001",
                Description: "Create feedback tables.",
                Up:          migrateFeedbackTables,
            },
        },
        Seeds: []capability.Seed{
            {
                ID:          "feedback-seed-0001",
                Description: "Seed feedback dictionaries.",
                Run:         seedFeedbackDictionaries,
            },
        },
    }
}
```

Recoverable sensitive fields are capability configuration, not platform field-name conventions. Add resource-level AAD scope and field-level protection explicitly:

```go
resource.Protection = &capability.AdminResourceProtection{
    SchemaVersion: 1,
    Scope:         "tenant-field",
    TenantField:   "tenantCode",
}
resource.Fields = append(resource.Fields, capability.AdminField{
    Key: "governmentReference", Source: "values", Type: "text", InForm: true,
    Sensitivity: capability.FieldSensitivitySensitive,
    StorageMode: capability.FieldStorageEncrypted,
    ResponseMode: capability.FieldProjectionPrivileged,
    ExportMode: capability.FieldProjectionOmitted,
    Protection: &capability.AdminFieldProtection{
        Format: "aes-256-gcm-v1",
        Normalization: "trim-v1",
        BlindIndexNamespace: "feedback-government-reference",
    },
})
```

To allow controlled plaintext reveal, register a policy in the same manifest and reference it from an encrypted field. Do not infer policy from the field name and do not add a custom plaintext route:

```go
manifest.Admin.RevealPolicies = append(manifest.Admin.RevealPolicies, capability.AdminRevealPolicy{
    ID: "feedback-sensitive-review-v1",
    Mode: capability.AdminRevealModeAnyOf,
    Factors: []string{capability.AdminRevealFactorOIDCReauthentication},
    Purposes: []capability.AdminRevealPurpose{{
        Code: "case-review",
        Label: capability.Text("工单复核", "Case Review"),
    }},
    ChallengeTTLSeconds: 300,
    GrantTTLSeconds: 30,
})
resource.Fields[len(resource.Fields)-1].Reveal = &capability.AdminFieldReveal{
    PolicyID: "feedback-sensitive-review-v1",
    Permission: "admin:feedback-ticket:sensitive-reveal",
    CopyAllowed: false,
}
```

Revealable fields must remain recoverably encrypted and use masked or omitted ordinary projections. The platform exposes the generic policy/challenge/factor/reveal routes, enforces the field permission and policy, and consumes a short-lived single-use grant before privileged projection.

Use `raw-v1`, `trim-v1`, `email-v1`, `phone-e164-cn-v1` or `identity-cn-v1` according to the value contract. Do not infer the normalizer from the key. Omit `BlindIndexNamespace` when lookup is unnecessary; otherwise only exact `=` queries are available. A `tenant-field` must be declared, required, plain and stable. Ordinary API and export projection omit encrypted values. Do not add a capability-private plaintext route: authorized viewing must use the platform's generic reveal flow, while historical plaintext migration remains a separate offline approved capability.

The platform does not infer sensitivity from names such as `phone`, `email`, `address`, `password` or `token`. Manifest authors must classify every non-public value explicitly; contract validation enforces the declared policy but does not replace that ownership decision with a built-in name list.

## Manifest Registration Rules

`Registry.Register` is the runtime entry gate for every capability manifest. It normalizes leading and trailing whitespace, then rejects manifests that do not meet the shared plugin contract:

- `ID` is required and must use lowercase letters, numbers and hyphens;
- `Version` is required and must use numeric semver such as `0.1.0`;
- each dependency ID is trimmed, required and must use the same lowercase/hyphen identifier format;
- duplicate dependencies are rejected after trimming;
- a capability must not depend on itself.

Keep `Name` human-readable and stable, but do not use it for dependency resolution, permissions, menus or routing. Those contracts must reference the normalized `ID`.

`config.Load` preserves empty `PLATFORM_CAPABILITIES` entries so malformed comma-separated lists can fail validation instead of being silently skipped. `config.Config.ValidateRuntime` requires at least one enabled capability and checks for empty, malformed or duplicate capability IDs before stores are opened. Use the `minimal-admin` profile when a deployment needs the smallest admin foundation; do not use an empty capability list to mean "disable everything." `Registry.ResolveEnabled` applies the same identifier contract to the enabled capability list after configuration parsing and in any downstream composition root, and it also rejects nil or empty enabled lists. Enabled IDs are trimmed, must use lowercase letters, numbers and hyphens, and duplicate enabled IDs are rejected after trimming. A startup failure here means the profile or deployment list is invalid; do not hide it by silently skipping unknown or malformed capabilities.

Lifecycle migration and seed IDs are part of the manifest contract. Use lowercase letters, numbers and hyphens, trim whitespace before review, keep IDs globally unique for their step kind, and do not reuse the same step ID across migration and seed declarations. Lifecycle step bodies must be idempotent because memory, file and GORM-backed lifecycle history adapters may skip already recorded steps by ID.

## Unified Calling Model

Platform core should expose stable ports or APIs. Business code should not reach into concrete storage, HTTP handlers or frontend internals.

Preferred internal style:

```go
platform.Identity.Users()
platform.Auth.Sessions()
platform.RBAC.Can()
platform.Audit.Record()
platform.Files.Store()
```

Current implemented ports:

- `AdminResourceRepository` for generic admin resource persistence;
- `session.Repository` for session persistence;
- `capability.LifecycleHistory` for migration and seed history;
- `cache.Store` for optional Redis-backed admin cache;
- `storage.ObjectStore` for file object storage behind the `file-storage` capability;
- capability manifests for admin resources, app API route contracts, auth providers and demo data.

Rule:

- platform services expose typed ports;
- HTTP handlers adapt external requests to those ports;
- frontend calls stable API clients from `admin/src/platform/api/client.ts`;
- business packages do not import admin shell components directly unless they are writing a custom page extension.

## Admin Resource Rules

Use `capability.Manifest.Admin.Resources` first.

The generic resource engine supports:

- schema endpoint;
- list;
- create;
- update;
- delete;
- search fields;
- filterable fields and multi-condition query;
- sortable fields;
- localizable fields through `<key>Zh` and `<key>En` values;
- form groups, localized field help and validation metadata;
- required field validation;
- permission codes;
- menu filtering.

Add custom schema fields only when the default schema is not enough. For frequently repeated UI patterns, extend the generic resource schema or platform UI primitives instead of branching on one resource name.

Backend checks remain authoritative. Frontend permission hiding is only a usability layer.

## Menu Rules

Menus are declared through `AdminMenu`, not hard-coded in the admin shell.

Fields:

- `Route`: internal route starting with `/`, or an `http(s)` URL when `External` is true;
- `Parent`: optional slash-separated parent path such as `runtime`, `access` or `support/workbench`;
- `Group`: top-level shell group;
- `Icon`: semantic shell icon key;
- `Order`: ordering hint;
- `External`: open in a new browser tab and do not create a work tab;
- `Cache`: frontend route/component cache hint.

Use `Parent` for second-level or deeper menus. Do not simulate hierarchy by duplicating titles in resource names.

## Auth Provider Rules

Auth providers are declared in manifests and discovered through:

```text
GET /api/auth/providers
```

Provider declarations may be visible but unconfigured. The UI should render unconfigured providers as disabled.

Each provider manifest must use stable lowercase id and kind values with only letters, numbers and hyphens. `Title` and `Description` are required localized text. `ConfigKeys` lists environment-style configuration keys, using uppercase letters, numbers and underscores, and duplicate or empty keys are rejected. This keeps WeChat, SSO and future provider plugins discoverable without adding provider-specific fields to the shell.

The current platform intentionally has no local-password provider or password repository. The API composition root rejects the explicit provider kind `password`; provider IDs and generic field names do not select authentication semantics. A future local-password plugin must be a separately approved capability with dedicated Argon2id hashing and migration/reset/rotation contracts; it must not persist password hashes in generic `Record.Values` or make hashing policy a mutable project-initialization switch.

Every provider declares an explicit `admin` or `app` audience. Admin discovery and login must not expose App-only providers, App login must reject Admin-only providers, and the two security domains must not share identity resolvers, binding stores or token types. Admin OIDC uses `POST /api/auth/providers/:provider/start` plus the existing login exchange, then resolves only an enabled `admin-identities` binding to an existing enabled Admin user with effective permissions.

Provider adapters must resolve to a platform username or platform user identity. Authorization is still calculated by:

```text
user -> roles -> permissions / denyPermissions -> menus/resources/actions
```

Do not put role logic inside auth providers. Business capabilities should declare permission prefixes and route permissions; they should not invent custom role fields for action denies. Use the shared `roles.denyPermissions` field when a project needs deny-overrides-allow behavior.

OIDC claims and groups authenticate an identity only. Provider adapters must not auto-create platform users or assign roles, permissions, tenants, organizations or areas. Persist only approved hash-and-mask binding fields; raw provider subjects, issuers, claims, authorization codes, tokens, state, nonce, PKCE material and credentials stay outside generic resources and audit payloads.

## Capability Admin Resource Rules

Platform system resources stay in `resources/admin-resources.json`. That file is the stable base for tenants, org units, area codes, users, role groups, roles, permissions, menus, audit and other shared management resources.

Optional business capabilities should declare their own admin resources through `capability.Manifest.Admin.Resources`. This keeps the platform base reusable while still allowing business modules to expose list/detail/form/schema/menu/permission metadata when the capability is enabled. The CLI wrapper is:

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
```

`resources/generated/admin-capability-resource-contract.json` is a generated artifact, not a source of truth. `scripts/generate-admin-resource-contract.mjs` merges enabled capability resources that are not already provided by the platform base into `resources/generated/admin-resource-contract.json`, which then feeds OpenAPI, codegen preview and scaffold planning. Duplicate resource keys are skipped in favor of the platform base definition. `rtk node scripts/validate-admin-resources.mjs` checks that the generated files stay fresh against both the static manifest and enabled Go manifests. Capability-contributed resources must ship a usable schema with fields, table/search/filter/sort metadata for queryable resources and permission codes; a menu-only placeholder is not a valid platform resource. Business resources that need ownership should use the shared `tenantCode`, `orgUnitCode` and `areaCode` fields instead of inventing capability-local organization or region fields.

Base governance resources are capability-owned even when their richer UI schema lives in `resources/admin-resources.json`:

- `tenant` owns `tenants`;
- `identity` owns `users`, `org-units` and `app-identities`;
- `rbac` owns `roles`, `role-groups` and `permissions`;
- `dictionary` owns `dictionaries`, `dictionary-parameters` and `area-codes`;
- `parameter` owns `parameters`, `branding` and `settings`;
- `menu`, `session`, `audit` and `system-admin` own their corresponding menus, sessions, logs, monitoring, versions and API token resources.

Role groups are a management and classification dimension for roles through `roles.groupCode`; they do not grant permissions, own role membership, carry data scopes or create inherited policies. `role-groups.parentCode` remains only for legacy snapshot/schema compatibility. New target-mode writes must use a strict non-nested `role group -> role` model, with every role owned by exactly one group. `users.roles` owns user-role membership, while role allow/deny/data-scope policy is changed through the reviewed target-mode domain command rather than direct generic mutation. If a project needs role inheritance, approval workflows, role templates or grouped membership operations, add that as an explicit RBAC enhancement with precedence tests instead of hiding it inside role groups. The optional `policy-review` capability follows this rule: it contributes `policy-reviews`, is enabled through the `enterprise-governance` profile, and exposes `POST /api/admin/policy-reviews/:id/request`, `POST /api/admin/policy-reviews/:id/reject`, `POST /api/admin/policy-reviews/:id/approve` and `GET /api/admin/policy-reviews/export` for controlled policy-change ledgers. Default platform profiles exclude this capability, and generated OpenAPI includes these routes only when the source contract includes `policy-reviews`. Area codes are default regional master data, but their attachment fields stay optional by default: tenants, org units, users and roles may reference them, and an address-code assignment does not imply access unless a role data scope uses the corresponding area-scope mode.

`users` is the platform account and current-principal resource. Do not overload it into a full HR/personnel model. The optional `personnel` capability owns `personnel-profiles`, `positions` and `position-assignments` for products that need HR-style modeling. These resources reuse shared `tenantCode`, `orgUnitCode`, `areaCode` and account-binding relations, so generic data scopes continue to work without expanding the default foundation.

Use the platform audit command as the capability onboarding gate:

```bash
rtk go run ./cmd/platform-contracts audit --stdout
```

The audit reuses the enabled Go manifests and validates admin resources, app routes, lifecycle declarations, auth providers and demo data in one pass. Its JSON output reports total capabilities, resources, routes, permission counts, provider count, demo-data count, migrations, seeds and each capability's contributed resources. The default `rtk node scripts/validate-admin-resources.mjs` gate also checks that `resources/generated/platform-capability-audit.json` is fresh against this command. A plugin is not ready to enable until this command passes with the same `PLATFORM_CAPABILITIES` list that the target deployment will use. The base governance resources `org-units`, `role-groups` and `area-codes` must stay visible in that output through `identity`, `rbac` and `dictionary`; configuration resources must stay visible through `dictionary -> dictionaries/dictionary-parameters/area-codes` and `parameter -> parameters/branding/settings`; operations resources must stay visible through their owning capabilities, including `session -> sessions`, `audit -> audit-logs/login-logs/error-logs` and `system-admin -> versions`. Business plugins should reference shared platform resources through relation fields instead of shadowing them.

Platform engineering capabilities are also tracked in `resources/platform-engineering-capabilities.json`. Update that matrix when a reusable platform slice adds or changes dynamic resources, menus, permission codes, organization/role-group/area governance, schema/form generation, OpenAPI, scaffold behavior, source-writing readiness, system management, app route contracts, production runtime gates, deployment topology/package files, goal completion gates, node closeout gates, promotion evidence templates, promotion evidence draft packages, production readiness preflight or cache invalidation. `rtk node scripts/validate-platform-engineering-capabilities.mjs` checks the matrix against required capability IDs, target-stack dependencies, key Gin/GORM/Casbin/JWT and Refine/React/Ant Design source wiring, generated files, admin resources, OpenAPI paths, scaffold source-writing boundaries, codegen readiness, node closeout and preflight evidence. The default admin resource gate runs it automatically, so stack drift fails before a capability is treated as ready.

Implemented task nodes must also be closed in `resources/platform-node-closeout-audit.json`. Add or update the node entry with cleanup evidence, resource-lock review, objective-conflict review and tests-or-validators before treating a task as closed. Routine small nodes and sub-agent tasks use `cleanupMode=focused` with `neatFreak=false`; invoke `neat-freak` only for phase closeout, major cross-module work or release preparation. If the task touches `admin-ui` or `browser-qa`, the closeout entry must include both `superpowers:brainstorming` and product-design evidence. Validate with `rtk node scripts/validate-platform-node-closeout-audit.mjs`.

Production env files are part of the same preflight boundary. `rtk node scripts/validate-platform-production-env.mjs` checks the standard template, while `rtk node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env> --run` must be used for real deployment secrets before config import, restore, migration or token rotation operations. Strict mode rejects copied placeholders, `demo-data`, enabled demo auth, non-Redis cache and non-GORM production stores.

Source-writing code generation remains disabled. `resources/platform-codegen-source-writing-readiness.json` now also declares target families such as backend models, repositories, API routes, admin resource pages and contract tests. Each family must map scaffold roles to allowed runtime target roots and at least one `rtk` test command. Runtime roots are registered in `runtimeTargetPolicy`: existing roots must point to real directories, while proposed roots such as future generated model or repository packages must require a separate architecture/source-writing spec before they can become runtime code. The readiness contract also carries a source-writing approval evidence package: promotion needs platform-architect, codegen-owner, runtime-owner and operations-owner approval plus an explicit source-writing spec, approved promotion review packet, reviewed diff, rollback runbook, target-family test output and runtime-root owner approval. The package declares a completed-evidence artifact schema with external absolute artifact URI, `sha256:` plus 64 lowercase hex artifact hash, reviewed commit, target families, runtime targets, `rtk` verification commands and rollback commands; self-approved, text-only, local-path or private-network evidence is not enough. Text-only approval, missing diff review, missing rollback, missing test output or a runtime-mutating review packet are rejected. `resources/generated/admin-scaffold-promotion-review.json` packages those target families, root-policy evidence, generated scaffold files, hashes, runtime targets, manual review status, approval evidence package and preflight commands into a non-mutating review artifact. This makes a future generator reviewable by target family without allowing a script flag to overwrite runtime source or silently introduce a new package layer.

`resources/platform-task-execution-audit.json` is the release-facing gate list for future source-writing promotion. The foundation node is implemented as a skeleton and review-entry capability, but runtime mutation must stay blocked while `mode.sourceWriting=disabled`, `sourceWritingApprovalPackage.status=blocked` or the generated promotion review remains `manualReview.decision=not-approved`. The audit must still list structured approval evidence, reviewed diff, target-family test output, runtime target owner approval and rollback evidence before promotion.

Promotion evidence collection follows a three-step non-mutating flow:

```bash
rtk node scripts/generate-platform-promotion-evidence-templates.mjs
rtk node scripts/generate-platform-promotion-evidence-package-draft.mjs
rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>
```

`resources/generated/platform-promotion-evidence-templates.json` remains the draft-only template contract. `resources/generated/platform-promotion-evidence-package-draft.json` is a reviewer-fillable draft with `approvalState=draft-submission`; it preserves each evidence description and carries read-only `reviewContext` for production-auth provider controls, source-writing target families, review artifacts, preflight commands and promotion rules. It must not pass the submitted evidence validator until reviewers change it to `submitted`, complete every evidence item and attach external absolute `https`/`s3`/`gs` artifact URIs, `sha256:` plus 64 lowercase hex artifact hashes, verification commands and rollback commands. The submitted validator rejects relative paths, local absolute paths, `file:`, `http:`, localhost and private-network artifact hosts. For production-auth submissions, the submitted validator checks provider IDs, provider controls and runtime test refs against `resources/platform-production-auth-hardening.json`. For source-writing submissions, it checks that evidence covers every declared target family, runtime target and target-family test command from `resources/platform-codegen-source-writing-readiness.json`. Passing the submitted package validator is review input only. It must not mutate runtime contracts, enable refresh-token-family runtime, enable source-writing generation or mark the active foundation goal complete.

Plugin-style capability governance is split into two gates. `resources/platform-capability-contracts.json` classifies every built-in, optional, local-demo and external-business capability, records its default/profile policy, owner boundary and replaceability, and keeps this governance metadata out of the runtime `capability.Manifest` struct. Verify the classification contract with:

```bash
rtk node scripts/validate-platform-capability-contracts.mjs
```

Executable cross-plane service declarations are separate from that classification metadata. A capability may add `capability.Manifest.Service` for Admin, Service/Data, Control, External/Partner and Event Plane contracts. The service surface owns identity, trusted tenant context, operations, events, reliability, PII and compatibility declarations; profile policy and replaceability remain in `resources/platform-capability-contracts.json`.

Generate and verify the service artifacts with:

```bash
rtk go run ./cmd/platform-contracts service-manifests --output resources/generated/platform-service-contract.json
rtk node scripts/generate-platform-service-contract-artifacts.mjs
rtk node --test scripts/platform-service-contract-standard.test.mjs
rtk node scripts/validate-platform-service-contract-standard.mjs
```

See `docs/platform-service-contract-standard.md` for the five-plane boundary and deferred runtime responsibilities.

High-risk Admin queries and commands use the separate server-registered service-object runtime. Generate the Admin OpenAPI and typed client, then verify the runtime contract with:

```bash
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk go test ./internal/platform/serviceobject ./internal/platform/httpapi
rtk node --test scripts/platform-service-object-runtime.test.mjs
rtk node scripts/validate-platform-service-object-runtime.mjs
```

Clients send only a stable query or command ID, version, typed logical arguments and the allowed pagination, sort or idempotency fields. Tenant context, data scope, predicates, physical fields and datasource routing stay server-owned. See `docs/platform-service-objects.md` for registration, execution, idempotency and composition boundaries.

The contract gate cross-checks `resources/platform-capability-profiles.json`, `internal/platform/config.defaultCapabilities` and `cmd/platform-contracts audit` output. It rejects unclassified profile capabilities, profile-only capabilities leaking into defaults, `external-business-capability` inside non-business profiles and declared resource/route/provider drift from actual Go manifests.

The default `rtk node scripts/validate-admin-resources.mjs` gate runs this contract gate before treating the platform resource contract as fresh. Capability profile changes, default capability changes and manifest-surface changes therefore fail during normal admin-resource validation, not only during a separate production preflight.

Capability compositions are documented in `resources/platform-capability-profiles.json` and verified with:

```bash
rtk node scripts/validate-platform-capability-profiles.mjs
```

The profile gate runs the same audit command for each declared composition, so missing dependencies, duplicate surfaces and business capability leakage are caught before runtime. `platform-default` must match `internal/platform/config.defaultCapabilities`; it must not include `external-business-capability`. `minimal-admin` proves the smallest admin shell can run without optional operations modules. `platform-app-ready` adds reusable app phone verification/binding. `platform-personnel-ready` adds the optional `personnel` capability without promoting HR-style resources into defaults. `scripts/validate-platform-personnel-runtime-readiness.mjs` is the runtime evidence gate for that profile: it rejects default admin contracts or audits that expose `personnel`, then verifies the personnel-ready contract produces personnel resources, read permissions, frontend entries and OpenAPI query paths/schemas together. `platform-notification-ready` adds the optional `notification` capability for reusable in-app templates, notification records and delivery ledgers without pulling SMS, email, WeChat push or scheduler adapters into the default base. `platform-job-ready` adds the optional `job` capability for job definitions, run records and attempt ledgers without bundling a worker engine, distributed lock or retry queue into the default base. The `external-business-capability` marker is a reference-classification placeholder only; it is not an executable profile and must not introduce business manifests into `platform-go`. When adding a capability, update this profile file only if the capability changes a reusable composition, then run the validator before updating docs or examples.

Reference project discovery is captured in `resources/platform-reference-discovery.json` and verified before coverage mapping:

```bash
rtk node scripts/validate-platform-reference-discovery.mjs
```

Discovery records which external `zshenmez` source sets were inspected, then classifies each candidate as `foundation`, `extension`, `business` or `deferred`. This is the evidence layer. A candidate can influence the default platform only after it has source signals, a foundation coverage area, capability/resource mappings, UI/i18n coverage when user-facing, and default-profile leakage tests. Candidate `evidenceSignals` must also resolve inside the candidate's own declared `sourceSets`; broad source-set required signals are not enough to prove that a specific candidate was inspected. Business-only candidates such as dispatch, fulfillment and support remain owned by `external-business-capability`, which means outside `platform-go`; optional candidates such as app-phone identity, detailed addresses and personnel stay behind profiles or owning capabilities. Admin query security and API boundary evidence from the reference project is promoted as a foundation gate, because every business-neutral admin module needs the same unified client, structured filter transport and schema-whitelisted backend query contract.

The default admin-resource gate runs reference discovery before reference coverage, so a resource or capability change cannot rely on stale classification evidence while the coverage map still happens to pass.

Reference project coverage is captured in `resources/platform-reference-coverage.json` and verified with:

```bash
rtk node scripts/validate-platform-reference-coverage.mjs
```

Coverage is the mapping layer: it maps the reusable management resources from the `zshenmez` reference backend to platform foundation resources and capabilities. Required foundation coverage areas are dashboard, identity and tenancy, RBAC and menu, API governance, dictionary/parameter/branding, audit and operations, file storage, auth providers and demo data; each required area must keep at least one platform capability mapping. It also asserts that business resources such as tasks, transfer chains, portfolio works and support tickets stay outside the default platform contract unless their owning business capability is explicitly enabled. Business app routes declared by an optional business capability must also appear in the matching `businessBoundary.appRoutes` list when a reference audit is used, so app-facing actions do not disappear from parity checks. The gate cross-checks `resources/platform-capability-profiles.json` so every `businessBoundary.expectedCapability` appears in profile `businessCapabilities`, and every declared business capability has a reference boundary owner. For the `zshenmez` reference, that owner must stay the abstract `external-business-capability`; individual business resource parity entries must match their boundary owner and must not introduce concrete business capability names into `platform-go`. It also tracks non-resource reference capabilities that do not appear in `resources/admin-resources.json`: `storage-settings` maps to `parameter` plus `file-storage`, `admin-api-boundary-query-security` maps to API governance, `app-phone-binding` maps to optional `app-phone` through `platform-app-ready`, and `user-addresses` stays outside the default foundation as a detailed-address boundary owned by the consuming capability. Update discovery and coverage together when a reference-project capability is promoted into the reusable foundation, and keep product-specific workflows in an application capability.

## App API Rules

Business-facing APIs belong under the app security domain and must be declared through `capability.Manifest.App.Routes` before custom route registration is added.

Fields:

- `Method`: one of `GET`, `POST`, `PUT`, `PATCH`, `DELETE` or `OPTIONS`;
- `Path`: full HTTP path starting with `/api/app/`, without query strings or fragments;
- `Auth`: `AppRouteAuthPublic` for intentionally public endpoints or `AppRouteAuthSession` for endpoints requiring an app bearer token;
- `Permission`: optional app permission code matching `app:<domain>:<action>` for business authorization. Both domain and action use lowercase letters, numbers or hyphens;
- `Description`: required localized route description for docs and future OpenAPI/codegen output.

Validation rejects duplicate method/path pairs, paths outside `/api/app/`, paths with query strings or fragments, missing localized descriptions, missing auth mode, unsupported methods, malformed app permission codes, `admin:` permission codes and permissions on public routes.

Use `capability.AppRouteContracts(enabledManifests)` when generators need app route metadata. It validates the same manifest surface, normalizes method/path/permission values and returns a stable sorted contract list. The CLI wrapper is:

```bash
rtk go run ./cmd/platform-contracts app-routes --output resources/generated/app-route-contract.json
```

`resources/generated/app-route-contract.json` is a generated artifact, not a source of truth. `rtk node scripts/validate-admin-resources.mjs` checks that it stays fresh against the Go manifests.

App API docs and preview artifacts are generated from that contract:

```bash
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
```

The App client API boundary is the frontend consumption contract for these artifacts. Downstream App, H5 and mini-program clients should generate or wrap calls through `future-app/src/platform/api/appClient.ts`, or through equivalent `appRequest` and `appUpload` ports owned by the target client shell. Business pages must not call `uni.request`, `wx.request`, `Taro.request`, `uni.uploadFile`, `wx.uploadFile` or `Taro.uploadFile` directly, and must not set or override `Authorization`. The request/upload port owns `/api/app` prefixing, app-token injection, upload target validation and rejection of admin-domain paths such as `/api/admin` or `/admin`. `resources/platform-app-client-api-boundary.json` and `scripts/validate-platform-app-client-api-boundary.mjs` keep this contract tied to `app-route-contract.json`, `openapi.app.json` and `app-codegen-preview.json`.

The base `file-storage` capability contributes reusable App file routes through this same surface: `POST /api/app/files` for multipart upload and `GET /api/app/files/:id/content` for session-scoped content streaming. These routes are generic file-object plumbing only; business attachment status, review flows, ownership beyond the app user/session boundary and domain-specific file categories must stay in the consuming capability.

This keeps business APIs discoverable and separable from admin APIs. Runtime handlers are registered through the neutral `approute.Registration` contract and passed into `httpapi.ServerOptions`; the server only exposes routes that are also declared in enabled `Manifest.App.Routes`. A declared route without a handler returns `APP_ROUTE_HANDLER_NOT_CONFIGURED`; a handler without a manifest declaration is ignored by the router. Business packages should export registration factories, while process composition roots decide which handler sets are supplied for a deployment.

For `AppRouteAuthSession`, the platform wrapper accepts only app JWT bearer tokens, stores the active app session for handlers through `approute.SessionFromContext(ctx)`, and enforces optional `app:` permission codes through the shared authorizer with tenant `app`. Business handlers should read the app session from that context helper instead of reparsing bearer tokens.

Configured app login providers attach through `httpapi.AppIdentityResolver`. The resolver owns provider-specific code exchange and trusted external identity lookup. It must return a trusted `ProviderSubject` for provider-backed login; platform core maps `provider + subject hash` to a stable app username through `AppIdentityBindingStore`, then issues the app JWT and records a provider marker in audit. The default binding store uses the generic `app-identities` resource and stores `providerSubjectHash`, `maskedSubject`, `appUsername`, `createdAt` and `lastLoginAt`. Raw OpenID, UnionID or provider subject values must not be returned by app login responses or written into generic audit records or generic resource rows.

The optional `app-phone` capability follows the same App security-domain rules. It declares `POST /api/app/identity/phone-verifications` and `POST /api/app/identity/phone-bindings` as app-session routes, plus `app-phone-verifications` and `app-phone-bindings` admin resources. The current foundation implementation is local/demo delivery: the verification API returns a `debugCode`, stores only `codeHash`, `phoneHash`, `maskedPhone`, app username and timestamps, applies a rolling-window limit per app username, phone hash and purpose, and the binding API enforces one enabled binding per phone hash. Production SMS delivery should be added behind a provider/service adapter while preserving the route and resource contracts; multi-instance deployments can move the same rate-limit semantics behind Redis or another distributed adapter. Do not store raw phone numbers or raw verification codes in generic resources, audit records or binding responses.

The built-in WeChat miniapp adapter lives under `internal/platform/authprovider/wechat` and is wired by `internal/platform/authprovider.AppIdentityResolverFromConfig` when `PLATFORM_WECHAT_MINIAPP_APP_ID` and `PLATFORM_WECHAT_MINIAPP_SECRET` are present. Business capabilities that need richer identity semantics should keep them behind their own service ports. Do not couple app login to admin users, admin roles or admin JWTs. A business-specific adapter may replace or wrap the platform resolver, but it must still perform provider exchange server-side, return the trusted subject to the resolver boundary, and let the platform binding store restore the reusable app username unless the capability deliberately owns a different binding model.

The production session-policy specification is `docs/platform-roadmap.md`. Capability authors must not enable offline renewal or refresh-token-family behavior through a provider adapter or app route. The reusable refresh-token-family slice under `internal/platform/refreshtoken` is implemented but disabled by default; production binding requires the spec's hashed token-family storage, reuse detection, revocation scope, Redis/session invalidation and audit redaction gates plus external approval evidence. `resources/platform-production-auth-hardening.json` also carries `productionPromotionApprovalPackage`; promotion needs security-owner, platform-architect and operations-owner approval plus signed session-policy review, runtime test output, provider rotation runbook, rollback evidence and redacted audit samples, and text-only approval is rejected. Completed evidence must include artifact URI/hash, reviewed commit, target environment, rollback commands, `rtk` verification commands, audit sample refs, provider rotation runbook refs, refresh-token-family test refs, provider IDs, provider controls and runtime test refs; `resources/generated/platform-operations-plan.json` carries the same schema for release review. `resources/generated/production-auth-promotion-review.json` is generated separately by `scripts/generate-production-auth-promotion-review.mjs` and must remain `not-approved` while the slice is disabled by default or approval artifacts and rollback/audit evidence are missing.

## Demo Data Rules

Demo data belongs in `capability.Manifest.DemoData`.

Use demo data for:

- local demos;
- acceptance fixtures;
- product walkthroughs.

Do not use demo data for:

- database structure changes;
- required production reference data;
- destructive reset behavior.

Demo records should be idempotent through stable IDs or codes.

Manifest validation rejects demo datasets whose target resource is not provided by the enabled admin resource manifest set. It also rejects malformed or duplicate dataset IDs and duplicate record IDs or codes within the same dataset. Demo `Values` are interpreted only through the target resource's declared field policy; key names do not infer sensitivity, and undeclared keys or policy-invalid writes fail when the dataset is applied. A capability that ships demo data for `tasks`, for example, must also enable the capability that contributes the `tasks` admin resource.

## Disable Behavior

Every capability must be safe to disable.

When disabled, it must not expose:

- capabilities list entries;
- app API route declarations;
- admin resource schemas;
- menu entries;
- generated permission catalog entries;
- auth providers;
- demo datasets.

Existing persisted records may remain in storage, but disabled capability schemas are not registered and should not be reachable through generic admin APIs.

Required tests:

- enabled capability registers its expected resource/menu/provider/demo data;
- disabled capability does not leak resource/menu/provider/demo data;
- missing dependencies fail startup;
- duplicate resources, routes, permission prefixes, provider IDs, demo dataset IDs or demo record keys fail manifest validation;
- demo datasets targeting disabled or missing resources fail manifest validation.

## Frontend Rules

Use Ant Design through platform UI wrappers when building shared admin surfaces.

Common components should provide:

- sensible defaults;
- `children` or render slots for custom content;
- prop overrides for density, actions, status and layout;
- permission-aware action visibility;
- i18n text from the platform dictionary layer.

Do not fork page-level UI for standard CRUD. Use the generic resource console first, then add custom slots or actions only when a workflow genuinely needs them.

Schema-driven form layout and slot promotion is governed by `resources/platform-form-schema-layout-slots.json`. Current shared forms use `PlatformResourceForm` and support single-column, grouped sections, `two-column-density` and `side-detail-preview` through `Schema.FormGroups`, `Schema.FormLayout`, `FieldDefinition.Group` and descriptor-only `Schema.RuntimeSlots`, plus helper text, validation, relation controls, AntD Form control-prop passthrough, Refine CRUD hooks and controlled source-level React slots. Dense forms are browser-verified for desktop two-column layout, mobile single-column fallback and viewport-safe `AdminFormModal` scrolling; runtime side-preview slots are browser-verified for desktop rail and mobile collapsed stack. Do not put React component names, runtime component paths or raw scripts into backend manifests. Business capability renderers must be registered at the app or capability composition boundary rather than imported by platform core or the generic shell.

New platform components must update `admin/src/platform/i18n.ts` in the same change. `rtk node scripts/validate-admin-i18n.mjs` is a hard gate for shared component text, and the admin build runs that gate before TypeScript and Vite. Localized resource, plugin or dashboard data should use `LocalizedText`, `nameZh/nameEn` plus `descriptionZh/descriptionEn` values, or a future translation table, not component literals.

Generic admin resources keep canonical `name` and `description` strings for storage compatibility. Common resource forms reserve localized fields by default, and business resources can declare the same keys when they need localized data display:

```text
nameZh
nameEn
descriptionZh
descriptionEn
```

The generic resource console reads those keys according to the active language for table cells, details, search and filtering. Business data can ignore these fields until it needs translation.

Use `PlatformDataTable` for normal lists. It provides search syntax help, multi-condition filters, datetime range filters, column sorting, column visibility, cross-page selection, batch action slots, row action slots, toolbar extension slots, optional expanded rows, inline editor hooks, detail drawer slots, custom empty states, pagination styling and mobile cards. Standard CRUD pages should put default row commands in `rowActions` instead of adding a page-local fixed operation column; custom operation buttons stay caller-owned, but alignment, density and future menu behavior stay centralized. Long list values should use `PlatformOverflowText` or a table-level equivalent so hover detail is consistent. SQL-like search must remain a safe DSL over whitelisted fields or become structured JSON conditions; supported operators are `:`, `~`, `=`, `!=`, `>`, `<`, `>=` and `<=`. Do not concatenate raw user text into SQL. `scripts/validate-platform-admin-api-boundary.mjs` is the hard gate for shared admin calling style: direct `fetch` and authorization handling stay in `admin/src/platform/api/client.ts`, Refine `dataProvider` carries structured query meta, App API paths are forbidden from admin source, and generated OpenAPI query schemas must expose field/filter/sort allowlists.

Use `PlatformDropdownPanel` for column settings, filters and future lightweight plugin menus so dropdown backgrounds, spacing, theme and accessibility stay consistent. Prefer its width and max-height props over page-local dropdown CSS.

Use custom platform pages only for platform-governance workflows that already have dedicated platform routes and cannot be expressed cleanly as standard CRUD. `PolicyReviewConsole` is the implemented platform-governance example: it mounts only when `/policy-reviews` is enabled, uses shared API-client wrappers and platform UI primitives, keeps request/approve/reject/export behavior inside the optional `policy-review` capability, and queries audit timelines through schema-declared `audit-logs` fields. Business approval or fulfillment consoles should live in downstream capability packages and attach through manifests, routes, actions and panels instead of entering `platform-go`.

Admin field types currently supported by the shared console are `text`, `textarea`, `select`, `multiselect`, `datetime`, `switch`, `number` and `color`. Use `switch` for boolean UI such as external-link and cache flags; avoid modeling those as free text.

Use field `Relation` when a field references another admin resource, such as tenant code, org unit code, area code, role group code, role codes or permission codes. A relation declares the target resource plus value and label fields, and the shared console loads options through the generic resource query API via the Refine `dataProvider`. Do not hard-code these options in business pages. Use `Display: "tree"` plus `ParentField` for hierarchical targets such as organization units or area codes; the UI will render `PlatformTreeSelect` in both forms and filters while still submitting the selected code. Relation fields store the selected code in the owning record; they do not automatically add data scopes, cascaded filters, role inheritance or normalized relation tables. `scripts/validate-admin-ui-contracts.mjs` guards the frontend side: relation options must keep using the shared provider, form inputs must pass AntD Form control props through, tree relations must use the shared tree control, switch fields must bind `checked`, and edit modals must hydrate values after opening.

Business capabilities that expose tenant-owned operational records should declare ownership relation fields explicitly: `tenantCode` to `tenants`, `orgUnitCode` to `org-units`, and `areaCode` to `area-codes` when those dimensions apply. These fields are what let the generic admin resource store apply tenant, organization and area data-scope filtering after a read action is allowed. If a business resource intentionally has no tenant/org/area ownership, document that decision in the capability package and do not rely on menu visibility as a data boundary.

Capability manifest schemas are validated with the same standards as static platform resources. Field keys and form group keys must be unique, form groups and field help text must be localized, select options need values plus localized labels, `SearchFields` and `DefaultSortKey` must reference declared or standard record fields, and custom action/panel/runtime slot permissions must stay under the declaring resource `PermissionPrefix`. Treat these checks as part of the plugin contract, not as optional UI polish.

Relation targets must also be reachable through the enabled manifest set. `ResolveEnabled` fails startup when any declared `Relation.Resource` is not provided by an enabled capability, so a business resource that declares `areaCode -> area-codes` must depend on the capability that provides `area-codes`. It also validates that relation value, label, filter, sort, parent and path fields are exposed by the target resource. Standard record fields `id`, `code`, `name`, `status`, `description` and `updatedAt` are always available; any custom relation metadata such as `parentCode` or `path` must be declared by the target capability resource. Core manifests follow the same rule: `dictionary` provides shared `area-codes` before `tenant` consumes it, and RBAC provides the `roles` target used by user role bindings.

For external business capability integration, keep the implementation outside `platform-go` and expose only the public platform contracts: `capability.Manifest`, admin resource schemas, permission codes, app/admin route registrations, ownership relation fields and generated OpenAPI/resource contracts. Business state machines, dedicated business tables, write cutovers, custom panels and domain audit actions belong to the downstream business package. The platform validates that reference business candidates stay outside `platform-default` through `resources/platform-reference-discovery.json`, `resources/platform-reference-coverage.json`, `scripts/validate-platform-reference-discovery.mjs` and `scripts/validate-platform-reference-coverage.mjs`.

The platform standard resources have extra governance gates in `scripts/validate-admin-resources.mjs`: tenant rows expose optional `areaCode`; user rows require `tenantCode` and expose optional `orgUnitCode`, optional `areaCode` and multirole `roles`; org units carry required `type`, required `tenantCode`, tree `parentCode` and optional `areaCode`; area codes carry tree `parentCode`, `level` and `path`; roles carry `groupCode`, required `dataScope`, custom org and area scope relations, and permission relations. `resources/platform-governance-topology.json` adds the architecture-level topology gate for the same model: `org-units` must stay a default multi-level tree, tenant `areaCode` must stay optional by default, organization units must be tenant-owned, role groups must not own membership or policy, default profiles must exclude `personnel`, `platform-personnel-ready` must include the shared tenant, org, area and user resources, and its generated personnel admin contract must keep the declared personnel/position relation fields. The topology gate also treats `tenants`, `org-units`, `users`, `roles`, `role-groups` and `area-codes` as non-droppable default governance primitives, separately asserting that `area-codes` is a default resource while `areaCode` attachment fields are not mandatory, and rejects default-foundation promotion of optional personnel resources. These checks keep the base from shrinking back to tenant-only ownership while still letting business resources opt into only the ownership dimensions they actually need.

Role groups are intentionally classification and governance metadata. `parentCode` is retained only for legacy snapshot/schema compatibility; new target-mode capabilities must not create nested role groups. The target Admin tree has exactly two levels, role group then role, and every `roles.groupCode` names one owning group. Role groups do not grant permissions, inherit permissions, own role membership, carry data scopes or override `roles.denyPermissions`. If a product later needs role templates, inheritance, grouped membership operations or approval gates, add that as a separate policy feature with precedence tests instead of overloading `role-groups`. The provided `policy-review` capability is the optional review-ledger boundary; request, reject, approve and export actions all write policy-review audit records, and approve changes RBAC only through an explicit `admin:policy-review:update` action.

Personnel and position management is an optional capability boundary, not a default platform resource set. The reference coverage gate keeps `personnel`, `employees`, `staff`, `employeeProfiles`, `positions` and `positionAssignments` out of the default admin contract unless an explicit `personnel` capability owns them. The runtime readiness gate keeps the same boundary tied to generated artifacts, so `platform-personnel-ready` cannot claim readiness unless admin resources, read permissions, frontend resource entries and OpenAPI paths/schemas are all generated. This keeps the default foundation useful for account and organization governance without forcing HR workflows onto every business.

The default foundation already supports organization and department needs through `org-units`; use `org-units.type` for group, company, branch, organization, department, team, store and custom levels instead of creating parallel default resources such as `organizations` and `departments`. Tenant-only governance is explicitly rejected by `resources/platform-governance-topology.json` because it cannot model reusable institution, department, team or store hierarchies without page-local fields. Every org-unit row is tenant-owned through required `tenantCode`, but user-to-org assignment remains optional so deployments can support platform operators, service accounts or accounts not yet mapped to a department. Role catalog management should use `role-groups` through `roles.groupCode`, but role groups remain classification-only and must not own permission grants, role membership, inheritance or data scopes. Address-code governance should use `area-codes` as shared regional master data; the resource is included in the default foundation, while `tenants.areaCode`, `org-units.areaCode` and `users.areaCode` remain optional attachment fields. Its default consumers are `tenants`, `org-units` and `users`; the optional personnel consumer is `personnel-profiles`; the authorization consumer is explicitly `roles.dataScopeAreaCodes`. Detailed addresses beyond `areaCode` are excluded from the default foundation; keep them inside the owning capability until at least two reusable platform capabilities need the same address model. When HR-style records are needed, enable the `personnel` capability: `personnel-profiles` reuses `tenantCode`, `orgUnitCode`, `areaCode` and `userCode`, `positions` reuses `tenantCode` and `orgUnitCode`, and `position-assignments` links personnel, positions, tenants and org units.

The `zshenmez` reference includes a `user_org_memberships` model for future multi-organization assignment, but its current docs say the admin V1 exposes only a primary org. The platform default follows that boundary: `users.orgUnitCode` is the primary org relation, while multi-org membership rows stay in an optional identity, personnel or consuming capability until a separate reusable membership spec promotes them. Do not introduce a default `user-org-memberships` resource just to mirror the reference table.

Capability profiles must also declare these boundaries explicitly. `minimal-admin` and `platform-default` must list `tenants`, `org-units`, `users`, `roles`, `role-groups` and `area-codes` in `mustIncludeResources`; both default profiles must exclude the `personnel` capability. `platform-personnel-ready` must list `personnel-profiles`, `positions`, `position-assignments`, `tenants`, `org-units`, `area-codes` and `users`. The profile validator rejects profile drift, and the governance-topology validator rejects generated personnel contract drift, so the platform does not regress back to tenant-only ownership or enable personnel resources without shared organization and region context.

Track execution state in `resources/platform-task-execution-audit.json` when a capability is deliberately preview, planned or deferred. The audit validator is part of production preflight and rejects blocked nodes that are marked implemented, so extension work should expose blockers explicitly instead of weakening the base contract or overstating readiness.

Capability fields can declare `Searchable`, `Filterable`, `Sortable` and `Localizable`. Defaults are intentionally useful: searchable fields become filterable; status/select/multiselect/switch/datetime/number fields become filterable; datetime/number and normal table fields become sortable; `name` and `description` are localizable by default. Declare explicit localizable fields when a business resource needs multi-language values. Store localized values as `<key>Zh` / `<key>En` in `Record.values`; the generic console renders the active language for list and detail surfaces while keeping the canonical field value as fallback. Do not force translation on every business row.

## File Storage Capability

`file-storage` is an optional platform capability. It declares the `files` admin resource, `/files` menu entry, `admin:file:*` permission codes and lifecycle boundary through the same manifest system as other capabilities.

Runtime API:

```text
POST /api/admin/files/upload
GET /api/admin/files/:id/content
DELETE /api/admin/resources/files/:id
POST /api/app/files
GET /api/app/files/:id/content
```

`POST /upload` accepts multipart form field `file`, stores the object through `storage.ObjectStore`, then saves metadata and the redacted `file.upload` audit in one repository snapshot. If the snapshot save fails, the just-written object is deleted and no visible metadata remains. Declared, detected and allowlisted MIME values are normalized with `mime.ParseMediaType` and compared by canonical base media type; parameters such as `charset` do not participate in the equality check. `GET /content` rejects tombstoned metadata, streams the stored object by metadata id and records `file.content` after the object is opened successfully.

Delete first saves platform-owned tombstone lifecycle metadata and its redacted audit atomically. Tombstoned records are immediately hidden from list/query/content projections while retaining the internal object key during the declared recovery window. The maintenance runner later claims cleanup; object deletion is idempotently retryable, `storage.ErrObjectNotFound` is treated as already clean, and other failures leave the tombstone hidden for a later retry. Metadata is purged only after cleanup completion is durable. This ordering preserves a usable restore window and prevents a visible record from pointing at a missing object.

The App upload route uses the same storage port and metadata resource, but it is exposed only when `file-storage` is enabled and the caller has a valid App session. App uploads write internal `tenantId` and stable `ownerId` ownership metadata plus the display `uploadedBy` value; session tokens, physical paths and public URLs are not persisted. App content reads return 404 unless the file belongs to the current app user, preventing C-end clients from turning the generic file resource into a cross-user file browser.

Admin experience is implemented as a generic resource-console extension and gated by `resources/platform-file-storage-experience.json` plus `rtk node scripts/validate-platform-file-storage-experience.mjs`. The UI extends the existing `files` resource with metadata, preview and audit drawer panels plus upload, preview, download and delete actions. Internal `storageKey`, `tenantId`, `ownerId`, `deletionState` and `deletionRequestedAt` fields remain persistence metadata with omitted response/export projections; they must not appear in search, filters, tables, forms, detail drawers or visible metadata contracts. Audit visualization uses `/audit-logs` only when that optional resource is exposed; detached deployments fall back to the declared `file.upload`, `file.content`, `file.delete.request` and `file.delete` events. Do not introduce a standalone file manager page, a new storage port or a business-specific file workflow in the platform base. Keep preview support explicit for image, text, PDF and unsupported fallback states, and keep product-design, i18n, UI contract, build and browser screenshot evidence current before changing the experience.

Configuration:

```bash
PLATFORM_PUBLIC_BASE_URL=https://platform.example.test
PLATFORM_TRUSTED_PROXIES=172.30.0.10
PLATFORM_EDGE_TRUSTED_PROXY=172.30.0.1
PLATFORM_HTTP_MAX_BODY_BYTES=1048576
PLATFORM_FILE_STORAGE_DRIVER=local
PLATFORM_FILE_STORAGE_LOCAL_DIR=.platform/uploads
PLATFORM_FILE_MAX_UPLOAD_BYTES=10485760
PLATFORM_FILE_ALLOWED_MIME_TYPES=application/pdf,image/jpeg,image/png,text/plain
```

In the standard Compose adapter, `PLATFORM_ADMIN_PROXY_IP` must be contained by `PLATFORM_TRUSTED_PROXIES`, while `PLATFORM_EDGE_TRUSTED_PROXY` identifies one canonical reviewed TLS-edge peer IP and must be contained by `PLATFORM_INTERNAL_SUBNET`. Edge CIDRs, loopback, unspecified and multicast addresses are rejected. Nginx rebuilds one canonical client IP from that peer and overwrites the forwarded chain before the API applies trusted-proxy and rate-limit policy. Complementary API trusted-proxy CIDRs that cumulatively trust an entire IPv4 or IPv6 address family are rejected. The HTTP body limit applies to non-upload request bodies; valid multipart requests are exempt only on `POST /api/admin/files/upload` and `POST /api/app/files`, where the upload-specific limit remains authoritative.

S3-compatible object storage uses the same `storage.ObjectStore` port:

```bash
PLATFORM_FILE_STORAGE_DRIVER=s3
PLATFORM_FILE_STORAGE_S3_REGION=us-east-1
PLATFORM_FILE_STORAGE_S3_BUCKET=platform
PLATFORM_FILE_STORAGE_S3_ENDPOINT=https://s3.example.test
PLATFORM_FILE_STORAGE_S3_ACCESS_KEY=access
PLATFORM_FILE_STORAGE_S3_SECRET_KEY=secret
PLATFORM_FILE_STORAGE_S3_PREFIX=tenant/platform
PLATFORM_FILE_STORAGE_S3_FORCE_PATH_STYLE=true
PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION=AES256
PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID=
```

All file bytes remain private and flow through authenticated content endpoints. Local and S3 object keys use a 256-bit cryptographically random opaque identifier under an `objects/` prefix; the original filename is never embedded in the key. Local storage canonicalizes its configured root, rejects root replacement and symlink traversal, creates directories with `0700` and objects with `0600`, and returns `storage.ErrObjectNotFound` when deleting a missing object. S3 uses no public ACL, requires HTTPS outside loopback development/test, and requires explicit `AES256` or `aws:kms` encryption. S3 `DeleteObject` keeps the provider's idempotent semantics and therefore does not prove that an object existed before deletion. When `aws:kms` is selected, configure a KMS key ID. Production promotion must separately verify bucket-level Block Public Access and private bucket policy; the application cannot infer those external settings from `PutObject` configuration.

Endpoint, credentials, prefix and path-style mode are optional. If static credentials are supplied, access key and secret key must be supplied together. Business code should call the object-store port or the admin API client, not concrete local filesystem paths.

## Persistence Rules

The foundation supports memory, file and GORM-backed adapter boundaries for admin resources, sessions and lifecycle history. The older `database/sql` lifecycle adapter remains as a compatibility adapter behind the same port, but startup should prefer the GORM-backed lifecycle history when `PLATFORM_LIFECYCLE_HISTORY_DRIVER` is configured.

Use memory for tests and local throwaway runs.

Use file mode for demos and local platform configuration:

```bash
PLATFORM_ADMIN_RESOURCE_FILE=.platform/admin-resources.json
PLATFORM_SESSION_FILE=.platform/sessions.json
PLATFORM_LIFECYCLE_HISTORY_FILE=.platform/lifecycle-history.json
```

Use database-backed mode for production-like deployments:

```bash
PLATFORM_RUNTIME_ENV=production
PLATFORM_JWT_SECRET=<at-least-32-characters-and-not-the-dev-default>
PLATFORM_ADMIN_RESOURCE_DRIVER=mysql
PLATFORM_ADMIN_RESOURCE_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_SESSION_DRIVER=mysql
PLATFORM_SESSION_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_LIFECYCLE_HISTORY_DRIVER=mysql
PLATFORM_LIFECYCLE_HISTORY_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=127.0.0.1:6379
PLATFORM_CAPABILITIES=tenant,identity,session,rbac,menu,api-resource,audit,admin-oidc,dictionary,parameter,file-storage,admin-shell,system-admin
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
PLATFORM_ADMIN_OIDC_ISSUER_URL=https://identity.example/realms/platform
PLATFORM_ADMIN_OIDC_CLIENT_ID=platform-admin
PLATFORM_ADMIN_OIDC_CLIENT_SECRET=<redacted-secret>
PLATFORM_ADMIN_OIDC_REDIRECT_URL=https://admin.example/login
PLATFORM_ADMIN_OIDC_SCOPES=openid,profile,email
```

When an enabled manifest declares a reveal policy, add the conditional reveal configuration below. SMS factors require all phone fields and keys together. The stock process does not bundle a production SMS vendor; a downstream composition root must register a non-debug sender before production startup can pass.

```bash
PLATFORM_SENSITIVE_REVEAL_HMAC_KEY=<dedicated-at-least-32-byte-secret>
PLATFORM_PHONE_HMAC_KEY=<different-dedicated-at-least-32-byte-secret>
PLATFORM_PHONE_CODE_HMAC_KEY=<different-dedicated-at-least-32-byte-secret>
PLATFORM_PHONE_VERIFICATION_PROVIDER=<registered-non-debug-provider>
PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE=<resource>
PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD=<username-field>
PLATFORM_ADMIN_STEP_UP_PHONE_FIELD=<encrypted-phone-field>
PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD=<verified-at-field>
PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD=<verified-phone-digest-field>
```

`cmd/platform-api` calls `config.Config.ValidateRuntime()` before opening stores. `PLATFORM_ADMIN_RESOURCE_DRIVER`, `PLATFORM_SESSION_DRIVER` and `PLATFORM_LIFECYCLE_HISTORY_DRIVER` use GORM-backed repositories for `mysql`, `postgres` and `sqlite`; when `PLATFORM_RUNTIME_ENV=production`, they must be configured with DSNs, the JWT secret must not be the development default, Redis must be selected for cache/invalidation, `PLATFORM_CAPABILITIES` must not include `demo-data`, and `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true` must disable demo login. If `admin-oidc` is enabled, all OIDC settings must be complete, scopes must include `openid`, the production redirect must use absolute HTTPS, and an operator must bind an existing enabled Admin identity through `platform-admin bind-admin-oidc --subject-stdin` before the demo-disabled readiness gate can pass. Standard platform resources including tenants, org units, users, role groups, roles, permissions, menus, area codes and operations resources are normalized in the GORM admin resource adapter; external business resources can remain generic in the consuming package until that package needs dedicated tables.

Run `rtk node scripts/generate-platform-operations-plan.mjs`, `rtk node scripts/generate-production-auth-promotion-review.mjs`, `rtk node scripts/generate-platform-promotion-evidence-templates.mjs`, `rtk node scripts/validate-platform-promotion-evidence-templates.mjs`, `rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs`, `rtk node scripts/validate-platform-personnel-runtime-readiness.mjs`, `rtk node scripts/validate-platform-production-auth-hardening.mjs`, `rtk node scripts/validate-platform-cache-invalidation.mjs` and `rtk node scripts/validate-platform-production-readiness.mjs` before release work. Use `rtk node scripts/run-platform-production-preflight.mjs --list` for the unified production preflight catalog and `rtk node scripts/run-platform-production-preflight.mjs --policy <policy-id>` for non-mutating policy dry-runs; append `--run` only when an operator intentionally executes the selected checks. These gates keep production auth, cache invalidation, production readiness, source-writing readiness, optional personnel readiness, external reference coverage, required env vars, runtime validation snippets, production session-policy spec evidence, Provider Promotion Matrix evidence, admin UI contract drift tests, docs, declared preflight commands, policy-level preflight requirements, draft-only promotion evidence templates, `resources/generated/platform-operations-plan.json` and `resources/generated/production-auth-promotion-review.json` consistent.

Production operation policies are declared in `resources/platform-production-readiness.json` and checked by the same validator:

- `config-backup-export` requires reviewed backup/export artifacts before production configuration changes or generated-contract promotion, and must keep capability-contract, capability-profile, cache-invalidation and task-execution-audit checks attached because settings, branding, menus, principals and schemas can be cached.
- `config-import-restore` keeps destructive import/restore out of the default runtime until a workflow has dry-run, diff, human review, audit, capability-contract, capability-profile, cache invalidation and rollback gates.
- `database-migration` ties schema changes to capability lifecycle declarations, capability-contract validation, capability-profile validation, task-execution-audit visibility, GORM-backed lifecycle history and a backup or forward-fix plan.
- `token-rotation` requires session-impact review, cache/session invalidation decisions, task-execution-audit visibility, admin UI contract drift checks and secret-redacted audit records before changing JWT secrets or `pgo_` API-token material.

These policies are deliberately stricter than the generic admin resource API. They prevent a reusable platform base from shipping unsafe production mutation paths while still documenting how a deployment can add a reviewed operations workflow later.

Policy-level required gates live in `policyPreflightRequirements` inside `resources/platform-production-readiness.json`; they are not validator-private constants. The generated operations plan includes each policy's `requiredPreflightCommands` and `missingRequiredPreflightCommands`, so production review can see whether capability-contract, capability-profile, cache-invalidation, task-execution-audit and admin-ui-contract-tests gates are still attached where required.

The generated operations plan is deliberately non-executable: `dryRun=true`, `runtimeMutation=disabled` and `sourceWriting=disabled`. Treat it as a checklist and review packet for production operators. It also carries the same `preflightRunner` metadata from `resources/platform-production-readiness.json`, so operators have one standard entrypoint for list, policy dry-run, selected command dry-run and explicit execution. The plan also includes the Provider Promotion Matrix from `resources/platform-production-auth-hardening.json`, including each provider's production usage, config keys, secret-owner requirement, rotation-runbook requirement and raw credential/subject exposure flags. The same generated plan carries the production auth promotion approval package and completed-evidence schema so release review can see required approvers, required evidence, empty completed evidence, prohibited evidence, rollback commands, audit sample refs, provider rotation runbook refs, refresh-token-family test refs, provider IDs, provider controls, runtime test refs, artifact hash policy and artifact URI policy before any runtime change. The dedicated production-auth promotion review packet lists active blockers and missing evidence for token rotation; it is evidence for review readiness, not approval. If external reviewers produce a completed promotion evidence package, run `rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>` before any promotion discussion; the validator checks required evidence IDs, owners, approval separation, external absolute artifact URIs, `sha256:` plus 64 lowercase hex artifact hashes, `rtk` verification commands, rollback commands, production environment fields, production-auth provider/control/test coverage and forbidden sensitive fields, but it does not mutate approval state. If a future deployment needs real restore, migration, token-rotation or provider-promotion execution, add that as a separate reviewed capability with diff, approval, rollback, audit and test evidence instead of extending generated plans into mutating APIs.

## Code Intelligence Rules

Keep `.codegraph/` local and ignored. It is generated cache, not source.

Use CodeGraph before changing shared platform contracts, UI primitives, repositories, authorization or route registration:

```bash
rtk codegraph sync .
rtk codegraph status
```

After structural edits, refresh the index before making further architecture or impact claims.

## Review Checklist

Before merging a capability:

- manifest ID, dependencies and version are stable;
- admin resources have titles, descriptions, menu metadata and permission prefixes;
- custom fields have labels, valid sources, valid types and either options or relation metadata when required;
- lifecycle steps have stable lowercase hyphenated IDs, no migration/seed ID collisions and idempotent bodies;
- demo data is optional and safe to reapply;
- disabled behavior is covered by tests;
- docs mention configuration, APIs and persistence assumptions;
- no platform core or optional platform capability package imports business-domain packages.
- downstream composition roots, not platform core packages, inject concrete business manifests.
