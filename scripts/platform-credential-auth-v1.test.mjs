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

  it("rejects product UI credential debug data exposure", () => {
    const adminLoginView = `${fs.readFileSync(path.join(repoRoot, "admin/src/platform/auth/AdminLoginView.tsx"), "utf8")}\nvoid smsOTPTransaction?.debugCode;\n`;
    const adminClient = `${fs.readFileSync(path.join(repoRoot, "admin/src/platform/api/client.ts"), "utf8")}\nexport type LeakedCredentialDebug = { debugVisible?: boolean };\n`;

    const result = runValidator([
      "--admin-login-view",
      tempFile("platform-credential-auth-v1-login-", "AdminLoginView.tsx", adminLoginView),
      "--admin-client",
      tempFile("platform-credential-auth-v1-client-", "client.ts", adminClient),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin login view must not expose or consume debugCode development-only credential debug data/);
    assert.match(result.stderr, /Admin API client must not expose or consume debugVisible development-only credential debug data/);
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
    assert.match(result.stderr, /runtimeBoundary\.status must be deliverable-v1/);
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
    contract.challengeContract.sliderProofTolerancePixels = 3;
    contract.configurationContract.securityKeys = contract.configurationContract.securityKeys.filter(
      (key) => key !== "PLATFORM_CREDENTIAL_AUTH_CHALLENGE_TTL_SECONDS",
    );
    contract.notificationSmsBoundary.productionMockProviderAllowed = true;
    contract.notificationSmsBoundary.adapterContract = "SMTP live supplier integration is claimed";
    contract.implementationPackages.find((item) => item.id === "C-notification-sms-adapters").scope = "internal/platform/notification";
    contract.messageCenterContract.runtimeEndpoints = ["POST /api/admin/message-center/test-send"];
    contract.messageCenterContract.deliveryWorker.providerRuntimeTruth = "SMTP live supplier integration is claimed";
    contract.apiContract.implementedNow = contract.apiContract.implementedNow.filter((item) => !item.includes("POST /api/auth/challenges"));
    contract.apiContract.endpoints = contract.apiContract.endpoints.filter((item) => item.path !== "/api/auth/sms-otp/start");

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerModes must include phone-sms-otp/);
    assert.match(result.stderr, /challengeContract\.modes must be the mandatory always baseline/);
    assert.match(result.stderr, /challengeContract\.sliderProofTolerancePixels must be 9/);
    assert.match(result.stderr, /configurationContract\.securityKeys must include PLATFORM_CREDENTIAL_AUTH_CHALLENGE_TTL_SECONDS/);
    assert.match(result.stderr, /notificationSmsBoundary\.productionMockProviderAllowed must stay false/);
    assert.match(result.stderr, /notificationSmsBoundary\.adapterContract must describe official SDK live adapters plus dry-run\/config validation/);
    assert.match(result.stderr, /C-notification-sms-adapters scope must document Aliyun\/Tencent official SDK-backed live adapters/);
    assert.match(result.stderr, /C-notification-sms-adapters scope must document approved-account delivery evidence/);
    assert.match(result.stderr, /messageCenterContract\.runtimeEndpoints must include POST \/api\/admin\/message-center\/deliveries\/run/);
    assert.match(result.stderr, /messageCenterContract\.deliveryWorker\.providerRuntimeTruth must describe Aliyun\/Tencent live SMS SDK adapters plus dry-run\/config validation/);
    assert.match(result.stderr, /apiContract\.implementedNow must include POST \/api\/auth\/challenges creates digest-only CAPTCHA\/slider challenge transactions for login/);
    assert.match(result.stderr, /apiContract\.implementedNow must include POST \/api\/auth\/challenges\/verify prevalidates slider proofs without consuming successful challenges, counts failed attempts and returns only valid/);
    assert.match(result.stderr, /apiContract\.endpoints must include POST \/api\/auth\/sms-otp\/start/);
  });

  it("locks completed HTTP and Admin UI packages while keeping production promotion open", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.implementationPackages.find((item) => item.id === "D-auth-api-compatibility").status = "in-progress";
    contract.implementationPackages.find((item) => item.id === "E-admin-login-ui").status = "in-progress";
    const security = contract.implementationPackages.find((item) => item.id === "F-security-governance");
    security.status = "done";
    security.scope = "OpenAPI contract coverage and rate-limit enforcement";

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /D-auth-api-compatibility must be done after the deliverable-v1 HTTP runtime closeout/);
    assert.match(result.stderr, /E-admin-login-ui must be done after the deliverable-v1 provider-driven Admin login UI closeout/);
    assert.match(result.stderr, /F-security-governance must remain in-progress until independent production promotion evidence exists/);
    assert.match(result.stderr, /F-security-governance scope must document production env validation/);
    assert.match(result.stderr, /F-security-governance scope must document CI test\/build gates/);
    assert.match(result.stderr, /F-security-governance scope must document independent production promotion evidence remains/);
  });

  it("rejects credential-auth production env or CI gate removal", () => {
    const result = runValidator([
      "--production-env-validator",
      tempFile("platform-credential-auth-v1-production-env-", "validate-platform-production-env.mjs", "// credential auth gate removed\n"),
      "--production-env-test",
      tempFile("platform-credential-auth-v1-production-env-test-", "platform-production-env.test.mjs", "// credential auth tests removed\n"),
      "--ci-workflow",
      tempFile("platform-credential-auth-v1-ci-", "ci.yml", "name: CI\n"),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /credential-auth production env validation must remain implemented/);
    assert.match(result.stderr, /credential-auth production env validation must retain its contract test/);
    assert.match(result.stderr, /credential-auth CI integration must retain go test \.\/\.\.\./);
    assert.match(result.stderr, /credential-auth CI integration must retain node --test scripts\/\*\.test\.mjs/);
    assert.match(result.stderr, /credential-auth CI integration must retain npm run build --prefix admin/);
  });

  it("rejects weakened slider preverification consumption and response contracts", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.challengeContract.sliderPreverification.allowedKinds = ["captcha", "slider"];
    contract.challengeContract.sliderPreverification.successConsumesChallenge = true;
    contract.challengeContract.sliderPreverification.failureIncrementsAttempts = false;
    contract.challengeContract.sliderPreverification.finalLoginConsumesChallenge = false;
    contract.challengeContract.sliderPreverification.requestWriteOnlyFields = ["proof"];
    contract.challengeContract.sliderPreverification.responseFields = ["valid", "targetX"];
    contract.challengeContract.sliderPreverification.forbiddenResponseFields = [];
    contract.challengeContract.atomicStateTransitions.consume = "read-modify-write";
    contract.challengeContract.atomicStateTransitions.failureCounting = "load-increment-save";
    contract.challengeContract.atomicStateTransitions.preconditions = ["enabled"];
    contract.challengeContract.atomicStateTransitions.staleWritesCanReactivate = true;
    contract.apiContract.endpoints = contract.apiContract.endpoints.filter((item) => item.path !== "/api/auth/challenges/verify");

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /challengeContract\.sliderPreverification\.allowedKinds must contain only slider/);
    assert.match(result.stderr, /successful checks non-consuming, failed checks attempt-counted and final login consuming/);
    assert.match(result.stderr, /challengeContract\.sliderPreverification\.requestWriteOnlyFields must include clientFingerprint/);
    assert.match(result.stderr, /challengeContract\.sliderPreverification\.responseFields must contain only valid/);
    assert.match(result.stderr, /challengeContract\.sliderPreverification\.forbiddenResponseFields must include proof/);
    assert.match(result.stderr, /challengeContract\.atomicStateTransitions must require conditional consumption and atomic failure counting/);
    assert.match(result.stderr, /challengeContract\.atomicStateTransitions\.preconditions must include unused/);
    assert.match(result.stderr, /challengeContract\.atomicStateTransitions\.staleWritesCanReactivate must stay false/);
    assert.match(result.stderr, /apiContract\.endpoints must include POST \/api\/auth\/challenges\/verify/);
  });

  it("declares slider preverification rate-limit responses", () => {
    const openapi = readJSON("resources/generated/openapi.admin.json");
    const responses = openapi.paths["/api/auth/challenges/verify"].post.responses;

    assert.deepEqual(responses["429"], { $ref: "#/components/responses/TooManyRequests" });
    assert.deepEqual(responses["503"], { $ref: "#/components/responses/ServiceUnavailable" });
    assert.equal(openapi.components.responses.TooManyRequests.description, "Request rate limit exceeded");
    assert.equal(openapi.components.responses.ServiceUnavailable.description, "Required runtime unavailable");
  });

  it("rejects weakened credential governance and retry-target retention contracts", () => {
    const contract = readJSON("resources/platform-credential-auth-v1.json");
    contract.challengeContract.defaultMode = "off";
    contract.challengeContract.mandatoryBaseline = false;
    contract.passwordPolicy.rotationAndIncidentGovernance.privilegedResetSetsMustChange = false;
    contract.passwordPolicy.rotationAndIncidentGovernance.migration = "generic Record.Values are allowed";
    contract.messageCenterContract.secureRetryTarget.leaseTTLSeconds = 0;
    contract.messageCenterContract.secureRetryTarget.genericRecordValuesRawTargetAllowed = true;

    const result = runValidator(["--contract", tempFile("platform-credential-auth-v1-", "contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /challengeContract\.defaultMode must be always/);
    assert.match(result.stderr, /challengeContract must keep the mandatory baseline/);
    assert.match(result.stderr, /passwordPolicy\.rotationAndIncidentGovernance must require rehash-on-success and MustChange on privileged reset/);
    assert.match(result.stderr, /passwordPolicy\.rotationAndIncidentGovernance\.migration is incomplete/);
    assert.match(result.stderr, /messageCenterContract\.secureRetryTarget must retain a bounded write-only target lease/);
  });

  it("rejects missing OpenAPI coverage for credential auth and message-center v1 endpoints", () => {
    const openapi = readJSON("resources/generated/openapi.admin.json");
    delete openapi.paths["/api/auth/challenges"];
    delete openapi.paths["/api/auth/challenges/verify"];
    delete openapi.paths["/api/admin/profile/current/password/change"];
    delete openapi.paths["/api/admin/profile/{id}/password/reset"];
    openapi.paths["/api/admin/message-center/deliveries/run"] = {
      post: {
        operationId: "runMessageCenterDeliveries",
        description: "SMTP live supplier integration is claimed",
        "x-platform-runtime": "wrong",
      },
    };

    const result = runValidator(["--openapi-admin", tempFile("platform-credential-auth-v1-openapi-", "openapi.admin.json", openapi)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resources\/generated\/openapi\.admin\.json must include POST \/api\/auth\/challenges/);
    assert.match(result.stderr, /resources\/generated\/openapi\.admin\.json must include POST \/api\/auth\/challenges\/verify/);
    assert.match(result.stderr, /resources\/generated\/openapi\.admin\.json must include POST \/api\/admin\/profile\/current\/password\/change/);
    assert.match(result.stderr, /resources\/generated\/openapi\.admin\.json must include POST \/api\/admin\/profile\/\{id\}\/password\/reset/);
    assert.match(result.stderr, /OpenAPI message-center deliveries run must use notification-delivery-worker-v1 runtime marker/);
    assert.match(result.stderr, /OpenAPI message-center deliveries run must describe Aliyun\/Tencent live SMS SDK adapters plus dry-run\/config validation/);
  });

  it("rejects slider preverification OpenAPI leakage and implementation drift", () => {
    const openapi = readJSON("resources/generated/openapi.admin.json");
    const verifyRequest = openapi.components.schemas.AdminCredentialSliderChallengeVerifyRequest;
    verifyRequest.properties.kind.enum = ["captcha", "slider"];
    verifyRequest.properties.proof.writeOnly = false;
    verifyRequest.properties.clientFingerprint.writeOnly = false;
    const verifyData = openapi.components.schemas.AdminCredentialSliderChallengeVerifyData;
    verifyData.properties.proof = { type: "string" };
    delete openapi.paths["/api/auth/challenges/verify"].post.responses["429"];
    delete openapi.paths["/api/auth/challenges/verify"].post.responses["503"];
    const service = fs
      .readFileSync(path.join(repoRoot, "internal/platform/credentialauth/service.go"), "utf8")
      .replace("return s.checkCredentialChallenge(ctx, input, false)", "return s.checkCredentialChallenge(ctx, input, true)")
      .replace("s.repository.RecordCredentialChallengeFailure", "s.repository.UpsertCredentialChallenge");
    const memoryRepository = fs
      .readFileSync(path.join(repoRoot, "internal/platform/credentialauth/memory_repository.go"), "utf8")
      .replaceAll("challengeTransitionAllowed(", "challengeTransitionAllowedRemoved(");
    const gormRepository = fs
      .readFileSync(path.join(repoRoot, "internal/platform/credentialauth/gorm_repository.go"), "utf8")
      .replaceAll("status = ? AND used_at IS NULL AND expires_at > ?", "status = ? AND expires_at > ?")
      .replaceAll("attempts = attempts + 1", "attempts = 1");
    const httpCredentialAuth = fs
      .readFileSync(path.join(repoRoot, "internal/platform/httpapi/credential_auth.go"), "utf8")
      .replace("kind != credentialauth.ChallengeKindSlider", "kind == credentialauth.ChallengeKindSlider")
      .replace("decoder.DisallowUnknownFields()", "// unknown fields allowed")
      .replace("runtime.Service.CheckCredentialChallenge", "runtime.Service.VerifyCredentialChallenge");
    const adminClient = fs
      .readFileSync(path.join(repoRoot, "admin/src/platform/api/client.ts"), "utf8")
      .replace('request<CredentialChallengeVerifyResult>("/auth/challenges/verify"', 'request<CredentialChallengeVerifyResult>("/auth/challenges/missing"');

    const result = runValidator([
      "--openapi-admin",
      tempFile("platform-credential-auth-v1-openapi-", "openapi.admin.json", openapi),
      "--credential-service",
      tempFile("platform-credential-auth-v1-service-", "service.go", service),
      "--credential-memory-repository",
      tempFile("platform-credential-auth-v1-memory-", "memory_repository.go", memoryRepository),
      "--credential-gorm-repository",
      tempFile("platform-credential-auth-v1-gorm-", "gorm_repository.go", gormRepository),
      "--http-credential-auth",
      tempFile("platform-credential-auth-v1-http-", "credential_auth.go", httpCredentialAuth),
      "--admin-client",
      tempFile("platform-credential-auth-v1-client-", "client.ts", adminClient),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /credential-auth service preverification must not consume a successful challenge/);
    assert.match(result.stderr, /credential-auth service must use repository atomic transition RecordCredentialChallengeFailure/);
    assert.match(result.stderr, /credential-auth memory repository must apply ConsumeCredentialChallenge under one conditional lock/);
    assert.match(result.stderr, /credential-auth GORM repository must condition every challenge transition on enabled, unused and unexpired state/);
    assert.match(result.stderr, /credential-auth GORM repository must atomically increment failures and disable both challenge record types at the threshold/);
    assert.match(result.stderr, /credential-auth HTTP challenge preverification must accept only slider/);
    assert.match(result.stderr, /credential-auth HTTP challenge preverification must reject unknown request fields/);
    assert.match(result.stderr, /credential-auth HTTP challenge preverification must use the non-consuming service check/);
    assert.match(result.stderr, /Admin API client must call the slider challenge preverification endpoint/);
    assert.match(result.stderr, /OpenAPI AdminCredentialSliderChallengeVerifyRequest must accept only slider/);
    assert.match(result.stderr, /OpenAPI slider challenge proof and clientFingerprint must be write-only/);
    assert.match(result.stderr, /OpenAPI AdminCredentialSliderChallengeVerifyData must expose only required boolean valid/);
    assert.match(result.stderr, /OpenAPI \/api\/auth\/challenges\/verify must declare rate-limit and rate-limit-runtime responses/);
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
