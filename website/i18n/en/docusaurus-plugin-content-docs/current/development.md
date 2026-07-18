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

Platform capabilities may live under `internal/platform`; concrete business
capabilities should start in a downstream repository or downstream composition
root instead of editing platform core.

1. Copy or adapt the minimum example in `examples/external-capability`.
2. Import only the public contract package `github.com/GnosiST/platform-go/pkg/platform/capability`.
3. Declare resources, menus, permissions, App routes, service contracts, lifecycle steps and demo data in the manifest.
4. Inject storage, HTTP handlers, Admin action handlers and optional UI registration from the downstream composition root.
5. Generate OpenAPI, resource contracts, codegen previews and documentation.
6. Add unit, consumer-contract and migration/rollback evidence.

The example gate verifies that the external package does not import
`internal/platform/**`, then runs `go test ./...` and `go run .` inside the
example directory:

```bash
node scripts/validate-external-capability-example.mjs
node scripts/validate-admin-resources.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-foundation-alignment.mjs
npm --prefix website run build
```
