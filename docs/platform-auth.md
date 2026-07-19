# Platform Auth Provider Contract

Date: 2026-07-04
Last updated: 2026-07-19

## Purpose

Authentication is now declared through capability manifests instead of being hard-coded into the admin shell or business modules.

Enabled capabilities can expose login providers through `capability.Manifest.AuthProviders`. This gives the platform a stable discovery and login API while allowing projects to add or remove providers such as demo login, WeChat login or SSO by changing enabled capabilities.

## Provider Contract

Each provider declares:

- `id`: stable provider id, such as `demo` or `wechat`, using lowercase letters, numbers or hyphens;
- `kind`: provider adapter kind, using lowercase letters, numbers or hyphens;
- `title` and `description`: localized admin/client display text;
- `enabled`: whether the provider is visible through provider discovery;
- `configured`: whether login can be attempted;
- `configKeys`: optional environment-style configuration keys required by the provider, using uppercase letters, numbers or underscores.

Provider ids must be unique across enabled capabilities after trimming whitespace. Missing id, kind, localized title, localized description or malformed/duplicate config keys fail capability resolution.

## Credential Boundary

The current platform still has no legacy local-password provider kind or production credential repository. `cmd/platform-api` rejects startup when an enabled provider uses the local `password` kind; `credential-auth` uses explicit `credential-password` and `credential-sms-otp` provider kinds instead. Generic Admin field names have no credential semantics: each capability must explicitly declare sensitivity, storage and projection policy, and undeclared or policy-invalid values are rejected before persistence.

Do not add password hashes to `Record.Values`. A future local-password capability requires a separately approved authentication and migration design with an Argon2id password-hashing boundary, dedicated storage, upgrade parameters, reset/rotation behavior, breach response and historical-data migration. Passwords are not reversibly encrypted, and changing hashing policy cannot be treated as an ordinary runtime configuration toggle.

The current browser login flows send demo usernames or provider authorization codes, while `credential-auth` local password and SMS OTP flows use application-layer hybrid encryption for credential secrets. HTTPS is still the production baseline for all credential-bearing requests, but it is not the application-layer credential secret encryption mechanism and cannot be used as a substitute for `secret.encrypted`. The Admin client fetches `GET /api/auth/credential-secret-key`, uses ECDH P-256 to agree on a shared key, derives the encryption key with HKDF-SHA256, encrypts the password or SMS OTP secret with AES-256-GCM/A256GCM, and submits only `secret.encrypted` to `POST /api/auth/login`. When `RequireEncryptedSecrets=true`, the server rejects plaintext `secret.value` or `secret.code`, including requests that try to send those fields alongside an encrypted envelope. JWTs, provider codes and API tokens are credentials and must cross browser-to-server and provider-to-server boundaries over the production HTTPS contract. The Admin client currently keeps its bearer token in browser storage, so CSP and same-origin delivery reduce exposure but do not remove the residual localStorage/XSS risk. Moving credentials to cookies requires a separate CSRF and session-client specification.

## Credential Auth v1

`credential-auth` is the local credential authentication capability. The current package has the contract, documentation, validation gate, internal service foundation in `internal/platform/credentialauth`, dedicated GORM persistence, application-layer hybrid encrypted secret transport and a deliverable v1 HTTP/UI runtime tracked by `resources/platform-credential-auth-v1.json` and `rtk node scripts/validate-platform-credential-auth-v1.mjs`. It preserves the current demo/OIDC runtime, does not enable provider kind `password` in `cmd/platform-api`, and does not replace demo login, Admin OIDC exchange, app login or server-side session issuance flows.

The capability is business-neutral. It now declares provider modes for `username-password`, `phone-password`, `email-password` and `phone-sms-otp`, and the login UI must stay provider-discovery driven rather than hard-coding those four modes. A successful credential login still delegates JWT signing, session persistence, renewal and revocation to the existing `session` boundary, then RBAC and Casbin calculate authorization after login.

