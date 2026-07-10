# Admin RBAC And Dynamic Menu

Date: 2026-07-04
Last updated: 2026-07-06

## Purpose

The admin foundation now has a runtime RBAC slice for platform menus and generic admin resources. Menu records are generated from enabled capability manifests, then filtered by the current principal.

This slice turns resource permission codes into executable behavior:

- the current principal is resolved from platform user and role records;
- user role bindings prefer the `roles` value field, while legacy `role` values remain readable for old snapshots;
- role permissions support exact codes, resource wildcards such as `admin:tenant:*`, domain wildcards such as `admin:*`, and global `*`;
- role groups are a role-management classification dimension through `roles.groupCode`; they do not grant permissions, own role membership, carry data scopes or create role inheritance in the base model;
- HTTP permission enforcement is backed by Casbin policies generated from the dynamic `users` and `roles` resources, with the default local authorizer refreshed after role, user and permission writes;
- available permission codes are generated from enabled capability admin resource declarations and exposed as the `permissions` admin resource;
- admin menus are returned by the backend and filtered by the current principal;
- generic admin resource `read`, `create`, `update`, and `delete` operations are checked on the backend.

## Auth Boundary

Authentication has a unified provider boundary. The bundled `demo` provider is for local development and verification, while the HTTP credential boundary is JWT by default.

- `GET /api/auth/providers` returns enabled provider declarations from capability manifests.
- `POST /api/auth/login` currently supports the configured `demo` provider and returns a JWT admin bearer token plus `expiresAt`.
- The bearer token carries an internal server-side session id, so TTL and revocation remain authoritative.
- `POST /api/auth/refresh` renews a still-active admin session and returns a new JWT while keeping the same server-side session id.
- `POST /api/auth/logout` revokes the server-side session referenced by the bearer token.
- `GET /api/admin/session/current` resolves the current user from `Authorization: Bearer ...` first.
- If a bearer token is present but expired or revoked, the backend returns `401`.
- If no bearer token is present, the default backend returns `401`; it does not implicitly fall back to `admin`.
- `X-Platform-User` is retained only for tests or explicitly controlled local harnesses that set `httpapi.ServerOptions.AllowInsecureHeaderAuth`.
- The seed `admin` user has `super-admin`, and `super-admin` has `*`.
- The seed `ops` user has `operator`, and `operator` has read-only permissions for capabilities, tenants, and monitoring.
- Updating a role record's `permissions` value changes the effective permissions used by sessions, menu filtering and resource authorization.

This is enough to validate role-linked menus and backend authorization without coupling the platform base to a final login provider. Sessions use a repository-backed store, with memory, file-backed and GORM-backed modes available while keeping the auth provider, JWT, refresh, session, menu and permission APIs stable.

## APIs

```text
GET /api/admin/session/current
GET /api/admin/menus
```

`GET /api/admin/session/current` returns:

- `user`: current platform user, including `tenantCode`, `orgUnitCode` and `areaCode` when the user resource declares them;
- `roles`: role codes assigned to the user;
- `permissions`: effective role permissions.

`GET /api/admin/menus` returns only menu items whose `permission` is allowed by the current principal.

## Menu Contract

Menus are stored as the `menus` admin resource. In the default runtime they are seeded from `capability.Manifest.Admin.Resources`. Each record declares:

- `route`: frontend route;
- `resource`: resource key, such as `tenants`;
- `permission`: permission required to see the menu entry;
- `group`: shell group, such as `foundation` or `governance`;
- `icon`: shell icon key;
- `order`: sort order;
- localized title and description fields.

The frontend still owns icon rendering and layout behavior. The backend owns which menu items exist for the current principal.

Disabled capabilities do not contribute menu rows or admin resource schemas.

## Enforcement

Menu filtering is not the security boundary. The backend also checks resource action permissions:

- schema and list require `schema.permissions.read`;
- create requires `schema.permissions.create`;
- update requires `schema.permissions.update`;
- delete requires `schema.permissions.delete`.

Frontend buttons hide create, edit and delete actions based on the current session permissions, but backend checks remain authoritative.

The current HTTP enforcement path builds a Casbin authorizer from platform role records and keeps it as a local server cache. Successful writes to `roles`, `users` or `permissions` clear that local authorizer plus principal and dynamic-menu caches, so existing admin sessions see updated role policies without logging in again. When Redis cache mode is enabled, the platform publishes resource invalidation events so peer API instances clear the same local policy and read caches. This keeps the same admin resource contract while moving the execution engine to the target stack. In GORM mode, tenants, org units, users, user-role bindings, role groups, roles, role-permission bindings, permissions, menus and area codes are persisted through normalized tables and mapped back to the generic resource API contract.

## Configuration Model

The platform does not use a direct role-menu binding table. The model is:

