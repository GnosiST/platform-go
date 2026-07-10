# Policy Review Custom UI Design

Date: 2026-07-07

## Decision

Add a platform-governance `PolicyReviewConsole` for the optional `policy-review` capability. The console is a custom admin route for `/policy-reviews` when that resource is present, but it remains business-neutral: it manages platform policy-change reviews for roles, deny permissions and data scopes, not downstream business workflows.

## Product Design Brief

- Product surface: enterprise governance policy-review workbench inside the existing admin shell.
- Visual source: current `platform-go` admin design system, `AdminPage`, `AdminListPanel`, `PlatformDataTable`, `AdminFeedback`, Ant Design controls and the approved compact/refined admin UI direction.
- Interactivity: full working list refresh, status filtering, request/approve/reject actions, selected review inspector, diff summary, audit evidence and export JSON.

## UX Model

The screen is a dense operations workspace:

- Top summary metrics: total, pending, approved and rejected review counts.
- Main list: schema-safe policy-review records with status tags, role target, policy type, requested action and updated time.
- Right inspector: selected review details, policy diff, permission/data-scope evidence and audit timeline.
- Action model:
  - draft reviews can be requested;
  - pending reviews can be approved or rejected;
  - export is a read action that downloads a JSON evidence bundle;
  - failed actions render localized `AdminFeedback` without losing selection.

## Engineering Boundary

- Use existing API client and add typed wrappers for policy-review request, approve, reject and export routes.
- Do not add business state machines, business resources, business tables or hard-coded downstream actions.
- Do not replace the generic resource console. This custom console is only for a platform governance workflow that already has dedicated platform routes.
- Keep route availability dynamic: mount the console only when the enabled menu/resource list contains `/policy-reviews`.

## i18n And Accessibility

All visible strings must be dictionary keys in Chinese and English. Destructive or irreversible actions must have confirmation text. Rejection reason input must have an accessible label and validation.

## Verification

Required checks:

- `rtk node scripts/validate-admin-i18n.mjs`
- `rtk node scripts/validate-admin-ui-contracts.mjs`
- `rtk npm --prefix admin run build`
- focused API/client and policy-review tests where affected
- browser screenshots for desktop and narrow viewport before promotion from deferred
