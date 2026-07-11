# Platform Production Session Policy Design

## Goal

Define the production session policy enforced by the implemented `production-auth-provider-hardening` foundation gate. The current runtime remains JWT bearer tokens backed by server-side sessions with sliding renewal; this spec defines the boundary for the independent refresh-token-family runtime slice, which is implemented but disabled by default and still requires a separate production promotion package before default runtime enablement.

## Current Runtime Boundary

Admin and app HTTP credentials are JWT bearer tokens. The JWT carries `tokenType`, user, tenant and server-side `sessionId`; the server-side session store is authoritative for TTL, renewal and revocation. `POST /api/auth/refresh` renews the same server-side session and signs a new admin JWT. It is not a refresh-token-family model, and it must not issue long-lived offline credentials.

Raw server-side session handles exist only at the immediate HTTP/JWT and Store boundaries. Persisted session identifiers use the canonical `sha256:v1:` prefix followed by exactly 64 lowercase hexadecimal characters. File, SQL and GORM repositories must reject raw handles, non-canonical digests and mismatched snapshot keys before loading or returning records.

## Refresh Token Family Model

The refresh-token-family runtime is allowed to become part of the default auth path only when offline renewal is explicitly required and approved. It must use separate storage from the session table:

- `familyId`: stable family identifier tied to a server-side session;
- `tokenId`: identifier for one refresh token generation;
- `parentTokenId`: previous generation for rotation lineage;
- `sessionId`, `username`, `tenantId`, `tokenType`;
- `issuedAt`, `expiresAt`, `rotatedAt`, `revokedAt`, `reusedAt`;
- `replacedByTokenId`;
- `tokenHash`: hash of the refresh token value; raw refresh tokens must never be persisted.

The refresh token value is a one-time secret returned only on issuance or rotation. Later reads may expose only family id, token id, status and timestamps.

## Reuse Detection

Refresh token reuse detection is mandatory before production enablement. If a rotated, revoked, expired or unknown refresh token is presented:

- mark the token family as compromised;
- revoke the related server-side session;
- reject the request with a stable auth error code;
- write an audit event without raw token values;
- publish the `sessions` invalidation event so peer API instances reload state.

Successful rotation must happen in one authoritative database transaction: validate current token hash, mark it rotated, insert the new generation and renew the related session. Redis may speed up invalidation and cache lookups, but it is not the source of truth.

## Revocation Scope Matrix

Production enablement must declare and test these revocation scopes:

- logout: revoke the current server-side session and active refresh-token family;
- reuse detection: revoke the full family and related session;
- admin forced logout: revoke selected user sessions and their families;
- JWT signing-secret rotation: invalidate JWTs by secret rotation and optionally revoke affected sessions based on the rotation plan;
- provider credential rotation: does not revoke refresh-token families unless the identity provider compromise assessment requires it;
- API-token rotation: separate from human/app sessions and must not touch session families.

## Audit And Redaction

Required audit actions are `auth.refresh`, `auth.refresh.rotate`, `auth.refresh.reuse_detected`, `auth.logout` and provider-specific login/logout actions. Audit values may include actor, action, resource, provider, family id, token id and timestamps. Audit records must not store the raw session handle, its digest, or any shortened derivative. They must not include JWTs, bearer tokens, refresh token values, provider raw subjects, OpenID, UnionID, phone numbers or provider secrets. The generic audit schema has no `sessionId` field; session correlation belongs in protected runtime telemetry rather than the persisted Admin audit resource.

## Promotion Gate

`refreshTokenFamily.status` must remain `implemented-disabled` and `defaultRuntime` must remain `disabled` until the production approval package proves:

- separate refresh-token-family storage with hashed token values;
- rotation lineage and replay/reuse detection tests;
- revocation scope tests for logout, reuse detection and forced logout;
- Redis/session invalidation convergence tests;
- audit redaction tests for success and replay paths;
- provider rotation separation from session-family revocation;
- production readiness runbook updates and rollback verification commands.
