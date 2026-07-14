# Platform Data Governance And Integrations Assessment

Date: 2026-07-12
Governance updated: 2026-07-15

## Purpose

This assessment records the current implementation truth for sensitive-data display and controlled reveal, deletion and retention, multi-datasource portability, and optional messaging/search integrations.

It does not mark every assessed capability as implemented. Governance now records `66 total / 47 implemented / 19 controlled unfinished`; `mask-strategy-runtime`, `sensitive-data-reveal-step-up`, `data-lifecycle-retention`, `platform-service-contract-standard`, `persisted-query-command-object-runtime` and the disabled-by-default integration ports are implemented and closed, while organization governance, datasource, database certification, concrete MQ/search adapters and publication work remain pending.

The [remaining-task topology adjustment](superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md) remains the activated source of truth for the program boundaries and dependencies. The service contract, persisted Query/Command runtime and disabled integration ports are closed; the remaining 19 nodes retain their approved order and independent completion gates.

## Current-State Summary

| Area | Current implementation | Important limitation |
| --- | --- | --- |
| Sensitive display | Field contracts support versioned field-configurable masking and explicit reveal policy. Encrypted `masked` projections return only the masked value; an authorized detail-field action can create a scoped challenge, satisfy OIDC reauthentication and/or Admin SMS OTP, consume one short-lived grant and show plaintext only in the expiring modal. | Existing pre-masked or hashed records cannot be revealed because the original plaintext is unavailable. Email OTP, TOTP, WebAuthn and CAPTCHA risk gates are not implemented factors. |
| Passwords and transport | The default platform has no local-password provider or password repository. Production configuration requires HTTPS and HSTS. | A future local-password capability still needs a dedicated Argon2id repository and must never use generic resource values or reversible encryption. |
| Deletion | Every enabled Admin resource declares an explicit mode. Supported behavior includes restricted terminal removal, soft delete/restore, API-token revocation with 90-day retained metadata, and file tombstone recovery with 30-day cleanup eligibility. Final purge is maintenance-only. | The scheduler is disabled by default and initially supports only the default datasource. There is no online purge, universal recycle bin, general archive tier or substitute for external backups. |
| Query and command objects | Versioned server registrations own typed arguments, permission, tenant/data scope, logical AST construction, cost, timeout, result projection and command idempotency. Authenticated Admin routes and generated OpenAPI/TypeScript contracts use only IDs, versions and typed logical inputs. | Runtime composition is conditional and the stock API process does not register the reference definitions. Workload identity, datasource routing and federated query remain separate nodes. |
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

The initial reveal-capable factor set is implemented as OIDC reauthentication and Admin SMS OTP. OIDC uses a reveal-specific state flow and must resolve to the active bound Admin identity. SMS uses the phone verification provider port and a configured Admin phone source whose current phone digest must match the digest captured at verification time. Email OTP, TOTP, WebAuthn and slider/CAPTCHA adapters are not implemented in the current runtime.

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

The implemented deletion contract supports explicit modes:

- `disabled` or `append-only` for audit and immutable evidence;
- `restrict` for referenced master data;
- `soft-delete` for recoverable business records;
- `revoke` for credentials, sessions and tokens;
- `tombstone` for files and asynchronous external cleanup;
- `hard-delete` only for explicitly approved low-risk records or final purge.

Soft-deleted records use `deletedAt`, `deletedBy`, `deleteReason` and optional `purgeAfter`. Normal queries exclude them. Restore and purge use separate permissions and audit actions. File deletion creates a recoverable tombstone without deleting the object in the HTTP request; maintenance claims external cleanup after the restore window and retains restart-safe state until object deletion or not-found is durable. The maintenance runner is disabled by default, uses `24h / 100 / 3` schedule defaults, and supports bounded batches, resumable cursors, a lease, retries and per-policy retention windows.

Retention changes are audited. Apply requires a completed dry-run checkpoint, the exact current and proposed fingerprints, and matching persisted promotion evidence. Shortening a policy cannot immediately purge existing records merely because a new manifest is deployed. Audit and immutable evidence resources remain append-only even when other resources use recovery windows.

