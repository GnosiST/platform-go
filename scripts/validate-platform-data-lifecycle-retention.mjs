import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const paths = {
  contract: argValue("--contract", "resources/generated/admin-resource-contract.json"),
  openapi: argValue("--openapi", "resources/generated/openapi.admin.json"),
  config: argValue("--config", "internal/platform/config/config.go"),
  bootstrap: argValue("--bootstrap", "internal/platform/bootstrap/data_lifecycle.go"),
  scheduler: argValue("--scheduler", "internal/platform/bootstrap/data_lifecycle_scheduler.go"),
  api: argValue("--api", "cmd/platform-api/main.go"),
  cli: argValue("--cli", "cmd/platform-admin/main.go"),
  model: argValue("--model", "internal/platform/datalifecycle/model.go"),
  runner: argValue("--runner", "internal/platform/datalifecycle/runner.go"),
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

function goStruct(source, name) {
  return new RegExp(`type ${name} struct \\{([\\s\\S]*?)\\n\\}`).exec(source)?.[1] ?? "";
}

const errors = [];
const contract = readJSON(paths.contract);
const openapi = readJSON(paths.openapi);
const config = read(paths.config);
const bootstrap = read(paths.bootstrap);
const scheduler = read(paths.scheduler);
const api = read(paths.api);
const cli = read(paths.cli);
const model = read(paths.model);
const runner = read(paths.runner);
const engineering = readJSON(paths.engineering);
const graph = readJSON(paths.graph);
const closeout = readJSON(paths.closeout);

const expectedDeletion = new Map([
  ["auditLogs", { mode: "append-only", policyVersion: 1 }],
  ["apiTokens", { mode: "revoke", policyVersion: 1, retentionDays: 90, autoPurge: true }],
  ["files", { mode: "tombstone", policyVersion: 1, retentionDays: 30, autoPurge: true }],
]);
for (const [name, expected] of expectedDeletion) {
  const resource = (contract.resources ?? []).find((item) => item.name === name);
  if (!resource || JSON.stringify(resource.deletion) !== JSON.stringify(expected)) {
    errors.push(`generated resource ${name} must preserve its reviewed deletion policy`);
  }
}

const openapiPaths = Object.keys(openapi.paths ?? {});
if (!openapi.paths?.["/api/admin/resources/files/{id}/restore"]?.post) errors.push("Admin OpenAPI must expose file restore");
if (openapi.paths?.["/api/admin/resources/audit-logs/{id}"]?.delete) errors.push("append-only audit logs must not expose DELETE");
if (openapiPaths.some((item) => /purge/i.test(item))) errors.push("Admin OpenAPI must not expose maintenance purge");
if (JSON.stringify(openapi).includes("includeDeleted")) errors.push("Admin OpenAPI must not expose includeDeleted");

requireIncludes(config, 'boolEnvWithState("PLATFORM_RETENTION_RUNNER_ENABLED", false)', "retention config", errors);
requireIncludes(config, "defaultRetentionRunnerInterval   = 24 * time.Hour", "retention config", errors);
requireIncludes(config, "defaultRetentionRunnerBatchSize  = 100", "retention config", errors);
requireIncludes(config, "defaultRetentionRunnerMaxRetries = 3", "retention config", errors);
requireIncludes(config, "requires persistent GORM Admin resource storage and no file repository", "retention config", errors);
requireIncludes(bootstrap, "datalifecycle.OpenGORMRepository", "data lifecycle bootstrap", errors);
requireIncludes(bootstrap, "datalifecycle.NewGORMAdminResourceApplier", "data lifecycle bootstrap", errors);
requireIncludes(api, "if !cfg.RetentionRunnerEnabled", "API retention startup", errors);

for (const expected of ["CurrentFingerprint", "PromotedFingerprint", "DryRunID"]) {
  requireIncludes(goStruct(model, "PromotionApproval"), expected, "PromotionApproval", errors);
}
requireIncludes(runner, "!r.repository.Persistent()", "lifecycle runner", errors);
requireIncludes(runner, "promotion.CurrentFingerprint != approval.CurrentFingerprint", "apply promotion binding", errors);
requireIncludes(runner, "RunID: approval.DryRunID, Mode: ModeDryRun", "apply dry-run binding", errors);
requireIncludes(runner, "!dryRun.Complete || !validCheckpoint(dryRun, dryRunKey)", "apply dry-run completion gate", errors);
requireIncludes(scheduler, "CurrentFingerprint:  promotion.CurrentFingerprint, DryRunID: dryRunID", "scheduled apply", errors);
for (const expected of ['flags.String("dry-run-id"', 'flags.String("current-policy-file"', 'flags.Bool("confirm-apply"', "CurrentFingerprint: currentFingerprint, DryRunID: *dryRunID"]) {
  requireIncludes(cli, expected, "data lifecycle CLI", errors);
}

for (const [label, block] of [["Report", goStruct(model, "Report")], ["scheduler event", goStruct(scheduler, "dataLifecycleScheduleEvent")], ["promotion output", goStruct(cli, "lifecyclePromotionOutput")]]) {
  for (const forbidden of ["Reason", "ApprovalRef", "ActorID", "ObjectKey", "DSN", "Secret"]) {
    if (block.includes(forbidden)) errors.push(`${label} must not expose ${forbidden}`);
  }
}

const capability = (engineering.capabilities ?? []).find((item) => item.id === "data-lifecycle-retention");
if (capability?.status !== "implemented") errors.push("data-lifecycle-retention engineering capability must be implemented");
const expectedBoundary = {
  defaultRunner: "disabled",
  repository: "persistent-gorm-required",
  datasourceScope: "single-default",
  purgeExposure: "maintenance-only",
  dynamicDatasourceRouting: "not-implemented",
  crossDatasourceTransactions: "not-implemented",
};
if (JSON.stringify(capability?.evidence?.runtimeBoundary) !== JSON.stringify(expectedBoundary)) errors.push("data lifecycle capability boundary must remain explicit and non-expansive");
const tasks = graph.tasks ?? [];
const lifecycleTask = tasks.find((item) => item.id === "data-lifecycle-retention");
if (lifecycleTask?.status !== "implemented") errors.push("data-lifecycle-retention task must be implemented");
for (const evidencePath of ["docs/platform-data-lifecycle-retention.md", "scripts/validate-platform-data-lifecycle-retention.mjs", "internal/platform/datalifecycle/runner_test.go"]) {
  const evidence = lifecycleTask?.evidence ?? {};
  if (![...(evidence.docs ?? []), ...(evidence.validators ?? []), ...(evidence.tests ?? [])].includes(evidencePath)) errors.push(`data lifecycle task evidence must include ${evidencePath}`);
}
const lifecycleCloseout = (closeout.nodeCloseouts ?? []).find((item) => item.taskId === "data-lifecycle-retention");
if (lifecycleCloseout?.status !== "closed" || lifecycleCloseout?.neatFreak !== true) {
  errors.push("data lifecycle closeout must record the completed phase-level neat-freak cleanup");
}
for (const evidencePath of ["docs/platform-data-lifecycle-retention.md", "scripts/validate-platform-data-lifecycle-retention.mjs", "scripts/platform-data-lifecycle-retention.test.mjs"]) {
  if (!(lifecycleCloseout?.cleanupEvidence ?? []).includes(evidencePath)) errors.push(`data lifecycle closeout evidence must include ${evidencePath}`);
}
if ((closeout.pendingNodeEvidence ?? []).includes("data-lifecycle-retention")) errors.push("data lifecycle must be removed from pending closeout evidence");
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated platform data lifecycle retention governance");
