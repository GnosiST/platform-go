# Production Admin OIDC Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a production-capable Admin OIDC login path with explicit identity binding while preserving the existing JWT, server-side session, Casbin RBAC, audit, capability, and Admin resource contracts.

**Architecture:** Add audience-aware provider declarations, a composition-root-injected `httpapi.AdminIdentityResolver`, a built-in OIDC adapter using Authorization Code plus PKCE/state/nonce, and a resource-backed `admin-identities` binding store. The Admin UI starts and completes OIDC in a tab-scoped transaction, while the API continues issuing the existing Admin JWT/session response after binding and principal validation.

**Tech Stack:** Go 1.26, Gin, GORM-backed Admin resources, Casbin, JWT, `github.com/coreos/go-oidc/v3/oidc`, `golang.org/x/oauth2`, React, TypeScript, Refine, Ant Design, Vite, Node contract validators, local Keycloak-compatible OIDC rehearsal.

## Global Constraints

- Prefix every shell command with `rtk`.
- Run `rtk codegraph sync .` and `rtk codegraph status` before and after shared auth, capability, resource, or Admin UI contract edits.
- Keep Admin and App provider audiences, identity bindings, token types, tenants, and login endpoints isolated.
- OIDC authenticates an identity only; platform users, enabled status, roles, permissions, deny rules, tenant, organization, and area remain authoritative.
- Never persist or expose raw issuer subjects, claims, authorization codes, PKCE verifiers, state, nonce, ID tokens, access tokens, refresh tokens, client secrets, or provider response bodies.
- Do not enable the independent refresh-token-family runtime.
- Do not add local passwords, MFA, automatic user creation, role mapping, group mapping, multi-issuer management, provider logout, or global logout.
- Production demo authentication remains disabled and real secrets remain outside Git.
- Shared Admin UI changes require matching Chinese and English dictionary keys.
- The final visual node requires `superpowers:brainstorming`, Product Design, `ui-ux-pro-max`, accessibility, browser, and neat-freak evidence.
- Use `apply_patch` for manual edits and preserve unrelated user changes.

---

### Task 1: Provider Audiences, OIDC Configuration, Capability, And Resource Contract

**Files:**
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/capability/auth_contract.go`
- Modify: `internal/platform/capability/auth_contract_test.go`
- Modify: `internal/platform/core/capabilities.go`
- Modify: `internal/platform/core/capabilities_test.go`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `internal/platform/bootstrap/capabilities.go`
- Modify: `internal/platform/bootstrap/capabilities_test.go`

**Interfaces:**
- Produces: `type AuthProviderAudience string` with `AuthProviderAudienceAdmin` and `AuthProviderAudienceApp`.
- Produces: `AuthProvider.Audiences []AuthProviderAudience` and `AuthProvider.SupportsAudience(AuthProviderAudience) bool`.
- Produces: `Config.AdminOIDCIssuerURL`, `AdminOIDCClientID`, `AdminOIDCClientSecret`, `AdminOIDCRedirectURL`, and `AdminOIDCScopes`.
- Produces: `Config.AdminOIDCConfigured() bool`.
- Produces: optional `admin-oidc` capability, `oidc` provider, and `admin-identities` Admin resource.

- [ ] **Step 1: Write failing provider audience contract tests**

Add tests that require known, unique, non-empty audiences and verify `SupportsAudience`:

```go
func TestValidateAuthProviderDeclarationsRequiresKnownAudience(t *testing.T) {
	provider := AuthProvider{
		ID: "oidc", Kind: "oidc", Enabled: true,
		Title: Text("OIDC 登录", "OIDC Login"),
		Description: Text("OIDC 后台登录。", "OIDC Admin login."),
		Audiences: []AuthProviderAudience{"unknown"},
	}
	err := ValidateAuthProviderDeclarations([]Manifest{{ID: "admin-oidc", AuthProviders: []AuthProvider{provider}}})
	if err == nil || !strings.Contains(err.Error(), "unknown audience") {
		t.Fatalf("ValidateAuthProviderDeclarations() error = %v, want unknown audience", err)
	}
}

