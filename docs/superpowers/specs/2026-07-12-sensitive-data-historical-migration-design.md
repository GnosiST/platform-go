# Sensitive Data Historical Migration Design

## Goal

Provide an offline, resumable and auditable way to inventory, encrypt, verify and roll back historical plaintext values for any admin-resource field whose capability contract declares `storageMode=encrypted`.

The migration must preserve the fail-closed runtime boundary already implemented by the platform. Ordinary Store loading continues to reject plaintext under an encrypted manifest. Migration therefore operates through a dedicated maintenance store and never adds dual-read plaintext behavior to the API process.

## Scope

This node includes:

- contract validation that every encrypted field uses `Source="values"`;
- manifest-derived migration plans with no field-name heuristics;
- inventory and dry-run classification;
- explicit journal and target preparation;
- tenant-scoped keyset cursors and bounded batches;
- transactional GORM mutation for certified drivers;
- global revision compare-and-swap so stale API writers fail;
- append-only run, checkpoint and event journals;
- encrypted field-level rollback escrow;
- verification, resume, restore rehearsal and rollback modes;
- a maintenance-only `platform-admin sensitive-data-migrate` command;
- sanitized machine-readable summaries and operator documentation.

This node does not include an HTTP migration endpoint, background automatic migration, live dual reads, local-password migration, irreversible hash recovery, step-up reveal UI, KMS/HSM adapters, Oracle/Kingbase certification, or production file/legacy-SQL migration writes.

## Storage Boundary

Migration targets only fields that satisfy all of the following:

- declared by an enabled capability manifest;
- `storageMode=encrypted`;
- `Source="values"`;
- resource protection metadata is complete;
- the selected GORM resource table stores the field in `values_json`.

The capability validator rejects encrypted fields in record columns. This avoids driver-specific mutation of dedicated columns and ensures the migration never silently edits a different physical representation than the ordinary Store.

The GORM maintenance store uses an explicit resource-to-table whitelist shared with the normalized admin-resource repository. It supports the generic admin-resource table plus normalized resources only when the encrypted field is stored exclusively inside `values_json`. Several normalized resources also project selected values into dedicated columns or relation tables; historical migration of those keys is rejected because changing only JSON would leave plaintext and changing both representations requires a separate reviewed migration contract. Table and field identifiers are never accepted from command-line input without resolving them through manifest and repository metadata.

## Driver Policy

- MySQL and PostgreSQL are production migration targets after their integration suites pass.
- SQLite is supported for deterministic local tests and restore rehearsal only.
- The legacy generic `database/sql` repository is read/write incompatible with this migration because its snapshot save path is non-transactional and lacks the required row-level journal coupling.
- File storage may be inventoried by a separate future development tool, but this node does not expose file mutation or rollback as production-safe behavior.
- Unsupported drivers fail before journal schema creation or data access.

## Command Modes

The command accepts exactly one mode:

- `inventory`: read-only counts by resource, field and classification; no journal schema mutation;
- `dry-run`: read-only execution of the same plan and batch traversal used by apply, including key and policy validation;
- `prepare`: create or validate only the dedicated migration journal tables, snapshot target coordinates and physical-layout compatibility, and seal the run plan hash; no protected business value is changed;
- `apply`: encrypt plaintext values, write encrypted rollback escrow and advance checkpoints atomically;
- `verify`: prove that all targeted non-empty values validate against the active manifest and that journal counts reconcile;
- `rehearse-restore`: decrypt every escrow value in memory using the reserved rollback context, discard it immediately and emit counts only;
- `rollback`: restore original field values from escrow when the current migrated value hash still matches the recorded post-migration hash.

`inventory` and `dry-run` require persistent GORM configuration and data-protection keys but do not require mutation approvals. `prepare`, `apply`, `rehearse-restore` and `rollback` require:

- a canonical run ID;
- a non-empty actor ID;
- a non-empty reason;
- an approval reference;
- an external backup URI;
- a canonical `sha256:` backup hash;
- a restore-rehearsal evidence reference;
- explicit maintenance-window confirmation.

`verify` requires a run ID and remains read-only. Apply refuses to create or alter journal schema implicitly and requires a completed prepared run with the same manifest-derived plan hash. Unknown flags, positional arguments and unsupported modes fail with normalized, value-free errors.

## Manifest-Derived Plan

The planner produces ordered resource and field entries from enabled manifests. Each field entry contains only contract metadata required for migration: resource, field key, source, protection policy, schema version, scope and tenant field.

The planner rejects:

- duplicate resources or field keys;
- encrypted fields outside `values` storage;
- missing or unsupported protection metadata;
- tenant-field scope without a required plain tenant field;
- migration plans with no encrypted fields;
- irreversible `hashed` fields presented as migration targets.

The maintenance store performs a second physical-layout check. It rejects any encrypted field also projected into a normalized dedicated column or relation table, including currently duplicated keys such as tenant/area, username/IP, roles and permissions. This is not a field-name sensitivity heuristic; it is a repository-owned persistence-layout constraint.

No identity-number, phone, email or address name is embedded in migration logic. Custom sensitive fields behave identically when their manifest policies match.

## Classification

Each targeted value is classified without revealing encrypted values:

- `missing`: key absent or value empty;
- `plaintext`: non-empty value without a `pgo:enc:` prefix;
- `target-envelope`: `Runtime.Validate` succeeds for the exact field policy and AAD context;
- `foreign-envelope`: envelope-shaped value fails with policy, key-version or key-fingerprint mismatch;
- `malformed-envelope`: envelope-shaped value fails structural or authentication validation.

