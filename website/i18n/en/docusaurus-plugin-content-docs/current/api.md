---
sidebar_position: 4
title: API and contracts
---

# API and contracts

Platform APIs are Admin, App, and public capability contracts. Resource queries always use structured JSON conditions. The backend accepts only schema-whitelisted fields and parameterized predicates; UI search syntax is never transported or executed as SQL.

## Service entry points

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | Process liveness |
| GET | `/api/ready` | Constrained runtime-dependency readiness |
| GET | `/api/capabilities` | Capability discovery |
| GET | `/api/openapi.json` | Generated OpenAPI contract |
| POST | `/api/admin/resources/:resource/query` | Structured resource query |
| GET | `/api/admin/resources/:resource/schema` | Admin resource schema |
| GET | `/api/admin/session/current` | Current Admin session |

## Authentication contracts

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/auth/providers` | Discover enabled and configured login providers |
| GET | `/api/auth/credential-secret-key` | Public-key information for credential secret transport |
| POST | `/api/auth/challenges` | Create the mandatory login challenge |
| POST | `/api/auth/challenges/verify` | Slider preverification without consuming the challenge |
| POST | `/api/auth/sms-otp/start` | Request OTP send after a valid challenge |
| POST | `/api/auth/login` | Authenticate into the existing session/JWT boundary |
| POST | `/api/app/auth/login` | App login contract |

Request and response schemas are defined by `/api/openapi.json` and generated artifacts. `credential-auth` secrets, OTPs, challenge proofs, and client fingerprints are write-only and must not appear in browser logs, error responses, or generic resource fields.

## Errors, tracing, and evolution

Errors should include a stable code, `request_id`, and a safe display message. Use request IDs to correlate server logs, but do not log JWTs, keys, full identifiers, passwords, OTPs, or plaintext sensitive fields.

Public-contract changes update OpenAPI/AsyncAPI, consumer contract tests, idempotency/timeout/retry/rate-limit semantics, and deprecation notes together. Validate breaking changes against downstream consumers before release.