func TestAuthProviderSupportsAudience(t *testing.T) {
	provider := AuthProvider{Audiences: []AuthProviderAudience{AuthProviderAudienceAdmin}}
	if !provider.SupportsAudience(AuthProviderAudienceAdmin) || provider.SupportsAudience(AuthProviderAudienceApp) {
		t.Fatalf("audience support mismatch: %+v", provider.Audiences)
	}
}
```

- [ ] **Step 2: Run the audience tests and verify failure**

Run:

```bash
rtk go test ./internal/platform/capability -run 'TestValidateAuthProviderDeclarationsRequiresKnownAudience|TestAuthProviderSupportsAudience' -count=1
```

Expected: FAIL because audience types and validation do not exist.

- [ ] **Step 3: Add the minimal provider audience contract**

Add:

```go
type AuthProviderAudience string

const (
	AuthProviderAudienceAdmin AuthProviderAudience = "admin"
	AuthProviderAudienceApp   AuthProviderAudience = "app"
)

type AuthProvider struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"`
	Title       LocalizedText          `json:"title"`
	Description LocalizedText          `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Configured  bool                   `json:"configured"`
	ConfigKeys  []string               `json:"configKeys,omitempty"`
	Audiences   []AuthProviderAudience `json:"audiences"`
}

func (p AuthProvider) SupportsAudience(audience AuthProviderAudience) bool {
	return slices.Contains(p.Audiences, audience)
}
```

Update `ValidateAuthProviderDeclarations` to reject missing, duplicate, and values other than `admin` or `app`.

- [ ] **Step 4: Write failing OIDC config and capability tests**

Add tests that verify:

```go
func TestLoadParsesAdminOIDCConfiguration(t *testing.T) {
	t.Setenv("PLATFORM_ADMIN_OIDC_ISSUER_URL", "https://id.example/realms/platform")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_ID", "platform-admin")
	t.Setenv("PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("PLATFORM_ADMIN_OIDC_REDIRECT_URL", "https://admin.example/login")
	t.Setenv("PLATFORM_ADMIN_OIDC_SCOPES", "openid,profile,email")
	cfg := Load()
	if !cfg.AdminOIDCConfigured() || len(cfg.AdminOIDCScopes) != 3 {
		t.Fatalf("OIDC config = %+v", cfg)
	}
}
```

Also require production rejection for partial credentials, missing `openid`, non-HTTPS production redirects, and demo-disabled production without a configured Admin provider. Development and test may use loopback HTTP redirect URLs.

- [ ] **Step 5: Run config and bootstrap tests and verify failure**

Run:

```bash
rtk go test ./internal/platform/config ./internal/platform/bootstrap ./internal/platform/core -run 'OIDC|AuthProvider|DefaultManifestsExposeAuthProviderDeclarations' -count=1
```

Expected: FAIL because OIDC configuration and capability declarations do not exist.

- [ ] **Step 6: Implement OIDC configuration, capability, provider status, and resource schema**

Load the five environment variables, implement complete-pair validation, and add this provider declaration shape:

```go
authProvider(
	"oidc", "oidc",
	"企业单点登录", "Enterprise SSO",
	"通过 OpenID Connect 登录管理台。", "Sign in to Admin through OpenID Connect.",
	false,
	[]capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin},
	"PLATFORM_ADMIN_OIDC_ISSUER_URL",
	"PLATFORM_ADMIN_OIDC_CLIENT_ID",
	"PLATFORM_ADMIN_OIDC_CLIENT_SECRET",
	"PLATFORM_ADMIN_OIDC_REDIRECT_URL",
)
```

Change the helper signature so every declaration passes audiences explicitly. Classify `demo` as Admin plus App and `wechat` as App only. Add `adminIdentityAdminResource()` with `provider`, `providerKind`, `issuerHash`, `providerSubjectHash`, `platformUsername`, `createdAt`, and `lastLoginAt`; raw issuer and subject fields must not exist.

