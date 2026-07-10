# Production Auth Hardening And Codegen Skeleton Design

## Goal

Finish the reusable platform foundation without changing the default auth runtime or enabling source-writing code generation.

## Decisions

1. Production auth hardening is complete when the foundation has JWT plus server-side sessions, GORM-backed production stores, Redis-backed invalidation as an optimization, provider redaction controls, provider production guardrails, credential rotation policy, non-mutating promotion review packets and machine-checked approval evidence schemas.
2. The default `POST /api/auth/refresh` runtime remains sliding session renewal. It must not become a refresh-token-family endpoint in this task.
3. Refresh-token-family storage, rotation, reuse detection, revocation and audit redaction stay implemented but disabled by default. Future production enablement remains a separate promotion gate with external evidence and rollback requirements.
4. Source-writing codegen is not implemented in this task. The foundation only keeps scaffold preview, generated draft artifacts, a promotion review packet, target-family mappings, explicit runtime-target policy and validators that keep runtime mutation disabled.
5. Completion audits must no longer treat disabled refresh-token-family runtime or disabled source writing as incomplete platform-foundation work. They remain future promotion gates, not blockers for this foundation goal.

## Contracts

The task graph will mark `production-auth-provider-hardening` and `source-writing-codegen-promotion` as implemented only in the foundation sense described above. The wording of each node must make clear that runtime promotion remains non-mutating and manually reviewed.

The goal-completion and objective-conformance audits will move from `not-complete-controlled` to `complete` only if all task graph nodes are implemented and the promotion gates remain non-mutating.

## Verification

Focused validation must prove:

- refresh-token-family default runtime is still disabled;
- source-writing mode is still disabled;
- promotion review packets remain dry-run and not approved;
- completed platform status does not depend on external approval evidence that is only required for future runtime mutation;
- generated scaffold artifacts are review artifacts, not source writes.
