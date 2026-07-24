---
sidebar_position: 3
title: Authentication and authorization
---

# Authentication and authorization

The platform models identity, sessions, authorization, and local credential authentication separately. JWT carries identity, server-side session controls TTL and revocation, and Casbin makes final authorization decisions on the backend. Menus only signal visibility.

## Provider discovery

The login page reads `GET /api/auth/providers` first and renders only providers that are both `enabled` and `configured`. Do not hard-code username/password, phone/password, email/password, and SMS OTP as fixed menus. Do not render debug codes in product UI.

| Route | Purpose |
| --- | --- |
| `GET /api/auth/providers` | Discover available login providers |
| `GET /api/auth/credential-secret-key` | Fetch ECDH public-key information for local credentials |
| `POST /api/auth/challenges` | Create the mandatory CAPTCHA/slider challenge |
| `POST /api/auth/challenges/verify` | Get server-authoritative slider preverification only |
| `POST /api/auth/sms-otp/start` | Request OTP delivery after a valid challenge |
| `POST /api/auth/login` | Complete credential/OIDC and other login flows |
| `GET /api/admin/session/current` | Read the current Admin session |

## Credential auth v1

`credential-auth` supports `username-password`, `phone-password`, `email-password`, and `phone-sms-otp` provider modes. Its dedicated GORM stores contain normalized identifier hashes, Argon2id verifiers, challenge/OTP digests, expiry, and consumption state. Passwords, raw OTPs, challenge answers, and raw phone/email values never enter generic `Record.Values`.

The browser fetches a public key, derives a key with ECDH P-256 + HKDF-SHA256, and submits `secret.encrypted` with AES-256-GCM. HTTPS remains the production baseline but is not a substitute for application-layer secret transport. When `RequireEncryptedSecrets` is enabled, plaintext secret fields are rejected.

## Challenge and SMS OTP

Each credential-password login, OTP send, and OTP login requires one challenge. A successful slider preverification does not consume the challenge; final login atomically consumes it. Incorrect proofs count attempts, and expired, consumed, or exhausted transactions cannot be reactivated.

SMS delivery belongs to the `notification` SMS channel. An OTP transaction persists before send and is disabled after a send failure; rate limits use provider/IP and normalized phone hashes rather than plaintext targets. Aliyun/Tencent delivery still needs a controlled environment, account ownership, rollback, and redacted evidence. `mock-local` is development/test-only; SMTP and WeChat adapters are not implemented.

## Production requirements

Credential auth in production requires the `credential-auth` capability, a dedicated repository driver/DSN, a 32-byte-or-longer identifier HMAC key, and a valid P-256 secret-transport private key/key ID. Phone OTP also needs `notification`, a non-debug SMS provider, and a login template ID.

Set `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true` and use distinct JWT, data-encryption, and blind-index keyrings. Rotate by adding a new key ID, deploying old and new material, switching the active ID, verifying reads and backups, then retiring old material under review. Never replace material under an existing key ID.

For parameters, session/revocation, step-up, password changes, and rollback rules, see the source-of-truth [`docs/platform-auth.md`](https://github.com/GnosiST/platform-go/blob/main/docs/platform-auth.md).
