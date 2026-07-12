# Admin Watermark And Export Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved screen-count and policy-review JSON watermark contract, then close `admin-watermark-export-governance` with automated and browser evidence.

**Architecture:** Keep watermark preferences in the existing Admin UI configuration, normalized at the local-storage/import boundary. Render screen marks through one inert grid layer inside `AdminShell`; pass export intent explicitly from the Policy Review console to the API, where the server adds structured provenance metadata and records only the applied boolean in audit policy. Watermark settings never create a marked OpenAPI derivative, while the enabled policy-review schema remains aligned with the runtime contract and original file bytes remain unchanged.

**Tech Stack:** React 18, TypeScript, Ant Design, CSS grid, Gin, Go, Node contract tests, local in-app browser QA.

## Global Constraints

- Preserve legacy `watermark: boolean` configuration compatibility.
- Allowed counts are exactly `1 | 4 | 9 | 16`; default is `1`.
- Allowed scopes are exactly `"screen" | "export"`; default is `["screen"]`.
- Watermark controls remain bilingual, keyboard operable and at least 44px high on mobile.
- Screen watermark content is `aria-hidden="true"` and `pointer-events: none`.
- Policy-review JSON is the only export format changed in this node.
- Watermark settings never create or replace a marked OpenAPI derivative; generated schemas may evolve to describe the new policy-review query and response.
- Original file downloads remain byte-identical.
- Do not claim PDF, image, CSV, XLSX or arbitrary-file watermark support.
- Use TDD and commit each independently reviewable task.

---

### Task 1: UI Configuration Contract And Normalization

**Files:**
- Modify: `admin/src/platform/ui/SystemSettingsDrawer.tsx`
- Modify: `admin/src/App.tsx`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Produces: `type WatermarkCount = 1 | 4 | 9 | 16`
- Produces: `type WatermarkScope = "screen" | "export"`
- Extends: `AdminUIConfig` with `watermarkCount` and `watermarkScopes`
- Produces: `normalizeUIConfig(value: unknown): AdminUIConfig` with legacy compatibility

- [ ] **Step 1: Write failing contract tests**

Add mutation tests that reject removal of `watermarkCount`, `watermarkScopes`, allowed counts, allowed scopes and normalization of legacy `{ watermark: true }` settings.

- [ ] **Step 2: Verify RED**

Run:

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
```

Expected: new watermark configuration tests fail because count/scope support is absent.

- [ ] **Step 3: Implement minimal configuration contract**

Use:

```ts
export type WatermarkCount = 1 | 4 | 9 | 16;
export type WatermarkScope = "screen" | "export";

export type AdminUIConfig = {
  // existing fields
  watermark: boolean;
  watermarkCount: WatermarkCount;
  watermarkScopes: WatermarkScope[];
};
```

Normalize counts against `[1, 4, 9, 16]`, scopes against `screen/export`, and fall back to `watermarkCount: 1`, `watermarkScopes: ["screen"]` without rejecting valid unrelated preferences.

- [ ] **Step 4: Verify GREEN**

Run the Node contract test and `rtk npm --prefix admin run build`.

- [ ] **Step 5: Commit**

```bash
rtk git add admin/src/platform/ui/SystemSettingsDrawer.tsx admin/src/App.tsx scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "feat: add watermark configuration contract"
```

### Task 2: Accessible Settings And Exact Screen Rendering

**Files:**
- Modify: `admin/src/platform/ui/SystemSettingsDrawer.tsx`
- Modify: `admin/src/platform/shell/AdminShell.tsx`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Consumes: `AdminUIConfig.watermarkCount`, `AdminUIConfig.watermarkScopes`
- Produces: exactly `watermarkCount` elements in `.platform-watermark-layer`

- [ ] **Step 1: Write failing UI drift tests**

Require the scope checkbox group, segmented `1/4/9/16` control, `aria-hidden="true"`, exact array generation, CSS grid and mobile 44px targets.

- [ ] **Step 2: Verify RED**

Run the Admin UI contract tests; expect missing controls/layer failures.

- [ ] **Step 3: Implement progressive settings controls**

Keep the master switch. When enabled, show `Checkbox.Group` for screen/export. When screen is selected, show a labeled `Segmented` count selector. Add synchronized Chinese/English labels and format-limitation helper text.

- [ ] **Step 4: Replace the pseudo-element watermark**

Render:

```tsx
{uiConfig.watermark && uiConfig.watermarkScopes.includes("screen") ? (
  <div className="platform-watermark-layer" aria-hidden="true" data-count={uiConfig.watermarkCount}>
    {Array.from({ length: uiConfig.watermarkCount }, (_, index) => (
      <span key={index}>{watermarkText}</span>
    ))}
  </div>
) : null}
```

Use CSS grid templates for `1`, `4`, `9`, `16`; keep content above the layer, no pointer capture, no horizontal overflow and no viewport-scaled typography.

- [ ] **Step 5: Verify GREEN**

Run UI contract tests, i18n validation and Admin build.

- [ ] **Step 6: Commit**

```bash
rtk git add admin/src/platform/ui/SystemSettingsDrawer.tsx admin/src/platform/shell/AdminShell.tsx admin/src/platform/i18n.ts admin/src/styles.css scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "feat: render configurable admin watermarks"
```

### Task 3: Policy Review Export Provenance

**Files:**
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/adminresource/policy_review.go`
- Modify: `internal/platform/adminresource/policy_review_test.go`
- Modify: `admin/src/platform/api/client.ts`
- Modify: `admin/src/platform/policy-review/PolicyReviewConsole.tsx`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- HTTP query: `GET /api/admin/policy-reviews/export?watermark=true|false`
- Produces JSON metadata:

