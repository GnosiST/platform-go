# Database Certification Matrix

`resources/platform-database-certification-matrix.json` is the only machine-readable source for database support claims. Its entries are deliberately narrower than platform support: an `experimental` smoke result is not a production certification.

The local GORM smoke test is opt-in. It checks a configured MySQL 8.4 or PostgreSQL 16 target for connection, migration, committed transaction and rolled-back transaction behavior. It does not run when the certification DSN environment variables are absent, and it must not be pointed at a production database.

```zsh
PLATFORM_CERTIFICATION_MYSQL_DSN='redacted local test DSN' \
PLATFORM_CERTIFICATION_POSTGRES_DSN='redacted local test DSN' \
rtk go test ./internal/platform/storage -run '^TestGORMCertificationSmoke$' -count=1
rtk node scripts/validate-platform-database-certification.mjs
```

MySQL and PostgreSQL become `certified` only after full-platform repository, migration, query, transaction, pagination and locking evidence is recorded. SQLite remains development/test-only. Oracle and KingbaseES stay `unsupported` until their distinct driver, controlled environment and compatibility lanes have passed.
