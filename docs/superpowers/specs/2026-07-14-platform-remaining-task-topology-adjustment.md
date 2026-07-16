# Platform Remaining Task Topology Adjustment

> **Status:** Activated governance topology with the 2026-07-16 release overlay. The current task graph projection is `67 total / 54 implemented / 13 controlled unfinished` and remains `not-complete-controlled`.

## Authority

This document is the activated and only authoritative topology for the remaining platform program.

## 2026-07-16 Release Overlay

Task 6 closes `menu-tree-and-button-permission-configuration`, `unified-error-code-governance` and the organization E2E gate, and separates current unfinished work into two machine-checkable lanes.

- v0.1.0 release blockers: `open-source-portability`, `public-docs-community`, `public-docs-site`, `github-release-publication`.
- Post-release optional deferred nodes: `multi-datasource-contract-and-runtime`, `tenant-placement-and-request-routing`, `datasource-read-write-routing`, `sharding-and-tenant-migration`, `federated-read-query`, `xa-optional-adapter`, `database-certification-matrix`, `transactional-outbox-and-one-mq-adapter`, `asynchronous-search-projection`.
- `open-source-portability` now depends on `admin-watermark-export-governance`, the implemented organization E2E and unified error-code closeouts; no release blocker may depend directly or transitively on a deferred node.
- Deferred nodes remain full-goal blockers. Release eligibility can become ready when the release-blocker lane is empty, but the persistent foundation objective cannot become complete while deferred nodes remain unfinished.
- `unified-error-code-governance` owns the canonical registry, ownership/audience/HTTP/retry/redaction/compatibility/deprecation metadata, request/trace correlation, generated public contracts and duplicate/reassignment/deprecation validation; its closeout is implemented and remains a required shared service-contract boundary.

- The consolidated SaaS data-plane, persisted Query Object and Platform Service Contract input confirmed on 2026-07-14 is authoritative.
- Earlier cross-session variants describing only static datasources, separate Query Object drafts, SQL-less drafts or partial service-contract drafts are superseded and must not be accumulated as extra requirements.
- The organization/RBAC/menu decision remains active: one role belongs to one role group; an organization may bind multiple role groups; role groups classify and isolate roles but do not grant permissions.
- `data-lifecycle-retention` remains closed at commit `0de9f2d7`. This activated topology may reuse its contracts but must not reopen or expand that node.

## Boundary Rationale

The program keeps these areas separate because their contracts, runtime risks and evidence gates are materially different:

- `multi-datasource-contract-and-runtime` owns named sources and capability bindings; TenantPlacement, trusted request routing, read/write routing, sharding, federation and XA retain separate post-release gates.
- `integration-ports-disabled-default` owns only disabled business-neutral ports; it does not imply an implemented Outbox, MQ adapter or search projection.
- the implemented Service Contract and persisted Query/Command runtime are shared foundations, not permission to activate the deferred data-plane lane.
- organization, role-group, role, menu and permission migration keeps cutover and rollback separate from the already-implemented authoring surfaces.
- domain documentation locks remain narrow until the serial open-source publication lane begins.

## Post-Release Stable Data Plane Target

The target request path is:

```text
Request / Service Call
-> Identity + Trusted TenantContext
-> Capability Service Contract
-> Query Object / Command Object
-> TenantPlacementResolver
-> ShardRouter
-> ReadWriteRouter
-> GORM dbresolver or equivalent adapter
-> Physical Database
```

Runtime routing is configuration-driven and deterministic. Datasource, DatasourceGroup, TenantPlacement, shard, read/write and consistency policies are declared through configuration or an authorized control plane. Ordinary clients cannot submit a DSN, physical datasource, database, schema or shard.

## Current Remaining Node Order

The only current projection is `67 total / 54 implemented / 13 controlled unfinished`. Release blockers and deferred nodes remain disjoint while preserving task-graph order.

v0.1.0 release path:

1. `open-source-portability`
2. `public-docs-community`
3. `public-docs-site`
4. `github-release-publication`

Post-release optional deferred path:

1. `multi-datasource-contract-and-runtime`
2. `tenant-placement-and-request-routing`
3. `datasource-read-write-routing`
4. `sharding-and-tenant-migration`
5. `federated-read-query`
6. `xa-optional-adapter`
7. `database-certification-matrix`
8. `transactional-outbox-and-one-mq-adapter`
9. `asynchronous-search-projection`

