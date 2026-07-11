# Admin UI System Quality Hardening Design

Date: 2026-07-11
Status: approved for implementation

## Goal

Harden the shared Admin UI for keyboard access, mobile and tablet operation, responsive data scanning, stale-session recovery, and reduced-motion preferences without changing the existing desktop information architecture or quiet operational visual language.

The implementation uses Product Design audit evidence as the primary product input and `ui-ux-pro-max` plus the accessibility specialist as implementation checklists. `design-taste-frontend` remains deferred because this task concerns a dense admin workflow rather than a landing, portfolio, marketing, or brand-redesign surface.

## Approved Direction

The approved mobile direction is **B: compact two-tier shell**.

- At widths up to 1023px, replace the current five persistent shell bands with two persistent bands.
- Keep the desktop shell unchanged at 1024px and above.
- Preserve existing Ant Design, Refine, platform wrapper, theme, layout, work-tab, navigation, context, and i18n contracts.
- Use progressive disclosure for mobile global search, full navigation, open work tabs, environment context, and tenant context.

The selected direction was reviewed through `superpowers:brainstorming` and the browser-based visual companion. Product Design audit evidence was captured from the current implementation before this design was approved.

## Current Evidence

Fresh evidence from 2026-07-11 is stored under `tmp/product-design/p1-admin-ui-audit-20260711/`.

- The mobile menu heading begins at approximately 319px at 390x844 and 325px at 768x1024.
- Common mobile shell, toolbar, work-tab, and context controls are 30-32px high.
- The mobile menu and alert buttons do not have explicit localized accessible names.
- No skip link is present and route changes leave focus on `body`.
- The default focus rule is conditional on `.visual-aid-enabled`.
- The 1280px menu table has a 2266px intrinsic width and relies on horizontal clipping or scrolling to reach lower-priority columns.
- An invalid stored session can expose the raw `unauthorized` response on the login screen.
- Page-entry motion has no `prefers-reduced-motion` override.
- Ant Design restores focus to the create trigger when the form modal closes, but the modal initially focuses a generic container rather than the first editable field.

## Scope

### 1. Compact Responsive Shell

The responsive shell uses these boundaries:

- `0-767px`: compact two-tier shell plus existing mobile resource cards.
- `768-1023px`: compact two-tier shell plus a priority-reduced data table.
- `1024px and above`: existing desktop shell, side navigation, top navigation, work tabs, and separate environment/tenant controls.

The compact shell contains:

1. A command bar with a 44px mobile navigation trigger, brand/current-page label, language control, alert control, and account/settings control.
2. A context bar with two 44px controls:
   - current domain and active work tab;
   - combined environment and tenant summary.

Progressive disclosure behavior:

- The mobile navigation Drawer keeps the full grouped navigation and places the existing global search input at the top.
- The active-tab control opens the existing open work-tab set and allows route selection; close operations remain available inside the tab menu.
- The context control opens one panel containing the current environment and tenant values. The values remain read-only.
- Desktop language, search, alert, account, navigation, work-tab, and context placement stays unchanged.

Geometry acceptance:

- The first page heading begins at or before 120px on 375x812 and 390x844.
- The first page heading begins at or before 128px on 768x1024.
- All compact-shell interactive targets are at least 44x44px.
- The compact shell does not create page-level horizontal overflow.

### 2. Keyboard And Focus Behavior

The shell adds a localized skip link before navigation. It targets a stable `main` region identifier.

The main region:

- uses native `main` semantics;
- has `tabIndex={-1}` so it can receive programmatic focus;
- receives focus after an actual route change, not on the first authenticated render;
- preserves normal browser focus when the active route does not change.

Visible focus is a platform default:

- all `:focus-visible` interactive elements receive a 2px primary outline with a 2px offset;
- the visual-aid preference adds stronger boundary emphasis but is no longer the only source of visible focus;
- native Ant Design focus styles are preserved where they are stronger.

Icon-only controls receive explicit localized names. This includes the mobile navigation trigger and alert trigger, while existing language, sidebar, account, table-toolbar, row-action, and work-tab-close labels remain intact.

The generic resource form modal moves initial focus to the first editable schema field after opening. Ant Design continues to own focus trapping, Escape handling, and trigger restoration.

### 3. Responsive Table Priority

The backend admin resource schema remains unchanged. Existing schema field order is the priority contract for generic tables.

`PlatformDataTableColumn` gains a frontend-only responsive tier:

- `essential`: visible at 768px and above;
- `standard`: visible at 1200px and above;
- `extended`: visible at 1600px and above.

`GenericResourceConsole` assigns tiers by visible schema order:

- the first four table fields are essential;
- the next three are standard;
- remaining fields are extended.

Selection and row-action columns remain visible at every table breakpoint. At 767px and below, existing mobile cards remain authoritative. Full record values stay discoverable through the existing row detail action and column settings; no business field names are hard-coded into the platform shell or table primitive.

