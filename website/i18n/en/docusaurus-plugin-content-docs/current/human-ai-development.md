---
sidebar_position: 4
title: Human + AI development
---

# Human + AI development

`platform-go` is designed for human developers and AI agents sharing the same foundation. The rule is contract first, customize within boundaries and verify with machine gates.

## Start a business system

1. Read `README.md`, `CONTRIBUTING.md`, the capability development guide and `examples/external-capability`.
2. Put concrete business capabilities in a downstream repository or downstream composition root.
3. Import only the public contract package `github.com/GnosiST/platform-go/pkg/platform/capability`.
4. Declare manifests, resource schemas, menus, permissions, App routes, service contracts and lifecycle steps first.
5. Then attach storage, HTTP handlers, Admin action handlers, optional UI panels and tests.

## Required boundaries

| Area | Rule |
| --- | --- |
| Interfaces | Declare Admin resources, App routes and service contracts before implementation |
| UI | Use platform UI wrappers, Refine providers and schema-driven forms for Admin |
| Visual system | Customize through theme, layout, density, branding and registered components without bypassing accessibility or i18n |
| Code generation | Generate contracts, previews and scaffolds by default; runtime source writes require human review |
| Capability lifecycle | Install, disable and uninstall operations follow the operation policy; foundation capabilities are non-removable |
| Data security | Tenant, organization, area and sensitive fields are controlled by server-side contracts and policies |

See the full [Human + AI Development Protocol](https://github.com/GnosiST/platform-go/blob/main/docs/platform-human-ai-development-protocol.md).

## Extension Lifecycle

The platform supports startup-time capability composition, not runtime hot-plugging. Enable a capability by registering its manifest, then selecting it through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root. After disabling it, regenerate contracts and restart; disabled resources must not remain exposed through Admin or API surfaces. Persisted data purge or source removal is not a generic uninstall button and needs migration, rollback and owner evidence.

Plugin management v1 is defined by `resources/platform-plugin-management-v1.json`. It fixes install/disable as a restart-required desired-state model with manual restart after declaring the desired capability set through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root and regenerating contracts. v1 does not integrate WebSocket, pull plugins from remote repositories, support runtime hot install/uninstall or provide destructive uninstall. New business projects should live downstream; the platform only provides the generic capability boundary.

## Validation entrypoint

```bash
rtk node scripts/validate-platform-human-ai-development-protocol.mjs
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
rtk node scripts/validate-external-capability-example.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
```
