import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-task-execution-audit.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-task-execution-audit-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-task-execution-audit", () => {
  it("accepts the current task execution audit", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform task execution audit/);
  });

  it("keeps zero unfinished nodes after production hardening and codegen skeleton closeout", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");

    assert.deepEqual(audit.requiredUnfinishedNodes, []);
  });

  it("rejects execution audits that omit the admin API boundary validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-admin-api-boundary.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-admin-api-boundary\.mjs/);
  });

  it("rejects reintroducing completed foundation nodes as unfinished", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredUnfinishedNodes = ["source-writing-codegen-promotion"];
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredUnfinishedNodes must be empty after foundation completion/);
  });

  it("rejects future promotion gates without status reason or completion gate", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "production-auth-provider-hardening");
    delete task.statusReason;
    delete task.completionGate;
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-provider-hardening must declare zh\/en statusReason/);
    assert.match(result.stderr, /production-auth-provider-hardening must declare zh\/en completionGate/);
  });

  it("rejects visual tasks that bypass product design gates", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "form-schema-layout-and-slots");
    task.designGate = ["superpowers:brainstorming"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visual task form-schema-layout-and-slots must require superpowers:brainstorming and product-design/);
  });

  it("rejects production auth blockers without structured approval evidence", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    const blocker = audit.knownPromotionBlockers.find((item) => item.taskId === "production-auth-provider-hardening");
    blocker.runtimeMutationBlockedWhile = blocker.runtimeMutationBlockedWhile.filter(
      (item) =>
        item !== "resources/platform-production-auth-hardening.json sessionCredentialPolicy.refreshTokenFamily.defaultRuntime is disabled" &&
        item !== "resources/platform-production-auth-hardening.json productionPromotionApprovalPackage.status is blocked",
    );
    blocker.requiredEvidenceBeforePromotion = blocker.requiredEvidenceBeforePromotion.filter(
      (item) =>
        item !== "runtime test output bundle including refresh-token-family tests" &&
        item !== "redis session invalidation convergence evidence" &&
        item !== "security-owner approval" &&
        item !== "structured approval evidence package" &&
        item !== "provider rotation runbook" &&
        item !== "rollback plan",
    );
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.runtimeMutationBlockedWhile must include resources\/platform-production-auth-hardening\.json sessionCredentialPolicy\.refreshTokenFamily\.defaultRuntime is disabled/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.runtimeMutationBlockedWhile must include resources\/platform-production-auth-hardening\.json productionPromotionApprovalPackage\.status is blocked/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include runtime test output bundle including refresh-token-family tests/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include redis session invalidation convergence evidence/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include security-owner approval/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include structured approval evidence package/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include provider rotation runbook/);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include rollback plan/);
  });

  it("allows production auth hardening implemented while refresh-token family default runtime stays disabled", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "production-auth-provider-hardening");
    task.status = "implemented";
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const productionAuth = readJSON("resources/platform-production-auth-hardening.json");
    const productionAuthPath = tempJSON("platform-production-auth-hardening.json", productionAuth);

    const result = runValidator(["--task-graph", graphPath, "--production-auth", productionAuthPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects source-writing blockers without structured approval evidence", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    const blocker = audit.knownPromotionBlockers.find((item) => item.taskId === "source-writing-codegen-promotion");
    blocker.runtimeMutationBlockedWhile = blocker.runtimeMutationBlockedWhile.filter(
      (item) =>
        item !== "resources/platform-codegen-source-writing-readiness.json mode.sourceWriting is disabled" &&
        item !== "resources/platform-codegen-source-writing-readiness.json sourceWritingApprovalPackage.status is blocked",
    );
    blocker.requiredEvidenceBeforePromotion = blocker.requiredEvidenceBeforePromotion.filter(
      (item) =>
        item !== "source-writing architecture spec" &&
        item !== "platform-architect approval" &&
        item !== "approved promotion review packet" &&
        item !== "reviewed diff" &&
        item !== "target-family test mapping" &&
        item !== "rollback plan" &&
        item !== "structured source-writing approval evidence package",
    );
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.runtimeMutationBlockedWhile must include resources\/platform-codegen-source-writing-readiness\.json mode\.sourceWriting is disabled/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.runtimeMutationBlockedWhile must include resources\/platform-codegen-source-writing-readiness\.json sourceWritingApprovalPackage\.status is blocked/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include source-writing architecture spec/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include platform-architect approval/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include approved promotion review packet/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include reviewed diff/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include target-family test mapping/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include rollback plan/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include structured source-writing approval evidence package/);
  });

  it("rejects parallel batches with resource lock conflicts", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "non-visual-contract-gates");
    batch.taskIds = ["resource-schema-contract", "admin-api-boundary-query-security"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch non-visual-contract-gates has resource lock conflict admin-resource-api/);
  });

  it("rejects task graphs without resource lock policies", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.resourceLockPolicies = graph.resourceLockPolicies.filter((policy) => policy.lock !== "production-config");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resourceLockPolicies must describe production-config/);
  });

  it("rejects parallel batches with resource lock group conflicts", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "non-visual-contract-gates");
    batch.taskIds = ["openapi-app-contracts", "cache-redis-invalidation"];
    const cacheTask = graph.tasks.find((task) => task.id === "cache-redis-invalidation");
    cacheTask.resourceLocks = ["codegen"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch non-visual-contract-gates has resource lock group conflict contract-generation-surface/);
  });

  it("rejects engineering matrices that do not cite the execution audit validator", () => {
    const engineering = readJSON("resources/platform-engineering-capabilities.json");
    const capability = engineering.capabilities.find((item) => item.id === "task-dependency-governance");
    capability.evidence.validators = capability.evidence.validators.filter((item) => item !== "scripts/validate-platform-task-execution-audit.mjs");
    const engineeringPath = tempJSON("platform-engineering-capabilities.json", engineering);

    const result = runValidator(["--engineering", engineeringPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task-dependency-governance must cite validator scripts\/validate-platform-task-execution-audit\.mjs/);
  });

  it("rejects execution audits that omit the file storage experience validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-file-storage-experience.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-file-storage-experience\.mjs/);
  });

  it("rejects execution audits that omit the refresh-token family promotion validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-refresh-token-family-promotion.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-refresh-token-family-promotion\.mjs/);
  });

  it("rejects execution audits that omit the goal completion audit validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-goal-completion-audit.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-goal-completion-audit\.mjs/);
  });

  it("rejects execution audits that omit the node closeout audit validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-node-closeout-audit.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-node-closeout-audit\.mjs/);
  });

  it("rejects execution audits that omit the promotion evidence template validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-promotion-evidence-templates.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-promotion-evidence-templates\.mjs/);
  });

  it("rejects execution audits that omit the submitted promotion evidence package validator", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-promotion-evidence-package.mjs");
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-promotion-evidence-package\.mjs/);
  });

  it("rejects promotion blockers that do not require external artifact URI evidence", () => {
    const audit = readJSON("resources/platform-task-execution-audit.json");
    for (const blocker of audit.knownPromotionBlockers) {
      blocker.requiredEvidenceBeforePromotion = blocker.requiredEvidenceBeforePromotion.filter(
        (item) => item !== "external absolute artifact URI evidence",
      );
    }
    const auditPath = tempJSON("platform-task-execution-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /knownPromotionBlockers\.production-auth-provider-hardening\.requiredEvidenceBeforePromotion must include external absolute artifact URI evidence/);
    assert.match(result.stderr, /knownPromotionBlockers\.source-writing-codegen-promotion\.requiredEvidenceBeforePromotion must include external absolute artifact URI evidence/);
  });
});
