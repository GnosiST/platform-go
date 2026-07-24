---
sidebar_position: 4
title: Architecture overview
---

# Architecture overview

`platform-go` separates platform capability into declaration, runtime, and adapter layers. Business packages depend on public contracts, not the Admin shell, a concrete database, or internal HTTP handlers.

## Request and contract path

```text
Trusted identity + TenantContext
  -> capability manifest / route declaration
  -> session + Casbin + data scope
  -> Query / Command contract
  -> service and storage port
  -> GORM, cache, file or external adapter
```

Identity comes from a trusted JWT session, OIDC, or declared credential provider. The trusted server context resolves tenant and data scope; ordinary clients cannot submit a DSN, physical database, schema, shard, or arbitrary permission.

## Five planes

| Plane | Responsibility | Key boundary |
| --- | --- | --- |
| Admin Plane | Management resources, menus, forms, audit entry points | Menus are visibility only; backend authorization is independent |
| Service/Data Plane | Business services, queries, commands, data scope | Default is one datasource and one native transaction |
| Control Plane | Capabilities, configuration, migrations, release policy | Desired state requires contract regeneration and restart |
| External/Partner Plane | Stable service contracts for external consumers | OpenAPI, versioning, errors, and idempotency semantics are reviewable |
| Event Plane | Versioned events and controlled asynchronous boundary | Outbox/MQ/search are default-off, not default enabled |

## Extension model

Each `capability.Manifest` owns its resource schema, routes, permissions, lifecycle, and optional UI registration. External business packages use only public surfaces such as `pkg/platform/capability`; when that contract is insufficient, extend the public contract before touching internal platform implementation.

Platform core and optional platform capabilities do not import business applications. Business tables, migrations, state machines, handlers, and UI remain in a downstream composition root, keeping the foundation reusable while domain behavior evolves independently.

Read [business integration](./business-integration) for the smallest practical example and the [support matrix](./support-matrix) to distinguish the default path from controlled integrations.
