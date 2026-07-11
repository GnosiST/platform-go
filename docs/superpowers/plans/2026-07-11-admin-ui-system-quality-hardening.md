# Admin UI System Quality Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver an accessible, compact responsive Admin shell, reliable stale-session recovery, breakpoint-aware generic tables, reduced-motion support, and complete governance/browser evidence.

**Architecture:** Preserve the existing Refine, React, Ant Design, platform-wrapper, capability-manifest, and resource-schema boundaries. Implement responsive behavior in shared frontend primitives, use existing schema field order as the table-priority contract, normalize 401 behavior in the shared API client, and close the work through the repository's existing governance validators.

**Tech Stack:** React 18, TypeScript 5, Refine 5/6, Ant Design 5, Vite 8, Node test runner, repository JSON governance contracts, in-app browser QA.

## Global Constraints

- Prefix every shell command with `rtk`.
- Do not add a frontend dependency.
- Do not change backend admin resource contracts, capability manifests, API routes, or persistence behavior.
- Keep `>=1024px` desktop shell placement unchanged.
- Use the approved compact two-tier shell at `<=1023px`.
- Keep existing mobile resource cards authoritative at `<=767px`.
- Use existing schema field order, not business field names, for responsive table priority.
- Add Chinese and English i18n keys in the same change.
- Preserve Ant Design focus trapping, Escape handling, and trigger restoration.
- Every mobile interactive target introduced or modified by this plan must be at least 44x44px.
- Respect `prefers-reduced-motion` without suppressing loading, validation, focus, or status feedback.
- The final task graph must report 36 implemented nodes, zero preview/planned/deferred nodes for this goal, and a closed neat-freak record for `admin-ui-system-quality-hardening`.

---

### Task 1: Compact Shell, Skip Link, Route Focus, And Mobile Hit Areas

**Files:**
- Modify: `admin/src/platform/shell/AdminShell.tsx`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Consumes: `AdminShellProps.activeRoute`, `Dictionary`, `PlatformDropdownPlugin`, existing work-tab and context state.
- Produces: stable `#platform-main-content`, localized skip/mobile control labels, `.platform-mobile-contextbar`, and default focus-visible styles.

- [ ] **Step 1: Add failing UI contract checks**

Extend `files` and assertions in `scripts/validate-admin-ui-contracts.mjs` so the validator requires these source contracts:

```js
requireIncludes(files.shell, 'href="#platform-main-content"', "AdminShell must expose a skip-to-content link.");
requireIncludes(files.shell, 'id="platform-main-content"', "AdminShell main region must expose a stable focus target.");
requireIncludes(files.shell, "previousRouteRef", "AdminShell must move focus only after actual route changes.");
requireIncludes(files.shell, "dictionary.openMobileNavigation", "Mobile navigation must use an explicit localized accessible name.");
requireIncludes(files.shell, "dictionary.alerts", "The alert icon control must use an explicit localized accessible name.");
requireIncludes(files.shell, "platform-mobile-contextbar", "AdminShell must provide the approved compact mobile context bar.");
requireIncludes(files.styles, ".platform-skip-link", "styles.css must expose skip-link focus behavior.");
requireRegex(files.styles, /:focus-visible[\s\S]*outline:\s*2px solid var\(--primary\)/, "Visible focus must be a default platform behavior.");
requireRegex(files.styles, /@media\s*\(max-width:\s*1023px\)[\s\S]*min-height:\s*44px/, "Responsive shell controls must use 44px minimum targets.");
```

Add a negative fixture test that replaces `href="#platform-main-content"` with `href="#missing-content"` and expects the skip-link failure.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
```

Expected: FAIL because the shell, styles, and i18n implementation do not yet contain the new contracts.

- [ ] **Step 3: Add localized labels**

Add matching Chinese and English dictionary entries:

```ts
skipToContent: "跳到主要内容", // en: "Skip to main content"
openMobileNavigation: "打开主导航", // en: "Open primary navigation"
mobileWorkContext: "当前域与工作页", // en: "Current domain and work tab"
mobileRuntimeContext: "环境与租户上下文", // en: "Environment and tenant context"
sessionExpired: "会话已过期，请重新登录。", // en: "Your session expired. Sign in again."
```

- [ ] **Step 4: Implement the shell focus contract**

In `AdminShell.tsx`, add refs and route-change focus:

```tsx
const mainRef = useRef<HTMLElement | null>(null);
const previousRouteRef = useRef(activeRoute);

