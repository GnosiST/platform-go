# Admin UI Foundation

Date: 2026-07-05

## Positioning

The admin UI uses Ant Design v5 as the base UI engine and `admin/src/platform/ui` as the platform design layer.

Ant Design provides mature primitives such as `Table`, `Modal`, `Alert`, `Button`, `Form`, `Select`, `Segmented` and `Dropdown`. Product and business pages should not rebuild these primitives.

The platform layer provides reusable backoffice semantics, default density, theme tokens and extension slots. Business pages can customize by props, `children`, `actions`, `toolbar`, `summary`, `extra`, `footer` and class names. If a prop is not provided, the component keeps the platform default.

The admin entry now starts with `AdminLoginView`. It consumes the same theme tokens, i18n dictionary and auth provider API as the rest of the shell. The login screen is intentionally provider-driven: demo keeps the username flow, while the Admin-only `oidc` provider uses a configured provider action, tab-scoped state and PKCE transaction, sanitized callback recovery and the existing bound platform principal/session path. Adding another login mode must add an auth provider capability and adapter rather than hard-code business login logic into the shell.

The design rule is unified defaults with flexible adaptation. Shared components own typography, spacing, color, responsive behavior, i18n, accessibility labels and common states. Business modules may override through documented props, slots and schema fields, but should not fork the base visual rules for tables, buttons, drawers, modals, dropdowns, pagination, prompts or alerts.

Follow-on UI optimization is tracked in `docs/platform-ui-optimization-assessment.md`. Implementation-level admin improvements use `ui-ux-pro-max` as a review aid after production persistence correctness: focus visibility, keyboard flow, explicit icon labels, mobile hit areas, mobile shell density, table prioritization, reduced motion and localized recovery feedback are platform concerns. `design-taste-frontend` is reserved for login/brand entry and future public landing, portfolio or marketing surfaces; it must not make repeated operational workflows more promotional or card-heavy.

## Layering

```text
AdminLoginView
  uses auth provider discovery and platform theme/i18n tokens
Business page / plugin page
  uses platform UI components and supplies slots
Platform UI layer
  AdminDesignProvider, AdminPage, AdminMetricStrip, AdminListPanel,
  AdminActionButton, AdminFeedback, AdminFormModal, PlatformDataTable,
  PlatformDropdownPanel, PlatformDropdownPlugin, PlatformPaginationBar, PlatformOverflowText,
  SystemSettingsDrawer
Ant Design v5
  Table, Button, Modal, Alert, Form, Select, Dropdown...
CSS tokens
  theme, typography, spacing, radius, density, responsive behavior
```

## Encapsulation Principle

The platform layer is a thin semantic layer over Ant Design. It should not wrap every AntD component.

Wrap a component when at least one of these is true:

- it appears frequently across admin pages;
- it needs platform-wide density, spacing, accessibility or i18n defaults;
- it needs shared states such as loading, empty, error, disabled, permission or destructive confirmation;
- it defines a backoffice pattern, such as page shell, table/list panel, CRUD dialog, action button, feedback tip or detail inspector;
- changing it later should affect all business modules consistently.

Do not wrap a component when:

- it is used once in one page;
- AntD's API is already the exact business API needed;
- wrapping would hide important AntD capability;
- the wrapper only renames props without adding platform rules;
- the component is a domain-specific widget that belongs to a business package.

## Rules

