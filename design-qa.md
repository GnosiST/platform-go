# Product Design QA

Date: 2026-07-04

## Source Visual Truth Path

- `<temporary-path>`
- `<temporary-path>`

## Implementation Screenshot Path

- `./tmp/product-design/admin-api-resources-desktop-1440x1024.png`
- `./tmp/product-design/admin-settings-menu-desktop-1440x1024.png`
- `./tmp/product-design/admin-api-resources-mobile-390x844.png`
- `./tmp/product-design/admin-ui-comparison.png`

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

Comparison image: `./tmp/product-design/admin-ui-comparison.png`

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

## 2026-07-15 Menu Tree And Button Permission Configuration QA

Status: implemented for `menu-tree-and-button-permission-configuration`; full organization E2E and migration cutover remain pending.

The tracked manifest `resources/evidence/menu-tree-and-button-permission-configuration-20260715.json` covers `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720` and `1440x1024`. Click collapse/expand, page selection, edit forms, typed parameters, page-button metadata, focus return, modal scrolling, zero horizontal overflow, zero console warnings/errors and zero failed resource responses were accepted.

The frozen audit found that focusing Ant Tree's hidden screen-reader input and pressing ArrowRight did not expand the selected collapsed directory. `AdminTreeWorkbench` now synchronizes the selected key to Ant Tree's active keyboard node while preserving Ant's click and arrow navigation, with executable Admin UI contract coverage. A scoped post-fix live-browser rerun at `1440x1024` collapsed and selected `验收治理`, focused `.admin-tree-workbench-tree input[aria-label="for screen reader"]`, verified `ArrowRight` expanded it and exposed `菜单验收页`, then verified `ArrowLeft` collapsed it again. The manifest does not claim broader keyboard coverage, Tree Transfer 10,000-node performance, full Tree Transfer acceptance, principal-level dual-read equivalence, migration cutover/rollback or organization E2E.

final result: passed with scoped post-fix keyboard rerun

## 2026-07-11 Task 8 Production-Like Admin OIDC Acceptance

Status: implemented and accepted for the reusable production Admin OIDC foundation node. External production promotion remains `not-approved`; runtime mutation and the independent refresh-token-family runtime remain disabled.

Production-like runtime:

- Keycloak image: `quay.io/keycloak/keycloak:26.3.3`, digest `sha256:6a7217a100bd3e5de4063a27a538ef999a3c5a88c4b4ec0ffc0a642aee7b2597`.
- Container: `platform-oidc-task8`, bound as `127.0.0.1:19180 -> 8080`.
- Realm/client: `platform-rehearsal` with confidential `platform-admin`, authorization-code flow, direct grants disabled, exact redirect and origin `http://127.0.0.1:19182/login`.
- Platform API: `127.0.0.1:19181`; Admin Vite: `127.0.0.1:19182`.
- Admin resource, session and lifecycle stores used ignored SQLite files below `tmp/product-design/production-admin-oidc-auth-20260711/runtime/`.
- Demo auth was disabled, `admin-oidc` was enabled, and the subject entered only through stdin. No raw subject or secret is recorded in this QA artifact.

Sanitized commands:

```bash
docker run -d --name platform-oidc-task8 -p 127.0.0.1:19180:8080 \
  -e KC_BOOTSTRAP_ADMIN_USERNAME=<redacted-admin> \
  -e KC_BOOTSTRAP_ADMIN_PASSWORD=<redacted-secret> \
  quay.io/keycloak/keycloak:26.3.3 \
  start-dev --http-port=8080 --hostname-strict=false

kcadm.sh create realms \
  -s realm=platform-rehearsal \
  -s enabled=true \
  -s sslRequired=external \
  -s registrationAllowed=false

# client platform-admin: protocol=openid-connect, enabled=true,
# publicClient=false, standardFlowEnabled=true, directAccessGrantsEnabled=false,
# redirectUris=[http://127.0.0.1:19182/login],
# webOrigins=[http://127.0.0.1:19182], secret=<redacted-secret>

printf '%s' '<subject-via-stdin>' | env <same-sqlite-and-oidc-env> \
  go run ./cmd/platform-admin bind-admin-oidc \
  --provider oidc \
  --issuer http://127.0.0.1:19180/realms/platform-rehearsal \
  --username admin \
  --subject-stdin

env PLATFORM_RUNTIME_ENV=development \
  PLATFORM_HTTP_ADDR=127.0.0.1:19181 \
  PLATFORM_CAPABILITIES=tenant,identity,session,rbac,menu,api-resource,audit,dictionary,parameter,file-storage,admin-shell,system-admin,admin-oidc \
  PLATFORM_ADMIN_RESOURCE_DRIVER=sqlite PLATFORM_ADMIN_RESOURCE_DSN=tmp/.../admin.db \
  PLATFORM_SESSION_DRIVER=sqlite PLATFORM_SESSION_DSN=tmp/.../session.db \
  PLATFORM_LIFECYCLE_HISTORY_DRIVER=sqlite PLATFORM_LIFECYCLE_HISTORY_DSN=tmp/.../lifecycle.db \
  PLATFORM_CACHE_DRIVER=memory PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true \
  PLATFORM_ADMIN_OIDC_ISSUER_URL=http://127.0.0.1:19180/realms/platform-rehearsal \
  PLATFORM_ADMIN_OIDC_CLIENT_ID=platform-admin \
  PLATFORM_ADMIN_OIDC_CLIENT_SECRET=<redacted-secret> \
  PLATFORM_ADMIN_OIDC_REDIRECT_URL=http://127.0.0.1:19182/login \
  go run ./cmd/platform-api

env VITE_PLATFORM_API_PROXY_TARGET=http://127.0.0.1:19181 \
  npm --prefix admin run dev -- --host 127.0.0.1 --port 19182
```

