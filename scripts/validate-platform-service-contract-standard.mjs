import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { loadPlatformErrorContract, validateOpenAPIErrorContract } from "./platform-error-contract.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const paths = {
  standard: argValue("--standard", "resources/platform-service-contract-standard.json"),
  contract: argValue("--contract", "resources/generated/platform-service-contract.json"),
  baseline: argValue("--baseline", "resources/fixtures/platform-service-contract/compatibility-baseline.json"),
  serviceOpenAPI: argValue("--service-openapi", "resources/generated/openapi.service.json"),
  controlOpenAPI: argValue("--control-openapi", "resources/generated/openapi.control.json"),
  externalOpenAPI: argValue("--external-openapi", "resources/generated/openapi.external.json"),
  asyncapi: argValue("--asyncapi", "resources/generated/asyncapi.events.json"),
  goSDK: argValue("--go-sdk", "resources/generated/service-sdk/go/service_contract_sdk.go"),
  typeScriptSDK: argValue("--typescript-sdk", "resources/generated/service-sdk/typescript/serviceContractSDK.ts"),
  positiveConsumer: argValue("--positive-consumer", "resources/fixtures/platform-service-contract/valid-file-storage-consumer.json"),
  negativeConsumer: argValue("--negative-consumer", "resources/fixtures/platform-service-contract/rejected-physical-routing-consumer.json"),
  codegen: argValue("--codegen", "resources/platform-codegen-source-writing-readiness.json"),
  graph: argValue("--graph", "resources/platform-foundation-task-graph.json"),
  engineering: argValue("--engineering", "resources/platform-engineering-capabilities.json"),
  closeout: argValue("--closeout", "resources/platform-node-closeout-audit.json"),
  errorContract: argValue("--error-contract", "resources/generated/platform-error-code-contract.json"),
};

