# Sensitive Data Historical Migration Task 4 Important Fixes Report

## Result

DONE. All three Important review findings are fixed without adding HTTP or CLI scope.

## Fixes

1. Canonical plan identity
   - Added `sensitivemigration.PlanHash`, a deterministic SHA-256 over ordered JSON containing every resource, scope, tenant field, schema version, field key and complete protection policy.
   - Runner derives and retains the canonical plan hash and rejects prepare, apply or verify requests before store dispatch when the caller hash differs.
   - GORM prepare and resume independently recompute the request plan hash.
   - Prepared targets persist a resource-plan hash, and target reads plus batch apply reject policy/AAD plan changes.

2. Event-bound checkpoints
   - Events persist and hash-bind the committed last record ID.
   - Journal verification validates the full event chain, reconstructs cumulative per-scope apply state from events, and requires exact checkpoint equality for cursor, rows, counts, batches, revision, event sequence and event hash.
   - Missing, extra or tampered checkpoints fail resume and finish.
   - Apply verifies journal/checkpoint consistency before and after appending each transactional batch.

3. Number-preserving JSON change authorization
   - `migrationChangedFields` now compares trimmed `json.RawMessage` bytes instead of decoding numbers through `float64`.
   - Non-target integer changes above 2^53 are rejected and rolled back.

## RED Evidence

- `PlanHash` tests initially failed to compile because no canonical hash existed.
- Eight event/checkpoint tests failed: missing event cursor, accepted cursor/count/hash tampering, accepted missing/extra checkpoints and accepted forged skipped-target completion.
- The >2^53 non-target mutation test failed because apply succeeded instead of returning a conflict.

## GREEN Verification

- Combined focused review regressions: 18 passed.
- Affected package suites: 254 passed across migration, admin-resource and data-protection.
- Race detector: 207 passed across migration and admin-resource.
- Full repository: 1087 passed across 26 packages.
- `rtk go vet ./...`: no issues.
- `rtk git diff --check`: clean.
- CodeGraph sync/status: index up to date.

## Concerns

None within Task 4.