- [ ] **Step 7: Run focused tests and commit**

Run:

```bash
rtk go test ./internal/platform/capability ./internal/platform/config ./internal/platform/bootstrap ./internal/platform/core -count=1
rtk git diff --check
```

Expected: PASS.

Commit:

```bash
rtk git add internal/platform/capability internal/platform/config internal/platform/bootstrap internal/platform/core
rtk git commit -m "feat: declare admin oidc capability"
```

---

### Task 2: OIDC Resolver, Signed State, PKCE, And Provider Error Normalization

**Files:**
- Create: `internal/platform/httpapi/admin_identity.go`
- Create: `internal/platform/authprovider/oidc/resolver.go`
- Create: `internal/platform/authprovider/oidc/resolver_test.go`
- Create: `internal/platform/authprovider/oidc/state.go`
- Create: `internal/platform/authprovider/oidc/state_test.go`
- Modify: `internal/platform/authprovider/resolver.go`
- Modify: `internal/platform/authprovider/resolver_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `httpapi.AdminIdentityResolver`.
- Produces: `StartAdminIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)`.
- Produces: `ResolveAdminIdentity(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)`.
- Produces: `authprovider.AdminIdentityResolverFromConfig(config.Config) (httpapi.AdminIdentityResolver, error)`.
- Consumes: audience-aware configured `capability.AuthProvider` and OIDC `Config` from Task 1.

- [ ] **Step 1: Define the generic Admin resolver contract**

Create the HTTP-owned neutral types:

```go
var (
	ErrAdminIdentityInvalid          = errors.New("invalid admin identity")
	ErrAdminIdentityTransaction      = errors.New("invalid admin identity transaction")
	ErrAdminIdentityProviderExchange = errors.New("admin identity provider exchange failed")
)

type AdminIdentityResolver interface {
	StartAdminIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	ResolveAdminIdentity(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)
}

type AdminIdentityStartInput struct {
	Provider      capability.AuthProvider
	CodeChallenge string
}

type AdminIdentityStart struct {
	AuthorizationURL string
	State            string
	ExpiresAt        time.Time
}

type AdminIdentityResolveInput struct {
	Provider     capability.AuthProvider
	Code         string
	State        string
	CodeVerifier string
}

type AdminIdentity struct {
	Issuer          string
	ProviderSubject string
}
```

- [ ] **Step 2: Write failing state and resolver tests**

Cover five-minute expiry, signature corruption, provider mismatch, PKCE mismatch, nonce mismatch, issuer mismatch, audience mismatch, upstream error normalization, and redaction. Use `httptest.Server` as a deterministic discovery/token/JWKS provider; never require network access.

The successful test must assert:

```go
start, err := resolver.StartAdminIdentity(ctx, httpapi.AdminIdentityStartInput{
	Provider: provider,
	CodeChallenge: challenge,
})
// provider callback fixtures return a signed ID token with matching nonce.
identity, err := resolver.ResolveAdminIdentity(ctx, httpapi.AdminIdentityResolveInput{
	Provider: provider,
	Code: "single-use-code",
	State: start.State,
	CodeVerifier: verifier,
})
if err != nil || identity.ProviderSubject != "subject-123" || identity.Issuer != issuer.URL {
	t.Fatalf("identity = %+v error = %v", identity, err)
}
```

- [ ] **Step 3: Run the resolver tests and verify failure**

Run:

```bash
rtk go test ./internal/platform/authprovider/oidc ./internal/platform/authprovider -count=1
```

Expected: FAIL because the packages and constructors do not exist.

- [ ] **Step 4: Add OIDC dependencies and minimal implementation**

Run:

```bash
rtk go get github.com/coreos/go-oidc/v3/oidc golang.org/x/oauth2
```

Implement:

```go
type Config struct {
	IssuerURL   string
	ClientID    string
	ClientSecret string
	RedirectURL string
	Scopes      []string
	StateKey    []byte
	Now         func() time.Time
	HTTPClient  *http.Client
}
```

Use S256 only. The state payload contains provider ID, nonce, challenge, random transaction ID, issued-at, and expiry. Sign `base64url(payload)` using HMAC-SHA256 with a key derived from `JWTSecret` using a distinct `platform-admin-oidc-state-v1` context. Compare signatures and challenges with constant-time operations.

Use `oidc.NewProvider`, `oauth2.Config.AuthCodeURL`, `oauth2.Config.Exchange` with `oauth2.SetAuthURLParam("code_verifier", verifier)`, and `oidc.IDTokenVerifier.Verify`. Extract only `sub` after nonce validation.

- [ ] **Step 5: Wire resolver construction**

Implement:

```go
func AdminIdentityResolverFromConfig(cfg config.Config) (httpapi.AdminIdentityResolver, error) {
	if !cfg.AdminOIDCConfigured() {
		return nil, nil
	}
	return authoidc.NewResolver(authoidc.Config{
		IssuerURL: cfg.AdminOIDCIssuerURL,
		ClientID: cfg.AdminOIDCClientID,
		ClientSecret: cfg.AdminOIDCClientSecret,
		RedirectURL: cfg.AdminOIDCRedirectURL,
		Scopes: cfg.AdminOIDCScopes,
		StateKey: authoidc.DeriveStateKey(cfg.JWTSecret),
	})
}
```

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
rtk go test ./internal/platform/authprovider/... ./internal/platform/httpapi -run 'AdminIdentity|OIDC|State|PKCE' -count=1
rtk git diff --check
```

