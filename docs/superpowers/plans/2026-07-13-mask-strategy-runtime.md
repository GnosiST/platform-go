# Mask Strategy Runtime Implementation Plan

## Goal

Close `mask-strategy-runtime` with configurable backend masking, consistent projections, Admin update safety and synchronized governance evidence.

## Test Seams

1. `masking.Runtime` public policy validation and masked output.
2. `capability.Validate` manifest acceptance and rejection.
3. `adminresource.Store` response, query and export projection, update preservation and mutation atomicity.
4. Admin contract/OpenAPI/TypeScript propagation.
5. `GenericResourceConsole` update payload and display contract.

## Steps

1. Add failing masking runtime tests for the five strategies, Unicode, short values and invalid policies.
2. Add failing capability and Admin resource tests for arbitrary encrypted masked fields and fail-closed projection.
3. Implement the masking policy contract and runtime, then integrate it with encrypted response/export projection.
4. Remove duplicate query and policy-review export projection while preserving authorization and audit behavior.
5. Propagate masking metadata through generated contracts, OpenAPI and Admin TypeScript types.
6. Keep encrypted edit fields blank, send only externally writable public values in status updates, preserve unsubmitted stored values and add localized persistent helper text.
7. Preflight mutation response projection before state or persistence changes, deep-clone masking metadata and keep Go/JS replacement validation aligned.
8. Mark the task node implemented, add closeout and engineering evidence, update current status documentation and run focused then full verification.
9. Run independent review, repair findings, refresh CodeGraph, commit and leave the worktree clean.
