# Platform Service Contract Standard

`platform-go` uses `capability.Manifest.Service` as the executable source for stable service declarations outside the existing Admin resource and App route surfaces.

The standard separates five planes:

- **Admin**: operator-facing management resources and commands. Existing Admin OpenAPI remains the runtime document of record.
- **Service/Data**: trusted service-to-service commands and queries.
- **Control**: privileged platform configuration and operational control.
- **External/Partner**: explicitly published client or partner HTTP contracts.
- **Event**: versioned publish/subscribe schemas. This node defines the envelope and AsyncAPI only; delivery remains unimplemented.

## Manifest Contract

Each declared service includes:

- service ID, owner, audiences, stability and semantic version;
- management-user or workload identity mode;
- allowed auth modes;
- trusted tenant-context fields and provenance;
- commands, queries and events with permissions and data scopes;
- request, response and event payload schemas with PII classification;
- idempotency, optimistic-concurrency, timeout, retry, rate and cost declarations;
- SLA, compatibility, deprecation and explicit runtime boundaries.

Capabilities without a Service surface remain valid. This preserves compatibility for existing Admin-only and App-only manifests.

`file-storage` is the reference capability. Its existing App upload and content routes are the only `bound` operations: upload declares the real `201` response with `{data:{resource,record}}`, while content read declares `200 application/octet-stream`. Admin, Service/Data and Control operations are `contract-only`. The `file-stored` event is also `contract-only`; the repository does not claim an Outbox, broker or reliable delivery runtime.

## Trusted Tenant Context

The standard context contains:

```text
tenantId
tenantCode
organizationId
configurationVersion
```

Accepted provenance is limited to authenticated identity, a trusted gateway or an authorized Control Plane override. Ordinary clients cannot select a DSN, physical datasource, database, schema or shard. Request payload schemas are closed, explicitly reject those fields and generated SDKs do not expose them.

This contract does not resolve the repository's older tenant representations. Later organization and datasource nodes must adapt their runtime context to this stable string-based boundary without moving physical routing into client input.

## Identity Boundary

Management-user identity and workload identity are separate:

- Admin session, App session and scoped API token are management-user modes already present in the platform runtime.
- OAuth2 client credentials, mTLS and workload JWT are approved contract modes but remain declaration-only.
- Anonymous authentication is not part of this two-identity standard; a future public anonymous plane requires an explicit identity-model extension rather than overloading management-user identity.

The contract standard does not implement an OAuth2 authorization server, certificate trust plane or workload-token issuer.

## Trace And Event Envelope

Generated contracts use W3C `traceparent` and `tracestate` fields. Runtime HTTP extraction, OpenTelemetry export and cross-message propagation are not implemented by this node.

Event envelope version `1.0` requires:

```text
eventId, eventType, eventVersion, occurredAt, producer,
tenantContext, traceContext, payload
```

`tenantContext` is required only for events whose `tenantMode` is `required`; it remains optional for `optional`, `none` and platform-level events. AsyncAPI messages carry the complete versioned envelope and reference the business payload schema inside `payload`. Channels keep `runtimeStatus=contract-only` until the Outbox/MQ node supplies transactional publication, retry, idempotent consumption, replay and operational recovery.

## Generation

Generate the canonical service contract and derived artifacts:

```bash
rtk go run ./cmd/platform-contracts service-manifests --output resources/generated/platform-service-contract.json
rtk node scripts/generate-platform-service-contract-artifacts.mjs
```

The generator writes:

- `resources/generated/openapi.service.json`
- `resources/generated/openapi.control.json`
- `resources/generated/openapi.external.json`
- `resources/generated/asyncapi.events.json`
- `resources/generated/service-sdk/go/service_contract_sdk.go`
- `resources/generated/service-sdk/typescript/serviceContractSDK.ts`

SDK output stays under `resources/generated`. It is compiled in an isolated temporary module by tests and does not enable runtime source writing.

## Compatibility And Consumers

The compatibility gate rejects stable-service version downgrade and same-major removal of services, operations or events. It also locks service classification and identity modes; operation identity, HTTP binding, success status and request/response schemas; event version, direction, envelope and payload schema; plus auth, tenant, permission, data-scope and required-field contracts.

Deprecated contracts must declare an RFC3339 sunset and replacement. Positive and negative consumer fixtures prove reference resolution and physical-routing rejection.

Run the focused checks:

```bash
rtk go test ./internal/platform/capability ./internal/platform/core ./cmd/platform-contracts
rtk node --test scripts/platform-service-contract-standard.test.mjs
rtk node scripts/validate-platform-service-contract-standard.mjs
```

## Deferred Runtime Work

This node does not implement:

- QueryDefinition or CommandDefinition execution;
- workload identity protocols;
- idempotency or rate-limit persistence;
- transactional Outbox, MQ, DLQ or replay;
- OpenTelemetry propagation;
- tenant placement, read/write routing, sharding, federation or XA;
- public runtime publication of Service, Control or External OpenAPI documents.

Those remain owned by the activated follow-up nodes.