useEffect(() => {
  if (previousRouteRef.current === activeRoute) return;
  previousRouteRef.current = activeRoute;
  window.requestAnimationFrame(() => mainRef.current?.focus({ preventScroll: true }));
}, [activeRoute]);
```

Render the skip link before the shell navigation and make the existing main region the target:

```tsx
<a className="platform-skip-link" href="#platform-main-content">
  {dictionary.skipToContent}
</a>
<main ref={mainRef} className="platform-main" id="platform-main-content" tabIndex={-1}>
```

Add `aria-label={dictionary.openMobileNavigation}` to the mobile menu button and `aria-label={dictionary.alerts}` to the alert button.

- [ ] **Step 5: Implement the approved two-tier mobile shell**

Keep desktop markup in place. Add a mobile-only context bar that reuses the current active resource, open tabs, environment value, tenant value, and `PlatformDropdownPlugin` panels. The first compact bar remains `.platform-topbar`; the second uses `.platform-mobile-contextbar` and contains exactly two buttons with the localized labels from Step 3.

Move the existing global search input into the mobile navigation Drawer as an additional mobile-only instance while keeping the desktop input unchanged. Keep the full grouped navigation beneath it.

- [ ] **Step 6: Implement responsive CSS and default focus**

Add:

```css
.platform-skip-link {
  position: fixed;
  top: 8px;
  left: 8px;
  z-index: 1200;
  transform: translateY(-160%);
}

.platform-skip-link:focus-visible {
  transform: translateY(0);
}

.platform-shell :where(a, button, input, select, textarea, [role="button"], [role="tab"], [role="menuitem"], [tabindex]:not([tabindex="-1"])):focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}
```

At `max-width: 1023px`, hide the desktop top-nav/workbar bands, show `.platform-mobile-contextbar`, keep the command and context bars at 44px minimum targets, and keep content heading geometry within the design thresholds. At `min-width: 1024px`, hide the compact mobile context bar.

- [ ] **Step 7: Run focused validation**

Run:

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk npm --prefix admin run typecheck
```

Expected: all pass.

- [ ] **Step 8: Commit Task 1**

```bash
rtk git add admin/src/platform/shell/AdminShell.tsx admin/src/platform/i18n.ts admin/src/styles.css scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "feat: harden admin shell accessibility"
```

### Task 2: Typed API Errors And Localized Session Recovery

**Files:**
- Modify: `admin/src/platform/api/client.ts`
- Modify: `admin/src/App.tsx`
- Modify: `admin/src/platform/refine/authProvider.ts`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Produces: `AdminAPIError`, `ADMIN_SESSION_EXPIRED_EVENT`, shared response parsing, and an App-level recovery listener.
- Consumes: existing local-storage token functions and `dictionary.sessionExpired` from Task 1.

- [ ] **Step 1: Add failing auth-recovery source contracts**

Require:

```js
requireIncludes(files.client, "export class AdminAPIError", "Admin API failures must expose typed status codes.");
requireIncludes(files.client, "ADMIN_SESSION_EXPIRED_EVENT", "The shared client must expose the session-expired event contract.");
requireIncludes(files.client, "statusCode", "Admin API errors must carry HTTP status.");
requireIncludes(files.client, "dispatchEvent", "Stored-token 401 responses must notify the app.");
requireIncludes(files.app, "ADMIN_SESSION_EXPIRED_EVENT", "App must listen for shared session expiry.");
requireIncludes(files.app, "dictionary.sessionExpired", "Session expiry feedback must be localized.");
requireIncludes(files.client, "parsePlatformResponse", "Direct fetch helpers must share response normalization.");
```

Add a negative fixture that renames `ADMIN_SESSION_EXPIRED_EVENT` and expects failure.

- [ ] **Step 2: Run the focused test and verify failure**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
```

Expected: FAIL on the new auth-recovery contracts.

- [ ] **Step 3: Implement typed response errors**

Add:

```ts
export const ADMIN_SESSION_EXPIRED_EVENT = "platform:auth:session-expired";