The storage boundary is deliberately separate from generic Admin resources: password credentials must not be stored in generic `Record.Values`. The first service foundation implements normalized identifier hashes, an in-memory repository for development/test, a dedicated GORM repository for persistent runtime, Argon2id PHC verification, SMS OTP one-time consumption semantics and a bootstrap runtime seeded from Admin credential environment variables. Production credential-auth requires `PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER`, `PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN`, `PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_KEY_ID` and `PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY`. The dedicated stores remain `auth_identifiers`, `password_credentials`, `credential_challenges` and `sms_otp_challenges`. They store normalized hashes, masked identifiers, Argon2id verifier metadata, digests, expiry, attempt and one-time consumption state, not raw passwords, raw OTP codes, raw challenge answers or raw phone/email values.

Challenge support is scoped to login for v1: `off`, `always`, `after-failure` or `risk-based`, with `captcha` or `slider` as implementation choices. `POST /api/auth/challenges` is implemented in the backend runtime and creates digest-only challenge transactions with rate-limit enforcement and redacted audit/error surfaces; the Admin login UI currently closes the CAPTCHA flow, while slider presentation and risk strategy can continue to improve without changing the credential secret transport contract.

The password change/reset contract belongs to the same credential-auth boundary. `POST /api/admin/profile/current/password/change` changes the current Admin password, and `POST /api/admin/profile/{id}/password/reset` is the privileged reset contract. Current and new secrets use the same ECDH-P256-HKDF-SHA256+A256GCM `secret.encrypted` envelope as login; raw password, verifier, reset token and challenge answer material must not be returned or stored in generic resources.

SMS OTP login belongs to `credential-auth` as a secret type, while SMS delivery itself belongs to the `notification` SMS channel so delivery ledgers, provider adapters, templates, rate limits and production provider validation stay reusable outside authentication. The notification SMS foundation defines the SMS sender port, `mock-local` development/test sender, provider canonicalization and production fail-closed config validation; Aliyun/Tencent use official SDK-backed live adapters when `PLATFORM_NOTIFICATION_SMS_LIVE_SEND_ENABLED=true` and keep dry-run/config validation for rehearsals. Production must reject mock SMS providers.

Credential-auth configuration should surface through provider discovery and the system settings center, not through four hard-coded login menus. The enabled provider modes are rendered from `GET /api/auth/providers`; password policy, challenge policy and SMS OTP policy belong to the `credential-auth` capability contract. SMS account configuration, SMS templates, provider selection, retry and rate-limit policy belong to `notification` resources such as `notification-channels`, `notification-providers`, `notification-send-policies`, `notification-templates` and `notification-deliveries`, normally reached through `/settings` and `/message-center`. Message center v1 includes SMS provider configuration, Aliyun/Tencent live SMS SDK adapters, dry-run validation, the message delivery worker contract, the manual run endpoint `POST /api/admin/message-center/deliveries/run` and the configuration-page closed loop; it still does not claim SMTP/WeChat supplier sending before separate adapters and evidence exist.

The current deliverable v1 API shape is:

```text
GET /api/auth/providers
GET /api/auth/credential-secret-key
POST /api/auth/challenges
POST /api/auth/sms-otp/start
POST /api/auth/login
POST /api/admin/profile/current/password/change
POST /api/admin/profile/{id}/password/reset
POST /api/admin/message-center/deliveries/run
```

Password login request:

```json
{
  "provider": "phone-password",
  "identifier": { "type": "phone", "value": "+8613800000000" },
  "secret": {
    "type": "password",
    "encrypted": {
      "version": "pgo-auth-secret-v1",
      "algorithm": "ECDH-P256-HKDF-SHA256+A256GCM",
      "keyId": "auth-transport-v1",
      "clientPublicKey": "base64url-client-public-key",
      "salt": "base64url-salt",
      "nonce": "base64url-nonce",
      "ciphertext": "base64url-ciphertext"
    }
  },
  "challenge": { "id": "challenge-id", "kind": "slider", "proof": "client-proof" }
}
```

