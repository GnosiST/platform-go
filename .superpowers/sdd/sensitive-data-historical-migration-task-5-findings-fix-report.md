# Sensitive Data Historical Migration Task 5 Findings Fix Report

## Findings Closed

- Restore rehearsal now requires the complete mutation approval, backup, restore-evidence, and maintenance request and matches those immutable values against the prepared run.
- Rehearsal evidence is bound to a canonical SHA-256 hash over the sorted escrow coordinates, protected originals, and migrated-value hashes.
- The runner computes the escrow-set hash only after every escrow value reveals successfully. The store reloads and recomputes the set inside the rehearsal transaction before appending the event.
- Rehearsal events persist `escrow_set_hash`, include it in the event-chain hash, and reject duplicate or mismatched rehearsal evidence.
- Repeated rehearsal reloads, reveals, hashes, and recommits the complete set. It no longer returns from cached run state alone.
- Rehearsal and rollback `StartOrResume` reconcile the live escrow set against apply plaintext counts and persisted rehearsal evidence. Deleted or replaced escrow fails closed even after rehearsal or rollback completed.
- Apply and rollback row CAS now share driver-specific byte-exact predicates: MySQL `BINARY`, PostgreSQL text equality, and SQLite BLOB casts.
- Nil runner contexts return the normalized read failure before runtime readiness is called.

## TDD Evidence

The focused RED run failed on the missing canonical escrow-set hash, count-plus-hash rehearsal commit contract, and exact driver predicate. Additional tests captured incomplete rehearsal approvals, immutable approval mismatch, escrow substitution between reveal and commit, completed-mode escrow deletion, repeated full reveal, MySQL binary SQL, and nil-context readiness ordering.

GREEN verification:

- `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource -run 'Test.*(NilContext|Rehearse|Rehearsal|EscrowSet|CompletedModes|ExactPredicate)' -count=1`
- `rtk go test ./internal/platform/sensitivemigration ./internal/platform/adminresource ./internal/platform/dataprotection -count=1`
- `rtk go test -race ./internal/platform/sensitivemigration ./internal/platform/adminresource -count=1`
- `rtk go test ./...`
- `rtk go vet ./...`
- `rtk git diff --check`

SQLite remains local rehearsal evidence. MySQL and PostgreSQL integration evidence is still required before production promotion.
