import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const paths = {
  contract: argValue("--contract", "resources/platform-integration-ports.json"),
  ports: argValue("--ports", "internal/platform/integration/integration.go"),
  disabled: argValue("--disabled", "internal/platform/integration/disabled.go"),
  bootstrap: argValue("--bootstrap", "internal/platform/bootstrap/integration.go"),
  config: argValue("--config", "internal/platform/config/config.go"),
  api: argValue("--api", "cmd/platform-api/main.go"),
  compose: argValue("--compose", "deploy/compose/docker-compose.prod.yml"),
  env: argValue("--env", "deploy/env/production.example.env"),
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

function requireIncludes(source, expected, label, errors) {
  if (!source.includes(expected)) errors.push(`${label} must include ${expected}`);
}

const errors = [];
const contract = readJSON(paths.contract);
const ports = read(paths.ports);
const disabled = read(paths.disabled);
const bootstrap = read(paths.bootstrap);
const config = read(paths.config);
const api = read(paths.api);
const compose = read(paths.compose);
const env = read(paths.env);
const engineering = readJSON(paths.engineering);
const graph = readJSON(paths.graph);
const closeout = readJSON(paths.closeout);

if (contract.defaultState !== "disabled") errors.push("integration ports defaultState must stay disabled");
if (JSON.stringify(contract.healthStates) !== JSON.stringify(["disabled", "ready", "unavailable"])) {
  errors.push("integration ports healthStates must stay disabled, ready, unavailable");
}
for (const id of ["message-bus", "search-indexer", "search-reader"]) {
  if (!(contract.ports ?? []).some((item) => item.id === id)) errors.push(`integration contract must include ${id}`);
}
const expectedBoundary = {
  capabilityProfilesImplicitlyEnable: false,
  vendorAdapters: "not-implemented",
  transactionalOutbox: "not-implemented",
  deadLetterQueue: "not-implemented",
  replay: "not-implemented",
  searchProjection: "not-implemented",
  adapterHealthErrorExposure: "redacted",
};
if (JSON.stringify(contract.runtimeBoundary) !== JSON.stringify(expectedBoundary)) {
  errors.push("integration runtime boundary must remain explicit and non-expansive");
}

for (const expected of ["type MessageBus interface", "type SearchIndexer interface", "type SearchReader interface", "func (r Runtime) Status"]) {
  requireIncludes(ports, expected, "integration ports", errors);
}
for (const expected of [
  "func (disabledMessageBus) Publish(context.Context, Message) error {\n\treturn ErrMessageBusDisabled\n}",
  "func (disabledSearchIndexer) Index(context.Context, SearchDocument) error {\n\treturn ErrSearchDisabled\n}",
  "func (disabledSearchIndexer) Delete(context.Context, SearchDocumentRef) error {\n\treturn ErrSearchDisabled\n}",
  "func (disabledSearchReader) Search(context.Context, SearchRequest) (SearchResult, error) {\n\treturn SearchResult{}, ErrSearchDisabled\n}",
]) {
  requireIncludes(disabled, expected, "disabled integrations", errors);
}
for (const expected of ["validateIntegrationAdapter", "interfaceNil", "NewDisabledMessageBus", "NewDisabledSearchIndexer", "NewDisabledSearchReader"]) {
  requireIncludes(bootstrap, expected, "integration bootstrap", errors);
}
for (const expected of [
  'boolEnvWithState("PLATFORM_MESSAGE_BUS_ENABLED", false)',
  'boolEnvWithState("PLATFORM_SEARCH_ENABLED", false)',
  'production runtime requires %s to be explicitly configured',
]) {
  requireIncludes(config, expected, "integration config", errors);
}
requireIncludes(api, "bootstrap.IntegrationsFromConfig", "platform API startup", errors);
requireIncludes(api, "integrationRuntime.Status(ctx)", "platform API integration status", errors);
requireIncludes(api, "optional integration capability=%s enabled=%t state=%s adapter=%s", "platform API integration status", errors);

for (const key of ["PLATFORM_MESSAGE_BUS_ENABLED", "PLATFORM_SEARCH_ENABLED"]) {
  requireIncludes(compose, `${key}: \${${key}:?required}`, "production Compose", errors);
  requireIncludes(env, `${key}=false`, "production environment template", errors);
}
for (const key of ["PLATFORM_MESSAGE_BUS_ADAPTER", "PLATFORM_SEARCH_ADAPTER"]) {
  requireIncludes(env, `${key}=`, "production environment template", errors);
}

const capability = (engineering.capabilities ?? []).find((item) => item.id === "integration-ports-disabled-default");
if (capability?.status !== "implemented") errors.push("integration-ports-disabled-default engineering capability must be implemented");
if (JSON.stringify(capability?.evidence?.runtimeBoundary) !== JSON.stringify(expectedBoundary)) {
  errors.push("integration engineering boundary must match the executable contract");
}
const task = (graph.tasks ?? []).find((item) => item.id === "integration-ports-disabled-default");
if (task?.status !== "implemented") errors.push("integration-ports-disabled-default task must be implemented");
for (const evidencePath of ["docs/platform-integration-ports.md", "scripts/validate-platform-integration-ports.mjs", "internal/platform/integration/integration_test.go"]) {
  const evidence = task?.evidence ?? {};
  if (![...(evidence.docs ?? []), ...(evidence.validators ?? []), ...(evidence.tests ?? [])].includes(evidencePath)) {
    errors.push(`integration task evidence must include ${evidencePath}`);
  }
}
const nodeCloseout = (closeout.nodeCloseouts ?? []).find((item) => item.taskId === "integration-ports-disabled-default");
if (nodeCloseout?.status !== "closed") errors.push("integration ports closeout must be closed");
for (const evidencePath of ["docs/platform-integration-ports.md", "scripts/validate-platform-integration-ports.mjs", "scripts/platform-integration-ports.test.mjs"]) {
  if (!(nodeCloseout?.cleanupEvidence ?? []).includes(evidencePath)) errors.push(`integration closeout evidence must include ${evidencePath}`);
}
if ((closeout.pendingNodeEvidence ?? []).includes("integration-ports-disabled-default")) {
  errors.push("integration ports must be removed from pending closeout evidence");
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated platform integration ports governance");
