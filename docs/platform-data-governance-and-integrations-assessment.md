# Platform Data Governance And Integrations Assessment

Date: 2026-07-12

## Purpose

This assessment records the current implementation truth and a proposed delivery order for sensitive-data display and controlled reveal, deletion and retention, multi-datasource portability, and optional messaging/search integrations.

It does not mark the proposed capabilities as implemented and does not change the current completion task graph. New nodes require explicit approval because they expand the release scope beyond the existing 45-node program.

## Current-State Summary

| Area | Current implementation | Important limitation |
| --- | --- | --- |
| Sensitive display | Field contracts support sensitivity, storage, response and export modes. The `masked` projection returns the stored value unchanged; current App phone bindings therefore persist a hash plus an already-masked value. | The Admin frontend renders returned values directly. There is no generic mask strategy, reveal API, reveal grant or step-up flow. |
| Passwords and transport | The default platform has no local-password provider or password repository. Production configuration requires HTTPS and HSTS. | A future local-password capability still needs a dedicated Argon2id repository and must never use generic resource values or reversible encryption. |
| Deletion | Generic resources use physical deletion. Sessions and API tokens use domain-specific expiration or revocation. Files use tombstone, object deletion and purge in one request path. | There is no platform-wide deletion policy, recycle bin, restore window, retention runner or historical purge strategy. |
| Databases | GORM openers are wired for MySQL, PostgreSQL and SQLite. Admin resources, sessions and lifecycle history can use separate DSNs. | This is subsystem separation, not named business datasources or tenant/capability routing. The three wired drivers do not yet have a full-platform certification matrix; `PLATFORM_DATABASE_DRIVER` and `PLATFORM_DATABASE_DSN` are not wired into process composition. Oracle and KingbaseES are not implemented or certified. |
| Messaging and search | Redis Pub/Sub distributes cache invalidation and triggers resource/session reload paths. Notification and job profiles provide resource contracts only. | This is best-effort cache coherence, not a durable message bus. There is no transactional outbox, worker engine, Kafka/RabbitMQ/NATS adapter, Elasticsearch/OpenSearch adapter or replay/DLQ runtime. |

The `sensitive-data-historical-migration` node is `implemented` as a maintenance-only CLI with manifest-derived targeting, inventory, dry-run, prepare, apply, verify, restore rehearsal and guarded rollback. MySQL and PostgreSQL remain production targets that require real driver/version integration rehearsal and certification evidence before promotion; SQLite is limited to local development/test rehearsal, and Oracle and KingbaseES remain uncertified.

## Recommended Architecture Decisions

### Sensitive Data Display And Controlled Reveal

Three viable policies were considered:

1. Global masking switch. Simple to understand, but one setting can expose every list, detail, Tooltip and export. This is not recommended.
2. Always-mask plus controlled field reveal. Masked values remain the default; authorized users request one field through a short-lived, single-use grant after step-up verification. This is recommended.
3. Never reveal through the platform. Safest and simplest, but unsuitable for operational workflows that legitimately need verified access to the original value.

The recommended model separates masking from revealing:

- `MaskStrategy` is versioned per field type and locale. It defines examples such as identity number `17******04`, phone `138****8000`, email `na***@example.com` and structured address masking.
- Normal list, detail, Tooltip and export rendering always uses the masked projection unless the field policy explicitly permits controlled reveal.
- Configuration controls whether reveal is allowed for a field or resource. It does not provide a normal runtime switch that renders plaintext everywhere.
- Reveal requires a dedicated permission, purpose code, record and field scope, rate limit, short TTL, single-use grant and append-only audit.
- `StepUpProvider` is an extensible registry. Policies can use `anyOf` or `allOf` over OIDC re-authentication, SMS OTP, email OTP, TOTP or WebAuthn.
- A slider or CAPTCHA is a risk and anti-automation condition, not an identity factor. It may precede step-up but cannot satisfy reveal by itself.
- Revealed values automatically return to the masked state and must not enter browser storage, URLs, logs, analytics or client-generated exports.

There are currently zero reveal-capable step-up factors. Existing Admin OIDC is a login flow, and existing SMS verification is scoped to App phone binding. They are reusable implementation primitives, not evidence of an Admin reveal flow. Email OTP, TOTP, WebAuthn and slider/CAPTCHA adapters do not exist today.

The first supported step-up set should be small and evidence-based:

- OIDC re-authentication and SMS OTP are the recommended first identity factors;
- policy supports one factor through `anyOf` or a required combination through `allOf`;
- email OTP, TOTP and WebAuthn remain adapter milestones with their own recovery and enrollment rules;
- slider/CAPTCHA may be a required risk gate before an identity factor, but never satisfies identity verification alone.