Expected: PASS with no raw fixture secret in test failures or serialized values.

Commit:

```bash
rtk git add go.mod go.sum internal/platform/httpapi/admin_identity.go internal/platform/authprovider
rtk git commit -m "feat: add admin oidc resolver"
```

---

### Task 3: Explicit Admin Identity Binding, Principal Validation, And Runtime Readiness

**Files:**
- Modify: `internal/platform/httpapi/admin_identity.go`
- Create: `internal/platform/httpapi/admin_identity_test.go`
- Modify: `internal/platform/adminresource/principal.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `cmd/platform-api/main.go`

**Interfaces:**
- Produces: `AdminIdentityBindingStore` with resolve, provision, and readiness methods.
- Produces: `NewResourceAdminIdentityBindingStore(*adminresource.Store, func() time.Time) AdminIdentityBindingStore`.
- Produces: `ValidateAdminPrincipal(*adminresource.Store, string) (rbac.Principal, error)`.
- Produces: `ValidateAdminAuthReadiness(context.Context, []capability.Manifest, AdminIdentityBindingStore, bool) error`.
- Consumes: provider audiences and Admin identity output from Tasks 1-2.

- [ ] **Step 1: Write failing binding and principal tests**

Create tests for exact issuer/subject hash lookup, no auto-create, disabled/duplicate/conflicting binding rejection, disabled/missing user rejection, no-effective-permission rejection, and successful `lastLoginAt` update.

Use this contract:

```go
type AdminIdentityBindingInput struct {
	Provider        capability.AuthProvider
	Issuer          string
	ProviderSubject string
	Now             time.Time
}

type AdminIdentityBinding struct {
	Username string
}

type AdminIdentityProvisionInput struct {
	Provider        capability.AuthProvider
	Issuer          string
	ProviderSubject string
	Username        string
	Now             time.Time
}
```

- [ ] **Step 2: Run binding tests and verify failure**

Run:

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/adminresource -run 'AdminIdentityBinding|AdminPrincipal|Disabled.*User|AdminAuthReadiness' -count=1
```

Expected: FAIL because the binding store and enabled-user validator do not exist.

- [ ] **Step 3: Implement hash-only binding behavior**

Hash normalized values:

```go
func adminProviderSubjectHash(provider capability.AuthProvider, issuer, subject string) string {
	normalized := strings.Join([]string{
		strings.TrimSpace(provider.ID),
		strings.TrimSpace(provider.Kind),
		strings.TrimSpace(issuer),
		strings.TrimSpace(subject),
	}, "\x00")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
```

