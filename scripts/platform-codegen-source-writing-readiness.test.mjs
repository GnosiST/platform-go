import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-codegen-source-writing-readiness.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-codegen-readiness-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

describe("validate-platform-codegen-source-writing-readiness", () => {
  it("accepts the current source-writing readiness contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated codegen source-writing readiness/);
  });

  it("rejects enabled source writing", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.mode.sourceWriting = "enabled";
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /mode\.sourceWriting disabled/);
  });

  it("rejects missing human review and diff review gates", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.mode.requiresHumanReview = false;
    readiness.mode.requiresDiffReview = false;
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiresHumanReview=true/);
    assert.match(result.stderr, /requiresDiffReview=true/);
  });

  it("rejects source-writing approval packages that bypass external evidence", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.sourceWritingApprovalPackage.status = "ready";
    readiness.sourceWritingApprovalPackage.defaultRuntimeMutation = "enabled";
    readiness.sourceWritingApprovalPackage.completedEvidence = ["approved-promotion-review-packet"];
    readiness.sourceWritingApprovalPackage.requiredApprovals = ["codegen-owner"];
    readiness.sourceWritingApprovalPackage.requiredEvidence = readiness.sourceWritingApprovalPackage.requiredEvidence.filter(
      (item) => item.id !== "rollback-plan",
    );
    readiness.sourceWritingApprovalPackage.prohibitedEvidence = readiness.sourceWritingApprovalPackage.prohibitedEvidence.filter(
      (item) => item !== "text-only approval",
    );
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.status must stay blocked/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.defaultRuntimeMutation must stay forbidden/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.completedEvidence must stay empty before promotion/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.requiredApprovals must include platform-architect/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.requiredApprovals must include operations-owner/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.requiredEvidence must include rollback-plan/);
    assert.match(result.stderr, /sourceWritingApprovalPackage\.prohibitedEvidence must include text-only approval/);
  });

  it("rejects source-writing approval packages without a completed evidence artifact schema", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    delete readiness.sourceWritingApprovalPackage.completedEvidenceSchema;
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include artifactURI/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include rollbackCommands/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include reviewed-diff-required-per-runtime-target/);
    assert.match(result.stderr, /completedEvidenceSchema\.forbiddenFields must include privateKey/);
  });

  it("rejects source-writing approval packages without a strong artifact hash policy", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    delete readiness.sourceWritingApprovalPackage.completedEvidenceSchema.artifactHashPolicy;
    readiness.sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules =
      readiness.sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules.filter(
        (rule) => rule !== "artifact-hash-must-be-sha256-hex",
      );
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.algorithm must be sha256/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.format must be prefix-hex/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.hexLength must be 64/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include artifact-hash-must-be-sha256-hex/);
  });

  it("rejects source-writing approval packages without an external artifact URI policy", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    delete readiness.sourceWritingApprovalPackage.completedEvidenceSchema.artifactURIPolicy;
    readiness.sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules =
      readiness.sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules.filter(
        (rule) => rule !== "artifact-uri-must-be-external-review-artifact",
      );
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.sourceOfTruth must be external-review-artifacts/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.allowedSchemes must include https/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.allowedSchemes must include s3/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.forbidLocalhost must be true/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include artifact-uri-must-be-external-review-artifact/);
  });

  it("rejects unsafe allowed runtime targets", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.allowedRuntimeTargets.push("../outside");
    readiness.allowedRuntimeTargets.push("resources/generated/");
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /allowed runtime target is unsafe/);
    assert.match(result.stderr, /allowed runtime target cannot be generated/);
  });

  it("rejects scaffold plans that enable source writing", () => {
    const scaffoldPlan = readJSON("resources/generated/admin-scaffold-plan.json");
    scaffoldPlan.mode.sourceWriting = "enabled";
    const scaffoldPlanPath = tempJSON("admin-scaffold-plan.json", scaffoldPlan);

    const result = runValidator(["--scaffold-plan", scaffoldPlanPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin scaffold plan must keep sourceWriting disabled/);
  });

  it("rejects readiness contracts without target family test mappings", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.targetFamilies = [];
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /targetFamilies must not be empty/);
  });

  it("rejects target family mappings that reference unknown scaffold roles", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.targetFamilies[0].scaffoldRoles.push("unknownDraft");
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /target family backend-models references unknown scaffold role unknownDraft/);
  });

  it("rejects target families whose runtime roots are not declared in the runtime target policy", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.runtimeTargetPolicy.roots = readiness.runtimeTargetPolicy.roots.filter((root) => root.path !== "internal/platform/model/");
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /target family backend-models runtime target internal\/platform\/model\/ must be declared in runtimeTargetPolicy\.roots/);
  });

  it("rejects readiness contracts without an explicit runtime target policy", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    delete readiness.runtimeTargetPolicy;
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeTargetPolicy\.mode must be explicit-root-registry/);
  });

  it("rejects promotion review packets without runtime target policy evidence", () => {
    const review = readJSON("resources/generated/admin-scaffold-promotion-review.json");
    delete review.runtimeTargetPolicy;
    const reviewPath = tempJSON("admin-scaffold-promotion-review.json", review);

    const result = runValidator(["--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin scaffold promotion review must include runtimeTargetPolicy/);
  });

  it("rejects promotion review packets without completed evidence schema", () => {
    const review = readJSON("resources/generated/admin-scaffold-promotion-review.json");
    delete review.sourceWritingApprovalPackage.completedEvidenceSchema;
    const reviewPath = tempJSON("admin-scaffold-promotion-review.json", review);

    const result = runValidator(["--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-scaffold-promotion-review\.json is stale/);
    assert.match(result.stderr, /promotion review sourceWritingApprovalPackage\.completedEvidenceSchema\.requiredFields must include artifactURI/);
    assert.match(result.stderr, /promotion review sourceWritingApprovalPackage\.completedEvidenceSchema\.approvalRules must include reviewed-diff-required-per-runtime-target/);
  });

  it("rejects promotion review packets that bypass manual review", () => {
    const review = readJSON("resources/generated/admin-scaffold-promotion-review.json");
    review.mode.runtimeMutation = "enabled";
    review.mode.promotion = "approved";
    review.manualReview.decision = "approved";
    const reviewPath = tempJSON("admin-scaffold-promotion-review.json", review);

    const result = runValidator(["--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-scaffold-promotion-review\.json is stale/);
    assert.match(result.stderr, /generate-admin-scaffold-promotion-review\.mjs/);
    assert.match(result.stderr, /runtimeMutation disabled/);
    assert.match(result.stderr, /manual review before promotion/);
    assert.match(result.stderr, /must remain not-approved/);
  });

  it("rejects missing promotion review packets", () => {
    const missingReviewPath = path.join(os.tmpdir(), "missing-admin-scaffold-promotion-review.json");

    const result = runValidator(["--promotion-review", missingReviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing-admin-scaffold-promotion-review\.json is missing/);
    assert.match(result.stderr, /admin scaffold promotion review must keep sourceWriting disabled/);
  });
});
