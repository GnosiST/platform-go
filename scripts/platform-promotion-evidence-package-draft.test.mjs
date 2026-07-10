import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");
const draftPath = path.join(repoRoot, "resources/generated/platform-promotion-evidence-package-draft.json");

function runGenerator(args = []) {
  return spawnSync(process.execPath, ["scripts/generate-platform-promotion-evidence-package-draft.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function runSubmittedValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-promotion-evidence-package.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

describe("platform promotion evidence package draft generator", () => {
  it("generates a non-submitted draft package file for all controlled promotion gates", () => {
    const result = runGenerator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Generated resources\/generated\/platform-promotion-evidence-package-draft\.json/);
    const draft = readJSON(draftPath);

    assert.equal(draft.generatedBy, "scripts/generate-platform-promotion-evidence-package-draft.mjs");
    assert.equal(draft.mode.approvalState, "draft-submission");
    assert.equal(draft.mode.runtimeMutation, "disabled");
    assert.equal(draft.mode.sourceWriting, "disabled");
    assert.equal(draft.packages.length, 2);
    for (const pkg of draft.packages) {
      assert.equal(pkg.approvalState, "draft-submission");
      assert.equal(pkg.defaultRuntimeMutation, "forbidden");
      assert.ok(Array.isArray(pkg.completionBlockers));
      assert.ok(Array.isArray(pkg.requiredBeforeCompletion));
      assert.ok(Array.isArray(pkg.completedEvidenceSchema.requiredFields));
      assert.ok(pkg.reviewContext);
      assert.ok(pkg.evidence.length > 0);
      assert.ok(pkg.evidence.every((item) => item.status === "draft"));
      assert.ok(pkg.evidence.every((item) => item.description));
      assert.ok(pkg.evidence.every((item) => item.approvedBy === ""));
      assert.ok(pkg.evidence.every((item) => item.artifactURI === ""));
    }
    const sourceWriting = draft.packages.find((pkg) => pkg.packageId === "source-writing-codegen-promotion");
    assert.equal(sourceWriting.reviewContext.reviewArtifact, "resources/generated/admin-scaffold-promotion-review.json");
    assert.ok(sourceWriting.reviewContext.targetFamilies.some((family) => family.id === "api-routes"));
    assert.ok(sourceWriting.reviewContext.preflightCommands.includes("rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs"));
  });

  it("can write a single package draft to stdout", () => {
    const result = runGenerator(["--package-id", "production-auth-promotion", "--stdout"]);

    assert.equal(result.status, 0, result.stderr);
    const draft = JSON.parse(result.stdout);

    assert.equal(draft.packages.length, 1);
    assert.equal(draft.packages[0].packageId, "production-auth-promotion");
    assert.equal(draft.packages[0].taskId, "production-auth-provider-hardening");
    assert.ok(draft.packages[0].evidence.some((item) => item.id === "session-policy-review"));
  });

  it("rejects unknown package ids", () => {
    const result = runGenerator(["--package-id", "missing-package"]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Unknown promotion evidence package missing-package/);
  });

  it("does not let draft packages pass the submitted evidence validator", () => {
    const generate = runGenerator();
    assert.equal(generate.status, 0, generate.stderr);

    const result = runSubmittedValidator(["--package", "resources/generated/platform-promotion-evidence-package-draft.json"]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /approvalState must be submitted/);
    assert.match(result.stderr, /status must be complete/);
  });
});
