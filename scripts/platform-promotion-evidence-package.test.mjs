import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");
const reviewedCommit = "0123456789abcdef0123456789abcdef01234567";

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-promotion-evidence-package.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function sourceWritingCoverage() {
  const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
  const families = readiness.targetFamilies ?? [];
  return {
    targetFamilies: families.map((family) => family.id),
    runtimeTargets: Array.from(new Set(families.flatMap((family) => family.runtimeTargets ?? []))),
    verificationCommands: Array.from(new Set(families.flatMap((family) => family.testCommands ?? []))),
  };
}

function productionAuthCoverage() {
  const contract = readJSON("resources/platform-production-auth-hardening.json");
  const providers = contract.providerPromotionMatrix?.providers ?? [];
  return {
    providerIds: providers.map((provider) => provider.id),
    rotationProviderIds: providers.filter((provider) => provider.rotationRunbookRequired === true).map((provider) => provider.id),
    providerControls: Array.from(new Set(providers.flatMap((provider) => provider.requiredControls ?? []))).sort(),
    runtimeTestRefs: contract.providerRuntimePolicy?.requiredTests ?? [],
  };
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-promotion-evidence-package-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function completeEvidence(template, packageId) {
  const completed = {
    ...template,
    status: "complete",
    artifactURI: `s3://platform-review/${packageId}/${template.id}.json`,
    artifactHash: `sha256:${"a".repeat(64)}`,
    approvedBy: `${template.owner}-reviewer`,
    approvedAt: "2026-07-08T00:00:00Z",
    reviewedCommit,
    verificationCommands: ["rtk node scripts/validate-platform-production-readiness.mjs"],
    rollbackCommands: ["rtk node scripts/run-platform-production-preflight.mjs --list"],
  };
  if ("environment" in completed) {
    completed.environment = "production";
  }
  if ("auditSampleRefs" in completed) {
    completed.auditSampleRefs = ["s3://platform-review/audit/redacted-sample.json"];
  }
  if ("providerRotationRunbookRefs" in completed) {
    completed.providerRotationRunbookRefs = ["s3://platform-review/runbooks/provider-rotation.md"];
  }
  if ("refreshTokenFamilyTestRefs" in completed) {
    completed.refreshTokenFamilyTestRefs = ["s3://platform-review/tests/refresh-token-family.txt"];
  }
  if (packageId === "production-auth-promotion") {
    completed.providerIds = productionAuthCoverage().providerIds;
    completed.providerControls = productionAuthCoverage().providerControls;
    completed.runtimeTestRefs = productionAuthCoverage().runtimeTestRefs;
  }
  if ("targetFamilies" in completed) {
    completed.targetFamilies =
      packageId === "source-writing-codegen-promotion"
        ? sourceWritingCoverage().targetFamilies
        : ["backend-models", "api-routes", "admin-resource-pages"];
  }
  if ("runtimeTargets" in completed) {
    completed.runtimeTargets =
      packageId === "source-writing-codegen-promotion"
        ? sourceWritingCoverage().runtimeTargets
        : ["internal/platform/httpapi/", "admin/src/platform/resources/"];
  }
  if (packageId === "source-writing-codegen-promotion" && template.id === "target-family-test-run") {
    completed.verificationCommands = sourceWritingCoverage().verificationCommands;
  }
  return completed;
}

function packageFromTemplate(packageId) {
  const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
  const template = templates.packages.find((item) => item.id === packageId);
  assert.ok(template, `missing template package ${packageId}`);
  return {
    packageId: template.id,
    taskId: template.taskId,
    source: template.source,
    approvalState: "submitted",
    defaultRuntimeMutation: "forbidden",
    reviewedCommit,
    evidence: template.evidenceTemplates.map((item) => completeEvidence(item, template.id)),
  };
}

describe("platform promotion evidence package validation", () => {
  it("accepts completed evidence packages for the controlled promotion gates without changing runtime state", () => {
    const packagePath = tempJSON("promotion-evidence-package.json", {
      packages: [packageFromTemplate("production-auth-promotion"), packageFromTemplate("source-writing-codegen-promotion")],
    });

    const result = runValidator(["--package", packagePath]);

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated 2 platform promotion evidence packages/);
  });

  it("rejects duplicate package ids in a submitted bundle", () => {
    const packagePath = tempJSON("promotion-evidence-package.json", {
      packages: [packageFromTemplate("production-auth-promotion"), packageFromTemplate("production-auth-promotion")],
    });

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotion evidence packages contains duplicate packageId production-auth-promotion/);
  });

  it("rejects empty submitted package bundles", () => {
    const packagePath = tempJSON("promotion-evidence-package.json", { packages: [] });

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotion evidence package bundle must include at least one package/);
  });

  it("rejects missing required evidence and self approval", () => {
    const pkg = packageFromTemplate("production-auth-promotion");
    pkg.evidence = pkg.evidence.filter((item) => item.id !== "runtime-test-output");
    pkg.evidence[0].approvedBy = pkg.evidence[0].owner;
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-promotion\.evidence must include runtime-test-output/);
    assert.match(result.stderr, /session-policy-review\.approvedBy must not equal owner/);
  });

  it("rejects duplicate evidence ids in a submitted package", () => {
    const pkg = packageFromTemplate("production-auth-promotion");
    pkg.evidence.push({ ...pkg.evidence[0] });
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-promotion\.evidence contains duplicate id session-policy-review/);
  });

  it("rejects non-rtk verification commands and missing rollback commands", () => {
    const pkg = packageFromTemplate("source-writing-codegen-promotion");
    pkg.evidence[0].verificationCommands = ["npm test"];
    pkg.evidence[1].rollbackCommands = [];
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /explicit-source-writing-spec\.verificationCommands must use rtk commands/);
    assert.match(result.stderr, /approved-promotion-review-packet\.rollbackCommands must not be empty/);
  });

  it("rejects submitted evidence with weak artifact hashes", () => {
    const pkg = packageFromTemplate("source-writing-codegen-promotion");
    pkg.evidence[0].artifactHash = "sha256:abc";
    pkg.evidence[1].artifactHash = "md5:0123456789abcdef0123456789abcdef";
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /explicit-source-writing-spec\.artifactHash must be sha256: followed by 64 lowercase hex characters/);
    assert.match(result.stderr, /approved-promotion-review-packet\.artifactHash must be sha256: followed by 64 lowercase hex characters/);
  });

  it("rejects evidence reviewed against a different commit than the package", () => {
    const pkg = packageFromTemplate("source-writing-codegen-promotion");
    pkg.evidence[0].reviewedCommit = "fedcba9876543210fedcba9876543210fedcba98";
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /source-writing-codegen-promotion\.explicit-source-writing-spec\.reviewedCommit must match package reviewedCommit/);
  });

  it("rejects submitted evidence with local or private artifact URIs", () => {
    const pkg = packageFromTemplate("source-writing-codegen-promotion");
    pkg.evidence[0].artifactURI = "file:///tmp/source-writing-spec.json";
    pkg.evidence[1].artifactURI = "docs/local-review.md";
    pkg.evidence[2].artifactURI = "http://localhost:8080/review.json";
    pkg.evidence[3].artifactURI = "https://192.168.1.20/review.json";
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /explicit-source-writing-spec\.artifactURI must be an external absolute review artifact URI using https, s3, or gs/);
    assert.match(result.stderr, /approved-promotion-review-packet\.artifactURI must be an external absolute review artifact URI using https, s3, or gs/);
    assert.match(result.stderr, /diff-review\.artifactURI must be an external absolute review artifact URI using https, s3, or gs/);
    assert.match(result.stderr, /rollback-plan\.artifactURI must be an external absolute review artifact URI using https, s3, or gs/);
  });

  it("rejects source-writing submissions without full target family, runtime target and test-command coverage", () => {
    const pkg = packageFromTemplate("source-writing-codegen-promotion");
    for (const item of pkg.evidence) {
      item.targetFamilies = item.targetFamilies.filter((family) => family !== "repositories");
      item.runtimeTargets = item.runtimeTargets.filter((target) => target !== "internal/platform/repository/");
    }
    const testRun = pkg.evidence.find((item) => item.id === "target-family-test-run");
    testRun.verificationCommands = testRun.verificationCommands.filter((command) => command !== "rtk go test ./internal/platform/adminresource");
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /source-writing-codegen-promotion targetFamilies must include repositories/);
    assert.match(result.stderr, /source-writing-codegen-promotion runtimeTargets must include internal\/platform\/repository\//);
    assert.match(result.stderr, /target-family-test-run\.verificationCommands must include rtk go test \.\/internal\/platform\/adminresource/);
  });

  it("rejects production auth submissions without provider, provider-control and runtime-test coverage", () => {
    const pkg = packageFromTemplate("production-auth-promotion");
    for (const item of pkg.evidence) {
      item.providerIds = item.providerIds.filter((provider) => provider !== "wechat");
      item.providerControls = item.providerControls.filter((control) => control !== "secret-rotation-plan");
    }
    const runtimeTest = pkg.evidence.find((item) => item.id === "runtime-test-output");
    runtimeTest.runtimeTestRefs = runtimeTest.runtimeTestRefs.filter((testRef) => testRef !== "provider-error-normalization");
    const providerRunbook = pkg.evidence.find((item) => item.id === "provider-secret-rotation-runbook");
    providerRunbook.providerRotationRunbookRefs = [];
    const auditSample = pkg.evidence.find((item) => item.id === "audit-redaction-sample");
    auditSample.auditSampleRefs = [];
    const refreshSpec = pkg.evidence.find((item) => item.id === "refresh-token-family-runtime-spec");
    refreshSpec.refreshTokenFamilyTestRefs = [];
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-promotion providerIds must include wechat/);
    assert.match(result.stderr, /production-auth-promotion providerControls must include secret-rotation-plan/);
    assert.match(result.stderr, /runtime-test-output\.runtimeTestRefs must include provider-error-normalization/);
    assert.match(result.stderr, /provider-secret-rotation-runbook\.providerRotationRunbookRefs must not be empty/);
    assert.match(result.stderr, /audit-redaction-sample\.auditSampleRefs must not be empty/);
    assert.match(result.stderr, /refresh-token-family-runtime-spec\.refreshTokenFamilyTestRefs must not be empty/);
  });

  it("rejects forbidden sensitive fields inside submitted evidence", () => {
    const pkg = packageFromTemplate("production-auth-promotion");
    pkg.evidence[0].jwt = "do-not-submit";
    pkg.evidence[1].nested = {
      refreshToken: "do-not-submit",
    };
    const packagePath = tempJSON("promotion-evidence-package.json", pkg);

    const result = runValidator(["--package", packagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /session-policy-review must not include forbidden field jwt/);
    assert.match(result.stderr, /refresh-token-family-runtime-spec must not include forbidden field refreshToken/);
  });
});