Runtime outcomes:

- Successful and repeated bound-user login reached authenticated overview; refresh and protected `/users` navigation returned 200, logout returned 200, and the revoked session returned 401.
- Missing binding and disabled user returned the same normalized `AUTH_IDENTITY_NOT_BOUND` response without credential or identity leakage.
- Cancellation and invalid state returned to `/login`, cleared URL search values, announced the failure through `aria-live="polite"`, focused the error heading and exposed a 44px recovery action.
- A real Keycloak form remained open for 470 seconds, beyond the five-minute server transaction window. Submission returned to `/login`, cleared URL search values, focused the error heading and announced `登录已超时，请重新开始登录。`.
- The archived evidence set comprises the tracked manifest, top-level redacted JSON summaries and screenshots. It contains no authorization code, state, nonce, verifier, claims, subject, token or credential values. The local `runtime/` harness directory is excluded, and its three credential-bearing browser fixture files were deleted after verification.

Responsive and accessibility evidence:

- Viewports: `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720`, `1440x1024`.
- The four touch-oriented viewports had `overflow=false`, 52px provider controls, a 44px OIDC primary action, a 44x44 language control and four 44x44 theme swatches. Failure recovery actions were 313px, 328px, 380px and 380px wide, all 44px high.
- Desktop widths had no horizontal overflow. Theme swatches were 26x26px, above the 24px WCAG target-size floor; the 44px touch rule applies through 1024px.
- CDP reduced-motion emulation inspected 67 login descendants; the maximum computed animation or transition duration was `0.00001s`.
- Final console warning/error collection was empty.

Evidence:

- `resources/evidence/production-admin-oidc-auth-20260711.json` (tracked screenshot hashes, runtime outcomes, redaction result and promotion boundary)
- `tmp/product-design/production-admin-oidc-auth-20260711/runtime-rehearsal-redacted.json`
- `tmp/product-design/production-admin-oidc-auth-20260711/missing-binding-redacted.json`
- `tmp/product-design/production-admin-oidc-auth-20260711/disabled-user-redacted.json`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-375x812.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-390x844.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-768x1024.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-1024x768.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-1280x720.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-final-login-1440x1024.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-success-overview-390x844.jpg`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-success-overview-1280x720.jpg`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-protected-users-refresh-1280x720.jpg`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-cancel-recovery-focus-390x844.jpg`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-invalid-state-recovery-390x844.jpg`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-missing-binding-recovery-390x844.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-disabled-user-recovery-390x844.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-expired-transaction-recovery-1440x1024.png`
- `tmp/product-design/production-admin-oidc-auth-20260711/screenshots/task8-keyboard-focus-390x844.jpg`

Review result: the Task-level review of `52ab75b` reported no Critical, Important or Minor findings.

final result: passed

## 2026-07-05 Follow-Up QA

Additional screenshots:

- `./.superpowers/brainstorm/28542-1783178508/content/admin-list-column-dropdown-20260705.png`
- `./.superpowers/brainstorm/28542-1783178508/content/admin-advanced-filter-20260705.png`
- `./.superpowers/brainstorm/28542-1783178508/content/admin-settings-drawer-20260705.png`
- `./.superpowers/brainstorm/28542-1783178508/content/admin-sidebar-collapsed-20260705.png`

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

