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

- The MySQL migration path was source-reviewed but not exercised against a live MySQL server in this environment. SQLite integration tests prove the replacement semantics and raw-column removal, but they are not MySQL production proof.
- `internal/platform/refreshtoken` still persists its `SessionID`. The refresh-token-family runtime remains implemented-disabled and was intentionally not enabled or expanded in this task. It must be digest-hardened before any production promotion.
- The App current-session response continues returning the immediate raw server-side handle as explicitly retained by the task decision; persistence, logs and audits do not receive it.

## Scope Notes

- `docs/platform-auth.md` changed only to synchronize the implemented digest-only persistence and removal of `shortSessionID` from auth audits.
- No refresh-token-family enablement and no source-writing capability were introduced.
