# Task 7 Report: Runtime Security Containment

Date: 2026-07-12
Branch: `codex/platform-completion`

## Outcome

Task 7 is implemented and the `runtime-security-containment` node is closed. The platform now has explicit policy-review export authorization, schema-driven response/export redaction, atomic protected mutations with audit persistence, credential compensation on audit failure, durable file deletion tombstones, stable public/runtime error reporting and an enforced no-local-password boundary.

Governance now reports `45 total / 38 implemented / 7 controlled unfinished`. The remaining nodes retain their approved order:

1. `admin-watermark-export-governance`
2. `sensitive-data-protection-runtime`
3. `sensitive-data-historical-migration`
4. `open-source-portability`
5. `public-docs-community`
6. `public-docs-site`
7. `github-release-publication`

## Implementation

- Added atomic Store APIs for create, update, delete, file tombstone/purge and redacted audit persistence in one repository snapshot.
- Generic Admin CRUD, API-token mutations and file uploads now fail closed when audit persistence fails.
- Admin/app login and Admin refresh revoke or discard newly issued credential state after token-signing or audit failure.
- File deletion stores a hidden tombstone plus request audit before idempotent object cleanup, then purges metadata with a completion audit only after cleanup succeeds.
- Policy-review export requires `admin:policy-review:export`; response and export projections omit legacy `targetCode`/`traceId` and protected fields.
- The Admin export control is omitted for unauthorized principals, preserving keyboard and focus behavior.
- Runtime error sinks and default internal responses expose stable codes/messages instead of wrapped backend errors.
- API startup rejects local password providers and generic password fields. Future local-password support requires a separately approved Argon2id capability and dedicated migration boundary.
- Updated auth, resource schema, deployment and capability-development documentation.
- Regenerated Admin resource contracts, OpenAPI, codegen preview and scaffold review artifacts.

## Independent Review Remediation

- Admin refresh now persists an `allowed` refresh-attempt audit before renewing the caller-known session. Audit failure leaves the existing session expiry and revocation state unchanged and does not depend on cleanup compensation.
- Admin actors use principal user IDs, API-token calls use the API-token record ID, App actors and file owners use a domain-separated opaque SHA-256 ID, and unauthenticated/system events use `system:platform`. Legacy `app:<username>` ownership remains read-compatible without being written by new uploads.
- Authentication, App phone and file-content audit paths now use the validated `AuditEvent` builder. Unknown audit resources and repository failures fail protected operations; file-content audits complete before response headers or bytes are written.
- OIDC binding audits also use the same approved field set and deterministic opaque event IDs. Provider IDs, usernames, provider subjects, target codes and session-derived values are not persisted in new audit records.
- Internal diagnostics now expose only a public code, a whitelisted cause class and a safe request/event correlation ID. The same structured event is attached to Gin error metadata and sent to the internal sink; raw adapter errors, request IDs, paths, URLs, filenames and personal values are omitted.
- Added independent rollback coverage for Store update/delete audit failures, issued-session cleanup failures, purge-audit retry recovery and durable tombstone/storage-key preservation.
- Updated the production-auth hardening contract and validator so the approved audit field set and refresh-before-renew ordering are enforced rather than the legacy username/provider audit shape.
- Final review split policy-review identity semantics: `requestedBy`, `reviewedBy` and `exportedBy` remain `users.code` values, while their audit records use opaque principal IDs. App file-content audits now hash email-shaped usernames before persistence/export.
- Internal error cause classification now uses an explicit ordered priority, so compound codes such as `AUTH_PROVIDER_UNAVAILABLE` and `PROVIDER_UNAVAILABLE` have deterministic classes.

## TDD Evidence

The focused RED run initially reported 7 expected failures: export reused read permission, legacy audit fields leaked, generic mutation survived audit failure, file mutation survived audit failure, failed deletion stayed visible, runtime error sinks exposed backend errors and the credential boundary was missing.

Focused GREEN coverage includes export/redaction, audit rollback, file cleanup/tombstone retry, runtime error sanitization, credential-boundary validation, app-login/session compensation, Admin refresh compensation and API-token audit-failure rollback.

Final review RED coverage reproduced four issues: raw App file-content actor persistence, policy-review `reviewedBy` drift, policy-review `requestedBy` drift and nondeterministic compound internal-error classification. The corresponding seven focused App/policy/security tests passed after the identity split and ordered classification fix.

Governance mutation coverage rejects:

- missing `runtime-security-containment` future-to-required migration;
- stale `45/37/8` counts;
- reordered remaining seven nodes;
- missing runtime-security closeout evidence;
- regression of the runtime-security engineering capability to `partial`.

## Verification

- `rtk go test ./...`: 874 passed in 24 packages.
- `rtk go vet ./...`: no issues.
- Explicit Node suites passed, 385 tests across 18 suites:
  - Admin resource generators: 13.
  - Admin UI contracts: 63.
  - Admin API boundary: 8.
  - Admin resource validator tests: 40.
  - File storage experience: 6.
  - Foundation task graph: 26.
  - Task execution audit: 21.
  - Goal completion audit: 17.
  - Node closeout audit: 8.
  - Objective conformance: 19.
  - Engineering capabilities: 35.
  - Deployment topology: 34.
  - Production environment: 19.
  - Production readiness: 36.
- Production auth hardening: 34.
- Platform foundation docs drift: passed.
- Final review relevant Node suites: 53 passed across production-auth hardening, foundation docs drift and Admin resource generators.
- All Task 7 brief validators passed, including Admin resources, Admin API boundary, deployment topology, production readiness/env, production auth hardening, foundation alignment, task execution, goal completion, node closeout, objective conformance, engineering capabilities, file-storage experience, Admin i18n and Admin UI contracts.
- `rtk npm --prefix admin run build`: TypeScript and Vite production build passed; 3759 modules transformed.
- The final review fix changed only Go backend/tests and a validator; Admin build was not rerun for that small fix because no UI or generated frontend contract changed.
- `rtk git diff --check`: passed.
- `rtk codegraph sync .` and `rtk codegraph status`: index is up to date.

## Boundary Review

- No refresh-token-family runtime was enabled.
- No source-writing mode was enabled or approved.
- Marker review found no raw adapter errors, password/token values, personal values, session handles/digests or physical object paths added to responses, audits or runtime error sinks.
- Generated artifact changes are derived from the approved Admin contract generation chain; the scaffold promotion review remains review-only.
- README did not require a history-style update; existing production and completion-program guidance already routes readers to the authoritative documents.