- All admin pages must be rendered under `AdminDesignProvider`. It maps platform theme tokens into AntD `ConfigProvider`.
- All shared components must source visible text from `admin/src/platform/i18n.ts` or localized data contracts. New component text in `.tsx` files under `admin/src` is blocked by `rtk node scripts/validate-admin-i18n.mjs`, except approved localized data files such as resource registries, capability metadata and dashboard sample data. `rtk npm --prefix admin run build` runs that gate before TypeScript and Vite. The validator also requires the Chinese and English dictionaries to expose the same keys.
- i18n is a hard gate for every new shared component. When adding a component, add Chinese and English dictionary keys, localized sample data if needed, and run the i18n validator in the same change.
- Admin preferences are platform concerns. Language, theme, layout mode and UI configuration are persisted in browser storage by the app shell; branding default theme is used only when the user has not chosen a theme.
- Login screens must use provider declarations from `GET /api/auth/providers`; do not couple the shell to one concrete login provider.
- Admin login must filter provider audiences to `admin`. OIDC must never render irrelevant username/password controls, auto-provision users or authorization relationships, expose callback material, or reuse the App identity resolver boundary.
- Logout must clear the platform token and return to the login view.
- Prefer `AdminPage` for page structure instead of hand-writing `page-heading`.
- Prefer `AdminListPanel` for table or list containers instead of hand-writing toolbar panels.
- Prefer `AdminActionButton` for frequent command buttons so labels, tooltips, icons and accessibility stay consistent.
- Prefer `AdminFeedback` for warning/error/success tips so feedback density and visual treatment stay consistent.
- Prefer `AdminFormModal` for CRUD dialogs so width, spacing and lifecycle defaults stay consistent.
- Prefer `PlatformOverflowText` for table/list text that may be clipped; long values should reveal their full content on hover. `PlatformDataTable` applies this default to raw string, number and boolean cells when a column does not provide its own renderer.
- Prefer `PlatformDropdownPlugin` plus `PlatformDropdownPanel` for dropdown plugins such as column settings, filter panels, context selectors and small command menus. The plugin owns trigger/container/overlay behavior; the panel owns solid background, spacing, max-height and header/body/footer slots.
- Prefer `PlatformPaginationBar` through `PlatformDataTable` for list pagination. Do not place ad hoc pagination outside the table panel; use the integrated operation bar so total count, page size, page numbers and page jump stay visually grouped in one compact table footer. Page buttons must remain horizontally centered even when total, page-size and jump controls are visible.
- Prefer `PlatformDataTable` extension slots before forking table columns or panels. `toolbarExtra`, `batchActions`, `rowActions`, `expandedRow`, `inlineEditor`, `detailDrawer`, `emptyState` and `mobileCards` provide caller-owned behavior while the table keeps platform density, sorting, filters, pagination, dropdowns, overflow handling and accessibility defaults. If a slot is not provided, default table behavior remains unchanged.
- Prefer `PlatformTreeSelect` for schema-declared tree relations. Generic resource forms and filters should switch to it from `relation.display`, not from resource-name branches.
- Shared admin shell, list and generic resource-form behavior must remain covered by `rtk node scripts/validate-admin-ui-contracts.mjs`, including settings drawer controls, sidebar collapse, work tabs, multi-level menu paths, table plugins, pagination density tokens, relation option loading through the Refine data provider, tree relation controls and AntD Form control-prop passthrough. `rtk node --test scripts/admin-ui-contracts.test.mjs` must stay in the task graph, goal audit and objective conformance evidence so the UI contract validator itself has drift coverage.
- Global context controls such as environment and tenant belong near work tabs because they affect every page. They should remain lighter than resource-level filters and use dropdown panels for explanation or future selection.
- Use raw AntD components inside slots when the business page needs field-level or layout-specific control.
- Do not hard-code colors, radii, major spacing or typography in business pages. Add or use CSS tokens in `admin/src/styles.css`.
- Keep page-specific class names for domain layout only, not for redefining base button, modal, alert or table style.
- Navigation entries should come from `GET /api/admin/menus`; do not hard-code business menus in the shell.
- List pages should default to dense, full-width tables with pagination. Avoid always-on side inspectors unless the task requires continuous comparison.
- Details should open through row actions in `Drawer`; edit forms should use `AdminFormModal` and must hydrate from the selected record.
- Resource schemas may declare metadata-only `actions` and `panels`. Row and batch actions must carry localized labels and permission codes; dangerous actions require confirmation metadata. Drawer panels render as tabs and may use semantic kinds such as detail fields, permissions, audit, approval, files and custom extension. Component values are semantic keys, not React import paths or scripts.
- Binary enable/disable states should use `Switch`; non-binary runtime states such as `healthy` or `recorded` should remain tags.
- Resource, plugin and dashboard sample data should use `LocalizedText`. Business data that needs translation should reserve localized fields or translation tables instead of hard-coded UI strings.
- Platform-owned data entry resources should use localized fields for names and descriptions by default. Business resources may opt in with localized columns or translation tables when the business needs multi-language content.
- Generic resource rows may provide `values.nameZh`, `values.nameEn`, `values.descriptionZh` and `values.descriptionEn`; the shared console prefers those fields for display, search, filters and detail views.
- SQL-like search must stay a structured DSL or structured JSON conditions. `GenericResourceConsole` compiles UI input into `keywords` and `conditions`, passes them through Refine `useList` meta, and the Refine `dataProvider` calls the backend query endpoint with pagination and sorting. Do not pass raw query text to database SQL builders. `scripts/validate-platform-admin-api-boundary.mjs` guards this calling style so pages cannot add direct `fetch`, App API targets or query-string collection filters outside the platform API client.
- Environment and tenant are global context indicators, not resource filters. `production` means the currently active runtime environment, and `platform` means the platform-level tenant used to administer foundation resources. Keep these controls close to work tabs until real switching is implemented.

