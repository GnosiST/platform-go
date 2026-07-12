# Sensitive Data Protection Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task, with test-first RED/GREEN checkpoints and review after each task.

**Goal:** Provide configurable, versioned application-layer encryption and exact-match blind indexes for any declared admin-resource value field, while keeping plaintext out of Store snapshots and decrypting only after an explicit authorization callback succeeds.

**Architecture:** Capability manifests declare protection policy per field and AAD scope per resource; no field name is hard-coded as an encryption trigger. The data-protection runtime owns AES-256-GCM envelopes, normalization, blind indexes and key lookup. The admin-resource Store transforms plaintext after the record ID is stable and keeps only opaque envelopes in memory and repositories. Ordinary response/export projection omits protected values. A separate privileged projection calls an injected authorizer before decrypting. File, SQL and GORM repositories remain policy-agnostic and persist envelope strings unchanged.

**Tech Stack:** Go 1.26 standard-library cryptography, Gin/GORM admin-resource runtime, Node contract/governance validators, Docker production configuration.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-12-sensitive-data-encryption-design.md`.
- Use test-first RED/GREEN cycles for every behavior change.
- Do not implement historical plaintext migration, step-up verification UI, SMS/email reveal verification, KMS/HSM adapters or a reveal HTTP endpoint in this node.
- Do not identify protected fields by names such as `phone`, `email`, `identityNumber` or `address`. Protection is activated only by declared field policy.
- Do not accept client-supplied ciphertext. A declared encrypted field accepts plaintext input and the Store creates the envelope.
- Do not decrypt repository snapshots during load, query, cache refresh or ordinary response/export projection.
- Do not add blind-index companion keys to `Record.Values`. Store index metadata inside the protected field envelope.
- Do not log plaintext, raw keys, nonces, ciphertext, AAD or blind-index digests.
- Prefix shell commands with `rtk` and refresh CodeGraph after structural edits.

## Configurability Contract

Field protection is configured in capability manifests and carried through generated resource contracts:

```go
type AdminFieldProtection struct {
    Format              string
    Normalization       string
    BlindIndexNamespace string
}

type AdminResourceProtection struct {
    SchemaVersion uint32
    Scope         string // global or tenant-field
    TenantField   string // required only for tenant-field
}
```

- `storageMode=encrypted` requires field protection metadata and resource protection context.
- `Format` initially supports only `aes-256-gcm-v1`.
- `Normalization` initially supports versioned, reusable rules: `raw-v1`, `trim-v1`, `email-v1`, `phone-e164-cn-v1` and `identity-cn-v1`. The selected rule is explicit; field names do not select it.
- An empty `BlindIndexNamespace` disables exact-match lookup. A non-empty namespace enables only `=` conditions for that encrypted field.
- `Scope=global` uses a documented stable sentinel. `Scope=tenant-field` requires a declared non-encrypted tenant field on the same record.
- The envelope records format, normalization, namespace, schema version and key IDs. Startup validation rejects incompatible manifest changes once protected data exists.

Deployment configuration selects key material and active versions:

```text
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID=enc-v1
PLATFORM_DATA_ENCRYPTION_KEYRING_JSON={"enc-v1":"<base64-32-byte-key>"}
PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID=idx-v1
PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON={"idx-v1":"<base64-32-byte-key>"}
```

`local-test` is explicit and allowed only in development/test. `env-aes256` is the first production-capable provider. KMS/HSM remain documented future adapters and must not be reported as implemented.

---

### Task 1: Extend The Configurable Field And Resource Contracts

**Files:**
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/capability/admin_contract.go`
- Modify: `internal/platform/capability/admin_contract_test.go`
- Modify: `internal/platform/adminresource/schema.go`
- Modify: `internal/platform/adminresource/security.go`
- Modify: `internal/platform/adminresource/security_test.go`
- Modify: `cmd/platform-contracts/main.go`
- Modify: `scripts/generate-admin-resource-contract.mjs`
- Modify: `scripts/generate-admin-openapi.mjs`
- Modify: `scripts/generate-admin-codegen-preview.mjs`
- Modify: `scripts/validate-admin-resources.mjs`
- Modify: `scripts/validate-admin-resources.test.mjs`
- Modify: `scripts/admin-resource-contract-generators.test.mjs`
- Modify: `admin/src/platform/api/client.ts`
- Regenerate: `resources/generated/*admin*`

**Interfaces:**
- Produces `AdminFieldProtection`, `AdminResourceProtection` and matching generated JSON/TypeScript contracts.
- Preserves defaults for existing public/plain, personal/masked and secret/hashed fields.

