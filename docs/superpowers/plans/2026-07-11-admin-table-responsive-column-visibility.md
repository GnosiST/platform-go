# Admin Table Responsive Column Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clarify that organization units belong to the default platform foundation and make table column settings distinguish operator selection from breakpoint-driven rendering.

**Architecture:** Keep runtime capability manifests and responsive priority tiers unchanged. Correct only the user-facing capability/profile language, then derive current rendered-column state from Ant Design's existing breakpoint observer and the same effective responsive rule already passed to `Table`.

**Tech Stack:** React, TypeScript, Ant Design Grid/Table, platform i18n dictionaries, Node.js contract validators and tests.

## Global Constraints

- Prefix every shell command with `rtk`.
- Do not change capability manifests, profile composition, generated resource contracts, routes, permissions, persistence, or API behavior.
- Preserve schema-order priority: first four essential, next three standard, remaining extended.
- Preserve caller-provided `column.responsive` rules.
- Keep row selection, row actions, mobile cards, keyboard behavior, reduced-motion behavior, and 44px mobile targets unchanged.
- Add Chinese and English copy in the same change.
- Add no dependency and no business-field-name logic.

---

### Task 1: Correct Organization And Personnel Capability Language

**Files:**
- Modify: `admin/src/platform/capabilities/metadata.ts:21-49,158-168`
- Modify: `resources/platform-capability-profiles.json:148-153`
- Modify: `scripts/validate-admin-ui-contracts.mjs:12-30,120-150`
- Modify: `scripts/admin-ui-contracts.test.mjs`
- Modify: `scripts/platform-capability-profiles.test.mjs:140-170`

**Interfaces:**
- Consumes: current `capabilityMetadata` and `optionalCapabilities` view-model contracts.
- Produces: core label `身份与组织 / Identity & Organization`, optional label `人员与岗位 / Personnel & Positions`, and a profile label that cannot be mistaken for the default organization tree.

- [ ] **Step 1: Add failing UI contract coverage for capability boundary copy**

Add `capabilityMetadata` to the validator source map and require the corrected language:

```js
capabilityMetadata: readSource("admin/src/platform/capabilities/metadata.ts"),
```

```js
requireIncludes(files.capabilityMetadata, 'label: { zh: "身份与组织", en: "Identity & Organization" }', "Core identity capability must make default organization ownership explicit.");
requireIncludes(files.capabilityMetadata, 'makeOptional("personnel", { zh: "人员与岗位", en: "Personnel & Positions" }', "Optional personnel capability must not be labeled as the organization capability.");
requireIncludes(files.capabilityMetadata, "默认平台底座已提供组织机构", "Optional personnel copy must state that organization units are part of the default foundation.");
```

Add a negative fixture test that replaces `人员与岗位` with `人员组织` and expects the second validator message.

- [ ] **Step 2: Add failing profile-copy assertions**

In the existing personnel-ready profile test, add:

```js
assert.equal(personnelProfile.label.zh, "人员与岗位增强");
assert.equal(personnelProfile.label.en, "Personnel & Positions Platform Foundation");
assert.match(personnelProfile.purpose, /organization units remain part of the default platform foundation/i);
```

- [ ] **Step 3: Run focused tests and confirm RED**

