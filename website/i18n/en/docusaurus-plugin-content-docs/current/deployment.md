---
sidebar_position: 8
title: Deployment guide
---

# Deployment guide

Deploy the Gin API as a long-running service behind TLS, health checks and a rolling-release system. Vercel is optional for Admin static hosting only.

```text
PLATFORM_RUNTIME_ENV=production
PLATFORM_JWT_SECRET=<non-default-secret>
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=<redis-address>
PLATFORM_CAPABILITIES=<explicit-profile>
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
```

Run production preflight, migration, backup/restore and rollback rehearsals before release. GitHub Pages builds `website/` from `main`; Chinese is the default locale and English is available at `/platform-go/en/`.
