# Admin Resource Schema

Date: 2026-07-04
Last updated: 2026-07-12

## Purpose

Admin resources are declared through a backend-owned schema. The schema is the contract used by the generic admin resource engine to render lists, search records, build forms, validate required fields and expose permission codes.

The backend now owns menu availability through `GET /api/admin/menus`. The frontend still owns icon rendering and layout behavior. The backend schema owns resource behavior and action permission codes.

## API

```text
GET /api/admin/resources/:resource/schema
POST /api/admin/resources/:resource/query
GET /api/admin/resources/:resource
POST /api/admin/resources/:resource
PUT /api/admin/resources/:resource/:id
DELETE /api/admin/resources/:resource/:id
```

The response returns:

- `resource`: resource key, such as `api-resources`;
- `title` and `description`: localized text;
- `permissions`: action permission codes for `read`, `create`, `update`, `delete`;
- `formGroups`: optional localized form sections for schema-driven create/edit dialogs;
- `formLayout`: optional layout preset, currently `single-column`, `grouped-sections`, `two-column-density` or `side-detail-preview`;
- `runtimeSlots`: optional descriptor-only form insertion points resolved by frontend registries;
- `fields`: field contracts;
- `searchFields`: fields used by the generic search box;
- `defaultSortKey`: optional default sort field.

`POST /api/admin/resources/:resource/query` accepts a structured payload:

```json
{
  "keywords": ["admin"],
  "conditions": [{ "field": "status", "operator": "=", "value": "enabled" }],
  "sort": [{ "field": "updatedAt", "order": "desc" }],
  "page": 1,
  "pageSize": 10
}
```

The response returns `items`, `total`, `page` and `pageSize`. `pageSize` is capped by the backend so large list pages cannot force unbounded reads.

## Field Contract

Each field declares:

- `key`: stable field key;
- `label`: localized display label;
- `type`: currently `text`, `textarea`, `select`, `multiselect`, `datetime`, `switch`, `number`, or `color`;
- `source`: `record` for built-in record fields or `values` for extensible resource attributes;
- `group`: optional form group key. When present, the generic form renders the field under the matching `formGroups` section;
- `help`: optional localized helper text shown below the form control;
- `required`: backend validation and frontend form rule;
- `searchable`: whether it can participate in search;
- `filterable`: whether it can be used in structured conditions and advanced filters;
- `sortable`: whether table sort requests may target this field;
- `localizable`: whether the generic engine should also inspect `<key>Zh` and `<key>En` values for display/search extension;
- `inTable`: whether it renders as a table column;
- `inForm`: whether it renders in create/edit forms;
- `inDetail`: whether it renders in the right-side detail inspector;
- `width`: preferred table column width;
- `options`: select or multiselect options for enum-like fields;
- `relation`: optional dynamic option source for `select` or `multiselect` fields. It declares `resource`, `valueField`, `labelField`, optional `filters`, `sortField`, `sortOrder`, `multiple`, `display`, `parentField`, `pathField` and `rootValue`;
- `validation`: optional form validation metadata. Supported keys are `minLength`, `maxLength`, `min`, `max` and `pattern`.
- `sensitivity`: `public`, `internal`, `personal`, `sensitive` or `secret` classification;
- `storageMode`: `plain`, `masked`, `hashed` or `encrypted` storage contract;
- `responseMode` and `exportMode`: `full`, `masked`, `privileged` or `omitted` projections applied before values leave the Store;
- `protection`: required only for `storageMode=encrypted`; declares `format`, `normalization` and optional `blindIndexNamespace`.
- `reveal`: optional only for recoverable encrypted fields; declares a registered `policyId`, dedicated `permission` and `copyAllowed` decision.

## Configurable Encrypted Fields

Sensitive fields are not identified by a built-in list of names. Any capability-owned `values` field can opt into recoverable protection by declaring `storageMode=encrypted`, field protection metadata and a resource protection context. Names such as `phone`, `email`, `identityNumber`, `address` and `governmentReference` have no runtime meaning.