## Current Components

### AdminDesignProvider

AntD `ConfigProvider` wrapper. It owns:

- locale mapping;
- light/dark algorithm selection;
- semantic color mapping from platform themes;
- base radius, font, control height and table density defaults.

### AdminPage

Standard page shell with compact heading and slots:

- `title`
- `description`
- `actions`
- `summary`
- `eyebrow`
- `extra`
- `children`

### AdminMetricStrip

Summary metric strip with configurable columns and semantic tones:

- `default`
- `accent`
- `warning`
- `danger`

### AdminListPanel

Reusable table/list panel with toolbar slots:

- `title`
- `toolbar`
- `actions`
- `footer`
- `children`

### AdminActionButton

Button wrapper with required accessible `label`, optional `tooltip`, and all normal AntD button props.

### AdminFeedback

Alert wrapper for compact platform feedback.

### AdminFormModal

Modal wrapper for CRUD forms. Defaults to platform width, `destroyOnHidden`, viewport-safe top spacing and internal body scrolling so dense resource forms do not push the footer out of view. Schema-driven resource forms can render lightweight sections from `formGroups`, field helper text from `help`, and AntD validation rules from field `validation` metadata.

### PlatformResourceForm

Schema-driven form wrapper over AntD `Form`. It owns the common section structure, label/helper/rules wiring, default single-column layout and controlled source-level React slots:

- `header` and `footer`;
- `sectionBefore` and `sectionAfter`;
- `fieldControl` for replacing a rendered control from application-owned React code.

Use it before creating page-local CRUD forms. Callers provide localized labels, rules and field controls; the component keeps defaults useful when slots are omitted.

Form layout and slot promotion is guarded by `resources/platform-form-schema-layout-slots.json`. Current shared forms support single-column, grouped-section, schema-driven `two-column-density` and `side-detail-preview` layouts, controlled source-level slots, controlled runtime slot descriptors, AntD Form control-prop passthrough and Refine CRUD hooks. Desktop and mobile browser evidence covers dense edit forms, internal modal scrolling, single-column mobile fallback, the desktop side-preview rail and the mobile collapsed preview stack. Do not pass backend manifest React component names, runtime component paths or raw scripts into shared forms. Backend schemas may declare descriptor data only; registered React renderers live in `admin/src/platform/ui/formSlotRegistry.tsx` or downstream capability registries composed outside platform core.

File storage admin experience is implemented and guarded by `resources/platform-file-storage-experience.json`. The `files` route stays a generic resource console extension: table/list behavior remains in `PlatformDataTable`, row actions open theme-synced drawer tabs for metadata, preview and audit, and upload/download/delete controls use platform action wrappers with localized labels. The audit tab uses Refine's data provider only when `/audit-logs` is exposed by the current resource list; otherwise it shows the declared file events as a localized fallback instead of issuing invalid optional-resource requests. Do not build a standalone file manager page or change `storage.ObjectStore` for UI promotion. Unsupported preview, object-not-found, permission-denied and failed download/delete states must use `AdminFeedback`.

Policy-review custom UI is an implemented platform-governance console mounted only when `/policy-reviews` is enabled. `PolicyReviewConsole` uses `AdminPage`, `AdminMetricStrip`, `AdminListPanel`, `PlatformDataTable`, `AdminFeedback` and shared API-client wrappers for request, approve, reject and export actions. It queries `audit-logs` with schema-declared fields so safe resource querying remains authoritative. It is not a template for business workflow pages; standard CRUD resources should still use the generic resource console, and downstream business approval panels must stay in their owning capability.

