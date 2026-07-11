# Platform Auth Provider Contract

Date: 2026-07-04
Last updated: 2026-07-11

## Purpose

Authentication is now declared through capability manifests instead of being hard-coded into the admin shell or business modules.

Enabled capabilities can expose login providers through `capability.Manifest.AuthProviders`. This gives the platform a stable discovery and login API while allowing projects to add or remove providers such as demo login, WeChat login, SSO or password login by changing enabled capabilities.

## Provider Contract

Each provider declares:

- `id`: stable provider id, such as `demo` or `wechat`, using lowercase letters, numbers or hyphens;
- `kind`: provider adapter kind, using lowercase letters, numbers or hyphens;
- `title` and `description`: localized admin/client display text;
- `enabled`: whether the provider is visible through provider discovery;
- `configured`: whether login can be attempted;
- `configKeys`: optional environment-style configuration keys required by the provider, using uppercase letters, numbers or underscores.

Provider ids must be unique across enabled capabilities after trimming whitespace. Missing id, kind, localized title, localized description or malformed/duplicate config keys fail capability resolution.

## APIs

```text
GET /api/auth/providers
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET /api/admin/session/current
POST /api/app/auth/login
POST /api/app/auth/logout
GET /api/app/session/current
POST /api/app/identity/phone-verifications
POST /api/app/identity/phone-bindings
```

`GET /api/auth/providers` returns enabled provider declarations. A provider can be visible but unconfigured; the UI should show or disable it based on `configured`.

`POST /api/auth/login` accepts:

```json
{
  "provider": "demo",
  "username": "ops"
}
```

For the current slice, only the configured `demo` provider returns a session. The response includes a JWT admin bearer token and `expiresAt`. The token carries the platform user, tenant, token type and internal session id. The admin client stores that token and sends it as `Authorization: Bearer ...` for later requests.

The JWT token is the external HTTP credential. The internal session id remains server-side state so TTL, logout and future forced revocation are still authoritative. This is intentionally not a weaker stateless-only session model.

`GET /api/admin/session/current` parses the bearer token, requires `tokenType=admin`, resolves the token's session id in the session store, and only then loads the principal. Default admin runtime does not fall back to an implicit admin user when no bearer token is present. `X-Platform-User` is retained only for tests or explicitly controlled local harnesses that set `httpapi.ServerOptions.AllowInsecureHeaderAuth`; it must not be enabled for the default API process. API tokens are separate integration credentials and must not be accepted as admin session bearer tokens.

`POST /api/auth/refresh` accepts a still-valid admin bearer token, renews the same server-side session to the configured session TTL and returns a newly signed admin JWT plus updated `expiresAt` and current principal. The original JWT remains bounded by its own `exp`; after that timestamp it cannot be used even if the underlying server-side session was renewed. This is sliding session renewal, not a separate refresh-token rotation model.

`POST /api/auth/logout` revokes the server-side session referenced by the bearer token. A revoked, expired, wrong-secret, wrong-type or mismatched token returns `401` instead of falling back to `admin`.

## App Security Domain

The app HTTP domain is intentionally separate from the admin HTTP domain.

- `POST /api/app/auth/login` issues a guest-first app session when no provider is supplied. If no username is supplied, the session username is `guest`.
- The same endpoint can accept `provider` and `code` for configured app login providers. Provider-specific exchange attaches through `httpapi.AppIdentityResolver`; the platform then resolves or creates a generic `app-identities` binding before issuing the app session.
- The app JWT uses `tokenType=app` and tenant `app`, not the admin tenant.
- `GET /api/app/session/current` accepts only app JWT bearer tokens backed by an active server-side session.
- `POST /api/app/auth/logout` revokes the app session referenced by the app bearer token.
- When the optional `app-phone` capability is enabled, `POST /api/app/identity/phone-verifications` creates a short-lived local/demo verification record for the active app session and returns a `debugCode` for development verification. The verification endpoint applies a rolling-window limit per app username, phone hash and purpose; over-limit calls return `APP_PHONE_VERIFICATION_RATE_LIMITED` with HTTP `429`. `POST /api/app/identity/phone-bindings` validates the code and creates a unique phone binding for the app username.
- Admin JWTs and `pgo_` API tokens are rejected by app session APIs with `401`.