```go
capability.AdminResource{
    Resource: "customer-profiles",
    Protection: &capability.AdminResourceProtection{
        SchemaVersion: 1,
        Scope:         "tenant-field",
        TenantField:   "tenantCode",
    },
    Fields: []capability.AdminField{
        {
            Key: "governmentReference", Source: "values",
            Sensitivity: capability.FieldSensitivitySensitive,
            StorageMode: capability.FieldStorageEncrypted,
            ResponseMode: capability.FieldProjectionPrivileged,
            ExportMode: capability.FieldProjectionOmitted,
            Protection: &capability.AdminFieldProtection{
                Format: "aes-256-gcm-v1",
                Normalization: "trim-v1",
                BlindIndexNamespace: "customer-government-reference",
            },
        },
    },
}
```

Supported normalizers are `raw-v1`, `trim-v1`, `email-v1`, `phone-e164-cn-v1` and `identity-cn-v1`. The manifest selects the rule explicitly; the field name never selects it. An empty `blindIndexNamespace` disables querying. A declared namespace enables only structured `=` conditions; encrypted fields cannot participate in keyword search, range conditions or sorting. `Scope=global` uses the stable platform sentinel. `Scope=tenant-field` requires a declared, required, plain tenant field whose value cannot change after encrypted data exists.

The Store assigns the record ID before encryption, creates an AES-256-GCM envelope, and persists the blind-index metadata inside that envelope. Ordinary response and export projection remain masked or omitted. `ProjectRecordPrivileged` authorizes each field before calling `Reveal`; HTTP access exists only through the manifest-declared reveal policy, dedicated permission, active Admin session, step-up challenge and one-time scoped grant. Hashed fields remain one-way and are never revealable. File, SQL and GORM repositories persist opaque envelopes without understanding the field policy.

Reveal policies live in `AdminSurface.RevealPolicies`, not in frontend code. A policy declares `anyOf` or `allOf`, versioned factors (`oidc-reauth-v1`, `admin-sms-otp-v1`), localized purposes, challenge TTL and grant TTL. The generated Admin contract and OpenAPI preserve the field policy. Plaintext is returned only by `POST /api/admin/resources/:resource/:id/fields/:field/reveal` after consuming the matching grant; list, query, detail, Tooltip and export responses never reuse that privileged result.

The complete server-owned reveal flow is:

```text
GET  /api/admin/resources/:resource/:id/fields/:field/reveal-policy
POST /api/admin/resources/:resource/:id/fields/:field/reveal/challenges
POST /api/admin/resources/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/oidc/start
POST /api/admin/resources/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/oidc/complete
POST /api/admin/resources/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/sms/start
POST /api/admin/resources/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/sms/complete
POST /api/admin/resources/:resource/:id/fields/:field/reveal
```

Failed factor verification returns `422` without clearing the active Admin session. Challenges and grants are short-lived, grants are single-use and scoped to actor/session/resource/record/field/purpose, and all reveal responses are `Cache-Control: no-store`.

Format, normalization, namespace, tenant scope and schema version are compatibility contracts once data exists. Startup authenticates stored envelopes and fails when policy drifts or a referenced historical key is missing or replaced.

Historical migration is implemented through the offline `platform-admin sensitive-data-migrate` command. The plan includes every encrypted field declared by enabled manifests when `Source="values"`; it never infers sensitivity from field names. Inventory, dry-run and verify are read-only and never create journal tables. `prepare` is the sole journal-creation mode, while apply uses bounded tenant cursors, encrypted escrow, append-only events, checkpoints and global revision compare-and-swap. Encrypted fields duplicated into normalized columns or relation tables fail physical-layout validation. Ordinary Store loading continues to reject plaintext, and no migration HTTP endpoint exists. See [Sensitive Data Historical Migration Runbook](platform-sensitive-data-migration.md).

## Current Behavior

- Backend validates required fields before creating or updating records.
- Backend rejects unknown `values` keys and validates every declared field against its explicit `sensitivity`, `storageMode`, `responseMode`, `exportMode` and `protection` policy before persistence. Field names never change write, query, storage or projection behavior. External writes cannot populate internal/personal/sensitive/secret or read-only fields unless a writable field explicitly declares encrypted storage and the Store has a protection runtime.
- List, query, detail and export responses are rebuilt from declared schema fields. Internal and protected values do not leak through a cloned `Record.Values` map, and any projection failure aborts the response rather than falling back to raw values.
- Generic create, update and delete persist the business record and redacted audit record in one repository snapshot. A repository or audit validation failure restores the previous snapshot.
- Backend checks resource action permissions before schema, query, list, create, update and delete responses.
- Frontend `GenericResourceConsole` reads the schema and dynamically builds:
  - table columns;
  - search fields;
  - create/edit form sections and form items;
  - detail inspector rows;
  - permission code display.
