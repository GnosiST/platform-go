---
sidebar_position: 3
title: Development guide
---

# Development guide

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run dev
```

The API defaults to `http://127.0.0.1:9200`; Admin defaults to `http://127.0.0.1:9202`.

To add a capability, define a manifest, resource schema, permissions, lifecycle and composition-root wiring. Generate OpenAPI and resource contracts, then add unit, consumer-contract and migration/rollback evidence. Do not hard-code business menus into platform core or bypass the shared UI wrappers and authorization middleware.

```bash
node scripts/validate-admin-resources.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-foundation-alignment.mjs
npm --prefix website run build
```
