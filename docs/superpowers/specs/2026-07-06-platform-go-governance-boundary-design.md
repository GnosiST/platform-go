# Platform Governance Boundary Design

## Goal

Lock the default platform foundation to reusable governance primitives while preserving extension points for richer organization and personnel domains.

## Current State

The base platform already includes tenants, org units, users, role groups, roles and area codes. These resources are schema-driven, permission-checked and available through the generic admin resource API. The GORM adapter normalizes the standard platform resources while generic capability resources can remain manifest-backed.

## Boundary Decisions

1. `org-units` is the default tenant-owned organization tree. It belongs to `identity`, requires `tenantCode`, carries `parentCode` and optional `areaCode`, and feeds data-scope filtering when roles use organization scopes. `org-units.type` covers group, company, branch, organization, department, team, store and custom levels.
2. `role-groups` is classification and governance metadata for roles through `roles.groupCode`. It may use `parentCode` for a tree-shaped catalog, but it must not grant permissions, inherit policies, own role membership or define data scopes. `users.roles` owns role membership; `roles.permissions`, `roles.denyPermissions` and `roles.dataScope` own policy.
3. `area-codes` is shared regional master data. Tenants, org units, users and roles may reference area codes, but area-code assignment is not authorization. Region permissions are explicit role data scopes through `roles.dataScopeAreaCodes`.
4. `users` is the platform account and admin principal resource. It is enough for account, login, role binding and current-principal context.
5. Personnel files, employees, staff profiles, positions and position assignments are not default foundation resources. They should enter through an optional `personnel` capability only when a product needs HR-style modeling.
6. `platform-capability-profiles.json` must explicitly declare these resources in the relevant profile. `minimal-admin` and `platform-default` must include tenants, org units, users, roles, role groups and area codes. `platform-personnel-ready` must include personnel resources plus tenants, org units, area codes and users.

## Latest Assessment

- The foundation must not stop at tenant-only ownership. `tenantCode`, `orgUnitCode` and `areaCode` are the standard ownership fields that reusable and business resources should declare when those dimensions apply.
- Organization and department support should stay in one tenant-owned `org-units` tree instead of separate `organizations` and `departments` resources. `org-units.type` distinguishes common institution levels such as group, company, branch, organization, department, team, store and custom while preserving one tree relation and one data-scope implementation. The default foundation must include this resource so the base does not regress to tenant-only ownership.
- Role groups are useful for large role catalogs, permission review and UI organization. `role-groups.parentCode` supports multi-level catalogs such as security, operations and regional role families, but it should not become a policy engine or membership container. Permission grants, deny rules and data scopes stay on `roles`; role membership stays on `users.roles`; `role-groups` remains classification-only.
- Address-code and region governance should use `area-codes` as shared master data. Tenants, org units, users and future personnel records can reference `areaCode`; the field is a reusable optional ownership dimension by default, not a universal required field. Detailed street addresses or employment-location workflows belong to the owning capability unless at least two reusable platform capabilities need the same address model.
- Area-code assignment is data classification, not access control. Access still requires explicit role permissions plus optional `roles.dataScopeAreaCodes`.
- Profile-level declaration is now part of the governance contract. A profile cannot silently rely on audit output for organization, role-group or area support; the profile must state the required resources so deployments can reason about the enabled surface before startup.

## Extension Contract

Any future `personnel` capability should reuse the existing ownership fields when applicable:

- `tenantCode -> tenants`
- `orgUnitCode -> org-units`
- `areaCode -> area-codes`

This keeps row-level data scopes reusable and prevents business capabilities from inventing local organization or region fields.

The current optional `personnel` capability follows this contract. `personnel-profiles` references tenants, org units, area codes and linked platform users; `positions` references tenants and org units; `position-assignments` references personnel profiles, positions, tenants and org units. This gives products an HR-style model without promoting personnel workflows into the default platform surface.

The tightened 2026-07-07 rule is that organization units are always tenant-owned, while account-to-org and account-to-area bindings remain optional. This keeps department trees unambiguous in multi-tenant deployments without forcing every platform operator or service account into a department. The same rule must hold in both the static admin resource manifest and the Go manifest generated capability contract, so runtime contract generation cannot drift back to tenant-optional users or org units.

RBAC runtime contracts must also expose the full governance surface on `roles`: `groupCode`, `dataScope`, `dataScopeOrgCodes`, `dataScopeAreaCodes`, `permissions` and `denyPermissions`. This keeps role groups useful for catalog management while preserving explicit role-owned grants, deny rules and data scopes. Grouped membership and inherited roles remain separate future policy features, not hidden `role-groups` fields.

Do not add separate default `organizations`, `departments`, `employees` or `regions` resources unless repeated product use proves that `org-units`, `personnel-profiles` and `area-codes` are too shallow. Do not add detailed address fields to the default foundation until repeated reusable demand justifies promotion. The preferred extension path is to add fields or an optional capability behind the manifest/profile contract, then prove relation and data-scope behavior with tests.

## Validation

The default admin resource validator blocks regressions that shrink the base to tenant-only ownership. `resources/platform-governance-topology.json` is the explicit topology contract: it requires tenant, org-unit, role-group and area-code relations, keeps `org-units` required in the default foundation, keeps tenant `areaCode` optional by default, prevents role groups from becoming a hidden policy or membership engine, keeps detailed addresses out of the default foundation and keeps personnel resources out of the default contract. The governance-topology validator now checks both `resources/admin-resources.json` and `resources/generated/admin-capability-resource-contract.json`, so static schema and runtime Go manifest output must agree on tenant-owned org units, required user tenant ownership and role data-scope fields. The capability profile validator also blocks profiles that stop declaring organization, role-group, area or personnel governance resources in `mustIncludeResources`. The admin resource store tests verify organization, role-group and area relations plus the optional personnel capability's reuse of shared governance relations. The reference coverage validator blocks personnel or position resources from entering the default platform contract unless an explicit optional capability owns them.