SMS login request:

```json
{
  "provider": "phone-sms-otp",
  "identifier": { "type": "phone", "value": "+8613800000000" },
  "secret": {
    "type": "sms-otp",
    "transactionId": "otp-id",
    "encrypted": {
      "version": "pgo-auth-secret-v1",
      "algorithm": "ECDH-P256-HKDF-SHA256+A256GCM",
      "keyId": "auth-transport-v1",
      "clientPublicKey": "base64url-client-public-key",
      "salt": "base64url-salt",
      "nonce": "base64url-nonce",
      "ciphertext": "base64url-ciphertext"
    }
  },
  "challenge": { "id": "challenge-id", "kind": "captcha", "proof": "abcd" }
}
```

Development configuration keys for the current partial runtime:

```text
PLATFORM_CAPABILITIES=identity,session,rbac,audit,notification,credential-auth
PLATFORM_CREDENTIAL_AUTH_USERNAME_PASSWORD=true
PLATFORM_CREDENTIAL_AUTH_PHONE_PASSWORD=true
PLATFORM_CREDENTIAL_AUTH_EMAIL_PASSWORD=true
PLATFORM_CREDENTIAL_AUTH_PHONE_SMS_OTP=true
PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER=postgres
PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN=...
PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY=...
PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_KEY_ID=auth-transport-v1
PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY=<base64url-32-byte-p256-private-key>
PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_USERNAME=admin
PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PASSWORD=...
PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PHONE=+8613800000000
PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_EMAIL=admin@example.com
PLATFORM_CREDENTIAL_AUTH_ARGON2_PARAMS_VERSION=v1
PLATFORM_CREDENTIAL_AUTH_PASSWORD_MAX_ATTEMPTS=5
PLATFORM_CREDENTIAL_AUTH_LOCK_SECONDS=900
PLATFORM_NOTIFICATION_SMS_PROVIDER=aliyun
PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID=...
PLATFORM_AUTH_SMS_OTP_TTL_SECONDS=300
PLATFORM_AUTH_SMS_OTP_MAX_ATTEMPTS=5
```

The remaining implementation packages are slider/risk-policy hardening, real-vendor SMS delivery evidence, password rotation/breach-response/migration governance, production environment gates and production promotion evidence. The current runtime is a deliverable v1 slice, not a production-complete credential system.

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
- Repositories persist only `sha256:v1` session-token digests. Raw handles exist only at the Store and JWT call boundaries and are never written to file, SQL or GORM storage.
- File v1 snapshots and SQL/GORM tables with the legacy raw `token` column are replaced with empty digest-only storage during initialization, intentionally revoking those historical sessions.
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

Login, app login and Admin refresh treat audit persistence as part of credential issuance. If token signing or audit persistence fails after a server-side session is created or renewed, the new credential state is revoked or discarded and the HTTP response fails without returning a usable credential.

## Audit Events

Successful login and logout write audit records when the `audit-logs` admin resource is enabled:

- `auth.login`
- `auth.refresh`
- `auth.logout`
- `app.auth.login`
- `app.auth.logout`

Audit records use stable investigation fields: actor ID, action, resource, target ID, outcome, event ID, reason code and created time, plus provider only when the schema explicitly allows it. Target labels, filenames, personal values, raw adapter errors, bearer tokens and credential material are omitted. The audit record does not store the raw session handle, its digest or a shortened derivative. Legacy `targetCode` and `traceId` fields remain readable only as internal compatibility data and are omitted from response and export projections.

Generic Admin resource create, update and delete handlers persist the business mutation and its `admin_resource.create`, `admin_resource.update` or `admin_resource.delete` audit in one repository snapshot. If the audit record cannot be validated or saved, the business mutation is rolled back. Authentication and file flows follow the same fail-closed rule instead of returning success after swallowing an audit error. The `audit-logs` resource is read-focused: its runtime schema exposes structured query fields but no create/edit form fields.

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
- API-token create, update and revoke persist the sanitized record and redacted audit atomically. The clear `pgo_` token is returned only after that transaction succeeds; audit failure leaves no token record and no usable credential response.

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

