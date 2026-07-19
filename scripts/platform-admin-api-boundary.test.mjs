import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-admin-api-boundary.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-api-boundary-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-admin-api-boundary", () => {
  it("accepts the current admin API boundary contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated admin API boundary and query security/);
  });

  it("requires the admin oidc start and exchange OpenAPI contract", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    const start = openAPI.paths?.["/api/auth/providers/{provider}/start"]?.post;
    const login = openAPI.paths?.["/api/auth/login"]?.post;

    assert.equal(start?.operationId, "startAdminAuthProvider");
    assert.deepEqual(start?.security, []);
    assert.equal(
      start?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminAuthProviderStartRequest",
    );
    assert.equal(
      start?.responses?.["200"]?.content?.["application/json"]?.schema?.properties?.data?.$ref,
      "#/components/schemas/AdminAuthProviderStartData",
    );
    assert.ok(start?.responses?.["501"]);
    assert.equal(login?.operationId, "adminAuthLogin");
    assert.deepEqual(login?.security, []);
    assert.equal(
      login?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminAuthLoginRequest",
    );
    assert.ok(login?.responses?.["501"]);
    const smsOTPStart = openAPI.paths?.["/api/auth/sms-otp/start"]?.post;
    assert.equal(smsOTPStart?.operationId, "startAdminCredentialSMSOTP");
    assert.deepEqual(smsOTPStart?.security, []);
    assert.equal(smsOTPStart?.["x-platform-runtime-status"], "persistent-runtime-p0");
    assert.equal(
      smsOTPStart?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminCredentialSMSOTPStartRequest",
    );
    assert.ok(smsOTPStart?.responses?.["201"]);
    assert.ok(smsOTPStart?.responses?.["501"]);
    const credentialSecretKey = openAPI.paths?.["/api/auth/credential-secret-key"]?.get;
    assert.equal(credentialSecretKey?.operationId, "getAdminCredentialSecretKey");
    assert.deepEqual(credentialSecretKey?.security, []);
    assert.equal(credentialSecretKey?.["x-platform-runtime-status"], "persistent-runtime-p0");
    const challenge = openAPI.components?.schemas?.AdminAuthProviderStartRequest?.properties?.codeChallenge;
    assert.equal(challenge?.minLength, 43);
    assert.equal(challenge?.maxLength, 43);
    assert.equal(challenge?.pattern, "^[A-Za-z0-9_-]{43}$");
    const loginProperties = openAPI.components?.schemas?.AdminAuthLoginRequest?.properties ?? {};
    assert.ok(loginProperties.state);
    assert.ok(loginProperties.codeVerifier);
    assert.equal(loginProperties.codeVerifier.writeOnly, true);
    assert.equal(loginProperties.identifier?.$ref, "#/components/schemas/AdminCredentialAuthIdentifier");
    assert.equal(loginProperties.secret?.$ref, "#/components/schemas/AdminCredentialAuthSecret");
    assert.equal(loginProperties.challenge?.$ref, "#/components/schemas/AdminCredentialAuthChallengeProof");
    const credentialSecret = openAPI.components?.schemas?.AdminCredentialAuthSecret;
    assert.deepEqual(credentialSecret?.properties?.type?.enum, ["password", "sms-otp"]);
    assert.equal(credentialSecret?.properties?.value?.writeOnly, true);
    assert.equal(credentialSecret?.properties?.code?.writeOnly, true);
    assert.equal(credentialSecret?.properties?.encrypted?.$ref, "#/components/schemas/AdminCredentialAuthSecretEnvelope");
    const credentialEnvelope = openAPI.components?.schemas?.AdminCredentialAuthSecretEnvelope;
    assert.equal(credentialEnvelope?.properties?.ciphertext?.writeOnly, true);
    assert.ok(credentialEnvelope?.required?.includes("clientPublicKey"));
    const credentialKey = openAPI.components?.schemas?.AdminCredentialAuthSecretKeyData;
    assert.ok(credentialKey?.required?.includes("publicKey"));
    assert.deepEqual(credentialKey?.properties?.algorithm?.enum, ["ECDH-P256-HKDF-SHA256+A256GCM"]);
    const smsData = openAPI.components?.schemas?.AdminCredentialSMSOTPStartData;
    assert.equal(smsData?.properties?.debugCode?.["x-platform-development-only"], true);
    assert.equal(smsData?.properties?.debugCode?.["x-platform-sensitivity"], "secret");
    const settingsRuntime = openAPI.paths?.["/api/admin/settings"]?.get;
    assert.equal(settingsRuntime?.operationId, "getAdminSettingsRuntime");
    assert.equal(settingsRuntime?.["x-platform-runtime"], "settings-runtime-p0");
    assert.equal(settingsRuntime?.["x-platform-permission"], "admin:settings:read");
    const settingsUpdate = openAPI.paths?.["/api/admin/settings/{resource}/{id}"]?.put;
    assert.equal(settingsUpdate?.operationId, "updateAdminSettingsResource");
    assert.equal(settingsUpdate?.["x-platform-runtime"], "settings-runtime-p0");
    assert.equal(settingsUpdate?.["x-platform-permission"], "admin:settings:update");
    assert.equal(
      settingsUpdate?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminSettingsUpdateRequest",
    );
    assert.equal(
      openAPI.components?.schemas?.AdminSettingsResourceItem?.["x-platform-secret-projection"],
      "response projection must mask or omit protected provider secrets",
    );
    const messageCenterTestSend = openAPI.paths?.["/api/admin/message-center/test-send"]?.post;
    assert.equal(messageCenterTestSend?.operationId, "testSendMessageCenter");
    assert.equal(messageCenterTestSend?.["x-platform-runtime"], "notification-sms-test-send-p0");
    assert.equal(messageCenterTestSend?.["x-platform-permission"], "admin:message-center:update");
    assert.equal(
      messageCenterTestSend?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminMessageCenterTestSendRequest",
    );
    assert.ok(messageCenterTestSend?.responses?.["201"]);
    assert.ok(messageCenterTestSend?.responses?.["503"]?.["x-platform-error-codes"]?.includes("ADMIN_MESSAGE_CENTER_UNAVAILABLE"));
    const messageCenterRequest = openAPI.components?.schemas?.AdminMessageCenterTestSendRequest;
    assert.equal(messageCenterRequest?.properties?.recipient?.writeOnly, true);
    assert.equal(messageCenterRequest?.properties?.recipient?.["x-platform-response-policy"], "redacted target only");
    assert.equal(messageCenterRequest?.properties?.templateParams?.writeOnly, true);
    assert.equal(
      openAPI.components?.schemas?.AdminMessageCenterSMSReceipt?.["x-platform-secret-projection"],
      "only redacted SMS target and provider receipt metadata are returned",
    );
    assert.deepEqual(
      Object.keys(openAPI.components?.schemas?.AdminAuthProviderStartData?.properties ?? {}).sort(),
      ["authorizationUrl", "expiresAt", "state"],
    );
  });

  it("rejects secret fields in the admin oidc start response", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    openAPI.components.schemas.AdminAuthProviderStartData ??= { type: "object", properties: {} };
    openAPI.components.schemas.AdminAuthProviderStartData.properties.clientSecret = { type: "string" };
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminAuthProviderStartData must not expose sensitive response field clientSecret/);
  });

  it("rejects a weakened admin oidc challenge or missing resolver response", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    openAPI.components.schemas.AdminAuthProviderStartRequest.properties.codeChallenge.maxLength = 128;
    delete openAPI.paths["/api/auth/providers/{provider}/start"].post.responses["501"];
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminAuthProviderStartRequest must require an exact S256 codeChallenge/);
    assert.match(result.stderr, /admin OpenAPI auth operations must declare the missing resolver 501 response/);
  });

  it("rejects a readable admin oidc authorization code field", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    openAPI.components.schemas.AdminAuthLoginRequest.properties.code.writeOnly = false;
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminAuthLoginRequest code must stay writeOnly/);
  });

  it("rejects missing credential-auth SMS OTP OpenAPI and readable credential secrets", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    delete openAPI.paths["/api/auth/sms-otp/start"];
    delete openAPI.paths["/api/auth/credential-secret-key"];
    delete openAPI.paths["/api/admin/settings"];
    delete openAPI.paths["/api/admin/settings/{resource}/{id}"];
    delete openAPI.paths["/api/admin/message-center/test-send"];
    openAPI.components.schemas.AdminAuthLoginRequest.properties.secret = { type: "object" };
    openAPI.components.schemas.AdminCredentialAuthSecret.properties.value.writeOnly = false;
    delete openAPI.components.schemas.AdminCredentialAuthSecret.properties.encrypted;
    openAPI.components.schemas.AdminCredentialAuthSecretEnvelope.properties.ciphertext.writeOnly = false;
    openAPI.components.schemas.AdminCredentialAuthSecretKeyData.properties.algorithm.enum = ["none"];
    openAPI.components.schemas.AdminCredentialSMSOTPStartData.properties.debugCode["x-platform-development-only"] = false;
    delete openAPI.components.schemas.AdminSettingsResourceItem["x-platform-secret-projection"];
    openAPI.components.schemas.AdminMessageCenterTestSendRequest.properties.recipient.writeOnly = false;
    delete openAPI.components.schemas.AdminMessageCenterTestSendRequest.properties.recipient["x-platform-response-policy"];
    openAPI.components.schemas.AdminMessageCenterTestSendRequest.properties.templateParams.writeOnly = false;
    delete openAPI.components.schemas.AdminMessageCenterSMSReceipt["x-platform-secret-projection"];
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /credential-auth SMS OTP start operation/);
    assert.match(result.stderr, /credential-auth secret key operation/);
    assert.match(result.stderr, /settings runtime P0 aggregation operation/);
    assert.match(result.stderr, /settings runtime P0 update operation/);
    assert.match(result.stderr, /message-center SMS test-send runtime operation/);
    assert.match(result.stderr, /structured credential-auth identifier, secret and challenge fields/);
    assert.match(result.stderr, /AdminCredentialAuthSecret must constrain password and SMS OTP secrets as encrypted write-only request fields/);
    assert.match(result.stderr, /AdminCredentialAuthSecretEnvelope must require client public key and write-only ciphertext/);
    assert.match(result.stderr, /AdminCredentialAuthSecretKeyData must expose ECDH\/AES public key metadata/);
    assert.match(result.stderr, /debugCode development-only secret/);
    assert.match(result.stderr, /AdminMessageCenterTestSendRequest must keep recipient and template params write-only/);
    assert.match(result.stderr, /AdminMessageCenterSMSReceipt must declare redacted SMS target projection/);
  });

  it("rejects direct fetch outside the platform API client allowlist", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.adminSourceBoundary.allowedDirectFetchFiles = [];
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin\/src\/platform\/api\/client\.ts:\d+ calls fetch\(\) outside the platform API client/);
  });

  it("rejects missing backend query safety snippets", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.requiredSourceSnippets.push({
      path: "internal/platform/adminresource/query.go",
      contains: "raw sql concatenation is allowed",
    });
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /internal\/platform\/adminresource\/query\.go is missing required snippet raw sql concatenation is allowed/);
  });

  it("rejects weakening the structured query policy", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.querySecurity.rawSQLAllowed = true;
    boundary.querySecurity.sensitivityPolicySource = "field-name-list";
    boundary.querySecurity.fieldNameInferenceAllowed = true;
    boundary.querySecurity.encryptedFieldQueryPolicy = "all-operators";
    boundary.querySecurity.encryptedFieldSortAllowed = true;
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /querySecurity\.rawSQLAllowed must stay false/);
    assert.match(result.stderr, /querySecurity\.sensitivityPolicySource must stay capability manifest/);
    assert.match(result.stderr, /querySecurity\.fieldNameInferenceAllowed must stay false/);
    assert.match(result.stderr, /querySecurity\.encryptedFieldQueryPolicy must stay declared-blind-index-exact-match-only/);
    assert.match(result.stderr, /querySecurity\.encryptedFieldSortAllowed must stay false/);
  });
});