```json
{
  "watermark": {
    "applied": true,
    "product": "Platform Go",
    "exportedBy": "admin",
    "exportedAt": "2026-07-12T00:00:00Z"
  }
}
```

- [ ] **Step 1: Write failing Go tests**

Cover default `applied=false`, explicit `watermark=true`, stable product/actor/time, and audit value `watermarkApplied=true|false` without storing a free-form watermark string.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/adminresource ./internal/platform/httpapi -run 'Watermark|PolicyReviewExport' -count=1
```

- [ ] **Step 3: Implement backend metadata**

Pass `watermarkApplied bool` into `ExportPolicyReviews`, return structured metadata, and add only the boolean to the export audit record. Continue using projection rules for reviews/audits.

- [ ] **Step 4: Write and verify frontend RED**

Require `exportAdminPolicyReviews({ watermark: boolean })` and the Policy Review console to pass `uiConfig.watermark && uiConfig.watermarkScopes.includes("export")` through props supplied by `App`.

- [ ] **Step 5: Implement frontend export intent**

Download the returned JSON unchanged. Do not add client-generated provenance, watermark the canonical OpenAPI artifact or alter original file downloads. Keep the enabled policy-review Admin OpenAPI schema aligned with the runtime query and response.

- [ ] **Step 6: Verify GREEN and commit**

Run focused Go tests, UI contracts and Admin build, then commit:

```bash
rtk git commit -m "feat: add policy review export watermark metadata"
```

### Task 4: Governance Closeout And Browser Acceptance

**Files:**
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `README.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-ui-optimization-assessment.md`
- Modify: relevant governance tests under `scripts/`

**Interfaces:**
- Moves `admin-watermark-export-governance` from pending/future/blocker to implemented/required/closeout.
- Produces governance state `45 total / 39 implemented / 6 controlled unfinished`.

- [ ] **Step 1: Write failing governance mutation tests**

Reject stale `45/38/7`, a watermark node remaining future/pending, missing Product Design/UI UX/browser evidence and reordered six-node blocker projections.

- [ ] **Step 2: Update governance resources and docs**

Keep the remaining ordered nodes:

```text
sensitive-data-protection-runtime
sensitive-data-historical-migration
open-source-portability
public-docs-community
public-docs-site
github-release-publication
```

- [ ] **Step 3: Run browser acceptance**

Verify desktop and `390x844`: switch progressive disclosure, every scope combination, exact counts `1/4/9/16`, light/dark, keyboard focus, no horizontal overflow, policy-review export metadata and clean console. Save accepted screenshots and reference them in closeout evidence.

- [ ] **Step 4: Run final verification**

```bash
rtk go test ./... -count=1
rtk go vet ./...
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

- [ ] **Step 5: Commit**

```bash
rtk git commit -m "docs: close watermark export governance"
```

## Self-Review

- Spec coverage: legacy config, exact counts, screen/export scopes, policy-review JSON metadata, unwatermarked canonical OpenAPI, aligned policy-review schema, unchanged original artifacts, i18n, accessibility, browser evidence and governance migration are covered.
- Placeholder scan: no deferred implementation placeholders; unsupported formats are explicitly excluded by the approved scope.
- Type consistency: `WatermarkCount`, `WatermarkScope`, `AdminUIConfig`, export query and metadata names remain consistent across Tasks 1-4.