Persist only `issuerHash`, `providerSubjectHash`, `platformUsername`, provider metadata, status, and timestamps. Resolve requires exactly one enabled matching record. Provision is idempotent only for the same tuple and rejects every conflicting mapping.

- [ ] **Step 4: Implement shared Admin principal validation**

Add a store lookup that verifies the user record status before calling `CurrentPrincipal`, then require a non-empty user ID and at least one effective permission. Reuse this validator for demo and OIDC login so disabled demo users are denied as well.

- [ ] **Step 5: Implement data-aware runtime readiness**

When demo is disabled, require at least one enabled/configured Admin-audience provider. For `oidc`, require at least one enabled binding that resolves to an enabled platform user with effective permissions. Return only aggregate startup errors; do not include subject hashes or usernames.

Call readiness after `AdminResourcesFromConfig` and resolver construction but before `httpapi.New` and `server.Run`.

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
rtk go test ./internal/platform/httpapi ./internal/platform/adminresource ./cmd/platform-api -count=1
rtk git diff --check
```

Expected: PASS.

Commit:

```bash
rtk git add internal/platform/httpapi/admin_identity.go internal/platform/httpapi/admin_identity_test.go internal/platform/adminresource cmd/platform-api/main.go
rtk git commit -m "feat: bind oidc identities to admin users"
```

---

### Task 4: Generic Admin OIDC HTTP Start And Exchange

**Files:**
- Modify: `internal/platform/httpapi/server.go`
- Modify: `internal/platform/httpapi/server_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `resources/generated/openapi.admin.json`
- Modify: `scripts/validate-platform-admin-api-boundary.mjs`
- Modify: `scripts/platform-admin-api-boundary.test.mjs`

**Interfaces:**
- Produces: `POST /api/auth/providers/:provider/start`.
- Extends: `POST /api/auth/login` with `state` and `codeVerifier`.
- Adds: `ServerOptions.AdminIdentityResolver` and `AdminIdentityBindings`.
- Consumes: resolver, binding, and principal contracts from Tasks 2-3.

- [ ] **Step 1: Write failing HTTP tests**

Add tests for:

- Admin discovery returns demo/oidc but not App-only WeChat;
- App login rejects Admin-only OIDC;
- start rejects non-Admin, disabled, unconfigured, missing resolver, and malformed challenge;
- start returns authorization URL, exact state, and expiry without credentials;
- exchange rejects missing state/verifier, resolver error, missing binding, and disabled principal;
- exchange success returns the existing Admin token/principal shape, publishes invalidation, and records redacted audit;
- demo login still works and now rejects disabled users.

Use a test resolver:

```go
type adminIdentityResolverFunc struct {
	start func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	resolve func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)
}
```

- [ ] **Step 2: Run HTTP tests and verify failure**

Run:

```bash
rtk go test ./internal/platform/httpapi -run 'AuthProviders|AdminOIDC|AuthLogin|AppAuthLoginRejectsAdmin' -count=1
```

Expected: FAIL because audience filtering, start route, and exchange orchestration are absent.

- [ ] **Step 3: Add route, request types, and audience filtering**

Add:

```go
api.POST("/auth/providers/:provider/start", s.authProviderStart)
```

Extend login input:

```go
type authLoginRequest struct {
	Provider     string `json:"provider"`
	Username     string `json:"username"`
	Code         string `json:"code"`
	State        string `json:"state"`
	CodeVerifier string `json:"codeVerifier"`
}
```

Filter Admin discovery and Admin login with `SupportsAudience(AuthProviderAudienceAdmin)`. Filter App login independently with `AuthProviderAudienceApp`.

- [ ] **Step 4: Reuse one Admin credential issuance helper**

Extract the existing session/JWT/audit path:

```go
func (s *Server) issueAdminLogin(ctx *gin.Context, principal rbac.Principal, provider capability.AuthProvider) {
	issued, err := s.sessions.Issue(principal.User.Username)
	// preserve existing error normalization, invalidation, tokenType=admin,
	// tenant=platform, auth.login audit, and response shape.
}
```

