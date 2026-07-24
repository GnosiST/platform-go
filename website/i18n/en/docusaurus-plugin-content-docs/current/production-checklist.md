---
sidebar_position: 2
title: Production checklist
---

# Production checklist

Use this checklist for pre-release review. It does not replace a private-environment audit, external-account evidence, database recovery rehearsal, or change approval.

## Architecture and network

- Run the API as a long-lived Gin service. Admin can be same-origin static content or a separate static site.
- Terminate public HTTPS at a reviewed edge and trust only explicit proxy IPs. Never treat arbitrary `X-Forwarded-*` values as trusted input.
- `/api/health` is liveness; `/api/ready` is the constrained dependency readiness check. File content is delivered through authenticated endpoints, never by exposing an Nginx static directory.

## Configuration and secrets

- Set `PLATFORM_RUNTIME_ENV=production` and a canonical HTTPS `PLATFORM_PUBLIC_BASE_URL`.
- Use independent JWT, data-encryption, blind-index, and rate-limit HMAC keys.
- Use GORM-backed Admin, session, and lifecycle stores; select Redis cache/invalidation; set `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`.
- Keep private environment files out of git. Replace all template placeholders for production and never log tokens, DSNs, complete recipients, or secrets.

## Data, files, and identity

- Default to one datasource and one native transaction. Promote multi-datasource, read/write routing, sharding, federation, XA, messages, and search only through their controlled contracts/profiles.
- File keys are opaque random identifiers. S3 uses HTTPS and `AES256` or `aws:kms`; operations independently confirms that the bucket has no public access.
- Credential auth requires its dedicated store, HMAC, and P-256 transport key. Phone OTP also needs a non-debug SMS provider and template. Do not convert demo/OIDC development flows into production identity.

## Verify before release

```bash
node scripts/validate-platform-production-env.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/run-platform-production-preflight.mjs --list
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

Then run Compose `config` against the private file and confirm containers, proxy, mounts, and health checks match the approved topology. Preflight does not deploy, migrate, or mutate production.

## Change and rollback

Migration, key rotation, config import/restore, retention purge, SMS delivery, message worker, and search promotion each need their own change packet: backup, impact analysis, execution window, metrics, stop condition, rollback, and redacted evidence. A completed contract is not proof that an external production integration is ready.

The full topology and environment list live in the source-of-truth [deployment guide](https://github.com/GnosiST/platform-go/blob/main/docs/platform-deployment.md). Use [operations](./operations) for runtime diagnosis.
