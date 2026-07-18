import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-capability-operation-policy.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-capability-operation-policy-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-capability-operation-policy", () => {
  it("accepts the current install, disable and uninstall policy", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ capability operation policies/);
  });

  it("rejects operation policies that do not cover every capability contract", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    policy.capabilities = policy.capabilities.filter((capability) => capability.id !== "admin-oidc");

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability admin-oidc from contracts is missing from operation policy/);
  });

  it("rejects foundation core capabilities that become uninstallable", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    const tenant = policy.capabilities.find((capability) => capability.id === "tenant");
    tenant.uninstallMode = "disable-only-retain-data";

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability tenant is foundation-core and must use uninstallMode not-supported-foundation/);
  });

  it("rejects policies that claim runtime plugin management support", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    policy.operationModel.activation = "hot-apply";
    policy.operationModel.manualRestartSupported = false;
    policy.operationModel.remoteRepositoryPull = true;
    policy.operationModel.oneClickRemoteInstall = true;
    policy.operationModel.webSocketRequired = true;

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operationModel\.activation must be manual-restart/);
    assert.match(result.stderr, /operationModel\.manualRestartSupported must stay true/);
    assert.match(result.stderr, /operationModel\.remoteRepositoryPull must stay false/);
    assert.match(result.stderr, /operationModel\.oneClickRemoteInstall must stay false/);
    assert.match(result.stderr, /operationModel\.webSocketRequired must stay false/);
  });

  it("rejects operation policies without desired-state restart semantics", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    policy.desiredState.desiredStateSources = policy.desiredState.desiredStateSources.filter(
      (source) => source !== "PLATFORM_CAPABILITY_LOCK_FILE",
    );
    policy.desiredState.stateFields = policy.desiredState.stateFields.filter((field) => field !== "pendingRestart");
    policy.desiredState.manualRestartClearsPendingRestart = false;
    policy.desiredState.runtimeHotApplySupported = true;

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /desiredState\.desiredStateSources must include PLATFORM_CAPABILITY_LOCK_FILE/);
    assert.match(result.stderr, /desiredState\.stateFields must include pendingRestart/);
    assert.match(result.stderr, /desiredState\.manualRestartClearsPendingRestart must stay true/);
    assert.match(result.stderr, /desiredState\.runtimeHotApplySupported must stay false/);
  });

  it("rejects operation policies that stop proving disabled contract surfaces disappear", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    policy.contractDisappearanceAfterDisable.appliesToSurfaces = policy.contractDisappearanceAfterDisable.appliesToSurfaces.filter(
      (surface) => surface !== "authProviders",
    );
    policy.contractDisappearanceAfterDisable.validatedBy =
      policy.contractDisappearanceAfterDisable.validatedBy.filter(
        (field) => field !== "validatedCombinations.expectedExcludedAuthProviders",
      );

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /contractDisappearanceAfterDisable\.appliesToSurfaces must include authProviders/);
    assert.match(
      result.stderr,
      /contractDisappearanceAfterDisable\.validatedBy must include validatedCombinations\.expectedExcludedAuthProviders/,
    );
  });

  it("rejects combinations that still expose disabled resources, routes, providers or demo data", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    const minimalAdmin = policy.validatedCombinations.find((combination) => combination.id === "minimal-admin");
    minimalAdmin.expectedExcludedAdminResources.push("sessions");
    minimalAdmin.expectedExcludedAppRoutes.push("POST /api/app/auth/login");
    minimalAdmin.expectedExcludedAuthProviders.push("demo");
    const defaultProfile = policy.validatedCombinations.find((combination) => combination.id === "platform-default");
    defaultProfile.expectedExcludedDemoDataSets = ["platform-demo-tenants"];

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /combination minimal-admin must exclude admin resource sessions/);
    assert.match(result.stderr, /combination minimal-admin must exclude app route POST \/api\/app\/auth\/login/);
    assert.match(result.stderr, /combination minimal-admin must exclude auth provider demo/);
    assert.match(result.stderr, /combination platform-default must exclude demo data set platform-demo-tenants/);
  });

  it("rejects optional operation combinations that do not resolve through capability audit", () => {
    const policy = readJSON("resources/platform-capability-operation-policy.json");
    policy.validatedCombinations.push({
      id: "broken-combination",
      purpose: "Invalid combination used by the test.",
      capabilities: ["session"],
      expectedCapabilities: ["session"],
      expectedExcludedCapabilities: [],
      expectedAdminResources: [],
      expectedAppRoutes: [],
    });

    const result = runValidator(["--policy", tempJSON("operation-policy.json", policy)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /combination broken-combination failed capability audit/);
  });

  it("rejects production OIDC profiles that do not include admin-oidc", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    const profile = profiles.profiles.find((item) => item.id === "production-admin-oidc-ready");
    profile.capabilities = profile.capabilities.filter((capability) => capability !== "admin-oidc");

    const result = runValidator(["--profiles", tempJSON("profiles.json", profiles)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile production-admin-oidc-ready must include required capability admin-oidc/);
  });
});