The demo branch validates the Admin principal and calls the helper. The OIDC branch resolves the provider identity, resolves the explicit binding, validates the Admin principal, and calls the same helper.

- [ ] **Step 5: Generate and validate OpenAPI and Admin API boundary**

Add the start request/response schemas and the two login exchange fields. Run:

```bash
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node --test scripts/platform-admin-api-boundary.test.mjs
```

Expected: generated OpenAPI exposes no secret response fields and the boundary validator passes.

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
rtk go test ./internal/platform/httpapi ./cmd/platform-api -count=1
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node --test scripts/platform-admin-api-boundary.test.mjs
rtk git diff --check
```

Commit:

```bash
rtk git add internal/platform/httpapi cmd/platform-api scripts/generate-admin-openapi.mjs scripts/validate-platform-admin-api-boundary.mjs scripts/platform-admin-api-boundary.test.mjs resources/generated/openapi.admin.json
rtk git commit -m "feat: expose admin oidc login flow"
```

---

### Task 5: Explicit OIDC Binding Provisioning Command

**Files:**
- Create: `cmd/platform-admin/main.go`
- Create: `cmd/platform-admin/main_test.go`
- Modify: `docs/platform-auth.md`
- Modify: `deploy/env/production.example.env`
- Modify: `deploy/compose/docker-compose.prod.yml`

**Interfaces:**
- Produces: `platform-admin bind-admin-oidc --provider oidc --issuer <issuer> --username <username> --subject-stdin`.
- Consumes: `AdminIdentityBindingStore.ProvisionAdminIdentityBinding` and runtime/bootstrap constructors.

- [ ] **Step 1: Write failing CLI tests**

Factor execution as:

```go
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config) error
```

Test missing `--subject-stdin`, empty stdin, unknown provider, incomplete OIDC config, missing/disabled user, no permissions, idempotent success, conflict rejection, and raw-subject output redaction.

- [ ] **Step 2: Run CLI tests and verify failure**

Run:

```bash
rtk go test ./cmd/platform-admin -count=1
```

Expected: FAIL because the command does not exist.

- [ ] **Step 3: Implement the minimal operator command**

The command loads capability manifests and the persistent Admin resource store, reads the subject from stdin, calls the exported provision method, records a redacted audit entry when available, and prints only provider ID plus platform username on success.

Reject `--subject` and positional subject input so raw subjects cannot enter normal process arguments.

- [ ] **Step 4: Document production initialization and configuration**

Update `docs/platform-auth.md` with the exact safe command, required pre-start order, idempotency, conflict behavior, and redaction boundary. Add OIDC variables to production env and Compose without real values.

- [ ] **Step 5: Run focused tests and commit**

Run:

```bash
rtk go test ./cmd/platform-admin ./internal/platform/httpapi -run 'Provision|BindAdminOIDC|AdminIdentity' -count=1
rtk git diff --check
```

Commit:

```bash
rtk git add cmd/platform-admin docs/platform-auth.md deploy/env/production.example.env deploy/compose/docker-compose.prod.yml
rtk git commit -m "feat: provision admin oidc bindings"
```

---

### Task 6: Accessible Provider-Specific Admin Login Experience

**Required skills before editing:**
- `product-design:index` and the applicable Product Design context/audit workflow.
- `ui-ux-pro-max` for component, responsive, and production UI rules.
- `fixing-accessibility` for keyboard, focus, status, and accessible-name checks.

**Files:**
- Modify: `admin/src/platform/api/client.ts`
- Modify: `admin/src/platform/auth/AdminLoginView.tsx`
- Modify: `admin/src/platform/refine/authProvider.ts`
- Modify: `admin/src/App.tsx`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`
- Modify: `scripts/admin-ui-contracts.test.mjs`
- Update evidence: `design-qa.md`

**Interfaces:**
- Produces: `startAdminAuthProvider(provider, codeChallenge)`.
- Produces: `beginOIDCLogin(provider)` and `consumePendingOIDCLogin(search)` helpers.
- Extends: `AuthLoginInput` with `state` and `codeVerifier`.
- Consumes: start/exchange HTTP contracts from Task 4.

