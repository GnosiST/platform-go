# Platform Branding

Date: 2026-07-04
Last updated: 2026-07-19

## Purpose

Branding is a reusable platform capability for product name, logo, theme and login copy. Business modules should not hard-code product identity or read raw admin resource rows directly.

## API

```text
GET /api/platform/branding
```

The response is public platform configuration:

- `productName`
- `shortName`
- `logoUrl`
- `faviconUrl`
- `primaryColor`
- `defaultTheme`
- `loginTitle`
- `loginSubtitle`
- `supportEmail`

The admin frontend uses the same API to render the shell brand and initial theme.

## Admin Configuration

Branding is managed through the `/settings` system configuration workbench and the generic `settings` admin resource. The default record is:

```text
resource: settings
id: setting-branding
code: branding
```

`GET /api/admin/resources/settings/schema` exposes the branding field contract, including required `productName`, `capability` and `defaultTheme` fields.

Admin writes still go through the generic admin resource API:

```text
PUT /api/admin/resources/settings/setting-branding
```

## Boundary

Schemas and defaults are provided by the enabled platform capabilities. Runtime values come from the admin resource Store, which can be memory-backed, file-backed or GORM-backed through the `AdminResourceRepository` port.

Business code should call the branding API or a typed service wrapper around it. It should not depend on the `settings` resource row shape.

The topbar settings drawer is a frontend interface-preference drawer for theme, layout, density, watermark and local import/export. It must not become the product entry for branding, notification provider accounts, credential policy or other capability-owned system configuration.
