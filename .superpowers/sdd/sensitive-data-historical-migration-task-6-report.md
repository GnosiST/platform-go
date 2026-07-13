# Sensitive Data Historical Migration Task 6 Report

## Scope

Integrated the maintenance-only `platform-admin sensitive-data-migrate` command and its persistent-GORM bootstrap boundary. No HTTP routes or ordinary Admin resource repository behavior changed.

## Implementation

- Added exact dispatch for `inventory`, `dry-run`, `prepare`, `apply`, `verify`, `rehearse-restore` and `rollback`.
- Added normalized parsing for run identity, immutable approval/backup/restore evidence, maintenance-window confirmation and batch sizes from 1 through 1000.
- Added JSON-only success output and value-free operational errors.
- Added a bootstrap-owned migration session that resolves enabled capabilities, builds the data-protection runtime and manifest-derived plan, opens `mysql`, `postgres` or `sqlite` through `storage.OpenGORM`, constructs `GORMProtectedValueMigrationStore` directly and closes the underlying `sql.DB` reliably.
- Rejected memory, file, legacy generic SQL, unsupported-driver and missing-DSN configurations before storage access.
- Kept `AutoMigrate` confined to the existing explicit `prepare` implementation. Inventory, dry-run and verify bootstrap paths do not create migration journal tables.

## TDD Evidence

RED:

```text
rtk go test ./cmd/platform-admin ./internal/platform/bootstrap -run 'Test.*SensitiveDataMigrat' -count=1
Go test: 0 passed, 2 failed in 2 packages
Failures were the intended missing OpenSensitiveDataMigration, sensitiveMigrationSession and runWithDependencies interfaces.
```

The explicit zero-batch regression also failed before the bound was tightened:

```text
rtk go test ./cmd/platform-admin -run TestRunSensitiveDataMigrationRejectsMalformedOrIncompleteArgumentsWithoutValues -count=1
Go test: 9 passed, 2 failed
TestRunSensitiveDataMigrationRejectsMalformedOrIncompleteArgumentsWithoutValues/zero_batch: run() error = nil
```

GREEN:

```text
rtk go test ./cmd/platform-admin ./internal/platform/bootstrap -run 'Test.*SensitiveDataMigrat' -count=1
Go test: 29 passed in 2 packages

rtk go test ./cmd/platform-admin ./internal/platform/bootstrap -count=1
Go test: 99 passed in 2 packages

rtk go vet ./cmd/platform-admin ./internal/platform/bootstrap
Go vet: No issues found

rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource ./cmd/platform-admin ./internal/platform/bootstrap -count=1
Go test: 335 passed in 4 packages

rtk go test ./... -count=1
Go test: 1144 passed in 26 packages

rtk git diff --check
clean
```

## Residual Evidence Boundary

The bootstrap test exercises a real SQLite database with a registry-valid injected encrypted-field manifest and valid test keyrings. MySQL and PostgreSQL runtime execution remains covered by the existing opt-in migration integration evidence rather than this CLI integration task.

## Review Fixes

- Extended `storage.OpenGORM` with optional GORM options while preserving every existing one-argument caller. The sensitive migration bootstrap injects `logger.Discard` at `gorm.Open`, before PostgreSQL initialization and automatic ping can emit a raw driver error.
- Added subprocess tests that capture the real process stdout and stderr around a failed PostgreSQL connection containing a sensitive DSN marker. Bootstrap returns only `ErrSensitiveDataMigrationStorage`, and neither stream contains the marker or driver error.
- Restricted SQLite migration sessions to normalized `development` and `test` runtime environments. Staging and production return `ErrSensitiveDataMigrationConfig` before storage opens.
- Encoded a completed migration report before closing storage. If `Close` fails, stdout remains exactly one JSON report and the command returns the normalized `close sensitive data migration storage` error without the underlying value.

Review RED:

```text
rtk go test ./internal/platform/storage -run '^TestOpenGORMCanSilenceInitializationErrors$' -count=1
Go test: build failed
internal/platform/storage/gorm_test.go:44:4: too many arguments in call to OpenGORM

rtk go test ./internal/platform/bootstrap -run '^TestSensitiveDataMigrationSilencesStorageInitializationErrors$' -count=1
Go test: 0 passed, 1 failed
The captured stdout contained GORM's failed-database-initialization log from the PostgreSQL ping path.

rtk go test ./internal/platform/bootstrap -run '^TestSensitiveDataMigrationSQLiteEnvironmentPolicy$' -count=1
Go test: 2 passed, 2 failed
The production case returned a live SQLite migration session.

rtk go test ./cmd/platform-admin -run '^TestRunSensitiveDataMigrationEmitsReportBeforeNormalizedCloseFailure$' -count=1
Go test: 0 passed, 1 failed
stdout was empty after a successful run followed by a Close failure.
```

Review GREEN:

```text
rtk go test ./cmd/platform-admin ./internal/platform/bootstrap ./internal/platform/storage -run 'Test.*SensitiveDataMigrat|TestOpenGORM' -count=1
Go test: 39 passed in 3 packages

rtk go test ./cmd/platform-admin ./internal/platform/bootstrap ./internal/platform/storage -count=1
Go test: 126 passed in 3 packages

rtk go vet ./cmd/platform-admin ./internal/platform/bootstrap ./internal/platform/storage
Go vet: No issues found

rtk go test ./... -count=1
Go test: 1152 passed in 26 packages
```