### PlatformOverflowText

Small reusable list-cell helper for ellipsis plus hover detail. Use it for long names, codes, descriptions and generated values instead of page-local tooltip wrappers.

### PlatformDataTable

Standard data-work surface over AntD `Table`. It owns:

- table toolbar composition;
- toolbar extension slot for page-specific controls;
- search syntax hint;
- schema-driven advanced filters;
- date range filters for schema datetime fields;
- numeric range filters for schema number fields;
- active filter count display in the filter dropdown;
- column visibility dropdown;
- column visibility dropdown with visible count, select-all and reset;
- column sorting through Ant Design sorters;
- controlled server-side pagination, sorting and filter callbacks for generic resources;
- local pagination for static datasets when no server-side `onTableChange` handler is supplied;
- cross-page row selection;
- batch action slot;
- row action slot with platform-owned alignment and width controls;
- optional expanded-row, inline-editor, detail-drawer and custom empty-state slots;
- integrated pagination operation bar with page size, page jump and total display;
- overflow hover detail through `PlatformOverflowText`;
- mobile card slot;
- visual grouping for pagination so it does not sit flush against the panel edge;
- content-first mobile ordering so cards render before pagination when the table is hidden;
- consistent dropdown plugin surfaces for filters, column settings and future table tools;
- default overflow handling for raw cells that do not supply custom renderers;
- platform density tokens for 32px-class table headers and rows, compact cell padding and 24px-class pagination controls. Multi-line resource identity cells may grow naturally instead of clipping content.

Callers provide columns, data, filter fields, actions and i18n labels. The table keeps default behavior when optional slots are omitted. Standard resource CRUD pages should attach row commands through `rowActions` instead of adding a page-local fixed operation column; this keeps action spacing and future menu behavior centralized.

Custom list pages should still start from `PlatformDataTable` unless they have a clear non-table workflow. `APIDocsPage` uses the same table component for OpenAPI operations, so API docs, generic resources and future business lists share pagination, column settings, search hints, filters and overflow behavior.

Generic resource pages must prefer the backend `POST /api/admin/resources/:resource/query` endpoint. Page-local in-memory filtering is acceptable only for static local datasets such as generated docs or small capability previews.

Schema-declared resource actions and drawer panels are supported in the generic console. Default view, edit, delete, status switch, create and batch delete stay platform-owned. Additional row actions render through a compact overflow menu, batch actions render inside the selection command bar, and declared drawer panels render as tabs beside details and permission codes. Routed actions execute through the shared admin API executor and declared confirmation metadata; unsupported actions and panels still display localized safe unavailable states until the owning capability provides a route or data slot. Business workflows must provide handlers through external capability-owned routes; platform UI must not import business packages or ship product-specific business actions.

### PlatformPaginationBar

Reusable pagination operation bar used by `PlatformDataTable`. It owns:

- total/range display;
- page-size selector;
- page buttons;
- quick jump input;
- responsive wrapping;
- theme-synced block styling;
- a three-zone desktop grid where total/page size sit before the centered page buttons and jump controls sit after them;
- stacked responsive layout on narrow screens: page buttons stay centered on the first row, while total/page-size and jump controls sit below without pushing the pager off-center;
- compact table-footer density so controls do not look larger than the list rows;
- 24px-class pager control tokens and 36px-class footer height by default, so pagination reads as a precise operation footer rather than a separate page block without becoming too small to scan or click;
- a stable center axis for page buttons, with side controls adapting around it instead of pushing the pager off-center.

Callers pass labels from the dictionary and a normal AntD `TablePaginationConfig`. Server-side lists receive pagination events through `onTableChange`; static local lists are sliced inside `PlatformDataTable`.

### PlatformDropdownPanel

Reusable solid dropdown surface for column settings, filters and future lightweight plugin menus. Use it instead of transparent ad hoc dropdown content. It exports `PlatformDropdownPanelProps` and supports default platform spacing plus header, body, footer, width and max-height slots so dropdown plugins can stay visually consistent without losing flexibility.

