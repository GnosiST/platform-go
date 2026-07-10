import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-goal-completion-audit.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-goal-completion-audit-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-goal-completion-audit-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
  return filePath;
}

describe("validate-platform-goal-completion-audit", () => {
  it("accepts the current goal completion audit", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform goal completion audit/);
  });

  it("requires the foundation goal to be complete after production hardening and codegen skeleton closeout", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");

    assert.equal(audit.completionStatus, "complete");
    assert.equal(audit.taskSummary.expectedControlledUnfinished, 0);
    assert.deepEqual(audit.completionPolicy.requiredControlledUnfinishedNodes, []);
  });

  it("rejects reverting the goal to controlled incomplete after all task nodes are implemented", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    audit.completionStatus = "not-complete-controlled";
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completionStatus must be complete when all foundation task nodes are implemented/);
  });

  it("rejects reintroducing controlled unfinished nodes into the completion policy", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    audit.completionPolicy.requiredControlledUnfinishedNodes = ["source-writing-codegen-promotion"];
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completionPolicy\.requiredControlledUnfinishedNodes must be empty after foundation completion/);
  });

  it("rejects business reference wording that turns zshenmez into a migration source", () => {
    const readme = fs
      .readFileSync(path.join(repoRoot, "README.md"), "utf8")
      .replace("not a business migration target", "a business migration target");
    const readmePath = tempText("README.md", readme);

    const result = runValidator(["--readme", readmePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /README\.md must state platform-go is not a business migration target/);
  });

  it("rejects default profile leakage of external business capabilities", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    const defaultProfile = profiles.profiles.find((profile) => profile.default === true);
    defaultProfile.capabilities.push("external-business-capability");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /default profile platform-default must not include external-business-capability/);
  });

  it("rejects production auth promotion requirements that stop being verified foundation gates", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "production-auth-promotion-gate");
    requirement.status = "controlled-blocker";
    requirement.taskId = "production-auth-provider-hardening";
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requirement production-auth-promotion-gate must be verified after foundation completion/);
  });

  it("rejects future promotion gates that drop external artifact URI evidence", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    for (const requirementID of ["production-auth-promotion-gate", "source-writing-codegen-promotion-gate"]) {
      const requirement = audit.requirements.find((item) => item.id === requirementID);
      requirement.requiredBeforeRuntimeMutation = requirement.requiredBeforeRuntimeMutation.filter(
        (item) => item !== "external absolute artifact URI evidence",
      );
    }
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requirement production-auth-promotion-gate\.requiredBeforeRuntimeMutation must include external absolute artifact URI evidence/);
    assert.match(result.stderr, /requirement source-writing-codegen-promotion-gate\.requiredBeforeRuntimeMutation must include external absolute artifact URI evidence/);
  });

  it("rejects completion audits whose promotion evidence gate omits the submitted package validator", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "promotion-evidence-template-gate");
    requirement.evidence.validators = requirement.evidence.validators.filter(
      (item) => item !== "scripts/validate-platform-promotion-evidence-package.mjs",
    );
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotion-evidence-template-gate evidence\.validators must include scripts\/validate-platform-promotion-evidence-package\.mjs/);
  });

  it("rejects quality closeout gates that omit the node closeout audit", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "quality-closeout-gate");
    requirement.evidence.sourcePaths = requirement.evidence.sourcePaths.filter(
      (item) => item !== "resources/platform-node-closeout-audit.json",
    );
    requirement.evidence.validators = requirement.evidence.validators.filter(
      (item) => item !== "scripts/validate-platform-node-closeout-audit.mjs",
    );
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /quality-closeout-gate evidence\.sourcePaths must include resources\/platform-node-closeout-audit\.json/);
    assert.match(result.stderr, /quality-closeout-gate evidence\.validators must include scripts\/validate-platform-node-closeout-audit\.mjs/);
  });

  it("rejects task summary drift from the current task graph", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    audit.taskSummary.expectedImplemented += 1;
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /taskSummary\.expectedImplemented must match implemented task count/);
  });

  it("rejects dropping or weakening the deployment topology completion requirement", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    audit.requirements = audit.requirements.filter((requirement) => requirement.id !== "deployment-topology-runtime-boundary");
    const deploymentTopology = readJSON("resources/platform-deployment-topology.json");
    deploymentTopology.decision.vercelRequired = true;
    deploymentTopology.decision.defaultApiRuntime = "vercel-go-runtime";
    deploymentTopology.decision.selectedTopology = "split-admin-vercel-api-service";
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);
    const deploymentTopologyPath = tempJSON("platform-deployment-topology.json", deploymentTopology);

    const result = runValidator(["--audit", auditPath, "--deployment-topology", deploymentTopologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required completion requirement deployment-topology-runtime-boundary/);
    assert.match(result.stderr, /deployment topology decision\.vercelRequired must stay false/);
    assert.match(result.stderr, /deployment topology decision\.defaultApiRuntime must stay long-lived-service/);
    assert.match(result.stderr, /deployment topology decision\.selectedTopology must stay single-service-production/);
  });

  it("rejects dropping deployment package evidence from the completion audit", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "deployment-topology-runtime-boundary");
    requirement.evidence.sourcePaths = requirement.evidence.sourcePaths.filter(
      (item) => item !== "Dockerfile" && item !== "deploy/compose/docker-compose.prod.yml",
    );
    const deploymentTopology = readJSON("resources/platform-deployment-topology.json");
    deploymentTopology.deploymentPackage.status = "missing";
    deploymentTopology.deploymentPackage.selectedTopology = "split-admin-vercel-api-service";
    deploymentTopology.deploymentPackage.dockerTargets.api = "vercel-go-runtime";
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);
    const deploymentTopologyPath = tempJSON("platform-deployment-topology.json", deploymentTopology);

    const result = runValidator(["--audit", auditPath, "--deployment-topology", deploymentTopologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deployment-topology-runtime-boundary evidence\.sourcePaths must include Dockerfile/);
    assert.match(result.stderr, /deployment-topology-runtime-boundary evidence\.sourcePaths must include deploy\/compose\/docker-compose\.prod\.yml/);
    assert.match(result.stderr, /deployment package must stay implemented/);
    assert.match(result.stderr, /deployment package selectedTopology must stay single-service-production/);
    assert.match(result.stderr, /deployment package dockerTargets\.api must stay api/);
  });

  it("rejects deployment completion requirements without production and engineering drift tests", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "deployment-topology-runtime-boundary");
    requirement.evidence.tests = requirement.evidence.tests.filter(
      (item) =>
        item !== "scripts/platform-production-readiness.test.mjs" &&
        item !== "scripts/platform-engineering-capabilities.test.mjs",
    );
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deployment-topology-runtime-boundary evidence\.tests must include scripts\/platform-production-readiness\.test\.mjs/);
    assert.match(result.stderr, /deployment-topology-runtime-boundary evidence\.tests must include scripts\/platform-engineering-capabilities\.test\.mjs/);
  });

  it("rejects admin UI design gate requirements without the component contract drift test", () => {
    const audit = readJSON("resources/platform-goal-completion-audit.json");
    const requirement = audit.requirements.find((item) => item.id === "admin-ui-i18n-design-gates");
    requirement.evidence.tests = [];
    const auditPath = tempJSON("platform-goal-completion-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-ui-i18n-design-gates evidence\.tests must include scripts\/admin-ui-contracts\.test\.mjs/);
  });

  it("rejects engineering matrices that do not include the goal completion audit capability", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "goal-completion-audit");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--engineering", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /engineering capability matrix must include goal-completion-audit/);
  });
});
