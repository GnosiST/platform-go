# Production Admin OIDC Auth Design

Date: 2026-07-11
Status: approved for implementation

## Goal

Provide a production-capable Admin login path when demo authentication is disabled, while preserving the existing platform JWT, server-side session, Casbin RBAC, capability-manifest, audit, and Admin resource contracts.

The implementation adds OIDC only as an authentication adapter. It does not make the identity provider authoritative for platform users, roles, permissions, organization scope, area scope, or tenant scope.

## Approved Direction

Use an optional `admin-oidc` capability with a composition-root-injected `httpapi.AdminIdentityResolver`, Authorization Code flow with PKCE, signed state, nonce validation, explicit Admin identity binding, and the existing Admin session/JWT response.

This direction was selected over two alternatives:

- Direct email or username claim mapping was rejected because mutable or recycled claims are weaker account identifiers and make issuer collisions, account recovery, and deprovisioning harder to govern.
- Local password, reset, lockout, MFA, and recovery support was rejected because it would make the reusable foundation responsible for password lifecycle and substantially expand the security boundary.

The OIDC callback does not place the platform JWT in a URL. The browser receives the standard short-lived authorization code and state from the identity provider, removes them from the visible URL before exchange, and receives the platform bearer token only in the existing JSON login response.

## Current Problem

Production runtime requires `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`, and the API filters the demo provider from both discovery and login. The current Admin login handler supports only provider kind `demo`; every other kind returns `AUTH_PROVIDER_UNSUPPORTED`.

The built-in WeChat resolver is an App authentication adapter. It resolves App identities behind `httpapi.AppIdentityResolver`, persists App-specific bindings, and issues App credentials. It must not be reused as an Admin authentication path because Admin and App token types, tenants, users, authorization, and account lifecycle are intentionally separate.

The production Compose baseline can therefore start successfully while exposing no usable Admin login provider. This task closes that runtime gap.

## Scope

### 1. Capability And Provider Contract

Add an optional `admin-oidc` capability with dependencies on `identity`, `session`, and `audit`.

The capability declares one built-in Admin provider:

- provider ID: `oidc`;
- provider kind: `oidc`;
- enabled by declaration;
- configured only when the required OIDC runtime settings are complete;
- intended for Admin login only, not App login.

Extend `capability.AuthProvider` with an explicit audience list containing only `admin` and/or `app`. The declaration validator requires at least one known, unique audience. Existing providers are classified as follows:

- `demo`: `admin` and `app`;
- `wechat`: `app` only;
- `oidc`: `admin` only.

`GET /api/auth/providers` returns only providers with the `admin` audience. Admin login rejects providers without the `admin` audience even if they are enabled and configured. App login independently rejects providers without the `app` audience. This closes the existing ambiguity where provider declarations are shared but the two credential domains must remain isolated.

Required configuration:

- `PLATFORM_ADMIN_OIDC_ISSUER_URL`;
- `PLATFORM_ADMIN_OIDC_CLIENT_ID`;
- `PLATFORM_ADMIN_OIDC_CLIENT_SECRET`;
- `PLATFORM_ADMIN_OIDC_REDIRECT_URL`.

Optional configuration:

- `PLATFORM_ADMIN_OIDC_SCOPES`, defaulting to `openid,profile,email`.

The redirect URL is an exact configured value. Request input cannot override it. Provider discovery exposes provider metadata and configured status but never returns credentials, issuer metadata containing secrets, redirect state, nonce, PKCE material, authorization codes, ID tokens, or access tokens.

The provider remains optional in development profiles. The production example enables `admin-oidc` and supplies placeholder environment variables without committing real credentials.

### 2. Admin Identity Resolver Boundary

Introduce an Admin-specific platform port rather than extending the hard-coded provider switch with OIDC protocol logic.

The HTTP layer owns generic Admin login orchestration:

- provider discovery and enabled/configured checks;
- request validation;
- platform identity-binding lookup;
- platform principal validation;
- existing server-side session issue;
- existing Admin JWT signing;
- existing session invalidation publication;
- audit recording and normalized HTTP errors.