- [ ] **Step 1: Write failing contract tests**

Cover:

- arbitrary custom field `governmentReference` can declare encrypted storage without a security-name heuristic;
- encrypted field without format, normalization or resource scope is rejected;
- `tenant-field` scope rejects missing, protected or undeclared tenant fields;
- blind-index namespace must be canonical and unique inside the resource;
- encrypted field cannot use keyword search, range operators or sorting;
- generated Admin/OpenAPI/TypeScript contracts retain every protection property.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/capability ./internal/platform/adminresource -run 'Test.*(Protection|Encrypted|BlindIndex|CustomSensitive)' -count=1
rtk node --test scripts/validate-admin-resources.test.mjs scripts/admin-resource-contract-generators.test.mjs
```

Expected: FAIL because protection metadata and validation do not exist.

- [ ] **Step 3: Implement manifest, runtime-schema and generator support**

Keep protection metadata nested and explicit. Defaults must not silently enable encryption or indexing. Change protected-field validation so encrypted fields may use `privileged` or `omitted` projection, while hashed fields remain permanently omitted. Generic resource input may contain plaintext for a declared writable encrypted field; ciphertext-shaped input is rejected later by the Store protection boundary.

- [ ] **Step 4: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/capability ./internal/platform/adminresource -run 'Test.*(Protection|Encrypted|BlindIndex|CustomSensitive)' -count=1
rtk node scripts/generate-admin-resource-contract.mjs
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node --test scripts/validate-admin-resources.test.mjs scripts/admin-resource-contract-generators.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk git add internal/platform/capability internal/platform/adminresource/schema.go internal/platform/adminresource/security.go internal/platform/adminresource/security_test.go cmd/platform-contracts scripts admin/src/platform/api/client.ts resources/generated
rtk git commit -m "feat: declare configurable field protection"
```

### Task 2: Implement Versioned Encryption, Key Providers And Blind Indexes

**Files:**
- Create: `internal/platform/dataprotection/policy.go`
- Create: `internal/platform/dataprotection/provider.go`
- Create: `internal/platform/dataprotection/env_provider.go`
- Create: `internal/platform/dataprotection/envelope.go`
- Create: `internal/platform/dataprotection/normalization.go`
- Create: `internal/platform/dataprotection/runtime.go`
- Create: `internal/platform/dataprotection/runtime_test.go`
- Create: `internal/platform/dataprotection/env_provider_test.go`

**Interfaces:**

```go
type FieldContext struct {
    TenantID     string
    Resource     string
    RecordID     string
    FieldKey     string
    SchemaVersion uint32
}

type FieldPolicy struct {
    Format              string
    Normalization       string
    BlindIndexNamespace string
}

type Runtime interface {
    Protect(context.Context, string, FieldPolicy, FieldContext) (string, error)
    Validate(context.Context, string, FieldPolicy, FieldContext) error
    Reveal(context.Context, string, FieldPolicy, FieldContext) (string, error)
    MatchExact(context.Context, string, string, FieldPolicy, FieldContext) (bool, error)
}
```

The concrete runtime depends on a `KeyProvider` that returns the active AEAD and blind-index key plus historical keys by ID. The env provider parses canonical key IDs and base64-encoded 32-byte keys, rejects duplicate material across purposes, and exposes no raw configuration in errors.

- [ ] **Step 1: Write failing crypto tests**

Cover:

- two encryptions of the same plaintext produce different envelopes;
- reveal succeeds only with the exact tenant/resource/record/field/schema AAD;
- nonce, ciphertext, algorithm, format version, key ID, normalization, namespace and schema-version tampering fail closed;
- exact blind indexes are stable within a key/version/namespace and differ across key versions or namespaces;
- custom fields use the declared normalizer rather than field-name inference;
- active key rotation writes the new IDs while old envelopes remain readable;
- missing historical key and same key ID with replacement material fail validation;
- encryption and blind-index keys cannot reuse material.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/dataprotection -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the runtime**

Use AES-256-GCM with `crypto/rand` nonces. Encode the envelope as a versioned, opaque string with a stable prefix and base64url JSON payload. Build deterministic AAD from a fixed Go struct, not delimiter concatenation. Store blind-index metadata inside the envelope. Use domain-separated HMAC-SHA-256 over the versioned normalized value. Return typed, value-free errors.

