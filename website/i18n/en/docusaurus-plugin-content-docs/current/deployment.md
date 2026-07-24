---
sidebar_position: 2
title: Deployment guide
---

# Deployment guide

The default topology is a long-running Gin API behind a reviewed TLS edge. Admin can be same-origin Nginx static content or a separate static site that reaches the API through a reviewed `/api/*` proxy or CORS contract. Vercel is an Admin-static adapter only; it does not change the API long-lived-service boundary.

## Recommended topology

```text
Browser -> reviewed HTTPS edge -> Nginx/Admin static -> Gin API -> durable stores + Redis
```

- A trusted edge terminates HTTPS. Canonical redirects use `PLATFORM_PUBLIC_BASE_URL`, not the request Host.
- The API accepts forwarded headers only from explicit proxy IPs. Never trust client-supplied `X-Forwarded-For` or protocol headers.
- The health check is loopback `GET /api/ready`; `GET /api/health` remains liveness.
- File uploads and content are delivered through authenticated endpoints. Do not configure Nginx `/uploads`, `alias`, or a public object-store URL.

## Minimum production configuration

```text
PLATFORM_RUNTIME_ENV=production
PLATFORM_PUBLIC_BASE_URL=https://platform.example.test
PLATFORM_JWT_SECRET=<dedicated-secret>
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_ADMIN_RESOURCE_DRIVER=<certified-gorm-driver>
PLATFORM_ADMIN_RESOURCE_DSN=<private-dsn>
PLATFORM_SESSION_DRIVER=<certified-gorm-driver>
PLATFORM_SESSION_DSN=<private-dsn>
PLATFORM_LIFECYCLE_HISTORY_DRIVER=<certified-gorm-driver>
PLATFORM_LIFECYCLE_HISTORY_DSN=<private-dsn>
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=<redis-address>
PLATFORM_CAPABILITIES=<explicit-profile>
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
```

Configure `PLATFORM_TRUSTED_PROXIES`, `PLATFORM_EDGE_TRUSTED_PROXY`, distinct data-encryption/blind-index keyrings, and `PLATFORM_RATE_LIMIT_HMAC_KEY`. Private DSNs, keys, and provider credentials never enter git, logs, or support tickets. The source-of-truth template and constraints are in [`deploy/env/production.example.env`](https://github.com/GnosiST/platform-go/blob/main/deploy/env/production.example.env).

## Release flow

1. Run strict preflight against the private environment file and confirm Compose/Nginx matches the reviewed topology.
2. Prepare backup, restore rehearsal, stop conditions, and rollback for schema, data, permission, or key changes.
3. Deploy API and Admin, then monitor readiness, error rate, request IDs, and redacted audit.
4. Enable credential auth, SMS, messaging, search, or multi-datasource default-off integrations only after controlled validation completes.

```bash
node scripts/validate-platform-production-env.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

GitHub Pages builds and deploys `website/` from `main` through `.github/workflows/pages.yml`. Chinese is the default locale and English is available at `/platform-go/en/`. Pages publishes static documentation; it does not run the API.