`httpapi.AdminIdentityResolver` owns provider protocol behavior:

- building an authorization URL for the declared provider;
- signing and validating the short-lived state envelope;
- issuing and validating nonce values;
- exchanging the authorization code with the configured client credentials and PKCE verifier;
- validating issuer, signature, audience, expiry, nonce, and provider subject;
- returning only the trusted provider subject required by the binding layer.

The built-in implementation lives under `internal/platform/authprovider/oidc` and uses `github.com/coreos/go-oidc/v3/oidc` plus `golang.org/x/oauth2`. Resolver construction remains in the process composition root through `internal/platform/authprovider`, matching the existing App provider pattern.

Provider failures cross the resolver boundary as normalized sentinel errors. Raw provider response bodies, authorization codes, PKCE verifiers, client credentials, ID tokens, access tokens, refresh tokens, issuer subjects, email claims, and upstream error descriptions must not appear in HTTP responses or audit records.

### 3. Authorization Start And Callback Exchange

Add a public start endpoint:

```text
POST /api/auth/providers/:provider/start
```

Request body:

```json
{
  "codeChallenge": "base64url-sha256-challenge"
}
```

Response data:

```json
{
  "authorizationUrl": "https://identity.example/authorize?...",
  "state": "opaque-signed-state",
  "expiresAt": "2026-07-11T12:00:00Z"
}
```

The Admin client generates a high-entropy PKCE verifier with Web Crypto, derives the S256 challenge, calls the start endpoint, stores only the pending provider, exact state, verifier, and expiry in tab-scoped `sessionStorage`, and redirects the browser to `authorizationUrl`.

The signed state envelope contains only the minimum non-secret transaction claims:

- provider ID;
- nonce;
- PKCE challenge;
- issued-at and expiry timestamps;
- a random transaction identifier.

State is authenticated with a dedicated HMAC key derived from the configured platform JWT secret and a distinct context label. The state lifetime is five minutes. The server rejects invalid signatures, expiry, provider mismatch, malformed claims, unsupported PKCE methods, and challenge/verifier mismatches.

The identity provider redirects to the exact configured Admin redirect URL with the standard `code` and `state` query parameters. The Admin callback view reads the values, immediately calls `history.replaceState` to remove them from the visible URL and browser history, validates the pending tab-scoped transaction, and submits the exchange through the existing login endpoint with these additional fields:

```json
{
  "provider": "oidc",
  "code": "authorization-code",
  "state": "opaque-signed-state",
  "codeVerifier": "pkce-verifier"
}
```

The authorization code is the identity provider's single-use exchange credential. A separate platform exchange ticket is not introduced because it would require an additional authoritative transient store without improving the final token transport: the platform JWT already stays in the response body, while code single-use, signed state, nonce validation, PKCE binding, exact redirect matching, and immediate URL cleanup cover the callback boundary.

### 4. Explicit Admin Identity Binding

Add an `admin-identities` Admin resource owned by `admin-oidc`. A binding maps one OIDC identity to one existing platform Admin username.

Persisted fields:

- provider ID;
- provider kind;
- issuer hash;
- provider subject hash;
- platform username;
- status;
- created time;
- last login time;
- optional operator-facing description.

The subject hash is derived from normalized provider ID, issuer URL, and OIDC `sub`. The issuer and subject are not persisted in raw form. ID tokens, access tokens, refresh tokens, authorization codes, PKCE material, state values, nonce values, client secrets, emails, and other provider claims are never stored in the generic resource.

Binding lookup is exact and deny-by-default:

1. Hash the validated issuer and subject.
2. Find one enabled binding for the configured provider and hashes.
3. Resolve the referenced platform user.
4. Require the platform user to exist and have status `enabled`.
5. Calculate the existing principal from platform roles and permissions.
6. Require at least one effective permission after deny rules.
7. Update only `lastLoginAt` after successful identity and principal validation.

The foundation does not automatically create users, assign roles, copy identity-provider groups, or derive authorization from OIDC claims. Missing, disabled, duplicate, or invalid bindings fail with one normalized credential error that does not disclose which account condition failed.