- [ ] **Step 4: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/dataprotection -count=1
rtk go vet ./internal/platform/dataprotection
rtk git add internal/platform/dataprotection
rtk git commit -m "feat: add sensitive data cryptography runtime"
```

### Task 3: Protect Store State And Add Authorized Projection

**Files:**
- Modify: `internal/platform/adminresource/store.go`
- Modify: `internal/platform/adminresource/audit.go`
- Modify: `internal/platform/adminresource/repository.go`
- Modify: `internal/platform/adminresource/file_store.go`
- Modify: `internal/platform/adminresource/security.go`
- Modify: `internal/platform/adminresource/query.go`
- Modify: `internal/platform/adminresource/store_test.go`
- Modify: `internal/platform/adminresource/security_test.go`
- Modify: `internal/platform/adminresource/file_store_test.go`
- Modify: `internal/platform/adminresource/gorm_store_test.go`
- Modify: `internal/platform/adminresource/sql_store_test.go`

**Interfaces:**

```go
type ProtectedFieldAuthorizer interface {
    AuthorizeProtectedField(context.Context, string, string, string, ProjectionPurpose) error
}

func NewStoreFromCapabilitiesWithProtection([]capability.Manifest, dataprotection.Runtime) (*Store, error)
func NewRepositoryBackedStoreFromCapabilitiesWithProtection(AdminResourceRepository, []capability.Manifest, dataprotection.Runtime) (*Store, error)
func (s *Store) ProjectRecordPrivileged(context.Context, string, Record, ProjectionPurpose, ProtectedFieldAuthorizer) (Record, error)
func (s *Store) ValidateProtectedData(context.Context) error
```

Existing constructors remain compatible for manifests with no encrypted fields and fail clearly when encrypted fields are declared without a runtime.

- [ ] **Step 1: Write failing Store and repository tests**

Use a test-only manifest containing arbitrary encrypted fields, including one exact-match field. Cover:

- create/update and audited create/update receive plaintext but Store snapshots contain only envelopes;
- record ID is assigned before protection so AAD is stable;
- updates preserve omitted encrypted values when the caller does not submit them, and re-encrypt only submitted plaintext;
- envelope-shaped client input is rejected;
- ordinary list/query/create/update/export projection never returns envelope or plaintext;
- privileged projection calls the authorizer before `Reveal`; denial proves the runtime was not asked to decrypt;
- File, SQL and GORM persistence contain no marker plaintext;
- file snapshots are written with `0600` permissions;
- reload validates envelope policy and required historical keys without decrypting into Store state;
- changed format, normalization, namespace, tenant scope or schema version fails startup;
- missing runtime for an encrypted manifest fails constructor startup.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/adminresource -run 'Test.*(Encrypted|Protected|Privileged|BlindIndex|Ciphertext|HistoricalKey)' -count=1
```

- [ ] **Step 3: Integrate protection at the Store boundary**

Add a narrow runtime dependency to `Store`. In all four create/update paths, assign or retain the record ID first, derive the declared tenant context, convert submitted plaintext fields to envelopes, then place the record in `s.resources`. Keep stored records encrypted. `scrubSnapshot` validates declared envelopes and removes invalid legacy values; it never reveals them.

Ordinary `ProjectRecord` continues to omit `privileged` and `omitted` fields. `ProjectRecordPrivileged` invokes the authorizer for each privileged field before calling `Reveal`. Hashed fields are never revealed. This node exposes no HTTP route for privileged projection.

- [ ] **Step 4: Implement exact-match query without decryption**

Encrypted fields are excluded from keyword search and sorting. A condition is accepted only when the field has a blind-index namespace and operator `=`. Match the caller value against the envelope index through the data-protection runtime. Keep the query value and digest out of errors and logs.