- Create/edit dialogs use `PlatformResourceForm` plus `formGroups` for lightweight sections. Fields without a group stay in an additional-fields section when any grouped fields exist; schemas without grouping keep the old flat form behavior.
- `resources/platform-form-schema-layout-slots.json` is the current form layout and slot gate. `single-column`, `grouped-sections`, `two-column-density`, `side-detail-preview`, controlled source-level React slots and controlled runtime slot descriptors are implemented through the shared form wrapper, schema field metadata and a frontend slot registry. Dense forms use `AdminFormModal` viewport-safe internal scrolling and collapse to one column on narrow screens. `side-detail-preview` renders registered `side.preview` slots beside the form on desktop and below the form on mobile. Backend manifests may declare only descriptor data such as `slotId`, `region`, localized labels, permission and data binding fields; React component names, component paths, raw scripts and source-writing generators remain forbidden.
- `help` and `validation` are optional. Missing metadata does not change existing resource behavior. When present, the frontend maps validation metadata to Ant Design form rules and still relies on backend validation for authoritative writes.
- Filterable `datetime` fields render date range filters, and filterable `number` fields render numeric range filters through `PlatformDataTable`.
- Fields default to compatible behavior: searchable fields are filterable, status/select/multiselect/switch/datetime/number fields are filterable, table fields and datetime/number fields are sortable unless the generic engine would produce poor behavior such as sorting textarea or multiselect content.
- `GenericResourceConsole` uses Refine CRUD hooks for list/create/update/delete. The Refine data provider is the only frontend layer that calls the backend generic resource APIs, including the query endpoint for pagination, filters, sorting and safe keyword/condition meta.
- `select` and `multiselect` fields may use static `options`, dynamic `relation`, or both. Static options are fallback values; dynamic relation options come from the same structured resource query API.
- Relation fields with `display: "tree"` render through the shared `PlatformTreeSelect` in both forms and advanced filters. Tree relations must declare `parentField`; `pathField` is optional and is used only for clearer labels. The submitted value remains the relation `valueField` code.
- `relation` fields still store their selected `valueField` code in the owning record. `multiselect` relation values are stored as comma-separated values in `Record.values` for the current generic store, while the UI renders them as multiple selectable tags.
- Frontend hides create, edit and delete actions when the current session lacks the matching action permission or the matching permission is explicitly denied by `roles.denyPermissions`.
- `values` fields are stored in `Record.values` and can be promoted to first-class fields later without changing the generic page engine.
- `switch` values are stored as `"true"` or `"false"` strings in `Record.values` or record fields. Lists render them as read-only switches by default; edit forms render a real `Switch`.
- The current migration-source schema declares organization and area relations through `tenants.areaCode`, `org-units.tenantCode`, `org-units.parentCode`, `org-units.areaCode`, `users.tenantCode`, `users.orgUnitCode`, `users.areaCode`, and `area-codes.parentCode`. `org-units.tenantCode` and `users.tenantCode` are currently required, while `users.orgUnitCode`, `users.areaCode`, `tenants.areaCode` and `org-units.areaCode` remain optional. These fields continue to feed existing data-scope filtering during the organization contract migration. Address-code fields remain optional reusable dimensions; detailed addresses stay capability-owned.
- The current migration-source RBAC schema exposes nested `role-groups.parentCode`, assigns roles through `roles.groupCode`, binds users through `users.roles`, and derives menu visibility from role permissions. Roles own allow permissions, deny permissions and data scope; role groups are classification-only and do not grant or inherit authorization. These behaviors are migration inputs, not the activated target model.
- The organization target backend in `docs/platform-organization-rbac-menu-contract.md` is implemented behind `PLATFORM_ORGANIZATION_RBAC_MODE=target` and required in production. It requires one primary organization for each ordinary user, derives tenant ownership server-side from that organization, and defines an explicit exception for platform-level users without an organization. An organization may bind multiple enabled role groups from the same tenant. Role groups form a strict non-nested `role group -> role` tree, each role belongs to exactly one role group, and the organization's effective role pool is the deduplicated union of enabled roles in its enabled directly bound groups. Backend create/update, relation mutation, service-command and migration paths enforce that user roles remain a subset of the pool. `organization-user-admin-experience` removes editable tenant selection and implements organization-dependent role selectors.
- The activated menu target separates directory menus from routable page menus and persists role-to-menu visibility independently from API and page-button permissions. Roles continue to own allow/deny permissions and data scope, while Casbin remains the final protected-API boundary. Organization changes, role-group unbinding, role movement or disablement, and migration from permission-derived menus require explicit conflict handling and audit; they must not silently retain invalid grants or discard existing access.
- `users` is the platform account and principal resource. Personnel files, employees, staff profiles, positions and position assignments are optional capability resources, not default foundation resources. When the `personnel` capability is enabled, `personnel-profiles` must declare `tenantCode`, `orgUnitCode`, optional `areaCode` and optional `userCode` relations; `positions` and `position-assignments` reuse shared tenant/org unit relations rather than creating a separate organization model.
- The legacy Admin surface still uses the current `users.roles` multiselect relation and accepts legacy `role` values for compatibility snapshots. Target mode keeps multi-role selection backend-validated; the Admin loads organization-dependent role options, shows role-group provenance and preserves out-of-pool selections as explicit invalid values until the operator removes them. Backend validation remains authoritative.
- `audit-logs` is a system-written, read-focused resource. Its exposed structured fields are `actor`, `action`, `resource`, `targetId`, `provider`, `outcome`, `eventId`, `reasonCode` and `createdAt`; list/query pages can search and sort only declared fields, while the form field set stays empty. Legacy `targetCode` and `traceId` remain internal compatibility fields with omitted response/export projections. The audit schema does not expose `sessionId`, raw session handles, session digests or shortened derivatives. Audit records also omit target labels, raw errors, personal values, object paths and credential material.
- The optional policy-review export route requires `admin:policy-review:export`, independently from read access. It projects every review and audit record with `ProjectionExport`; one projection failure aborts the whole export. The Admin export button is not rendered for principals without that permission, so an unauthorized control cannot receive keyboard focus.
- App phone verification and binding are declared as `app-phone-verifications` and `app-phone-bindings`. They expose `appUsername`, `maskedPhone`, `phoneHash`, timestamps and status for governance; they must not store raw phone numbers or raw verification codes in generic records. The verification code is stored as `codeHash`; local/demo verification returns `debugCode` only from the create-verification App API response.
- Generic default resources reserve optional localized input fields in `values.nameZh`, `values.nameEn`, `values.descriptionZh` and `values.descriptionEn`. They are not table columns by default; the active language decides which value is displayed in the shared list and detail UI.
- Business resources are not forced to translate every row. When a business domain needs multi-language content, declare `localizable` on the field and store values as `<key>Zh` / `<key>En` in `Record.values`; the generic list/detail/search path uses the active language and falls back to the other language or canonical value. Promote to a translation table or domain-specific adapter only when volume or workflow requires it.