Admin UI uses a read-only `MaskedField` component and a server-driven verification dialog rather than local security decisions. The reveal trigger keeps a minimum 44px target, the dialog traps and restores focus, async and error states are announced, OTP inputs request the appropriate mobile keyboard, `allOf` policies show factor progress, and expiry automatically restores the masked projection. Plaintext never enters `localStorage`, `sessionStorage`, route state, query strings, telemetry or generic client-side export code.

Configuration is split by risk. Field classification, storage mode, normalization and encryption envelope rules become immutable after protected data exists. Mask presentation and step-up policy may be versioned and changed with audit, but there is no global runtime switch that makes all sensitive fields render plaintext.

### Deletion And Retention

Three approaches were considered:

1. Universal soft delete. Easy to describe, but wrong for sessions, tokens, files, append-only audits and immutable reference data.
2. Resource-declared deletion policy. Each capability declares the supported deletion mode and retention behavior. This is recommended.
3. Event sourcing for all resources. Strong history, but disproportionate to the current platform and not recommended for the foundation.

The recommended deletion contract supports explicit modes:

- `disabled` or `append-only` for audit and immutable evidence;
- `restrict` for referenced master data;
- `soft-delete` for recoverable business records;
- `revoke` for credentials, sessions and tokens;
- `tombstone` for files and asynchronous external cleanup;
- `hard-delete` only for explicitly approved low-risk records or final purge.

Soft-deleted records use `deletedAt`, `deletedBy`, `deleteReason` and optional `purgeAfter`. Normal queries exclude them. Restore and purge use separate permissions and audit actions. The maintenance runner is disabled by default, supports dry-run, bounded batches, resumable cursors, a lease, retries and per-policy retention windows.

Retention changes are audited. Shortening a policy cannot immediately purge existing records: it first produces a dry-run impact report and requires an explicit promotion decision. Audit and immutable evidence resources remain append-only even when other resources use recovery windows.

Default policy should be conservative: master data is recoverable and not auto-purged, sessions and verification records have bounded retention, files have a configurable recovery window, and audit data follows hot storage, archive and compliance purge stages.

### Multi-Datasource And Database Portability

Three approaches were considered:

1. Keep one global DSN. Lowest complexity, but does not meet reporting, replica or capability-isolation needs.
2. Named datasource registry with capability bindings. This is recommended.
3. Transparent cross-database federation and XA transactions. High operational cost and incompatible behavior across target databases; not recommended.

The recommended registry defines named sources such as `primary`, `read` and `reporting`. Capabilities bind to a datasource name instead of opening arbitrary DSNs. One business transaction stays within one datasource. Cross-source workflows use outbox plus saga or compensating actions rather than XA.

Support claims must be evidence-based:

- MySQL, PostgreSQL and SQLite enter the certified matrix only after repository, migration, query, transaction, pagination and locking tests cover the full platform, not only sessions.
- KingbaseES is a separate PostgreSQL-compatible adapter and certification lane. Compatibility must not be inferred from its protocol alone.
- Oracle is a separate phase covering driver licensing, identifier rules, sequences, pagination, JSON behavior, locking, migrations and CI or controlled-environment evidence.
- An adapter without a passing matrix is labeled experimental, not supported.

### Optional Messaging And Search Integrations

Three approaches were considered:

1. Call vendor clients directly from business transactions. Fast initially, but creates dual-write loss and vendor coupling.
2. Platform ports plus transactional outbox and asynchronous projections. This is recommended.
3. Enable a full integration stack by default. This increases deployment and operations cost for projects that do not need it and is not recommended.

The platform should define `MessageBus`, `SearchIndexer` and `SearchReader` ports with `disabled/no-op` defaults. A transactional outbox, idempotency keys, retries, dead-letter handling, replay, health checks and redacted operations views must exist before any vendor adapter is promoted.

Only one MQ adapter should be implemented for the first real workload. RabbitMQ is the better first candidate for task and notification delivery; Kafka is the better first candidate for durable event streams. NATS remains an optional later adapter. Elasticsearch and OpenSearch are asynchronous search projections; the relational database remains the source of truth for authorization, deletion, restore and audit semantics.

## Feasibility And Implementation Plan

All four requested capability groups are feasible within the current Gin/GORM/capability-manifest architecture, but none is a one-file switch. The work crosses shared contracts, persistence, authorization, operations and Admin UI. Each stage must be independently testable and must not claim vendor support before its runtime and compatibility matrix pass.

The work should be decomposed into four independent specifications and detailed implementation plans:

1. `sensitive-data-reveal-step-up`
   - Depends on the implemented `sensitive-data-protection-runtime` node, which now provides configurable field policy, encryption, key-provider and persistence contracts; reveal work still requires its own authorization and verification design.
   - Adds mask strategies, reveal policy, step-up registry, grants, audit and Admin masked-field components.
2. `data-lifecycle-retention`
   - Adds deletion policy contracts, reference guards, soft-delete/recycle-bin runtime, file recovery, session/token linkage and the maintenance retention runner.