The shipped policy is conservative: referenced master data is restricted or recoverable without automatic purge, API-token revocation is immediate with retained sanitized metadata, files have a 30-day recovery window, and audit/evidence resources are append-only. There is no built-in archive stage; any archive, legal-hold or compliance purge process is a separate governed system.

### Multi-Datasource And Database Portability

The target is a configuration-driven SaaS data plane. Three architectural levels were considered:

1. Keep one global DSN. Lowest complexity, but does not meet reporting, replica or capability-isolation needs.
2. Named Datasource and DatasourceGroup registry with capability bindings. This is the required runtime foundation, but it is not sufficient for tenant placement, read/write routing or sharding.
3. Configuration-driven tenant placement and routing with separately governed sharding, controlled federation and optional XA. This is the approved target, delivered as independent nodes rather than one oversized datasource task.

Datasource, DatasourceGroup, TenantPlacement, shard, read/write and consistency policies are explicitly declared through configuration or an authorized control plane. Runtime selection is deterministic from trusted identity, TenantContext, capability, operation type, request purpose and configuration version. Ordinary clients cannot submit DSNs, physical datasource names, databases, schemas or shards; privileged overrides require scoped authorization and audit.

One normal business transaction remains pinned to one datasource and shard. Writes and transactions use primary; configured read-after-write consistency uses a bounded primary-sticky window. Cross-source mutation defaults to transactional outbox plus saga or explicit compensation. Cross-database joins are allowed only as controlled read-only persisted report queries with tenant, field, row, timeout and cost limits. XA is an advanced, default-off adapter with compatibility and recovery gates; it is supported as an optional capability, not used as the foundation transaction path.

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

The remaining work is feasible within the Gin/GORM/capability-manifest architecture, but it crosses shared contracts, identity, persistence, authorization, operations and Admin UI. Each node must be independently testable and must not claim driver or feature support before the applicable certification lane passes.

The unique approved decomposition and all dependency, lock and completion-gate decisions live in the [remaining-task topology adjustment](superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md). The governing sequence is:

```text
[implemented: persisted-query-command-object-runtime + integration-ports-disabled-default]
  -> [organization-rbac-menu governance lane || datasource registry and routing lane]
  -> database-certification-matrix
  -> transactional-outbox-and-one-mq-adapter
  -> asynchronous-search-projection
  -> open-source-docs-and-release
```

The Query/Command runtime and disabled integration ports completed their approved parallel batch. The organization UI lane may later run in parallel with the datasource backend lane when their contract and file locks do not overlap.

Stable and future stage gates:

1. Keep the implemented `sensitive-data-protection-runtime` contract stable: versioned encryption, explicit normalizers, key-provider configuration, protected persistence and authorized internal projection are now available.
2. Keep the implemented `sensitive-data-historical-migration` contract stable: no HTTP route or plaintext dual-read, prepare-only journal creation, bounded resumable batches, verification, encrypted escrow and hash-guarded rollback. Require external backup/restore evidence and real MySQL/PostgreSQL integration certification before production promotion.
3. Keep the implemented `mask-strategy-runtime` contract stable: arbitrary sensitive fields remain manifest-driven; response, query, detail, Tooltip and export consume the same backend-owned projection; duplicate projection and plaintext fallback remain forbidden.
4. Keep the implemented `sensitive-data-reveal-step-up` contract stable: OIDC re-authentication and SMS OTP use short-lived single-use grants, rate limits, response-terminal audit and registered adapters. SMS delivery failures atomically cancel the factor transaction so the same challenge can retry; production startup fails when an SMS factor lacks a verified phone source or registered non-debug sender.
5. Keep `data-lifecycle-retention` stable: final purge remains maintenance-only, apply requires completed dry-run plus exact promotion evidence, and the runner remains disabled by default and single-datasource.
6. Keep the implemented executable Platform Service Contract and persisted Query/Command Object runtime stable. Organization authorization may now consume registered high-risk queries, while workload identity protocols, event delivery, datasource routing, federation and XA remain outside these closed nodes.
7. Keep `multi-datasource-contract-and-runtime` narrow: versioned Datasource/DatasourceGroup configuration, capability binding, health and transaction pinning. Tenant placement, read/write routing, sharding, federation and XA have independent completion gates.
8. Certify MySQL, PostgreSQL, SQLite, KingbaseES and Oracle by driver, version and feature. Unverified routing, sharding, federation or XA combinations remain experimental or unsupported.
9. Add disabled/no-op messaging and search ports early, then transactional Outbox, one MQ adapter and asynchronous search projection after the required transaction and event contracts are stable.
10. Synchronize the open-source manuals, operator runbook, compatibility matrix and public docs site before GitHub publication. Experimental adapters must remain clearly labeled.

