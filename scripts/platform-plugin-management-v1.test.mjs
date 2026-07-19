import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-plugin-management-v1.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-plugin-management-v1-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-plugin-management-v1", () => {
  it("accepts the current restart-required desired-state contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated plugin management v1 contract/);
  });

  it("rejects runtime hot install, remote repository pull and websocket activation", () => {
    const contract = readJSON("resources/platform-plugin-management-v1.json");
    contract.runtimePolicy.runtimeHotInstall = true;
    contract.runtimePolicy.remoteRepositoryPull = true;
    contract.runtimePolicy.webSocketIntegratedInV1 = true;

    const result = runValidator(["--contract", tempJSON("plugin-management-v1.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimePolicy\.runtimeHotInstall must stay false/);
    assert.match(result.stderr, /runtimePolicy\.remoteRepositoryPull must stay false/);
    assert.match(result.stderr, /runtimePolicy\.webSocketIntegratedInV1 must stay false/);
  });

  it("rejects missing desired-state sources and lifecycle gates", () => {
    const contract = readJSON("resources/platform-plugin-management-v1.json");
    contract.desiredState.sources = contract.desiredState.sources.filter((source) => source.id !== "PLATFORM_CAPABILITIES");
    contract.lifecycleGates = contract.lifecycleGates.filter((gate) => gate.id !== "manual-restart");

    const result = runValidator(["--contract", tempJSON("plugin-management-v1.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /desiredState\.sources must include PLATFORM_CAPABILITIES/);
    assert.match(result.stderr, /lifecycleGates must include manual-restart/);
  });

  it("rejects contracts that omit manifest projection surfaces or settings-center aggregation", () => {
    const contract = readJSON("resources/platform-plugin-management-v1.json");
    contract.manifestProjection.settingsCenter.route = "/capability-settings";
    contract.manifestProjection.settingsCenter.uses = contract.manifestProjection.settingsCenter.uses.filter(
      (source) => source !== "GET /api/capabilities configResources",
    );
    contract.manifestProjection.projectedSurfaces = contract.manifestProjection.projectedSurfaces.filter(
      (surface) => surface.id !== "configResources",
    );
    contract.manifestProjection.requiredPostRestartChecks = contract.manifestProjection.requiredPostRestartChecks.filter(
      (check) => !check.startsWith("/settings groups"),
    );

    const result = runValidator(["--contract", tempJSON("plugin-management-v1.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /manifestProjection\.settingsCenter\.route must be \/settings/);
    assert.match(result.stderr, /manifestProjection\.settingsCenter\.uses must include GET \/api\/capabilities configResources/);
    assert.match(result.stderr, /manifestProjection\.projectedSurfaces must include configResources/);
    assert.match(result.stderr, /manifestProjection\.requiredPostRestartChecks must include \/settings groups configuration resources/);
  });

  it("rejects accepted combinations that are not backed by profiles or operation policy", () => {
    const contract = readJSON("resources/platform-plugin-management-v1.json");
    const combination = contract.acceptedCombinations.find((item) => item.id === "minimal-admin");
    combination.profile = "missing-profile";
    combination.operationPolicyCombination = "missing-combination";

    const result = runValidator(["--contract", tempJSON("plugin-management-v1.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /accepted combination minimal-admin references missing profile missing-profile/);
    assert.match(result.stderr, /accepted combination minimal-admin references missing operation policy combination missing-combination/);
  });

  it("rejects operation policies that are not wired to the plugin management gate", () => {
    const operationPolicy = readJSON("resources/platform-capability-operation-policy.json");
    operationPolicy.requiredValidators = operationPolicy.requiredValidators.filter(
      (validator) => validator !== "scripts/validate-platform-plugin-management-v1.mjs",
    );
    operationPolicy.requiredTests = operationPolicy.requiredTests.filter(
      (test) => test !== "scripts/platform-plugin-management-v1.test.mjs",
    );

    const result = runValidator(["--operation-policy", tempJSON("operation-policy.json", operationPolicy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operation policy requiredValidators must include scripts\/validate-platform-plugin-management-v1\.mjs/);
    assert.match(result.stderr, /operation policy requiredTests must include scripts\/platform-plugin-management-v1\.test\.mjs/);
  });

  it("rejects human and AI protocols that omit plugin management gates", () => {
    const protocol = readJSON("resources/platform-human-ai-development-protocol.json");
    protocol.requiredValidators = protocol.requiredValidators.filter(
      (validator) => validator !== "scripts/validate-platform-plugin-management-v1.mjs",
    );
    protocol.requiredTests = protocol.requiredTests.filter((test) => test !== "scripts/platform-plugin-management-v1.test.mjs");
    protocol.minimumAcceptanceCommands = protocol.minimumAcceptanceCommands.filter(
      (command) => command !== "rtk node scripts/validate-platform-plugin-management-v1.mjs",
    );
    const lifecycleDomain = protocol.domains.find((domain) => domain.id === "capability-lifecycle-operations");
    lifecycleDomain.sourceOfTruth = lifecycleDomain.sourceOfTruth.filter(
      (source) => source !== "resources/platform-plugin-management-v1.json",
    );

    const result = runValidator(["--protocol", tempJSON("protocol.json", protocol)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /protocol requiredValidators must include scripts\/validate-platform-plugin-management-v1\.mjs/);
    assert.match(result.stderr, /protocol requiredTests must include scripts\/platform-plugin-management-v1\.test\.mjs/);
    assert.match(result.stderr, /protocol minimumAcceptanceCommands must include rtk node scripts\/validate-platform-plugin-management-v1\.mjs/);
    assert.match(result.stderr, /capability-lifecycle-operations\.sourceOfTruth must include resources\/platform-plugin-management-v1\.json/);
  });
});