- `<temporary-path>`
- `<temporary-path>`
- `<temporary-path>`
- `<temporary-path>`
- `docs/admin-ui-foundation.md`

Implementation screenshot path:

- `./tmp/product-design/visual-product-design-qa-20260706/03-menus-list-desktop-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/04-user-menu-desktop-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/05-settings-black-layout-tab-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/06-sidebar-collapsed-black-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/07-column-menu-black-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/08-filter-menu-black-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/09-work-tab-context-menu-black-1440x1024.png`
- `./tmp/product-design/visual-product-design-qa-20260706/11-menus-mobile-390x844-after-fix.png`
- `./tmp/product-design/visual-product-design-qa-20260706/12-dashboard-desktop-1440x1024-after-fix.png`

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

- `./tmp/product-design/ui-optimization-audit-20260710/01-login-current.png`
- `./tmp/product-design/ui-optimization-audit-20260710/02-dashboard-desktop-current.png`
- `./tmp/product-design/ui-optimization-audit-20260710/04-menus-desktop-current.png`
- `./tmp/product-design/ui-optimization-audit-20260710/06-menus-mobile-top-current.png`
- `./tmp/product-design/ui-optimization-audit-20260710/07-dashboard-mobile-current.png`

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
- At the time of this audit, the task candidates remained outside the then-closed foundation graph; the 2026-07-11 section below records Candidate A activation and closeout.

Detailed assessment: `docs/platform-ui-optimization-assessment.md`.

## 2026-07-11 Admin UI System Quality Hardening QA

Status: implemented and accepted for the `admin-ui-system-quality-hardening` node.

Fresh evidence:

- `tmp/product-design/p1-admin-ui-hardening-20260711/01-login-1280x720.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/02-dashboard-375x812.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/03-menus-390x844.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/04-menus-768x1024.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/05-menus-1024x768.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/06-menus-1280x720.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/07-menus-1440x1024.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/08-create-modal-1280x720.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/09-settings-drawer-390x844.png`
- `tmp/product-design/p1-admin-ui-hardening-20260711/10-stale-session-390x844.png`

Accepted behavior:

- the 375/390/768 compact shell uses two persistent tiers and keeps page headings within the approved geometry; at mobile resource widths the shell, toolbar actions, search, pagination buttons, quick-jumper and settings Drawer close/tabs/overflow controls measure at least 44x44px;
- the 1024/1280/1440 states preserve desktop shell placement while schema-order responsive tiers keep high-priority menu fields and row actions operable;
- the localized skip link targets the stable main region, route changes move focus to that region, and icon-only navigation/alert controls have explicit localized names;
- after the create modal settles, focus is on the first enabled editable field inside the visible form; Escape closes the modal and returns focus to the “新增记录” trigger;
- `09-settings-drawer-390x844.png` records the accepted narrow settings Drawer state with 44px controls;
- stale stored sessions recover to localized sign-in guidance instead of raw `unauthorized` feedback; `10-stale-session-390x844.png` shows “会话已过期，请重新登录。” as the visible state;
- reduced-motion emulation produces effectively immediate computed transition/animation styles for the platform shell and portaled modal/drawer/dropdown/popover surfaces while retaining focus, loading, validation and status feedback;
- accepted stable states have no page-level horizontal overflow, and focused browser console review found no new application errors.

Automated evidence remains `scripts/validate-admin-i18n.mjs`, `scripts/validate-admin-ui-contracts.mjs`, `scripts/admin-ui-contracts.test.mjs` and the Admin production build.

Evidence limit: this QA package is implementation evidence, not WCAG certification. It does not establish screen-reader behavior, high-zoom reflow or platform-specific assistive-technology conformance.

final result: passed

## 2026-07-11 Task 6 Admin OIDC Login QA

Status: implemented and accepted for the accessible provider-specific Admin login slice.

Baseline evidence:

- `.superpowers/design-audit/task6/01-login-current-390x844.jpg`
- `.superpowers/design-audit/task6/02-login-current-1280x720.jpg`

Baseline findings:

- The desktop hierarchy, spacing, colors, typography, provider selector and operational tone were stable and should be preserved.
- Every provider shared the demo username and disabled-password form.
- The `390x844` first viewport ended before the submit action.
- Callback progress, callback failure focus and recovery states were absent.

Required state definitions:

