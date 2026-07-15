# Platform Service Objects

`internal/platform/serviceobject` provides the executable Query Object and Command Object foundation for high-risk Admin operations. Clients select a server-registered definition by stable ID and version, submit typed logical arguments, and receive a projected logical result. They never submit SQL, physical columns, joins, datasource identifiers, schemas or shards.

The runtime is business-neutral and injected through `httpapi.ServerOptions.ServiceObjects`. The default `cmd/platform-api` composition does not register the reference definitions, so the Admin routes remain conditionally available until an application composition root supplies a registry, executors and idempotency store.

## Admin Transport

Authenticated management users use two POST endpoints:

```text
POST /api/admin/service-objects/query
POST /api/admin/service-objects/command
```

Query request:

```json
{
  "queryId": "platform.reference-records.list",
  "version": "1.0.0",
  "arguments": {
    "status": "active",
    "codePrefix": "OPS-"
  },
  "pagination": {
    "page": 1,
    "pageSize": 20
  },
  "sort": [
    { "name": "code", "order": "asc" }
  ]
}
```

Command request:

```json
{
  "commandId": "platform.reference-records.rename",
  "version": "1.0.0",
  "arguments": {
    "code": "OPS-001",
    "name": "Operations"
  },
  "idempotencyKey": "rename-ops-001-20260715"
}
```

Request bodies are closed JSON objects with a 1 MiB decode limit. Unknown fields and trailing JSON values are rejected. The generated OpenAPI union and TypeScript client expose only registered logical arguments and sort names.

## Trusted Execution Context

The client does not send tenant, organization, area, actor or permission scope. The Admin handler derives them from the authenticated principal and current authorization state:

```text
Admin session
  -> management-user principal
  -> trusted tenant context
  -> Casbin permission check
  -> organization / area / self data scope
  -> service-object runtime
```

An unknown definition and a definition denied by authorization both return `SERVICE_OBJECT_UNAVAILABLE`. This preserves not-found-equivalent behavior and avoids definition enumeration.

## Definition Contract

Each `QueryDefinition` or `CommandDefinition` declares:

- stable lowercase ID and numeric semantic version;
- logical resource, permission and action;
- tenant mode and matching data scope;
- typed arguments and projected result fields;
- timeout and cost policy;
- allowed logical sort names for queries;
- idempotency and maximum affected rows for commands;
- a server-owned builder that produces a logical AST.

The registry rejects duplicate ID/version pairs, invalid types, inconsistent tenant modes, unsafe names and client-visible names beginning with physical-routing prefixes. Supported argument and result types are currently `string`, `integer` and `boolean`.

## Execution Pipeline

```text
queryId / commandId + version
  -> registry lookup
  -> permission and trusted scope check
  -> typed argument validation
  -> server-owned logical AST builder
  -> cost, pagination, sort and timeout enforcement
  -> GORM executor with registered resource binding
  -> projected logical response
```

`GORMResourceBinding` is the only place that maps logical fields to physical tables and columns. Predicates, updates, tenant constraints and data-scope constraints use parameterized GORM clauses. A definition cannot introduce an unregistered predicate, value, sort or result field at request time.

Query cost includes base, row, offset, predicate, sort and optional total-count costs. Enabled dimensions must have positive weights, and the registry applies a platform hard offset ceiling of 10,000 before each definition's lower budget. The runtime rejects excessive offsets or costs before database execution. Definition timeouts are limited to one minute. Commands separately enforce `MaxAffectedRows`.

## Idempotency

Commands may declare `required-key`. The runtime fingerprints the normalized command definition and validated arguments, then scopes the key by command, version, actor and tenant.

`GORMIdempotencyStore` provides the production storage implementation:

- a deterministic scope digest is the unique database key, while the original scope is stored and verified;
- `processing` and `completed` states carry fingerprint, owner lease, result and expiry timestamps;
- atomic insert and conditional lease takeover coordinate concurrent processes;
- completed results are replayed without executing the command again;
- reuse with a different fingerprint returns `SERVICE_OBJECT_IDEMPOTENCY_CONFLICT` until expiry;
- command callbacks run outside database transactions and process-wide locks;
- database errors are returned as sanitized service-object errors.

`MemoryIdempotencyStore` is bounded by TTL and capacity and is intended for tests and local harnesses. It coordinates each scope independently and does not serialize unrelated commands.

## Domain Command Extension

The organization/RBAC target runtime extends the generic AST with registered domain handlers for relationship and authorization changes that must be atomic. Role movement or disablement uses `platform.identity.role-state-or-group-change.prepare`, its persisted impact/conflict queries, then `platform.identity.role.move` or `platform.identity.role.disable`. Role authorization uses `platform.authorization.role-permission-change.prepare`, the persisted impact query, then `platform.authorization.role-permissions.replace` to change allow permissions, deny permissions and data scope as one revision.

Prepare stores the normalized change set server-side and returns `previewId`, `expectedRevision` and `impactHash`. Apply accepts those values plus an idempotency key, reloads the reviewed set and rejects stale or changed state. In target mode, generic Admin mutation cannot bypass these handlers for role group ownership, role state or role policy fields.

## Reference Definition

`internal/platform/serviceobject/reference.go` supplies one query and one command definition plus a GORM binding. The HTTP integration tests prove authenticated Admin execution, permission denial, tenant/data-scope derivation, strict request decoding, rate limiting and sanitized responses. The generated artifacts are:

- `resources/generated/openapi.admin.json`;
- `resources/generated/admin-codegen-preview.json`;
- `resources/generated/admin-service-object-client.ts`.

## Error Contract

| HTTP | Code | Meaning |
| --- | --- | --- |
| 400 | `SERVICE_OBJECT_REQUEST_INVALID` | Closed-schema, type, pagination, sort or AST validation failed. |
| 404 | `SERVICE_OBJECT_UNAVAILABLE` | Runtime, definition or authorization is unavailable. |
| 409 | `SERVICE_OBJECT_IDEMPOTENCY_CONFLICT` | The scoped idempotency key was reused for another command fingerprint. |
| 422 | `SERVICE_OBJECT_COST_LIMIT` | Query or command cost exceeded the registered budget. |
| 500 | `SERVICE_OBJECT_EXECUTION_FAILED` | Execution or result projection failed without exposing physical details. |

## Boundaries

This node does not implement workload identity protocols, tenant-to-datasource placement, request-level datasource selection, read/write routing, sharding, cross-database federation, XA, Outbox/MQ delivery or search projection. Those remain separate controlled nodes.

The existing generic Admin resource query remains available for low-risk management lists. High-risk, cross-service, report and future federated queries should use registered service objects so authorization, typed parameters, cost and result projection remain server-owned.

## Verification

```bash
rtk go test ./internal/platform/serviceobject ./internal/platform/httpapi
rtk node --test scripts/platform-service-object-runtime.test.mjs
rtk node scripts/validate-platform-service-object-runtime.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk npm --prefix admin run build
```