This is a runtime boundary, not the final business identity model. The built-in WeChat miniapp adapter performs server-side `jscode2session` exchange behind `httpapi.AppIdentityResolver`; account linking, app/provider token refresh behavior and product-specific app authorization should still attach behind business capability services without allowing admin/app credentials to cross-call each other. The resolver must return a trusted provider subject for configured provider logins. Platform binding persistence maps `provider + subject hash` to a stable app username and stores only `providerSubjectHash` plus `maskedSubject`. Phone binding persistence stores only `phoneHash`, `maskedPhone`, verification timestamps and app username; raw phone numbers and raw verification codes must not appear in app responses after binding, audit records or generic resource rows. Resolver outputs must not expose raw OpenID, UnionID or provider subject material through login responses, audit records or generic admin resource rows.

Business capabilities declare app APIs through `capability.Manifest.App.Routes`. The manifest contract requires static `/api/app/` paths without query strings or fragments, explicit `public` or `session` auth mode, required localized descriptions, and optional `app:<domain>:<action>` permission codes for business app authorization. `cmd/platform-contracts app-routes` exports the enabled manifest surface to `resources/generated/app-route-contract.json`, keeping app APIs discoverable without coupling them to admin resources or admin permission codes.

Handlers attach through the neutral `approute.Registration` contract and are injected into `httpapi.ServerOptions` by process composition roots. The runtime only registers handlers whose method/path are declared by an enabled manifest. Session routes accept only app JWTs, expose the active session through `approute.SessionFromContext(ctx)`, and enforce optional `app:<domain>:<action>` permissions through the shared authorizer with tenant `app`.

## Session Persistence

Sessions are managed through a repository-backed session store.

- Default mode is in-memory.
- `PLATFORM_SESSION_FILE` enables JSON file persistence for issued and revoked sessions.
- `PLATFORM_SESSION_DRIVER` and `PLATFORM_SESSION_DSN` enable the GORM-backed repository for `mysql`, `postgres` and `sqlite`.
- File persistence is useful for local demos and early deployments.
- GORM persistence is selected before file persistence when both are configured.
- The platform repository defines the adapter boundary and creates `platform_sessions` through the shared GORM storage opener.
- When an invalidation bus is configured, successful login, refresh and logout publish a `sessions` invalidation event. Peer API instances reload their repository-backed session store after that event, so issued, renewed and revoked sessions converge across instances that share the same session repository.

## Admin UI Flow

The admin app now starts with a provider-driven login view when no platform token exists in browser storage.

- The view loads branding from `GET /api/platform/branding`.
- The view loads login providers from `GET /api/auth/providers`.
- Configured providers are selectable; unconfigured providers are visible but disabled.
- Demo login accepts a platform username such as `admin` or `ops`.
- Successful login stores the returned token and then loads current session, menus and resource pages.
- Logout calls `POST /api/auth/logout`, clears the stored token and returns to the login view.
- The shared API client exposes `refreshCurrentSession()` for admin sliding renewal. The shell does not run an implicit refresh scheduler yet; products should wire renewal timing through their session policy rather than hiding it in layout code.

The login UI should remain generic. Provider-specific behavior belongs in auth adapters and capability manifests, not in the shell layout.

## Audit Events

Successful login and logout write audit records when the `audit-logs` admin resource is enabled:

- `auth.login`
- `auth.refresh`
- `auth.logout`
- `app.auth.login`
- `app.auth.logout`

The audit record stores actor, action, resource, provider when available, created time and a shortened session id. It does not store the raw bearer token.

Generic admin resource create, update and delete handlers also write `admin_resource.create`, `admin_resource.update` and `admin_resource.delete` records after successful writes. These records store the actor, resource code, target id, target code, target name and created time, and skip the audit resource itself to avoid recursive audit rows. The `audit-logs` resource is read-focused: its runtime schema exposes structured query fields but no create/edit form fields.

## Admin API Tokens

The `system-admin` capability registers the `api-tokens` resource under the security menu. It is for platform-to-platform or tool integrations that need a scoped credential without coupling to a human login session.

- `POST /api/admin/resources/api-tokens` issues a clear `pgo_` token once.
- The persisted record stores `tokenPrefix` for identification and `tokenHash` for future verification. `token`, `tokenHash` and other token material are stripped from create, query and update responses.
- The requested `scope` must match existing permission codes. Unknown scopes are rejected before persistence.
- A valid active `pgo_` token can call protected platform APIs with `Authorization: Bearer ...` only when its scope contains the exact permission required by that endpoint.
- A `pgo_` token is not an admin session credential. `GET /api/admin/session/current`, menu loading and human identity flows still require a JWT admin bearer token.
- Updating an API token preserves `tokenPrefix`, `tokenHash`, `createdAt` and `revokedAt`; callers cannot replace token material through the generic update endpoint.
- `DELETE /api/admin/resources/api-tokens/:id` revokes the token by setting status to `revoked` and retaining the sanitized record.
- `api_token.create`, `api_token.update` and `api_token.revoke` audit events store actor and token prefix, not the raw token.