## Safe Query Contract

The list search box supports a SQL-like DSL for operator familiarity, but it is not SQL.

Supported examples:

```text
admin
name:admin
status=enabled
status!=disabled
updatedAt>=2026-01-01
updatedAt<=2026-12-31
updatedAt>2026-01-01
updatedAt<2026-12-31
```

Rules:

- field names must exist in the backend schema and be declared searchable/filterable before the backend executes a condition;
- sort fields must exist in the backend schema and be declared sortable;
- encrypted fields are excluded from keyword matching and sorting; they support only `=` when the manifest declares a `blindIndexNamespace` and the data-protection runtime is available;
- allowed operators are `contains`, `=`, `!=`, `>`, `<`, `>=` and `<=`; the UI maps `:` and `~` to `contains`;
- plain tokens become `keywords`; field expressions become structured `conditions`;
- UI parsing and backend parsing must convert tokens into structured conditions before filtering;
- raw query text must never be concatenated into database SQL.
- database-backed implementations must compile parsed conditions into parameterized GORM/database predicates; string concatenation is not allowed.
- UI components may expose SQL-like syntax hints, but they must keep sending the structured JSON query payload. Any future advanced query builder should compile into the same `conditions` array.
- Values that look like SQL, for example `enabled' OR '1'='1`, are treated as literal values. The backend regression test `TestStoreQueryTreatsSQLLikeValuesAsLiterals` locks this behavior.

