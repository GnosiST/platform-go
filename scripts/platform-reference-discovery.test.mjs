import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-reference-discovery.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-reference-discovery-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-reference-discovery", () => {
  it("accepts current reference discovery contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ reference discovery candidates/);
  });

  it("rejects missing source-set evidence signals", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.sourceSets.find((sourceSet) => sourceSet.id === "backend-models").requiredSignals.push("MissingReferenceSignal");
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /source set backend-models is missing required signal MissingReferenceSignal/);
  });

  it("rejects foundation candidates detached from reference coverage", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates.find((candidate) => candidate.id === "dashboard-shell").coverageArea = "missing-area";
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /candidate dashboard-shell references missing foundation coverage area missing-area/);
  });

  it("rejects candidate evidence signals that are absent from declared source sets", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates.find((candidate) => candidate.id === "demo-data-reset").evidenceSignals.push("MissingCandidateEvidenceSignal");
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /candidate demo-data-reset evidence signal MissingCandidateEvidenceSignal is missing from declared sourceSets/);
  });

  it("rejects demoting admin query security and API boundary away from the foundation gate", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    const candidate = discovery.candidates.find((item) => item.id === "query-security-and-api-boundary");
    candidate.classification = "deferred";
    candidate.decision = "promote-later";
    candidate.reason = "Should fail because this is now a platform foundation gate.";
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /candidate query-security-and-api-boundary must be promoted as a foundation API-boundary gate/);
    assert.match(result.stderr, /candidate query-security-and-api-boundary decision must stay reuse-as-foundation-gate/);
  });

  it("rejects demoting app client API boundary away from the foundation gate", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    const candidate = discovery.candidates.find((item) => item.id === "app-client-api-boundary");
    candidate.classification = "deferred";
    candidate.decision = "promote-later";
    candidate.reason = "Should fail because app clients need the same unified request boundary.";
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /candidate app-client-api-boundary must be promoted as a foundation App client API boundary gate/);
    assert.match(result.stderr, /candidate app-client-api-boundary decision must stay reuse-as-foundation-gate/);
  });

  it("rejects business candidates that are promoted into a platform capability", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates.find((candidate) => candidate.id === "business-dispatch-transfer").expectedCapability = "platform-dispatch";
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /candidate business-dispatch-transfer must be owned by external-business-capability outside platform-go/);
  });

  it("rejects default profile leakage of optional or business capabilities", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === "platform-default").capabilities.push("app-phone");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform-default must not enable app-phone/);
  });

  it("rejects coverage areas that have no discovery candidate", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates = discovery.candidates.filter((candidate) => candidate.id !== "file-storage-assets");
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference discovery candidate file-storage-assets is missing/);
    assert.match(result.stderr, /reference discovery is missing a foundation candidate for coverage area file-storage/);
  });

  it("rejects missing multi-org membership reference classification", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates = discovery.candidates.filter((candidate) => candidate.id !== "multi-org-membership");
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference discovery candidate multi-org-membership is missing/);
    assert.match(result.stderr, /reference discovery is missing an extension candidate for boundary multi-org-membership/);
  });

  it("rejects non-resource candidates that are absent from reference coverage", () => {
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates.find((candidate) => candidate.id === "detailed-addresses").nonResourceCapabilities = ["missing-address-module"];
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /references non-resource capability missing-address-module that is missing from platform-reference-coverage/);
  });
});