The admin UI treats the returned token as a one-time secret: it is shown in a modal immediately after creation and must not be recoverable from later list or detail reads.

## Token Rotation Policy

`token-rotation` is a production operation policy in `resources/platform-production-readiness.json`.

Production auth hardening is also tracked in `resources/platform-production-auth-hardening.json` and checked by `rtk node scripts/validate-platform-production-auth-hardening.mjs`. The independent refresh-token-family runtime slice is tracked in `resources/platform-refresh-token-family-promotion.json` and checked by `rtk node scripts/validate-platform-refresh-token-family-promotion.mjs`. `rtk node scripts/generate-production-auth-promotion-review.mjs` generates `resources/generated/production-auth-promotion-review.json` as a non-mutating review packet for the same gate. The matching task graph node is `implemented` for the reusable foundation: the contract gate is active for provider adapter controls, provider runtime policy, credential rotation evidence and the disabled refresh-token-family slice, but the current approved runtime model remains sliding session renewal, not a separate refresh-token rotation model.

The same production auth contract carries `productionPromotionApprovalPackage`. Promotion needs security-owner, platform-architect and operations-owner approval plus a signed session-policy review, separate refresh-token-family runtime spec, credential and provider rotation runbooks, runtime test output, redacted audit samples and rollback evidence. The approval package also declares a completed-evidence artifact schema: future evidence must include artifact URI, hash, approver, reviewed commit, target environment, rollback commands, `rtk` verification commands, audit sample refs, provider rotation runbook refs and refresh-token-family test refs, and approval must not be self-approved by the evidence owner. The generated operations plan carries the same schema so release review cannot drop it. The dedicated production-auth promotion review packet carries the active blocker list and missing evidence list and remains `not-approved` while the package is blocked. The validator keeps this package `blocked`, requires completed evidence to stay empty before promotion and rejects text-only or single-person self approval.

The current foundation supports JWT admin/app bearer tokens backed by server-side sessions, admin sliding renewal and scoped `pgo_` API tokens. It also contains an independent refresh-token-family store, GORM repository, rotation/reuse-detection service and audit-redaction adapter under `internal/platform/refreshtoken`, but that slice is not bound to the default `/api/auth/refresh` path and remains disabled until production approval. Production rotations must therefore be planned around the actual enabled credential classes:

- JWT signing secret rotation affects admin JWTs and app JWTs that have not yet expired.
- Session repository invalidation affects admin and app sessions independently of JWT expiry.
- API-token rotation affects scoped `pgo_` tokens and must never expose raw token values after creation.

Before changing production credential material, record the overlap window, invalidation decision, affected token classes, rollback handling and verification commands. Audit records must identify the operator and scope while omitting raw JWT secrets, raw bearer tokens, OpenID, UnionID, phone numbers and API token values.

## Production Session Policy Gate

`docs/superpowers/specs/2026-07-07-platform-production-session-policy-design.md` is the current production session-policy specification. The hardening validator requires it before the implemented refresh-token-family slice can be enabled in production.

The specification keeps the current runtime boundary explicit: `POST /api/auth/refresh` is sliding renewal of the same server-side session and a newly signed JWT, not offline renewal. The disabled refresh-token-family slice provides separate token-family storage, hashed refresh token values, rotation lineage, replay/reuse detection, affected-family and server-side-session revocation, `sessions` invalidation hooks and redacted audit events. Redis can help convergence but must not become the source of truth for refresh-token-family validity.

The refresh-token-family promotion gate is intentionally blocking default runtime enablement. It must continue to report the current runtime as `sliding-renewal-only` with `notARefreshTokenFamily=true` while `refreshTokenFamily.status=implemented-disabled` and `defaultRuntime=disabled`, until the production approval package, Redis/session convergence evidence, redacted audit samples and rollback plan exist.

