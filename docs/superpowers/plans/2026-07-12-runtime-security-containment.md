# Runtime Security Containment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close current plaintext, file-bypass, transport, input and abuse-control gaps before watermark, data migration or public release work.

**Architecture:** Security enforcement is centralized behind resource-store write/projection APIs, digest-only session repositories, injected phone and rate-limit ports, private object delivery and production configuration gates. Handlers compose these ports but do not duplicate redaction or storage rules.

**Tech Stack:** Go 1.26, Gin, GORM, Redis, AWS S3 client, Node governance validators, Docker/Nginx deployment adapter.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-12-runtime-security-hardening-design.md`.
- Use test-first RED/GREEN cycles for every behavior change.
- Do not enable refresh-token-family default runtime or source writing.
- Do not persist raw passwords, tokens, provider subjects, phones, identity numbers, emails, addresses, verification codes or physical storage paths.
- Existing active sessions may be invalidated during the digest migration; do not persist compatibility plaintext.
- Original file downloads remain authorized API operations; `/uploads/` is removed.
- Prefix commands with `rtk`.

---

### Task 1: Resource Write Firewall And Projection

**Files:**
- Create: `internal/platform/adminresource/security.go`
- Create: `internal/platform/adminresource/security_test.go`
- Modify: `internal/platform/adminresource/store.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/capability/admin_contract.go`
- Modify: `internal/platform/capability/admin_contract_test.go`
- Modify: `internal/platform/core/capabilities.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `resources/admin-resources.json`
- Modify: `scripts/generate-admin-resource-contract.mjs`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `scripts/admin-resource-contract-generators.test.mjs`
- Modify: `admin/src/platform/api/client.ts`

**Interfaces:**
- Produces: `WriteOrigin`, `ProjectionPurpose`, `Store.CreateInternal`, `Store.UpdateInternal`, `Store.ProjectRecord`.
- Consumed by: generic HTTP mutations, system audit/auth/file/phone writers and policy-review export.

- [ ] **Step 1: Write failing Store tests**

```go
func TestStoreCreateAndUpdateRejectUndeclaredValuesBeforeSave(t *testing.T) {
    store := NewStore()
    _, err := store.Create("tenants", WriteInput{Name: "Tenant", Values: map[string]string{"password": "marker-secret"}})
    if !errors.Is(err, ErrInvalidRecord) { t.Fatalf("Create() error = %v", err) }
    records, _ := store.List("tenants")
    if strings.Contains(fmt.Sprint(records), "marker-secret") { t.Fatal("rejected value persisted") }
}

func TestProjectRecordDropsLegacyUnknownAndResponseOmittedValues(t *testing.T) {
    projected, err := store.ProjectRecord("app-phone-verifications", legacyRecord, ProjectionResponse)
    if err != nil { t.Fatal(err) }
    if _, ok := projected.Values["codeHash"]; ok { t.Fatal("codeHash exposed") }
}

func TestPersistBoundaryRejectsInvalidDirectSnapshotWrites(t *testing.T) {
    // exercise policy-review, identity-binding and demo-data paths that mutate the in-memory snapshot directly
}

func TestRepositoryLoadScrubsLegacyUnknownAndProhibitedValues(t *testing.T) {
    // load a legacy marker, assert the installed and rewritten snapshots omit it
}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/adminresource -run 'Test.*(CreateAndUpdateReject|InternalWrite|ProjectRecord|PersistBoundary|RepositoryLoadScrubs)' -count=1
```

Expected: FAIL because the security APIs and validation do not exist.

- [ ] **Step 3: Implement the security API**

```go
type WriteOrigin string
const (
    WriteOriginExternal WriteOrigin = "external"
    WriteOriginInternal WriteOrigin = "internal"
)
type ProjectionPurpose string
const (
    ProjectionResponse ProjectionPurpose = "response"
    ProjectionExport ProjectionPurpose = "export"
)

func (s *Store) CreateInternal(resource string, input WriteInput) (Record, error)
func (s *Store) UpdateInternal(resource, id string, input WriteInput) (Record, error)
func (s *Store) ProjectRecord(resource string, record Record, purpose ProjectionPurpose) (Record, error)
```