### PlatformDropdownPlugin

Reusable dropdown trigger wrapper over Ant Design `Dropdown`. It keeps overlays mounted inside the current admin shell, applies the platform overlay class and accepts caller-supplied content. Use it with `PlatformDropdownPanel` unless a standard AntD `menu` dropdown is required, such as the work-tab right-click menu.

### SystemSettingsDrawer

Account and system settings drawer opened from the avatar trigger. It owns:

- appearance theme and custom primary preview;
- layout mode, density, work tabs, page transition and sidebar collapse through visual setting cards;
- layout legend cards for side, top, mixed and split layouts;
- watermark and visual-aid switches;
- config import, export and reset actions;
- persistence through the app shell, so language, theme, layout and UI preferences survive page reloads.

### DashboardHome

Default `/overview` page. It is a reusable platform dashboard with role-aware metrics, a theme-synced trend chart, quick actions, announcements, docs and plugin/update tables. Projects can replace its data source later without changing the shell contract.

## Shell Interaction Rules

`AdminShell` is owned by this repository. It is not an external open-source framework.

The shell supports:

- fixed default home tab at `/overview`;
- browser-like work tabs that are opened on navigation and can be closed;
- right-click tab menu for closing current, other, all, left and right tabs;
- theme and layout settings inside the account/settings dropdown;
- manual sidebar collapse in the sidebar brand area for desktop side/mixed/split layouts;
- collapsed side/mixed navigation renders a flat icon list from the same resource contract so hidden multi-level `<details>` branches cannot overlap or block clicks;
- icon-only language switch;
- side, top, mixed and split layout modes;
- multi-level navigation through slash-separated menu `parent` paths.
- top and mixed layout modes render a horizontal resource navigation row; work tabs remain browser-like task history instead of replacing primary navigation.

### System Quality Hardening Contract

`admin-ui-system-quality-hardening` defines the shared responsive, keyboard, session-recovery and motion behavior. These are platform contracts, not page-local exceptions.

Responsive shell and data rules:

- `0-767px`: use the compact two-tier command/context shell and the existing mobile resource cards;
- `768-1023px`: keep the compact shell and render the priority-reduced table;
- `1024px+`: preserve the desktop shell, side/top navigation, work tabs and separate environment/tenant controls;
- compact-shell interactive controls must remain at least 44x44px and must not create page-level horizontal overflow;
- at `0-767px`, resource toolbar buttons, search input, pagination controls, the quick-jumper input, settings Drawer close control, tabs, tab buttons and overflow actions must also remain at least 44x44px;
- generic table priority follows visible schema field order: the first four fields are `essential`, the next three are `standard`, and remaining fields are `extended`; selection and row actions remain available at every table breakpoint;
- full values remain discoverable through row detail and column settings rather than hard-coded business column exceptions.

Keyboard and focus rules:

- render the localized skip link before navigation and keep `#platform-main-content` as the stable native `main` focus target;
- move focus to the main region after an actual route change, not on the first authenticated render or when the route is unchanged;
- provide a default 2px `:focus-visible` platform outline independently of the visual-aid preference;
- icon-only shell controls require explicit localized names;
- after the generic resource create/edit modal settles, focus the first enabled editable field inside the currently visible resource form; Ant Design continues to own focus trapping, Escape handling and restoration to the create/edit trigger.

Session and motion rules:

- shared API paths normalize failures as typed `AdminAPIError` values with HTTP status information;
- a 401 with a stored admin token clears that token once, emits the platform session-expired event, clears authenticated workspace state and shows localized sign-in recovery instead of raw backend `unauthorized` copy;
- provider discovery and unauthenticated login failures do not emit session-expired recovery when no stored token exists;
- `prefers-reduced-motion: reduce` overrides non-essential page-entry, transform and opacity motion, including Ant Design modal/drawer/dropdown/popover portals, while preserving loading, focus, validation and status feedback. Browser-computed styles must resolve those transitions and animations to effectively immediate values.

