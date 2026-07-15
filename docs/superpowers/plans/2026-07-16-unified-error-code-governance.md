# Unified Error Code Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the `unified-error-code-governance` release blocker with one executable error registry, deterministic response and correlation behavior, generated public contracts, compatibility gates and safe Admin/App/service adapter coverage.

**Architecture:** `internal/platform/errorcode` is the canonical runtime registry and exports a deterministic contract through `cmd/platform-contracts`. Gin middleware creates opaque request and W3C trace correlation before any error-producing middleware, while one registry-backed response writer owns HTTP status and public messages. Admin, App, Service/Data, Control and External contracts consume generated error artifacts; domain and adapter errors remain private and are translated at HTTP or service boundaries.

**Tech Stack:** Go 1.26, Gin, GORM, Node.js contract generators and validators, OpenAPI 3.1, generated Go/TypeScript SDK artifacts, Admin React/TypeScript API client.

## Global Constraints

- The task definition in `resources/platform-foundation-task-graph.json` is authoritative: ownership, audience, HTTP mapping, retryability, redaction, compatibility, deprecation, correlation and public code generation are mandatory completion gates.
- Keep every public error code stable after the initial v0.1 registry baseline. A code cannot change owner, planes, audience, category, HTTP status, retry policy or redaction class within the same major version.
- Resolve current pre-contract collisions before freezing the initial baseline. In particular, upload parsing must use `ADMIN_FILE_UPLOAD_OPEN_FAILED` and `APP_FILE_UPLOAD_OPEN_FAILED`; object reads retain `ADMIN_FILE_OPEN_FAILED` and `APP_FILE_OPEN_FAILED`.
- App routes must not emit `ADMIN_*` authorization or resource codes. App authorization uses `APP_FORBIDDEN`; App file metadata failures use App-owned codes.
- The response error body is exactly `code`, `message`, `requestId` and `traceId`. Do not add arbitrary details, stack traces, SQL, physical schema, datasource, shard, credentials, PII, file paths or raw adapter messages.
- Error responses contain no success `data`. Unknown or unregistered codes fail closed to registered `INTERNAL_ERROR` behavior.
- The server generates opaque request IDs. Never reflect a raw client `X-Request-ID`. Accept only valid W3C `traceparent`; otherwise generate a new trace and span context.
- Keep `Retry-After` as the HTTP mechanism for delay-based retries. The registry describes retry policy but does not invent client-side automatic retries.
- Service/Data and Control operations that remain contract-only gain error schemas and SDK types but do not gain runtime handlers.
- Expose shared correlation through `internal/platform/kernel`; `organization-rbac-menu-e2e-qa` may consume that API in its own work, but this plan must not modify organization/RBAC/menu contracts, UI, migration gates or browser evidence.
- Do not add datasource routing, sharding, federation, XA, Outbox, MQ or search behavior. Their deferred nodes may register codes only when activated later.
- Use test-first RED/GREEN cycles, prefix commands with `rtk`, edit with `apply_patch`, preserve unrelated work and run CodeGraph before shared contract changes.
- Run `neat-freak` once at Task 7 phase closeout, not for the individual implementation tasks.

---

### Task 1: Freeze The Canonical Registry And Compatibility Baseline

**Files:**
- Create: `internal/platform/errorcode/registry.go`
- Create: `internal/platform/errorcode/builtin.go`
- Create: `internal/platform/errorcode/registry_test.go`
- Modify: `cmd/platform-contracts/main.go`
- Create: `resources/platform-error-code-standard.json`
- Create: `resources/generated/platform-error-code-contract.json`
- Create: `resources/fixtures/platform-error-codes/compatibility-baseline.json`
- Create: `scripts/validate-platform-error-code-registry.mjs`
- Create: `scripts/platform-error-code-registry.test.mjs`

**Interfaces:**
- Consumes: current production error literals in `internal/platform/httpapi`, generic kernel codes in `internal/platform/kernel/errors.go`, service-object sentinels in `internal/platform/serviceobject/errors.go`.
- Produces: `errorcode.Code`, `errorcode.Definition`, `errorcode.Lookup`, `errorcode.All`, `errorcode.New`, `errorcode.Wrap`, `errorcode.CodeOf`, and `platform-contracts error-codes`.
- Used by: Tasks 2-6 response writers, adapters, OpenAPI generators, SDK generators and compatibility validators.

- [ ] **Step 1: Write failing Go registry tests**

```go
func TestBuiltinsAreUniqueCompleteAndStable(t *testing.T) {
	definitions := All()
	seen := map[Code]struct{}{}
	for _, definition := range definitions {
		if _, exists := seen[definition.Code]; exists {
			t.Fatalf("duplicate code %q", definition.Code)
		}
		seen[definition.Code] = struct{}{}
		if err := definition.Validate(); err != nil {
			t.Fatalf("definition %q: %v", definition.Code, err)
		}
	}
	for _, code := range []Code{CodeInternal, CodeAdminForbidden, CodeAppForbidden, CodeRateLimited, CodeServiceObjectCostLimit} {
		if _, ok := Lookup(code); !ok {
			t.Fatalf("Lookup(%q) = false", code)
		}
	}
}

func TestPublicErrorDoesNotRenderWrappedCause(t *testing.T) {
	err := Wrap(CodeInternal, errors.New("password=marker physical_table=users"))
	if strings.Contains(err.Error(), "marker") || strings.Contains(err.Error(), "physical_table") {
		t.Fatalf("public error leaked cause: %v", err)
	}
	if code, ok := CodeOf(err); !ok || code != CodeInternal {
		t.Fatalf("CodeOf() = %q, %t", code, ok)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

```bash
rtk go test ./internal/platform/errorcode -count=1
```

Expected: FAIL because the package and registry API do not exist.

- [ ] **Step 3: Implement the registry types and immutable API**

```go
package errorcode