The deferred IDs and their dependency contracts remain stable but dormant. They do not enter a current implementation batch until a post-release objective explicitly reactivates them.

## Program Node Boundaries And Completion Gates

The group labels below preserve stable program ownership only. Current status, release order and activation state are defined exclusively by the current remaining-node projection above and the task graph.

### 1. Platform Service Contract Standard

Owns the executable common contract used by every later node:

- Admin, Service/Data, Control, External/Partner and Event plane separation;
- Capability Service Manifest identity, owner, audience, stability, version, auth mode, tenant mode, permissions, data scopes, commands, persisted queries and events;
- idempotency, optimistic concurrency, timeout, retry, rate/cost limit, SLA, PII classification, compatibility and deprecation;
- management-user identity separated from service identity, with evaluated OAuth2 client credentials, mTLS and workload JWT patterns;
- Trusted TenantContext fields and provenance; the client cannot choose tenant placement or physical routing;
- OpenAPI, AsyncAPI, Go/TypeScript SDK generation, consumer contract tests, breaking-change/deprecation validation, W3C trace context and versioned event envelopes.

Completion requires executable schemas, validators, positive and negative consumer fixtures and one reference capability. It does not implement datasource routing.

### 2. Persisted Query And Command Object Runtime

Owns server-side QueryDefinition and CommandDefinition registries:

- stable ID/version, typed argument schema, permission, tenant/data scope, allowed sort, result schema, cost and timeout;
- server predicate builders compiled to a Query AST and database-native parameterized GORM/dialect execution;
- high-risk, cross-service, federated and report reads must use persisted queries;
- low-risk Admin lists may keep restricted logical filtering only with server-declared fields, type checks, field permissions, parameterization and budgets;
- production paths must begin replacing snapshot Load/Save plus in-memory filtering where scale or isolation requires native execution;
- tests cover authorization bypass, argument tampering, replay, enumeration, count side channels, expensive queries and physical schema leakage.

The browser never submits physical fields, SQL operators, joins, SQL functions, datasource IDs or shard IDs for persisted queries.

### 3. Integration Ports Disabled By Default

The existing node moves immediately after the Service Contract and may execute in parallel with the Query/Command runtime. It owns business-neutral MessageBus, SearchIndexer and SearchReader contracts, disabled defaults, capability-profile wiring, health state and production configuration gates. It does not implement Outbox, an MQ adapter or search projection.

### 4-9. Organization, RBAC And Menu Governance

#### Contract And Migration Design

Defines the complete target and migration before runtime edits:

- role-group ownership uses `scopeType=platform|tenant` plus tenant ownership; role groups are not nested at either scope because the management model is the strict two-level `role group -> role` tree;
- each role has exactly one required `groupCode`; no role-role-group join table is introduced;
- organizations bind multiple enabled tenant role groups through `org_unit_role_groups`; every binding must match the organization's derived tenant and platform role groups cannot be bound to organizations;
- organization effective role pool is the distinct union of enabled roles from enabled bound tenant role groups;
- users have one primary organization; tenant is derived by the server and may only be redundantly persisted under an invariant; every assigned user role must remain within the effective role pool;
- platform principals without an organization are an explicit class and may only receive platform roles;
- menu schema separates `directory` and `page`, introduces `role_menu`, and separates menu assignment from API/page-button permission assignment;
- permission catalog classifies API and page-button resources without weakening Casbin as the final API boundary;
- existing `menu.permission` behavior is frozen, backfilled to `role_menu`, dual-read compared, switched and only then deprecated;
- standard Tree Transfer and browser acceptance contracts are defined before UI work.

#### Backend Constraints And Migration

Adds relational migrations, same-tenant constraints, role-pool validation and common domain validators for create, update, bulk assignment and import. Bind, unbind, organization move, role move and disable operations require impact analysis, optimistic concurrency, audit and explicit reject-or-migrate behavior. No operation silently preserves or removes out-of-pool authorization.

#### Organization And User Admin Experience

Adds organization role-group bindings, role-pool source detail, derived read-only tenant, organization-dependent multi-role selection and explicit conflict handling when organization changes.

#### Role Tree And Authorization Entry

Adds the strict two-level role-group/role tree and separate `Assign Menus` and `Assign Permissions` entry points. Role-group state never grants permission. Role permissions, deny permissions and data scope remain role-owned.

#### Menu Tree And Button Permission Configuration

