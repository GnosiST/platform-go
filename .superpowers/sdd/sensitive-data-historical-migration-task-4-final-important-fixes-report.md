# Sensitive Data Historical Migration Task 4 Final Important Fixes Report

## Result

DONE. The completed-run verification bypass and duplicate top-level JSON key ambiguity are fixed without HTTP or CLI changes.

## Fixes

- `FinishRun` now validates the full event/checkpoint journal and processed target reconciliation before returning success for both prepared and already-completed runs.
- Added `sensitivemigration.DecodeUniqueObject`, a token-based top-level JSON object decoder that preserves `json.RawMessage` values and rejects duplicate keys, trailing values and non-object roots with a value-free error.
- Runner apply, verify and read-only classification use the unique-object decoder before protection, validation or remarshal.
- Admin-resource tenant parsing and target-only change authorization use the same decoder.
- Duplicate non-target keys now fail closed before `Protect`, store mutation, checkpoint or event writes.

## RED Evidence

- A completed run with a tampered checkpoint was accepted by `FinishRun`.
- A duplicate non-target key was collapsed by map decoding and direct batch apply succeeded.
- Decoder tests failed to compile because no reusable unique-object decoder existed.

## GREEN Verification

- Focused regressions: 9 passed.
- Affected package suites: 263 passed.
- Race detector: 216 passed.
- Full repository: 1096 passed across 26 packages.
- `rtk go vet ./...`: no issues.
- `rtk git diff --check`: clean.
- CodeGraph sync/status: index up to date.

## Concerns

None within Task 4.
