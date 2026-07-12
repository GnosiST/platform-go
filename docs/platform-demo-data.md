# Platform Demo Data

Date: 2026-07-04
Last updated: 2026-07-08

## Purpose

Demo data is a reusable platform capability for local demos, acceptance environments and product walkthroughs. Business capabilities can declare demo datasets in their capability manifest, and the platform exposes one consistent admin API to list and apply them.

## Manifest Contract

Capabilities declare demo data through `capability.Manifest.DemoData`:

```go
DemoData: []capability.DemoDataSet{
    {
        ID:          "platform-demo-tenants",
        Title:       capability.Text("平台演示租户", "Platform Demo Tenants"),
        Description: capability.Text("用于本地演示和底座验证的租户数据。", "Tenant data for local demos and platform validation."),
        Resource:    "tenants",
        Records: []capability.DemoRecord{
            {
                ID:     "tenant-demo-acme",
                Code:   "demo-acme",
                Name:   "Demo Acme Tenant",
                Status: "enabled",
                Values: map[string]string{"isolation": "sandbox"},
            },
        },
    },
}
```

The platform validates enabled demo data declarations during capability resolution:

- dataset ID is required, must use lowercase letters, numbers or hyphens, and is unique across enabled capabilities after trimming whitespace;
- localized title and description are required;
- target resource is required and must be provided by the enabled admin resource manifest set;
- at least one record is required;
- each record requires stable `id`, `code` and `name`;
- record IDs and record codes must be unique inside the same dataset.

Field-policy validation happens when the dataset is applied to the Store, not during capability declaration resolution. At apply time, every `Values` key must exist in the target resource schema and follow that field's explicit security policy. The Store does not infer sensitivity from a key name; demo writes use the same write and protection rules as other internal mutations.

This makes demo datasets plugin-safe: disabling the capability that owns a resource also disables demo data that would write to that resource, instead of failing later at apply time or silently creating orphaned fixtures.

## APIs

```text
GET /api/admin/demo-data
POST /api/admin/demo-data/:capability/:dataset/apply
```

`GET /api/admin/demo-data` lists demo datasets from enabled capabilities.

`POST /api/admin/demo-data/:capability/:dataset/apply` applies the declared records into the generic admin resource Store. Records are upserted by stable ID or code, so applying the same dataset is idempotent for the same record keys.

## Permissions

Demo data APIs use RBAC permission codes:

- `admin:demo-data:read`
- `admin:demo-data:apply`

The default `demo-data` capability registers the `admin:demo-data` permission prefix and a `/demo-data` admin menu entry. The admin frontend includes a dedicated demo data page that uses `listAdminDemoData()` and `applyAdminDemoData()`.

## Persistence

Demo data writes go through the generic admin resource Store. That means memory, file-backed and GORM-backed `AdminResourceRepository` modes all observe the same applied records.

## Boundary

Demo data is for demos and acceptance fixtures, not production migrations. Structural database changes belong in capability migrations. Required reference data belongs in capability seeds. Demo data should be safe to reapply and easy to remove from production deployments by disabling the capability or withholding its permissions.
