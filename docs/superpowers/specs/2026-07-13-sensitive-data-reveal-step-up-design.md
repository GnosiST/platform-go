# Sensitive Data Reveal Step-Up Design

## Goal

Allow an authorized Admin operator to reveal one manifest-declared encrypted field only after a server-owned step-up policy is satisfied. Ordinary list, query, detail, Tooltip and export projections remain masked or omitted; plaintext is never added to those shared projections.

## Manifest Contract

- A field opts in explicitly through `AdminFieldReveal`; field names never imply reveal behavior.
- The field declaration selects a registered policy, a dedicated permission and whether copy is allowed.
- `AdminRevealPolicy` declares `anyOf` or `allOf`, versioned factors, localized purpose codes, challenge TTL and grant TTL.
- Initial public factors are `oidc-reauth-v1` and `admin-sms-otp-v1`. Internal factor names are mapped to this stable Admin contract.
- Only encrypted, recoverable, non-full projection fields may declare reveal. Hashed or legacy pre-masked values remain unrecoverable.

## Runtime And Persistence

- Challenges, factor transactions, grants and audit events are server-owned records. Tokens are random, returned once and stored only as purpose-separated HMAC digests.
- Grants are scoped to actor, active Admin session, resource, record, field and purpose. They expire quickly and are consumed atomically once.
- The GORM runtime persists state when the Admin resource driver is GORM; development fallback may use memory. Production reveal policies require a dedicated `PLATFORM_SENSITIVE_REVEAL_HMAC_KEY` of at least 32 bytes.
- Factor failure, cancellation, grant issue, grant consumption and terminal reveal outcome are append-only audit events. `reveal.succeeded` means the full payload was accepted by the server-side HTTP writer; writer failure records `response_aborted`. Audit payloads reject plaintext and arbitrary error text.
- Transactional store operations roll back factor/grant state when the matching audit write fails.

## Verification Factors

### OIDC Reauthentication

- Reauthentication uses the configured Admin OIDC provider, PKCE and a reveal-specific state flow separate from login.
- Callback completion must resolve to the same bound Admin username as the active session.
- Login and reveal callback state are mutually rejected so one flow cannot consume the other flow's state.

### Admin SMS OTP

- SMS delivery uses the existing phone verification provider port; production must not use the debug provider.
- The stock API process intentionally registers only the local debug sender. A production SMS vendor is a downstream composition-root adapter; without one, SMS startup and delivery fail closed.
- The Admin phone source is configured as one resource plus actor, encrypted phone, verified-at and verified-phone-digest fields.
- The current phone digest must constant-time match the digest captured when the phone was verified. Changing the phone while retaining an old verification timestamp cannot authorize delivery.
- JWT, phone, verification-code, reveal and rate-limit keys are distinct configuration secrets.

## HTTP And Error Contract

- Generic routes expose policy, challenge creation, OIDC/SMS start and completion, then one grant-consuming reveal operation.
- Backend permission checks are authoritative; the UI only shows the action when the field contract and permission allow it.
- Sensitive responses use `Cache-Control: no-store` and never log tokens, codes or plaintext values.
- Verification failure returns `422 ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED`; it does not revoke or clear the administrator's main session.
- Expired state returns `410`, conflicts return `409`, rate limits return `429`, unavailable dependencies return `503`, and provider delivery/exchange failures use `502` where applicable.

## Admin Experience

- The reveal action exists only beside an eligible field in the record detail surface.
- The modal progressively presents purpose, available factor selection or required-factor progress, verification, and the short-lived plaintext result.
- The heading receives focus on open and after terminal state changes. Controls keep 44px targets, bilingual labels, keyboard operation and mobile single-column actions.
- Plaintext exists only in the modal, is cleared on close, page visibility loss or expiry, and is rendered once. Copy requires both field and runtime policy approval.
- OIDC return hydrates the target record and resumes the same modal without exposing callback parameters to the login consumer.

## Boundaries

- This does not make plaintext available in generic list/detail/export APIs.
- This does not make SMS mandatory. A policy can use OIDC only, SMS only, or an explicit combination.
- CAPTCHA, email OTP and other future factors require new versioned factor adapters and contract evidence; they are not silently accepted by the current runtime.
- Screenshots and browser checks support the interaction acceptance but do not certify full WCAG conformance.

## Acceptance

- Arbitrary encrypted fields can declare reveal without name-based logic.
- Any-of and all-of policies enforce the declared factor order/choice and purpose.
- OIDC and SMS verification are bound to the active Admin identity and reveal scope.
- Grants are short-lived, single-use and transactionally audited.
- Failed verification preserves the main Admin session and returns the documented `422` response.
- Plaintext is confined to the modal, clears on close/hide/expiry, and respects copy policy.
- Desktop and mobile browser evidence covers factor choice, SMS verification, revealed value, focus, no horizontal overflow and clean current-run console output.
