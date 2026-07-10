# Product Design QA

Date: 2026-07-04

## Source Visual Truth Path

- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-ead03ade-ae5c-4b33-b8ad-257c44e525b0.png`
- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-8d140a6b-250b-4601-b041-f20d7ba6c946.png`

## Implementation Screenshot Path

- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/admin-api-resources-desktop-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/admin-settings-menu-desktop-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/admin-api-resources-mobile-390x844.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/admin-ui-comparison.png`

## Viewport

- Desktop API resource state: `1440x1024`
- Desktop settings menu state: `1440x1024`
- Mobile API resource state: `390x844`

## State

- Theme: `tech`
- Language: `zh`, with icon toggle verified to switch the page title to `API Resources`
- Layout: `mixed`
- Active resource: `API 资源`
- API: local API on `127.0.0.1:9200`
- Admin web: local Vite server on `127.0.0.1:9202`

## Full-View Comparison Evidence

Comparison image: `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/admin-ui-comparison.png`

The comparison checks the two provided issue screenshots against:

- desktop API resource page after navigation changes;
- desktop account/settings menu;
- mobile API resource page.

## Focused Region Comparison Evidence

- Navigation conflict: `.platform-resource-tabs` count is `0`; work tabs render under `.platform-work-tabs` and show closable tabs for `能力清单` and `API 资源`.
- Top-right controls: `.theme-select` count is `0`; `.language-select` count is `0`; `.user-menu-trigger` count is `1`.
- Settings menu: account trigger opens a menu containing theme options `科技风 / 高级白 / 炫酷黑 / 温暖黄` and layout options `左右布局 / 上下布局 / 混合布局 / 分栏布局`.
- Language switch: `.language-toggle-button` count is `1`; clicking it switches the API resource title from `API 资源` to `API Resources`.
- Mobile responsive state: `overflowX` is `false`; sidebar display is hidden; mobile nav button is visible; resource table display is `none`; mobile resource list display is `grid`; avatar text remains `A`.

## Findings

No actionable P0/P1/P2 findings remain.

P3 follow-up polish:

- Mobile metric strip is intentionally stacked for safety at `390px`; later slices can add a denser mobile metric variant if real data pages need more above-the-fold list rows.

## Patches Made Since Previous QA Pass

- Replaced permanent horizontal resource navigation with browser-like closable work tabs.
- Moved theme and layout controls into the account/settings dropdown.
- Replaced language dropdown with an icon button.
- Reduced page heading, card, metric and table toolbar density through shared CSS tokens.
- Added `AdminDesignProvider` as the AntD theme/locale provider for the admin foundation.
- Added platform UI primitives over AntD: `AdminPage`, `AdminMetricStrip`, `AdminListPanel`, `AdminActionButton`, `AdminFeedback`, `AdminFormModal`.
- Rewired capability and generic resource pages to use platform UI primitives instead of hand-writing repeated page, list, feedback, action and modal structure.
- Added `docs/admin-ui-foundation.md` to define when to use platform wrappers versus raw AntD primitives.
- Fixed mobile account trigger so hiding the username does not hide the AntD avatar text.

## Final Result

final result: passed

## 2026-07-05 Follow-Up QA

Additional screenshots:

- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/.superpowers/brainstorm/28542-1783178508/content/admin-list-column-dropdown-20260705.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/.superpowers/brainstorm/28542-1783178508/content/admin-advanced-filter-20260705.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/.superpowers/brainstorm/28542-1783178508/content/admin-settings-drawer-20260705.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/.superpowers/brainstorm/28542-1783178508/content/admin-sidebar-collapsed-20260705.png`

Checked states:

- Menu resource list renders the shared `PlatformDataTable` with centered integrated pagination.
- Column settings and advanced filters use solid `PlatformDropdownPanel` surfaces.
- Advanced filters include keyword fields, status select and datetime range fields with safe query guidance.
- Row selection checkboxes expose localized row labels.
- Settings drawer shows current user, theme, layout and language tags; layout and assist tabs expose page transition, sidebar collapse, watermark, visual aid and layout legend controls.
- Manual sidebar collapse changes the shell class to `platform-shell layout-mixed sider-collapsed transition-enabled`, sidebar width to `64px`, and the button label to `展开侧栏`.
- Browser console after refresh has no business errors or accessibility issues; only Vite and React DevTools development info remains.

## 2026-07-06 Current-State QA

Source visual truth path:

- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-49c95835-1877-44a3-910a-23aef216edce.png`
- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-2cb5b51d-69a9-48e6-93bb-6d86cc460d09.png`
- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-46454074-9e7f-46bb-ac1b-202777c4d6d9.png`
- `/var/folders/22/yqxb34ks75jdwy4c2_n1mwwm0000gn/T/codex-clipboard-e3e8b4b3-13a2-408f-9d0a-f11b5241f204.png`
- `docs/admin-ui-foundation.md`

Implementation screenshot path:

- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/03-menus-list-desktop-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/04-user-menu-desktop-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/05-settings-black-layout-tab-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/06-sidebar-collapsed-black-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/07-column-menu-black-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/08-filter-menu-black-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/09-work-tab-context-menu-black-1440x1024.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/11-menus-mobile-390x844-after-fix.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/visual-product-design-qa-20260706/12-dashboard-desktop-1440x1024-after-fix.png`

