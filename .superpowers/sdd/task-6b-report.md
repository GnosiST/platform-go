# Task 6B Report: Generic Audit Correlation And Admin Contract

## Status

Complete. Every request-owned generic audit producer now persists the exact server-owned request and trace identifiers from its authenticated request context. Background generic audit producers continue to use Task 6A's fail-closed kernel correlation generator. Specialized organization, RBAC, and menu audit storage was not changed.

## Producer Coverage

- Admin login, refresh, and logout plus App login, logout, and phone binding use the request-aware generic audit helper.
- Admin and App file upload/content/delete/restore paths use request-aware mutation or file audit events.
- API-token issue, update, and revoke propagate the request context through their transactional audit writes.
- Generic Admin create, update, delete, and restore mutations preserve their existing event identifiers while persisting the exact request/trace pair.
- Policy-review request, approve, reject, and export expose context-aware store methods; the existing context-free methods remain compatible and generate background correlation.
- Admin identity binding attaches request correlation without changing its idempotent audit ownership.
- Lifecycle, retention, and file-cleanup producers without request context receive a generated valid opaque pair through `auditRecordLocked`.

## Contracts And Generation

- The static audit resource now has hidden `legacyTraceId` and canonical internal `requestId`/`traceId` fields with read-only response, omitted export, search/filter, fixed length, and regex constraints.
- Generic OpenAPI field generation now projects `minLength`, `maxLength`, and `pattern` from field validation metadata.
- Admin resource contract, OpenAPI, and codegen preview were regenerated deterministically.
- `admin-scaffold-draft.md` required a six-line transitive refresh because its audit field/search/filter counts derive from the codegen preview; no scaffold source or unrelated generated artifact changed.

## Verification

- Focused correlation/audit/migration Go suite: 130 passed in 3 packages.
- Full target Go packages: 722 passed in 3 packages.
- Whole repository Go suite: 2001 passed in 36 packages.
- Admin resource generator suite: 29 passed.
- `validate-admin-resources.mjs`: 25 resources validated.
- `validate-platform-admin-api-boundary.mjs`: passed.
- Admin capability contract, resource contract, OpenAPI, codegen preview, and scaffold draft each matched a fresh stdout generation byte-for-byte.
- `git diff --check`: passed.
- CodeGraph: current at 446 files and 12,830 nodes.

## Residual Concerns

- The generic Admin GORM repository still uses its existing normalized snapshot rewrite behavior on ordinary saves. Task 6A preserves audit identity, legacy values, and canonical columns across that round trip; Task 6B does not redesign persistence.
- Context-free policy-review compatibility methods intentionally generate a new background correlation pair. HTTP request paths use the context-aware variants and preserve the exact request pair.
