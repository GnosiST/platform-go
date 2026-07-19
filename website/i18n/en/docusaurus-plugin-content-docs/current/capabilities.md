---
title: Capabilities
---

# Capabilities

Capabilities are declared through versioned manifests. A manifest owns its resources, routes, permissions, lifecycle hooks and optional UI registration without coupling business packages to the Admin shell.

The default foundation includes identity and tenancy, organization units, roles, permissions, menus, audit, sessions, file storage and production readiness contracts. Optional capabilities are enabled through explicit profiles and remain disabled by default.

## Install, Disable And Uninstall Boundary

The platform is not a runtime hot-plug marketplace. Installing a capability means declaring desired state before startup through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root. Disabling means removing it from the desired set, regenerating contracts and manually restarting. A mismatch between the running process and desired set is only `pendingRestart`, not a hot-apply path. After restart, Admin resources, App routes, auth providers and demo data sets must disappear from the exposed surface. `dictionary`, `tenant`, `identity`, `session`, `rbac`, `menu` and `admin-shell` are non-removable foundation capabilities. Destructive uninstall or persisted data purge needs reviewed migration and rollback evidence.

## Plugin management v1

Plugin management v1 uses a restart-required desired-state model. Declare the desired capability set through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root, run contract gates, regenerate contracts and restart manually. Client notices use HTTP polling, `version.json` or an API version check; v1 does not use WebSocket for hot updates. It also does not support runtime hot install/uninstall, remote repository pull, source deletion, data purge or one-click destructive uninstall.

## Credential Auth v1

`credential-auth` is the planned local credential authentication capability for username/password, phone/password, email/password and phone/SMS OTP login. The current package has contract, docs, validation and the first internal service foundation: it does not enable the `password` provider kind and does not change the current demo/OIDC/App login runtime. Passwords, OTPs, challenge answers and proofs must not be stored in generic `Record.Values`; the internal package now covers identifier hashes, Argon2id PHC verification, a memory repository and SMS OTP one-time consumption semantics, while HTTP APIs, persistent repositories and login UI wiring remain pending. SMS delivery stays a `notification` SMS channel extension, with an SMS sender port, `mock-local` dev/test sender and production mock-provider rejection already in place.

New business projects should keep concrete business capabilities in a downstream repository or composition root. Only cross-domain reusable behavior should become a platform profile.

```bash
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-credential-auth-v1.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
```
