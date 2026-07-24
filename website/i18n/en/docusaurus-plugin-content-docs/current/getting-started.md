---
sidebar_position: 2
title: Run locally in five minutes
---

# Run locally in five minutes

This guide ends with a healthy API response and a visible Admin page. It is not a production deployment guide and it does not initialize a real SMS supplier or external message adapter.

## Prerequisites

- Go 1.26+;
- Node.js and npm;
- a writable local workspace.

Development defaults can use memory or SQLite boundaries. SQLite is only permitted in development/test; production requires durable Admin, session, and lifecycle stores plus Redis.

## Start the API

```bash
git clone https://github.com/GnosiST/platform-go.git
cd platform-go

go run ./cmd/platform-api
```

The default address is `http://127.0.0.1:9200`. In a new terminal, check liveness:

```bash
curl -fsS http://127.0.0.1:9200/api/health
```

A successful response includes `"ok":true`. `/api/health` is process liveness only. Dependency readiness is exposed by `/api/ready`; do not publish its internal detail on an unauthenticated page.

## Start Admin

```bash
npm --prefix admin install
npm --prefix admin run dev
```

Open [http://127.0.0.1:9202](http://127.0.0.1:9202). The Vite server proxies `/api/` to `http://127.0.0.1:9200` by default; use `VITE_PLATFORM_API_PROXY_TARGET` when the local API address changes.

Development can validate the demo/OIDC shell and resource UI. Production must disable the demo provider and must not promote local accounts, debug codes, development JWT keys, or SQLite data as launch configuration.

## Pass the baseline gates

```bash
go test ./...
node scripts/validate-admin-resources.mjs
npm --prefix admin run build
npm --prefix website run build
```

Once API, Admin, and docs builds pass, continue to [business integration](./business-integration). Read [authentication and authorization](./authentication) before configuring identity, and use the [production checklist](./production-checklist) before deploying.

## Common issues

| Symptom | Check first |
| --- | --- |
| `9200` is unavailable | Confirm the API process is running, the port is free, and `PLATFORM_HTTP_ADDR` is not overridden |
| Admin has no API data | Confirm `/api/health`, the Vite proxy target, and that browser requests use `/api/` |
| Production configuration fails | Do not remove validation; copy the private environment from `deploy/env/production.example.env` and run production preflight |
| A login provider is absent | Providers are discovery-driven; check the capability, configuration, and demo-disable switch |
