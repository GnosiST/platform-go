# Sensitive Data Encryption Design

## Goal

Provide reusable, versioned application-layer protection for identity numbers, phone numbers, email addresses, detailed addresses and future recoverable personal data without making historical data unreadable during key rotation or migration.

## Field Policy

Resource field contracts add four explicit dimensions:

- `sensitivity`: public, internal, personal, sensitive or secret;
- `storageMode`: plain, masked, hashed or encrypted;
- `responseMode`: full, masked, privileged or omitted;
- `exportMode`: full, masked, privileged or omitted.

Sensitive and secret fields without an allowed policy fail manifest validation. Sensitive fields cannot use generic record columns such as `name`, `code` or `description`; they must use protected value storage or a dedicated repository.

## Encryption Envelope

Recoverable PII uses AES-256-GCM with a random nonce and authenticated additional data containing tenant ID, resource, record ID, field key and schema version. The persisted envelope identifies format version, algorithm and key version. Production uses a KMS/HSM-backed key-encryption key where available; local development uses an explicit test key provider that is rejected in production.

Encryption happens at the platform persistence boundary. Decryption happens only after authorization and before response projection. No API accepts client-supplied ciphertext as a way to bypass field policy.

## Blind Indexes

Exact-match search and uniqueness use domain-separated HMAC-SHA-256 blind indexes. Phone, email and identity-number normalization rules are versioned. Encryption and blind-index keys are distinct. Partial search over encrypted fields is unsupported unless a separate privacy review approves a specialized index.

## Immutable And Rotatable Configuration

Immutable after data exists:

- field classification and allowed modes;
- normalization algorithm and version;
- envelope format and AAD composition;
- blind-index namespace;
- password algorithm family and minimum policy if local passwords are added.

Rotatable:

- encryption KEK/DEK versions;
- blind-index HMAC key versions;
- password pepper versions;
- JWT signing keys and provider credentials;
- database, Redis, object-storage and TLS credentials.

Runtime keeps one active write key and explicitly configured historical read keys. Missing required historical keys fail startup or migration checks.

## Historical Migration

A dedicated command supports inventory, dry-run, batched migration, verification, resume and rollback. It records a cursor, counts, source and target versions, failures and integrity hashes without recording plaintext.

Migration order:

1. Deploy field policies, unknown-field rejection and response allowlisting.
2. Deploy versioned encrypted storage and dual-read support.
3. Write new data only in the new format.
4. Encrypt historical recoverable plaintext in tenant-scoped batches.
5. Verify counts and integrity, switch the read preference and remove plaintext.
6. Retire old keys only after backup restore rehearsal and the retention window.

Unkeyed legacy phone or subject hashes cannot be upgraded without the original value. They remain readable only for duplicate checks during a bounded transition and upgrade after user re-verification or provider re-binding.

## Password Boundary

The current platform has no local password provider. If one is added, credentials live in a dedicated repository, never generic `Record.Values`. Passwords use Argon2id with random salt, PHC encoding, minimum parameter policy and a versioned server-side pepper. Passwords are never reversibly encrypted. Browser-to-server requests rely on enforced HTTPS; request bodies, logs, traces and errors must not record password values.

## Acceptance

- Database snapshots contain ciphertext envelopes and indexes, not test plaintext.
- Ciphertext, AAD or version tampering fails closed.
- Normalization is stable within a version and different key versions produce different indexes.
- Authorization and response projection prevent unauthorized decryption output.
- Migration is idempotent, resumable and produces no plaintext logs.
- Rollback and backup restore are demonstrated before old key destruction.
- Local-password tests, if the capability is later added, cover salt uniqueness, parameter enforcement, pepper upgrade, rate limits, reset revocation and log redaction.
