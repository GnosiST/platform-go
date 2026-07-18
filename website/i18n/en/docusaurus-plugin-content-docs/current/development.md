---
sidebar_position: 3
title: Development guide
---

# Development guide

```bash
rtk go test ./...
rtk npm --prefix admin install
rtk npm --prefix admin run dev
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
6. Update capability classification, profiles, operation policy and the plugin management v1 contract.
7. Add unit, consumer-contract and migration/rollback evidence.

The example gate verifies that the external package does not import
`internal/platform/**`, then runs `go test ./...` and `go run .` inside the
example directory:

```bash
rtk node scripts/validate-external-capability-example.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk npm --prefix website run build
```

Plugin enable/disable is restart-required in v1: change a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or downstream composition root, regenerate contracts and restart the API manually. v1 does not support WebSocket hot updates, remote repository pull or destructive uninstall.

For local credential login work, start from the `credential-auth` contract. Do not store passwords or verification codes in generic `Record.Values`, and do not enable provider kind `password` before the runtime package deliberately changes that boundary:

```bash
rtk node scripts/validate-platform-credential-auth-v1.mjs
rtk node --test scripts/platform-credential-auth-v1.test.mjs
```
