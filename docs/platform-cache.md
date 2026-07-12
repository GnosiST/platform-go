# Platform Cache And Redis Plan

Date: 2026-07-04
Last updated: 2026-07-10

## Purpose

The admin backend should support Redis to reduce repeated database reads in production-like deployments.

Redis must be an infrastructure adapter behind a platform cache port. Business capabilities should depend on platform services or typed APIs, not on a Redis client.

## Cache Boundary

Recommended platform port:

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, keys ...string) error
    DeletePrefix(ctx context.Context, prefix string) error
}
```

Adapters:

- `noop`: default for tests and local memory mode;
- `memory`: optional local development cache with TTL;
- `redis`: production adapter selected by configuration.

Suggested config:

```bash
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=127.0.0.1:6379
PLATFORM_REDIS_PASSWORD=
PLATFORM_REDIS_DB=0
PLATFORM_RATE_LIMIT_HMAC_KEY=replace-with-dedicated-rate-limit-key
PLATFORM_CACHE_DEFAULT_TTL=300s
```

Current behavior:

- empty `PLATFORM_CACHE_DRIVER`: use noop cache;
- `PLATFORM_CACHE_DRIVER=memory`: use process-local TTL cache;
- `PLATFORM_CACHE_DRIVER=redis`: use Redis through `github.com/redis/go-redis/v9`;
- `PLATFORM_RATE_LIMIT_HMAC_KEY`: dedicated key that HMACs normalized abuse-control dimensions before Redis storage; production requires at least 32 bytes and it must differ from phone and verification-code HMAC keys;
- `PLATFORM_CACHE_DRIVER=redis`: also enables Redis pub/sub invalidation events on `platform:cache:invalidations` so peer API instances refresh local policy, principal and menu caches after resource writes, and reload repository-backed session stores after session issue/renew/revoke writes;
- cache read/write/delete errors fall back to the source of truth and do not fail the HTTP request.

## Initial Cache Targets

Cache only stable, read-heavy admin data.

Implemented first:

- dynamic admin menu result for a principal;
- branding config;
- current principal permission expansion: `user -> roles -> permissions`;
- local Casbin authorizer generated from the dynamic user/role resources;
- auth provider discovery result from enabled capability manifests;
- resource schemas generated from enabled capabilities;
- permission catalog list generated from enabled capability admin declarations;
- cache stats for the active cache adapter through `GET /api/platform/cache/stats`.

Planned next:

- extend cache coverage only when a resource has a clear source of truth and invalidation point;
- evaluate session-current response caching only after distributed session revocation is designed.

Do not cache:

- raw tokens, passwords, WeChat secrets or OAuth codes;
- mutable write payloads before persistence succeeds;
- audit writes;
- data that changes per request and lacks a clear invalidation rule.

## Invalidation Rules

Invalidate after successful writes:

- `users`, `roles`, `permissions`: clear the local Casbin authorizer plus principal permission and menu caches;
- `permissions`: also clear permission catalog and capability-derived schema caches;
- `menus`: clear menu caches;
- `settings`: clear branding and settings-derived caches;
- capability enable/disable or restart: clear capability-derived schema/provider/menu caches;
- session issue, renewal or revoke: publish `sessions` so peer API instances reload the session repository and converge issued, renewed or revoked sessions across instances;
- logout or session revoke: clear current-session derived caches if session response caching is added later.

The invalidation point should live in platform services or repositories, not in frontend code and not in business capability packages.

Single-process policy refresh is local to the API server. Redis cache mode adds a pub/sub invalidation bus so successful writes can notify peer API instances to clear their local policy and read caches. Deployments that use another cache or message infrastructure can implement the same `cache.InvalidationBus` port without changing business capabilities.

## Production Persistence Correctness

- Repository-backed GORM session operations are record-scoped. Issue inserts one session, resolve reads one token, and renew/revoke use conditional single-record updates; normal session operations do not replace the complete session table.
- GORM-backed admin resource saves use snapshot revision compare-and-swap. A stale revision is rejected before table replacement and HTTP maps the conflict to `409 Conflict` so the caller can reload and retry.
- On a peer admin resource event, each API process reloads its independently constructed repository-backed Admin Store before clearing derived policy, principal, menu, branding, permission and schema caches. Reload failure preserves the last valid Store snapshot and derived caches.
- File-backed repositories and the legacy `database/sql` adapters retain format and interface compatibility for local or legacy harnesses. They are not approved production multi-process consistency modes and do not provide the GORM revision-CAS guarantee.
- Redis Pub/Sub invalidation is best-effort convergence. It is not a durable consistency log: it provides no replay, acknowledgement or delivery guarantee. The shared GORM database remains authoritative, and a subscriber reload installs state only after a successful repository read.

## Rollout Order

1. Add cache port and noop adapter. Done.
2. Add Redis adapter and config parsing. Done.
3. Cache admin menus and branding first; both have clear read paths and invalidation points. Done.
4. Cache principal permission expansion after role/permission persistence is stable. Done.
5. Add metrics for hit rate, miss rate, errors and invalidation count. Done.
6. Add a platform invalidation bus with memory test adapter and Redis pub/sub adapter. Done.

## Metrics

`GET /api/platform/cache/stats` returns:

- `driver`;
- `hits`;
- `misses`;
- `sets`;
- `deletes`;
- `deletePrefixes`;
- `errors`;
- `lastError`: the latest bounded operation code (`CACHE_GET_FAILED`, `CACHE_SET_FAILED`, `CACHE_DELETE_FAILED` or `CACHE_DELETE_PREFIX_FAILED`); adapter error text is never exposed.

The endpoint requires `admin:monitoring:read`.

## Design Rule

Redis is an optimization, not the source of truth. The shared GORM database is authoritative for production multi-process correctness; file and legacy SQL repositories are compatibility modes. Cache misses must still return correct data, and Pub/Sub delivery must not be treated as durable consistency evidence.

## Contract Gate

`resources/platform-cache-invalidation.json` records cache targets, resource invalidation rules, Redis pub/sub boundaries, session reload behavior and no-cache policy. Run `rtk node scripts/validate-platform-cache-invalidation.mjs` before changing cache keys, invalidation resources, session invalidation behavior or Redis pub/sub wiring. The validator checks the contract against `internal/platform/httpapi/server.go`, `internal/platform/cache`, this document and the cache-related tests.
