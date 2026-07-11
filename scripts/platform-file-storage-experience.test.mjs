import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-file-storage-experience.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-file-storage-experience-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-file-storage-experience", () => {
  it("accepts the current file-storage experience gate", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated file-storage experience gate/);
  });

  it("rejects implemented UI promotion without product design and browser evidence", () => {
    const contract = readJSON("resources/platform-file-storage-experience.json");
    contract.designGate.productDesignStatus = "pending";
    contract.designGate.browserQaStatus = "pending";
    contract.designGate.implementationStatus = "blocked";
    delete contract.designGate.browserEvidence;
    const contractPath = tempJSON("platform-file-storage-experience.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /designGate\.productDesignStatus must be approved/);
    assert.match(result.stderr, /designGate\.browserQaStatus must be passed/);
    assert.match(result.stderr, /designGate\.implementationStatus must be implemented/);
    assert.match(result.stderr, /designGate\.browserEvidence\.tool must be chrome-devtools/);
  });

  it("rejects approaches that abandon the generic resource console extension", () => {
    const contract = readJSON("resources/platform-file-storage-experience.json");
    contract.designGate.requiredOrder = ["product-design", "superpowers:brainstorming"];
    contract.designGate.recommendedApproach = "standalone-file-manager-page";
    contract.experienceContract.surface = "custom file manager";
    const contractPath = tempJSON("platform-file-storage-experience.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /designGate\.requiredOrder must keep superpowers:brainstorming before product-design/);
    assert.match(result.stderr, /designGate\.recommendedApproach must stay generic-resource-console-extension/);
    assert.match(result.stderr, /experienceContract\.surface must stay files generic resource route/);
  });

  it("rejects experience contracts without preview, download, audit and mobile requirements", () => {
    const contract = readJSON("resources/platform-file-storage-experience.json");
    contract.experienceContract.requiredActions = ["upload"];
    contract.experienceContract.requiredPanels = ["metadata"];
    contract.experienceContract.previewTypes = ["image"];
    contract.experienceContract.auditVisualization = [];
    contract.experienceContract.responsiveRequirements = ["desktop table with drawer preview"];
    delete contract.experienceContract.auditFallback;
    const contractPath = tempJSON("platform-file-storage-experience.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /experienceContract\.requiredActions must include download/);
    assert.match(result.stderr, /experienceContract\.requiredPanels must include audit/);
    assert.match(result.stderr, /experienceContract\.previewTypes must include unsupported-fallback/);
    assert.match(result.stderr, /experienceContract\.auditVisualization must include file\.delete/);
    assert.match(result.stderr, /experienceContract\.responsiveRequirements must include mobile file cards before pagination/);
    assert.match(result.stderr, /experienceContract\.auditFallback must document audit-log fallback/);
  });

  it("rejects exposing the internal storage key as visible metadata", () => {
    const contract = readJSON("resources/platform-file-storage-experience.json");
    contract.experienceContract.metadataFields.push("storageKey");
    const contractPath = tempJSON("platform-file-storage-experience.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /experienceContract\.metadataFields must not include storageKey/);
  });

  it("rejects browser evidence with missing screenshot files", () => {
    const contract = readJSON("resources/platform-file-storage-experience.json");
    contract.designGate.browserEvidence.screenshots[0].path = "tmp/product-design/file-storage-experience-20260707/missing.png";
    const contractPath = tempJSON("platform-file-storage-experience.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /designGate\.browserEvidence screenshot path is missing or unsafe/);
  });
});