Run:

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs scripts/platform-capability-profiles.test.mjs
```

Expected: failures report the old `身份中心`, `人员组织`, and profile wording.

- [ ] **Step 4: Implement the minimal copy correction**

Update the metadata entries to:

```ts
identity: {
  label: { zh: "身份与组织", en: "Identity & Organization" },
  description: {
    zh: "用户、组织机构树、账号身份与统一认证能力。",
    en: "Users, organization-unit hierarchy, account identities, and unified authentication.",
  },
  // Existing domain, group, kind, health, owner, dependencies and APIs stay unchanged.
},
```

```ts
makeOptional(
  "personnel",
  { zh: "人员与岗位", en: "Personnel & Positions" },
  {
    zh: "扩展人员档案、岗位和任职关系；默认平台底座已提供组织机构。",
    en: "Adds personnel profiles, positions, and assignments; organization units are already part of the default platform foundation.",
  },
),
```

Update the profile source to:

```json
"label": {
  "zh": "人员与岗位增强",
  "en": "Personnel & Positions Platform Foundation"
},
"purpose": "Default platform foundation plus optional personnel profiles, positions and position assignments; organization units remain part of the default platform foundation."
```

- [ ] **Step 5: Run focused tests and confirm GREEN**

Run the Step 3 command again.

Expected: both test files pass.

- [ ] **Step 6: Commit Task 1**

```bash
rtk git add admin/src/platform/capabilities/metadata.ts resources/platform-capability-profiles.json scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs scripts/platform-capability-profiles.test.mjs
rtk git commit -m "fix: clarify organization and personnel capability boundaries"
```

---

### Task 2: Distinguish Selected And Currently Rendered Columns

**Files:**
- Modify: `admin/src/platform/ui/PlatformDataTable.tsx:9-26,54-80,159-194,280-307,480-491`
- Modify: `admin/src/platform/i18n.ts:332-346,779-793`
- Modify: `admin/src/platform/api-docs/APIDocsPage.tsx:225-242`
- Modify: `admin/src/platform/capabilities/CapabilityConsole.tsx:293-309`
- Modify: `admin/src/platform/resources/GenericResourceConsole.tsx:660-676`
- Modify: `admin/src/platform/policy-review/PolicyReviewConsole.tsx:421-437`
- Modify: `admin/src/styles.css:1672-1702`
- Modify: `scripts/validate-admin-ui-contracts.mjs:125-145`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Replaces: `visibleColumns(visible: number, total: number) => string`.
- Produces: `selectedColumns(selected: number, total: number) => string`, `renderedColumns(rendered: number, selected: number) => string`, and `hiddenAtCurrentWidth: string`.
- Produces: `effectiveResponsiveBreakpoints(column)` and `columnRenderedAtCurrentWidth(column, screens)` using the same priority mapping as Ant Design `Table`.

- [ ] **Step 1: Add failing table contract checks**

Add validator requirements:

```js
requireIncludes(files.table, "Grid.useBreakpoint()", "PlatformDataTable must use AntD breakpoint state for rendered-column clarity.");
requireIncludes(files.table, "effectiveResponsiveBreakpoints", "PlatformDataTable must share one effective responsive rule for rendering and column settings.");
requireIncludes(files.table, "columnRenderedAtCurrentWidth", "PlatformDataTable must compute columns rendered at the current width.");
requireIncludes(files.table, "labels.selectedColumns", "Column settings must distinguish selected columns.");
requireIncludes(files.table, "labels.renderedColumns", "Column settings must report currently rendered columns.");
requireIncludes(files.table, "labels.hiddenAtCurrentWidth", "Column settings must explain breakpoint-hidden selected columns.");
```

Add one negative fixture test that replaces `Grid.useBreakpoint()` with `{}` and expects the first message. Add a second fixture that removes `labels.hiddenAtCurrentWidth` and expects the last message.

- [ ] **Step 2: Run the Admin UI contract test and confirm RED**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
```

Expected: the new breakpoint-aware contract tests fail.

- [ ] **Step 3: Add localized label contracts and update all callers**

Replace the dictionary keys with:

```ts
selectedColumns: "已选择 {selected}/{total}",
renderedColumns: "当前显示 {rendered}/{selected}",
hiddenAtCurrentWidth: "当前宽度隐藏",
```

```ts
selectedColumns: "{selected}/{total} selected",
renderedColumns: "{rendered}/{selected} currently shown",
hiddenAtCurrentWidth: "Hidden at this width",
```

Each of the four `PlatformDataTable` callers must map both count templates with `formatTemplate` and pass `dictionary.hiddenAtCurrentWidth` directly.

- [ ] **Step 4: Implement breakpoint-aware rendered-column state**

Import `Grid` from Ant Design. Replace the label interface and add:

```ts
type PlatformBreakpoint = NonNullable<PlatformDataTableColumn<Record<string, unknown>>["responsive"]>[number];
type PlatformBreakpointState = Partial<Record<PlatformBreakpoint, boolean>>;
```

Inside `PlatformDataTable`:

```ts
const screens = Grid.useBreakpoint();
const selectedColumns = useMemo(
  () => columns.filter((column) => column.lockVisible || visibleColumnKeys.includes(column.key)),
  [columns, visibleColumnKeys],
);
const renderedColumnKeys = useMemo(
  () => new Set(selectedColumns.filter((column) => columnRenderedAtCurrentWidth(column, screens)).map((column) => column.key)),
  [screens, selectedColumns],
);
```

Use `selectedColumns` to create the existing display columns. The settings header renders two secondary lines:

```tsx
<Space align="end" direction="vertical" size={0}>
  <Typography.Text type="secondary">{labels.selectedColumns(selectedColumns.length, columns.length)}</Typography.Text>
  <Typography.Text type="secondary">{labels.renderedColumns(renderedColumnKeys.size, selectedColumns.length)}</Typography.Text>
</Space>
```

