# Platform UI Optimization Assessment

Date: 2026-07-12
Governance updated: 2026-07-15
Status: Candidate A implemented; Candidate B deferred; completion program active

## Purpose

This assessment records how two design aids are routed without disrupting the platform-foundation goal:

- `ui-ux-pro-max` for implementation-level admin UI quality, accessibility, responsive behavior and shared-component hardening;
- `design-taste-frontend` for the visual direction of login, brand-entry, landing, portfolio, marketing or deliberate redesign surfaces.

Candidate A has completed that activation path as `admin-ui-system-quality-hardening`. Candidate B's broader brand-entry redesign remains deferred until a real public or brand brief is approved. The bounded `admin-watermark-export-governance` and `sensitive-data-reveal-step-up` visual nodes have completed implementation and fresh browser acceptance; `public-docs-site` remains pending and still requires `superpowers:brainstorming`, Product Design and fresh browser evidence.

`runtime-security-containment` and `admin-watermark-export-governance` remain `implemented`; this non-visual service-object node does not replace their security or browser-evidence closeouts.

The original 37-node foundation baseline remains closed. Eleven continuation nodes, including the non-visual `platform-service-contract-standard`, `persisted-query-command-object-runtime` and `integration-ports-disabled-default`, plus the contract-only visual design node `organization-rbac-menu-contract-and-migration-design`, are `implemented`, so current governance is `66 total / 48 implemented / 18 controlled unfinished` with `completionStatus=not-complete-controlled`.

The activated 18-node remainder includes four operational Admin UI nodes: `organization-user-admin-experience`, `role-tree-and-authorization-entry`, `menu-tree-and-button-permission-configuration` and `organization-rbac-menu-e2e-qa`. They use Product Design, platform wrappers, `ui-ux-pro-max`, accessibility and browser evidence, not marketing composition. `public-docs-site` remains the only public marketing/art-direction node and additionally requires `design-taste-frontend`. The exact order and dependencies live in the [activated topology](superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md).

## Current Evidence

The 2026-07-10 audit inspected the local admin at `1280x720` and `390x844` with fresh browser captures:

- `tmp/product-design/ui-optimization-audit-20260710/01-login-current.png`
- `tmp/product-design/ui-optimization-audit-20260710/02-dashboard-desktop-current.png`
- `tmp/product-design/ui-optimization-audit-20260710/04-menus-desktop-current.png`
- `tmp/product-design/ui-optimization-audit-20260710/06-menus-mobile-top-current.png`
- `tmp/product-design/ui-optimization-audit-20260710/07-dashboard-mobile-current.png`

Observed strengths:

- the admin shell, resource routes, mobile resource cards and integrated pagination remain functional;
- desktop and mobile captures have no page-level horizontal overflow;
- shared Ant Design and platform wrappers provide a stable base for system-level improvements;
- active navigation, localized visible labels, explicit form labels and semantic table structures are present.

Observed risks:

- the authenticated mobile shell places global search, account actions, primary navigation, work tabs and two context controls in five persistent bands before page content; the first page heading begins after roughly 300px of browser height;
- many mobile controls measure 30-32px high, including navigation, work tabs, icon actions and table toolbar buttons, below the 44px touch-target baseline used by the requested UI/UX review;
- the default focus-ring rule is conditional on `.visual-aid-enabled`; shared custom controls need a visible focus baseline even when that preference is off;
- the mobile navigation button and alert button rely on icon-derived accessible names instead of explicit localized `aria-label` values;
- the desktop `menus` table at 1280px truncates several high-value columns, reducing scan and comparison quality even though the page itself does not overflow;
- an expired or invalid stored session can expose the raw `unauthorized` backend message on the login screen instead of a localized recovery message;
- the login/brand entry uses a large empty split hero, a one-hue dark treatment and generic capability claims. It is coherent, but it does not yet show real product proof or a distinctive brand asset.

Evidence limits:

- screenshots and DOM inspection do not prove WCAG conformance;
- color contrast, screen-reader announcements, zoom/reflow, reduced motion, keyboard route focus and modal focus return still require focused automated and manual tests;
- no standalone landing, portfolio or marketing page exists in the current repository, so the aesthetic task is conditional rather than an immediate admin redesign.