The SQL-like input stays a UI convenience. The transport contract is the JSON query payload (`conditions`, `keywords`, `sort`, `page`, `pageSize`).

`resources/platform-admin-api-boundary.json` is the platform gate for this boundary. `scripts/validate-platform-admin-api-boundary.mjs` checks that admin source code uses `admin/src/platform/api/client.ts` as the only direct `fetch` and authorization layer, rejects admin frontend calls to App APIs, rejects query-string collection filters, verifies Refine `dataProvider` keeps passing structured query meta, and verifies generated OpenAPI query schemas expose field/filter/sort allowlists. Run it when changing admin API helpers, Refine CRUD adapters, generic list filters or backend query validation.

## Manifest Validation Gates

`scripts/validate-admin-resources.mjs` enforces the reusable contract:

- resource labels must declare `zh` and `en`;
- capability `PermissionPrefix` values must use `admin:<resource>` and custom action, panel or runtime slot permissions must stay under that resource prefix;
- every field label must declare `zh` and `en`;
- field keys must be unique within a resource;
- every form group key must be unique, every form group label must declare `zh` and `en`, and form group descriptions must declare `zh` and `en` when present;
- every field help text and static select option label must declare `zh` and `en` when present;
- field `group` values must reference declared `schema.formGroups` when form groups are declared;
- field validation metadata must use numeric bounds and string regex patterns;
- field `relation` metadata must declare a resource provided by the enabled manifest set. `ResolveEnabled` rejects relation targets whose provider capability is disabled or missing, and rejects relation value, label, filter, sort, parent or path keys that are not exposed by the target resource. Standard record fields `id`, `code`, `name`, `status`, `description` and `updatedAt` are always referenceable; other relation metadata fields must be declared by the target resource manifest. Multi-relation fields must use `multiselect`; single relations must use `select`; tree relations must declare `parentField`; relation filters may only use `contains`, `=`, `!=`, `>`, `>=`, `<` or `<=`;
- `searchFields` and `defaultSortKey` values must reference declared fields or standard record fields;
- internal menu paths must start with `/`;
- external menu paths must start with `http://` or `https://`;
- `visible`, `hidden`, `external` and `keepAlive` must be explicit booleans;
- current validators intentionally preserve the migration-source fields during the organization migration: `users.tenantCode` and `org-units.tenantCode` are required, `users.orgUnitCode` remains optional, nested `role-groups.parentCode` is still accepted, and `roles.groupCode`, role permission and data-scope fields remain hard gates. These checks are current-state compatibility evidence, not completion evidence for the activated organization target;
- `resources/platform-governance-topology.json` currently checks default foundation inclusion/exclusion, keeps `org-units` in the default foundation, keeps tenant `areaCode` optional, blocks personnel resources from default contracts, verifies shared personnel relations, and prevents role groups from acquiring permission, membership, inheritance or data-scope semantics;
- the organization migration completion gates must atomically replace the incompatible current checks: ordinary users require one primary organization, tenant is derived and same-tenant integrity is enforced, role groups are non-nested, each role belongs to one group, organizations bind multiple same-tenant groups, user roles are constrained to the effective organization role pool, role-menu bindings are independent, and API/page-button permissions remain separate. Migration validators must distinguish legacy snapshots from new writes and must not claim completion until data migration, conflict handling, generated contracts and backend enforcement pass together;
- personnel and position resource names listed in `resources/platform-reference-coverage.json` cannot appear in the default platform contract unless an explicit `personnel` capability is enabled;
- generated contract, OpenAPI, codegen preview, scaffold safety plan, generated scaffold file package, scaffold draft and scaffold promotion review packet must stay fresh;
- the merged generated resource contract must reject schema-empty default resources. Every generated resource must declare schema fields, table fields and permission codes; queryable resources must also declare search, filter and sort fields. Capability-contributed resources therefore cannot expose only menus and permissions without a usable list/query schema;
- scaffold safety plan must keep source writing disabled, run as dry-run, restrict candidates to `resources/generated/scaffold/` and report zero path conflicts;
- future source-writing generators must pass `resources/platform-codegen-source-writing-readiness.json`; source writing stays disabled until a separate spec, platform/codegen/runtime/operations owner approvals, completed-evidence artifact schema, scaffold promotion review, reviewed diff, rollback plan and target-family test mapping are approved;
- future form layout or slot generators must pass `resources/platform-form-schema-layout-slots.json`; arbitrary runtime component paths, raw script slots, unlocalized slot labels and backend manifest React component names are forbidden in the shared foundation; source-level React slots must stay owned by application code around `PlatformResourceForm`;
- `resources/platform-engineering-capabilities.json` must keep platform engineering coverage tied to real source files, generated files, admin resources, OpenAPI paths and scaffold safety constraints.