Only `plaintext` is eligible for apply. Target envelopes are idempotent no-ops. Foreign and malformed envelopes fail the batch closed and are never double-encrypted. Classification reports contain counts only, never values, ciphertext, blind indexes, tenant-field values or record IDs.

## Tenant Batches And Cursors

Read-only modes stream resource rows in stable record-ID order, parse whitelisted `values_json` in the process and aggregate tenant-scoped counts without creating tables. `prepare` writes target coordinates into `platform_sensitive_migration_targets`; the table contains run/resource/tenant/record/field coordinates and hashes but no protected values. Apply and rollback use that sealed target set for consistent tenant-scoped keyset traversal across MySQL, PostgreSQL and SQLite. Global resources use the stable `platform:global` sentinel.

A checkpoint is unique by run, resource, tenant-scope hash and mode. It records the last processed record ID, expected platform revision, counts, status and event-chain head. The raw tenant scope may be retained only inside the migration database for resume queries; reports expose its SHA-256 domain-separated hash.

Batch size defaults to 100 and is limited to 1 through 1000. Resume starts after the committed cursor. Replaying an already committed batch is idempotent.

## Transaction And Concurrency Model

Every mutating batch runs in one database transaction:

1. lock and read the global admin-resource revision;
2. compare it with the checkpoint's expected revision;
3. load the selected rows and verify each original `values_json` snapshot;
4. protect eligible plaintext through `Runtime.Protect`;
5. write encrypted rollback escrow and the row update;
6. append sanitized migration events and advance the checkpoint;
7. compare-and-swap the global revision to the next value;
8. commit all changes together.

Row updates include the original `values_json` in their predicate. A concurrent row change or revision mismatch aborts the batch without advancing escrow, events or checkpoint state. Maintenance-window confirmation is still mandatory because the ordinary GORM Store rebuilds normalized tables during snapshot saves.

## Journal And Event Integrity

The dedicated journal consists of:

- `platform_sensitive_migration_runs` for immutable approvals and run status;
- `platform_sensitive_migration_checkpoints` for resumable resource/tenant cursors;
- `platform_sensitive_migration_events` for append-only batch and mode events;
- `platform_sensitive_migration_escrow` for encrypted originals and post-migration value hashes;
- `platform_sensitive_migration_targets` for the sealed, value-free target coordinates prepared for a run.

Events contain run ID, sequence, mode, resource, tenant-scope hash, counters, prior event hash, current event hash and timestamps. The hash is SHA-256 over canonical metadata with a fixed domain prefix. Events never contain values, ciphertext, keys, nonces, AAD, blind indexes, DSNs or PII.

The journal schema is created only by explicit `prepare`. Existing journal schema is validated before use; automatic migration must not alter ordinary admin-resource tables. Inventory, dry-run and verify never call `AutoMigrate`.

## Rollback Escrow

Before a plaintext value is replaced, the original is encrypted with the active data-protection key under a reserved context:

- tenant ID: the target record tenant scope;
- resource: `migration-rollback`;
- record ID: run ID plus a domain-separated hash of target resource, record and field;
- field key: `original-value`;
- schema version: 1;
- policy: AES-256-GCM v1, raw normalization and no blind index.

Escrow cannot be substituted for the target field because its AAD context is different. The escrow row stores target coordinates and a SHA-256 hash of the post-migration envelope, but not the envelope itself.

`rehearse-restore` calls `Reveal` for every escrow row using the reserved context, immediately discards the result and records counts only. `rollback` additionally proves that the current target value hash matches the escrow row before restoring it. Any post-migration application edit causes a conflict and is not overwritten.

External database backup and restore evidence remains mandatory. Escrow is a precise rollback aid, not a substitute for a tested database backup.

## Failure And Output Rules

- Stop a mutating batch on the first malformed/foreign envelope, revision conflict, row conflict, missing key, journal error or escrow error.
- Persist only the previous committed checkpoint; never report an uncommitted cursor as durable.
- Return stable error categories without including argument values, DSNs, record IDs, field values or database payloads.
- Command output is one JSON object containing run ID, mode, status, aggregate counts, completed checkpoints, remaining classifications and event-chain head.
- Standard error contains only normalized operational errors.
- No logger or report may receive plaintext, ciphertext, keys, nonces, AAD, blind indexes or PII.

## Acceptance

- Arbitrary manifest-declared sensitive fields migrate without field-name heuristics.
- Contract validation rejects every encrypted field outside `values` storage.
- Inventory and dry-run do not create tables or change the platform revision.
- Prepare creates only dedicated migration tables, rejects duplicated physical projections and seals a value-free target set.
- Apply is idempotent, bounded, tenant-scoped, resumable and revision-safe.
- Plaintext is absent from migrated `values_json`, journals, command output and error text.
- Foreign or malformed envelopes are never double-encrypted.
- Batch row updates, escrow, events, checkpoints and revision CAS commit atomically.
- Verify reconciles all target envelopes and journal counts.
- Restore rehearsal proves every escrow value can be decrypted without output.
- Rollback restores unchanged migrated fields and refuses fields edited after migration.
- MySQL and PostgreSQL integration evidence is required before production promotion; SQLite tests alone do not certify production migration.
- No migration HTTP route exists.