## Candidate A: Admin UI System Quality Hardening

Status: implemented on 2026-07-11.

Requested aid: `ui-ux-pro-max`

Priority: P1 after the production persistence correctness work described in `docs/superpowers/specs/2026-07-10-platform-production-persistence-correctness-design.md`, before the next large admin capability or downstream business UI is added.

Completed activation gate:

- the P0 production persistence correctness slice is implemented and its shared repository contracts are stable;
- `superpowers:brainstorming` and Product Design approved the compact two-tier direction, focus behavior and table prioritization;
- `admin-ui-system-quality-hardening` is implemented with `admin-ui`, `i18n`, `browser-qa` and `docs` resource locks.

Implementation scope:

- make visible focus treatment a default platform behavior; keep the visual-aid preference as an enhancement rather than the only focus-ring source;
- add a skip-to-content path and move focus to the page heading or main region after route changes;
- add explicit localized accessible names for icon-only shell and table controls;
- bring mobile interactive hit areas to at least 44px without inflating desktop density;
- reduce authenticated mobile chrome so page content is visible earlier, using progressive disclosure for secondary navigation, work tabs and environment/tenant context;
- preserve mobile cards, but review card information priority, active state, pagination order and long-label wrapping at 375/390px;
- define table priority columns and overflow behavior at 1024/1280px so high-value data remains scannable and full values remain discoverable;
- normalize stale-session and authentication errors into localized, actionable recovery feedback;
- respect `prefers-reduced-motion` for route and panel transitions;
- add browser and contract tests for target size, focus visibility, reduced motion, narrow chrome geometry, table/card switching and localized icon labels.

Acceptance evidence:

- browser captures at 375x812, 390x844, 768x1024, 1024x768, 1280x720 and 1440x1024;
- keyboard walkthrough covering login, global navigation, one resource list, create/edit modal and settings drawer;
- automated checks for no horizontal page overflow, visible focus styles, explicit icon labels and mobile hit-area geometry;
- admin i18n, UI contract tests, React build and focused browser console review remain green.

Implemented result:

- `0-767px` uses the compact two-tier shell with mobile resource cards; `768-1023px` keeps the compact shell with priority-reduced tables; `1024px+` preserves the existing desktop shell;
- compact-shell controls use a 44px minimum target; at mobile resource widths the toolbar buttons, search, pagination controls, quick-jumper, settings Drawer close control, tabs and overflow actions also measure at least 44x44px;
- accepted stable states reject page-level horizontal overflow, keep the page heading within the approved compact-shell geometry and report no new application console errors;
- a localized skip link targets the stable main region, actual route changes move focus there, and shared focus-visible styling no longer depends on the visual-aid preference;
- after the generic resource modal settles, it focuses the first enabled editable field inside the currently visible form; Escape closes it and focus returns to the create/edit trigger through Ant Design;
- schema field order drives essential, standard and extended table tiers without adding business field names to the shared shell;
- stored-token 401 responses clear the stale token once, emit the shared session-expired event and present localized sign-in recovery;
- operating-system reduced-motion preference produces effectively immediate computed animation/transition styles for platform and portaled Ant Design motion without hiding focus, validation, loading or status feedback.

Fresh implementation evidence is under `tmp/product-design/p1-admin-ui-hardening-20260711/` and covers login, dashboard, menu list, create modal, settings drawer and localized stale-session recovery across the required 375, 390, 768, 1024, 1280 and 1440 widths. `09-settings-drawer-390x844.png` records the accepted narrow Drawer state. `10-stale-session-390x844.png` records “会话已过期，请重新登录。” without raw `unauthorized` copy.

## Watermark And Export Governance Closeout

Status: implemented on 2026-07-12.

Requested aid: `ui-ux-pro-max`, with `superpowers:brainstorming` and Product Design evidence.

Implemented result:

