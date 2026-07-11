import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runScript(script, args = []) {
  return spawnSync(process.execPath, [script, ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-operations-plan-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("platform operations dry-run plan", () => {
  it("generates a non-mutating operations plan from production readiness policies", () => {
    const result = runScript("scripts/generate-platform-operations-plan.mjs", ["--stdout"]);

    assert.equal(result.status, 0, result.stderr);
    const plan = JSON.parse(result.stdout);
    assert.equal(plan.generatedBy, "scripts/generate-platform-operations-plan.mjs");
    assert.equal(plan.mode.dryRun, true);
    assert.equal(plan.mode.runtimeMutation, "disabled");
    assert.equal(plan.mode.sourceWriting, "disabled");
    assert.equal(plan.summary.policyCount, 4);
    assert.equal(plan.summary.providerPromotionCount, 3);
    assert.equal(plan.summary.optionalProductionProviderCount, 2);
    assert.equal(plan.preflightRunner.script, "scripts/run-platform-production-preflight.mjs");
    assert.equal(plan.preflightRunner.dryRunCommand, "rtk node scripts/run-platform-production-preflight.mjs");
    assert.match(plan.preflightRunner.policyCommand, /--policy <policy-id>/);
    assert.match(plan.preflightRunner.strictEnvCommand, /--strict-env-file <private-production-env>/);
    assert.ok(plan.preflightCommands.some((command) => command.id === "admin-ui-contract-tests"));
    assert.ok(plan.productionPromotionApprovalPackage, "missing productionPromotionApprovalPackage");
    assert.equal(plan.productionPromotionApprovalPackage.status, "blocked");
    assert.equal(plan.productionPromotionApprovalPackage.defaultRuntimeMutation, "forbidden");
    assert.ok(plan.productionPromotionApprovalPackage.requiredApprovals.includes("security-owner"));
    assert.ok(plan.productionPromotionApprovalPackage.requiredEvidence.some((item) => item.id === "runtime-test-output"));
    assert.deepEqual(plan.productionPromotionApprovalPackage.completedEvidence, []);
    assert.ok(plan.productionPromotionApprovalPackage.completedEvidenceSchema.requiredFields.includes("rollbackCommands"));
    assert.ok(plan.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules.includes("refresh-token-family-tests-required-before-runtime-mutation"));
    assert.ok(plan.productionPromotionApprovalPackage.completedEvidenceSchema.forbiddenFields.includes("tokenHash"));
    assert.equal(plan.providerPromotionMatrix.source, "resources/platform-production-auth-hardening.json");
    assert.ok(plan.providerPromotionMatrix.newProviderRequirements.includes("audit-redaction-test"));
    const demoProvider = plan.providerPromotionMatrix.providers.find((provider) => provider.id === "demo");
    const wechatProvider = plan.providerPromotionMatrix.providers.find((provider) => provider.id === "wechat");
    const oidcProvider = plan.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    assert.equal(demoProvider.productionUsage, "local-harness-only");
    assert.equal(wechatProvider.productionUsage, "optional-production-provider");
    assert.equal(wechatProvider.rawCredentialExposureAllowed, false);
    assert.equal(wechatProvider.rawSubjectExposureAllowed, false);
    assert.ok(wechatProvider.configKeys.includes("PLATFORM_WECHAT_MINIAPP_SECRET"));
    assert.equal(oidcProvider.capability, "admin-oidc");
    assert.equal(oidcProvider.kind, "oidc");
    assert.deepEqual(oidcProvider.audiences, ["admin"]);
    assert.equal(oidcProvider.productionLikeRehearsalRequired, true);
    for (const policy of ["config-backup-export", "config-import-restore", "database-migration", "token-rotation"]) {
      const item = plan.policies.find((candidate) => candidate.id === policy);
      assert.ok(item, `missing ${policy}`);
      assert.ok(item.requiredPreflightCommands.length > 0, `${policy} must expose required preflight commands`);
      assert.deepEqual(item.missingRequiredPreflightCommands, []);
    }
  });

  it("rejects operations plans that enable runtime mutation", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    plan.mode.runtimeMutation = "enabled";
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform operations plan must keep runtimeMutation disabled/);
  });

  it("rejects operations plans that drop provider promotion matrix evidence", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    delete plan.providerPromotionMatrix;
    plan.summary.providerPromotionCount = 0;
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform operations plan providerPromotionMatrix.source must point to resources\/platform-production-auth-hardening\.json/);
    assert.match(result.stderr, /platform operations plan summary.providerPromotionCount must be 3/);
  });

  it("rejects operations plans that drop production auth approval package evidence", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    delete plan.productionPromotionApprovalPackage;
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform operations plan productionPromotionApprovalPackage.status must match production auth hardening contract/);
    assert.match(result.stderr, /platform operations plan productionPromotionApprovalPackage.requiredEvidence must match production auth hardening contract/);
  });

  it("rejects operations plans that drop production auth completed evidence schema", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    plan.productionPromotionApprovalPackage.completedEvidenceSchema.requiredFields = [];
    plan.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules = [];
    plan.productionPromotionApprovalPackage.completedEvidenceSchema.forbiddenFields = [];
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(
      result.stderr,
      /platform operations plan productionPromotionApprovalPackage\.completedEvidenceSchema\.requiredFields must match production auth hardening contract/,
    );
    assert.match(
      result.stderr,
      /platform operations plan productionPromotionApprovalPackage\.completedEvidenceSchema\.approvalRules must match production auth hardening contract/,
    );
    assert.match(
      result.stderr,
      /platform operations plan productionPromotionApprovalPackage\.completedEvidenceSchema\.forbiddenFields must match production auth hardening contract/,
    );
  });

  it("rejects operations plans that weaken provider promotion controls", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    const wechat = plan.providerPromotionMatrix.providers.find((provider) => provider.id === "wechat");
    wechat.configKeys = ["PLATFORM_WECHAT_MINIAPP_APP_ID"];
    wechat.rawSubjectExposureAllowed = true;
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform operations plan provider wechat configKeys must match production auth hardening contract/);
    assert.match(result.stderr, /platform operations plan provider wechat rawSubjectExposureAllowed must stay false/);
  });

  it("rejects operations plans that weaken typed OIDC projection invariants", () => {
    const plan = readJSON("resources/generated/platform-operations-plan.json");
    const oidc = plan.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    oidc.audiences = ["app"];
    oidc.productionLikeRehearsalRequired = false;
    const planPath = tempJSON("platform-operations-plan.json", plan);

    const result = runScript("scripts/validate-platform-production-readiness.mjs", ["--operations-plan", planPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform operations plan provider oidc audiences must match production auth hardening contract/);
    assert.match(result.stderr, /platform operations plan provider oidc productionLikeRehearsalRequired must match production auth hardening contract/);
  });
});
