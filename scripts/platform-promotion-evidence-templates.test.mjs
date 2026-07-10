import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-promotion-evidence-templates.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function runGenerator(args = []) {
  return spawnSync(process.execPath, ["scripts/generate-platform-promotion-evidence-templates.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-promotion-evidence-templates-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("platform promotion evidence templates", () => {
  it("generates and validates draft-only promotion evidence templates", () => {
    const generate = runGenerator();
    assert.equal(generate.status, 0, generate.stderr);
    assert.match(generate.stdout, /Generated resources\/generated\/platform-promotion-evidence-templates\.json/);

    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated 2 platform promotion evidence template packages/);

    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    assert.equal(templates.submittedEvidenceValidator.script, "scripts/validate-platform-promotion-evidence-package.mjs");
    assert.match(templates.submittedEvidenceValidator.command, /--package <promotion-evidence-package>/);
    assert.ok(templates.submittedEvidenceValidator.tests.includes("scripts/platform-promotion-evidence-package.test.mjs"));
    assert.equal(templates.draftPackageGenerator.script, "scripts/generate-platform-promotion-evidence-package-draft.mjs");
    assert.match(templates.draftPackageGenerator.command, /generate-platform-promotion-evidence-package-draft\.mjs/);
    assert.ok(templates.draftPackageGenerator.generatedFiles.includes("resources/generated/platform-promotion-evidence-package-draft.json"));
    assert.ok(templates.draftPackageGenerator.tests.includes("scripts/platform-promotion-evidence-package-draft.test.mjs"));
    const sourceWriting = templates.packages.find((item) => item.id === "source-writing-codegen-promotion");
    assert.equal(sourceWriting.reviewContext.reviewArtifact, "resources/generated/admin-scaffold-promotion-review.json");
    assert.equal(sourceWriting.reviewContext.sourceWriting, "disabled");
    assert.equal(sourceWriting.reviewContext.runtimeMutation, "disabled");
    assert.ok(sourceWriting.reviewContext.targetFamilies.some((family) => family.id === "api-routes"));
    assert.ok(sourceWriting.reviewContext.preflightCommands.includes("rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs"));
  });

  it("rejects marking templates submitted or runtime-mutating", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    templates.mode.approvalState = "approved";
    templates.mode.runtimeMutation = "enabled";
    templates.packages[0].status = "approved";
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /mode\.approvalState must stay not-submitted/);
    assert.match(result.stderr, /mode\.runtimeMutation must stay disabled/);
    assert.match(result.stderr, /production-auth-promotion\.status must stay draft-template/);
  });

  it("rejects missing required evidence templates", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    const pkg = templates.packages.find((item) => item.id === "source-writing-codegen-promotion");
    pkg.evidenceTemplates = pkg.evidenceTemplates.filter((template) => template.id !== "diff-review");
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /source-writing-codegen-promotion\.evidenceTemplates must include diff-review/);
  });

  it("rejects completed approval values inside draft templates", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    const pkg = templates.packages.find((item) => item.id === "production-auth-promotion");
    const item = pkg.evidenceTemplates.find((template) => template.id === "session-policy-review");
    item.status = "complete";
    item.artifactURI = "s3://example/session-policy-review.json";
    item.artifactHash = "sha256:abc";
    item.approvedBy = "security-owner";
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /session-policy-review\.status must stay missing/);
    assert.match(result.stderr, /session-policy-review must not contain completed approval values/);
  });

  it("rejects forbidden sensitive fields in templates", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    const pkg = templates.packages.find((item) => item.id === "source-writing-codegen-promotion");
    pkg.evidenceTemplates[0].secret = "do-not-store";
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not include forbidden field secret/);
  });

  it("rejects templates that drop required completed-evidence fields", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    const pkg = templates.packages.find((item) => item.id === "production-auth-promotion");
    delete pkg.evidenceTemplates[0].rollbackCommands;
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /is missing required field rollbackCommands/);
  });

  it("rejects templates that drop the submitted evidence package validator", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    templates.submittedEvidenceValidator.script = "scripts/missing-validator.mjs";
    templates.submittedEvidenceValidator.command = "rtk node scripts/missing-validator.mjs";
    templates.submittedEvidenceValidator.tests = [];
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /submittedEvidenceValidator\.script must be scripts\/validate-platform-promotion-evidence-package\.mjs/);
    assert.match(result.stderr, /submittedEvidenceValidator\.command must include --package <promotion-evidence-package>/);
    assert.match(result.stderr, /submittedEvidenceValidator\.tests must include scripts\/platform-promotion-evidence-package\.test\.mjs/);
  });

  it("rejects templates that drop the draft package generator", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    templates.draftPackageGenerator.script = "scripts/missing-draft-generator.mjs";
    templates.draftPackageGenerator.command = "rtk node scripts/missing-draft-generator.mjs";
    templates.draftPackageGenerator.generatedFiles = [];
    templates.draftPackageGenerator.tests = [];
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /draftPackageGenerator\.script must be scripts\/generate-platform-promotion-evidence-package-draft\.mjs/);
    assert.match(result.stderr, /draftPackageGenerator\.generatedFiles must include resources\/generated\/platform-promotion-evidence-package-draft\.json/);
    assert.match(result.stderr, /draftPackageGenerator\.tests must include scripts\/platform-promotion-evidence-package-draft\.test\.mjs/);
  });

  it("rejects source-writing templates without review context for target families", () => {
    const templates = readJSON("resources/generated/platform-promotion-evidence-templates.json");
    const pkg = templates.packages.find((item) => item.id === "source-writing-codegen-promotion");
    pkg.reviewContext.reviewArtifact = "resources/generated/admin-scaffold-draft.md";
    pkg.reviewContext.sourceWriting = "enabled";
    pkg.reviewContext.targetFamilies = pkg.reviewContext.targetFamilies.filter((family) => family.id !== "api-routes");
    const templatesPath = tempJSON("platform-promotion-evidence-templates.json", templates);

    const result = runValidator(["--templates", templatesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /source-writing-codegen-promotion\.reviewContext\.reviewArtifact must be resources\/generated\/admin-scaffold-promotion-review\.json/);
    assert.match(result.stderr, /source-writing-codegen-promotion\.reviewContext\.sourceWriting must stay disabled/);
    assert.match(result.stderr, /source-writing-codegen-promotion\.reviewContext\.targetFamilies must include api-routes/);
  });
});
