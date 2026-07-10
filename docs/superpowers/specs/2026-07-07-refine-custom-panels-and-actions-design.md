# Refine Custom Panels And Actions Design

Date: 2026-07-07

## Decision

Use the selected combined direction: keep the generic resource list as the primary work surface, open details and extension content in a right drawer, add a compact selection command bar for batch work, and expose custom resource actions plus plugin panels through resource schema metadata.

The visual target is anchored by the generated resource-list drawer direction at `/Users/irainbow/.codex/generated_images/019f2ad3-1ed2-7810-849b-1fa073078dcf/ig_078f9851005a1fc5016a4c660ed5588191acdb7faaf7e52994.png`, with batch/filter behavior borrowed from the dark command-bar option and approval/file/audit tabs borrowed from the warm plugin-panel option.

## Goals

- Keep Refine, React and Ant Design as the admin implementation path.
- Preserve the schema-driven generic resource console as the default CRUD surface.
- Let capabilities declare row actions, batch actions and detail drawer panels without hard-coding business pages.
- Make actions permission-aware, i18n-ready and audit-addressable.
- Provide extension slots for future zshenmez business approval panels, file previews, audit timelines and policy-review flows.
- Keep tenant, org-unit, area-code and role-group governance visible through existing field relations and data scopes.

## Non-Goals

- Do not move business state machines into `internal/platform`.
- Do not enable zshenmez business-store writes.
- Do not implement source-writing code generation.
- Do not create a separate business-specific admin page for standard CRUD resources.
- Do not introduce role-group permission inheritance or implicit area-code authorization.

## Contract Model

Extend the admin resource schema with optional action and panel metadata.

Resource actions describe UI placement and API intent:

- `key`: stable action key, unique per resource;
- `label`: localized text;
- `kind`: `row`, `batch` or `resource`;
- `tone`: `default`, `primary`, `danger` or `warning`;
- `icon`: semantic icon key consumed by the frontend icon map;
- `permission`: required permission code;
- `route`: optional platform/custom route for server-side actions;
- `method`: HTTP method for route actions;
- `confirm`: optional localized confirmation metadata;
- `auditAction`: optional audit action namespace;
- `refresh`: whether successful execution refreshes the list and detail record.

Resource panels describe drawer tabs:

- `key`: stable panel key, unique per resource;
- `label`: localized text;
- `kind`: `fields`, `permissions`, `audit`, `approval`, `files` or `custom`;
- `permission`: optional permission required to see the panel;
- `component`: semantic component key, not a React import path;
- `order`: tab ordering;
- `empty`: optional localized empty state.

The schema stays metadata-only. Runtime React component paths, raw scripts and business package imports are forbidden.

## Frontend Behavior

`GenericResourceConsole` continues to use Refine `useList`, `useCreate`, `useUpdate` and `useDelete`.

The list toolbar remains compact. Standard actions are:

- search;
- advanced filters;
- column settings;
- refresh;
- create when allowed.

Row actions merge default platform actions with declared row actions. View and edit remain icon actions. Delete remains destructive and confirmed. Extra actions go into an overflow menu when there are more than two custom actions.

Batch actions appear only after selection. The command bar shows selected count, declared batch actions, default batch delete when allowed and clear selection.

The drawer becomes a tabbed inspector. Default tabs are detail fields and permission codes. Declared panels add tabs for approval, files, audit or custom capability content. Unknown custom panels render a safe unavailable state instead of failing the page.

## Backend Behavior

Schema endpoints include the new metadata. Existing CRUD routes continue to work.

Custom actions with `route` must be executed through capability-owned admin route registration, not generic store magic. Platform code only provides metadata, permission checks and neutral route seams. Business handlers remain under their capability package.

Validators must reject:

- duplicate action or panel keys on one resource;
- missing localized labels;
- unsupported action kind, tone, method or panel kind;
- actions without permission codes;
- dangerous actions without confirmation metadata;
- business-specific component paths or raw scripts;
- panel component keys that look like file paths;
- declared action routes outside `/api/admin/`.

## Error Handling

Failed action execution displays compact `AdminFeedback` inside the resource console. Confirmed destructive actions must never execute without a user confirmation. Unauthorized actions are hidden in the UI and still denied by backend permission checks.

Custom panel rendering must degrade safely. Missing data or unsupported panel kind shows localized empty or unavailable copy.

## Testing And Verification

Required checks:

- Go tests for schema cloning and manifest validation.
- Node validator tests for admin resource action/panel metadata.
- UI contract validator updates for platform action components, drawer tabs, i18n and slot boundaries.
- Admin i18n validator.
- Admin build.
- Browser visual check for desktop and narrow viewport after implementation.

## Follow-On Nodes

This node unlocks richer custom UI for:

- `policy-review-custom-ui`;
- `file-storage-preview-and-audit-workflow`;
- Downstream business custom admin panels.

It does not itself complete those nodes.