Production auth promotion is also blocked while `productionPromotionApprovalPackage.status` is `blocked`. This no longer blocks foundation completion, but it still prevents runtime boundary changes merely because the sliding-renewal runtime, Provider Promotion Matrix or token-rotation documents exist; the external approval artifacts and runtime evidence must be present before the runtime boundary changes.

Provider credential rotation is a separate decision from session-family revocation. Rotating WeChat or another identity-provider secret does not automatically revoke refresh-token families unless the incident review says the provider compromise also affects platform sessions. API-token rotation remains separate from human/app sessions.

## Built-In Providers

The `session` capability declares the configured `demo` provider. It is meant for local development, demos and platform verification.

Production runtime must not enable the `demo-data` capability and must set `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`. `config.Config.ValidateRuntime()` rejects `PLATFORM_RUNTIME_ENV=production` when `PLATFORM_CAPABILITIES` still includes `demo-data` or demo auth remains enabled. `cmd/platform-api` passes that switch into `httpapi.ServerOptions`, and the API filters the `demo` provider from discovery and login when disabled. This keeps local demo surfaces available without letting sample-data loaders or demo login leak into production.

The `wechat-login` capability declares the `wechat` provider as enabled but not configured by default. This is intentional: the platform exposes the capability boundary without requiring every project to carry WeChat credentials. Provider-backed app login must deny unconfigured providers by default; a visible provider is not usable until configuration marks it configured and the process composition root wires the matching `httpapi.AppIdentityResolver`.

When both credentials are set, `bootstrap.CapabilitiesFromConfig` marks the provider configured and the API process wires the built-in WeChat miniapp resolver:

```bash
PLATFORM_WECHAT_MINIAPP_APP_ID=wx...
PLATFORM_WECHAT_MINIAPP_SECRET=...
PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT=https://api.weixin.qq.com/sns/jscode2session
```

`PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT` is optional. Leave it empty to use the official WeChat endpoint. The adapter sends `appid`, `secret`, `js_code` and `grant_type=authorization_code`, returns OpenID/UnionID only through the resolver boundary, and never writes raw provider subjects into login responses, audit records or generic admin resource rows. Provider exchange failures return the normalized `ErrProviderResolveFailed` sentinel with sanitized reason codes only; WeChat `errmsg`, request code values, credentials and provider subjects must not appear in resolver errors or HTTP responses. Platform binding storage remains hash-and-mask only: `providerSubjectHash` and `maskedSubject` are the allowed persisted generic fields, while raw OpenID, UnionID and provider subject material stay outside responses, audit payloads and generic resources.

Production provider promotion must keep adapter registration manifest-declared and composition-root injected. Before enabling additional production providers, add tests for unconfigured-provider rejection, subject redaction, configured-provider-only login and provider error normalization. These are contract gates; they do not require enabling a refresh-token family or adding a provider to the default platform profile.

## Production Admin OIDC Initialization

Production Admin OIDC requires the optional `admin-oidc` capability, complete OIDC configuration and the same persistent Admin resource store used by `cmd/platform-api`. Provisioning is an explicit operator action; API startup never creates users, roles, permissions or OIDC bindings.

For the first production administrator, complete these steps before starting a demo-disabled API process:

1. Configure `PLATFORM_CAPABILITIES` with `admin-oidc`, set all `PLATFORM_ADMIN_OIDC_*` values and point `PLATFORM_ADMIN_RESOURCE_DRIVER` plus `PLATFORM_ADMIN_RESOURCE_DSN` at the production Admin store.
2. Ensure the target platform user already exists, is enabled and resolves to at least one effective permission.
3. Obtain the immutable OIDC `sub` value through the trusted identity-provider administration path and expose it only to the command's standard input.
4. Start the production database and cache without starting the API readiness gate:

```bash
docker compose --env-file deploy/env/production.env \
  -f deploy/compose/docker-compose.prod.yml \
  up -d platform-mysql platform-redis
```

5. Run the binding command once inside the API image and Compose network. The image keeps `platform-api` as its default entrypoint, so the one-shot command must override it explicitly:

```bash
printf '%s' "$OIDC_SUBJECT" | docker compose \
  --env-file deploy/env/production.env \
  -f deploy/compose/docker-compose.prod.yml \
  run --rm --no-deps --entrypoint /app/platform-admin platform-api \
  bind-admin-oidc \
  --provider oidc \
  --issuer "$PLATFORM_ADMIN_OIDC_ISSUER_URL" \
  --username admin \
  --subject-stdin
```

6. Start the API and Admin services:

