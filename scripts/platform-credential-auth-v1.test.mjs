import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-credential-auth-v1.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempFile(prefix, name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, typeof value === "string" ? value : `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-credential-auth-v1", () => {
  it("accepts the current credential-auth v1 contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform credential-auth v1 contract/);
  });

  it("rejects runtime enablement or weakening the existing password-provider guard", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.runtimeBoundary.status = "implemented-runtime";
    contract.runtimeBoundary.defaultRuntimeMutation = "allowed";
    contract.runtimeBoundary.existingPasswordProviderGuardMustRemain = false;
    const mainGo = fs.readFileSync(path.join(repoRoot, "cmd/platform-api/main.go"), "utf8").replace("kind == \"password\"", "kind == \"credential-auth\"");

    const result = runValidator([
      "--contract",
      tempFile("platform-credential-auth-v1-", "contract.json", contract),
      "--main-go",
      tempFile("platform-credential-auth-v1-main-", "main.go", mainGo),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeBoundary\.status must stay service-foundation-not-wired/);
    assert.match(result.stderr, /runtimeBoundary\.defaultRuntimeMutation must stay forbidden/);
    assert.match(result.stderr, /existing password provider guard must remain active/);
    assert.match(result.stderr, /cmd\/platform-api\/main\.go must still reject provider kind password/);
  });

  it("rejects generic Record.Values credential storage", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.genericRecordValuesPolicy.passwordCredentialStorageAllowed = true;
    contract.storageContracts.find((item) => item.id === "password_credentials").genericRecordValuesAllowed = true;
    contract.storageContracts.find((item) => item.id === "password_credentials").rawSecretPersistenceAllowed = true;

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /genericRecordValuesPolicy\.passwordCredentialStorageAllowed must stay false/);
    assert.match(result.stderr, /storage password_credentials must forbid generic Record.Values/);
    assert.match(result.stderr, /storage password_credentials must forbid raw secret persistence/);
  });

  it("rejects provider, challenge and notification SMS contract drift", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.providerModes = contract.providerModes.filter((item) => item.id !== "phone-sms-otp");
    contract.challengeContract.modes = ["off"];
    contract.notificationSmsBoundary.productionMockProviderAllowed = true;
    contract.apiContract.endpoints = contract.apiContract.endpoints.filter((item) => item.path !== "/api/auth/sms-otp/start");

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerModes must include phone-sms-otp/);
    assert.match(result.stderr, /challengeContract\.modes must include after-failure/);
    assert.match(result.stderr, /notificationSmsBoundary\.productionMockProviderAllowed must stay false/);
    assert.match(result.stderr, /apiContract\.endpoints must include POST \/api\/auth\/sms-otp\/start/);
  });

  it("rejects docs that omit credential-auth boundary language", () => {
    const result = runValidator([
      "--auth-doc",
      tempFile("platform-credential-auth-v1-doc-", "platform-auth.md", "# Auth\n"),
      "--capability-doc",
      tempFile("platform-credential-auth-v1-doc-", "platform-capability-development.md", "# Capability\n"),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /docs\/platform-auth\.md must document credential-auth v1/);
    assert.match(result.stderr, /docs\/platform-capability-development\.md must document credential-auth capability rules/);
  });
});
