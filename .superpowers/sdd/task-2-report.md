# Task 2 Report: Correlation Context And One Error Response Writer

## Status

Implemented and verified in commit `b5d96cd` (`feat: unify error response correlation`).

## Scope

- Added cross-plane `kernel.Correlation` context helpers.
- Added server-owned request IDs and strict W3C version `00` traceparent handling.
- Added one registry-backed HTTP error writer with unknown-code fallback.
- Replaced `gin.Recovery()` with a safe recovery middleware.
- Routed global request-body invalid/too-large failures through the registry writer.
- Extended the existing internal error event with safe registry metadata and server correlation.
- Added a narrow legacy response-body helper for unmigrated auth, file and resource helpers required to preserve current behavior during Tasks 3-4.
- Did not change organization runtime/UI or deferred datasource, XA, MQ, outbox or search nodes.

## RED Evidence

Command:

```text
rtk go test ./internal/platform/kernel ./internal/platform/httpapi -run 'Test.*(Correlation|ErrorResponse|Recovery)' -count=1
```

Observed expected build failures:

- `undefined: Correlation`
- `undefined: CorrelationFromContext`
- `undefined: WithCorrelation`
- `undefined: requestCorrelation`
- `undefined: writePlatformError`
- `ErrorBody` missing `RequestID` and `TraceID`

This failed because the Task 2 contracts did not exist, before production implementation was added.

## GREEN Evidence

Focused behavior:

```text
rtk go test ./internal/platform/kernel ./internal/platform/httpapi -run 'Test.*(Correlation|RequestBody|Recovery|ErrorResponse)' -count=1
Go test: 20 passed in 2 packages
```

Complete affected packages:

```text
rtk go test ./internal/platform/kernel ./internal/platform/httpapi -count=1
Go test: 346 passed in 2 packages
```

Full Go repository:

```text
rtk go test ./... -count=1
Go test: 1828 passed in 35 packages
```

Registry governance:

```text
rtk node --test scripts/platform-error-code-registry.test.mjs
19 passed
rtk node scripts/validate-platform-error-code-registry.mjs
platform error-code registry valid: 117 definitions
```

Task graph governance:

```text
rtk node --test scripts/platform-foundation-task-graph.test.mjs
41 passed
rtk node scripts/validate-platform-foundation-task-graph.mjs
Validated 67 platform foundation task nodes
```

Repository checks:

- `rtk git diff --check`: passed.
- `rtk codegraph sync .`: synchronized 11 changed files.
- `rtk codegraph status`: index up to date.

## Files

Created:

- `internal/platform/kernel/correlation.go`
- `internal/platform/kernel/correlation_test.go`
- `internal/platform/httpapi/request_correlation.go`
- `internal/platform/httpapi/request_correlation_test.go`
- `internal/platform/httpapi/error_response.go`
- `internal/platform/httpapi/error_response_test.go`

Modified:

- `internal/platform/httpapi/response.go`
- `internal/platform/httpapi/security_headers.go`
- `internal/platform/httpapi/server.go`
- `internal/platform/httpapi/server_test.go`
- `internal/platform/httpapi/service_objects_test.go`

The service-object test now compares status/code/public message instead of byte-equal JSON because independently generated request/trace IDs intentionally differ per request.

## Security And Contract Decisions

- `X-Request-ID` is always generated as `req_` plus 32 lowercase hex characters. A client value is never reflected or used for internal event IDs.
- Only one exact lowercase W3C `00-<trace>-<parent>-<flags>` header is accepted. Duplicate, uppercase, non-00, all-zero trace ID and all-zero parent ID inputs fail closed to a new context.
- A valid incoming trace ID and flags are preserved, while a new non-zero server span ID is always generated.
- `writePlatformError` resolves status and public message exclusively from the frozen registry and falls back to `INTERNAL_ERROR` for unknown codes.
- Error responses written by the registry writer contain only `error.code`, `error.message`, `error.requestId` and `error.traceId`; no success `data` is emitted.
- `writePlatformErrorWithCause` intentionally ignores the raw cause after the call boundary. This is a security invariant, not an omission: the response, Gin metadata and `InternalErrorSink` receive only the registered code/owner/category/retry/redaction metadata plus opaque server correlation.
- Recovery never formats or forwards the panic value, stack, authorization header, request body or other request details.
- `InternalErrorEvent.Err` remains for compatibility but contains only `errorcode.New(definition.Code)`, whose rendered value is the stable code.

## Self-Review

- Middleware order is correlation, recovery, security headers, then body limit, so every server error source after construction has correlation and is covered by safe recovery.
- Registry body-limit statuses remain 400 and 413; recovery remains 500.
- Unknown typed codes fail closed to registered `INTERNAL_ERROR`.
- Traceparent parsing rejects ambiguous multiple headers and never accepts zero trace/span identifiers.
- Generated trace and span identifiers are forced non-zero even in the cryptographic-random all-zero edge case.
- Kernel context helpers reject incomplete request/trace pairs and do not read HTTP headers.
- Existing direct unit-test Gin contexts without an HTTP request remain supported and do not panic.
- No production code reads client `X-Request-ID`; the final targeted search returned no matches.

## Concerns / Follow-Up Boundaries

- Tasks 3-4 still own migration of remaining legacy string call sites to the typed registry writer. `legacyErrorBody` is deliberately limited to existing auth/file/resource helper compatibility; it must be removed when those tasks complete.
- Some direct legacy `ErrorBody` constructions outside those helpers are not migrated in Task 2. Their JSON shape includes the new fields, but full registry-backed correlation behavior remains a Task 3-4 completion gate.
- Task 6 owns durable logging/audit correlation. Task 2 only establishes a safe in-process event contract and intentionally does not retain raw causes or panic diagnostics.