`docs/platform-auth.md` is the current production session-policy specification. The hardening validator requires it before the implemented refresh-token-family slice can be enabled in production.

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

### Production-Like Rehearsal Evidence

The 2026-07-11 Task 8 rehearsal used Keycloak `26.3.3`, a confidential `platform-admin` client, an exact loopback redirect, ignored SQLite Admin/session/lifecycle stores, demo authentication disabled and `admin-oidc` enabled. The subject entered only through `platform-admin bind-admin-oidc --subject-stdin`. `resources/evidence/production-admin-oidc-auth-20260711.json` is the tracked redacted manifest for local source evidence under `tmp/product-design/production-admin-oidc-auth-20260711/`; it integrity-addresses successful and repeated login, exact-state rejection, a transaction expired after the five-minute server window, missing binding, disabled user, retry, logout, refresh, revoked-session rejection and protected navigation without requiring ignored files in a clean checkout.

Browser acceptance covers `375x812`, `390x844`, `768x1024`, `1024x768`, `1280x720` and `1440x1024`, including keyboard focus, live announcements, callback URL cleanup, reduced motion, touch targets, no page overflow and an empty warning/error console. These local production-like results close the reusable foundation node; they do not approve production promotion. Provider-secret ownership, rotation runbooks, rollback evidence and external release approvals remain governed by the `not-approved` production promotion package, and the independent refresh-token-family runtime remains disabled.

## Provider Promotion Matrix

`resources/platform-production-auth-hardening.json` keeps the production Provider Promotion Matrix. It records each built-in provider's capability owner, runtime boundary, production usage, required controls, config keys and source-backed evidence. The matrix is cross-checked against `resources/generated/platform-capability-audit.json`, so every auth provider declared by enabled capability manifests must have a promotion-matrix entry before production promotion can be claimed.

- `demo` stays `local-harness-only`. It can support local demos and controlled verification, but it is not a production external identity provider, must be disabled in production with `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`, and must not gain raw subject or credential exposure.
- `wechat` is an optional production provider. It requires `PLATFORM_WECHAT_MINIAPP_APP_ID`, `PLATFORM_WECHAT_MINIAPP_SECRET` and the optional `PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT`, and production use requires a credential owner, rotation runbook, configured-provider-only login, unconfigured-provider rejection, subject hash/masking and normalized provider errors.
- New production providers must follow the same matrix before runtime promotion: manifest-declared provider, composition-root-injected resolver, config-key contract, secret owner, rotation runbook, hash-and-mask subject storage, unconfigured-provider rejection test, configured-provider-only login test, provider-error normalization test and audit-redaction test.

The matrix is intentionally stricter than provider discovery. Discovery may show an enabled but unconfigured provider; production use needs explicit configuration plus the matrix evidence.

## Sensitive Reveal Step-Up

Sensitive-field reveal is an authenticated authorization flow, not a second login provider. A manifest-declared field selects a dedicated permission and an `anyOf` or `allOf` policy over versioned factors. The initial factors are Admin OIDC reauthentication and Admin SMS OTP.

OIDC reveal uses PKCE and a reveal-specific state flow. Completion must resolve the same provider binding and platform username as the active Admin session. Login state cannot complete a reveal, and reveal state cannot complete login. A failed identity match returns `422 ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED` without revoking the main session.

SMS reveal uses the phone verification provider port plus a configured Admin phone source. Configure `PLATFORM_PHONE_HMAC_KEY`, `PLATFORM_PHONE_CODE_HMAC_KEY`, `PLATFORM_PHONE_VERIFICATION_PROVIDER`, `PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE`, `PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD`, `PLATFORM_ADMIN_STEP_UP_PHONE_FIELD`, `PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD` and `PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD` together. The current encrypted phone's digest must constant-time match the stored verified digest, so changing the phone cannot inherit an old verification timestamp. When reveal policies are enabled, `PLATFORM_SENSITIVE_REVEAL_HMAC_KEY` must be a dedicated value of at least 32 bytes. Production must use a non-debug delivery provider and distinct JWT, phone, code, reveal and rate-limit keys. The stock `cmd/platform-api` process registers only the local debug sender and rejects it outside development/test; a production SMS vendor therefore requires a downstream composition-root adapter and fails closed when none is registered.

