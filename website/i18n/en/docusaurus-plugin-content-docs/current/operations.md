---
title: Operations
---

# Operations

Start with the generated contracts and read-only preflight checks:

```bash
go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-foundation-alignment.mjs
```

Production configuration must provide a non-development JWT secret, durable stores, Redis invalidation and an explicit public base URL. Destructive migrations and data lifecycle purge operations are separate, reviewed workflows.