3. `multi-datasource-database-portability`
   - Adds named datasource configuration, capability binding, transaction boundaries and the certified database matrix. KingbaseES and Oracle remain separate certification milestones.
4. `optional-messaging-search-integrations`
   - Adds disabled ports first, then transactional outbox, one MQ adapter and Elasticsearch/OpenSearch projections.

Recommended execution sequence:

```text
sensitive-data-protection-runtime
  -> sensitive-data-historical-migration
  -> mask-strategy-runtime
  -> sensitive-data-reveal-step-up
  -> data-lifecycle-retention
  -> multi-datasource-contract-and-runtime
  -> database-certification-matrix
  -> integration-ports-disabled-default
  -> transactional-outbox-and-one-mq-adapter
  -> asynchronous-search-projection
  -> open-source-docs-and-release
```

Stage gates:

1. Keep the implemented `sensitive-data-protection-runtime` contract stable: versioned encryption, explicit normalizers, key-provider configuration, protected persistence and authorized internal projection are now available.
2. Keep the implemented `sensitive-data-historical-migration` contract stable: no HTTP route or plaintext dual-read, prepare-only journal creation, bounded resumable batches, verification, encrypted escrow and hash-guarded rollback. Require external backup/restore evidence and real MySQL/PostgreSQL integration certification before production promotion.
3. Add `mask-strategy-runtime`; prove list, detail, Tooltip and export projections use the same backend-owned strategies.
4. Add `sensitive-data-reveal-step-up`; start with OIDC re-authentication and SMS OTP, short-lived single-use grants, rate limits and append-only audit. Add other factors only through registered adapters.
5. Add `data-lifecycle-retention`; declare deletion semantics per resource, then implement restore, final purge and a disabled-by-default maintenance runner.
6. Add `multi-datasource-contract-and-runtime`; provide named sources, capability binding, health checks and one-datasource transaction boundaries without XA.
7. Add `database-certification-matrix`; certify MySQL, PostgreSQL and SQLite across repositories, migrations, transactions, pagination and locking. Run separate KingbaseES and Oracle milestones before labeling either supported.
8. Add disabled/no-op messaging and search ports, then a transactional outbox. Promote exactly one MQ adapter for the first real workload and build search as an asynchronous projection with rebuild and replay paths.
9. Synchronize the open-source manuals, operator runbook, compatibility matrix and public docs site before GitHub publication. Experimental adapters must remain clearly labeled.

The first two nodes are implemented in the completion program. The newly proposed nodes must not silently reuse existing closeouts. If approved, the task graph, dependency locks, engineering capability inventory, release criteria and open-source documentation expand together. `design-taste-frontend` applies only to the future public documentation and marketing surfaces; the dense Admin workflows remain governed by Product Design, existing Ant Design wrappers and `ui-ux-pro-max` accessibility/responsive checks.

## Release Recommendation

Before a public v0.1 release, retain the historical-migration runbook and external promotion evidence boundary, publish honest deletion semantics, and avoid claiming database or integration support without a passing matrix. The configurable encryption runtime and offline historical migration are implemented, but controlled reveal, named datasources and disabled integration ports remain future foundation capabilities. Vendor-specific Oracle, KingbaseES, MQ and search adapters may ship in staged releases only when their experimental status and verification limits are explicit.

## Source Evidence

- Field and projection policies: `internal/platform/capability/manifest.go`, `internal/platform/adminresource/security.go`.
- Historical migration: `docs/platform-sensitive-data-migration.md`, `internal/platform/sensitivemigration/`, `internal/platform/adminresource/sensitive_migration_gorm.go`, `internal/platform/bootstrap/sensitive_migration.go`, `cmd/platform-admin/main.go`.
- Admin value rendering: `admin/src/platform/resources/GenericResourceConsole.tsx`, `admin/src/platform/ui/AdminPrimitives.tsx`.
- Phone masking and verification: `internal/platform/httpapi/app_phone.go`, `internal/platform/httpapi/phone_protection.go`.
- Password and transport boundaries: `cmd/platform-api/main.go`, `internal/platform/httpapi/security_headers.go`.
- Generic deletion and file tombstones: `internal/platform/adminresource/store.go`, `internal/platform/adminresource/audit.go`, `internal/platform/httpapi/server.go`.
- Session and token revocation: `internal/platform/session/store.go`, `internal/platform/httpapi/server.go`.
- Database openers and configuration: `internal/platform/storage/gorm.go`, `internal/platform/config/config.go`, `internal/platform/bootstrap/`.
- Cache-coherence Pub/Sub and optional profiles: `internal/platform/cache/invalidation.go`, `internal/platform/cache/redis_invalidation.go`, `internal/platform/httpapi/server.go`, `resources/platform-capability-profiles.json`.
