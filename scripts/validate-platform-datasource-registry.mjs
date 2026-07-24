import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function read(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function readJSON(relativePath) {
  return JSON.parse(read(relativePath));
}

function sameList(actual, expected) {
  return Array.isArray(actual) && actual.length === expected.length && actual.every((value, index) => value === expected[index]);
}

const paths = {
  contract: argValue("--contract", "resources/platform-datasource-registry.json"),
  registry: argValue("--registry", "internal/platform/datasource/registry.go"),
  bootstrap: argValue("--bootstrap", "internal/platform/bootstrap/capabilities.go"),
  config: argValue("--config", "internal/platform/config/config.go"),
  api: argValue("--api", "cmd/platform-api/main.go"),
  docs: argValue("--docs", "docs/platform-datasource-registry.md"),
  graph: argValue("--graph", "resources/platform-foundation-task-graph.json"),
};
const contract = readJSON(paths.contract);
const registry = read(paths.registry);
const bootstrap = read(paths.bootstrap);
const config = read(paths.config);
const api = read(paths.api);
const docs = read(paths.docs);
const graph = readJSON(paths.graph);
const errors = [];

if (contract.version !== "v1") errors.push("datasource registry contract version must be v1");
if (contract.connectionMaterial !== "process-owned-config-ref-only") errors.push("datasource registry must retain process-owned config references only");
if (contract.clientPhysicalRouting !== "forbidden") errors.push("datasource registry must forbid client physical routing");
if (!sameList(contract.startup, ["resolve", "open", "ping", "redacted-health"])) errors.push("datasource registry startup sequence must remain resolve, open, ping, redacted-health");
if (contract.transactionBoundary !== "one-datasource-one-shard") errors.push("datasource registry must retain one-datasource-one-shard transaction boundary");
if (!sameList(contract.deferred, ["tenant-placement", "read-write-routing", "sharding", "federation", "xa"])) errors.push("datasource registry deferred boundaries must remain explicit");

for (const snippet of [
  "type Spec struct", "ConfigRef", "func Open", "ResolveDatasource", "PingContext", "func (r *Runtime) ValidateCapabilityBindings", "func (r *Runtime) Transaction", "func (r *Runtime) Health", "DatasourceID string `json:\"datasourceId\"`", "Ready        bool   `json:\"ready\"`",
]) {
  if (!registry.includes(snippet)) errors.push(`datasource registry source must include ${snippet}`);
}
for (const forbidden of ["DSN", "Password", "Database", "Schema"]) {
  if (read(paths.contract).includes(`\"${forbidden}\"`)) errors.push(`datasource registry contract must not carry ${forbidden}`);
}
for (const snippet of ["DatasourceRegistryFile", "PLATFORM_DATASOURCE_REGISTRY_FILE", "DatasourceRuntimeFromConfig"]) {
  if (!config.includes(snippet) && !bootstrap.includes(snippet)) errors.push(`datasource registry configuration must include ${snippet}`);
}
for (const snippet of ["bootstrap.DatasourceRuntimeFromConfig", "datasource registry datasource=%s ready=%t"]) {
  if (!api.includes(snippet)) errors.push(`platform API must include datasource registry startup evidence ${snippet}`);
}
for (const snippet of ["PLATFORM_DATASOURCE_REGISTRY_FILE", "does not permit client-controlled", "does not enable tenant placement"]) {
  if (!docs.includes(snippet)) errors.push(`datasource registry documentation must include ${snippet}`);
}

const task = (graph.tasks ?? []).find((item) => item.id === "multi-datasource-contract-and-runtime");
if (!task) {
  errors.push("task graph must include multi-datasource-contract-and-runtime");
} else {
  for (const evidencePath of ["docs/platform-datasource-registry.md", "scripts/validate-platform-datasource-registry.mjs", "scripts/platform-datasource-registry.test.mjs", "internal/platform/datasource/registry_test.go"]) {
    const evidence = [...(task.evidence?.docs ?? []), ...(task.evidence?.validators ?? []), ...(task.evidence?.tests ?? [])];
    if (!evidence.includes(evidencePath)) errors.push(`datasource registry task evidence must include ${evidencePath}`);
  }
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated datasource registry contract");