- one master switch progressively reveals independent screen/export scope controls; valid empty, screen-only, export-only and combined scope selections persist through normalization;
- screen watermark count is restricted to exact `1`, `4`, `9` and `16` DOM marks, rendered through one fixed inert viewport layer with `aria-hidden="true"` and no pointer capture;
- the layer covers the topbar, sidebar, lists, dashboards, drawers, dropdowns and modals; at `<=768px`, sixteen marks reflow to `2x8` and retain readable attribution;
- the master switch has an explicit localized accessible name, mobile controls retain the shared 44px target contract and the `390x844` settings state has no horizontal overflow;
- light and dark themes preserve readable settings hierarchy without turning the operational drawer into a promotional surface;
- Policy Review JSON export alone accepts explicit watermark intent and returns structured provenance; canonical OpenAPI, original file bytes and other export formats are unchanged.

Accepted evidence is stored under `.superpowers/product-design-audit/watermark/`: settings controls, sixteen-mark desktop layout, full-viewport dashboard/navigation/dropdown/modal coverage, responsive `2x8` mobile layout, dark theme and Policy Review export success. Runtime inspection confirmed exact `1/4/9/16` DOM counts, full viewport bounds, overlay stacking, pointer pass-through, no truncated narrow sixteen-mark text, a clean current-run console, and export metadata whose `exportedAt` matches the export timestamp while audit policy records only `watermarkApplied=true`.

## Sensitive Data Reveal Step-Up Closeout

Status: implemented on 2026-07-13.

Requested aids: Product Design audit and `ui-ux-pro-max`, with the required `superpowers:brainstorming` design gate.

Implemented result:

- only manifest-declared encrypted detail fields expose the reveal action, with dedicated permission, purpose and copy policy;
- the modal progressively handles purpose, `anyOf`/`allOf` factor selection/progress, SMS OTP, OIDC reauthentication and the expiring plaintext state;
- modal headings receive focus, controls meet the 44px target contract, mobile actions stack without horizontal overflow, and plaintext clears on close, page hide or expiry;
- verification failure returns a sanitized message and `422` without clearing the main Admin session;
- browser evidence under `.superpowers/product-design-audit/sensitive-reveal/` covers factor selection, SMS verification, revealed value and the exact `390x844` mobile result.

## Candidate B: Brand Entry And Public-Surface Visual Redesign

Requested aid: `design-taste-frontend`

Priority: P2 deferred until a public-facing surface or an approved login/brand-entry redesign has a real product-positioning goal.

Activation triggers:

- a landing, portfolio, marketing or public product page is added;
- product name, audience, primary promise, brand assets and primary conversion action are approved;
- or the login page is explicitly promoted from a utility authentication screen into a product/brand entry experience.

Scope boundary:

- apply the aesthetic direction to login, branding, public documentation and future marketing surfaces;
- do not apply marketing composition, oversized promotional typography or decorative card grids to CRUD consoles, policy review, file management or other repeated operational workflows;
- show the actual product or a clear product state when a public hero needs media; avoid generic abstract decoration and empty atmospheric space;
- make the brand or literal product offer a first-viewport signal, with restrained supporting copy and one clear primary action;
- keep responsive, accessible and performance acceptance criteria owned by Candidate A rather than trading usability for visual novelty.

Completion gate:

- three bounded visual directions are reviewed against the same content and viewport set;
- one direction is approved through `superpowers:brainstorming` and Product Design;
- final implementation has real brand/product assets, desktop/mobile screenshots, contrast checks and no regression to login completion speed.

## Scheduling Decision

1. Production persistence correctness and Candidate A are implemented.
2. Keep Candidate B deferred until there is a real brand/public-surface brief. If only the admin console changes, use the existing quiet operational design language and the implemented Candidate A contracts.
3. Any future visual candidate requires an explicit task-graph node, design gates, resource locks and fresh screenshot evidence. Candidate A and production Admin OIDC remain closed in the original 37-node baseline; `admin-watermark-export-governance`, `sensitive-data-reveal-step-up` and the contract-only `organization-rbac-menu-contract-and-migration-design` node are implemented, while the organization Admin runtime/UX lane and `public-docs-site` remain controlled pending work in the active `66 total / 48 implemented / 18 controlled unfinished` completion program.

Evidence remains scoped: screenshots, DOM measurements and keyboard checks support this implementation but do not certify WCAG conformance. Screen-reader announcements, high zoom/reflow and platform-specific assistive technology require separate evidence.