Add the four field-policy dimensions already approved for the later data-protection phase: `sensitivity`, `storageMode`, `responseMode` and `exportMode`. This task uses them only for write allowlisting and projection; encryption, blind indexes and privileged decryption remain in the later sensitive-data runtime node. Defaults preserve current public/plain/full behavior for ordinary declared fields.

Carry all four dimensions through the capability manifest, static resource JSON, generated Admin contract, generated OpenAPI schema and Admin TypeScript schema types. Generator drift tests must fail if any dimension is dropped or renamed.

External writes reject unknown, read-only, internal, sensitive and secret fields unless their declared policy explicitly permits the operation. Internal writes accept declared internal/read-only fields but reject undeclared keys and raw-secret classes. Derived hashes or encrypted envelopes are accepted only when the declared field policy names the matching non-plain storage mode and omits them from response/export. Projection reconstructs the record from declared schema fields and emits only fields whose response/export policy allows the requested purpose.

`persistContextLocked` is the final firewall: validate the complete snapshot against the active schemas immediately before every repository save. This catches policy-review, identity-binding, demo-data and any future direct in-memory mutation path even when it does not call `CreateInternal`/`UpdateInternal`. On validation failure, restore the previous snapshot and never call the repository.

`reloadContextLocked` applies a one-way containment scrub before installing legacy snapshots: remove undeclared fields and prohibited raw-secret classes, keep only declared derived hashes/envelopes allowed by policy, and atomically rewrite the sanitized snapshot before serving traffic. Diagnostics may record resource, field name and counts but never rejected values. This scrub is not the recoverable-PII encryption migration; declared plaintext personal fields remain disabled until the later sensitive-data runtime and historical-migration nodes provide encryption and rollback.

- [ ] **Step 4: Route HTTP responses and internal writers through the APIs**

Replace narrow `sanitizeAdminResourceRecord` behavior with `Store.ProjectRecord`. Convert audit, API-token, file, identity and phone system writes to `CreateInternal`/`UpdateInternal`.

- [ ] **Step 5: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/adminresource ./internal/platform/httpapi -run 'Test.*(Undeclared|Prohibited|Project|Response)' -count=1
rtk go test ./internal/platform/capability ./internal/platform/core
rtk node scripts/generate-admin-resource-contract.mjs
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/generate-admin-scaffold-promotion-review.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node --test scripts/admin-resource-contract-generators.test.mjs
rtk npm --prefix admin run build
rtk git add admin internal/platform/adminresource internal/platform/capability internal/platform/core internal/platform/httpapi resources scripts
rtk git commit -m "fix: enforce admin resource field boundaries"
```

### Task 2: Digest-Only Session Persistence

**Files:**
- Modify: `internal/platform/session/store.go`
- Modify: `internal/platform/session/file_repository.go`
- Modify: `internal/platform/session/gorm_repository.go`
- Modify: `internal/platform/session/sql_repository.go`
- Modify: `internal/platform/session/file_repository_test.go`
- Modify: `internal/platform/session/gorm_repository_test.go`
- Modify: `internal/platform/session/sql_repository_test.go`
- Modify: `internal/platform/session/store_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`

**Interfaces:**
- Produces: `session.DigestToken`, repository-only `StoredSession`, persisted `TokenDigest`, v2 file snapshot.
- Preserves: raw opaque token at Store and JWT call sites.

- [ ] **Step 1: Write failing persistence tests**

```go
func TestFileRepositoryDoesNotPersistRawSessionToken(t *testing.T) {
    raw := "raw-session-marker"
    // create through Store, read file, assert raw absent and DigestToken(raw) present
}

