# Admin Productization UI Audit

## Scope

This audit covers the admin productization surfaces that were raised during the v1 closeout:

- Login page multi-provider presentation.
- Topbar avatar, profile, settings and logout entry points.
- Capability management list and detail modal.
- Role, menu and permission tree-governance pages.
- Settings center configuration modal and message-center configuration entry.

The audit intentionally avoids large deferred work items and does not modify the mainline files that are under active implementation.

## Confirmed

- Login provider selection is driven by provider discovery and filters to providers that are both `enabled` and `configured`; unconfigured credential providers are not shown as login options.
- Login layout supports one, two, three and many provider counts through explicit selector classes, and the credential challenge field is rendered for password and SMS OTP credential flows.
- Avatar and settings are split in the topbar. Settings opens interface preferences only; profile opens on click, closes on outside click or Escape, and contains the logout action.
- The profile dropdown is guarded by Admin UI contract tests for click-only trigger, no username in the trigger, role-name display, outside-click close, scrollable body and viewport-safe height.
- Capability management has one combined catalog table with status filters and opens details in an `AdminModal`; optional and disabled capabilities are projected into the same list.
- Capability detail includes install impact surfaces such as menu routes, admin resources, configuration resources, permissions, service operations and auth providers.
- Roles, menus and permissions route through dedicated governance consoles instead of the generic resource console.
- Role, menu and permission pages use the shared `AdminTreeWorkbench` tree layout.
- Permission management supports controlled custom API permission create/edit. Seeded/system permissions are intentionally locked from editing.
- Settings center projects base and manifest-backed capability configuration resources, including credential auth and notification resources.
- Message center exposes channels for SMS, email and WeChat, includes runtime loop status, dry-run/test-send entry, manual delivery run, retry and delivery detail/log panels.
- Shared modal wrappers exist through `AdminModal` and `AdminFormModal`, and current contract tests check modal/focus/accessibility behavior.
- Authenticated browser verification completed on `2026-07-22`: `/overview` emitted no warning or error logs, and `/settings` completed configuration validation, an unsupported connection preflight with explicit feedback, and a same-value save with runtime reload.

## Remaining Gaps

- Visual acceptance for the login page should cover the realistic maximum of three enabled login methods at desktop and mobile widths.
- Message center external delivery remains productized as configuration and dry-run unless real SMS/email/WeChat provider credentials are supplied.

## Verification Run

```bash
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-i18n.mjs
```