Adds directory/page menu editing, route metadata, parameters, page buttons and the independent role-menu assignment contract. Directory nodes do not navigate. API and page-button permissions remain independent resources.

#### End-To-End And Browser QA

Proves migration equivalence, all backend entry points, platform-principal exceptions, conflict paths, separate audit trails, Tree Transfer keyboard/ARIA behavior, large data sets and desktop/mobile workflows.

Organization commands remain single-tenant and single-datasource. Federation and XA are forbidden for authorization decisions.

### 10. Datasource Registry Runtime Foundation

The existing `multi-datasource-contract-and-runtime` node is narrowed to:

- versioned Datasource and DatasourceGroup configuration;
- capability-to-group binding;
- encrypted configuration references, startup validation, health and status;
- one datasource/shard transaction pinning;
- adapter boundaries for MySQL, PostgreSQL, SQLite, KingbaseES and Oracle without claiming certification.

It does not own tenant placement, replicas, sharding, federation or XA.

### 11. Tenant Placement And Request Routing

Adds the authorized control plane and resolver for `tenant -> datasource group / shard / schema`, including version, status, migration state and audit. Runtime selection derives from trusted identity, capability, operation type, request purpose and placement version. Operations override routing only through a privileged, scoped and audited control-plane action.

Lifecycle's existing `datasourceID` becomes resolver input/output evidence; lifecycle remains single-datasource and is not rewritten in this node.

### 12. Datasource Read/Write Routing

Adds GORM dbresolver or an equivalent adapter inside each DatasourceGroup:

- primary/replica selection by operation type;
- transaction and write operations forced to primary;
- configurable read-after-write sticky window;
- replication-lag policy, health, fallback and failure behavior;
- routing evidence and integration tests without exposing physical targets to clients.

### 13. Sharding And Tenant Migration

Adds explicit shard keys, global IDs, routing versions, expansion, rebalancing and tenant migration. A normal transaction stays on one datasource and shard. Migration uses versioned placement state, resumable checkpoints, audit and compensation; it does not rely on XA.

### 14. Federated Read Query

Adds controlled read-only federation for persisted report queries. Ordinary OLTP repositories cannot issue arbitrary cross-database joins. Every definition has tenant isolation, allowed fields, row limits, timeout, cost, cancellation and redacted observability. Federated reads never decide authorization or mutate state.

### 15. Optional XA Adapter

Adds an advanced adapter that is default-off and never the foundation transaction path. Completion requires a resource-manager compatibility matrix, bounded timeouts, prepare/commit/rollback evidence, heuristic recovery records and operator remediation. Unsupported driver/version combinations remain experimental or unsupported.

### 16. Database Certification Matrix

The existing node becomes the aggregation gate for driver and feature claims. MySQL, PostgreSQL, SQLite, KingbaseES and Oracle are certified independently across repositories, native queries, migrations, pagination, locks, base transactions, read/write routing, sharding, federation and XA where applicable. SQLite remains local/test unless its matrix says otherwise. A compatible protocol is not certification.

Database lanes may run in parallel, but one node publishes the authoritative matrix.

### 17. Transactional Outbox And One MQ Adapter

The existing node depends on the early integration-port contract plus the certified single-source transaction path. It adds transactional Outbox, versioned event envelopes, trace context, idempotent consumption, retry, dead letter, replay and one MQ adapter for a real workload. It also owns the default cross-source consistency pattern: saga and explicit compensation. XA remains optional and is never required by Outbox.

### 18. Asynchronous Search Projection

The existing node consumes Outbox events and persisted query contracts. It adds rebuild, replay, delete synchronization, field allowlists, tenant isolation, cost limits and relational-source-of-truth boundaries. Search never becomes the authority for permissions, restore or audit.

### Open Source And Publication

The portability, community docs, docs site and GitHub publication nodes remain serial. v0.1.0 manuals and compatibility claims must state the current one-datasource, one-native-transaction boundary and mark deferred routing, sharding, federation, XA, MQ and search capabilities unsupported or default-off as applicable; later releases update those claims only after their own certification gates close. The documentation site remains the only visual/marketing surface that uses `design-taste-frontend`; Admin workflows use Product Design, platform wrappers and `ui-ux-pro-max` quality gates.

## Dependencies And Parallel Windows

Current v0.1.0 order:

```text
unified-error-code-governance
organization-rbac-menu-e2e-qa

[after both close]
  -> open-source-portability
  -> public-docs-community
  -> public-docs-site
  -> github-release-publication
```

