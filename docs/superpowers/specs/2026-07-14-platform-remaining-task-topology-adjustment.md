# Platform Remaining Task Topology Adjustment

> **Status:** Activated governance topology with the 2026-07-16 release overlay. The current task graph projection is `67 total / 52 implemented / 15 controlled unfinished` and remains `not-complete-controlled`.

## Authority

This document is the activated and only authoritative topology for the remaining platform program.

## 2026-07-16 Release Overlay

Task 6 closes `menu-tree-and-button-permission-configuration`, adds `unified-error-code-governance`, and separates current unfinished work into two machine-checkable lanes.

- v0.1.0 release blockers: `organization-rbac-menu-e2e-qa`, `unified-error-code-governance`, `open-source-portability`, `public-docs-community`, `public-docs-site`, `github-release-publication`.
- Post-release optional deferred nodes: `multi-datasource-contract-and-runtime`, `tenant-placement-and-request-routing`, `datasource-read-write-routing`, `sharding-and-tenant-migration`, `federated-read-query`, `xa-optional-adapter`, `database-certification-matrix`, `transactional-outbox-and-one-mq-adapter`, `asynchronous-search-projection`.
- `open-source-portability` now depends on `admin-watermark-export-governance`, `organization-rbac-menu-e2e-qa` and `unified-error-code-governance`; no release blocker may depend directly or transitively on a deferred node.
- Deferred nodes remain full-goal blockers. Release eligibility can become ready when the release-blocker lane is empty, but the persistent foundation objective cannot become complete while deferred nodes remain unfinished.
- `unified-error-code-governance` owns the future canonical registry, ownership/audience/HTTP/retry/redaction/compatibility/deprecation metadata, request/trace correlation, generated public contracts and duplicate/reassignment/deprecation validation. This overlay registers the node but does not implement the registry.

- The consolidated SaaS data-plane, persisted Query Object and Platform Service Contract input confirmed on 2026-07-14 is authoritative.
- Earlier cross-session variants describing only static datasources, separate Query Object drafts, SQL-less drafts or partial service-contract drafts are superseded and must not be accumulated as extra requirements.
- The organization/RBAC/menu decision remains active: one role belongs to one role group; an organization may bind multiple role groups; role groups classify and isolate roles but do not grant permissions.
- `data-lifecycle-retention` remains closed at commit `0de9f2d7`. This activated topology may reuse its contracts but must not reopen or expand that node.

## Current Conflict Summary

The original nine unfinished nodes remain valid program areas, but activation expands and reorders them because their previous boundaries were insufficient:

- `multi-datasource-contract-and-runtime` only describes named sources and capability bindings. It cannot also own TenantPlacement, trusted request routing, read/write routing, sharding, federation and XA.
- `integration-ports-disabled-default` is incorrectly placed after database certification even though Event Plane contracts and disabled ports can be completed earlier.
- the repository has Admin and App contracts but no executable standard for Admin, Service/Data, Control, External/Partner and Event planes;
- high-risk queries still lack server-persisted QueryDefinition contracts and a database-native QueryExecutor;
- organization, role-group, role, menu and permission contracts conflict with the confirmed target model and require a migration rather than an in-place visual rewrite;
- a broad exclusive `docs` lock makes otherwise independent contract and runtime work serial.

## Stable Data Plane

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

## Adjusted Unfinished Node Order

The original activation preserved the then-current 44 implemented nodes and replaced the former nine-node unfinished projection with this ordered 22-node activation snapshot. The 2026-07-16 release overlay above is the current projection:

1. `platform-service-contract-standard`
2. `persisted-query-command-object-runtime`
3. `integration-ports-disabled-default`
4. `organization-rbac-menu-contract-and-migration-design`
5. `organization-role-pool-backend-and-migration`
6. `organization-user-admin-experience`
7. `role-tree-and-authorization-entry`
8. `menu-tree-and-button-permission-configuration`
9. `organization-rbac-menu-e2e-qa`
10. `multi-datasource-contract-and-runtime`
11. `tenant-placement-and-request-routing`
12. `datasource-read-write-routing`
13. `sharding-and-tenant-migration`
14. `federated-read-query`
15. `xa-optional-adapter`
16. `database-certification-matrix`
17. `transactional-outbox-and-one-mq-adapter`
18. `asynchronous-search-projection`
19. `open-source-portability`
20. `public-docs-community`
21. `public-docs-site`
22. `github-release-publication`

