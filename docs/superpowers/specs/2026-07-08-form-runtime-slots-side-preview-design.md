# Form Runtime Slots And Side Preview Design

Date: 2026-07-08

## Goal

Promote `form-schema-layout-and-slots` from a contract-gated preview into a controlled runtime slot capability for reusable admin forms.

The platform should let downstream capabilities extend standard CRUD forms without forking `GenericResourceConsole`, but it must not become an arbitrary runtime component loader. The target is a safe middle layer: schema-declared slot descriptors plus a frontend slot registry owned by platform UI or capability packages.

## Current State

`PlatformResourceForm` already provides source-level React slots:

- `header`
- `footer`
- `sectionBefore`
- `sectionAfter`
- `fieldControl`

Before this promotion, `resources/platform-form-schema-layout-slots.json` kept runtime slots deferred and forbade raw scripts, backend manifest React component names and arbitrary component paths. Existing form layouts already covered `single-column`, `grouped-sections` and `two-column-density`.

## Product Design Brief

The admin form experience should stay compact, quiet and utilitarian. Runtime slots are for useful operational context, not decoration. The visual direction is:

- keep the existing Ant Design based form rhythm and platform theme tokens;
- make inserted content feel native to the form, using the same spacing, typography and border rules;
- keep forms readable on dense desktop modals and degrade to one column on mobile;
- reserve `side-detail-preview` for workflows where the user benefits from seeing current values, audit context, file metadata, permission summaries or generated previews while editing.

## Chosen Approach

Use a `Slot Registry + Declarative Slot Descriptor` model.

Backend and resource contracts may declare only data, intent and placement:

```text
slotId
region
label
description
permission
visibleWhen
dataBinding
variant
order
```

Frontend code resolves `slotId` through a local registry. The registry maps known slot IDs to React renderers that are imported by source code, reviewed, typed and covered by UI contracts.

This keeps extension flexible while preserving platform safety:

- no backend-provided component paths;
- no dynamic imports from manifests;
- no raw script slots;
- no business imports inside platform core;
- no standard CRUD page forks for slot-only requirements.

## Slot Regions

Runtime slots are limited to these regions:

| Region | Purpose | Owner |
| --- | --- | --- |
| `form.header` | Above all sections, for record summary, warnings or workflow state | platform UI or capability package |
| `form.section.before` | Before a named form section, for localized help or prerequisite context | platform UI or capability package |
| `form.section.after` | After a named form section, for calculated summaries or linked data hints | platform UI or capability package |
| `form.footer` | Above form actions, for approval notes, policy hints or secondary actions | platform UI or capability package |
| `field.control` | Replace or wrap a field control when a capability owns a specialized editor | capability package only |
| `side.preview` | Right-side preview/inspector used by `side-detail-preview` layout | platform UI or capability package |

`field.control` remains the most restricted region. It must preserve Ant Design Form value props, validation, disabled state and accessibility labels.

## Descriptor Contract

Add a reusable runtime slot descriptor shape to admin resource schemas. The descriptor is not a React component declaration.

```json
{
  "slotId": "platform.record-summary",
  "region": "form.header",
  "label": { "zh": "记录摘要", "en": "Record Summary" },
  "description": { "zh": "展示当前记录关键字段。", "en": "Shows key fields for the current record." },
  "permission": "admin:user:read",
  "targetSection": "base",
  "targetField": "",
  "variant": "compact",
  "order": 10,
  "dataBinding": {
    "fields": ["code", "name", "status"],
    "mode": "record"
  }
}
```

Required safety rules:

- `slotId` must be a stable allowlisted ID.
- `region` must be one of the supported regions.
- `label` must provide `zh` and `en`.
- `permission`, when present, must match the resource permission namespace or an already declared route/action permission.
- `targetSection` is required for section slots.
- `targetField` is required for `field.control`.
- `dataBinding.fields` can reference only fields declared by the same schema.
- renderer props expose only resource metadata, current form values, current record snapshot, selected language and permission helpers.

## Frontend Registry

Add `admin/src/platform/ui/formSlotRegistry.tsx` with:

- `AdminFormRuntimeSlotDescriptor`
- `AdminFormRuntimeSlotRendererProps`
- `AdminFormRuntimeSlotRegistry`
- `createAdminFormSlotRegistry`
- a default platform registry with safe platform slots

`GenericResourceConsole` receives schema slot descriptors, filters them by permission and passes renderer output into `PlatformResourceForm.slots`.

Downstream capability packages can compose a registry at the app boundary, but platform UI must not import downstream business packages.

## Side Detail Preview

`side-detail-preview` is a form layout preset, not a custom page.

Desktop layout:

- left: editable form sections;
- right: fixed-width preview rail using `side.preview` slots;
- modal/drawer body scrolls internally;
- footer stays visible.

Mobile layout:

- form stays first;
- preview rail collapses below the editable sections;
- no horizontal overflow;
- slot cards keep compact spacing and readable headings.

Use cases:

- current record summary beside edit fields;
- before/after preview for localized fields;
- file metadata and download preview;
- permission/action summary for role resources;
- generated scaffold preview for codegen resources.

## Error Handling

Unknown slot IDs render no UI and record a development warning in the console. They must not break CRUD forms.

Invalid descriptor shapes are rejected by validators before runtime. Runtime filtering also skips slots whose permissions are denied.

Renderer errors must be isolated to the slot region. The form remains usable and shows a localized unavailable state for the failed slot.

## Testing And Gates

Promotion requires:

- `scripts/validate-platform-form-schema-layout-slots.mjs`
- `scripts/validate-admin-ui-contracts.mjs`
- `scripts/validate-admin-i18n.mjs`
- `rtk npm --prefix admin run build`
- browser screenshots for desktop `side-detail-preview`
- browser screenshots for mobile collapsed preview
- a contract test proving unknown slot IDs, missing i18n, invalid regions and unsafe component names are rejected

## Non-Goals

- Do not enable arbitrary component names from backend manifests.
- Do not add dynamic import paths to resource schemas.
- Do not introduce source-writing generation.
- Do not fork standard CRUD pages for slot rendering.
- Do not implement business-specific slot renderers inside platform core.

## Acceptance

This design is accepted when:

- runtime slot descriptors are machine-checkable;
- `PlatformResourceForm` can render registered runtime slots;
- `side-detail-preview` has desktop and mobile evidence;
- standard CRUD remains functional without slots;
- default platform resources do not import business packages;
- validators keep unsafe slot patterns blocked.
