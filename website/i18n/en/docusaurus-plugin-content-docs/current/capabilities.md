---
title: Capabilities
---

# Capabilities

Capabilities are declared through versioned manifests. A manifest owns its resources, routes, permissions, lifecycle hooks and optional UI registration without coupling business packages to the Admin shell.

The default foundation includes identity and tenancy, organization units, roles, permissions, menus, audit, sessions, file storage and production readiness contracts. Optional capabilities are enabled through explicit profiles and remain disabled by default.

## Install, Disable And Uninstall Boundary

The platform is not a runtime hot-plug marketplace. Installing a capability means enabling a registered manifest before startup through a profile, `PLATFORM_CAPABILITIES` or a downstream composition root. Disabling means removing it from the enabled set, regenerating contracts and restarting; resources, menus, providers, routes and demo data must disappear from the exposed surface. `dictionary`, `tenant`, `identity`, `session`, `rbac`, `menu` and `admin-shell` are non-removable foundation capabilities. Destructive uninstall or persisted data purge needs reviewed migration and rollback evidence.

```bash
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-capability-profiles.mjs
node scripts/validate-platform-capability-operation-policy.mjs
```
