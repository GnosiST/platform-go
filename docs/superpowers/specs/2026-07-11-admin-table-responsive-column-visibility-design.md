# Admin Table Responsive Column Visibility Design

Date: 2026-07-11
Status: approved for implementation

## Goal

Remove two sources of capability and table-state ambiguity without changing platform runtime contracts:

1. Make it explicit that organization units are part of the default platform foundation and that only personnel records, positions, and assignments are optional.
2. Distinguish columns selected by the operator from columns currently rendered by the active responsive breakpoint.

## Current Evidence

- `identity` registers `org-units` in `core.DefaultManifests()` and the default profile requires the resource.
- `resources/platform-governance-topology.json` marks `org-units` as `requiredDefault` and keeps only the `personnel` capability optional.
- The optional capability catalog currently labels `personnel` as `人员组织` and describes optional enterprise organization management. That wording incorrectly conflates the default organization tree with the optional personnel extension.
- `PlatformDataTable` counts all operator-selected columns as visible even when Ant Design responsive breakpoints hide standard or extended columns.
- At 768px and 1024px, only essential columns render. At 1280px and 1440px, essential and standard columns render. Extended columns require the Ant Design `xxl` breakpoint.

## Considered Approaches

### A. Selected And Currently Rendered State (selected)

- Rename the aggregate state from `visible` to `selected`.
- Compute the currently rendered subset from the active Ant Design breakpoint state and each column's effective responsive rule.
- Show both counts in column settings.
- Mark selected columns that are hidden at the current width.
- Keep checkboxes enabled so a choice remains stable when the viewport grows.

This is the selected approach because it explains the actual state without changing the responsive priority contract.

### B. Copy-Only Count Rename

Only rename `已显示` to `已选择`. This removes the false claim but does not explain why a checked column is absent from the table.

### C. User-Configurable Responsive Priorities

Allow operators to override essential, standard, and extended tiers in the column menu. This is rejected because it adds persistent configuration, can recreate horizontal overflow, and weakens schema-order priority governance.

## Approved Behavior

### Capability Boundary Copy

- Rename core `identity` from `身份中心` to `身份与组织` and explicitly name the default organization tree in its description.
- Rename optional `personnel` from `人员组织` to `人员与岗位` and from `Personnel` to `Personnel & Positions`.
- State that the extension adds personnel profiles, positions, and assignments.
- State that organization units are already supplied by the default platform foundation.
- Rename the `platform-personnel-ready` profile from `人员组织增强` to `人员与岗位增强` and remove organization-management wording from its purpose.
- Do not change manifests, profile composition, generated resource contracts, routes, permissions, or persistence behavior.

### Column Settings

- Replace `visibleColumns(visible, total)` with separate localized labels for selected and currently rendered counts.
- Use Ant Design's breakpoint state as the single viewport source.
- Resolve each column's effective responsive list using the same helper used to build table columns.
- A selected column is currently rendered when it has no responsive restriction or at least one of its responsive breakpoints is active.
- Add a localized secondary annotation to columns hidden by the current width.
- Locked columns remain selected. Row selection and row actions remain outside this count and remain available at their existing breakpoints.

### Accessibility And Responsive Quality

- Hidden-at-current-width state is conveyed with text, not color alone.
- The aggregate summary remains readable in Chinese and English.
- Existing keyboard-operable checkboxes, focus behavior, 44px mobile targets, reduced-motion rules, and mobile cards remain unchanged.
- No new dependency, visual theme, icon family, animation, or business-field-name rule is introduced.

## Files

- `admin/src/platform/capabilities/metadata.ts`
- `resources/platform-capability-profiles.json`
- `admin/src/platform/ui/PlatformDataTable.tsx`
- `admin/src/platform/i18n.ts`
- `admin/src/platform/api-docs/APIDocsPage.tsx`
- `admin/src/platform/capabilities/CapabilityConsole.tsx`
- `admin/src/platform/resources/GenericResourceConsole.tsx`
- `admin/src/platform/policy-review/PolicyReviewConsole.tsx`
- `scripts/validate-admin-ui-contracts.mjs`
- `scripts/admin-ui-contracts.test.mjs`

## Verification

- Add a failing Admin UI contract test before implementation.
- Verify the validator rejects loss of breakpoint-aware rendered-column state and the corrected personnel boundary copy.
- Run Admin UI contract tests, i18n validation, Admin UI validation, and the Admin production build.
- Run capability profile and governance topology validation to prove no runtime classification changed.
- Browser-check column settings at 768px, 1024px, 1280px, and 1440px in Chinese and one English pass.
- Run `git diff --check`, refresh CodeGraph, review the final diff, commit, and confirm a clean worktree.

## Non-Goals

- No change to the default capability list or optional profile composition.
- No new organization, department, employee, or address resource.
- No user-defined column priority or per-user persistence.
- No redesign of the table, capability console, navigation, theme, or desktop information architecture.
