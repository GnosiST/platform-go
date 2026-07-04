# platform-go Admin Shell Layout Design

Date: 2026-07-04

## Purpose

The admin web app must evolve from a single capability list into a reusable management-console foundation. It should look like a mature enterprise platform, support multiple administration layout patterns, support Chinese and English, and allow projects to switch visual themes without duplicating page implementations.

The selected visual target is the third Product Design direction, `Enterprise Foundation Console`: a foundation console with deep navigation, a top resource switcher, dense but breathable tables, capability detail inspection, and plugin-ready optional capability areas.

## Goals

- Keep the existing `/api/capabilities` API integration.
- Provide an `AdminShell` owned by this repository, not an external admin framework.
- Support four layout modes:
  - `side`: classic left navigation plus content.
  - `top`: top navigation with maximum content width.
  - `mixed`: global top region plus left feature navigation; default mode.
  - `split`: multi-level side navigation for deep enterprise systems.
- Support four themes:
  - `tech`: technology console with blue-green accents.
  - `white`: premium white, restrained and office-focused.
  - `black`: cool dark mode for monitoring and operations.
  - `warm`: warm yellow/amber for friendlier business operations.
- Support Chinese and English through a lightweight local dictionary.
- Preserve responsive behavior across desktop, tablet and mobile.
- Keep future system-management resources first-class: tenant, user, role, menu, API resource, dictionary, parameter, audit, capability and settings.

## Non-goals

- No real authentication, tenant persistence or RBAC enforcement in this slice.
- No routing library in this slice; navigation state can remain in local React state.
- No full CRUD pages yet. The capability console is the first implemented management surface.
- No runtime theme marketplace. Themes are local tokens.
- No separate implementation per theme or per layout.

## Architecture

```text
App
  AdminShell
    ThemeProvider-like state
    i18n dictionary state
    LayoutEngine
      side / top / mixed / split
    Navigation
      global groups
      resource switcher
      breadcrumb
      page tabs
    Content slot
  CapabilityConsole
    overview metrics
    capability matrix
    optional plugins
    detail inspector
```

`AdminShell` owns platform-level chrome. Product pages own their domain content. `CapabilityConsole` must not duplicate global shell responsibilities.

## Layout Modes

### side

Use a fixed-width left navigation with a top utility bar and a right content area. This is the standard complex-backoffice layout for CMS, ERP and resource-heavy systems.

### top

Use a top horizontal navigation and maximize content width. This is useful for monitoring consoles, dashboards and modules with shallow hierarchy.

### mixed

Use top-level global controls and a left navigation for detailed resources. This is the default because `platform-go` is a multi-capability foundation that may host multiple subsystems.

### split

Use a first-level side rail plus secondary navigation next to it. This supports deep enterprise systems without forcing long nested menus into one sidebar.

## Responsive Rules

- `>=1280px`: full mixed layout, sidebar visible, top resource switcher visible, detail inspector docked on the right.
- `1024px-1279px`: sidebar can collapse, detail inspector becomes less wide or drawer-like.
- `768px-1023px`: top navigation remains, feature navigation moves to a drawer pattern, detail inspector stacks under the main table.
- `<768px`: single-column layout, compact toolbar, table rows become stacked capability records, sidebar and split navigation move behind a menu button.

All controls must keep stable dimensions. Text must not overflow buttons, tabs, table cells or status chips in either Chinese or English.

## Theme System

Themes share the same DOM and component tree. Each theme maps semantic tokens:

- background, surface, elevated surface;
- text, muted text, border;
- primary, success, warning, error;
- sidebar background and sidebar text;
- table row hover, selected row and chip backgrounds.

The implementation should set a root `data-theme` value and CSS custom properties. Ant Design components can be styled through class wrappers and CSS variables in this slice.

## Internationalization

Use a lightweight dictionary for this slice:

```text
Language = zh | en
dictionaries[language].key
```

Every visible global-shell label, navigation item, table heading, status label, button and empty/error state must come from the dictionary or from localized capability metadata derived in code.

The layout should be designed around the longer English labels, not only around Chinese labels.

## Capability Console

The first real page is the capability console.

It should show:

- governance summary metrics;
- install health;
- capability tabs: all, core, plugin, optional, disabled;
- capability matrix grouped by domain;
- selected capability detail panel;
- dependency chain;
- provided API list;
- optional plugin cards.

The console uses real capability IDs from `/api/capabilities` and augments them with front-end presentation metadata for domain, owner, dependency and description until the backend exposes richer metadata.

## Interaction Requirements

- Theme switching updates the visible theme immediately.
- Language switching updates shell, page and table labels immediately.
- Layout mode switching demonstrates side, top, mixed and split patterns.
- Capability row selection updates the detail panel.
- Search filters capabilities by name, code, domain or description.
- Capability tabs filter the list.
- Optional plugin install buttons are present but can be non-persistent in this slice.

## Verification

Required before handoff:

- `rtk go test ./...`
- `rtk npm --prefix admin run typecheck`
- `rtk npm --prefix admin run build`
- `rtk npm --prefix admin audit`
- browser screenshot at desktop `1440x1024`
- browser smoke for theme switching and language switching
- browser screenshot at a narrower responsive viewport
- `design-qa.md` with final result `passed` or a clear blocker

## Acceptance

The work is acceptable when the admin preview visibly resembles the selected enterprise foundation direction, all four themes are switchable, Chinese/English labels are switchable, the shell supports the four layout modes, the capability console still uses real API data, and verification passes.