Challenges, factor transactions and grants are short-lived. Grant tokens are scoped to the active actor/session/resource/record/field/purpose, stored only as HMAC digests, consumed once and followed by a terminal audit outcome. `reveal.succeeded` means the complete response body was accepted by the server-side HTTP writer; it is not proof that a remote client rendered the value. A writer failure records `reveal.failed` with `response_aborted`. Sensitive HTTP responses are `no-store`; plaintext, tokens and codes must not enter logs, browser storage or generic resource projections.

## Authorization Link

Login does not own authorization. After a provider resolves a platform username, authorization is still calculated by the generic RBAC model:

```text
user -> roles -> permissions / denyPermissions -> menus/resources/actions
```

This keeps menu filtering, backend resource authorization and future button/API permissions independent from the concrete login provider.

Backend resource authorization is now enforced through Casbin policies generated from platform users and roles. `roles.permissions` grants action permissions and `roles.denyPermissions` explicitly denies action permissions; deny matches override wildcard or exact allow matches. The current-session permissions and denied permissions remain useful for frontend controls and visibility, but Casbin is the execution engine for protected HTTP actions.

The current-session `user` also carries `tenantCode`, `orgUnitCode` and `areaCode` when those fields are present on the platform user resource. `tenantCode`, `orgUnitCode` and `areaCode` are consumed by the generic admin resource data-scope filter after Casbin has allowed a read action. Area scopes are explicit role metadata through `current_area`, `current_and_children_areas` or `custom_areas`; none of these fields grants action permissions or changes Casbin checks by itself.

## Current Boundary

### Current Runtime Boundary

The default runtime uses server-side sessions with sliding renewal. The optional Refresh Token Family Model is implemented but disabled by default. Reuse Detection revokes the affected family and session, and the Revocation Scope Matrix distinguishes session, actor and provider consequences. Redis may speed up invalidation and cache lookups, but it is not the source of truth. provider credential rotation is an operational control separate from token renewal. raw refresh tokens must never be persisted.

Audit records must not store the raw session handle, its digest, or any shortened derivative.
OIDC audit records must not store the raw session handle, its digest, or any shortened derivative.

Persisted session identifiers use the canonical `sha256:v1:` prefix followed by exactly 64 lowercase hexadecimal characters. Audit records must not store the raw session handle, its digest or any shortened derivative. The generic audit schema has no `sessionId` field. OIDC audit records must not store the raw session handle, its digest or any shortened derivative. The persisted OIDC audit schema does not expose a `sessionId` field.

The current admin HTTP session credential is a JWT admin bearer token backed by a server-side session with TTL, sliding renewal and revoke support. Memory, file-backed and GORM-backed session stores are available. Repository-backed session stores can be reloaded across API instances through the shared invalidation bus, giving the base platform distributed issue/renew/revoke convergence when instances share the same session repository. API tokens now support scoped Bearer access to protected platform APIs. The app HTTP session credential is a separate JWT app bearer token backed by the same session store contract. App phone verification currently uses a local/demo `debugCode` response, hash-backed generic resources and a local rolling-window abuse guard; production SMS delivery and distributed rate limiting should be added as provider/service adapters without changing the `/api/app/identity/phone-*` contract. Product-specific business API adoption, default-path refresh-token-family enablement, additional provider adapters and normalized identity-binding storage remain later production slices. Production auth should add provider-specific adapters through `httpapi.AppIdentityResolver` without changing provider discovery, login, refresh, logout, current-session and menu APIs.