### 5. First Administrator Provisioning

Explicit pre-binding creates a bootstrap requirement because production cannot use demo login. Add a narrow operator command under `cmd/platform-admin`:

```text
printf '%s' "$OIDC_SUBJECT" | platform-admin bind-admin-oidc --provider oidc --issuer <issuer> --username <platform-user> --subject-stdin
```

The command:

- loads the same runtime configuration and persistent Admin resource repository as the API process;
- requires the `admin-oidc` capability and complete OIDC configuration;
- requires an existing enabled platform user with effective permissions;
- reads the raw subject from standard input without echoing it or accepting it as a command-line flag;
- hashes issuer and subject in memory;
- creates or verifies one enabled binding without persisting or printing the raw subject;
- is idempotent for the same provider, issuer, subject, and username;
- rejects conflicting bindings;
- records a redacted provisioning audit event when the audit resource is available.

The command never runs automatically during API startup. Deployment automation must execute it explicitly before starting a production runtime that has no existing enabled Admin binding. This keeps production data mutation operator-controlled and avoids environment-variable-based automatic role or account provisioning.

After the API opens the persistent Admin resource store, the composition root performs an Admin authentication readiness check. When demo authentication is disabled, startup fails unless at least one enabled, configured Admin provider exists and at least one enabled binding resolves to an enabled platform user with effective permissions. This is a data-aware readiness check and complements `Config.ValidateRuntime()`, which continues to validate only static configuration.

### 6. Existing Session, JWT, RBAC, And Audit Contracts

After successful OIDC resolution and binding, login uses the same Admin credential path as demo login:

1. Load the existing platform principal.
2. Issue the existing server-side session.
3. Publish the existing session invalidation event.
4. Sign the existing `tokenType=admin` JWT with tenant `platform`.
5. Record `auth.login` with the provider ID and credential-free audit metadata.
6. Return the existing token, expiry, and principal response.

The shared Admin principal validation requires the platform user to be enabled and to have effective permissions for both demo and OIDC login. This closes the current demo-path gap where `CurrentPrincipal` alone does not reject a disabled user, without changing how roles, deny rules, or Casbin policies are calculated.

OIDC does not change refresh, logout, current-session, menu, resource authorization, API-token, App-token, or refresh-token-family behavior. The independent refresh-token-family runtime remains disabled. Redis remains an invalidation and cache optimization, not the source of truth for OIDC identity binding or Admin session validity.

Add redacted audit coverage for:

- successful OIDC login;
- rejected OIDC login;
- binding provisioning;
- binding conflict;
- provider configuration or exchange failure categories.

Audit fields may include provider ID, normalized outcome, platform username only after successful binding, and transaction trace ID. OIDC audit records must not store the raw session handle, its digest, or any shortened derivative. They must not include provider subjects, raw issuer values, claims, authorization codes, PKCE material, state, nonce, provider tokens, or credentials. The persisted OIDC audit schema does not expose a `sessionId` field.

### 7. Production Runtime Gate

Static validation rejects production configuration when:

- demo authentication is not disabled;
- `admin-oidc` is enabled but required OIDC settings are incomplete;
- only part of the OIDC credential pair is configured;
- the redirect URL is not absolute HTTPS, except loopback HTTP in development and test;
- OIDC scopes omit `openid`;
- demo is disabled and no configured Admin-capable provider is enabled.

The production Compose and environment example enable `admin-oidc`, exclude `demo-data`, keep demo authentication disabled, and require issuer, client ID, client secret, and redirect URL variables.

The provider promotion matrix adds `oidc` with:

- composition-root resolver evidence;
- configured-provider-only discovery and exchange;
- OIDC issuer, signature, audience, nonce, state, PKCE, and redirect validation;
- subject hashing and raw-subject prohibition;
- explicit identity binding and disabled-user rejection;
- credential owner and rotation-runbook requirements;
- normalized provider errors and audit redaction;
- production-like runtime rehearsal evidence requirement.