func TestRepositoryBackedStoreResolvesRenewsAndRevokesRawTokenThroughDigest(t *testing.T) {
    // issue raw token, then resolve/renew/revoke through the public Store API
}
```

Add equivalent raw-marker assertions for SQL and GORM tables.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/session -run 'Test.*(Digest|RawSessionToken|RepositoryBacked)' -count=1
```

- [ ] **Step 3: Implement digest-only repositories**

```go
func DigestToken(raw string) string {
    sum := sha256.Sum256(append([]byte("platform-session\x00"), []byte(raw)...))
    return "sha256:v1:" + hex.EncodeToString(sum[:])
}
```

`Store` computes the digest before every in-memory or repository lookup. Repository methods receive only a digest and `StoredSession`; repositories must never receive, return or log the raw handle. `Session.Token` is `json:"-"` and is restored only by the Store for the immediate in-memory return value. Persist `token_digest`, never raw `token`. File snapshot v1 records are ignored as expired during v2 load. SQL/GORM initialization transactionally replaces the legacy raw-token table with the digest schema, intentionally revoking existing sessions; tests must prove the legacy table/column and marker are absent after migration.

- [ ] **Step 4: Remove session credentials from audits**

Do not write either the raw session handle, its digest or a shortened digest-derived value into audit or file metadata. Use actor ID, request/event ID and stable action/result fields instead. Update production-auth contracts and tests that currently require `shortSessionID(raw)`.

- [ ] **Step 5: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/session ./internal/platform/httpapi -run 'Test.*(Session|Audit)' -count=1
rtk node --test scripts/platform-production-auth-hardening.test.mjs
rtk node scripts/validate-platform-production-auth-hardening.mjs
rtk git add internal/platform/session internal/platform/httpapi resources/platform-production-auth-hardening.json scripts
rtk git commit -m "fix: store session digests only"
```

### Task 3: Keyed Phone Protection And Delivery Gate

**Files:**
- Create: `internal/platform/httpapi/phone_protection.go`
- Create: `internal/platform/httpapi/phone_protection_test.go`
- Modify: `internal/platform/httpapi/app_phone.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `internal/platform/bootstrap/capabilities.go`
- Modify: `internal/platform/bootstrap/capabilities_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `deploy/env/production.example.env`

**Interfaces:**
- Produces: `PhoneProtector`, `PhoneVerificationSender`, debug sender and production config gate.

- [ ] **Step 1: Write failing protector and runtime tests**

```go
func TestHMACPhoneProtectorSeparatesPhoneAndCodeDomains(t *testing.T) {
    protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
    phoneDigest, _ := protector.PhoneDigest("+8613800138000")
    codeDigest, _ := protector.CodeDigest(phoneDigest, "bind", "123456")
    if phoneDigest == codeDigest { t.Fatal("digest domains collapsed") }
}

func TestValidateRuntimeRejectsProductionAppPhoneWithoutProviderAndDistinctKeys(t *testing.T) {
    // enable app-phone in production and assert validation error
}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/config ./internal/platform/bootstrap -run 'Test.*(PhoneProtector|AppPhone|PhoneVerification)' -count=1
```

- [ ] **Step 3: Implement ports and configuration**

```go
type PhoneProtector interface {
    PhoneDigest(phone string) (string, error)
    CodeDigest(phoneDigest, purpose, code string) (string, error)
}
type PhoneVerificationSender interface {
    Send(context.Context, string, string, string) error
    Kind() string
}
```

Add `PLATFORM_PHONE_HMAC_KEY`, `PLATFORM_PHONE_CODE_HMAC_KEY` and `PLATFORM_PHONE_VERIFICATION_PROVIDER`. Production requires distinct 32-byte-or-longer keys and an injected sender whose `Kind()` matches the configured non-debug provider when `app-phone` is enabled. Unsupported configured providers fail startup; the foundation does not pretend that a vendor adapter exists. Development/test may compose the explicit debug sender. The response field is:

```go
DebugCode string `json:"debugCode,omitempty"`
```

Only the debug sender returns it. Phone and code digests are versioned, domain-separated and use the existing normalized phone value rather than the raw presentation string.

- [ ] **Step 4: Verify sender failure is atomic**

Write the record only after successful delivery, or delete/compensate it on sender failure. Tests assert no verification row exists after failure.

- [ ] **Step 5: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/config ./internal/platform/bootstrap -run 'Test.*(Phone|ValidateRuntime)' -count=1
rtk git add cmd/platform-api internal/platform/httpapi internal/platform/config internal/platform/bootstrap deploy/env/production.example.env
rtk git commit -m "fix: protect phone verification data"
```