function read(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function readJSON(relativePath) {
  return JSON.parse(read(relativePath));
}

function values(value) {
  return Array.isArray(value) ? value : [];
}

function sameList(actual, expected) {
  return actual.length === expected.length && actual.every((value, index) => value === expected[index]);
}

function sameSet(actual, expected) {
  return sameList([...actual].sort(), [...expected].sort());
}

function major(version) {
  return Number.parseInt(String(version).split(".")[0], 10);
}

function compareVersions(actual, expected) {
  const actualParts = String(actual).split(".").map((part) => Number.parseInt(part, 10));
  const expectedParts = String(expected).split(".").map((part) => Number.parseInt(part, 10));
  for (let index = 0; index < Math.max(actualParts.length, expectedParts.length); index += 1) {
    const difference = (actualParts[index] ?? 0) - (expectedParts[index] ?? 0);
    if (difference !== 0) return Math.sign(difference);
  }
  return 0;
}

function contractMap(contract) {
  return new Map(values(contract.services).map((service) => [service.id, service]));
}

function indexed(valuesList) {
  return new Map(values(valuesList).map((item) => [item.id, item]));
}

function preserveScalar(errors, label, field, previousValue, currentValue) {
  if (currentValue !== previousValue) errors.push(`${label} ${field} must not change within the same major version`);
}

function preserveSet(errors, label, field, previousValue, currentValue) {
  if (!sameSet(values(currentValue), values(previousValue))) errors.push(`${label} ${field} must not change within the same major version`);
}

function preserveSchema(errors, label, field, previousSchema, currentSchema) {
  for (const schemaField of ["ref", "pii"]) {
    preserveScalar(errors, label, `${field}.${schemaField}`, previousSchema?.[schemaField], currentSchema?.[schemaField]);
  }
  const previousRequired = values(previousSchema?.requiredFields);
  const currentRequired = values(currentSchema?.requiredFields);
  if (previousRequired.some((requiredField) => !currentRequired.includes(requiredField))) {
    errors.push(`${label} ${field} must not remove required fields within the same major version`);
  }
  if (currentRequired.some((requiredField) => !previousRequired.includes(requiredField))) {
    errors.push(`${label} ${field} must not add required fields within the same major version`);
  }
}

function compatibilityErrors(previous, current) {
  const errors = [];
  const currentServices = contractMap(current);
  for (const previousService of values(previous.services).filter((service) => service.stability === "stable")) {
    const currentService = currentServices.get(previousService.id);
    if (!currentService) {
      errors.push(`stable service ${previousService.id} must not be removed`);
      continue;
    }
    if (compareVersions(currentService.version, previousService.version) < 0) {
      errors.push(`stable service ${previousService.id} version must not be downgraded`);
    }
    if (major(currentService.version) > major(previousService.version)) continue;
    if (major(currentService.version) < major(previousService.version)) continue;
    const serviceLabel = `stable service ${previousService.id}`;
    preserveScalar(errors, serviceLabel, "stability", previousService.stability, currentService.stability);
    for (const field of ["audiences", "identityModes", "authModes"]) preserveSet(errors, serviceLabel, field, previousService[field], currentService[field]);
    const currentOperations = indexed(currentService.operations);
    for (const previousOperation of values(previousService.operations)) {
      const currentOperation = currentOperations.get(previousOperation.id);
      if (!currentOperation) {
        errors.push(`stable operation ${previousService.id}.${previousOperation.id} must not be removed within the same major version`);
        continue;
      }
      const label = `stable operation ${previousService.id}.${previousOperation.id}`;
      for (const field of ["kind", "plane", "runtimeStatus", "identityMode", "tenantMode", "method", "path", "requestMediaType", "responseMediaType", "successStatus"]) {
        preserveScalar(errors, label, field, previousOperation[field], currentOperation[field]);
      }
      for (const field of ["authModes", "permissions", "dataScopes"]) preserveSet(errors, label, field, previousOperation[field], currentOperation[field]);
      for (const schemaField of ["requestSchema", "responseSchema"]) preserveSchema(errors, label, schemaField, previousOperation[schemaField], currentOperation[schemaField]);
      for (const field of ["idempotency", "optimisticConcurrency", "timeoutMilliseconds", "maxRetries", "rateLimitPerMinute", "costLimit"]) {
        preserveScalar(errors, label, `reliability.${field}`, previousOperation.reliability?.[field], currentOperation.reliability?.[field]);
      }
    }
    const currentEvents = indexed(currentService.events);
    for (const previousEvent of values(previousService.events)) {
      const currentEvent = currentEvents.get(previousEvent.id);
      if (!currentEvent) {
        errors.push(`stable event ${previousService.id}.${previousEvent.id} must not be removed within the same major version`);
        continue;
      }
      const label = `stable event ${previousService.id}.${previousEvent.id}`;
      for (const field of ["name", "version", "direction", "runtimeStatus", "tenantMode", "envelopeVersion"]) {
        preserveScalar(errors, label, field, previousEvent[field], currentEvent[field]);
      }
      for (const field of ["permissions", "dataScopes"]) preserveSet(errors, label, field, previousEvent[field], currentEvent[field]);
      preserveSchema(errors, label, "payloadSchema", previousEvent.payloadSchema, currentEvent.payloadSchema);
    }
  }
  return errors;
}

function consumerErrors(fixture, contract, standard) {
  const errors = [];
  const service = contractMap(contract).get(fixture.serviceId);
  if (!service) errors.push(`consumer service ${fixture.serviceId} is not declared`);
  const operationIDs = new Set(values(service?.operations).map((operation) => operation.id));
  const eventIDs = new Set(values(service?.events).map((event) => event.id));
  for (const operation of values(fixture.operations)) {
    if (!operationIDs.has(operation)) errors.push(`consumer operation ${operation} is not declared`);
  }
  for (const event of values(fixture.events)) {
    if (!eventIDs.has(event)) errors.push(`consumer event ${event} is not declared`);
  }
  if (fixture.physicalRouting && Object.keys(fixture.physicalRouting).length > 0) {
    errors.push("consumer must not select physical routing");
  }
  if (!values(standard.tenantContext?.provenance).includes(fixture.tenantContextSource)) {
    errors.push("consumer tenant context source is not trusted");
  }
  return errors;
}

function artifactSourceHash(document) {
  return document["x-source-hash"];
}

const errors = [];
const standard = readJSON(paths.standard);
const contract = readJSON(paths.contract);
const baseline = readJSON(paths.baseline);
const serviceOpenAPI = readJSON(paths.serviceOpenAPI);
const controlOpenAPI = readJSON(paths.controlOpenAPI);
const externalOpenAPI = readJSON(paths.externalOpenAPI);
const asyncapi = readJSON(paths.asyncapi);
const goSDK = read(paths.goSDK);
const typeScriptSDK = read(paths.typeScriptSDK);
const positiveConsumer = readJSON(paths.positiveConsumer);
const negativeConsumer = readJSON(paths.negativeConsumer);
const codegen = readJSON(paths.codegen);
const graph = readJSON(paths.graph);
const engineering = readJSON(paths.engineering);
const closeout = readJSON(paths.closeout);
const errorContract = loadPlatformErrorContract(repoRoot, paths.errorContract);

if (standard.contractVersion !== contract.contractVersion) errors.push("standard and generated contract versions must match");
for (const [field, expected] of [
  ["planes", ["admin", "service", "control", "external", "event"]],
  ["identityModes", ["management-user", "workload"]],
  ["authModes", ["admin-session", "app-session", "api-token", "oauth2-client-credentials", "mtls", "workload-jwt"]],
]) {
  if (!sameList(values(standard[field]), expected)) errors.push(`standard ${field} must match the approved service contract`);
}
if (!sameList(values(standard.compatibility?.sameMajorMustPreserve), [
  "stable-service-version-and-classification",
  "service-identity-and-auth-modes",
  "operation-identity-and-http-binding",
  "operation-success-status",
  "request-response-schema-ref-pii-required-fields",
  "reliability-semantics",
  "event-identity-direction-envelope-payload",
  "tenantMode",
  "permissions",
  "dataScopes",
])) {
  errors.push("standard compatibility policy must enumerate every enforced same-major boundary");
}
if (standard.tenantContext?.clientPhysicalRoutingSelectable !== false || contract.tenantContext?.clientPhysicalRoutingSelectable !== false) {
  errors.push("trusted tenant context must forbid client physical routing selection");
}
if (!sameSet(values(standard.tenantContext?.forbiddenClientFields), ["dsn", "datasource", "database", "schema", "shard"])) {
  errors.push("standard tenant context must forbid every physical routing field");
}
if (standard.traceContext?.standard !== "W3C Trace Context" || !sameList(values(contract.traceContext?.fields), ["traceparent", "tracestate"])) {
  errors.push("service contract must preserve W3C trace context fields");
}
if (standard.eventEnvelope?.version !== contract.eventEnvelope?.version || contract.eventEnvelope?.runtimeStatus !== "schema-only") {
  errors.push("event envelope must remain versioned and schema-only in this node");
}
if (standard.eventEnvelope?.tenantContextRequirement !== "required-only-for-tenant-required-events" || contract.eventEnvelope?.tenantContextRequirement !== standard.eventEnvelope.tenantContextRequirement) {
  errors.push("event envelope tenant context must follow the declared event tenant mode");
}
if (!/^sha256:[0-9a-f]{64}$/.test(contract.contractHash ?? "")) errors.push("generated service contract must include a canonical sha256 hash");

const reference = contractMap(contract).get("file-storage");
if (!reference || reference.capabilityId !== "file-storage" || reference.stability !== "stable") errors.push("file-storage must remain the stable business-neutral reference service");
const referenceOperations = indexed(reference?.operations);
for (const [id, method, route] of [["upload-file", "POST", "/api/app/files"], ["read-file-content", "GET", "/api/app/files/:id/content"]]) {
  const operation = referenceOperations.get(id);
  if (operation?.runtimeStatus !== "bound" || operation?.method !== method || operation?.path !== route || operation?.plane !== "external") {
    errors.push(`file-storage ${id} must match the existing App route binding`);
  }
}
for (const operation of values(reference?.operations).filter((item) => item.plane !== "external")) {
  if (operation.runtimeStatus !== "contract-only" || operation.method || operation.path) errors.push(`file-storage ${operation.id} must stay contract-only`);
}
for (const event of values(reference?.events)) {
  if (event.runtimeStatus !== "contract-only") errors.push(`file-storage event ${event.id} must not claim runtime delivery`);
}

for (const [label, artifact] of [["service OpenAPI", serviceOpenAPI], ["control OpenAPI", controlOpenAPI], ["external OpenAPI", externalOpenAPI], ["AsyncAPI", asyncapi]]) {
  if (artifactSourceHash(artifact) !== contract.contractHash) errors.push(`${label} source hash must match the canonical service contract`);
}
for (const [label, artifact, planes] of [
  ["service OpenAPI", serviceOpenAPI, ["service", "data"]],
  ["control OpenAPI", controlOpenAPI, ["control"]],
  ["external OpenAPI", externalOpenAPI, ["external"]],
]) {
  errors.push(...validateOpenAPIErrorContract(artifact, errorContract, { label, planes }));
}
if (!externalOpenAPI.paths?.["/api/app/files"]?.post || !externalOpenAPI.paths?.["/api/app/files/{id}/content"]?.get) errors.push("external OpenAPI must preserve bound file-storage routes");
if (Object.keys(serviceOpenAPI.paths ?? {}).length !== 0 || Object.keys(controlOpenAPI.paths ?? {}).length !== 0) errors.push("contract-only service and control operations must not claim OpenAPI runtime paths");
if (!values(serviceOpenAPI["x-platform-contract-only-operations"]).length || !values(controlOpenAPI["x-platform-contract-only-operations"]).length) errors.push("service and control OpenAPI must retain contract-only declarations");
if (asyncapi.asyncapi !== "3.0.0" || !Object.keys(asyncapi.channels ?? {}).length) errors.push("AsyncAPI must expose versioned Event Plane channels");
for (const channel of Object.values(asyncapi.channels ?? {})) {
  if (channel["x-platform-runtime-status"] !== "contract-only") errors.push("AsyncAPI channels must remain contract-only until delivery runtime is implemented");
}

for (const [label, sdk] of [["Go SDK", goSDK], ["TypeScript SDK", typeScriptSDK]]) {
  if (!sdk.includes("Code generated by scripts/generate-platform-service-contract-artifacts.mjs; DO NOT EDIT.")) errors.push(`${label} must carry the generated marker`);
  if (!sdk.includes("TenantContext") || !sdk.includes("TraceContext") || !sdk.includes("EventEnvelope")) errors.push(`${label} must expose tenant, trace and event envelope types`);
  if (/\b(dsn|datasource|database|shard)\b/i.test(sdk)) errors.push(`${label} must not expose physical routing controls`);
}
if (codegen.mode?.sourceWriting !== "disabled" || codegen.sourceWritingApprovalPackage?.mustNotEnableSourceWriting !== true) errors.push("runtime source-writing must remain disabled");

errors.push(...compatibilityErrors(baseline, contract));
const positiveErrors = consumerErrors(positiveConsumer, contract, standard);
if (positiveErrors.length) errors.push(...positiveErrors.map((error) => `positive consumer: ${error}`));
const negativeErrors = consumerErrors(negativeConsumer, contract, standard);
if (!negativeErrors.includes(negativeConsumer.expectedError)) errors.push("negative consumer fixture must prove physical routing rejection");

const task = values(graph.tasks).find((item) => item.id === "platform-service-contract-standard");
if (task?.status !== "implemented") errors.push("platform-service-contract-standard task must be implemented before closeout");
const capability = values(engineering.capabilities).find((item) => item.id === "platform-service-contract-standard");
if (capability?.status !== "implemented") errors.push("platform-service-contract-standard engineering capability must be implemented");
if (!sameList(values(capability?.dependsOn), ["data-lifecycle-retention", "capability-contract-governance"])) errors.push("service contract capability dependencies must include lifecycle and capability governance");
const nodeCloseout = values(closeout.nodeCloseouts).find((item) => item.taskId === "platform-service-contract-standard");
if (nodeCloseout?.status !== "closed" || nodeCloseout?.neatFreak !== true) errors.push("service contract closeout must record the phase-level neat-freak cleanup");

if (errors.length) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated platform service contract standard");
