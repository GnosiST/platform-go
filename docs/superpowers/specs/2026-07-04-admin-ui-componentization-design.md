# Admin UI Componentization Design

Date: 2026-07-04

## Purpose

The admin UI should move from page-local Ant Design composition to reusable platform components. The goal is a unified visual and interaction standard that remains flexible for business resources through props and slots.

This design is approved for implementation, but it must be implemented on top of the corrected platform stack direction: Refine + React + Ant Design for the admin frontend.

## Visual Direction

- The resource page header should be compact. It should not consume a large first-screen band.
- The list toolbar, search, refresh, create button, batch actions and pagination should feel like one data-work surface.
- Tables should avoid overlapping text through fixed layout rules, truncation, width constraints and detail affordances.
- Pagination should default to compact 24px-class controls and 36px-class footer density, keep page buttons on the visual center line, and keep range/page-size/jumper controls close to the pager instead of pinning them to table edges.
- System configuration should open as a right-side drawer from the avatar/settings trigger, not as a large dropdown.
- Drawer colors, selected states, buttons, tabs and table highlights must follow shared system theme tokens.

## Components

### ResourcePageHeader

`ResourcePageHeader` owns the compact page header for resource pages.

Interface:

- `title`
- `description`
- `search`
- `actions`
- `extra`

Rules:

- title and short description stay on the left;
- search and frequent actions live in the same row when width allows;
- on narrow screens the row stacks without overlapping;
- explanatory copy is optional and should stay short.

### PlatformDataTable

`PlatformDataTable` is the standard table surface built on Ant Design Table and compatible with Refine table state.

Default capabilities:

- column sorting;
- column visibility;
- column visibility menu with visible count, select-all and reset actions;
- column width and ellipsis defaults;
- hover detail for clipped cell values;
- cross-page row selection;
- batch action area;
- inline status switch support;
- structured multi-condition filters and date range filters;
- row actions;
- compact integrated pagination footer with centered page buttons;
- loading, empty and error states;
- mobile card fallback.

Extension points:

- `toolbarExtra`
- `batchActions`
- `rowActions`
- `expandedRow`
- `inlineEditor`
- `detailDrawer`
- `emptyState`

The default behavior should be useful without custom slots. Business pages can add slots without forking the common component.

### SystemSettingsDrawer

`SystemSettingsDrawer` opens from the avatar/settings trigger.

Tabs:

- `Appearance`: theme mode, theme color, custom color and current color.
- `Layout`: side, top, mixed and split layout modes; page tabs; transition setting; sidebar width; menu item height.
- `General`: system information, reset config, export config, import config and about project.

Rules:

- theme color changes update shared tokens immediately;
- layout changes update the shell immediately;
- settings remain grouped in a drawer configuration center with visual layout legend, watermark, visual-aid and page-transition controls;
- reset restores defaults after confirmation;
- export outputs the current UI configuration as JSON;
- import validates JSON before applying.

### AdminThemeProvider

`AdminThemeProvider` owns UI configuration state and emits:

- Ant Design theme tokens;
- CSS custom properties;
- persisted local config;
- reset, import and export functions.

State:

- `themeName`
- `primaryColor`
- `layoutMode`
- `density`
- `showWorkTabs`
- `pageTransition`
- `sidebarExpandedWidth`
- `sidebarCollapsedWidth`
- `menuItemHeight`

## Data Flow

```text
SystemSettingsDrawer
  -> AdminThemeProvider state
  -> Ant Design ConfigProvider tokens
  -> CSS variables on platform shell
  -> AdminShell, PlatformDataTable, ResourcePageHeader and buttons
```

Resource tables should consume schema and Refine table state:

```text
backend resource contract
  -> Refine resource
  -> useTable / dataProvider
  -> PlatformDataTable
```

Query input is intentionally not raw SQL. SQL-like text is parsed into whitelisted field conditions such as `name:admin`, `status=enabled` and `updatedAt>=2026-01-01`; future server-side search should carry a structured DSL or JSON condition model instead of SQL fragments.

## Accessibility And Responsive Rules

- All icon-only buttons need labels or tooltips.
- The drawer must trap focus and close with Escape.
- Table row actions must remain keyboard reachable.
- Text must not overlap fixed columns.
- Mobile widths should switch table rows to resource cards.
- No viewport-scaled typography.
- Pagination, dropdown menus and drawers must keep solid token-based backgrounds in every theme.
- Component copy must be added to the shared dictionary when the component is created; i18n is a hard gate, not a later cleanup task.

## Testing

Frontend verification:

- TypeScript check.
- Production build.
- Browser smoke for resource table on desktop and mobile widths.
- Visual QA against the approved screenshots and the generated design companion mock.

Core behaviors to test:

- toggling visible columns changes rendered columns;
- sorting updates table state;
- selecting rows across pages preserves selected keys;
- batch actions receive selected keys;
- theme changes update table/header/drawer/button color in one place;
- import rejects invalid config JSON.
- i18n validation blocks visible component copy outside the dictionary or localized data contracts.

## Out Of Scope

- Source-writing code generator.
- Full form generator with arbitrary custom slots.
- Business-specific workflows.
- Backend persistence for UI preferences. Local persistence is enough for the first slice.