The sensitive-data predecessors, `data-lifecycle-retention`, `platform-service-contract-standard`, `persisted-query-command-object-runtime` and `integration-ports-disabled-default` are implemented in the completion program. All 19 remaining nodes are activated as controlled unfinished work, but none may silently reuse existing closeouts or be described as runtime capability. In particular, federation remains a controlled read-only query boundary and XA remains an optional default-off adapter until their independent implementation and certification gates pass. The task graph, dependency locks, engineering capability inventory, release criteria and open-source documentation must remain synchronized as each node advances. `design-taste-frontend` applies only to the future public documentation and marketing surfaces; the dense Admin workflows remain governed by Product Design, existing Ant Design wrappers and `ui-ux-pro-max` accessibility/responsive checks.

## Release Recommendation

Before a public v0.1 release, retain the historical-migration and lifecycle runbooks and external promotion evidence boundary, publish honest deletion semantics, and avoid claiming database or integration support without a passing matrix. Configurable encryption, offline historical migration, manifest-driven masking, controlled reveal, lifecycle retention, the executable service-contract standard, persisted Query/Command execution and disabled integration ports are implemented. Named datasources, tenant routing, read/write routing, sharding, federation, optional XA, database certification, Outbox/MQ and search projection remain unimplemented until their own gates pass. Vendor-specific Oracle, KingbaseES, MQ and search adapters may ship in staged releases only when their experimental status and verification limits are explicit.

## Source Evidence

- Field and projection policies: `internal/platform/capability/manifest.go`, `internal/platform/adminresource/security.go`.
- Historical migration: `docs/platform-sensitive-data-migration.md`, `internal/platform/sensitivemigration/`, `internal/platform/adminresource/sensitive_migration_gorm.go`, `internal/platform/bootstrap/sensitive_migration.go`, `cmd/platform-admin/main.go`.
- Admin value rendering: `admin/src/platform/resources/GenericResourceConsole.tsx`, `admin/src/platform/ui/AdminPrimitives.tsx`.
- Phone masking and verification: `internal/platform/httpapi/app_phone.go`, `internal/platform/httpapi/phone_protection.go`.
- Service contract standard: `internal/platform/capability/service_contract.go`, `resources/platform-service-contract-standard.json`, `resources/generated/platform-service-contract.json`.
- Password and transport boundaries: `cmd/platform-api/main.go`, `internal/platform/httpapi/security_headers.go`.
- Lifecycle policies, runner and file tombstones: `internal/platform/capability/manifest.go`, `internal/platform/adminresource/lifecycle.go`, `internal/platform/datalifecycle/`, `internal/platform/bootstrap/data_lifecycle.go`, `internal/platform/httpapi/server.go`.
- Session and token revocation: `internal/platform/session/store.go`, `internal/platform/httpapi/server.go`.
- Database openers and configuration: `internal/platform/storage/gorm.go`, `internal/platform/config/config.go`, `internal/platform/bootstrap/`.
- Cache-coherence Pub/Sub and optional profiles: `internal/platform/cache/invalidation.go`, `internal/platform/cache/redis_invalidation.go`, `internal/platform/httpapi/server.go`, `resources/platform-capability-profiles.json`.
