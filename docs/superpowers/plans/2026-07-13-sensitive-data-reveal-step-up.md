# Sensitive Data Reveal Step-Up Implementation Plan

## Goal

Close `sensitive-data-reveal-step-up` with manifest-driven policy, persistent short-lived grants, OIDC and SMS factors, accessible Admin interaction, production configuration and synchronized governance evidence.

## Completed Steps

- [x] Extend capability and generated Admin contracts with reveal policies, field permissions, factor mode, purposes, TTLs and copy policy.
- [x] Add fail-closed validation for encrypted revealable fields and stable factor identifiers.
- [x] Implement the memory/GORM reveal store, purpose-separated token digests, atomic challenge/factor/grant transitions and append-only audit events.
- [x] Add generic policy, challenge, factor and reveal HTTP routes with permission, session, purpose, rate-limit, no-store and error mapping.
- [x] Add reveal-specific OIDC state/PKCE flow and reject login/reveal state crossover.
- [x] Add Admin SMS OTP delivery through the phone verification port and bind delivery to a verified phone digest.
- [x] Add the schema-driven reveal action and responsive modal, including focus management, expiry clearing, visibility clearing and copy policy.
- [x] Protect the main session from verification-failure `401` drift and document the canonical `422` response.
- [x] Generate Admin OpenAPI and add mutation tests for runtime, config, HTTP, OIDC, UI and OpenAPI contracts.
- [x] Capture Product Design and `ui-ux-pro-max` browser evidence for factor choice, SMS verification, desktop reveal and `390x844` reveal.
- [x] Synchronize deployment, auth, schema, UI, roadmap and governance resources.

## Verification

Focused verification:

```bash
rtk go test ./internal/platform/authprovider/oidc ./internal/platform/httpapi ./internal/platform/config ./internal/platform/bootstrap
rtk node --test scripts/admin-resource-contract-generators.test.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run typecheck
```

Phase closeout must also run the repository's full Go, governance, Admin build, diff and CodeGraph checks before commit.

## Production Configuration

- Set `PLATFORM_SENSITIVE_REVEAL_HMAC_KEY` to a dedicated value of at least 32 bytes whenever production manifests declare reveal policies.
- Configure all five `PLATFORM_ADMIN_STEP_UP_PHONE_*` values together when SMS is offered.
- Keep the JWT secret, `PLATFORM_PHONE_HMAC_KEY`, `PLATFORM_PHONE_CODE_HMAC_KEY`, `PLATFORM_SENSITIVE_REVEAL_HMAC_KEY` and `PLATFORM_RATE_LIMIT_HMAC_KEY` distinct.
- Use a registered non-debug phone verification adapter in production; the stock API process keeps SMS vendor integration disabled and fails closed.
- Preserve historical phone verification digests when migrating the configured Admin phone source.

## Closeout Evidence

- Design gates: `superpowers:brainstorming`, Product Design, `ui-ux-pro-max`.
- Browser evidence: `.superpowers/product-design-audit/sensitive-reveal/01-factor-selection.png` through `04-mobile-revealed-value.png`.
- Contract evidence: generated Admin OpenAPI, Go tests, Admin UI validator/tests, typecheck and build.
- Cleanup: one phase-level `neat-freak` invocation after documentation and governance synchronization.