1. Provider list: keep every Admin-capable provider visible; show localized configured status, disable unavailable providers, and expose a native-button accessible name containing provider title and status.
2. Demo form: render only the username form and submit action when the selected provider kind is `demo`; do not render an irrelevant disabled password field.
3. OIDC action: render exactly one full-width localized action when the selected provider kind is `oidc`; disable repeated provider selection and redirect actions while the start request is pending.
4. Callback progress: replace provider and credential controls with stable `aria-live="polite"` and `aria-busy="true"` status content while the code exchange is pending.
5. Callback failure: keep the callback values removed from browser history, show a localized sanitized message, and focus a `tabIndex=-1` error heading without scrolling.
6. Recovery: expose one full-width localized recovery action that clears stale transaction state and restores the provider list.

Fresh implementation evidence:

- `.superpowers/design-audit/task6/03-login-demo-after-1280x720.png`
- `.superpowers/design-audit/task6/04-login-oidc-after-1280x720.png`
- `.superpowers/design-audit/task6/05-login-oidc-after-390x844.png`
- `.superpowers/design-audit/task6/06-login-demo-after-390x844.png`
- `.superpowers/design-audit/task6/07-login-callback-progress-390x844.png`
- `.superpowers/design-audit/task6/08-login-callback-failure-390x844.png`

Browser and accessibility evidence:

- At `1280x720`, demo and OIDC states had no horizontal overflow; OIDC rendered one `380px`-wide action and no username or password inputs.
- At `390x844`, demo and OIDC primary actions were `44px` high and remained within the first viewport; the demo username control was `44px` high, provider controls were `52px` high, and toolbar controls were `44px` high.
- Callback progress removed provider/form actions, used `aria-live="polite"` plus `aria-busy="true"`, and normalized the URL to `/login` before exchange.
- Callback failure retained `/login`, used `aria-live="polite"`, focused the error heading with `tabIndex=-1`, kept the `44px` recovery action in the first viewport, and restored both providers after recovery.
- Keyboard order followed provider selection, provider action, language and theme controls. Focus-visible styling remained present for the callback error heading.
- Focused Chrome console review found no warning or error entries. Static contracts cover reduced-motion suppression for login transitions.

Evidence limit: this focused QA is not a WCAG certification and does not include a live external identity provider. Production-like OIDC acceptance remains a separate plan task.

final result: passed

## 2026-07-13 Full-Viewport Watermark And Sensitive Reveal QA

Status: implemented and accepted for the `admin-watermark-export-governance` correction and `sensitive-data-reveal-step-up` closeout.

Watermark evidence under `.superpowers/product-design-audit/watermark/` confirms one fixed viewport layer over the topbar, sidebar, dashboard/list data, dropdown, modal and mobile navigation Drawer. The layer matches the viewport, uses `z-index:2200`, remains above Ant Design overlays at `1000`, and keeps `pointer-events:none`. Narrow sixteen-mark mode uses two columns and eight rows with no truncated attribution or horizontal overflow.

Sensitive reveal evidence under `.superpowers/product-design-audit/sensitive-reveal/` covers factor selection, SMS verification, revealed value and the exact `390x844` mobile result. The modal keeps 44px controls, focuses its heading, renders plaintext once, clears it on close/hide/expiry, and does not expose it through the underlying masked resource view.

Automated evidence includes the Admin UI validator and 92 drift tests, Admin TypeScript checking, generated OpenAPI tests, focused Go runtime/config/HTTP/OIDC/bootstrap tests and the final Admin build. Screenshot and DOM evidence support the implemented interaction but do not certify full WCAG conformance.

final result: passed

## 2026-07-15 Organization And User Admin Experience QA

Status: implemented and accepted for `organization-user-admin-experience`.

The tracked browser manifest is `resources/evidence/organization-user-admin-experience-20260715.json`; source screenshots remain local under `.superpowers/product-design-audit/organization-user-admin-experience/2026-07-15/`. Acceptance covered `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x800` and `1440x900`.

Accepted behavior:

- organization list and create flows remain responsive without page-level horizontal overflow;
- organization role-group selection stays disabled until the organization exists;
- a new user starts with no automatically selected organization;
- tenant is derived from the selected organization and rendered read-only;
- role selection remains disabled until the organization role pool loads, and role options preserve role-group provenance;
- asynchronous option loading does not reset values already entered into an open modal;
- Refine third-party telemetry is disabled by default.

The accepted run recorded zero application console errors and zero failed first-party requests. The in-app Browser could not initialize because its module threw `Cannot redefine property: process`, so this evidence uses a current-run local Playwright 1.55 fallback and must not be represented as in-app Browser evidence. The package supports the implemented responsive interaction but is not a WCAG certification; the seeded dataset also did not contain a conflicting existing user assignment for visual conflict-remediation proof.

final result: passed
