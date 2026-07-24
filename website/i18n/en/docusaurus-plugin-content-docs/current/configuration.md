---
sidebar_position: 2
title: Configuration reference
---

# Configuration reference

Configuration follows environment and capability boundaries. Do not copy example values, test keys, or DSNs to production. The complete private template begins at [`deploy/env/production.example.env`](https://github.com/GnosiST/platform-go/blob/main/deploy/env/production.example.env).

## Base runtime

| Setting | Purpose |
| --- | --- |
| `PLATFORM_RUNTIME_ENV` | `development`, `test`, or `production`; production enables strict constraints |
| `PLATFORM_PUBLIC_BASE_URL` | Canonical HTTPS address visible to users |
| `PLATFORM_HTTP_MAX_BODY_BYTES` | General request-body limit; uploads have a separate file policy |
| `PLATFORM_CAPABILITIES` | Desired capability set before startup; cannot include production demo-data |
| `PLATFORM_CAPABILITY_LOCK_FILE` | Reviewable lock file for capability desired state |

## Stores and cache

`PLATFORM_ADMIN_RESOURCE_DRIVER/DSN`, `PLATFORM_SESSION_DRIVER/DSN`, and `PLATFORM_LIFECYCLE_HISTORY_DRIVER/DSN` own the three persistence boundaries. Production uses certified GORM drivers and DSNs; SQLite is development/test-only.

`PLATFORM_CACHE_DRIVER=redis` and `PLATFORM_REDIS_ADDR` are the production cache/invalidation baseline. Redis is an optimization, not the source of truth for permission or business data.

## Identity and data protection

`PLATFORM_JWT_SECRET`, `PLATFORM_RATE_LIMIT_HMAC_KEY`, data-encryption keyrings, and blind-index keyrings must be distinct. Keyrings preserve historical reads by version ID. Deploy old and new material together and verify reads/backups before changing the active ID.

Disable demo in production with `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`. Enabling `admin-oidc` also requires complete issuer, client, redirect, and Admin-binding flow.

## Optional capabilities

Credential auth uses a dedicated repository, identifier HMAC key, and P-256 secret-transport key. Notification/SMS, message, search, and multi-datasource paths all require their complete capability and adapter configuration. An environment variable cannot turn a missing stock adapter into an enabled integration.

Run the non-mutating checks in the [production checklist](./production-checklist) before changing production configuration. The source-of-truth [deployment guide](https://github.com/GnosiST/platform-go/blob/main/docs/platform-deployment.md) defines the full contract, defaults, and constraints.
