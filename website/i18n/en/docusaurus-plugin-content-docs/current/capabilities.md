---
sidebar_position: 2
title: Capabilities
---

# Capabilities

Capabilities are declared through versioned manifests. A manifest owns its resources, routes, permissions, lifecycle hooks and optional UI registration without coupling business packages to the Admin shell.

See the repository's [capability development guide](https://github.com/GnosiST/platform-go/blob/main/docs/platform-capability-development.md).

## Default capabilities

Identity, tenancy, organization, roles, role groups, menus, resources, audit, sessions, file storage and branding form the default foundation. Role groups classify roles and define governance boundaries; they do not grant permissions directly.

## Optional profiles

Personnel, notifications, jobs, enterprise governance and production Admin OIDC are enabled through explicit profiles. Multi-datasource routing, tenant placement, read/write routing, sharding, federation, XA, MQ and search projection have independent controlled contracts, but remain disabled by default and still require downstream composition, environment certification and external evidence for promotion.

## Install, Disable And Uninstall Boundary

The platform is not a runtime hot-plug marketplace. Installing a capability means declaring desired state before startup through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root. Disabling means removing it from the desired set, regenerating contracts and manually restarting. A mismatch between the running process and desired set is only `pendingRestart`, not a hot-apply path. After restart, Admin resources, App routes, auth providers and demo data sets must disappear from the exposed surface. `dictionary`, `tenant`, `identity`, `session`, `rbac`, `menu` and `admin-shell` are non-removable foundation capabilities. Destructive uninstall or persisted data purge needs reviewed migration and rollback evidence.

## Plugin management v1

Plugin management v1 uses a restart-required desired-state model. Declare the desired capability set through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root, run contract gates, regenerate contracts and restart manually. Client notices use HTTP polling, `version.json` or an API version check; v1 does not use WebSocket for hot updates. It also does not support runtime hot install/uninstall, remote repository pull, source deletion, data purge or one-click destructive uninstall.

## Credential Auth v1

`credential-auth` is the local credential authentication capability for username/password, phone/password, email/password and phone/SMS OTP login. Deliverable v1 provides provider discovery, GORM persistence, Argon2id, application-layer encrypted secret transport, digest-only CAPTCHA/slider challenges, password change/reset, audit and rate-limit gates; `GET /api/auth/providers`, `POST /api/auth/challenges`, slider-only non-consuming `POST /api/auth/challenges/verify`, `POST /api/auth/sms-otp/start` and `POST /api/auth/login` preserve demo/OIDC compatibility. It still does not enable the legacy `password` provider kind and is not production-complete. Passwords, OTPs, challenge answers and proofs must not be stored in generic `Record.Values`. SMS delivery belongs to the `notification` SMS channel: Aliyun/Tencent support dry-run and explicitly enabled SDK live send, while `mock-local` remains development/test only. Production still needs approved-account delivery evidence and independent promotion approval; SMTP and WeChat external-provider adapters are not implemented.

New business projects should keep concrete business capabilities in a downstream repository or composition root. Only cross-domain reusable behavior should become a platform profile.

## Registration checks

```bash
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-credential-auth-v1.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
```
