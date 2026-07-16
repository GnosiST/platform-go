---
sidebar_position: 2
title: Architecture overview
---

# Architecture overview

`platform-go` separates declarations, runtime and adapters. Business packages depend on public contracts instead of the Admin shell or a concrete database.

## Request path

```text
Identity + TenantContext -> Capability Manifest -> Casbin/data scope
  -> Query/Command contract -> GORM repository -> durable storage
```

Tenant and data scope are resolved by the trusted server context. Ordinary clients cannot submit a DSN, physical database, shard or arbitrary permission.

The platform is divided into Admin, Service/Data, Control, External/Partner and Event planes. Each capability owns its resource schema, routes, permissions and lifecycle declarations.