- [ ] **Step 1: Capture current Product Design evidence and define UI states**

Capture the current login page at 390x844 and 1280x720. Record these states in `design-qa.md`: provider list, demo form, OIDC action, callback progress, callback failure, and recovery. Do not redesign the page as a landing or marketing surface.

- [ ] **Step 2: Write failing UI contract tests**

Add negative fixtures that require:

- `AuthProvider.audiences`;
- Web Crypto verifier/challenge generation;
- tab-scoped `sessionStorage` pending transaction;
- exact state comparison;
- `history.replaceState` before exchange;
- demo-only username form;
- OIDC-only full-width action;
- no disabled password field for OIDC;
- `aria-live="polite"` callback status;
- error-heading focus and recovery action;
- duplicate-submit prevention;
- matching Chinese/English keys;
- reduced-motion and 44px mobile action rules.

- [ ] **Step 3: Run UI tests and verify failure**

Run:

```bash
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
```

Expected: FAIL on the missing OIDC flow contracts.

- [ ] **Step 4: Implement the API and transaction helpers**

Use Web Crypto:

```ts
const verifierBytes = crypto.getRandomValues(new Uint8Array(32));
const verifier = base64URL(verifierBytes);
const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
const codeChallenge = base64URL(new Uint8Array(digest));
```

Store only `{ provider, state, codeVerifier, expiresAt }` in `sessionStorage`. On callback, read `code` and `state`, immediately replace the URL with the login route, then compare exact state and expiry before posting the exchange. Clear the transaction on success, terminal error, or explicit recovery.

- [ ] **Step 5: Implement provider-specific rendering and accessibility**

Render the username form only for `kind === "demo"`. Render one OIDC action for `kind === "oidc"`. During callback exchange, replace form controls with stable status content and disable duplicate actions.

Use a polite live region for progress, focus an error heading with `tabIndex={-1}` after failure, keep all mobile actions at least 44px high, and suppress non-essential transitions under reduced motion.

- [ ] **Step 6: Run UI validation and build**

Run:

```bash
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
```

Expected: PASS.

- [ ] **Step 7: Commit the Admin UI slice**

```bash
rtk git add admin/src scripts/validate-admin-ui-contracts.mjs scripts/admin-ui-contracts.test.mjs design-qa.md
rtk git commit -m "feat: add admin oidc login experience"
```

---

### Task 7: Generated Contracts, Production Hardening, Task Graph, And Closeout Evidence

**Files:**
- Modify generated Admin resource and capability artifacts under `resources/generated/`
- Modify: `resources/platform-production-auth-hardening.json`
- Modify: `resources/platform-production-readiness.json`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `scripts/validate-platform-production-auth-hardening.mjs`
- Modify: `scripts/platform-production-auth-hardening.test.mjs`
- Modify relevant task graph, readiness, alignment, goal, objective, closeout, and engineering validator tests
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/admin-ui-foundation.md`

**Interfaces:**
- Produces: provider promotion matrix entry `oidc`.
- Produces: implemented task node `production-admin-oidc-auth` after evidence exists.
- Preserves: production promotion review remains `not-approved`, runtime mutation disabled, refresh-token-family disabled.

- [ ] **Step 1: Add failing production hardening and task graph mutation tests**

Require the OIDC matrix entry, exact config keys, Admin resolver boundary, audience isolation, state/nonce/PKCE controls, binding and disabled-user controls, secret ownership, rotation runbook, normalized errors, and redaction evidence. Require the task node and 37 implemented/0 pending/0 blocked counts only after its evidence lists are populated.

- [ ] **Step 2: Run governance tests and verify failure**

Run:

```bash
rtk node --test scripts/platform-production-auth-hardening.test.mjs scripts/platform-foundation-task-graph.test.mjs scripts/platform-node-closeout-audit.test.mjs
```

Expected: FAIL on missing OIDC matrix and task node.

- [ ] **Step 3: Regenerate platform contracts**

Run:

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk go run ./cmd/platform-contracts audit --output resources/generated/platform-capability-audit.json
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-platform-operations-plan.mjs
rtk node scripts/generate-production-auth-promotion-review.mjs
```