For each checkbox, show the state only when the column is selected but not rendered:

```tsx
const hiddenAtCurrentWidth = (column.lockVisible || visibleColumnKeys.includes(column.key)) && !renderedColumnKeys.has(column.key);
```

```tsx
<span className="platform-column-option">
  <span className="platform-column-option-label">{column.title as ReactNode}</span>
  {hiddenAtCurrentWidth ? (
    <Typography.Text className="platform-column-option-state" type="secondary">
      {labels.hiddenAtCurrentWidth}
    </Typography.Text>
  ) : null}
</span>
```

Use one effective responsive helper for both table columns and settings state:

```ts
function effectiveResponsiveBreakpoints<T extends object>(column: PlatformDataTableColumn<T>) {
  return [...(column.responsive ?? responsiveBreakpointsForPriority(column.priority))];
}

function withResponsivePriority<T extends object>(column: PlatformDataTableColumn<T>): PlatformDataTableColumn<T> {
  return { ...column, responsive: effectiveResponsiveBreakpoints(column) };
}

function columnRenderedAtCurrentWidth<T extends object>(column: PlatformDataTableColumn<T>, screens: PlatformBreakpointState) {
  return effectiveResponsiveBreakpoints(column).some((breakpoint) => screens[breakpoint]);
}
```

- [ ] **Step 5: Add stable option-row styles**

Add styles that preserve width and prevent the status from colliding with long labels:

```css
.platform-column-menu .ant-checkbox-wrapper > span:last-child {
  flex: 1;
  min-width: 0;
}

.platform-column-option {
  display: flex;
  min-width: 0;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.platform-column-option-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.platform-column-option-state {
  flex: 0 0 auto;
  font-size: 12px;
  white-space: nowrap;
}
```

- [ ] **Step 6: Run focused validation and confirm GREEN**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk npm --prefix admin run build
```

Expected: all commands pass.

- [ ] **Step 7: Commit Task 2**

```bash
rtk git add admin/src/platform/ui/PlatformDataTable.tsx admin/src/platform/i18n.ts admin/src/platform/api-docs/APIDocsPage.tsx admin/src/platform/capabilities/CapabilityConsole.tsx admin/src/platform/resources/GenericResourceConsole.tsx admin/src/platform/policy-review/PolicyReviewConsole.tsx admin/src/styles.css scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "fix: clarify responsive table column visibility"
```

---

### Task 3: Verify Runtime Boundaries, Responsive UX, And Clean Closeout

**Files:**
- Review only: all files changed in Tasks 1 and 2.
- Evidence: use the running local Admin and API services; do not commit temporary browser captures unless an existing repository governance record requires them.

**Interfaces:**
- Consumes: committed Task 1 and Task 2 behavior.
- Produces: evidence that runtime classification did not change and the responsive settings communicate the real table state.

- [ ] **Step 1: Run capability and governance validation**

```bash
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node --test scripts/platform-capability-profiles.test.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node --test scripts/platform-governance-topology.test.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node --test scripts/platform-personnel-runtime-readiness.test.mjs
```

Expected: all commands pass and default foundation still includes `org-units` while excluding `personnel`.

- [ ] **Step 2: Run UI/UX Pro Max pre-delivery query**

```bash
rtk python3 /Users/irainbow/.codex/skills/ui-ux-pro-max/scripts/search.py "responsive data table column settings accessibility loading" --domain ux
```

Use the result only as a checklist; preserve the existing Ant Design system and platform wrappers.

- [ ] **Step 3: Browser-check responsive column settings**

At 768x1024 and 1024x768:

- selected count equals the operator-selected columns;
- currently shown count includes essential columns only;
- selected standard and extended columns show `当前宽度隐藏`;
- row actions and selection remain operable;
- no page-level horizontal overflow.

At 1280x720 and 1440x1024:

- currently shown count includes essential and standard columns;
- only selected extended columns show the hidden annotation;
- all settings text fits without overlap.

Switch once to English and confirm the three new strings render correctly.

- [ ] **Step 4: Run the final relevant suite**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs scripts/platform-capability-profiles.test.mjs scripts/platform-governance-topology.test.mjs scripts/platform-personnel-runtime-readiness.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

Expected: all checks pass and CodeGraph reports an up-to-date index.

- [ ] **Step 5: Review commits and worktree**

```bash
rtk git log -4 --oneline
rtk git status --short --branch
```

Expected: design, capability-copy, and responsive-column commits are present; the worktree is clean.
