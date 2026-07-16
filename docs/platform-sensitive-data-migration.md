# Sensitive Data Historical Migration Runbook

This runbook is for the offline `platform-admin sensitive-data-migrate` maintenance command. It migrates historical values only when an enabled capability manifest declares the field as encrypted with `Source="values"`. It does not change the ordinary Store plaintext boundary, add an HTTP endpoint or run automatically at API startup.

## Promotion And Driver Policy

- MySQL and PostgreSQL are production targets, but promotion requires real integration rehearsal and certification evidence for the selected driver and version. Unit tests or SQLite evidence cannot certify either production target.
- SQLite is limited to development or test local rehearsal. The bootstrap rejects SQLite in staging and production before storage access.
- Oracle and Kingbase are not certified by this node. A separate reviewed adapter, transaction model and integration evidence are required before either can be considered.
- File storage and legacy SQL mutation modes are rejected. The maintenance command opens only the GORM drivers listed above.
- Encrypted escrow does not replace an external backup or tested external restore. Escrow supports field-level guarded rollback after apply; the external database backup remains the disaster-recovery boundary.

## Required Evidence

Before inventory begins, the operator must create an external database backup and complete an isolated restore rehearsal. Record these immutable inputs in the change ticket and retain them for every mutating invocation:

- canonical migration run ID;
- operator actor ID and approved reason;
- approval reference;
- external backup URI and canonical SHA-256 backup hash;
- restore evidence reference from the isolated rehearsal;
- maintenance-window confirmation;
- target driver, server version, schema revision and reviewed commit;
- driver-specific integration rehearsal and certification evidence for production promotion.

Do not paste database credentials, DSNs, field values, record IDs, tenant identifiers, ciphertext, keys, nonces, AAD, blind indexes or PII into commands, tickets, reports or logs. Supply approved values through the operator's secret and change-management environment.

## Command Contract

The command accepts exactly seven modes: `inventory`, `dry-run`, `prepare`, `apply`, `verify`, `rehearse-restore` and `rollback`. The examples below use environment references only; they intentionally contain no sample secret, DSN, PII or encrypted value.

Read-only inventory:

```bash
rtk go run ./cmd/platform-admin sensitive-data-migrate \
  --mode inventory \
  --batch-size "$MIGRATION_BATCH_SIZE"
```

Read-only dry-run:

```bash
rtk go run ./cmd/platform-admin sensitive-data-migrate \
  --mode dry-run \
  --batch-size "$MIGRATION_BATCH_SIZE"
```

Prepared-run modes use the same immutable evidence fields:

```bash
rtk go run ./cmd/platform-admin sensitive-data-migrate \
  --mode "$MIGRATION_MODE" \
  --run-id "$MIGRATION_RUN_ID" \
  --actor "$MIGRATION_ACTOR_ID" \
  --reason "$MIGRATION_REASON" \
  --approval-ref "$MIGRATION_APPROVAL_REF" \
  --backup-uri "$MIGRATION_BACKUP_URI" \
  --backup-sha256 "$MIGRATION_BACKUP_SHA256" \
  --restore-evidence-ref "$MIGRATION_RESTORE_EVIDENCE_REF" \
  --maintenance-window-confirmed \
  --batch-size "$MIGRATION_BATCH_SIZE"
```

`verify` is read-only and requires only the prepared run identity:

```bash
rtk go run ./cmd/platform-admin sensitive-data-migrate \
  --mode verify \
  --run-id "$MIGRATION_RUN_ID" \
  --batch-size "$MIGRATION_BATCH_SIZE"
```

Successful output is one JSON report with run ID, mode, status, aggregate counts, checkpoint count and event-chain head. A successful run report is written before storage close; if close then fails, the report remains the single JSON object and the command returns a normalized close error. Standard error and failed-connection logging remain value-free; GORM initialization uses a silent logger.

## Execution Sequence

1. **Freeze and external backup.** Enter the approved maintenance window, stop ordinary writers as required by the change plan, create the external backup, verify its SHA-256 digest and complete an isolated restore. If backup or restore evidence is incomplete, stop.
2. **Inventory.** Run `inventory` and store only its value-free JSON counts. It does not create migration tables, mutate rows or advance the platform revision.
3. **Dry-run.** Run `dry-run` through the same manifest-derived plan, tenant traversal, policy validation and bounded batch rules used by apply. It does not create migration tables or mutate rows.
4. **Prepare.** Run `prepare` with the immutable approval, backup and restore evidence. `prepare` is the only mode that may create the five dedicated journal tables. It validates physical layout, seals the plan hash and stores value-free target coordinates. It does not change protected business values.
5. **Apply.** Run `apply` with exactly the same run ID and immutable evidence. Apply never calls `AutoMigrate`. Each committed batch atomically couples the row update, encrypted escrow, append-only event, checkpoint and global revision compare-and-swap.
6. **Verify.** Run `verify` for the prepared run. Continue only when all non-empty targets validate under the active manifest and journal counts reconcile.
7. **Rehearse restore.** Run `rehearse-restore` with the immutable evidence. The command decrypts escrow in memory, discards it immediately and reports counts only.
8. **Rollback decision.** If rollback is approved, run `rollback` with the same evidence. Hash guards reject any target changed after migration; conflicts are never overwritten. After rollback, run `verify` and complete the external restore procedure if the incident plan requires it.

## Resume And Downtime

Resume a stopped `apply`, `rehearse-restore` or `rollback` by rerunning the same mode with the same run ID, plan, approval, backup and restore evidence. The runner resumes only after the last committed tenant-scoped cursor. Never invent a new run ID to bypass a conflict. Keep ordinary writers stopped for every mutating mode because the ordinary GORM Store can rebuild normalized tables during snapshot saves.

An operator-initiated stop is safe only between committed batches. Termination during a batch preserves the previous committed checkpoint; treat the current batch as uncommitted until the next invocation proves otherwise. Keep the maintenance window active until verify and the selected restore or rollback checks complete.

## Incident Stop Conditions

Stop the run, preserve the value-free report and escalate to the database, security and platform owners on the first occurrence of any of the following:

- malformed or foreign encrypted envelope classification;
- missing key material, policy mismatch or manifest/plan hash drift;
- row snapshot conflict, global revision conflict or unexpected active writer;
- journal, escrow, checkpoint or event-chain failure;
- backup digest mismatch or restore evidence loss;
- driver/version mismatch with the approved rehearsal evidence;
- any plaintext, encrypted value, DSN, record ID, tenant identifier or PII observed in output or logs;
- verify reconciliation failure, restore rehearsal failure or rollback hash conflict;
- maintenance-window expiry or loss of required operator approvals.

Do not delete journal rows, edit checkpoints, disable hash guards, retry with changed immutable evidence or fall back to file/legacy SQL mutation. Preserve the database and external backup state for incident review.

## Implementation Facts

The historical migration design is intentionally deferred from the default
runtime path: it is an operator-controlled, approval-gated maintenance
workflow and does not change ordinary request handling.

- The plan is derived from every enabled manifest-declared encrypted field with `Source="values"`; no field-name heuristic is used.
- Inventory, dry-run and verify do not call `AutoMigrate`; `prepare` is the sole journal-creation mode.
- The journal stores encrypted escrow and value-free coordinates, counts, hashes and event metadata. Reports never expose values.
- Rollback uses the recorded post-migration value hash and refuses application edits made after migration.
- The maintenance command has no HTTP route and does not alter ordinary Store behavior.
