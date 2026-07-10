# Form Runtime Slots And Side Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote the form layout preview gate into a controlled, business-neutral runtime slot capability with `side-detail-preview` support.

**Architecture:** Resource schemas declare safe slot descriptors only; React renderers stay in a typed frontend registry composed at the platform or app boundary. `GenericResourceConsole` filters descriptors by permission and renders registry output through `PlatformResourceForm`; no backend component paths, dynamic imports, raw scripts or business package imports are allowed.

**Tech Stack:** Gin, GORM, Casbin/JWT-aligned platform contracts, Refine, React, Ant Design.

---

### Task 1: Contract Tests For Runtime Slot Promotion

**Files:**
- Modify: `scripts/platform-form-schema-layout-slots.test.mjs`
- Modify: `scripts/validate-platform-form-schema-layout-slots.mjs`
- Modify: `resources/platform-form-schema-layout-slots.json`

- [x] Add a failing test that removes the `runtimeSlotDescriptor` contract from `resources/platform-form-schema-layout-slots.json` and expects `runtimeSlotDescriptor must declare slotId, region, label, description, permission, visibleWhen, dataBinding, variant, order`.
- [x] Add a failing test that removes `side.preview` from `allowedSlotRegions` and expects `allowedSlotRegions must include side.preview`.
- [x] Run `rtk node --test scripts/platform-form-schema-layout-slots.test.mjs` and confirm the new tests fail for missing validator coverage.
- [x] Update `resources/platform-form-schema-layout-slots.json` with a machine-checkable descriptor contract:

```json
"runtimeSlotDescriptor": {
  "status": "implemented",
  "fields": ["slotId", "region", "label", "description", "permission", "visibleWhen", "targetSection", "targetField", "dataBinding", "variant", "order"],
  "requiredLocalizedFields": ["label", "description"],
  "allowedRegions": ["form.header", "form.section.before", "form.section.after", "form.footer", "field.control", "side.preview"],
  "allowedDataBindingModes": ["record", "formValues", "resource", "none"],
  "allowedVariants": ["compact", "info", "warning", "preview", "inline"],
  "forbiddenFields": ["component", "componentName", "componentPath", "import", "script", "html"]
}
```

- [x] Update `scripts/validate-platform-form-schema-layout-slots.mjs` so it validates the descriptor contract, `side.preview`, forbidden descriptor fields and source evidence.
- [x] Run `rtk node --test scripts/platform-form-schema-layout-slots.test.mjs` and confirm it passes.

### Task 2: Backend And API Schema Types

**Files:**
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: `admin/src/platform/api/client.ts`
- Test: `internal/platform/capability/admin_contract_test.go`
- Test: `internal/platform/adminresource/store_test.go`

- [x] Add `AdminRuntimeSlot` / `RuntimeSlotDefinition` structs with JSON fields matching the descriptor contract.
- [x] Add `RuntimeSlots []RuntimeSlotDefinition` to `adminresource.Schema` and `RuntimeSlots []AdminRuntimeSlot` to `capability.AdminResource`.
- [x] Clone runtime slots in `cloneSchema` including `DataBinding.Fields`.
- [x] Map capability-declared runtime slots through `schemaFromCapability`.
- [x] Add TypeScript types `AdminResourceRuntimeSlot`, `AdminResourceRuntimeSlotDataBinding`, `AdminResourceRuntimeSlotRegion`, `AdminResourceRuntimeSlotVariant` and `runtimeSlots?: AdminResourceRuntimeSlot[]` to `AdminResourceSchema`.
- [x] Add tests proving runtime slots are copied from capability manifests to schemas and clone without aliasing nested field arrays.
- [x] Run `rtk go test ./internal/platform/capability ./internal/platform/adminresource`.

### Task 3: Frontend Slot Registry And Form Rendering

**Files:**
- Create: `admin/src/platform/ui/formSlotRegistry.tsx`
- Modify: `admin/src/platform/ui/PlatformResourceForm.tsx`
- Modify: `admin/src/platform/ui/index.ts`
- Modify: `admin/src/platform/resources/GenericResourceConsole.tsx`

- [x] Create a typed `AdminFormRuntimeSlotRegistry` that resolves allowlisted `slotId` values to renderers.
- [x] Ship platform defaults for `platform.record-summary`, `platform.permission-summary` and `platform.localized-preview`; each renderer uses existing Ant Design, platform typography and localized schema labels.
- [x] Extend `PlatformResourceFormSlots` with `sidePreview?: ReactNode` and update `PlatformResourceForm` so `side-detail-preview` renders editable sections and preview slots as sibling regions.
- [x] Update `GenericResourceConsole` to build runtime slots from `schema.runtimeSlots`, filter them by permission, and pass rendered header/footer/section/field/side preview slots into `PlatformResourceForm`.
- [x] Keep unknown slot IDs non-fatal by rendering a localized unavailable state in development-safe UI while leaving the form usable.
- [x] Run `rtk npm --prefix admin run build` after this task.

### Task 4: UI Contract, I18n And Styles

**Files:**
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`

- [x] Add Chinese and English dictionary keys for slot unavailable, record summary, permission summary and localized preview labels.
- [x] Add CSS for `.resource-form-side-preview`, `.resource-form-main`, `.resource-form-preview-rail` and compact slot cards using existing tokens.
- [x] Update `validate-admin-ui-contracts.mjs` to require `formSlotRegistry`, `sidePreview?: ReactNode`, `layout-side-detail-preview`, `.resource-form-preview-rail` and runtime slot rendering through `GenericResourceConsole`.
- [x] Run `rtk node scripts/validate-admin-i18n.mjs` and `rtk node scripts/validate-admin-ui-contracts.mjs`.

### Task 5: Documentation, Task Graph And Verification

**Files:**
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/admin-ui-foundation.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-engineering-capabilities.json`

- [ ] Document runtime slots as descriptor-only schema metadata and restate that business renderers are composed outside platform core.
- [ ] Update task graph status from `preview` to `implemented` only after validators, build and browser evidence exist.
- [ ] Run:

```bash
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node --test scripts/platform-form-schema-layout-slots.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk npm --prefix admin run build
rtk go test ./...
rtk node --test scripts/*.test.mjs
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

- [x] Capture browser evidence for desktop `side-detail-preview` and mobile collapsed preview before marking the node implemented.

## Self-Review

- Spec coverage: tasks map to descriptor contract, backend schema, frontend registry, side preview rendering, i18n, UI contracts, docs and verification.
- Scope check: this plan does not promote source-writing codegen, production auth hardening or any `zshenmez` business resource.
- Safety check: backend manifests never name React components; frontend registry is the only renderer resolver.