export class AdminAPIError extends Error {
  constructor(
    message: string,
    readonly statusCode: number,
    readonly code = "",
  ) {
    super(message);
    this.name = "AdminAPIError";
  }
}
```

Implement one `parsePlatformResponse` helper that reads JSON, throws `AdminAPIError`, and invokes one `handleUnauthorizedResponse(statusCode, hadToken)` path. The 401 path clears the stored token and dispatches `ADMIN_SESSION_EXPIRED_EVENT` only when the request started with a stored token.

Route `request`, OpenAPI fetch, upload, download, and authenticated file-content helpers through the same normalization or unauthorized handler.

- [ ] **Step 4: Implement App recovery state**

Add one `useEffect` event listener that clears session, permissions, denied permissions, dynamic resources, and sets `authError` to `dictionary.sessionExpired`. Remove the listener on unmount.

When initial workspace loading fails with `AdminAPIError.statusCode === 401`, use the localized recovery message instead of the raw backend message. Preserve normalized messages for non-401 failures.

- [ ] **Step 5: Align Refine auth handling**

Use `error instanceof AdminAPIError` in `authProvider.onError`; keep the existing logout/redirect behavior for status 401.

- [ ] **Step 6: Run focused validation**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk npm --prefix admin run typecheck
```

Expected: all pass.

- [ ] **Step 7: Commit Task 2**

```bash
rtk git add admin/src/platform/api/client.ts admin/src/App.tsx admin/src/platform/refine/authProvider.ts scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "fix: recover expired admin sessions"
```

### Task 3: Responsive Table Priority, Form Initial Focus, And Reduced Motion

**Files:**
- Modify: `admin/src/platform/ui/PlatformDataTable.tsx`
- Modify: `admin/src/platform/resources/GenericResourceConsole.tsx`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`

**Interfaces:**
- Produces: `PlatformDataTableColumnPriority`, deterministic schema-order priority assignment, first-field modal focus, and reduced-motion CSS.
- Consumes: Task 1 default focus CSS and existing `AdminFormModal`/Ant Design Table behavior.

- [ ] **Step 1: Add failing contracts**

Require:

```js
requireIncludes(files.table, 'export type PlatformDataTableColumnPriority = "essential" | "standard" | "extended"', "PlatformDataTable must expose responsive priority tiers.");
requireIncludes(files.table, "responsiveBreakpointsForPriority", "PlatformDataTable must map priority to AntD breakpoints.");
requireIncludes(files.resourceConsole, "tableColumnPriority(index)", "Generic resource tables must derive priority from schema order.");
requireIncludes(files.resourceConsole, "form.getFieldInstance", "Resource modals must focus the first editable schema field.");
requireRegex(files.styles, /@media\s*\(prefers-reduced-motion:\s*reduce\)/, "styles.css must respect reduced motion.");
```

Add one negative fixture for removed reduced-motion media query.

- [ ] **Step 2: Run focused tests and verify failure**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
```

Expected: FAIL on the new table, focus, and motion contracts.

- [ ] **Step 3: Add table priority mapping**

Extend the platform column type:

```ts
export type PlatformDataTableColumnPriority = "essential" | "standard" | "extended";

export type PlatformDataTableColumn<T> = TableColumnsType<T>[number] & {
  defaultHidden?: boolean;
  priority?: PlatformDataTableColumnPriority;
};
```

Map priorities to AntD breakpoints:

```ts
function responsiveBreakpointsForPriority(priority: PlatformDataTableColumnPriority | undefined) {
  if (priority === "standard") return ["xl", "xxl"] as const;
  if (priority === "extended") return ["xxl"] as const;
  return ["md", "lg", "xl", "xxl"] as const;
}
```

Merge this into caller columns without changing explicit caller-provided `responsive` values.

- [ ] **Step 4: Assign generic resource priorities by schema order**

Add:

```ts
function tableColumnPriority(index: number): PlatformDataTableColumnPriority {
  if (index < 4) return "essential";
  if (index < 7) return "standard";
  return "extended";
}
```

Use the map callback index to set `priority`. Do not inspect field keys or resource names.

- [ ] **Step 5: Focus the first editable form field**

Pass `afterOpenChange` to `AdminFormModal`. When opened, request one animation frame and focus `form.getFieldInstance(formFields[0]?.key)` when the instance exposes `focus()`. Do nothing when there is no editable field.

- [ ] **Step 6: Add reduced-motion overrides**

Add:

