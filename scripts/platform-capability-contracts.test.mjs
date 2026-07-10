import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-capability-contracts.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-capability-contracts-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

describe("validate-platform-capability-contracts", () => {
  it("accepts the current platform capability contracts", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform capability contracts/);
  });

  it("rejects profile capabilities that are not classified", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    contracts.capabilities = contracts.capabilities.filter((capability) => capability.id !== "notification");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability notification appears in profiles but is missing from platform capability contracts/);
  });

  it("rejects default-enabled capabilities removed from the runtime default", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault).capabilities = profiles.profiles
      .find((profile) => profile.id === profiles.runtimeDefault)
      .capabilities.filter((capability) => capability !== "parameter");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--contracts", contractsPath, "--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract parameter is default-enabled and must be in config defaults and runtime default profile/);
  });

  it("rejects profile-only capabilities leaking into the default profile", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault).capabilities.push("job");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract job is profile-only and must not be enabled by default/);
  });

  it("rejects declared contract surface drift from audited manifests", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    contracts.capabilities.find((capability) => capability.id === "rbac").adminResources =
      contracts.capabilities.find((capability) => capability.id === "rbac").adminResources.filter((resource) => resource !== "roles");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract rbac\.adminResources must match capability audit output/);
  });

  it("rejects external business capability inside non-business profiles", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === "minimal-admin").capabilities.push("external-business-capability");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract external-business-capability must not appear in non-business profile minimal-admin/);
  });
});
