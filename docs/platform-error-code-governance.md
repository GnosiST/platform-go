# Platform Error-Code Governance

The platform uses `internal/platform/errorcode` as the sole registry for public error codes. A code is stable within its major contract version and carries its owner, exposed planes and audiences, category, HTTP mapping, retry policy, redaction class, public message and lifecycle metadata.

## Runtime Contract

HTTP handlers return the registry-backed `Response.Error` envelope through the shared error writer. The envelope contains the registered code and message plus opaque request and trace identifiers. Internal causes are available only to the internal error sink; schema names, credentials, PII and adapter messages are never copied into a public response or persisted audit field. `Retry-After` is emitted only for rate-limit decisions with a bounded delay.

Admin, App, Service, Control and External contracts consume the same generated registry. The generated OpenAPI documents and Go/TypeScript error SDKs are derived from the registry and must not define a second code list.

## Adding Or Changing A Code

1. Choose an owner and an existing category/plane boundary.
2. Add one definition to `internal/platform/errorcode/builtin.go` with an `introducedIn` version and a safe public message.
3. Regenerate `resources/generated/platform-error-code-contract.json`, the error SDKs and OpenAPI artifacts.
4. Add a focused response test covering status, retry behavior, correlation and redaction.
5. Run the registry validator and source-coverage test before review.

Existing owner, plane, audience, category, status, retry, redaction and introduced-version fields are immutable within the major version. Removing a public code requires a retained deprecated definition with a valid sunset date and replacement code. Duplicate codes, replacement cycles, generated hash drift, unregistered literals and direct error-envelope construction fail validation.

## Operations And Lookup

Use the response `requestId` and `traceId` to correlate an incident with internal error and audit records. These identifiers are opaque and generated server-side when an inbound context is absent or invalid. The registry does not expose SQL, physical datasource names, raw causes or secrets. Event-plane propagation and OpenTelemetry export remain separate service-contract work; the current HTTP implementation still validates and preserves W3C trace context at the request boundary.

Focused checks:

```bash
rtk go test ./internal/platform/errorcode ./internal/platform/httpapi -count=1
rtk node --test scripts/platform-error-code-registry.test.mjs
rtk node scripts/validate-platform-error-code-registry.mjs
```