Browser evidence is stored under `tmp/product-design/p1-admin-ui-hardening-20260711/`. It covers login, compact dashboard, menu list breakpoints, settled create-modal focus plus Escape/trigger restoration, narrow settings drawer, computed reduced-motion styles and localized stale-session recovery at 375x812, 390x844, 768x1024, 1024x768, 1280x720 and 1440x1024. The accepted stable states have no page-level horizontal overflow and no new application console errors. The stale-session state is captured in `10-stale-session-390x844.png` and shows “会话已过期，请重新登录。” rather than raw backend error copy.

The OIDC login implementation is closed as `production-admin-oidc-auth` after automated UI checks, a redacted production-like Keycloak rehearsal, fresh browser acceptance at the same six viewports and neat-freak cleanup evidence. The final checks cover success, cancellation, invalid and expired transactions, missing binding, disabled user, retry, logout, refresh, protected navigation, keyboard focus, live announcements, recovery focus, reduced motion, touch targets, zero page overflow and zero new console warnings or errors. This evidence closes the reusable UI/runtime node; it does not certify WCAG or approve external production promotion.

This evidence validates the implemented contract but is not WCAG certification. Screen-reader announcements, high zoom/reflow and platform-specific assistive technology remain separate acceptance work when a deployment requires those claims.

Page rendering under the shell is routed through `PlatformRoutePages` in `admin/src/App.tsx`. Custom platform pages such as overview, capabilities, demo data and API docs are explicit route elements. Backend-declared internal menu resources become route elements that render `ResourceRoutePage` in `admin/src/platform/refine/ResourceRoutePage.tsx`. That route page reads Refine resource metadata with `useResourceParams`, guards read access with `useCan`, and then delegates the schema-driven CRUD surface to `GenericResourceConsole`. `GenericResourceConsole` owns the platform schema UI, but list/create/update/delete now flow through Refine `useList`, `useCreate`, `useUpdate` and `useDelete` instead of direct API calls. This keeps the visible shell stable while moving route ownership, access control and CRUD flow toward React Router and Refine resource semantics.

`rtk npm --prefix admin run build` runs `scripts/validate-admin-refine-runtime.mjs`, `scripts/validate-admin-refine-crud.mjs` and `scripts/validate-admin-ui-contracts.mjs` after i18n validation. Those gates prevent schema-driven resource routes from drifting back into local `App.tsx` adapters, direct resource API calls, relation-field option shortcuts, form-control passthrough regressions, or shell/list regressions that remove required platform UI primitives. `scripts/admin-ui-contracts.test.mjs` exercises the validator against temporary UI source copies, so dropping shared pagination or settings-drawer configuration support is caught as a validator regression instead of relying only on the live source tree.

Menu metadata:

- `parent`: slash-separated navigation parent path, for example `access` or `support/workbench`;
- `isExternal`: opens the route in a new browser tab and does not create a work tab;
- `cacheEnabled`: declares whether the frontend may keep the page/component state when route caching is introduced.
- internal menu paths must start with `/`; external menu paths must start with `http://` or `https://`.

The work-tab bar is not a second always-on navigation menu. It is a task history surface. Primary navigation remains the side, top or split navigation depending on layout mode.

The environment and tenant controls in the workbar are global context indicators. `production` is the active runtime environment placeholder, and `platform` is the platform-level tenant. Keep them visually lighter than resource filters until multi-environment or multi-tenant switching is fully wired. These dropdowns must behave as a single-open group so opening one context panel closes the other.

Default dashboard:

- `/overview` is the fixed home tab and common dashboard.
- Its content should be role-aware over time: platform admins see capability, permission and system signals; business operators can replace or extend cards through a project dashboard data source.
- The first slice may use realistic platform sample data, but strings still go through i18n or localized data contracts.

## Extensibility

Plugins should consume the platform layer first. If a plugin needs a special case, pass custom content through slots. If multiple plugins repeat the same special case, promote it into `admin/src/platform/ui` as a new platform component.

The expected extension path is:

1. Use `AdminPage` and `AdminListPanel` as the outer structure.
2. Use AntD primitives inside slots for field-level content.
3. Add a page-local class only for domain layout.
4. Promote repeated page-local patterns into `admin/src/platform/ui`.
5. Keep AntD imports allowed, but avoid direct ad hoc styling of AntD primitives in business pages.