This keeps dynamic menus, Refine resources, permission codes, OpenAPI and future code generation aligned from one manifest.

Use the default command for the platform-owned manifest and generated freshness gate:

```bash
rtk node scripts/validate-admin-resources.mjs
```

Use `--manifest` for a copied or generated manifest during tests or capability review:

```bash
rtk node scripts/validate-admin-resources.mjs --manifest /tmp/admin-resources.json
```

`--manifest` validates the manifest structure, relation targets, platform governance fields and i18n contract without checking generated artifact freshness. This is useful for negative tests and external capability manifests that should fail fast before they are copied into `resources/admin-resources.json`.

Use `--contract` when reviewing an externally generated admin resource contract package:

```bash
rtk node scripts/validate-admin-resources.mjs --manifest /tmp/admin-resources.json --contract /tmp/admin-resource-contract.json
```

`--contract` applies the generated-contract schema usability gate without promoting the package or writing source files.

## OpenAPI Contract

The admin API documentation is generated from the same resource contract:

```bash
rtk node scripts/generate-admin-resource-contract.mjs
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk go run ./cmd/platform-contracts app-routes --output resources/generated/app-route-contract.json
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/generate-admin-scaffold-promotion-review.mjs
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
```

The output file is `resources/generated/openapi.admin.json`. It includes resource routes, permission codes, audit action hints, field schemas, `x-platform-relation` metadata and a structured query payload. Generated paths are normalized to the current Gin resource engine shape, such as `POST /api/admin/resources/tenants/query`. `scripts/validate-admin-resources.mjs` treats this file as generated output and fails when it is stale.

Capability-level admin resources are generated separately from enabled Go manifests into `resources/generated/admin-capability-resource-contract.json`. That file comes from `capability.Manifest.Admin.Resources` through `cmd/platform-contracts admin-resources` and is checked by the validator. The main `resources/generated/admin-resource-contract.json` merges static platform resources with enabled capability resources that are not already provided by the platform base, allowing optional business plugins to add OpenAPI/codegen/scaffold-visible resources without expanding `resources/admin-resources.json`.

The Gin runtime exposes the configured document through:

```text
GET /api/openapi.json
```

`PLATFORM_OPENAPI_FILE` defaults to `resources/generated/openapi.admin.json`. The route requires `admin:api-docs:read` and returns `OPENAPI_NOT_CONFIGURED` when the server starts without an OpenAPI document.

Business-facing App API route declarations are generated separately from Go capability manifests into `resources/generated/app-route-contract.json`. That file comes from `capability.Manifest.App.Routes` through `cmd/platform-contracts app-routes` and is checked by the admin resource validator so app route docs/codegen do not drift from the manifest. App routes must use static `/api/app/` paths without query strings or fragments, localized descriptions, explicit `public` or `session` auth, and optional `app:<domain>:<action>` permission codes when business authorization is needed. `resources/generated/openapi.app.json` and `resources/generated/app-codegen-preview.json` consume the same contract and keep App API documentation separate from the admin security domain.

## Persistence Boundary

The generic Store owns resource behavior, validation, ID generation and menu/session integration. Persistence is delegated to the `AdminResourceRepository` snapshot port:

```go
type AdminResourceRepository interface {
    Load(context.Context) (ResourceSnapshot, error)
    Save(context.Context, ResourceSnapshot) error
}
```

The Store is in-memory by default. `PLATFORM_ADMIN_RESOURCE_FILE` enables a JSON file-backed repository:

```bash
PLATFORM_ADMIN_RESOURCE_FILE=.platform/admin-resources.json
```