The existing IDs in this list remain stable. `multi-datasource-contract-and-runtime` is deliberately narrowed rather than renamed so current governance references do not create a second identity.

## Node Boundaries And Completion Gates

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

### 19-22. Open Source And Publication

The existing portability, community docs, docs site and GitHub publication nodes remain in order. Public manuals and compatibility claims must reflect the final service, query, routing, database, messaging and search matrices. The documentation site remains the only visual/marketing surface that uses `design-taste-frontend`; Admin workflows use Product Design, platform wrappers and `ui-ux-pro-max` quality gates.

## Dependencies And Parallel Windows

Required order:

```text
platform-service-contract-standard
  -> persisted-query-command-object-runtime
  -> organization-rbac-menu-contract-and-migration-design
  -> organization-role-pool-backend-and-migration
  -> organization-user-admin-experience
  -> role-tree-and-authorization-entry
  -> menu-tree-and-button-permission-configuration
  -> organization-rbac-menu-e2e-qa

platform-service-contract-standard
  -> integration-ports-disabled-default

platform-service-contract-standard
  -> multi-datasource-contract-and-runtime
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

Additional cross-lane dependencies are mandatory:

- `organization-rbac-menu-contract-and-migration-design` depends on `persisted-query-command-object-runtime` so high-risk role-pool, impact and migration queries use the approved authorization and cost model;
- `tenant-placement-and-request-routing` depends on `organization-role-pool-backend-and-migration` for Admin user TenantContext derivation; before that dependency closes, datasource registry work may accept only service identities whose TenantContext provenance is already trusted;
- `federated-read-query` depends on `persisted-query-command-object-runtime` in addition to the routing chain;
- all three organization UI nodes are a serial lane after the backend migration, and E2E depends on all three completed UI surfaces;
- `transactional-outbox-and-one-mq-adapter` depends on the disabled integration ports and the certified single-source transaction path;
- `asynchronous-search-projection` depends on both Outbox/MQ and the persisted query runtime.
- `open-source-portability` depends on both `organization-rbac-menu-e2e-qa` and `unified-error-code-governance`, and no longer depends on `asynchronous-search-projection` for v0.1.0 release eligibility.

Approved parallelism:

1. After Service Contract: `persisted-query-command-object-runtime` and `integration-ports-disabled-default` may run in parallel.
2. After organization backend contracts freeze: the three organization UI nodes form one serial UI lane while the datasource lane continues from the already-started registry into tenant placement and read/write routing. The two lanes may run in parallel when their file and lock sets do not overlap. Tenant placement cannot start before the organization backend freeze when Admin user TenantContext is in scope.
3. Database certification executes driver/version lanes in parallel and aggregates them into one matrix.
4. Open-source publication remains serial after search closeout because module-path migration, public docs, Pages and release evidence share release state.

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

The governance activation:

1. adds 13 new task IDs and reorders the 9 existing unfinished IDs into the exact 22-node projection above;
2. records the required dependencies, completion gates, full resource locks and approved parallel windows;
3. projects execution, goal, closeout, objective, alignment and engineering governance as `66/44/22` while preserving all 44 implemented closeouts;
4. records the current direct-tenant, nested-role-group and `menu.permission` behavior as migration source rather than retained target behavior;
5. requires the governance-topology contract, Admin resource schema, tests and validators to migrate with the organization/RBAC/menu nodes;
6. keeps all 22 nodes controlled unfinished until their own implementation and evidence closeouts pass.

The activation commit established the prerequisite for runtime implementation. It does not authorize an implementation node to bypass its dependency, resource-lock or completion gate.