type Code string
type Plane string
type Audience string
type Category string
type RetryPolicy string
type RedactionClass string

const (
	PlaneAdmin Plane = "admin"
	PlaneApp Plane = "app"
	PlaneService Plane = "service"
	PlaneData Plane = "data"
	PlaneControl Plane = "control"
	PlaneExternal Plane = "external"
	AudienceOperator Audience = "operator"
	AudienceInternal Audience = "internal"
	AudiencePartner Audience = "partner"
	AudiencePublic Audience = "public"
	CategoryAuthorization Category = "authorization"
	CategoryValidation Category = "validation"
	CategoryNotFound Category = "not-found"
	CategoryConflict Category = "conflict"
	CategoryRateCost Category = "rate-cost"
	CategoryDependency Category = "dependency"
	CategoryInternal Category = "internal"
	RetryNever RetryPolicy = "never"
	RetryAfterDelay RetryPolicy = "after-delay"
	RetryBackoff RetryPolicy = "backoff"
	RedactionPublicSafe RedactionClass = "public-safe"
	RedactionGenericOnly RedactionClass = "generic-only"
	RedactionCorrelationOnly RedactionClass = "correlation-only"
)

type Definition struct {
	Code Code `json:"code"`
	Owner string `json:"owner"`
	Planes []Plane `json:"planes"`
	Audiences []Audience `json:"audiences"`
	Category Category `json:"category"`
	HTTPStatus int `json:"httpStatus"`
	RetryPolicy RetryPolicy `json:"retryPolicy"`
	RedactionClass RedactionClass `json:"redactionClass"`
	PublicMessage string `json:"publicMessage"`
	IntroducedIn string `json:"introducedIn"`
	Deprecated bool `json:"deprecated"`
	SunsetAt string `json:"sunsetAt,omitempty"`
	ReplacedBy Code `json:"replacedBy,omitempty"`
}

