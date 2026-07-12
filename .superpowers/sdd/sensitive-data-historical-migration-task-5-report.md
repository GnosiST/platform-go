# Sensitive Data Historical Migration Task 5 Report

## Scope

Implemented encrypted rollback escrow, restore rehearsal, and hash-guarded rollback for the offline sensitive-data migration runner and GORM maintenance store. No HTTP or CLI surface was added.

## Security Semantics

- Plaintext replacements now create a second AES-256-GCM v1 envelope under the reserved `migration-rollback` / `original-value` / schema 1 / `raw-v1` context with no blind index.
- The reserved record context binds the run ID to a domain-separated hash of target resource, record, and field coordinates. An escrow envelope cannot validate under the target field context.
- Escrow persistence accepts only platform envelope-shaped protected originals and stores the protected original plus a domain-separated SHA-256 hash of the migrated target envelope.
- Apply commits escrow, row CAS updates, chained events, checkpoints, global revision CAS, and run revision state in one transaction.
- Restore rehearsal loads and validates the complete escrow set, calls `Reveal` only with the reserved context, discards every plaintext result, and atomically appends a counts-only event plus `restore_rehearsed` evidence.
- Rollback requires the original mutation approval fields and maintenance confirmation, requires completed apply plus successful rehearsal, and verifies each current target field hash before revealing or restoring it.
- Rollback changes only escrowed target fields, retains other JSON values, rejects duplicate JSON keys, and atomically commits row CAS updates, rollback checkpoints, chained events, global revision CAS, and run revision state.
- Rehearsal and completed rollback are idempotent only after journal verification. Rollback checkpoints retain batch resume state.
- Runner reports and normalized errors contain counts and hashes only; plaintext, ciphertext, target record IDs, and protected originals are not returned.

## TDD Evidence

RED checkpoints:

- The required focused command initially failed in both packages because `EscrowContext`, `EscrowEntry`, row escrow payloads, rehearsal persistence, and rollback methods did not exist.
- A separate store-boundary test proved plaintext `ProtectedOriginal` values were accepted before envelope-shape validation was added.

GREEN checkpoints:

- `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource -run 'Test.*(Escrow|Rehearse|Rollback)' -count=1`
- `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource ./internal/platform/dataprotection -count=1`
- `rtk go test -race ./internal/platform/sensitivemigration ./internal/platform/adminresource -count=1`
- `rtk go test ./...`
- `rtk go vet ./...`
- `rtk git diff --check`

The real runner/GORM round-trip test covers apply, reveal-and-discard rehearsal, exact-field rollback, revision and event chaining, retained encrypted escrow, report redaction, and repeated rollback idempotency.

## Residual Boundary

SQLite provides local transactional rehearsal only. MySQL and PostgreSQL integration evidence remains required before production promotion, as specified by the migration design.
