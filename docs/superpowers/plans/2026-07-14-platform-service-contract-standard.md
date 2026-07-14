# Platform Service Contract Standard Implementation Plan

> **Status:** Implemented and verified. This node owns declarations, validation, deterministic artifacts and compatibility gates; it does not implement service execution, message delivery or datasource routing.

**Goal:** Close `platform-service-contract-standard` with one executable, business-neutral contract shared by Admin, Service/Data, Control, External/Partner and Event planes.

**Architecture:** `capability.Manifest` gains a zero-value-compatible service surface. The capability registry validates service declarations together with existing Admin/App surfaces. The contracts command projects enabled manifests into a canonical service document, then deterministic generators produce OpenAPI, AsyncAPI and isolated Go/TypeScript SDK artifacts. A compatibility validator compares canonical contracts and enforces deprecation rules. Runtime identity protocols, Query/Command execution, Outbox/MQ and datasource routing remain separate later nodes.

**Constraints:**

- Keep classification, profile and replaceability metadata in `resources/platform-capability-contracts.json`; only executable service declarations belong in the runtime manifest.
- Trusted tenant context uses string identifiers and explicit provenance. Clients cannot supply DSN, datasource, database, schema or shard selection.
- Management-user and workload identities are distinct contract modes. OAuth2 client credentials, mTLS and workload JWT are declared/evaluated modes, not claimed runtime implementations.
- W3C trace fields and versioned event envelopes are schema-level contracts here. Runtime extraction, propagation, persistence and delivery stay deferred.
- Generated SDK files stay under `resources/generated/service-sdk/`; global runtime source-writing remains disabled.
- The reference capability is business-neutral and marked contract-only wherever no runtime binding exists.
- Prefix shell commands with `rtk`, use `apply_patch` for edits and refresh CodeGraph after structural changes.

## Task 1: Lock The Go Service Contract

- [x] Add typed plane, audience, stability, auth, tenant, operation, schema, reliability, compatibility and event-envelope declarations.
- [x] Add service validation to registry resolution without changing manifests that omit the new surface.
- [x] Cover duplicate IDs, invalid enum combinations, missing tenant provenance, unsafe routing fields, PII omissions and event-version rules.

## Task 2: Add One Reference Capability

- [x] Add a business-neutral reference service surface to `file-storage`.
- [x] Reuse actual App upload/content routes only where runtime parity exists.
- [x] Mark service-only command/query/event declarations as contract-only; do not claim handlers or event delivery.

## Task 3: Generate Canonical Contracts And API Artifacts

- [x] Add `platform-contracts service-manifests` with deterministic canonical JSON output.
- [x] Generate Service/Data, Control and External OpenAPI documents plus Event Plane AsyncAPI.
- [x] Include TrustedTenantContext, W3C trace context and versioned event envelope schemas.

## Task 4: Generate Isolated SDK Artifacts

- [x] Generate Go and TypeScript SDK source under `resources/generated/service-sdk/`.
- [x] Compile/type-check generated SDKs in isolated temporary modules.
- [x] Keep current source-writing readiness mode disabled.

## Task 5: Add Consumer And Compatibility Gates

- [x] Add positive and negative consumer fixtures.
- [x] Reject stable service downgrade; operation/event removal; identity, HTTP, schema, auth, tenant, permission and data-scope drift; and deprecation without a sunset window.
- [x] Keep Query/Command execution, rate-limit storage and idempotency persistence out of scope.

## Task 6: Document And Close Governance

- [x] Add the service contract standard documentation and generation/verification commands.
- [x] Move the task to implemented only after all artifacts and validators pass.
- [x] Update task, capability, execution, goal, closeout, objective and alignment projections to `66/45/21` and `57 = 38 implemented + 19 partial`.
- [x] Preserve the activated topology specification as historical `66/44/22` evidence.

## Task 7: Review And Verify

- [x] Run focused Go and Node tests, existing Admin/App generator tests and capability/API boundary validators.
- [x] Run shared governance alignment suites, `git diff --check`, CodeGraph sync/status and an independent review.
- [x] Run one phase-level neat-freak cleanup after code, artifacts and docs stabilize, then commit and confirm a clean worktree.