Viewport:

- Desktop resource list, settings drawer, column menu, filter menu and work-tab menu: `1440x1024`
- Narrow resource list: `390x844`

State:

- Theme: `black` after theme switch verification; `tech` was also captured before switching.
- Layout: `mixed`.
- Sidebar: expanded and manually collapsed states verified.
- Active resource: `menus`.
- Admin web: local Vite server on `127.0.0.1:9202`.

Full-view comparison evidence:

- Desktop list state showed no horizontal page overflow; `PlatformDataTable` used compact table density, switch-style status cells and integrated pagination.
- Settings drawer showed appearance, layout, general and assist tabs, four themes, layout visual legends, transition, watermark and visual-aid controls.
- Work tabs rendered as browser-like tabs with a pinned default overview tab, closable active resource tab and right-click menu.
- Narrow `390x844` state rendered mobile cards before pagination and hid the desktop table.

Focused region comparison evidence:

- Initial narrow capture found a P1 responsive blocker: with `sider-collapsed` active, `.platform-shell` kept `grid-template-columns: 64px 326px`; the hidden sidebar left `.platform-main` in the 64px grid column, compressing content.
- Patch added mobile breakpoint overrides for `.platform-shell.sider-collapsed:not(.layout-top):not(.layout-split)` and `.platform-shell.layout-split.sider-collapsed`, plus mobile width/min-width constraints for main, topbar, top nav, workbar and content.
- Post-fix `390x844` measurement: `.platform-shell` width `390`, `.platform-main` width `390`, `.platform-mobile-list` width `364`, `.platform-pagination-bar` width `364`, `overflowX=false`.
- Desktop post-fix measurement: `.platform-shell` kept `layout-mixed sider-collapsed`, dashboard width stayed usable and `overflowX=false`.

Findings:

- No actionable P0/P1/P2 findings remain after the responsive fix.
- P3 follow-up: account/settings currently opens the settings drawer directly from the avatar button. This is acceptable for the confirmed drawer direction, but a lightweight account dropdown can still be added later if account actions expand beyond settings/logout.

Patches made since previous QA pass:

- Fixed collapsed-sidebar mobile grid override so narrow screens are single-column even when desktop sidebar collapse is persisted.
- Added an admin UI contract gate that requires mobile responsive rules to explicitly override collapsed sidebar grid columns.
- Added current governance assessment docs for org units, role groups and area codes.

final result: passed

## 2026-07-10 UI Optimization Assessment

Status: follow-up tasks identified; no visual implementation started.

Fresh evidence:

- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/ui-optimization-audit-20260710/01-login-current.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/ui-optimization-audit-20260710/02-dashboard-desktop-current.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/ui-optimization-audit-20260710/04-menus-desktop-current.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/ui-optimization-audit-20260710/06-menus-mobile-top-current.png`
- `/Users/irainbow/Documents/DevelopmentSpace/myProject/platform-go/tmp/product-design/ui-optimization-audit-20260710/07-dashboard-mobile-current.png`

Viewport and geometry evidence:

- Desktop dashboard and menu list: `1280x720`, `overflowX=false`.
- Mobile dashboard and menu list: `390x844`, `overflowX=false`.
- Mobile authenticated shell places page content after roughly 300px of global search/actions, primary navigation, work tabs and environment/tenant context controls.
- Measured mobile controls include 30-32px navigation, work-tab and icon-action targets; mobile cards are approximately 53px high.

Findings:

- P1: use `ui-ux-pro-max` for a focused admin system-quality hardening task after production persistence correctness and before the next large admin capability. Required scope includes default focus visibility, skip/route focus, explicit localized icon labels, 44px mobile hit areas, reduced mobile shell chrome, 1024/1280px table prioritization, localized stale-session recovery and reduced-motion handling.
- P1: the login screen may expose raw `unauthorized` after an expired or invalid stored session; normalize it to a localized recovery message instead of presenting a backend response as product copy.
- P2: use `design-taste-frontend` only for login/brand entry and future public landing, portfolio or marketing surfaces. The current repository has no standalone public marketing page, so this stays deferred until product positioning and real brand/product assets exist.
- Guardrail: do not apply marketing-style oversized type, decorative card grids or atmospheric hero treatment to resource lists and governance consoles.

Evidence limits:

- This pass does not claim WCAG conformance. Contrast, screen-reader announcements, zoom/reflow, reduced motion, keyboard route focus and modal focus return still need focused tests.
- The task candidates remain outside the closed 34/34 foundation graph until design approval and explicit activation.

Detailed assessment: `docs/platform-ui-optimization-assessment.md`.