### Task 4: Private Files, Upload Limits And MIME Validation

**Files:**
- Create: `internal/platform/httpapi/upload_policy.go`
- Create: `internal/platform/httpapi/upload_policy_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/storage/object.go`
- Modify: `internal/platform/storage/s3_object.go`
- Modify: `internal/platform/storage/object_test.go`
- Modify: `internal/platform/bootstrap/file_storage.go`
- Modify: `internal/platform/bootstrap/file_storage_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `internal/platform/core/capabilities.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `resources/admin-resources.json`
- Modify: `deploy/nginx/platform.conf`
- Modify: `deploy/compose/docker-compose.prod.yml`
- Modify: `deploy/env/production.example.env`
- Modify: `resources/platform-deployment-topology.json`
- Modify: `scripts/validate-platform-deployment-topology.mjs`
- Modify: `scripts/platform-deployment-topology.test.mjs`
- Modify: `docs/platform-deployment.md`

**Interfaces:**
- Produces: `UploadPolicy`, sanitized private file metadata and no public upload alias.

- [ ] **Step 1: Write failing upload and deployment tests**

```go
func TestAdminFileUploadRejectsOversizeBeforeObjectCreation(t *testing.T) {}
func TestAdminFileUploadRejectsSpoofedOrDisallowedMIME(t *testing.T) {}
func TestAppFileMetadataOmitsSessionPathAndPublicURL(t *testing.T) {}
func TestAppFileContentReturnsNotFoundForCrossUser(t *testing.T) {}
func TestS3ObjectStoreAppliesConfiguredServerSideEncryption(t *testing.T) {}
func TestValidateRuntimeRejectsProductionS3WithoutPrivateEncryptionPolicy(t *testing.T) {}
```

Add a Node mutation test that inserts `location /uploads/` and expects the deployment validator to fail.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/storage ./internal/platform/bootstrap ./internal/platform/config -run 'Test.*(File|S3)' -count=1
rtk node --test scripts/platform-deployment-topology.test.mjs
```

- [ ] **Step 3: Implement upload policy and private metadata**

```go
type UploadPolicy struct {
    MaxBytes int64
    AllowedMediaTypes map[string]struct{}
}
```

Wrap multipart request bodies with `http.MaxBytesReader`, inspect the first 512 bytes with `http.DetectContentType`, sanitize filenames and reject mismatches or disallowed media types. Persist only `mimeType`, `size`, `storageDriver`, internal `storageKey`, tenant, owner and timestamps. Remove new `storagePath`, `publicUrl` and `sessionId` writes and response fields. Local directories/files use private permissions and have no public URL default. Remove Nginx `/uploads/`, its shared read-only Admin volume and the public URL environment variable.

Add and validate `PLATFORM_FILE_MAX_UPLOAD_BYTES` and `PLATFORM_FILE_ALLOWED_MIME_TYPES` in this task, then inject the resulting `UploadPolicy` through `cmd/platform-api`. Defaults are explicit and bounded; invalid or empty production policies fail startup.

Add explicit S3 server-side encryption configuration (`AES256` or `aws:kms`, with a required KMS key ID for `aws:kms`). Production S3 rejects missing encryption policy and non-HTTPS endpoints outside loopback development/test. `PutObject` carries the configured encryption fields, never a public ACL or public URL. Document bucket-level Block Public Access as a production preflight requirement; application configuration cannot silently claim to inspect an external bucket policy.

- [ ] **Step 4: Verify GREEN, regenerate contracts and commit**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/storage ./internal/platform/bootstrap ./internal/platform/config -run 'Test.*(File|S3)' -count=1
rtk node scripts/generate-admin-resource-contract.mjs
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/generate-admin-scaffold-promotion-review.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git add cmd internal resources scripts deploy docs
rtk git commit -m "fix: make file delivery private"
```

