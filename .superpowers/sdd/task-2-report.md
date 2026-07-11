# Task 2 Report: Digest-Only Session Persistence

## Status

Implemented. Public Store and HTTP/JWT call sites still use the opaque raw session handle, while every in-memory repository key and persistence adapter now uses `sha256:v1` digests only.

## Changes

- Added `session.DigestToken(raw)` with the domain separator `platform-session\x00` and SHA-256 hex output prefixed by `sha256:v1:`.
- Added repository-only `StoredSession` with persisted `TokenDigest`; changed `Repository` methods and `Snapshot` to use `StoredSession` and digest lookup values.
- Changed `Session.Token` to `json:"-"`. Store issue/resolve/renew/revoke computes the digest before every map or repository operation and restores the raw token only for the immediate public return value.
- Changed File, SQL and GORM repositories to persist `tokenDigest` / `token_digest`, never the raw token.
- Silenced the GORM session repository logger so SQL error logging cannot interpolate session digests.
- Removed session credential arguments from `recordAudit`, removed `shortSessionID`, and updated all auth, app phone and API-token audit call sites.
- Updated the production-auth contract, validator, tests and auth documentation to forbid raw handles, digests and shortened derivatives in auth audits and to require digest-only session persistence.

## RED Evidence

- `rtk go test ./internal/platform/session -run 'Test.*(Digest|RawSessionToken|RepositoryBacked|LegacyV1|LegacyRawToken)' -count=1`
  - Failed to compile because `DigestToken` did not exist. The new file, SQL, GORM and Store tests therefore exercised a missing contract rather than an already-working path.
- `rtk node --test scripts/platform-production-auth-hardening.test.mjs`
  - The credential-free audit policy was rejected by the old `shortSessionID` validator.
  - A later mutation test proved the validator incorrectly accepted `raw-token`, `rawTokenPersistenceAllowed=true` and `legacyRawSessionMigration=preserve` before the new persistence gates were added.

## GREEN Evidence

- Focused digest/raw/legacy tests: 8 passed.
- Full `internal/platform/session`: 35 passed.
- Focused Session/Audit tests across session and httpapi: 47 passed.
- Full session and httpapi packages: 185 passed.
- Production-auth validator tests: 33 passed.
- Full Go repository: 617 passed in 23 packages.
- `rtk node scripts/validate-platform-production-auth-hardening.mjs`: passed.
- `rtk git diff --check`: passed.

## Migration Semantics

### File

- v2 snapshots contain only digest-keyed `StoredSession` records.
- Loading a v1 snapshot atomically rewrites the file to an empty v2 snapshot before returning. Historical raw-token sessions are intentionally revoked and the raw marker is removed from disk.
- Unknown snapshot versions fail closed.

### SQL Repository

- Initialization starts a transaction, inspects the current columns, and transactionally drops/recreates `platform_sessions` when a legacy `token` column exists or `token_digest` is missing.
- Replacement intentionally contains no historical rows, revoking all legacy sessions.

### GORM Repository

- SQLite and PostgreSQL replace a legacy table inside a GORM transaction and verify that `token_digest` exists and `token` does not.
- MySQL creates `platform_sessions_digest_v2`, atomically swaps it with the legacy table using one `RENAME TABLE` statement, then removes `platform_sessions_legacy_v1`.
- Startup recovery handles interrupted cleanup: it promotes an orphan replacement when the main table is missing, restores a legacy table only when it is the sole recoverable table so migration can rerun, and removes orphan replacement/legacy tables when the main table exists.
- Repository construction succeeds only after the final schema verification confirms that the raw `token` column is absent.

## Residual Risks

- Live MySQL/PostgreSQL migration tests are opt-in through `PLATFORM_TEST_MYSQL_DSN` and `PLATFORM_TEST_POSTGRES_DSN`; default CI still needs database services and these environment variables before the production-driver evidence runs automatically.
- `internal/platform/refreshtoken` still persists its `SessionID`. The refresh-token-family runtime remains implemented-disabled and was intentionally not enabled or expanded in this task. It must be digest-hardened before any production promotion.
- The App current-session response continues returning the immediate raw server-side handle as explicitly retained by the task decision; persistence, logs and audits do not receive it.

## Live Database Verification

- Added opt-in GORM integration tests that create real legacy raw-token tables and verify migration removes the raw marker, removes the `token` column, installs `token_digest`, empties legacy sessions and leaves no replacement/legacy tables.
- Integration tests refuse to run destructive table setup unless the connected database name starts with `platform_session_integration_`.
- MySQL 8.4.10 passed five recovery states: legacy current table, replacement-only, legacy-only, digest current with both leftovers, and legacy current with replacement.
- PostgreSQL 17.10 passed the transactional legacy-table replacement path.
- Both database suites were rerun after persisted-digest validation was added and passed against isolated temporary containers.

## Scope Notes

- `docs/platform-auth.md` changed only to synchronize the implemented digest-only persistence and removal of `shortSessionID` from auth audits.
- No refresh-token-family enablement and no source-writing capability were introduced.

## Review Remediation: Persisted Digest Validation

The follow-up review identified that `StoredSession.TokenDigest` was trusted by name rather than validated at runtime. A repository implementation, malformed database row or damaged file snapshot could therefore return a raw handle or malformed digest and have the Store cache it.

The remediation adds one fail-closed digest boundary shared by the Store and all session repositories:

- canonical persisted identifiers are exactly `sha256:v1:` followed by 64 lowercase hexadecimal characters;
- File, SQL and GORM repositories reject malformed Create and lookup inputs before persistence access;
- File v2 snapshots reject malformed values and map keys that differ from `StoredSession.TokenDigest`;
- SQL and GORM scans reject malformed persisted rows before returning them;
- Store construction and Reload validate the complete snapshot before replacing the last valid cache;
- repository Resolve, Renew and Revoke results are validated against the requested digest before caching;
- validation errors use a fixed message and do not interpolate the rejected raw or malformed value.

The session-policy, OIDC and Admin resource-schema documents now agree that auth audits do not persist raw session handles, digests or shortened derivatives, and that the generic audit schema has no `sessionId` field. The production-auth validator has mutation coverage for these statements and for the exact canonical digest format.

### Follow-up RED Evidence

- `rtk go test ./internal/platform/session -run 'Test(RepositoriesRejectNonCanonicalSessionDigests|FileRepositoryRejectsMalformedV2Snapshots|SQLRepositoryRejectsMalformedPersistedDigest|GORMRepositoryRejectsMalformedPersistedDigest|RepositoryBackedStoreRejectsInvalid)' -count=1`
  - 1 existing unknown-version assertion passed and 35 new digest-boundary assertions failed before implementation.
- `rtk node --test --test-name-pattern 'rejects session and OIDC documentation' scripts/platform-production-auth-hardening.test.mjs`
  - Failed because the validator ignored the mutated session-policy, OIDC and Admin resource-schema documents.

### Follow-up GREEN Evidence

- Focused digest-boundary tests: 36 passed.
- Full `internal/platform/session`: 71 passed.
- Session and HTTP API packages: 221 passed.
- Full Go repository: 653 passed in 23 packages.
- Production-auth validator mutation suite: 34 passed.
- `rtk node scripts/validate-platform-production-auth-hardening.mjs`: passed.
- `rtk git diff --check`: passed.

### Follow-up Scope

- This remediation does not change or enable `internal/platform/refreshtoken`.
- It does not enable source writing.
- Live MySQL/PostgreSQL integration coverage is committed separately so the digest/docs fix remains reviewable.
