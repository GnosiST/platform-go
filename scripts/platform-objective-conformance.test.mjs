import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const completionProgramTaskIDs = [
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-objective-conformance.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-objective-conformance-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-objective-conformance", () => {
  it("tracks the ordered completion program as controlled objective blockers", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");

    assert.deepEqual(audit.taskControlPolicy.requiredUnfinishedNodes, completionProgramTaskIDs);
    assert.equal(audit.completionPolicy.goalCompletionStatus, "not-complete-controlled");
    assert.deepEqual(audit.completionPolicy.controlledBlockers, completionProgramTaskIDs);
  });

  it("accepts the current objective conformance contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform objective conformance/);
  });

  it("rejects turning zshenmez from reference-only input into a migration source", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.referenceProjectPolicy.mode = "migration-source";
    audit.referenceProjectPolicy.defaultPlatformBusinessMigration = "allowed";
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /referenceProjectPolicy\.mode must stay reference-only/);
    assert.match(result.stderr, /referenceProjectPolicy\.defaultPlatformBusinessMigration must stay forbidden/);
  });

  it("rejects visual workflow gates that do not run brainstorming before product-design", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.visualPolicy.requiredOrder = ["product-design", "superpowers:brainstorming"];
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visualPolicy\.requiredOrder must stay superpowers:brainstorming before product-design/);
  });

  it("rejects capability policy drift away from plugin-style contracts", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.capabilityPolicy.runtimeManifestMutation = "allowed";
    audit.capabilityPolicy.defaultProfileBusinessCapabilitiesAllowed = true;
    const contracts = readJSON("resources/platform-capability-contracts.json");
    contracts.policies.defaultProfileBusinessCapabilitiesAllowed = true;
    const auditPath = tempJSON("platform-objective-conformance.json", audit);
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--audit", auditPath, "--capability-contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capabilityPolicy\.runtimeManifestMutation must stay forbidden/);
    assert.match(result.stderr, /capabilityPolicy\.defaultProfileBusinessCapabilitiesAllowed must stay false/);
  });

  it("rejects objective evidence that drops capability contract governance", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.contracts = audit.evidence.contracts.filter((item) => item !== "resources/platform-capability-contracts.json");
    audit.evidence.validators = audit.evidence.validators.filter((item) => item !== "scripts/validate-platform-capability-contracts.mjs");
    audit.evidence.tests = audit.evidence.tests.filter((item) => item !== "scripts/platform-capability-contracts.test.mjs");
    const engineering = readJSON("resources/platform-engineering-capabilities.json");
    engineering.capabilities = engineering.capabilities.filter((capability) => capability.id !== "capability-contract-governance");
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "capability-contracts");
    const auditPath = tempJSON("platform-objective-conformance.json", audit);
    const engineeringPath = tempJSON("platform-engineering-capabilities.json", engineering);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--audit", auditPath, "--engineering", engineeringPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /engineering capability matrix must include capability-contract-governance/);
    assert.match(result.stderr, /production readiness preflightCommands must include capability-contracts/);
    assert.match(result.stderr, /objective conformance evidence must include resources\/platform-capability-contracts\.json/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/validate-platform-capability-contracts\.mjs/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-capability-contracts\.test\.mjs/);
  });

  it("rejects objective evidence without the foundation alignment drift test", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.tests = audit.evidence.tests.filter((item) => item !== "scripts/platform-foundation-alignment.test.mjs");
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-foundation-alignment\.test\.mjs/);
  });

  it("rejects objective evidence without goal and task execution drift tests", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.tests = audit.evidence.tests.filter(
      (item) => item !== "scripts/platform-goal-completion-audit.test.mjs" && item !== "scripts/platform-task-execution-audit.test.mjs",
    );
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-goal-completion-audit\.test\.mjs/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-task-execution-audit\.test\.mjs/);
  });

  it("rejects objective evidence without the foundation task graph contract gate", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.contracts = audit.evidence.contracts.filter((item) => item !== "resources/platform-foundation-task-graph.json");
    audit.evidence.validators = audit.evidence.validators.filter((item) => item !== "scripts/validate-platform-foundation-task-graph.mjs");
    audit.evidence.tests = audit.evidence.tests.filter((item) => item !== "scripts/platform-foundation-task-graph.test.mjs");
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "foundation-task-graph");
    const auditPath = tempJSON("platform-objective-conformance.json", audit);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--audit", auditPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conformance evidence must include resources\/platform-foundation-task-graph\.json/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/validate-platform-foundation-task-graph\.mjs/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-foundation-task-graph\.test\.mjs/);
    assert.match(result.stderr, /production readiness preflightCommands must include foundation-task-graph/);
  });

  it("rejects objective evidence without deployment, engineering and production readiness drift tests", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.tests = audit.evidence.tests.filter(
      (item) =>
        item !== "scripts/platform-deployment-topology.test.mjs" &&
        item !== "scripts/platform-engineering-capabilities.test.mjs" &&
        item !== "scripts/platform-production-readiness.test.mjs",
    );
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-deployment-topology\.test\.mjs/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-engineering-capabilities\.test\.mjs/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/platform-production-readiness\.test\.mjs/);
  });

  it("rejects production preflight catalogs that drop actual stack evidence checks", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "engineering-capabilities");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production readiness preflightCommands must include engineering-capabilities/);
  });

  it("rejects objective evidence without the admin UI component contract drift test", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.evidence.tests = audit.evidence.tests.filter((item) => item !== "scripts/admin-ui-contracts.test.mjs");
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/admin-ui-contracts\.test\.mjs/);
  });

  it("rejects requiring neat-freak for every node closeout", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.closeoutPolicy.neatFreakRequiredForEveryNodeCloseout = true;
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /closeoutPolicy\.neatFreakRequiredForEveryNodeCloseout must stay false/);
  });

  it("rejects closeout policies without the node closeout audit contract", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    delete audit.closeoutPolicy.nodeCloseoutAudit;
    audit.evidence.contracts = audit.evidence.contracts.filter((item) => item !== "resources/platform-node-closeout-audit.json");
    audit.evidence.validators = audit.evidence.validators.filter((item) => item !== "scripts/validate-platform-node-closeout-audit.mjs");
    audit.evidence.tests = audit.evidence.tests.filter((item) => item !== "scripts/platform-node-closeout-audit.test.mjs");
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /closeoutPolicy\.nodeCloseoutAudit must be resources\/platform-node-closeout-audit\.json/);
    assert.match(result.stderr, /objective conformance evidence must include scripts\/validate-platform-node-closeout-audit\.mjs/);
  });

  it("rejects missing controlled blockers while completion nodes remain unfinished", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.completionPolicy.controlledBlockers = completionProgramTaskIDs.slice(1);
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completionPolicy\.controlledBlockers must exactly match unfinished task graph nodes in graph order/);
  });

  it("rejects reordering controlled objective blockers", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.completionPolicy.controlledBlockers = [completionProgramTaskIDs[1], completionProgramTaskIDs[0], ...completionProgramTaskIDs.slice(2)];
    const auditPath = tempJSON("platform-objective-conformance.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completionPolicy\.controlledBlockers must exactly match unfinished task graph nodes in graph order/);
  });

  it("rejects making Vercel the required default API runtime", () => {
    const deployment = readJSON("resources/platform-deployment-topology.json");
    deployment.decision.vercelRequired = true;
    deployment.decision.defaultApiRuntime = "vercel-go-runtime";
    const deploymentPath = tempJSON("platform-deployment-topology.json", deployment);

    const result = runValidator(["--deployment-topology", deploymentPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deployment decision\.vercelRequired must stay false/);
    assert.match(result.stderr, /deployment decision\.defaultApiRuntime must stay long-lived-service/);
  });

  it("rejects deployment topology selection drift away from scheme A", () => {
    const audit = readJSON("resources/platform-objective-conformance.json");
    audit.deploymentPolicy.selectedTopology = "split-admin-vercel-api-service";
    const deployment = readJSON("resources/platform-deployment-topology.json");
    deployment.decision.selectedTopology = "split-admin-vercel-api-service";
    const auditPath = tempJSON("platform-objective-conformance.json", audit);
    const deploymentPath = tempJSON("platform-deployment-topology.json", deployment);

    const result = runValidator(["--audit", auditPath, "--deployment-topology", deploymentPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deploymentPolicy\.selectedTopology must stay single-service-production/);
    assert.match(result.stderr, /deployment decision\.selectedTopology must stay single-service-production/);
  });

  it("rejects foundation alignment deployment policy drift away from scheme A", () => {
    const alignment = readJSON("resources/platform-foundation-alignment-audit.json");
    alignment.deploymentPolicy.selectedTopology = "split-admin-vercel-api-service";
    alignment.deploymentPolicy.defaultApiRuntime = "vercel-go-runtime";
    const alignmentPath = tempJSON("platform-foundation-alignment-audit.json", alignment);

    const result = runValidator(["--alignment", alignmentPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /alignment deploymentPolicy\.selectedTopology must stay single-service-production/);
    assert.match(result.stderr, /alignment deploymentPolicy\.defaultApiRuntime must stay long-lived-service/);
  });
});