- [ ] **Step 4: Update production and governance contracts**

Add `oidc` to the promotion matrix but keep external promotion evidence blocked. Add the task node with dependencies and visual design gates from the design spec. Update all exact counts from 36 to 37 only after source, automated, production-like OIDC, browser, and cleanup evidence exist.

- [ ] **Step 5: Run focused governance validation**

Run:

```bash
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-production-auth-hardening.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
```

Expected: PASS while promotion remains not approved and refresh-token-family remains disabled.

- [ ] **Step 6: Commit governance evidence**

```bash
rtk git add resources scripts docs/platform-foundation-task-map.md docs/platform-roadmap.md docs/admin-ui-foundation.md
rtk git commit -m "docs: govern production admin oidc auth"
```

---

### Task 8: Production-Like OIDC Rehearsal, Browser Acceptance, Final Review, And Workspace Closeout

**Files:**
- Create redacted local evidence under: `tmp/product-design/production-admin-oidc-auth-20260711/`
- Update: `design-qa.md`
- Update: `resources/platform-node-closeout-audit.json`
- Update other governance artifacts only if final evidence references change

**Interfaces:**
- Consumes: complete backend, CLI, Admin UI, deployment, and governance work from Tasks 1-7.
- Produces: final 37/37/0 implementation evidence and a clean worktree.

- [ ] **Step 1: Run the full automated verification baseline**

Run:

```bash
rtk go test ./...
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk node scripts/validate-platform-reference-coverage.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-production-auth-hardening.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
```

Expected: every command exits zero.

- [ ] **Step 2: Start a local production-like OIDC provider and platform runtime**

Use a local Keycloak-compatible provider with test-only credentials outside tracked files. Configure one realm/client/test subject, provision the subject through `platform-admin --subject-stdin`, start API and Admin dev servers, and record the exact redacted commands and ports in `design-qa.md`.

Do not claim actual production promotion; this is local production-like protocol evidence only.

- [ ] **Step 3: Run browser acceptance at six viewports**

Verify successful login, cancellation, invalid state, expired transaction, missing binding, disabled user, retry, logout, refresh, protected navigation, keyboard flow, live announcements, focus recovery, reduced motion, zero page overflow, and zero new console errors at 375x812, 390x844, 768x1024, 1024x768, 1280x720, and 1440x1024.

Before saving evidence, scan screenshots, URLs, console output, network summaries, and logs for codes, subjects, claims, tokens, state, nonce, verifier, and credentials. Store only redacted evidence.

- [ ] **Step 4: Run neat-freak closeout**

Invoke `neat-freak`. Reconcile code, generated artifacts, `docs/platform-auth.md`, roadmap/task-map/Admin UI documentation, design QA, task graph counts, closeout evidence, and skill/rule compliance. Do not alter unrelated documentation.

- [ ] **Step 5: Request two-stage code review**

Invoke `superpowers:requesting-code-review`. Review first for spec compliance, then for security/correctness/maintainability. Resolve every Critical and Important finding and rerun the narrow checks affected by each fix.

- [ ] **Step 6: Run verification-before-completion and refresh CodeGraph**

Invoke `superpowers:verification-before-completion`, rerun the complete automated baseline, then:

```bash
rtk codegraph sync .
rtk codegraph status
rtk git diff --check
rtk git status --short --branch
```

Expected: CodeGraph up to date and no uncommitted changes after the final commit.

- [ ] **Step 7: Commit final evidence and report**

```bash
rtk git add design-qa.md resources docs scripts
rtk git commit -m "test: verify production admin oidc auth"
rtk git status --short --branch
```

Expected: clean worktree. Report all commits, verification commands, browser evidence paths, production-promotion limits, and any residual external approval requirement.