- [ ] **Step 5: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/adminresource -count=1
rtk go test ./internal/platform/httpapi -run 'Test.*(AdminResource|PolicyReview|Projection)' -count=1
rtk git add internal/platform/adminresource internal/platform/httpapi
rtk git commit -m "feat: protect admin resource sensitive values"
```

### Task 4: Wire Production Configuration And Startup Gates

**Files:**
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Create: `internal/platform/bootstrap/data_protection.go`
- Create: `internal/platform/bootstrap/data_protection_test.go`
- Modify: `internal/platform/bootstrap/admin_resources.go`
- Modify: `internal/platform/bootstrap/admin_resources_test.go`
- Modify: `cmd/platform-api/main.go`
- Modify: `cmd/platform-api/main_test.go`
- Modify: `deploy/env/production.example.env`
- Modify: `deploy/compose/docker-compose.prod.yml`
- Modify: `scripts/validate-platform-production-env.mjs`
- Modify: `scripts/platform-production-env.test.mjs`
- Modify: `scripts/validate-platform-deployment-topology.mjs`
- Modify: `scripts/platform-deployment-topology.test.mjs`
- Modify: `resources/platform-production-readiness.json`
- Modify: `resources/platform-deployment-topology.json`

**Interfaces:**
- Produces `bootstrap.DataProtectionRuntimeFromConfig`.
- Injects the runtime before `AdminResourcesFromConfig` loads persistent records.

- [ ] **Step 1: Write failing config/bootstrap tests**

Cover:

- encrypted manifests require explicit provider and complete keyrings;
- `local-test` is rejected in staging/production;
- production rejects missing/unsupported provider, missing active IDs, non-canonical IDs, invalid JSON/base64, wrong key length and reused key material;
- active IDs must exist in their keyrings;
- provider errors never echo keyring JSON;
- manifests without encrypted fields remain compatible with an unconfigured development/test runtime;
- bootstrap loads persistent envelopes and fails when a historical key is missing or replaced.

- [ ] **Step 2: Verify RED**

```bash
rtk go test ./internal/platform/config ./internal/platform/bootstrap ./cmd/platform-api -run 'Test.*(DataProtection|EncryptionKeyring|HistoricalKey)' -count=1
rtk node --test scripts/platform-production-env.test.mjs scripts/platform-deployment-topology.test.mjs
```

- [ ] **Step 3: Implement fail-closed composition**

Track whether production key settings came explicitly from environment, following existing file/transport policy-source conventions. Build the provider before loading admin resources. After repository load, call `ValidateProtectedData` so missing, replaced or incompatible historical keys stop startup before the HTTP server is created.

- [ ] **Step 4: Verify GREEN and commit**

```bash
rtk go test ./internal/platform/config ./internal/platform/bootstrap ./cmd/platform-api -count=1
rtk node --test scripts/platform-production-env.test.mjs scripts/platform-deployment-topology.test.mjs
rtk node scripts/validate-platform-production-env.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk git add internal/platform/config internal/platform/bootstrap cmd/platform-api deploy resources/platform-production-readiness.json resources/platform-deployment-topology.json scripts
rtk git commit -m "feat: wire sensitive data production keys"
```

### Task 5: Close Governance, Documentation And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `docs/platform-deployment.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-data-governance-and-integrations-assessment.md`
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: matching governance validators and mutation tests
- Create/Update: `.superpowers/sdd/sensitive-data-progress.md`

- [ ] **Step 1: Document the actual boundary**

Document manifest examples for arbitrary custom encrypted fields, supported normalizers, exact-match-only indexing, privileged projection, TLS transport expectations, env key injection, rotation procedure and startup failure modes. Explicitly state that KMS/HSM, historical migration and reveal verification flows are not implemented by this node.

- [ ] **Step 2: Run neat-freak closeout review**

Reconcile README, schema/deployment/capability docs, generated contracts, environment examples and task records against code. Remove obsolete statements that encrypted fields must always be omitted, but retain the rule that hashed fields are never recoverable.

- [ ] **Step 3: Mark the node implemented only after evidence exists**

Move `sensitive-data-protection-runtime` from pending to implemented with source, tests and checks. Update the execution, goal, closeout, objective, alignment and engineering-capability artifacts consistently. Keep `sensitive-data-historical-migration` pending.

- [ ] **Step 4: Run full verification**

```bash
rtk go test ./...
rtk go vet ./...
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

- [ ] **Step 5: Independent review and final commit**

Request a read-only code review focused on plaintext leakage, authorization-before-decryption, key rotation, configurable-field behavior, manifest drift and governance consistency. Fix accepted findings, rerun affected checks, then commit the closeout.

## Success Criteria

- Any capability can declare a custom `values` field as encrypted without changing platform code or relying on field-name matching.
- Store state and File/SQL/GORM persistence contain envelopes and blind indexes, never test plaintext.
- Ordinary response/export paths omit protected data and never decrypt it.
- Privileged projection cannot decrypt until its authorizer succeeds.
- Exact match works only for explicitly indexed encrypted fields and does not decrypt records.
- Key rotation preserves historical reads and new writes use only the active versions.
- Missing or incompatible historical keys and immutable policy drift fail startup.
- Production key configuration is explicit, validated and redacted from errors.
- Historical migration, KMS/HSM and reveal verification remain honestly pending.