### Task 5: Production HTTPS, Trusted Proxies And Security Headers

**Files:**
- Create: `internal/platform/httpapi/security_headers.go`
- Create: `internal/platform/httpapi/security_headers_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `deploy/nginx/platform.conf`
- Modify: `deploy/compose/docker-compose.prod.yml`
- Modify: `deploy/env/production.example.env`
- Modify: `resources/platform-deployment-topology.json`
- Modify: `resources/platform-production-readiness.json`
- Modify: `scripts/platform-production-readiness.test.mjs`
- Modify: `scripts/validate-platform-production-readiness.mjs`

**Interfaces:**
- Produces: public-base URL validation, trusted proxy setup and security middleware.

- [ ] **Step 1: Write failing config and header tests**

```go
func TestValidateRuntimeRejectsProductionNonHTTPSPublicBaseURL(t *testing.T) {}
func TestValidateRuntimeRejectsInvalidOrEmptyProductionTrustedProxyPolicy(t *testing.T) {}
func TestSecurityHeadersRequireTrustedHTTPSContextForHSTS(t *testing.T) {}
func TestJSONRequestBodyLimitRejectsOversizeBeforeHandler(t *testing.T) {}
func TestForwardedProtoFromUntrustedPeerCannotClaimHTTPS(t *testing.T) {}
func TestStagingRejectsNonLoopbackHTTPProviderAndStorageEndpoints(t *testing.T) {}
func TestProductionEdgeRedirectsUntrustedHTTPAndEmitsHSTSAfterTrustedHTTPS(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/config ./internal/platform/httpapi ./cmd/platform-api -run 'Test.*(PublicBase|TrustedProx|SecurityHeader|JSONRequest|ForwardedProto|Staging|ProductionEdge)' -count=1
```

- [ ] **Step 3: Implement config and middleware**

Add `PLATFORM_PUBLIC_BASE_URL`, `PLATFORM_TRUSTED_PROXIES` and `PLATFORM_HTTP_MAX_BODY_BYTES`; consume the upload-specific configuration introduced in Task 4. Parse trusted proxies as IP/CIDR and call Gin `SetTrustedProxies`. The security middleware independently checks the direct peer against the trusted proxy prefixes before honoring forwarded HTTPS headers; an untrusted `X-Forwarded-Proto` is ignored. Apply the global body limit before every JSON binder and the upload-specific limit before multipart parsing. Add CSP, HSTS, frame, MIME and referrer headers to API and static Admin responses. Reject non-HTTPS provider/storage endpoints in staging and production; only loopback development/test endpoints may use HTTP.

The standard Nginx adapter remains an origin behind an external TLS edge: it preserves only the reviewed edge's HTTPS signal, redirects non-HTTPS public requests, emits the static headers and must not be directly exposed as a public HTTP origin. The production topology contract and docs make that boundary explicit.

- [ ] **Step 4: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/config ./internal/platform/httpapi ./cmd/platform-api -run 'Test.*(PublicBase|TrustedProx|SecurityHeader|JSONRequest|ForwardedProto|Staging|ProductionEdge|ValidateRuntime)' -count=1
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk git add cmd internal deploy resources scripts docs README.md
rtk git commit -m "fix: enforce production transport security"
```

### Task 6: Shared Rate Limiting

**Files:**
- Create: `internal/platform/ratelimit/limiter.go`
- Create: `internal/platform/ratelimit/memory.go`
- Create: `internal/platform/ratelimit/redis.go`
- Create: `internal/platform/ratelimit/limiter_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/app_phone.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/bootstrap/cache.go`
- Modify: `internal/platform/bootstrap/capabilities.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `deploy/compose/docker-compose.prod.yml`
- Modify: `deploy/env/production.example.env`
- Modify: `resources/platform-production-readiness.json`
- Modify: `scripts/validate-platform-production-env.mjs`
- Modify: `scripts/platform-production-env.test.mjs`
- Modify: `scripts/validate-platform-production-readiness.mjs`
- Modify: `scripts/platform-production-readiness.test.mjs`

**Interfaces:**
- Produces: shared memory and Redis `ratelimit.Limiter`.

- [ ] **Step 1: Write failing limiter tests**

```go
type Limiter interface {
    Allow(context.Context, string, int, time.Duration) (Decision, error)
}
type Decision struct {
    Allowed bool
    RetryAfter time.Duration
}
```

Tests cover threshold, window reset, shared Redis state, fail-closed errors and key redaction. Add a dedicated `PLATFORM_RATE_LIMIT_HMAC_KEY`; production requires at least 32 bytes and it must be distinct from phone/code keys.

Add the key to the standard production compose/env contract, production readiness required environment list and strict production-env validator. Mutation tests must prove a missing, short or duplicated key fails before runtime construction.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/ratelimit ./internal/platform/httpapi -run 'Test.*RateLimit' -count=1
```

- [ ] **Step 3: Implement and apply limiters**

Apply these initial defaults, with one centralized policy table so later configuration does not fork handler logic:

| Operation | Limit | Window |
| --- | ---: | ---: |
| Admin/App login | 10 | 5 minutes |
| OIDC provider start | 20 | 5 minutes |
| Phone verification request | 5 | 10 minutes |
| Phone binding verification | 10 | 10 minutes |
| Admin/App upload | 30 | 1 minute |

A key builder HMACs normalized dimensions before they reach memory/Redis; Redis keys and logs never contain raw username, phone, IP, credential or provider subject values. Use a Redis Lua operation for atomic shared counters and TTL. Production rejects a process-local limiter. Limiter errors return stable `503`; denied requests return `429` and `Retry-After`.

- [ ] **Step 4: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/ratelimit ./internal/platform/httpapi ./internal/platform/bootstrap ./internal/platform/config -run 'Test.*RateLimit' -count=1
rtk node --test scripts/platform-production-env.test.mjs scripts/platform-production-readiness.test.mjs
rtk node scripts/validate-platform-production-env.mjs --env-file deploy/env/production.example.env
rtk node scripts/validate-platform-production-readiness.mjs
rtk git add cmd/platform-api internal/platform/ratelimit internal/platform/httpapi internal/platform/bootstrap internal/platform/config deploy resources scripts
rtk git commit -m "fix: rate limit credential endpoints"
```

### Task 7: Audit And Export Redaction Closeout

**Files:**
- Modify: `internal/platform/adminresource/policy_review.go`
- Modify: `internal/platform/adminresource/policy_review_test.go`
- Modify: `internal/platform/adminresource/store.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `internal/platform/adminresource/security.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Create: `cmd/platform-api/main_test.go`
- Modify: `internal/platform/core/capabilities.go`
- Modify: `resources/admin-resources.json`
- Modify: `admin/src/platform/policy-review/PolicyReviewConsole.tsx`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`
- Modify: `docs/platform-auth.md`
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/platform-deployment.md`
- Modify: `docs/platform-capability-development.md`
- Modify: task graph and closeout governance resources from the completion-program plan.

**Interfaces:**
- Consumes: `Store.ProjectRecord(..., ProjectionExport)`.
- Produces: explicit export permission, redacted audits and closed `runtime-security-containment` node.

- [ ] **Step 1: Write failing export and redaction tests**

```go
func TestPolicyReviewExportRequiresExportPermissionSeparateFromRead(t *testing.T) {}
func TestLegacySecretMarkersDoNotAppearInListQueryExportOrAudit(t *testing.T) {}
func TestStorageAndProviderErrorsDoNotLeakMarkers(t *testing.T) {}
func TestRuntimeLogsDoNotContainSecretMarkers(t *testing.T) {}
func TestPlatformHasNoLocalPasswordProviderOrPasswordPersistencePath(t *testing.T) {}
func TestAdminResourceMutationRollsBackWhenAuditPersistenceFails(t *testing.T) {}
func TestFileUploadCleansObjectAndRecordWhenAuditPersistenceFails(t *testing.T) {}
func TestFileDeleteTombstoneKeepsContentHiddenUntilObjectCleanupCompletes(t *testing.T) {}
func TestFileDeleteRetriesObjectCleanupAfterObjectOrAuditFailure(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/adminresource ./internal/platform/httpapi ./cmd/platform-api -run 'Test.*(Export|Redact|Leak|Audit|Password|RollsBack|CleansObject|Tombstone|RetriesObjectCleanup)' -count=1
```

- [ ] **Step 3: Implement export permission and fail-closed projection**

Add `admin:policy-review:export`. Export projects every review and audit record through `ProjectionExport`; one projection error aborts the response. Build and validate redacted audit data before the protected operation; never fall back to raw event values. Audit records use actor ID, action, resource, target ID, result, request/event ID and reason code, with masked or omitted target labels and no session handle/digest. Replace adapter `err.Error()` HTTP responses with stable public errors.

Add transactional Store mutation APIs that persist the business record and audit record in one repository snapshot. Generic create/update/delete handlers use those APIs. Authentication/session flows revoke or discard newly issued credentials when audit persistence fails. File uploads delete the just-written object and roll back the resource record when the record+audit transaction fails; cleanup errors are logged only through the redacted error path. No handler may swallow audit errors after returning a successful protected operation.

File deletion uses a durable tombstone/outbox state in the same record+audit snapshot: content access rejects tombstoned records immediately, object deletion is idempotently retried, and the internal record is purged only after object deletion succeeds. Object-delete or audit-persistence failures must never restore public access, lose the only recoverable object reference or leave a visible record pointing at missing content.

Gate the Admin export button with `admin:policy-review:export` independently of read access, preserve keyboard/focus behavior, and add UI contract tests for the hidden/disabled state. Any new user-visible copy must be added to Chinese and English dictionaries in the same change.

Document and test the current credential boundary: the platform has no local password provider or password repository, generic resources reject password fields, and browser-to-server credential-bearing requests require the production HTTPS contract. A future local-password capability must use the separately approved Argon2id boundary and cannot be added to generic `Record.Values`.

- [ ] **Step 4: Regenerate contracts and close the node**

Regenerate Admin contracts/OpenAPI. Update `runtime-security-containment` to implemented, move it atomically from alignment `requiredFutureTaskNodes` to `requiredTaskNodes`, remove it from every unfinished/controlled-blocker projection, add a node-closeout entry and set engineering security capability evidence to implemented. Update the goal summary to `45 total / 38 implemented / 7 controlled unfinished`; keep the remaining seven program nodes pending in their original order. Add mutation tests for missing future-to-required migration, stale `45/37/8` counts and reordered seven-node projections.

Run the complete Admin resource generation chain (`admin-resource-contract`, Admin OpenAPI, codegen preview, scaffold plan/files/draft/promotion review) before validation so no generated artifact is stale.

- [ ] **Step 5: Run the complete node verification**

```bash
rtk go test ./...
rtk node --test scripts/*.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-production-env.mjs --env-file deploy/env/production.example.env
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

- [ ] **Step 6: Commit**

```bash
rtk git add README.md docs internal resources scripts deploy cmd
rtk git commit -m "fix: close runtime security containment"
```
