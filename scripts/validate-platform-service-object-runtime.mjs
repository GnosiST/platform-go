import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const paths = {
  contract: argValue("--contract", "resources/platform-service-object-runtime.json"),
  types: argValue("--types", "internal/platform/serviceobject/types.go"),
  registry: argValue("--registry", "internal/platform/serviceobject/registry.go"),
  request: argValue("--request", "internal/platform/serviceobject/request.go"),
  runtime: argValue("--runtime", "internal/platform/serviceobject/runtime.go"),
  executor: argValue("--executor", "internal/platform/serviceobject/gorm_executor.go"),
  idempotency: argValue("--idempotency", "internal/platform/serviceobject/idempotency_gorm.go"),
  memoryIdempotency: argValue("--memory-idempotency", "internal/platform/serviceobject/idempotency.go"),
  reference: argValue("--reference", "internal/platform/serviceobject/reference.go"),
  http: argValue("--http", "internal/platform/httpapi/server.go"),
  errorMapping: argValue("--error-mapping", "internal/platform/httpapi/service_object_error_mapping.go"),
  openapi: argValue("--openapi", "resources/generated/openapi.admin.json"),
  client: argValue("--client", "resources/generated/admin-service-object-client.ts"),
  engineering: argValue("--engineering", "resources/platform-engineering-capabilities.json"),
  graph: argValue("--graph", "resources/platform-foundation-task-graph.json"),
  closeout: argValue("--closeout", "resources/platform-node-closeout-audit.json"),
};