```text
user -> roles -> permissions / denyPermissions -> menus/resources/actions
```

Menus, resource actions and future buttons or APIs all declare permission codes. Roles grant permission codes through `roles.permissions` and can explicitly deny permission codes through `roles.denyPermissions`. A menu is visible when the current principal has the menu's required permission and no deny rule matches it.

Role groups sit beside this chain as management metadata:

```text
roleGroups.parentCode -> roleGroups
roleGroups -> roles.groupCode
```

They make large role catalogs easier to organize, including multi-level role-group catalogs, but they do not grant permissions by themselves. This keeps Casbin policy evaluation simple and avoids accidental broad access. The admin resource validator requires `role-groups.parentCode` as a tree relation and rejects permission, membership, inheritance and data-scope fields on `role-groups`. `roles.permissions`, `roles.denyPermissions` and `roles.dataScope` remain the only role policy owners; `users.roles` remains the user-to-role membership owner. If a future project needs role inheritance, role templates or grouped membership operations, add that as an explicit policy feature with precedence tests instead of hiding it inside role groups.

Permission precedence is explicit:

```text
denyPermissions > permissions > no match
```

For example, a role can grant `admin:*` while denying `admin:tenant:read`; the user can still access other admin permissions, but tenant reads, tenant menu visibility and tenant resource queries are blocked. Deny rules are action permissions only. They do not create data ownership scopes and do not replace `dataScope`.

Roles also declare `dataScope` as required role metadata for new writes. Supported declaration values are `all`, `current_org`, `current_and_children`, `custom_orgs`, `current_area`, `current_and_children_areas`, `custom_areas` and `self`. This field is persisted and editable through the generic roles resource. It is not an authorization grant: Casbin still decides whether an action is allowed, while the admin resource store applies data-scope filtering to human admin list/query calls after the read action is allowed. Multiple roles are unioned within the same scope dimension, and `all` wins. If organization and area dimensions are both declared, records that carry both fields must pass both dimensions. `custom_orgs` reads `roles.dataScopeOrgCodes`, an org-unit tree relation, and `custom_areas` reads `roles.dataScopeAreaCodes`, an area-code tree relation. Legacy role records without `dataScope` are read with the previous unscoped behavior for compatibility, but role edit/create forms must send an explicit value.

Organization and area references are available as resource metadata:

```text
tenants.areaCode
org-units.tenantCode + org-units.parentCode + org-units.areaCode
users.tenantCode + users.orgUnitCode + users.areaCode
area-codes.parentCode + area-codes.level + area-codes.path
```

These fields support tenant, institution/department, account-principal and regional administration. Org units are a default platform governance resource, not an optional tenant-only shortcut: every org unit is tenant-owned through required `org-units.tenantCode`, while `org-units.type` distinguishes group, company, branch, organization, department, team, store and custom levels inside the same tree. User org-unit and area-code assignments stay optional so the same account model can support platform operators, service accounts and staged onboarding. Current-session principals expose the user's tenant, org unit and area codes when present. The data-scope layer filters tenant/org/area-owned resources such as tenants, org units, users, area codes and business resources that declare `tenantCode`, `orgUnitCode`, `areaCode` or self-owner fields. Hierarchical organization and area fields are rendered as tree relations in the generic admin UI. Address codes remain reusable regional master data, are optional ownership metadata by default, and regional permissions are opt-in through explicit `roles.dataScopeAreaCodes` values instead of being inferred from every address-code reference. Detailed street/contact addresses belong to the owning capability until at least two reusable platform capabilities need the same address model. Full personnel files, employee profiles, positions and employment assignments should be added through an optional `personnel` capability and should reuse these ownership fields when applicable.

The base account model intentionally supports one primary org relation through `users.orgUnitCode`. Reference-project multi-org membership tables such as `user_org_memberships` are classified as an optional extension boundary, not a default RBAC primitive. If a deployment needs users in several org units, add that through an explicit identity/personnel/consumer capability and keep data-scope semantics documented there.

The `permissions` resource is a generated catalog from enabled capability manifests. The `roles` schema exposes the same catalog through `permissions` and `denyPermissions` multiselect fields, so role authorization can be edited through the generic resource console without hard-coding menu names or business pages.

Current persistence boundary: tenants, org units, users, role groups, roles, user-role bindings, role-permission bindings, permissions, menus, area codes and operations logs are in-memory by default. Set `PLATFORM_ADMIN_RESOURCE_FILE` to use the file-backed admin resource repository for local persistence, or set `PLATFORM_ADMIN_RESOURCE_DRIVER` and `PLATFORM_ADMIN_RESOURCE_DSN` to use the GORM-backed repository. The GORM adapter stores these standard platform resources in normalized tables while mapping them back to the generic resource API contract.