The expected result is:

- 768px and 1024px prioritize name, code, status, and the next schema-defined comparison field;
- 1280px adds the next three schema-defined fields;
- wide screens can show the full schema-defined table set;
- fixed row actions remain operable without page-level horizontal overflow.

### 4. Stale-Session Recovery

The shared API client introduces a typed `AdminAPIError` carrying `statusCode` and optional platform error code. Every platform API response path uses the same response/error normalization, including OpenAPI and file upload/download helpers that currently call `fetch` directly.

When a request returns HTTP 401 while an admin token is stored:

1. Clear the token once.
2. Dispatch one platform session-expired event.
3. Let `App` clear authenticated workspace state and return to the login screen.
4. Show a localized, actionable message stating that the session expired and the user should sign in again.

Provider discovery and initial unauthenticated login failures do not emit the session-expired event because no stored admin token exists. Non-401 errors keep their normalized backend message and status for existing resource-level error handling.

The Refine auth provider consumes the typed `statusCode` instead of relying on an untyped property that the current client never sets.

### 5. Reduced Motion

The CSS adds `@media (prefers-reduced-motion: reduce)` rules that:

- disable platform page-entry animations;
- reduce non-essential transform and opacity transitions to effectively immediate state changes;
- keep loading, focus, validation, and status feedback visible;
- avoid changing layout or disabling user input.

The existing page-transition preference remains available, but operating-system reduced-motion preference takes precedence.

## Components And Files

Primary implementation surfaces:

- `admin/src/platform/shell/AdminShell.tsx`: compact shell, skip link, route focus, icon labels, progressive-disclosure controls.
- `admin/src/styles.css`: default focus, compact-shell geometry, 44px mobile targets, responsive table behavior, reduced motion.
- `admin/src/platform/i18n.ts`: Chinese and English labels for skip link, mobile navigation, tab/context summaries, session expiry, and recovery.
- `admin/src/platform/api/client.ts`: typed response errors, shared unauthorized handling, session-expired event.
- `admin/src/App.tsx`: session-expired listener and localized login recovery state.
- `admin/src/platform/refine/authProvider.ts`: typed 401 integration.
- `admin/src/platform/ui/PlatformDataTable.tsx`: responsive column tier mapping.
- `admin/src/platform/resources/GenericResourceConsole.tsx`: schema-order priority assignment and form initial focus.
- `scripts/validate-admin-ui-contracts.mjs` and `scripts/admin-ui-contracts.test.mjs`: source contracts and negative fixtures for the new shared behavior.

No new frontend dependency is required.

## Testing And Acceptance

Automated acceptance:

- admin i18n validator passes with matching Chinese and English keys;
- Admin UI contract validator and tests cover skip link, route-focus target, explicit icon labels, compact-shell classes, responsive table tiers, 44px mobile targets, session-expired normalization, modal initial focus, and reduced-motion rules;
- TypeScript typecheck and Vite production build pass;
- existing Refine, resource, form-slot, and API-boundary validators remain green;
- `git diff --check` passes.

Browser acceptance uses fresh captures at:

- 375x812;
- 390x844;
- 768x1024;
- 1024x768;
- 1280x720;
- 1440x1024.

The keyboard walkthrough covers:

- skip link to main content;
- global and mobile navigation;
- route change focus;
- one generic resource list;
- create/edit modal initial focus, trap, Escape, and trigger restoration;
- settings drawer focus and restoration.

Browser checks also confirm:

- no page-level horizontal overflow;
- compact-shell geometry targets;
- 44px mobile hit areas;
- localized icon names;
- card/table switching;
- 1024/1280 column priority;
- reduced-motion computed behavior;
- no new console errors.

Screenshot and DOM inspection are evidence for this implementation, not a claim of complete WCAG conformance. Screen-reader announcements, high zoom, and platform-specific assistive technology remain explicit evidence limits unless separately tested.

## Governance And Closeout

Activate one new task node: `admin-ui-system-quality-hardening`.

The node:

- depends on `admin-ui-shell-and-list-components`, `visual-product-design-qa`, and `production-persistence-correctness`;
- locks `admin-ui`, `i18n`, `browser-qa`, and `docs`;
- records `superpowers:brainstorming` and Product Design evidence;
- records this design, implementation plan, contract tests, build, browser captures, and neat-freak cleanup evidence;
- updates task execution, goal completion, alignment, objective conformance, node closeout, roadmap, task map, Admin UI foundation, UI optimization assessment, and design QA documents.

Completion changes the task graph from 35 implemented nodes to 36 implemented nodes with no pending or blocked node.

## Non-Goals

- No landing, portfolio, marketing, or login-brand visual redesign.
- No color, typography, icon-family, theme, or desktop navigation redesign.
- No backend admin resource schema extension for table priority.
- No replacement of Ant Design, Refine, or platform wrappers.
- No new business-specific columns, menus, permissions, or workflows.
- No claim of full WCAG certification.