This implementation does not mark external production promotion evidence as approved. Existing non-mutating approval packages, provider rotation requirements, and rollback requirements remain blocking for an actual production promotion decision.

### 8. Admin Login Experience

The login page remains a quiet operational product surface rather than a marketing redesign.

Provider-specific behavior:

- demo renders the existing username form only when demo is available;
- OIDC renders one full-width provider action using the provider's localized title;
- unavailable providers remain visible but disabled with localized configuration status;
- callback processing replaces the form with a stable progress state;
- callback, state, timeout, configuration, and generic credential failures provide a localized recovery action;
- returning to the provider list clears stale callback transaction state;
- repeated submission and redirect actions are disabled while pending.

Accessibility and responsive requirements:

- provider actions use native buttons with explicit accessible names;
- callback progress and errors use a polite `aria-live` status region;
- focus moves to the error heading when callback recovery is required;
- keyboard order follows provider selection, provider action, language, and theme controls without hidden disabled fields;
- no disabled password field is rendered for OIDC;
- controls remain at least 44px high on mobile;
- 375px through 1440px layouts have no overlap or horizontal overflow;
- reduced-motion preferences suppress non-essential login transitions.

Implementation uses Product Design evidence as the primary UI workflow input, `ui-ux-pro-max` for implementation-level component and responsive quality, and `fixing-accessibility` for keyboard, focus, status, and accessible-name validation. `design-taste-frontend` remains intentionally unused because this is a dense Admin authentication workflow, not a landing, portfolio, marketing, or brand-redesign surface.

## Error Contract

Public errors remain stable and sanitized. The implementation adds or reuses codes in these categories:

- provider not found, disabled, or not configured;
- invalid start request or PKCE challenge;
- invalid, expired, or mismatched OIDC transaction;
- provider exchange or verification failure;
- invalid or unavailable Admin identity binding;
- invalid platform principal;
- session, token, or audit failure.

Binding absence, disabled binding, unknown user, disabled user, and missing effective permissions return the same external credential error. Provider-specific response bodies and internal validation details are logged only through sanitized categories, not copied into responses.

## Components And Files

Expected primary implementation surfaces:

- `internal/platform/httpapi/admin_identity.go`: Admin resolver and binding ports plus resource-backed binding behavior.
- `internal/platform/httpapi/server.go`: start route, generic OIDC exchange orchestration, existing Admin credential issuance reuse, and normalized errors.
- `internal/platform/authprovider/oidc/`: OIDC discovery, authorization URL, signed state, code exchange, token verification, nonce, and PKCE behavior.
- `internal/platform/authprovider/resolver.go`: composition-root resolver construction.
- `internal/platform/config/config.go`: OIDC settings and static runtime validation.
- `internal/platform/bootstrap/capabilities.go`: configured provider status.
- `internal/platform/core/capabilities.go`: optional capability, provider declaration, and `admin-identities` resource.
- `cmd/platform-api/main.go`: resolver injection and data-aware Admin auth readiness check.
- `cmd/platform-admin/`: explicit OIDC identity-binding provisioning command.
- `admin/src/platform/api/client.ts`: start and exchange contracts plus tab-scoped transaction handling helpers.
- `admin/src/platform/auth/AdminLoginView.tsx`: provider-specific login and callback states.
- `admin/src/platform/refine/authProvider.ts`: OIDC-compatible login input without changing session checks.
- `admin/src/platform/i18n.ts` and `admin/src/styles.css`: matching Chinese/English text, accessible status, focus, responsive, and reduced-motion behavior.
- provider, capability, resource, OpenAPI, UI contract, production-auth-hardening, production-readiness, task-graph, and closeout validators and tests.

Exact file splits may follow existing local package boundaries during implementation, but the resolver, binding, HTTP orchestration, provisioning command, and UI state responsibilities must remain separate.

## Testing And Acceptance

### Backend Unit And Integration Tests

Cover:

