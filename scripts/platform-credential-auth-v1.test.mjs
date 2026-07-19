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

  it("rejects production completion claims or weakening the existing password-provider guard", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.runtimeBoundary.status = "implemented-runtime";
    contract.runtimeBoundary.defaultRuntimeMutation = "allowed";
    contract.runtimeBoundary.existingPasswordProviderGuardMustRemain = false;
    contract.runtimeBoundary.productionComplete = true;
    const mainGo = fs.readFileSync(path.join(repoRoot, "cmd/platform-api/main.go"), "utf8").replace("kind == \"password\"", "kind == \"credential-auth\"");

    const result = runValidator([
      "--contract",
      tempFile("platform-credential-auth-v1-", "contract.json", contract),
      "--main-go",
      tempFile("platform-credential-auth-v1-main-", "main.go", mainGo),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeBoundary\.status must be persistent-runtime-p0/);
    assert.match(result.stderr, /runtimeBoundary\.defaultRuntimeMutation must stay forbidden/);
    assert.match(result.stderr, /existing password provider guard must remain active/);
    assert.match(result.stderr, /runtimeBoundary\.productionComplete must stay false/);
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

  it("rejects HTTPS-only or plaintext-compatible credential secret transport drift", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.runtimeBoundary.secretTransport = "HTTPS protects credential-bearing requests.";
    contract.apiContract.secretTransportContract.algorithm = "HTTPS";
    contract.apiContract.secretTransportContract.plaintextSecretFieldsRejectedWhenRequired = ["secret.encrypted"];
    contract.apiContract.secretTransportContract.httpsRole = "HTTPS satisfies credential secret encryption.";
    contract.apiContract.endpoints = contract.apiContract.endpoints.filter((item) => item.path !== "/api/auth/credential-secret-key");
    const authDoc = fs
      .readFileSync(path.join(repoRoot, "docs/platform-auth.md"), "utf8")
      .replace("When `RequireEncryptedSecrets=true`, the server rejects plaintext `secret.value` or `secret.code`", "")
      .replace("cannot be used as a substitute for `secret.encrypted`", "");
    const capabilityDoc = fs
      .readFileSync(path.join(repoRoot, "docs/platform-capability-development.md"), "utf8")
      .replace("When encrypted secrets are required, `secret.value` and `secret.code` are invalid", "")
      .replace("cannot be used as a substitute for `secret.encrypted`", "");
    const dataGovernanceDoc = fs
      .readFileSync(path.join(repoRoot, "docs/platform-data-governance-and-integrations-assessment.md"), "utf8")
      .replace("application-layer hybrid encryption", "HTTPS-only transport")
      .replace("HTTPS cannot be used as a substitute for `secret.encrypted`", "HTTPS protects credential secrets.");

    const result = runValidator([
      "--contract",
      tempFile("platform-credential-auth-v1-", "contract.json", contract),
      "--auth-doc",
      tempFile("platform-credential-auth-v1-doc-", "platform-auth.md", authDoc),
      "--capability-doc",
      tempFile("platform-credential-auth-v1-doc-", "platform-capability-development.md", capabilityDoc),
      "--data-governance-doc",
      tempFile("platform-credential-auth-v1-doc-", "platform-data-governance-and-integrations-assessment.md", dataGovernanceDoc),
      "--http-credential-auth",
      tempFile("platform-credential-auth-v1-http-", "credential_auth.go", "package httpapi\n"),
      "--admin-client",
      tempFile("platform-credential-auth-v1-client-", "client.ts", "export {};\n"),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeBoundary\.secretTransport must document application-layer hybrid encryption/);
    assert.match(result.stderr, /runtimeBoundary\.secretTransport must document ECDH P-256/);
    assert.match(result.stderr, /runtimeBoundary\.secretTransport must document AES-256-GCM\/A256GCM/);
    assert.match(result.stderr, /runtimeBoundary\.secretTransport must state HTTPS is only a production baseline/);
    assert.match(result.stderr, /runtimeBoundary\.secretTransport must state HTTPS cannot substitute secret\.encrypted/);
    assert.match(result.stderr, /apiContract\.secretTransportContract\.algorithm must be ECDH-P256-HKDF-SHA256\+A256GCM/);
    assert.match(result.stderr, /apiContract\.secretTransportContract\.plaintextSecretFieldsRejectedWhenRequired must include secret\.value/);
    assert.match(result.stderr, /apiContract\.secretTransportContract\.httpsRole must state HTTPS cannot substitute secret\.encrypted/);
    assert.match(result.stderr, /apiContract\.endpoints must include GET \/api\/auth\/credential-secret-key/);
    assert.match(result.stderr, /docs\/platform-auth\.md must document plaintext secret field rejection/);
    assert.match(result.stderr, /docs\/platform-auth\.md must state HTTPS cannot substitute secret\.encrypted/);
    assert.match(result.stderr, /docs\/platform-capability-development\.md must document plaintext secret field rejection/);
    assert.match(result.stderr, /docs\/platform-capability-development\.md must state HTTPS cannot substitute secret\.encrypted/);
    assert.match(result.stderr, /docs\/platform-data-governance-and-integrations-assessment\.md must document application-layer hybrid encryption/);
    assert.match(result.stderr, /docs\/platform-data-governance-and-integrations-assessment\.md must state HTTPS cannot substitute secret\.encrypted/);
    assert.match(result.stderr, /internal\/platform\/httpapi\/credential_auth\.go must reject plaintext secret fields/);
    assert.match(result.stderr, /admin API client must encrypt password and sms-otp secrets before login submission/);
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
