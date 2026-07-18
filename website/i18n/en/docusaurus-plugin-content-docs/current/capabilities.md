---
title: Capabilities
---

# Capabilities

Capabilities are declared through versioned manifests. A manifest owns its resources, routes, permissions, lifecycle hooks and optional UI registration without coupling business packages to the Admin shell.

The default foundation includes identity and tenancy, organization units, roles, permissions, menus, audit, sessions, file storage and production readiness contracts. Optional capabilities are enabled through explicit profiles and remain disabled by default.

## Install, Disable And Uninstall Boundary

The platform is not a runtime hot-plug marketplace. Installing a capability means enabling a registered manifest before startup through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root. Disabling means removing it from the enabled set, regenerating contracts and restarting; resources, menus, providers, routes and demo data must disappear from the exposed surface. `dictionary`, `tenant`, `identity`, `session`, `rbac`, `menu` and `admin-shell` are non-removable foundation capabilities. Destructive uninstall or persisted data purge needs reviewed migration and rollback evidence.

## Plugin management v1

Plugin management v1 uses a restart-required desired-state model. Declare the desired capability set through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root, run contract gates, regenerate contracts and restart manually. Client notices use HTTP polling, `version.json` or an API version check; v1 does not use WebSocket for hot updates. It also does not support runtime hot install/uninstall, remote repository pull, source deletion, data purge or one-click destructive uninstall.

New business projects should keep concrete business capabilities in a downstream repository or composition root. Only cross-domain reusable behavior should become a platform profile.

```bash
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
```