```css
@media (prefers-reduced-motion: reduce) {
  .transition-enabled .admin-page,
  .transition-enabled .dashboard-home {
    animation: none;
  }

  .platform-shell *,
  .platform-shell *::before,
  .platform-shell *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
```

- [ ] **Step 7: Run focused and shared validation**

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk npm --prefix admin run build
```

Expected: all pass.

- [ ] **Step 8: Commit Task 3**

```bash
rtk git add admin/src/platform/ui/PlatformDataTable.tsx admin/src/platform/resources/GenericResourceConsole.tsx admin/src/styles.css scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs
rtk git commit -m "feat: prioritize responsive admin tables"
```

### Task 4: Governance Activation, Browser QA, Documentation, And Closeout

**Files:**
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-ui-optimization-assessment.md`
- Modify: `docs/admin-ui-foundation.md`
- Modify: `design-qa.md`
- Create: browser evidence under `tmp/product-design/p1-admin-ui-hardening-20260711/`

**Interfaces:**
- Consumes: all Task 1-3 commits and verification evidence.
- Produces: implemented `admin-ui-system-quality-hardening` node, 36/36/0 governance counts, closeout evidence, and final responsive/browser acceptance.

- [ ] **Step 1: Add the implemented task node and aligned audit entries**

Add `admin-ui-system-quality-hardening` with:

```json
{
  "id": "admin-ui-system-quality-hardening",
  "title": {
    "zh": "管理台 UI 系统质量加固",
    "en": "Admin UI System Quality Hardening"
  },
  "phase": "admin-experience",
  "scope": "admin-ui",
  "status": "implemented",
  "visual": true,
  "designGate": ["superpowers:brainstorming", "product-design"],
  "resourceLocks": ["admin-ui", "i18n", "browser-qa", "docs"],
  "dependsOn": [
    "admin-ui-shell-and-list-components",
    "visual-product-design-qa",
    "production-persistence-correctness"
  ]
}
```

Populate evidence with the design, plan, validators, tests, browser screenshot paths, and source files. Update every governance count and node list to 36 implemented, zero preview/planned/deferred nodes. Add a node closeout with `neatFreak: true`, the four required dimensions, and visual evidence `superpowers:brainstorming` plus `product-design`.

- [ ] **Step 2: Update user-facing docs**

Record the implemented behavior, breakpoint rules, keyboard behavior, stale-session recovery, reduced-motion behavior, browser evidence, and remaining WCAG evidence limits. Correct the stale 34/34 references to the final 36/36 state.

- [ ] **Step 3: Run narrow governance validation**

```bash
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
```

Expected: all pass.

- [ ] **Step 4: Run browser acceptance**

Start or reuse the local API and Admin dev servers. Capture and inspect login, dashboard, menu list, create modal, and settings drawer at the six required viewports. Use DOM measurements to verify heading geometry, target sizes, no page overflow, explicit names, route focus, responsive columns, initial modal focus, focus return, reduced motion, and console errors.

Reject and recapture any blank, loading, cropped, or wrong-state screenshot.

- [ ] **Step 5: Run neat-freak closeout audit**

Follow the repository-scoped neat-freak workflow. Do not update Codex memory unless the user explicitly requests it. Confirm docs reflect the code, governance files agree, ignored evidence directories remain ignored, and no unrelated metadata was added.

- [ ] **Step 6: Run full final verification**

```bash
rtk go test ./...
rtk node --test scripts/*.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk node scripts/validate-platform-reference-coverage.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-platform-file-storage-experience.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

Expected: all pass and CodeGraph reports up to date.

- [ ] **Step 7: Commit Task 4**

```bash
rtk git add resources/platform-foundation-task-graph.json resources/platform-task-execution-audit.json resources/platform-foundation-alignment-audit.json resources/platform-goal-completion-audit.json resources/platform-objective-conformance.json resources/platform-node-closeout-audit.json docs/platform-roadmap.md docs/platform-foundation-task-map.md docs/platform-ui-optimization-assessment.md docs/admin-ui-foundation.md design-qa.md
rtk git commit -m "docs: close admin ui hardening node"
```

## Final Review

After all task commits:

- Generate one review package from commit `15831d7` to `HEAD`.
- Dispatch a whole-branch reviewer against this plan and the design spec.
- Fix every Critical or Important finding in one focused follow-up commit.
- Re-run affected tests and the final verification set.
- Confirm `rtk git status --short --branch` is clean before handoff.