func Lookup(code Code) (Definition, bool)
func All() []Definition
func New(code Code) error
func Wrap(code Code, cause error) error
func CodeOf(err error) (Code, bool)
```

`Error()` returns only the code. `Unwrap()` preserves the private cause for internal `errors.Is`/`errors.As` checks. `All()` returns a defensive sorted copy. `Definition.Validate()` enforces the approved enums, `400..599`, code pattern `^[A-Z][A-Z0-9_]{2,127}$`, non-empty owner/message/version and complete deprecation metadata.

- [ ] **Step 4: Enumerate the initial built-in registry and resolve pre-v1 collisions**

Add typed constants and definitions for every production HTTP code plus the generic kernel codes. Replace dynamic upload families with explicit Admin and App constants for required, too-large, upload-open, upload-read, MIME-invalid, MIME-mismatch and MIME-not-allowed cases. Assign each code exactly one HTTP status and one semantic category.

Do not freeze these invalid legacy relationships:

```text
ADMIN_FILE_OPEN_FAILED -> both upload parsing 400 and object read 500
APP_FILE_OPEN_FAILED   -> both upload parsing 400 and object read 500
ADMIN_FORBIDDEN        -> emitted from App authorization
ADMIN_RESOURCE_*       -> emitted from App file metadata paths
```

The first stable baseline contains the corrected v0.1 definitions. Document the replaced pre-contract behavior in `resources/platform-error-code-standard.json`; do not represent it as a stable alias.

- [ ] **Step 5: Add deterministic contract export**

Add `platform-contracts error-codes --output <path>`. The generated document contains `contractVersion`, `contractHash`, ordered enum catalogs and ordered definitions. Hash canonical JSON excluding `contractHash`, using the existing `sha256:<hex>` convention.

```bash
rtk go run ./cmd/platform-contracts error-codes --output resources/generated/platform-error-code-contract.json
```

Expected: deterministic generated JSON with no duplicate definitions and a valid canonical hash.

- [ ] **Step 6: Add compatibility RED/GREEN tests**

The Node test mutates one fixture per case and requires validator failure for duplicate code, owner/plane/audience/category/status/retry/redaction reassignment, removal without retained deprecation, invalid sunset, missing replacement, unknown replacement and generated hash drift. Add one positive additive-code fixture.

```bash
rtk node --test scripts/platform-error-code-registry.test.mjs
rtk node scripts/validate-platform-error-code-registry.mjs
rtk go test ./internal/platform/errorcode ./cmd/platform-contracts -count=1
```

Expected: all focused tests pass and the validator reports the registry valid.

- [ ] **Step 7: Commit the registry slice**

```bash
rtk git add internal/platform/errorcode cmd/platform-contracts/main.go resources/platform-error-code-standard.json resources/generated/platform-error-code-contract.json resources/fixtures/platform-error-codes scripts/validate-platform-error-code-registry.mjs scripts/platform-error-code-registry.test.mjs
rtk git commit -m "feat: add stable platform error registry"
```

### Task 2: Add Correlation Context And One Error Response Writer

**Files:**
- Create: `internal/platform/kernel/correlation.go`
- Create: `internal/platform/kernel/correlation_test.go`
- Create: `internal/platform/httpapi/request_correlation.go`
- Create: `internal/platform/httpapi/request_correlation_test.go`
- Create: `internal/platform/httpapi/error_response.go`
- Create: `internal/platform/httpapi/error_response_test.go`
- Modify: `internal/platform/httpapi/response.go`
- Modify: `internal/platform/httpapi/security_headers.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`

**Interfaces:**
- Consumes: Task 1 `errorcode.Lookup` and typed codes.
- Produces: `kernel.Correlation`, `kernel.WithCorrelation`, `kernel.CorrelationFromContext`, `requestCorrelation`, `writePlatformError`, `writePlatformErrorWithCause`, `recoveryMiddleware`.
- Used by: all HTTP adapters in Tasks 3-4 and audit/log correlation in Task 6.

- [ ] **Step 1: Write failing correlation and envelope tests**

```go
func TestErrorResponseCarriesServerOwnedRequestAndTraceIDs(t *testing.T) {
	server := New(ServerOptions{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "email@example.test/private/path")
	server.Router().ServeHTTP(recorder, request)
	var body Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if body.Error == nil || body.Error.RequestID == "" || body.Error.TraceID == "" { t.Fatalf("body = %+v", body) }
	if strings.Contains(recorder.Body.String(), "email@example.test") { t.Fatal("client request id reflected") }
}

func TestRecoveryUsesRegisteredInternalErrorEnvelope(t *testing.T) {
	server := New(ServerOptions{AdminRoutes: []AdminRouteRegistration{{
		Method: http.MethodGet,
		Path: "/api/admin/test-panic",
		Handler: func(*gin.Context) { panic("private-panic-marker") },
	}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/test-panic", nil)
	server.Router().ServeHTTP(recorder, request)
	var body Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if recorder.Code != http.StatusInternalServerError || body.Error == nil || body.Error.Code != errorcode.CodeInternal {
		t.Fatalf("status = %d body = %+v", recorder.Code, body)
	}
	if body.Error.RequestID == "" || body.Error.TraceID == "" || strings.Contains(recorder.Body.String(), "private-panic-marker") {
		t.Fatalf("unsafe recovery body = %s", recorder.Body.String())
	}
}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/kernel ./internal/platform/httpapi -run 'Test.*(Correlation|ErrorResponse|Recovery)' -count=1
```

Expected: FAIL because correlation fields, middleware and registry-backed writer do not exist.

- [ ] **Step 3: Add the cross-plane correlation context**

```go
package kernel

type Correlation struct {
	RequestID string
	TraceID string
	TraceParent string
}

func WithCorrelation(ctx context.Context, correlation Correlation) context.Context
func CorrelationFromContext(ctx context.Context) (Correlation, bool)
```

The stored values are already normalized opaque identifiers. Context helpers never read HTTP headers and never accept empty request or trace IDs.

- [ ] **Step 4: Add request/trace middleware before every error source**

`requestCorrelation()` generates `req_` plus 32 lowercase hex characters. Parse only W3C version `00` traceparent with non-zero 32-hex trace ID, non-zero 16-hex parent ID and two-hex flags. Preserve a valid incoming trace ID, generate a new server span ID, or generate both when the input is absent/invalid. Set `X-Request-ID` and `traceparent` response headers and place `kernel.Correlation` in the request context.

Middleware order becomes:

```go
router.Use(requestCorrelation())
router.Use(recoveryMiddleware(options.InternalErrorSink))
router.Use(securityHeaders(options.Security))
router.Use(jsonRequestBodyLimit(options.Security.MaxBodyBytes))
```

- [ ] **Step 5: Implement the single response writer**

```go
type ErrorBody struct {
	Code errorcode.Code `json:"code"`
	Message string `json:"message"`
	RequestID string `json:"requestId"`
	TraceID string `json:"traceId"`
}

func writePlatformError(ctx *gin.Context, code errorcode.Code)
func writePlatformErrorWithCause(ctx *gin.Context, sink InternalErrorSink, code errorcode.Code, cause error)
```

The writer resolves status/message from the registry, emits `Response[any]{Error: ...}` with no `Data`, calls `ctx.Abort()`, and falls back to `CodeInternal` if lookup fails. `writePlatformErrorWithCause` records only typed registry metadata and safe correlation; the cause is never formatted into the response.

- [ ] **Step 6: Route body-limit and recovery errors through the writer**

Replace direct `AbortWithStatusJSON` calls in `security_headers.go`. Replace `gin.Recovery()` with `recoveryMiddleware`, which returns `INTERNAL_ERROR` and records safe metadata without logging authorization headers, request bodies, panic values or stack traces to the public/error sink.

- [ ] **Step 7: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/kernel ./internal/platform/httpapi -run 'Test.*(Correlation|RequestBody|Recovery|ErrorResponse)' -count=1
rtk git add internal/platform/kernel internal/platform/httpapi
rtk git commit -m "feat: unify error response correlation"
```

### Task 3: Migrate Admin, Auth, Resource And Service Object Errors

**Files:**
- Create: `internal/platform/httpapi/admin_error_mapping.go`
- Create: `internal/platform/httpapi/admin_error_mapping_test.go`
- Create: `internal/platform/httpapi/service_object_error_mapping.go`
- Create: `internal/platform/httpapi/service_object_error_mapping_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/httpapi/service_objects_test.go`
- Modify: `internal/platform/httpapi/sensitive_reveal.go`
- Modify: `internal/platform/httpapi/sensitive_reveal_test.go`

**Interfaces:**
- Consumes: Task 2 `writePlatformError`/`writePlatformErrorWithCause` and Task 1 typed definitions.
- Produces: `adminResourceErrorCode`, `serviceObjectErrorCode`, `sensitiveRevealErrorCode`, registry-only Admin/auth/rate-limit error paths.
- Preserves: existing HTTP status semantics except the explicitly corrected pre-v1 collisions and data-leak removals.

- [ ] **Step 1: Write failing mapping matrix tests**

```go
func TestAdminResourceErrorCodeMatrix(t *testing.T) {
	tests := []struct{ err error; code errorcode.Code }{
		{adminresource.ErrUnknownResource, errorcode.CodeAdminResourceNotFound},
		{adminresource.ErrRecordNotFound, errorcode.CodeAdminResourceRecordNotFound},
		{adminresource.ErrRevisionConflict, errorcode.CodeAdminResourceRevisionConflict},
		{adminresource.ErrInvalidRecord, errorcode.CodeAdminResourceInvalidRecord},
		{adminresource.ErrDeletionDisabled, errorcode.CodeAdminResourceLifecycleConflict},
	}
	for _, test := range tests {
		if code := adminResourceErrorCode(test.err); code != test.code { t.Fatalf("%v -> %s", test.err, code) }
	}
}

func TestAdminResourceErrorsNeverExposeWrappedDetails(t *testing.T) {
	err := fmt.Errorf("%w: physical_table=users email=marker@example.test", adminresource.ErrInvalidRecord)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/resources/users", nil)
	writePlatformError(ctx, adminResourceErrorCode(err))
	if strings.Contains(recorder.Body.String(), "physical_table") || strings.Contains(recorder.Body.String(), "marker@example.test") {
		t.Fatalf("response leaked wrapped details: %s", recorder.Body.String())
	}
}
```

Add equivalent tables for all service-object sentinels and sensitive-reveal sentinels, including authorization, validation, not-found, conflict, cost, provider dependency and internal fallback.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/httpapi -run 'Test.*(AdminResourceErrorCode|ServiceObjectErrorCode|SensitiveRevealErrorCode|NeverExposeWrapped)' -count=1
```

Expected: FAIL because the mapping functions do not exist and current responses use caller-provided strings.

- [ ] **Step 3: Implement pure domain-to-code mappings**

Each mapper returns only `errorcode.Code`. It never returns status or public message. Known validation/conflict/not-found errors use `writePlatformError`; unexpected failures use `writePlatformErrorWithCause` so the internal sink receives safe correlation.

Remove these functions after their final caller migrates:

```text
writeAuthError(ctx, status, code, message)
writeServiceObjectError(ctx, err)
writeAdminResourceError(ctx, err)
writeSensitiveRevealError(ctx, err)
```

Do not return `err.Error()` for lifecycle or invalid-record responses.

- [ ] **Step 4: Migrate auth, rate-limit, OpenAPI and Admin route branches**

Replace free-form literals in provider start/login/refresh/logout, Admin authorization, rate limiting, OpenAPI availability, menu resolution, demo data and policy review with typed registry constants. Keep `Retry-After` for `RATE_LIMITED`. Internal provider/session/repository failures use the cause-recording writer.

- [ ] **Step 5: Migrate Service Object and sensitive reveal branches**

Preserve indistinguishable denied/missing service-object responses. Fix generated/runtime wording so `SERVICE_OBJECT_UNAVAILABLE` is the sole runtime 404 code unless a distinct registered `SERVICE_OBJECT_NOT_FOUND` behavior is implemented and tested. Preserve `Cache-Control: no-store` for sensitive reveal errors.

- [ ] **Step 6: Verify the complete Admin category matrix**

```bash
rtk go test ./internal/platform/httpapi -run 'Test.*(Auth|AdminResource|ServiceObject|SensitiveReveal|RateLimit|OpenAPI|Menu)' -count=1
rtk go test ./internal/platform/errorcode ./internal/platform/serviceobject ./internal/platform/adminresource -count=1
rtk git diff --check
```

Expected: authorization, validation, not-found, conflict, rate/cost, dependency and internal tests pass with registered codes and correlation fields.

- [ ] **Step 7: Commit the Admin/Service migration**

```bash
rtk git add internal/platform/httpapi
rtk git commit -m "refactor: route admin errors through registry"
```

### Task 4: Migrate App, External, Upload And File Errors

**Files:**
- Create: `internal/platform/httpapi/app_error_mapping.go`
- Create: `internal/platform/httpapi/app_error_mapping_test.go`
- Modify: `internal/platform/httpapi/app_routes.go`
- Modify: `internal/platform/httpapi/app_route_coverage_test.go`
- Modify: `internal/platform/httpapi/app_phone.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/httpapi/upload_policy.go`
- Modify: `internal/platform/httpapi/upload_policy_test.go`
- Modify: `internal/platform/storage/object.go`
- Modify: `internal/platform/storage/s3_object.go`
- Modify: `internal/platform/storage/object_test.go`

**Interfaces:**
- Consumes: Tasks 1-2 registry/writer and existing `storage.ObjectStore` sentinels.
- Produces: App-owned authorization/file/phone codes, `UploadErrorCodes`, `appFileMetadataErrorCode`, adapter-safe storage translation.
- Preserves: private file delivery, upload limits/MIME validation and existing App authentication boundaries.

- [ ] **Step 1: Write failing plane-isolation and collision tests**

```go
func TestAppRoutesNeverEmitAdminOwnedCodes(t *testing.T) {
	for _, code := range []errorcode.Code{errorcode.CodeAppForbidden, errorcode.CodeAppFileMetadataFailed} {
		definition, ok := errorcode.Lookup(code)
		if !ok { t.Fatalf("missing definition %q", code) }
		if strings.HasPrefix(string(code), "ADMIN_") { t.Fatalf("App code %q uses Admin prefix", code) }
		if !slices.Contains(definition.Planes, errorcode.PlaneApp) && !slices.Contains(definition.Planes, errorcode.PlaneExternal) {
			t.Fatalf("App code %q planes = %v", code, definition.Planes)
		}
	}
}

func TestUploadOpenAndObjectOpenUseDistinctStableCodes(t *testing.T) {
	for _, pair := range []struct{ upload, object errorcode.Code }{
		{errorcode.CodeAdminFileUploadOpenFailed, errorcode.CodeAdminFileOpenFailed},
		{errorcode.CodeAppFileUploadOpenFailed, errorcode.CodeAppFileOpenFailed},
	} {
		upload, uploadOK := errorcode.Lookup(pair.upload)
		object, objectOK := errorcode.Lookup(pair.object)
		if !uploadOK || !objectOK || pair.upload == pair.object { t.Fatalf("invalid pair %+v", pair) }
		if upload.HTTPStatus != http.StatusBadRequest || object.HTTPStatus != http.StatusInternalServerError {
			t.Fatalf("pair %+v statuses = %d/%d", pair, upload.HTTPStatus, object.HTTPStatus)
		}
	}
}

func TestAppRouteHandlerNotConfiguredOmitsRouteTopologyData(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/app/private", nil)
	appRouteHandlerNotConfigured(capability.AppRouteContract{CapabilityID: "private-capability", Method: http.MethodGet, Path: "/api/app/private"})(ctx)
	body := recorder.Body.String()
	for _, marker := range []string{"\"data\"", "private-capability", "\"method\"", "\"path\""} {
		if strings.Contains(body, marker) { t.Fatalf("response leaked %q: %s", marker, body) }
	}
}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/storage -run 'Test.*(AppRoutesNeverEmitAdmin|UploadOpenAndObjectOpen|RouteHandlerNotConfigured|AdapterError)' -count=1
```

Expected: FAIL because App routes reuse Admin codes, upload codes are dynamically concatenated and route topology is returned beside errors.

- [ ] **Step 3: Replace dynamic upload prefixes with typed code sets**

```go
type UploadErrorCodes struct {
	Required errorcode.Code
	TooLarge errorcode.Code
	OpenFailed errorcode.Code
	ReadFailed errorcode.Code
	MIMEInvalid errorcode.Code
	MIMEMismatch errorcode.Code
	MIMENotAllowed errorcode.Code
}

func readValidatedUpload(ctx *gin.Context, policy UploadPolicy, codes UploadErrorCodes) (validatedUpload, error)
```

Define complete immutable Admin and App code sets. `uploadPolicyError` stores only `errorcode.Code`; status/message come from the registry writer.

- [ ] **Step 4: Add App-specific authorization and metadata mappings**

`withAppRoutePolicy` emits `APP_FORBIDDEN`. App file handlers do not call `adminResourceErrorCode`; map record absence, invalid metadata, lifecycle conflict and unexpected persistence failures to App-owned definitions. Remove success `Data` from `APP_ROUTE_HANDLER_NOT_CONFIGURED` errors.

- [ ] **Step 5: Contain storage/provider errors at the adapter boundary**

Translate S3 `smithy.APIError` and local object failures to existing storage sentinels. HTTP handlers may inspect sentinels but never format provider error codes, bucket/key paths, endpoint details or credentials into response/log/audit values.

- [ ] **Step 6: Verify App and External matrices**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/storage -run 'Test.*(AppAuth|AppPhone|AppFile|Upload|Object|External|Adapter)' -count=1
rtk go test ./internal/platform/httpapi ./internal/platform/storage -count=1
rtk git diff --check
```

Expected: all App/External errors resolve through registered App/External definitions, upload/object codes are unambiguous and raw adapter markers remain absent.

- [ ] **Step 7: Commit the App/External migration**

```bash
rtk git add internal/platform/httpapi internal/platform/storage
rtk git commit -m "refactor: isolate app and external error contracts"
```

### Task 5: Generate OpenAPI And Go/TypeScript Error Contracts

**Files:**
- Create: `scripts/generate-platform-error-code-artifacts.mjs`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `scripts/generate-app-openapi.mjs`
- Modify: `scripts/generate-platform-service-contract-artifacts.mjs`
- Modify: `scripts/generate-admin-codegen-preview.mjs`
- Modify: `scripts/admin-resource-contract-generators.test.mjs`
- Modify: `scripts/platform-service-contract-standard.test.mjs`
- Modify: `scripts/platform-app-client-api-boundary.test.mjs`
- Modify: `scripts/validate-platform-service-contract-standard.mjs`
- Modify: `scripts/validate-platform-admin-api-boundary.mjs`
- Modify: `scripts/validate-platform-app-client-api-boundary.mjs`
- Create: `resources/generated/error-sdk/go/error_contract.go`
- Create: `resources/generated/error-sdk/typescript/errorContract.ts`
- Modify: `resources/generated/openapi.admin.json`
- Modify: `resources/generated/openapi.app.json`
- Modify: `resources/generated/openapi.service.json`
- Modify: `resources/generated/openapi.control.json`
- Modify: `resources/generated/openapi.external.json`
- Modify: `resources/generated/service-sdk/go/service_contract_sdk.go`
- Modify: `resources/generated/service-sdk/typescript/serviceContractSDK.ts`
- Modify: `resources/generated/admin-service-object-client.ts`
- Modify: `admin/src/platform/api/client.ts`
- Modify: `admin/src/platform/api/sessionExpiry.ts`
- Modify: `admin/src/platform/api/sessionExpiry.test.ts`

**Interfaces:**
- Consumes: generated registry contract and existing Admin/App/Service contract generators.
- Produces: generated `PlatformErrorCode`, `PlatformErrorDefinition`, `PlatformErrorBody`, typed Admin `AdminAPIError.code`, `requestId`, `traceId`, OpenAPI error schemas and registry provenance extensions.
- Preserves: generated files remain under `resources/generated`; global runtime source writing stays disabled.

- [ ] **Step 1: Write failing generator and consumer tests**

Require all five OpenAPI documents to contain a four-field required error schema and registry hash/source metadata. Require Admin, Service and standalone Go/TypeScript SDK outputs to compile with typed error codes. Mutate an OpenAPI `x-platform-error-codes` entry to an unknown code and require validation failure.

```bash
rtk node --test scripts/admin-resource-contract-generators.test.mjs scripts/platform-service-contract-standard.test.mjs scripts/platform-app-client-api-boundary.test.mjs
```

Expected: FAIL because the generated documents and SDKs do not expose the unified contract.

- [ ] **Step 2: Generate standalone deterministic error SDKs**

The TypeScript artifact exports a string-literal `PlatformErrorCode` union, readonly `PlatformErrorDefinition`, readonly `PlatformErrorBody` and a frozen definitions map. The Go artifact exports typed constants, `Definition`, `Definitions()` and `Lookup()`. Both carry the generated marker, source contract path and source hash.

```bash
rtk node scripts/generate-platform-error-code-artifacts.mjs
```

- [ ] **Step 3: Add registry-backed OpenAPI schemas**

Admin/App OpenAPI generators read `resources/generated/platform-error-code-contract.json`; they do not duplicate code arrays. Service/Control/External generation adds the same components even when a plane has only contract-only operations.

```json
{
  "ErrorBody": {
    "type": "object",
    "required": ["code", "message", "requestId", "traceId"],
    "properties": {
      "code": { "$ref": "#/components/schemas/PlatformErrorCode" },
      "message": { "type": "string" },
      "requestId": { "type": "string", "pattern": "^req_[0-9a-f]{32}$" },
      "traceId": { "type": "string", "pattern": "^[0-9a-f]{32}$" }
    },
    "additionalProperties": false
  }
}
```

Each document includes `x-platform-error-registry-source` and `x-platform-error-registry-hash`. Existing `x-platform-error-codes` lists must reference registered codes and match the operation plane.

- [ ] **Step 4: Extend generated service and Admin clients**

Service SDKs expose the error body and registry definitions alongside tenant/trace/event types. The Admin service-object client uses the generated `PlatformErrorBody`. `AdminAPIError` becomes:

```ts
export class AdminAPIError extends Error {
  constructor(
    message: string,
    readonly statusCode: number,
    readonly code: PlatformErrorCode,
    readonly requestId: string,
    readonly traceId: string,
  ) {
    super(message);
    this.name = "AdminAPIError";
  }
}
```

Malformed/non-JSON upstream errors use typed `INTERNAL_ERROR` with response-header correlation when available; they do not synthesize arbitrary code strings. Session-expiry policy continues to use the typed auth codes.

- [ ] **Step 5: Regenerate all affected artifacts**

```bash
rtk go run ./cmd/platform-contracts error-codes --output resources/generated/platform-error-code-contract.json
rtk node scripts/generate-platform-error-code-artifacts.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
rtk node scripts/generate-platform-service-contract-artifacts.mjs
```

- [ ] **Step 6: Verify generated consumers and Admin build**

```bash
rtk node --test scripts/admin-resource-contract-generators.test.mjs scripts/platform-service-contract-standard.test.mjs scripts/platform-app-client-api-boundary.test.mjs
rtk node scripts/validate-platform-error-code-registry.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-service-contract-standard.mjs
rtk npm --prefix admin run build
rtk git diff --check
```

Expected: deterministic regeneration, compiled Go/TypeScript consumers, valid OpenAPI references and a successful Admin production build.

- [ ] **Step 7: Commit the generated contract slice**

```bash
rtk git add admin/src/platform/api resources/generated scripts
rtk git commit -m "feat: generate public error contracts"
```

### Task 6: Correlate Internal Errors And Generic Audit Without Leaks

**Files:**
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `internal/platform/adminresource/audit.go`
- Modify: `internal/platform/adminresource/audit_test.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `internal/platform/adminresource/gorm_store.go`
- Modify: `internal/platform/adminresource/gorm_store_test.go`
- Modify: `resources/admin-resources.json`
- Modify: `scripts/generate-admin-resource-contract.mjs`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `scripts/admin-resource-contract-generators.test.mjs`

**Interfaces:**
- Consumes: Task 2 `kernel.Correlation`, Task 1 descriptor category/redaction metadata.
- Produces: correlated `InternalErrorEvent`, `AuditEvent.RequestID`, `AuditEvent.TraceID`, safe searchable audit fields and registry-derived cause classes.
- Boundary: specialized organization RBAC audit tables remain owned by `organization-rbac-menu-e2e-qa`; they consume `kernel.Correlation` independently and are not modified here.

- [ ] **Step 1: Write failing response/log/audit correlation tests**

```go
func TestInternalErrorAndAuditShareResponseCorrelation(t *testing.T) {
	correlation := kernel.Correlation{RequestID: "req_0123456789abcdef0123456789abcdef", TraceID: "0123456789abcdef0123456789abcdef", TraceParent: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/users", nil)
	request = request.WithContext(kernel.WithCorrelation(request.Context(), correlation))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	sink := &recordingInternalErrorSink{}
	writePlatformErrorWithCause(ctx, sink, errorcode.CodeInternal, errors.New("private-cause"))
	if len(sink.events) != 1 || sink.events[0].RequestID != correlation.RequestID || sink.events[0].TraceID != correlation.TraceID {
		t.Fatalf("events = %+v", sink.events)
	}
	server := &Server{}
	audit := server.mutationAuditEvent(ctx, "admin_resource.update", "users", "updated")
	if audit.RequestID != correlation.RequestID || audit.TraceID != correlation.TraceID { t.Fatalf("audit = %+v", audit) }
}

func TestCorrelationAuditFieldsNeverPersistInboundMarkersOrRawCause(t *testing.T) {
	server := New(ServerOptions{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "marker@example.test/private_table")
	server.Router().ServeHTTP(recorder, request)
	for _, marker := range []string{"marker@example.test", "private_table"} {
		if strings.Contains(recorder.Body.String(), marker) || strings.Contains(recorder.Header().Get("X-Request-ID"), marker) {
			t.Fatalf("correlation leaked %q", marker)
		}
	}
}
```

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/adminresource -run 'Test.*(InternalErrorAndAuditShare|CorrelationAuditFields|RawCause)' -count=1
```

Expected: FAIL because internal events and generic audit records do not yet carry the shared request/trace values.

- [ ] **Step 3: Replace string-inferred internal error metadata**

`InternalErrorEvent.Code` uses `errorcode.Code`. Add `Category`, `RequestID` and `TraceID`; derive category from the registry definition rather than substring matching. Preserve a private cause only for in-process diagnostics, but the sink-facing public error value and persisted error-log fields contain code/category/correlation only.

- [ ] **Step 4: Add safe generic audit correlation fields**

```go
type AuditEvent struct {
	Actor string
	Action string
	Resource string
	TargetID string
	Result string
	EventID string
	ReasonCode string
	RequestID string
	TraceID string
}
```

Add `requestId` and `traceId` to the audit resource as opaque internal fields that are readable/queryable by the dedicated audit permission and omitted from export. Keep the old `legacyTraceId`/legacy column interpretation omitted. GORM migration adds a request ID column and reuses or safely migrates the trace ID column without rewriting unrelated audit history.

- [ ] **Step 5: Route mutation audit creation through request context**

`mutationAuditEvent` reads `kernel.CorrelationFromContext(ctx.Request.Context())`. Audit `eventId` remains a distinct event identifier; it is not replaced by request ID. Any path without correlation generates a new opaque correlation through the same helper, never from raw external text.

- [ ] **Step 6: Regenerate and validate audit contracts**

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk go test ./internal/platform/adminresource ./internal/platform/httpapi -run 'Test.*(Audit|InternalError|Correlation)' -count=1
rtk node --test scripts/admin-resource-contract-generators.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
```

Expected: response/internal-error/audit correlation passes, new audit fields are contract-governed and raw markers are absent.

- [ ] **Step 7: Commit the correlation persistence slice**

```bash
rtk git add internal/platform/httpapi internal/platform/adminresource resources/admin-resources.json resources/generated scripts/generate-admin-resource-contract.mjs scripts/generate-admin-openapi.mjs scripts/admin-resource-contract-generators.test.mjs
rtk git commit -m "feat: correlate platform errors and audits"
```

### Task 7: Enforce Source Coverage And Close Governance

**Files:**
- Create: `internal/platform/errorcode/source_coverage_test.go`
- Modify: `scripts/validate-platform-error-code-registry.mjs`
- Modify: `scripts/platform-error-code-registry.test.mjs`
- Create: `docs/platform-error-code-governance.md`
- Modify: `docs/platform-service-contract-standard.md`
- Modify: `resources/platform-service-contract-standard.json`
- Modify: `README.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: corresponding governance validators and tests
- Create: `resources/evidence/unified-error-code-governance-20260716.json`

**Interfaces:**
- Consumes: all Task 1-6 runtime, generated and test evidence.
- Produces: AST-level prohibition of free-form public errors, implemented task/capability projections, integrity-addressed evidence and release-lane update.
- Preserves: full foundation remains `not-complete-controlled`; closing this node removes only `unified-error-code-governance` from `releaseBlockingNodes`.

- [ ] **Step 1: Add failing AST/source coverage tests**

Use `go/parser` and `go/ast` to scan non-test production Go files. Fail on direct `ErrorBody` composites outside `error_response.go`, calls supplying string status/code/message triples, dynamic error-code concatenation, unregistered `errorcode.Code("...")`, and App HTTP branches referencing Admin-only definitions. The Node test removes one registry entry and requires both source coverage and generated-contract validation to fail.

```bash
rtk go test ./internal/platform/errorcode -run TestProductionErrorConstructionCoverage -count=1
rtk node --test scripts/platform-error-code-registry.test.mjs
```

Expected before the final cleanup: FAIL with the exact remaining legacy construction sites.

- [ ] **Step 2: Remove the final legacy constructors and reach GREEN**

Delete unused free-form error helpers, string-based cause classification and dynamic code prefix fields. Keep only success `Response[T]` construction outside the error writer. Re-run the coverage tests until no production bypass remains.

- [ ] **Step 3: Add cross-category contract tests**

For each category, exercise at least one Admin and one App/External or Service path where applicable:

```text
authorization -> 401/403, retry never
validation    -> 400/422, retry never
not-found     -> 404, retry never
conflict      -> 409/410, retry after state change
rate-cost     -> 429 with Retry-After or 422 non-retryable cost rejection
dependency    -> 502/503, retry backoff
internal      -> 500, correlation-only redaction
```

All tests assert registered status/message, non-empty request/trace IDs and absence of schema, credentials, PII and adapter markers.

- [ ] **Step 4: Write the governance and operator documentation**

Document registry ownership, code naming, compatibility, deprecation, retry semantics, response envelope, correlation lookup and the process for adding a code. Update the Service Contract standard from schema-only trace fields to implemented HTTP extraction/correlation while keeping OpenTelemetry export and Event Plane propagation unimplemented.

- [ ] **Step 5: Record machine evidence and close the node**

The evidence manifest includes source hashes for registry, generated contract, five OpenAPI documents, Go/TypeScript SDKs, focused tests and validators. Mark the task and engineering capability implemented only after every completion gate passes. Remove it from `releaseBlockingNodes`; do not alter deferred datasource/MQ/search nodes or mark the persistent full-scope goal complete.

- [ ] **Step 6: Run focused and broad verification**

```bash
rtk go test ./internal/platform/errorcode ./internal/platform/kernel ./internal/platform/httpapi ./internal/platform/adminresource ./internal/platform/storage ./internal/platform/serviceobject ./cmd/platform-contracts -count=1
rtk node --test scripts/platform-error-code-registry.test.mjs
rtk node scripts/validate-platform-error-code-registry.mjs
rtk node --test scripts/admin-resource-contract-generators.test.mjs
rtk node --test scripts/platform-service-contract-standard.test.mjs
rtk node --test scripts/platform-app-client-api-boundary.test.mjs
rtk node scripts/validate-platform-service-contract-standard.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk npm --prefix admin run build
rtk go test ./...
rtk git diff --check
```

Expected: all focused and repository-wide checks pass; the full goal remains controlled-incomplete with the error-code node removed from release blockers.

- [ ] **Step 7: Run phase closeout review and commit**

Run one `neat-freak` synchronization for this cross-module release-blocking milestone, refresh CodeGraph, obtain an independent diff review, then commit only the closeout slice.

```bash
rtk codegraph sync .
rtk codegraph status
rtk git status --short
rtk git add README.md docs resources scripts internal/platform/errorcode/source_coverage_test.go
rtk git commit -m "feat: close unified error code governance"
rtk git status --short
```

Expected: CodeGraph is current, the review has no unresolved Critical/Important findings and the worktree is clean.

## Execution Order And Parallel Boundaries

1. Task 1 is the shared contract prerequisite.
2. Task 2 depends on Task 1 and freezes the response/correlation API.
3. Tasks 3 and 4 are separately reviewable but both touch `server.go`; execute them serially unless separate worktrees are rebased after Task 2.
4. Task 5 may run in parallel with Task 3 or 4 after Task 1 metadata and Task 2 envelope are frozen because it owns generators and generated artifacts rather than HTTP handler bodies.
5. Task 6 follows Task 2 and should merge after Tasks 3-4 so every internal failure path uses typed metadata.
6. Task 7 is serialized after all implementation tasks and after any concurrent `organization-rbac-menu-e2e-qa` governance closeout releases shared task-graph, engineering, closeout and roadmap files.

Declared task locks are disjoint from `organization-rbac-menu-e2e-qa`, so runtime work can proceed concurrently. Actual shared integration risks are `internal/platform/httpapi/server.go`, `internal/platform/httpapi/server_test.go` and final governance JSON/docs. Do not modify organization/RBAC/menu source during this plan; coordinate the shared closeout files at Task 7.