function read(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function readJSON(relativePath) {
  return JSON.parse(read(relativePath));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(actual, expected) {
  return values(actual).join("\u0000") === expected.join("\u0000");
}

function requireIncludes(source, expected, label, errors) {
  for (const value of expected) {
    if (!source.includes(value)) errors.push(`${label} must include ${value}`);
  }
}

const errors = [];
const contract = readJSON(paths.contract);
const types = read(paths.types);
const registry = read(paths.registry);
const request = read(paths.request);
const runtime = read(paths.runtime);
const executor = read(paths.executor);
const idempotency = read(paths.idempotency);
const memoryIdempotency = read(paths.memoryIdempotency);
const reference = read(paths.reference);
const http = read(paths.http);
const errorMapping = read(paths.errorMapping);
const openapi = readJSON(paths.openapi);
const client = read(paths.client);
const engineering = readJSON(paths.engineering);
const graph = readJSON(paths.graph);
const closeout = readJSON(paths.closeout);

if (contract.transport?.audience !== "management-user") errors.push("service-object transport audience must stay management-user");
if (contract.transport?.runtimeComposition !== "conditional-server-option") errors.push("service-object runtime composition must stay conditional-server-option");
if (contract.transport?.strictJSON !== true || contract.transport?.maximumBodyBytes !== 1048576) {
  errors.push("service-object transport must keep strict JSON with the 1 MiB limit");
}
if (!sameList(contract.clientContract?.queryFields, ["queryId", "version", "arguments", "pagination", "sort"])) {
  errors.push("service-object query client fields must stay logical and closed");
}
if (!sameList(contract.clientContract?.commandFields, ["commandId", "version", "arguments", "idempotencyKey"])) {
  errors.push("service-object command client fields must stay logical and closed");
}
for (const forbidden of ["field", "operator", "sql", "join", "dsn", "datasource", "database", "schema", "shard"]) {
  if (!values(contract.clientContract?.forbiddenInputs).includes(forbidden)) errors.push(`service-object forbiddenInputs must include ${forbidden}`);
}
if (contract.clientContract?.tenantContextClientSelectable !== false || contract.clientContract?.dataScopeClientSelectable !== false) {
  errors.push("service-object tenant and data scopes must remain server selected");
}
if (contract.definitionContract?.maximumQueryOffset !== 10000) {
  errors.push("service-object maximum query offset must stay at the platform hard limit of 10000");
}
if (!sameList(contract.definitionContract?.positiveQueryCostDimensions, ["row", "predicate", "enabled-offset", "enabled-sort", "enabled-total"])) {
  errors.push("service-object enabled query cost dimensions must remain positively priced");
}
if (contract.execution?.unknownAndUnauthorizedEquivalent !== true || contract.execution?.parameterizedClauses !== true) {
  errors.push("service-object execution must preserve enumeration resistance and parameterized clauses");
}
if (contract.idempotency?.productionStore !== "GORMIdempotencyStore" || contract.idempotency?.leaseTakeover !== true || contract.idempotency?.recordExpiry !== true) {
  errors.push("service-object idempotency must use the persistent GORM lease contract");
}
const expectedBoundary = {
  defaultAPIProcessRegistration: "not-enabled",
  workloadIdentity: "not-implemented",
  datasourceRouting: "deferred-to-multi-datasource-program",
  federatedQuery: "not-implemented",
  xa: "not-implemented",
  outboxMQ: "not-implemented",
  searchProjection: "not-implemented",
  genericLowRiskAdminFiltering: "retained",
};
if (JSON.stringify(contract.runtimeBoundary) !== JSON.stringify(expectedBoundary)) {
  errors.push("service-object runtime boundary must remain explicit and non-expansive");
}

requireIncludes(types, ["type QueryDefinition struct", "type CommandDefinition struct", "type QueryAST struct", "type CommandAST struct", "type ScopeConstraint struct"], "service-object types", errors);
requireIncludes(registry, ["maximumQueryOffset = 10000", "forbiddenClientNames", "forbiddenPhysicalPrefixes", "validateQueryDefinition", "validateCommandDefinition", "definitionKey"], "service-object registry", errors);
requireIncludes(request, ["io.LimitReader(reader, 1<<20)", "decoder.DisallowUnknownFields()", "decoder.UseNumber()"], "service-object request decoder", errors);
requireIncludes(runtime, ["r.allowed(execution, definition.Permission, definition.Action)", "queryCostExceeded", "context.WithTimeout", "validateArguments", "projectItems", "commandFingerprint"], "service-object runtime", errors);
requireIncludes(executor, ["type GORMResourceBinding struct", "applyTenant", "applyScope", "applyPredicates", "clause.Column", "MaxAffectedRows"], "service-object GORM executor", errors);
requireIncludes(idempotency, ["func NewGORMIdempotencyStore", "clause.OnConflict{DoNothing: true}", "reclaimLease", "idempotencyStatusProcessing", "idempotencyStatusCompleted", "idempotencyDatabaseError"], "service-object persistent idempotency", errors);
requireIncludes(memoryIdempotency, ["defaultMemoryIdempotencyCapacity", "removeExpiredLocked", "reserveCapacityLocked", "case <-ready:"], "service-object bounded memory idempotency", errors);
requireIncludes(reference, ["platform.reference-records.list", "platform.reference-records.rename", "ReferenceGORMBinding"], "service-object reference definitions", errors);
requireIncludes(http, ["/admin/service-objects/query", "/admin/service-objects/command", "adminServiceObjectAuthorizer", "DataScopeForPrincipal"], "service-object Admin transport", errors);
requireIncludes(errorMapping, ["CodeServiceObjectUnavailable", "CodeServiceObjectIdempotencyConflict"], "service-object Admin error mapping", errors);

for (const apiPath of [contract.transport?.queryPath, contract.transport?.commandPath]) {
  if (!openapi.paths?.[apiPath]?.post) errors.push(`Admin OpenAPI must include POST ${apiPath}`);
}
if (openapi["x-service-object-runtime-reference"] !== "internal/platform/serviceobject/reference.go") {
  errors.push("Admin OpenAPI must identify the service-object runtime reference source");
}
requireIncludes(client, ["export class AdminServiceObjectClient", "listReferenceRecords", "renameReferenceRecord", contract.transport.queryPath, contract.transport.commandPath], "generated service-object client", errors);

const capability = values(engineering.capabilities).find((item) => item.id === "persisted-query-command-object-runtime");
if (capability?.status !== "implemented") errors.push("persisted-query-command-object-runtime engineering capability must be implemented");
for (const evidencePath of ["resources/platform-service-object-runtime.json", "internal/platform/serviceobject/runtime.go", "internal/platform/serviceobject/idempotency_gorm.go", "docs/platform-service-objects.md"]) {
  if (!values(capability?.evidence?.sourcePaths).includes(evidencePath)) errors.push(`persisted query capability sourcePaths must include ${evidencePath}`);
}
if (!values(capability?.evidence?.validators).includes("scripts/validate-platform-service-object-runtime.mjs")) {
  errors.push("persisted query capability validators must include the service-object validator");
}

const task = values(graph.tasks).find((item) => item.id === "persisted-query-command-object-runtime");
if (task?.status !== "implemented") errors.push("persisted-query-command-object-runtime task must be implemented");
for (const evidencePath of ["docs/platform-service-objects.md", "scripts/validate-platform-service-object-runtime.mjs", "internal/platform/serviceobject/runtime_test.go", "internal/platform/httpapi/service_objects_test.go"]) {
  const evidence = task?.evidence ?? {};
  if (![...values(evidence.docs), ...values(evidence.validators), ...values(evidence.tests)].includes(evidencePath)) {
    errors.push(`persisted query task evidence must include ${evidencePath}`);
  }
}

const nodeCloseout = values(closeout.nodeCloseouts).find((item) => item.taskId === "persisted-query-command-object-runtime");
if (nodeCloseout?.status !== "closed" || nodeCloseout?.neatFreak !== true || nodeCloseout?.cleanupMode !== "phase-level") {
  errors.push("persisted query closeout must be phase-level, neat-freak and closed");
}
if (values(closeout.pendingNodeEvidence).includes("persisted-query-command-object-runtime")) {
  errors.push("persisted query runtime must be removed from pending closeout evidence");
}
for (const evidencePath of ["docs/platform-service-objects.md", "resources/platform-service-object-runtime.json", "scripts/validate-platform-service-object-runtime.mjs", "scripts/platform-service-object-runtime.test.mjs"]) {
  if (!values(nodeCloseout?.cleanupEvidence).includes(evidencePath)) errors.push(`persisted query closeout must include ${evidencePath}`);
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated platform service-object runtime governance");
