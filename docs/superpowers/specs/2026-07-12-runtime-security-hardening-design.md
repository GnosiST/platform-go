# Runtime Security Hardening Design

## Goal

Close current high-risk runtime boundaries before adding field-level encryption or publishing the repository.

## Current Risks

- Generic `Record.Values` accepts undeclared keys, persists them as plaintext JSON and returns them except for narrow API-token sanitization.
- App phone and verification-code hashes are unkeyed SHA-256 values, and the local `debugCode` response is not rejected by production configuration.
- App file metadata persists a raw session token and physical storage details.
- Session repositories use the raw server-side session token as a persisted lookup key.
- The production Nginx adapter exposes `/uploads/` directly, bypassing authenticated file-content routes.
- The standard production adapter assumes an external TLS edge but does not declare or validate a complete HTTPS and trusted-proxy contract.
- Audit and export paths have no reusable field-level redaction policy.

## Design

### Schema-Allowlisted Writes

Create and update operations must accept only fields declared by the active resource schema. Unknown `Values` keys return a stable `400` validation error. A global prohibited-key classifier rejects password, token, secret, credential, verification-code, provider-subject and raw session material even if a malformed schema attempts to declare it without an explicit security policy.

System-written resources may use a privileged internal write path, but that path still validates the resource's declared internal fields and prohibited secret classes.

### Schema-Allowlisted Responses

Resource responses are projected from schema metadata instead of cloning the complete `Values` map. Internal-only and redacted fields never enter generic list, query, detail or export responses.

### Phone Verification

Phone lookup and verification-code values use domain-separated HMAC-SHA-256 with versioned keys. Phone hashes and verification hashes use different keys. Raw phone numbers and verification codes never enter generic resources, audits or post-binding responses.

Production runtime rejects `app-phone` unless a non-debug delivery provider is configured. `debugCode` remains available only in development and test. Existing unkeyed phone hashes cannot be converted without the original phone number; they upgrade after re-verification.

### Private File Delivery

Remove raw session tokens, physical storage paths and default public URLs from file resource metadata. File ownership uses stable user and tenant identifiers. Local objects are stored under a non-public directory and all content access flows through authorized API handlers. The standard Nginx adapter must not expose `/uploads/` directly.

S3 defaults to a private bucket and HTTPS endpoint. Server-side encryption configuration is explicit; sensitive-file deployments require SSE-KMS or a reviewed application-level encryption adapter.

### Session Storage

Persist only a domain-separated digest of the server-side session token. JWT claims may continue to carry the opaque session handle during this phase, but repositories index and compare its digest. Migration is not required for active sessions: deployment revokes existing sessions and requires reauthentication. Logs, audits and file metadata must not copy either the handle or its digest.

### Transport And Browser Security

Production configuration declares an absolute HTTPS public base URL and trusted proxy policy. Provider and object-storage endpoints must use HTTPS outside loopback development and test. The production edge redirects HTTP to HTTPS and emits HSTS, CSP, frame, MIME-sniffing and referrer headers.

Bearer tokens remain credentials. This phase preserves the current JWT client contract but adds CSP and documents localStorage residual risk; a cookie migration requires a separate CSRF and session-client specification.

### Abuse And Input Controls

Apply bounded request-body sizes to JSON and multipart endpoints. File uploads enforce configurable maximum size, sanitized filenames, allowed MIME policy and server-side content sniffing; caller-provided MIME values are not authoritative. Login, provider-start, verification-code and other credential-adjacent endpoints use keyed rate-limit dimensions that do not log raw credentials or personal values. Multi-instance production uses the existing cache/invalidation boundary or another reviewed distributed limiter instead of process-local counters.

### Audit And Export Redaction

Audit events record actor ID, resource, target ID, action, result and stable reason codes. Names, filenames and personal values are masked or replaced by stable identifiers where they are not required for investigation. Export handlers apply resource field policies and require explicit export permissions.

## Error Handling

- Unknown or prohibited fields: `400` with a stable validation code and field name, never the rejected value.
- Missing production security configuration: startup fails before stores or listeners open.
- HMAC key unavailable: startup or operation fails closed; no fallback to unkeyed hashes.
- File authorization failure: return `404` for cross-user access to avoid object enumeration.
- Oversized or disallowed input: return stable `413` or `415` responses without echoing payload content.
- Rate-limited authentication or verification: return stable `429` responses without disclosing whether an account or phone exists.
- Redaction failure: export and audit write fail closed rather than emit unfiltered data.

## Acceptance

- Undeclared and prohibited fields fail before persistence.
- Database and file snapshots contain no test plaintext secrets or raw session tokens.
- Session repositories contain only versioned session digests, and deployment invalidates pre-migration sessions.
- App-phone production configuration without a delivery provider fails startup.
- Existing development debug flows remain available only in development and test.
- `/uploads/...` is not publicly reachable; authorized API download still works.
- HTTPS and trusted-proxy production validation has positive and negative tests.
- Login and provider-start abuse controls, request limits, upload limits, filename sanitization, MIME sniffing and distributed-limiter configuration have positive and negative tests.
- Logs, audits and exports omit phone, identity number, email, detailed address, verification code, password, token and physical path test markers.