- complete and incomplete OIDC configuration;
- provider discovery with configured and unconfigured OIDC;
- Admin discovery excluding App-only providers and App login excluding Admin-only providers;
- valid start response and exact redirect URL;
- malformed challenge and unsupported PKCE method rejection;
- state signature, expiry, provider mismatch, and challenge mismatch rejection;
- issuer, signature, audience, expiry, nonce, and subject validation;
- authorization code exchange failure normalization;
- raw provider error, claim, subject, token, code, state, nonce, verifier, and credential redaction;
- missing, disabled, duplicate, and conflicting binding rejection;
- missing and disabled platform user rejection;
- disabled demo user rejection through the shared Admin principal validator;
- user with no effective permissions rejection;
- successful binding resolution, last-login update, existing session issue, Admin JWT claims, Casbin permissions, invalidation publication, and audit event;
- App JWT and App identity paths remaining separate;
- idempotent provisioning and conflicting provisioning rejection;
- production startup rejection without a usable provider or binding;
- production startup acceptance with a configured provider and valid binding.

### Contract And Governance Tests

Regenerate and validate Admin resource, OpenAPI, capability audit, provider promotion matrix, production readiness, Admin API boundary, task graph, task execution, goal completion, alignment, objective conformance, and node closeout artifacts.

The new implemented task node is `production-admin-oidc-auth`. It:

- depends on `production-auth-provider-hardening`, `production-persistence-correctness`, and `admin-ui-system-quality-hardening`;
- locks auth contracts, Admin resource contracts, Admin UI, i18n, deployment configuration, production governance, browser QA, and docs;
- is visual because it changes the user-observable login workflow;
- records `superpowers:brainstorming`, Product Design, `ui-ux-pro-max`, accessibility, automated tests, production-like provider rehearsal, browser evidence, and neat-freak cleanup evidence;
- changes the closed task graph from 36 implemented nodes to 37 implemented nodes with zero pending or blocked nodes only after all evidence exists.

### Admin UI And Browser Acceptance

Automated UI contracts cover provider-specific rendering, absence of irrelevant fields, callback URL cleanup, tab-scoped transaction validation, duplicate-submit prevention, localized status, accessible names, error focus, 44px mobile controls, reduced motion, and no raw callback values in rendered errors.

Browser acceptance uses a local production-like OIDC provider such as Keycloak and fresh evidence at:

- 375x812;
- 390x844;
- 768x1024;
- 1024x768;
- 1280x720;
- 1440x1024.

The browser walkthrough covers successful login, user cancellation, invalid state, expired transaction, missing binding, disabled user, retry, logout, session refresh, protected navigation, mobile callback recovery, keyboard-only operation, and console errors. Captured screenshots and runtime logs must be redacted and must not contain authorization codes, provider subjects, claims, tokens, state, nonce, PKCE material, or credentials.

## Delivery Sequence

1. Add failing contract and configuration tests.
2. Add capability, config, resolver, state/PKCE, and binding contracts.
3. Add failing HTTP orchestration and provisioning tests.
4. Implement generic Admin OIDC start/exchange, binding, existing credential issuance reuse, audit, and readiness.
5. Regenerate contracts and update production hardening and deployment artifacts.
6. Use Product Design, `ui-ux-pro-max`, and `fixing-accessibility` evidence to implement the provider-specific Admin login flow.
7. Run focused Go, Node, Admin build, and governance verification.
8. Run production-like OIDC browser acceptance and collect redacted evidence.
9. Run neat-freak closeout, final code review, full verification, CodeGraph refresh, and workspace cleanliness checks.

## Non-Goals

- No local Admin password database, password reset, password lockout, MFA, recovery code, or account-recovery workflow.
- No automatic user, role, permission, tenant, organization, or area provisioning from OIDC claims or groups.
- No App login change and no reuse of Admin credentials by App endpoints.
- No refresh-token-family enablement and no storage of provider refresh tokens.
- No multi-issuer or dynamically registered provider management in the first implementation.
- No identity-provider logout or global single logout in the first implementation.
- No production secret commits, automatic production promotion, external approval fabrication, or deployment mutation.
- No landing-page, marketing, portfolio, or brand-art-direction redesign.