`PLATFORM_ADMIN_RESOURCE_DRIVER` and `PLATFORM_ADMIN_RESOURCE_DSN` enable the GORM-backed repository. Supported driver values are `mysql`, `postgres` and `sqlite`:

```bash
PLATFORM_ADMIN_RESOURCE_DRIVER=mysql
PLATFORM_ADMIN_RESOURCE_DSN=$PLATFORM_ADMIN_RESOURCE_DSN_SECRET
```

Selection order is GORM driver, file, then memory. The GORM adapter creates `platform_admin_resource_records` and `platform_admin_resource_state`, and persists generic resource snapshots through the shared GORM storage opener. The older `database/sql` adapter remains behind the same repository port for compatibility tests, but the target runtime path is GORM.

Mutation-plus-audit APIs use this snapshot as their transaction boundary. Create/update/delete either save both records and advance the revision, or restore the complete prior in-memory snapshot when `Save` fails. Recoverable delete writes platform-owned lifecycle state and its audit atomically; normal reads omit the record until a dedicated restore clears that state or maintenance performs final purge. File delete only tombstones metadata during the request. Object cleanup is claimed later by maintenance, and metadata is retained until object deletion or an idempotent not-found result is durably recorded.

The GORM adapter now stores core system resources in normalized tables while preserving the same `ResourceSnapshot` API:

- `platform_admin_users`
- `platform_admin_user_roles`
- `platform_admin_tenants`
- `platform_admin_org_units`
- `platform_admin_roles`
- `platform_admin_role_groups`
- `platform_admin_role_permissions`
- `platform_admin_permissions`
- `platform_admin_menus`
- `platform_area_codes`
- `platform_audit_logs`
- `platform_login_logs`
- `platform_error_logs`
- `platform_versions`

Non-core or business resources still use `platform_admin_resource_records`. This keeps capability resources lightweight while making tenants, org units, role groups, area codes, users, roles, permission policies, menus and operations records independently queryable and auditable. The current migration-source load/save boundary maps those normalized rows back to the existing `Record` shape, so `/api/admin/resources/*`, permission-derived dynamic menus and Casbin policy generation do not need a frontend contract change before the organization migration begins.

`app-identities` is intentionally a generic platform resource at this stage. It stores provider, provider kind, provider scope, app username, timestamps, a provider subject hash and a masked subject. `app-phone-verifications` and `app-phone-bindings` follow the same rule for phone numbers: store hashes and masked values only. These resources must not store raw OpenID, UnionID, phone, token, verification code or provider subject values. A future production adapter may move the same contracts to normalized tables, but the HTTP/resource surface should remain stable so business app login does not couple to admin users or role records.

The optional `admin-identities` resource is the Admin OIDC equivalent, but it is an explicit binding to an existing platform username rather than an account-provisioning model. It stores provider metadata, issuer and provider-subject hashes, binding status, platform username and timestamps. It must not store raw issuer or subject values, claims, authorization codes, provider tokens, state, nonce, PKCE verifier/challenge material or credentials. The binding does not create users, roles, permissions, tenants, organizations or areas.

Snapshots store records and the next generated ID. Schemas are not stored; they are always regenerated from the enabled capability manifests. During migration this keeps capability declarations authoritative while allowing current user, role, menu, permission and business-resource edits to survive API restarts. The activated organization nodes must migrate these snapshots and normalized tables explicitly rather than treating the current shape as the target contract.

When a file snapshot is loaded, unknown resources are ignored. Resources missing from the snapshot still use the current enabled capability seeds. Generated `menus` and `permissions` also keep newly declared capability entries, so enabling a new capability can still add its admin navigation and permission catalog entries even when an older snapshot exists.

## Extension Rule

New platform or business resources should first declare themselves through `capability.Manifest.Admin.Resources`. Add schema code in `internal/platform/adminresource/schema.go` only when the default schema is not enough or when the resource needs custom field definitions.

New menu entries should be declared through the `menus` admin resource contract and consumed by the shell without frontend registry edits. The current migration-source runtime derives menu filtering from role permissions; the activated target keeps the manifest-owned menu catalog but moves visibility to independent role-menu bindings while API and page-button permissions remain separate.

Use raw custom pages only when a resource cannot be expressed as standard fields, list actions, forms and detail inspectors. If multiple custom resources repeat the same UI behavior, promote that behavior into the generic resource engine instead of branching on resource names.