The implementation briefs found shared HTTP, OpenAPI/codegen, audit-correlation and governance files, so there is no whole-node parallel batch. The organization E2E node declares those real shared locks in addition to its identity/UI locks. Freeze the unified error-code registry and response-envelope contract first. After that contract is stable, non-overlapping organization migration comparison and Tree Transfer performance work may overlap error-adapter migration, while shared HTTP integration, generated artifacts and both closeouts remain serial. Publication starts only after both nodes close and does not wait for deferred search work.

Dormant post-release dependency contracts:

```text
multi-datasource-contract-and-runtime
  -> tenant-placement-and-request-routing
  -> datasource-read-write-routing
  -> sharding-and-tenant-migration
  -> federated-read-query
  -> xa-optional-adapter
  -> database-certification-matrix

[integration-ports-disabled-default + database-certification-matrix]
  -> transactional-outbox-and-one-mq-adapter

[transactional-outbox-and-one-mq-adapter + persisted-query-command-object-runtime]
  -> asynchronous-search-projection
```

Cross-lane dependency contracts remain mandatory when the deferred lane is reactivated:

- `organization-rbac-menu-contract-and-migration-design` depends on `persisted-query-command-object-runtime` so high-risk role-pool, impact and migration queries use the approved authorization and cost model;
- `tenant-placement-and-request-routing` depends on `organization-role-pool-backend-and-migration` for Admin user TenantContext derivation; before that dependency closes, datasource registry work may accept only service identities whose TenantContext provenance is already trusted;
- `federated-read-query` depends on `persisted-query-command-object-runtime` in addition to the routing chain;
- all three organization UI nodes are a serial lane after the backend migration, and E2E depends on all three completed UI surfaces;
- `transactional-outbox-and-one-mq-adapter` depends on the disabled integration ports and the certified single-source transaction path;
- `asynchronous-search-projection` depends on both Outbox/MQ and the persisted query runtime.
- `open-source-portability` depends on both `organization-rbac-menu-e2e-qa` and `unified-error-code-governance`, and no longer depends on `asynchronous-search-projection` for v0.1.0 release eligibility.

Post-release parallelism remains dormant with the nodes themselves. When reactivated, database certification may use independent driver/version lanes and aggregate them into one matrix.

No parallel batch may share `capability-manifest`, `admin-resource-contract`, `storage-runtime`, `migration-runtime`, `OpenAPI/codegen`, `admin-ui` or publication locks. Each activated node must retain its complete lock set, not only the new domain locks. Query and integration-port work may run in parallel only after Service Contract freezes their separate query and event extension seams; if they still edit the same manifest, generator or generated contract, the graph must serialize them.

## Resource Lock Adjustment

The activated graph adds these exclusive domain locks:

```text
service-contract
query-command-contract
tenant-context
organization-rbac-contract
organization-rbac-migration
menu-permission-contract
datasource-registry
routing-runtime
sharding-runtime
federated-query
xa-runtime
event-contract
transaction-outbox
database-certification
data-plane-docs
identity-governance-docs
integration-docs
```

Broad `docs` remains for release-wide documentation nodes, but implementation nodes use the appropriate domain documentation lock. This removes false conflicts without allowing concurrent edits to the same contracts.

## Lifecycle Reuse Boundary

Later nodes may reuse:

- canonical `datasourceID` evidence;
- single-datasource transaction assumptions;
- persistent lease, checkpoint, dry-run and promotion patterns;
- delete, restore, purge, file tombstone and audit semantics.

Later nodes may not move TenantPlacement, sharding, federation, XA, Query Object, Outbox or search behavior into lifecycle. Lifecycle policy fingerprints remain independent from routing configuration versions.

## Activation State

The current governance state:

1. preserves 67 stable task IDs with 54 implemented closeouts and 13 controlled unfinished nodes;
2. keeps four v0.1.0 release blockers separate from nine post-release optional deferred nodes;
3. requires the active release blockers to close before publication work begins;
4. retains the deferred contracts as full-scope goal blockers without scheduling their implementation for v0.1.0;
5. keeps target menu serving and role-menu migration writes behind their explicit migration policy despite the organization E2E closeout;
6. keeps the canonical error registry as an implemented shared service-contract boundary.

No node may bypass its dependency, resource-lock or completion gate, and release eligibility must not be interpreted as persistent full-scope completion.
