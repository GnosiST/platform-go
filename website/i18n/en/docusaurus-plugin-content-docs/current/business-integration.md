---
sidebar_position: 3
title: Integrate your first business capability
---

# Integrate your first business capability

A business capability belongs in a downstream repository or composition root. The platform supplies stable public contracts and common mechanisms; business packages must not import or modify `internal/platform/**`.

## 1. Begin with the external example

[`examples/external-capability`](https://github.com/GnosiST/platform-go/tree/main/examples/external-capability) is the supported minimum boundary. Copy its module structure, then replace the capability ID and resource declarations in its manifest.

```go
import "github.com/GnosiST/platform-go/pkg/platform/capability"

func Manifest() capability.Manifest {
    return capability.Manifest{
        ID: "inventory",
        // Declare resource schemas, permissions, menus, routes, and lifecycle here.
    }
}
```

An external manifest obtains its capability contract only from `github.com/GnosiST/platform-go/pkg/platform/capability`. App routes use the public `pkg/platform/app` surface; do not reuse internal handlers, stores, or Admin-shell implementation.

## 2. Declare before implementing

Use this sequence so contracts arrive before UI and storage detail:

1. Declare resource schemas, permission prefixes, menu visibility, and lifecycle.
2. Declare Admin/App routes and service contracts.
3. Register storage, handlers, Admin actions, and optional UI from the downstream composition root.
4. Keep domain tables, migrations, state machines, and tests in the business module.
5. Generate contracts so OpenAPI, resource artifacts, and codegen previews reflect the actual composition.

Menus are not authorization. Every protected read and write must pass backend Casbin/session scope checks; hiding a menu is not an access-control mechanism.

## 3. Verify the public boundary

```bash
node scripts/validate-external-capability-example.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-capability-profiles.mjs
node scripts/validate-platform-admin-api-boundary.mjs
```

The external-example gate confirms that the module does not import `internal/platform/**`, then runs its Go tests and manifest preview. When your capability adds Admin resources, also run the resource-contract, OpenAPI, codegen, and Admin-build gates.

## 4. Compose, enable, and disable

Capabilities declare their desired set before startup through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE`, or a downstream composition root. Regenerate contracts and restart the service after changing that set.

Runtime hot install, remote source pull, and generic destructive uninstall are unsupported. Once an optional capability is disabled, its Admin resources, App routes, auth providers, and demo surfaces must disappear from public contracts. Historical-data deletion needs a separate migration, retention, and rollback plan.

## Definition of integrated

- The business module depends only on public platform packages.
- Its manifest, routes, resources, permissions, and lifecycle are observable in generated contracts.
- Domain data and migrations remain in downstream ownership.
- Tests cover authorization, tenant scope, failure, and rollback paths.
- Production enablement has configuration, review, and rollback evidence.

Use the [support matrix](./support-matrix) to distinguish default support from controlled integrations.