```bash
docker compose --env-file deploy/env/production.env \
  -f deploy/compose/docker-compose.prod.yml \
  up -d platform-api platform-admin
```

With demo authentication disabled, the API data-aware readiness check rejects startup unless a configured Admin provider has at least one enabled binding to a valid Admin principal.

The command is idempotent for the same provider, issuer, subject and platform username. It rejects a tuple that is already bound to another username and does not replace the existing binding. It also rejects missing, disabled or permissionless users through the shared Admin principal validation path.

The raw subject is accepted only through standard input. `--subject` and positional subject arguments are rejected so the value cannot enter normal process arguments. Success output contains only the provider ID and platform username. Persistence stores issuer and subject hashes, and the provisioning audit contains only the provider ID, outcome and successful platform username; stdout, stderr and audit records do not contain raw issuer or subject values.

Do not add this command to API startup, Compose service startup or automatic deployment promotion. `/app/platform-admin` is bundled only for explicit one-shot operator use against the intended persistent Admin store.

## Provider Promotion Matrix

`resources/platform-production-auth-hardening.json` keeps the production Provider Promotion Matrix. It records each built-in provider's capability owner, runtime boundary, production usage, required controls, config keys and source-backed evidence. The matrix is cross-checked against `resources/generated/platform-capability-audit.json`, so every auth provider declared by enabled capability manifests must have a promotion-matrix entry before production promotion can be claimed.

- `demo` stays `local-harness-only`. It can support local demos and controlled verification, but it is not a production external identity provider, must be disabled in production with `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`, and must not gain raw subject or credential exposure.
- `wechat` is an optional production provider. It requires `PLATFORM_WECHAT_MINIAPP_APP_ID`, `PLATFORM_WECHAT_MINIAPP_SECRET` and the optional `PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT`, and production use requires a credential owner, rotation runbook, configured-provider-only login, unconfigured-provider rejection, subject hash/masking and normalized provider errors.
- New production providers must follow the same matrix before runtime promotion: manifest-declared provider, composition-root-injected resolver, config-key contract, secret owner, rotation runbook, hash-and-mask subject storage, unconfigured-provider rejection test, configured-provider-only login test, provider-error normalization test and audit-redaction test.

The matrix is intentionally stricter than provider discovery. Discovery may show an enabled but unconfigured provider; production use needs explicit configuration plus the matrix evidence.

## Authorization Link

Login does not own authorization. After a provider resolves a platform username, authorization is still calculated by the generic RBAC model:

```text
user -> roles -> permissions / denyPermissions -> menus/resources/actions
```

This keeps menu filtering, backend resource authorization and future button/API permissions independent from the concrete login provider.

Backend resource authorization is now enforced through Casbin policies generated from platform users and roles. `roles.permissions` grants action permissions and `roles.denyPermissions` explicitly denies action permissions; deny matches override wildcard or exact allow matches. The current-session permissions and denied permissions remain useful for frontend controls and visibility, but Casbin is the execution engine for protected HTTP actions.

The current-session `user` also carries `tenantCode`, `orgUnitCode` and `areaCode` when those fields are present on the platform user resource. `tenantCode`, `orgUnitCode` and `areaCode` are consumed by the generic admin resource data-scope filter after Casbin has allowed a read action. Area scopes are explicit role metadata through `current_area`, `current_and_children_areas` or `custom_areas`; none of these fields grants action permissions or changes Casbin checks by itself.

## Current Boundary

The current admin HTTP session credential is a JWT admin bearer token backed by a server-side session with TTL, sliding renewal and revoke support. Memory, file-backed and GORM-backed session stores are available. Repository-backed session stores can be reloaded across API instances through the shared invalidation bus, giving the base platform distributed issue/renew/revoke convergence when instances share the same session repository. API tokens now support scoped Bearer access to protected platform APIs. The app HTTP session credential is a separate JWT app bearer token backed by the same session store contract. App phone verification currently uses a local/demo `debugCode` response, hash-backed generic resources and a local rolling-window abuse guard; production SMS delivery and distributed rate limiting should be added as provider/service adapters without changing the `/api/app/identity/phone-*` contract. Product-specific business API adoption, default-path refresh-token-family enablement, additional provider adapters and normalized identity-binding storage remain later production slices. Production auth should add provider-specific adapters through `httpapi.AppIdentityResolver` without changing provider discovery, login, refresh, logout, current-session and menu APIs.
