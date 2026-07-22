import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-credential-auth-v1.json"));
const authDocPath = path.resolve(repoRoot, argValue("--auth-doc", "docs/platform-auth.md"));
const capabilityDocPath = path.resolve(repoRoot, argValue("--capability-doc", "docs/platform-capability-development.md"));
const dataGovernanceDocPath = path.resolve(repoRoot, argValue("--data-governance-doc", "docs/platform-data-governance-and-integrations-assessment.md"));
const openapiAdminPath = path.resolve(repoRoot, argValue("--openapi-admin", "resources/generated/openapi.admin.json"));
const mainGoPath = path.resolve(repoRoot, argValue("--main-go", "cmd/platform-api/main.go"));
const httpCredentialAuthPath = path.resolve(repoRoot, argValue("--http-credential-auth", "internal/platform/httpapi/credential_auth.go"));
const credentialServicePath = path.resolve(repoRoot, argValue("--credential-service", "internal/platform/credentialauth/service.go"));
const adminClientPath = path.resolve(repoRoot, argValue("--admin-client", "admin/src/platform/api/client.ts"));
const adminLoginViewPath = path.resolve(repoRoot, argValue("--admin-login-view", "admin/src/platform/auth/AdminLoginView.tsx"));

const requiredProviderModes = new Map([
  ["username-password", { identifierType: "username", secretType: "password", configKey: "PLATFORM_CREDENTIAL_AUTH_USERNAME_PASSWORD" }],
  ["phone-password", { identifierType: "phone", secretType: "password", configKey: "PLATFORM_CREDENTIAL_AUTH_PHONE_PASSWORD" }],
  ["email-password", { identifierType: "email", secretType: "password", configKey: "PLATFORM_CREDENTIAL_AUTH_EMAIL_PASSWORD" }],
  ["phone-sms-otp", { identifierType: "phone", secretType: "sms-otp", configKey: "PLATFORM_CREDENTIAL_AUTH_PHONE_SMS_OTP" }],
]);

const requiredStorageContracts = new Map([
  [
    "auth_identifiers",
    ["principalType", "principalId", "identifierType", "identifierHash", "maskedIdentifier", "verifiedAt", "status"],
  ],
  [
    "password_credentials",
    [
      "principalType",
      "principalId",
      "passwordHash",
      "algorithm",
      "paramsVersion",
      "passwordUpdatedAt",
      "mustChange",
      "failedAttempts",
      "lockedUntil",
      "status",
    ],
  ],
  [
    "credential_challenges",
    ["challengeId", "kind", "purpose", "answerDigest", "expiresAt", "attempts", "usedAt", "clientFingerprintHash", "status"],
  ],
  ["sms_otp_challenges", ["challengeId", "phoneHash", "codeDigest", "expiresAt", "attempts", "messageId", "usedAt", "status"]],
]);

const requiredEndpoints = new Set([
  "GET /api/auth/providers",
  "GET /api/auth/credential-secret-key",
  "POST /api/auth/challenges",
  "POST /api/auth/sms-otp/start",
  "POST /api/auth/login",
  "POST /api/admin/profile/current/password/change",
  "POST /api/admin/profile/{id}/password/reset",
  "POST /api/admin/message-center/deliveries/run",
  "POST /api/admin/message-center/deliveries/{id}/retry",
]);

const requiredOpenAPIPaths = new Map([
  ["/api/auth/challenges", "post"],
  ["/api/admin/profile/current/password/change", "post"],
  ["/api/admin/profile/{id}/password/reset", "post"],
  ["/api/admin/message-center/deliveries/run", "post"],
]);

const requiredDocs = [
  "docs/platform-auth.md",
  "docs/platform-capability-development.md",
  "docs/platform-data-governance-and-integrations-assessment.md",
];

const requiredValidators = ["scripts/validate-platform-credential-auth-v1.mjs"];
const requiredTests = ["scripts/platform-credential-auth-v1.test.mjs"];
const requiredBackendFiles = [
  "internal/platform/credentialauth/types.go",
  "internal/platform/credentialauth/normalizer.go",
  "internal/platform/credentialauth/hmac_identifier.go",
  "internal/platform/credentialauth/memory_repository.go",
  "internal/platform/credentialauth/gorm_repository.go",
  "internal/platform/credentialauth/service.go",
  "internal/platform/credentialauth/argon2id.go",
  "internal/platform/credentialauth/secret_transport.go",
  "internal/platform/credentialauth/service_test.go",
  "internal/platform/credentialauth/argon2id_test.go",
  "internal/platform/credentialauth/gorm_repository_test.go",
  "internal/platform/credentialauth/secret_transport_test.go",
];
const requiredRuntimeFiles = [
  "internal/platform/httpapi/credential_auth.go",
  "internal/platform/bootstrap/credential_auth.go",
  "admin/src/platform/auth/AdminLoginView.tsx",
  "admin/src/platform/api/client.ts",
];
const requiredNotificationSMSFiles = [
  "internal/platform/notification/sms.go",
  "internal/platform/notification/sms_test.go",
];
const requiredAcceptanceCommands = [
  "rtk node scripts/validate-platform-credential-auth-v1.mjs",
  "rtk node --test scripts/platform-credential-auth-v1.test.mjs",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function readText(filePath) {
  return fs.readFileSync(filePath, "utf8");
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function byID(items) {
  return new Map(values(items).map((item) => [item.id, item]));
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const item of required) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function uniqueErrors(items, label) {
  const errors = [];
  const seen = new Set();
  for (const item of values(items)) {
    if (seen.has(item)) {
      errors.push(`${label} contains duplicate value ${item}`);
    }
    seen.add(item);
  }
  return errors;
}

function validateRuntimeBoundary(contract, mainGo, errors) {
  const boundary = contract.runtimeBoundary ?? {};
  if (boundary.status !== "deliverable-v1") {
    errors.push("runtimeBoundary.status must be deliverable-v1 after challenge, password mutation and message-center delivery runtime work");
  }
  if (boundary.defaultRuntimeMutation !== "forbidden") {
    errors.push("runtimeBoundary.defaultRuntimeMutation must stay forbidden");
  }
  if (boundary.passwordProviderKindRuntimeAccepted !== false) {
    errors.push("runtimeBoundary.passwordProviderKindRuntimeAccepted must stay false");
  }
  if (boundary.existingPasswordProviderGuardMustRemain !== true) {
    errors.push("existing password provider guard must remain active");
  }
  if (boundary.productionComplete !== false) {
    errors.push("runtimeBoundary.productionComplete must stay false until risk policy hardening, real-vendor SMS evidence and full governance evidence are complete");
  }
  if (!String(boundary.devRuntimeStorage ?? "").includes("in-memory credential-auth repository")) {
    errors.push("runtimeBoundary.devRuntimeStorage must document the in-memory development repository fallback");
  }
  if (!String(boundary.v1RuntimeStorage ?? "").includes("dedicated GORM credential-auth repository")) {
    errors.push("runtimeBoundary.v1RuntimeStorage must document the dedicated GORM credential-auth repository");
  }
  const secretTransport = String(boundary.secretTransport ?? "");
  for (const snippet of [
    "application-layer hybrid encryption",
    "GET /api/auth/credential-secret-key",
    "ECDH P-256",
    "HKDF-SHA256",
    "AES-256-GCM/A256GCM",
    "password or SMS OTP secrets",
  ]) {
    if (!secretTransport.includes(snippet)) {
      errors.push(`runtimeBoundary.secretTransport must document ${snippet}`);
    }
  }
  if (!secretTransport.includes("secret.encrypted")) {
    errors.push("runtimeBoundary.secretTransport must document secret.encrypted envelopes");
  }
  if (!secretTransport.includes("HTTPS remains a production baseline") || !secretTransport.includes("not the credential secret encryption mechanism")) {
    errors.push("runtimeBoundary.secretTransport must state HTTPS is only a production baseline and not the credential secret encryption mechanism");
  }
  if (!secretTransport.includes("cannot be used as a substitute for secret.encrypted")) {
    errors.push("runtimeBoundary.secretTransport must state HTTPS cannot substitute secret.encrypted");
  }
  if (!String(boundary.productionEnablementGate ?? "").includes("repository driver/dsn")) {
    errors.push("runtimeBoundary.productionEnablementGate must require repository driver/dsn and encrypted secret transport keys");
  }
  requireIncludes(
    boundary.mustNotChange,
    ["existing demo provider runtime", "existing Admin OIDC runtime", "existing WeChat/app provider runtime"],
    "runtimeBoundary.mustNotChange",
    errors,
  );
  if (!/kind\s*==\s*"password"/.test(mainGo)) {
    errors.push('cmd/platform-api/main.go must still reject provider kind password');
  }
  if (!mainGo.includes("local password provider requires a separately approved Argon2id capability")) {
    errors.push("cmd/platform-api/main.go must keep the separately approved Argon2id capability error");
  }
}

function validateCapabilityBoundary(contract, errors) {
  const boundary = contract.capabilityBoundary ?? {};
  if (contract.id !== "credential-auth" || boundary.capabilityId !== "credential-auth") {
    errors.push("credential-auth contract id and capabilityBoundary.capabilityId must be credential-auth");
  }
  if (boundary.classification !== "optional-platform-capability") {
    errors.push("capabilityBoundary.classification must be optional-platform-capability");
  }
  if (boundary.businessNeutral !== true) {
    errors.push("capabilityBoundary.businessNeutral must stay true");
  }
  requireIncludes(boundary.dependencies, ["identity", "session", "rbac", "audit", "notification"], "capabilityBoundary.dependencies", errors);
  requireIncludes(
    boundary.owns,
    [
      "local credential provider declarations",
      "identifier normalization and hash lookup",
      "password credential verification policy",
      "login challenge transaction policy",
      "password change and reset contracts",
      "SMS OTP transaction policy",
    ],
    "capabilityBoundary.owns",
    errors,
  );
  requireIncludes(
    boundary.doesNotOwn,
    [
      "platform user/person identity source",
      "JWT signing or server-side session issuance",
      "post-login RBAC authorization",
      "audit ledger persistence",
      "SMS vendor transport implementation",
    ],
    "capabilityBoundary.doesNotOwn",
    errors,
  );
  const notificationExtension = values(boundary.extendsCapabilities).find(
    (item) => item.capability === "notification" && item.extension === "sms-channel",
  );
  if (!notificationExtension) {
    errors.push("capabilityBoundary.extendsCapabilities must include notification sms-channel");
  }
}

function validateProviderModes(contract, errors) {
  const providerModes = byID(contract.providerModes);
  for (const [id, expected] of requiredProviderModes.entries()) {
    const provider = providerModes.get(id);
    if (!provider) {
      errors.push(`providerModes must include ${id}`);
      continue;
    }
    for (const [key, value] of Object.entries(expected)) {
      if (provider[key] !== value) {
        errors.push(`providerMode ${id}.${key} must be ${value}`);
      }
    }
    if (provider.requiresChallengeDecision !== true) {
      errors.push(`providerMode ${id}.requiresChallengeDecision must be true`);
    }
  }
  errors.push(...uniqueErrors(values(contract.providerModes).map((item) => item.id), "providerModes"));
}

function validateGenericValuesPolicy(contract, errors) {
  const policy = contract.genericRecordValuesPolicy ?? {};
  for (const key of ["passwordCredentialStorageAllowed", "challengeAnswerStorageAllowed", "smsOtpStorageAllowed"]) {
    if (policy[key] !== false) {
      errors.push(`genericRecordValuesPolicy.${key} must stay false`);
    }
  }
  requireIncludes(
    policy.forbiddenCredentialFields,
    ["password", "plainPassword", "passwordHash", "smsOtp", "verificationCode", "captchaAnswer", "challengeAnswer"],
    "genericRecordValuesPolicy.forbiddenCredentialFields",
    errors,
  );
  if (!String(policy.allowedGenericUse ?? "").includes("credential secrets and challenge proofs require dedicated stores")) {
    errors.push("genericRecordValuesPolicy.allowedGenericUse must require dedicated stores for credential secrets and challenge proofs");
  }
}

function validateStorageContracts(contract, errors) {
  const contracts = byID(contract.storageContracts);
  for (const [id, requiredFields] of requiredStorageContracts.entries()) {
    const storage = contracts.get(id);
    if (!storage) {
      errors.push(`storageContracts must include ${id}`);
      continue;
    }
    if (storage.genericRecordValuesAllowed !== false) {
      errors.push(`storage ${id} must forbid generic Record.Values`);
    }
    if (storage.rawSecretPersistenceAllowed !== false) {
      errors.push(`storage ${id} must forbid raw secret persistence`);
    }
    requireIncludes(storage.requiredFields, requiredFields, `storage ${id}.requiredFields`, errors);
    if (id === "password_credentials") {
      if (storage.algorithm !== "argon2id") {
        errors.push("storage password_credentials.algorithm must be argon2id");
      }
      if (storage.paramsVersionEnv !== "PLATFORM_CREDENTIAL_AUTH_ARGON2_PARAMS_VERSION") {
        errors.push("storage password_credentials.paramsVersionEnv must be PLATFORM_CREDENTIAL_AUTH_ARGON2_PARAMS_VERSION");
      }
    }
  }
  errors.push(...uniqueErrors(values(contract.storageContracts).map((item) => item.id), "storageContracts"));
}

function validateChallengeAndSecretPolicies(contract, errors) {
  const challenge = contract.challengeContract ?? {};
  requireIncludes(challenge.kinds, ["captcha", "slider"], "challengeContract.kinds", errors);
  if (!Array.isArray(challenge.modes) || challenge.modes.length !== 1 || challenge.modes[0] !== "always") {
    errors.push("challengeContract.modes must be the mandatory always baseline");
  }
  if (challenge.defaultMode !== "always") {
    errors.push("challengeContract.defaultMode must be always");
  }
  if (challenge.defaultKind !== "captcha") {
    errors.push("challengeContract.defaultKind must be captcha");
  }
  if (challenge.proofPersistence !== "digest-only") {
    errors.push("challengeContract.proofPersistence must be digest-only");
  }
  if (challenge.singleUse !== true) {
    errors.push("challengeContract.singleUse must be true");
  }
  if (challenge.disableOnMaxAttempts !== true) {
    errors.push("challengeContract.disableOnMaxAttempts must be true");
  }
  if (challenge.sliderProofTolerancePixels !== 3) {
    errors.push("challengeContract.sliderProofTolerancePixels must be 3");
  }
  if (challenge.clientFingerprintPersistence !== "digest-only") {
    errors.push("challengeContract.clientFingerprintPersistence must be digest-only");
  }
  if (challenge.mandatoryBaseline !== true || !String(challenge.riskRouting ?? "").includes("valid single-use challenge") || !String(challenge.riskRouting ?? "").includes("Risk-based routing is not exposed")) {
    errors.push("challengeContract must keep the mandatory baseline and defer risk routing without an approved adapter");
  }

  const password = contract.passwordPolicy ?? {};
  if (password.algorithm !== "argon2id") {
    errors.push("passwordPolicy.algorithm must be argon2id");
  }
  if (password.rawPasswordPersistenceAllowed !== false) {
    errors.push("passwordPolicy.rawPasswordPersistenceAllowed must stay false");
  }
  if (password.rehashOnParamsUpgrade !== true) {
    errors.push("passwordPolicy.rehashOnParamsUpgrade must be true");
  }
  if (password.changeEndpoint !== "POST /api/admin/profile/current/password/change") {
    errors.push("passwordPolicy.changeEndpoint must be POST /api/admin/profile/current/password/change");
  }
  if (password.resetEndpoint !== "POST /api/admin/profile/{id}/password/reset") {
    errors.push("passwordPolicy.resetEndpoint must be POST /api/admin/profile/{id}/password/reset");
  }
  if (password.secretTransportRequired !== true) {
    errors.push("passwordPolicy.secretTransportRequired must stay true");
  }
  const governance = password.rotationAndIncidentGovernance ?? {};
  if (governance.rehashOnSuccessfulParamsUpgrade !== true || governance.privilegedResetSetsMustChange !== true) {
    errors.push("passwordPolicy.rotationAndIncidentGovernance must require rehash-on-success and MustChange on privileged reset");
  }
  for (const [key, snippets] of Object.entries({
    breachResponse: ["disable affected credentials", "revoke impacted sessions", "raw passwords"],
    migration: ["dedicated credential tables", "parameter version", "must not rewrite generic Record.Values"],
    promotionEvidence: ["independent security", "operations approval", "migration rehearsal evidence"],
  })) {
    if (!snippets.every((snippet) => String(governance[key] ?? "").includes(snippet))) {
      errors.push(`passwordPolicy.rotationAndIncidentGovernance.${key} is incomplete`);
    }
  }

  const sms = contract.smsOtpPolicy ?? {};
  if (sms.transactionEndpoint !== "POST /api/auth/sms-otp/start") {
    errors.push("smsOtpPolicy.transactionEndpoint must be POST /api/auth/sms-otp/start");
  }
  if (sms.digestOnly !== true || sms.singleUse !== true) {
    errors.push("smsOtpPolicy must stay digest-only and single-use");
  }
  if (sms.persistBeforeSend !== true) {
    errors.push("smsOtpPolicy.persistBeforeSend must stay true");
  }
  if (sms.disableOnMaxAttempts !== true) {
    errors.push("smsOtpPolicy.disableOnMaxAttempts must stay true");
  }
  if (sms.sendThroughCapability !== "notification") {
    errors.push("smsOtpPolicy.sendThroughCapability must be notification");
  }
}

function validateNotificationSms(contract, errors) {
  const sms = contract.notificationSmsBoundary ?? {};
  if (sms.capability !== "notification") {
    errors.push("notificationSmsBoundary.capability must be notification");
  }
  if (sms.channel !== "sms") {
    errors.push("notificationSmsBoundary.channel must be sms");
  }
  if (sms.providerEnv !== "PLATFORM_NOTIFICATION_SMS_PROVIDER") {
    errors.push("notificationSmsBoundary.providerEnv must be PLATFORM_NOTIFICATION_SMS_PROVIDER");
  }
  if (sms.loginTemplateEnv !== "PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID") {
    errors.push("notificationSmsBoundary.loginTemplateEnv must be PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID");
  }
  requireIncludes(sms.requiredAdapters, ["aliyun", "tencent", "mock-local"], "notificationSmsBoundary.requiredAdapters", errors);
  if (sms.productionMockProviderAllowed !== false) {
    errors.push("notificationSmsBoundary.productionMockProviderAllowed must stay false");
  }
  if (sms.vendorErrorRedactionRequired !== true) {
    errors.push("notificationSmsBoundary.vendorErrorRedactionRequired must be true");
  }
  if (sms.deliveryLedgerRequired !== true) {
    errors.push("notificationSmsBoundary.deliveryLedgerRequired must be true");
  }
  const adapterContract = String(sms.adapterContract ?? "");
  if (!adapterContract.includes("official SDK clients") || !adapterContract.includes("dry-run/config validation")) {
    errors.push("notificationSmsBoundary.adapterContract must describe official SDK live adapters plus dry-run/config validation");
  }
  if (/SMTP.*live|WeChat.*live/i.test(adapterContract)) {
    errors.push("notificationSmsBoundary.adapterContract must not claim SMTP/WeChat supplier integration");
  }
  if (!String(sms.configurationClosedLoop ?? "").includes("/settings") || !String(sms.configurationClosedLoop ?? "").includes("/message-center")) {
    errors.push("notificationSmsBoundary.configurationClosedLoop must include /settings and /message-center");
  }
}

function validateMessageCenterContract(contract, errors) {
  const messageCenter = contract.messageCenterContract ?? {};
  if (messageCenter.capability !== "notification") {
    errors.push("messageCenterContract.capability must be notification");
  }
  if (messageCenter.workbenchRoute !== "/message-center") {
    errors.push("messageCenterContract.workbenchRoute must be /message-center");
  }
  if (messageCenter.settingsRoute !== "/settings") {
    errors.push("messageCenterContract.settingsRoute must be /settings");
  }
  requireIncludes(
    messageCenter.requiredResources,
    [
      "notification-channels",
      "notification-providers",
      "notification-send-policies",
      "notification-templates",
      "notifications",
      "notification-deliveries",
    ],
    "messageCenterContract.requiredResources",
    errors,
  );
  requireIncludes(
    messageCenter.runtimeEndpoints,
    [
      "POST /api/admin/message-center/test-send",
      "POST /api/admin/message-center/deliveries/run",
      "POST /api/admin/message-center/deliveries/{id}/retry",
    ],
    "messageCenterContract.runtimeEndpoints",
    errors,
  );
  if (messageCenter.deliveryWorker?.manualRunEndpoint !== "POST /api/admin/message-center/deliveries/run") {
    errors.push("messageCenterContract.deliveryWorker.manualRunEndpoint must be POST /api/admin/message-center/deliveries/run");
  }
  const providerTruth = String(messageCenter.deliveryWorker?.providerRuntimeTruth ?? "");
  if (!providerTruth.includes("Aliyun/Tencent live SMS SDK adapters") || !providerTruth.includes("dry-run/config validation")) {
    errors.push("messageCenterContract.deliveryWorker.providerRuntimeTruth must describe Aliyun/Tencent live SMS SDK adapters plus dry-run/config validation");
  }
  if (!providerTruth.includes("in-app, Email and WeChat worker paths use local dry-run receipts")) {
    errors.push("messageCenterContract.deliveryWorker.providerRuntimeTruth must document in-app, Email and WeChat local dry-run worker paths");
  }
  if (!providerTruth.includes("SMTP/WeChat supplier integration is not claimed")) {
    errors.push("messageCenterContract.deliveryWorker.providerRuntimeTruth must not claim SMTP/WeChat supplier integration");
  }
  if (!String(messageCenter.configurationClosedLoop ?? "").includes("Provider configuration") || !String(messageCenter.configurationClosedLoop ?? "").includes("manual run")) {
    errors.push("messageCenterContract.configurationClosedLoop must document provider configuration through manual run closure");
  }
  const retryTarget = messageCenter.secureRetryTarget ?? {};
  if (retryTarget.persistence !== "write-only process-memory lease only" || retryTarget.leaseTTLSeconds !== 7200 || retryTarget.retainUntil !== "delivery succeeds or lease expires" || retryTarget.releaseOn !== "successful delivery ledger mutation" || retryTarget.genericRecordValuesRawTargetAllowed !== false) {
    errors.push("messageCenterContract.secureRetryTarget must retain a bounded write-only target lease without generic raw-target persistence");
  }
  if (!String(retryTarget.restartRecovery ?? "").includes("redacted ledger target") || !String(retryTarget.restartRecovery ?? "").includes("submit the recipient again")) {
    errors.push("messageCenterContract.secureRetryTarget must document restart recovery without raw-target persistence");
  }
}

function validateAPIContract(contract, errors) {
  const api = contract.apiContract ?? {};
  if (api.status !== "deliverable-v1") {
    errors.push("apiContract.status must be deliverable-v1 for the current encrypted credential-auth and message-center v1 slice");
  }
  if (api.providerDriven !== true) {
    errors.push("apiContract.providerDriven must be true");
  }
  if (api.frontEndMustNotHardCodeProviderModes !== true) {
    errors.push("apiContract.frontEndMustNotHardCodeProviderModes must be true");
  }
  const secretTransport = api.secretTransportContract ?? {};
  if (secretTransport.algorithm !== "ECDH-P256-HKDF-SHA256+A256GCM") {
    errors.push("apiContract.secretTransportContract.algorithm must be ECDH-P256-HKDF-SHA256+A256GCM");
  }
  if (secretTransport.keyDiscoveryEndpoint !== "GET /api/auth/credential-secret-key") {
    errors.push("apiContract.secretTransportContract.keyDiscoveryEndpoint must be GET /api/auth/credential-secret-key");
  }
  if (secretTransport.loginEnvelopeField !== "secret.encrypted") {
    errors.push("apiContract.secretTransportContract.loginEnvelopeField must be secret.encrypted");
  }
  requireIncludes(
    secretTransport.plaintextSecretFieldsRejectedWhenRequired,
    ["secret.value", "secret.code"],
    "apiContract.secretTransportContract.plaintextSecretFieldsRejectedWhenRequired",
    errors,
  );
  if (!String(secretTransport.httpsRole ?? "").includes("not the application-layer credential secret encryption mechanism")) {
    errors.push("apiContract.secretTransportContract.httpsRole must state HTTPS is not the application-layer credential secret encryption mechanism");
  }
  if (!String(secretTransport.httpsRole ?? "").includes("cannot be used as a substitute for secret.encrypted")) {
    errors.push("apiContract.secretTransportContract.httpsRole must state HTTPS cannot substitute secret.encrypted");
  }
  const endpoints = new Set(values(api.endpoints).map((item) => `${item.method} ${item.path}`));
  for (const endpoint of requiredEndpoints) {
    if (!endpoints.has(endpoint)) {
      errors.push(`apiContract.endpoints must include ${endpoint}`);
    }
  }
  requireIncludes(
    api.implementedNow,
    [
      "GET /api/auth/providers includes enabled credential-auth provider declarations",
      "GET /api/auth/credential-secret-key exposes short-lived public key metadata for application-layer hybrid encrypted credential secrets",
      "POST /api/auth/challenges creates digest-only CAPTCHA/slider challenge transactions for login",
      "POST /api/auth/sms-otp/start requires a single-use credential challenge, persists the digest-only OTP transaction before sending and disables it on send failure",
      "POST /api/auth/login accepts structured encrypted credential-password and credential-sms-otp requests while preserving demo/OIDC compatibility",
      "Admin login UI renders credential provider modes from discovery, encrypts credential secrets with WebCrypto and closes CAPTCHA/go-captcha slider challenges",
      "credential-auth request paths use rate-limit enforcement, login failure audit and redacted internal error/audit surfaces",
    ],
    "apiContract.implementedNow",
    errors,
  );
  requireIncludes(
    api.notProductionComplete,
    [
      "CAPTCHA and go-captcha slider login UI are wired; credential-auth v1 keeps the approved mandatory challenge baseline while any risk-signal adapter remains a separate reviewed capability",
      "real-vendor SMS delivery evidence against approved Aliyun/Tencent accounts",
      "production promotion evidence",
    ],
    "apiContract.notProductionComplete",
    errors,
  );
  const login = api.loginRequestShape ?? {};
  if (login.provider !== "phone-password" || login.identifier?.type !== "phone" || login.secret?.type !== "password") {
    errors.push("apiContract.loginRequestShape must show structured phone-password identifier and secret");
  }
  const smsLogin = api.smsLoginRequestShape ?? {};
  if (smsLogin.provider !== "phone-sms-otp" || smsLogin.identifier?.type !== "phone" || smsLogin.secret?.type !== "sms-otp") {
    errors.push("apiContract.smsLoginRequestShape must show structured phone-sms-otp identifier and secret");
  }
}

function validateConfiguration(contract, errors) {
  const config = contract.configurationContract ?? {};
  if (!String(config.capabilityEnvExample ?? "").includes("notification,credential-auth")) {
    errors.push("configurationContract.capabilityEnvExample must include notification,credential-auth");
  }
  requireIncludes(
    config.featureFlags,
    [
      "PLATFORM_CREDENTIAL_AUTH_USERNAME_PASSWORD",
      "PLATFORM_CREDENTIAL_AUTH_PHONE_PASSWORD",
      "PLATFORM_CREDENTIAL_AUTH_EMAIL_PASSWORD",
      "PLATFORM_CREDENTIAL_AUTH_PHONE_SMS_OTP",
    ],
    "configurationContract.featureFlags",
    errors,
  );
  requireIncludes(
    config.securityKeys,
    [
      "PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER",
      "PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN",
      "PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY",
      "PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_KEY_ID",
      "PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY",
      "PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_USERNAME",
      "PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PASSWORD",
      "PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PHONE",
      "PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_EMAIL",
      "PLATFORM_CREDENTIAL_AUTH_ARGON2_PARAMS_VERSION",
      "PLATFORM_CREDENTIAL_AUTH_PASSWORD_MAX_ATTEMPTS",
      "PLATFORM_CREDENTIAL_AUTH_LOCK_SECONDS",
      "PLATFORM_AUTH_SMS_OTP_TTL_SECONDS",
      "PLATFORM_AUTH_SMS_OTP_MAX_ATTEMPTS",
      "PLATFORM_NOTIFICATION_SMS_PROVIDER",
      "PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID",
    ],
    "configurationContract.securityKeys",
    errors,
  );
  if (config.productionRejectsMockSms !== true) {
    errors.push("configurationContract.productionRejectsMockSms must be true");
  }
}

function validateEvidenceWiring(contract, errors) {
  for (const docPath of requiredDocs) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`required doc is missing or unsafe: ${docPath}`);
    }
  }
  requireIncludes(contract.docs, requiredDocs, "docs", errors);
  requireIncludes(contract.validators, requiredValidators, "validators", errors);
  requireIncludes(contract.tests, requiredTests, "tests", errors);
  requireIncludes(contract.minimumAcceptanceCommands, requiredAcceptanceCommands, "minimumAcceptanceCommands", errors);
  for (const command of values(contract.minimumAcceptanceCommands)) {
    if (!command.startsWith("rtk ")) {
      errors.push(`minimumAcceptanceCommands must use rtk prefix: ${command}`);
    }
  }
  const packages = byID(contract.implementationPackages);
  const packageA = packages.get("A-contract-docs-validator");
  if (!packageA || !["in-progress", "done"].includes(packageA.status)) {
    errors.push("implementationPackages must keep A-contract-docs-validator tracked as in-progress or done");
  }
  const packageB = packages.get("B-backend-repositories-services");
  if (!packageB || packageB.status !== "done") {
    errors.push("implementationPackages.B-backend-repositories-services must be done after GORM repository and secret transport work");
  }
  if (!String(packageB?.scope ?? "").includes("GORM persistence") || !String(packageB?.scope ?? "").includes("encrypted secret transport")) {
    errors.push("implementationPackages.B-backend-repositories-services scope must point to internal/platform/credentialauth");
  }
  for (const filePath of requiredBackendFiles) {
    if (!relativeExistingPath(filePath)) {
      errors.push(`credential-auth backend service foundation file is missing or unsafe: ${filePath}`);
    }
  }
  for (const filePath of requiredRuntimeFiles) {
    if (!relativeExistingPath(filePath)) {
      errors.push(`credential-auth partial runtime file is missing or unsafe: ${filePath}`);
    }
  }
  const packageC = packages.get("C-notification-sms-adapters");
  if (!packageC || !["in-progress", "done"].includes(packageC.status)) {
    errors.push("implementationPackages must track C-notification-sms-adapters as in-progress or done after SMS port work starts");
  }
  const packageCScope = String(packageC?.scope ?? "");
  for (const snippet of ["internal/platform/notification", "Aliyun/Tencent official SDK-backed live adapters", "approved-account delivery evidence"]) {
    if (!packageCScope.includes(snippet)) {
      errors.push(`implementationPackages.C-notification-sms-adapters scope must document ${snippet}`);
    }
  }
  for (const filePath of requiredNotificationSMSFiles) {
    if (!relativeExistingPath(filePath)) {
      errors.push(`notification SMS foundation file is missing or unsafe: ${filePath}`);
    }
  }
  const packageD = packages.get("D-auth-api-compatibility");
  if (!packageD || packageD.status !== "in-progress") {
    errors.push("implementationPackages.D-auth-api-compatibility must be in-progress for the partial HTTP runtime slice");
  }
  if (
    !String(packageD?.scope ?? "").includes("internal/platform/httpapi") ||
    !String(packageD?.scope ?? "").includes("credential-secret-key") ||
    !String(packageD?.scope ?? "").includes("challenge creation")
  ) {
    errors.push("implementationPackages.D-auth-api-compatibility scope must point to internal/platform/httpapi, credential-secret-key and challenge creation");
  }
  const packageE = packages.get("E-admin-login-ui");
  if (!packageE || packageE.status !== "in-progress") {
    errors.push("implementationPackages.E-admin-login-ui must be in-progress for the provider-driven Admin login UI slice");
  }
  if (!String(packageE?.scope ?? "").includes("provider-discovery driven Admin login form state")) {
    errors.push("implementationPackages.E-admin-login-ui scope must document provider-discovery driven Admin login form state");
  }
  const packageF = packages.get("F-security-governance");
  if (!packageF || packageF.status !== "in-progress") {
    errors.push("implementationPackages.F-security-governance must be in-progress after OpenAPI/rate-limit/audit contract sync starts");
  }
  if (!String(packageF?.scope ?? "").includes("OpenAPI contract coverage") || !String(packageF?.scope ?? "").includes("rate-limit enforcement")) {
    errors.push("implementationPackages.F-security-governance scope must document OpenAPI contract coverage and rate-limit enforcement");
  }
  const packageG = packages.get("G-message-center-delivery");
  if (!packageG || packageG.status !== "in-progress") {
    errors.push("implementationPackages.G-message-center-delivery must be in-progress for message-center delivery v1");
  }
  const packageGScope = String(packageG?.scope ?? "");
  for (const snippet of ["SMS provider configuration", "message delivery worker contract", "manual deliveries run endpoint", "settings configuration loop"]) {
    if (!packageGScope.includes(snippet)) {
      errors.push(`implementationPackages.G-message-center-delivery scope must document ${snippet}`);
    }
  }
}

function validateDocs(authDoc, capabilityDoc, dataGovernanceDoc, errors) {
  const authSnippets = [
    ["Credential Auth v1", "docs/platform-auth.md must document credential-auth v1"],
    ["resources/platform-credential-auth-v1.json", "docs/platform-auth.md must point to the credential-auth v1 contract"],
    ["internal/platform/credentialauth", "docs/platform-auth.md must point to the credential-auth service foundation package"],
    ["deliverable v1 HTTP/UI runtime", "docs/platform-auth.md must document the deliverable v1 HTTP/UI runtime"],
    ["preserves the current demo/OIDC runtime", "docs/platform-auth.md must state current demo/OIDC runtime is preserved"],
    ["password credentials must not be stored in generic `Record.Values`", "docs/platform-auth.md must forbid password credentials in generic Record.Values"],
    ["notification` SMS channel", "docs/platform-auth.md must assign SMS delivery to notification"],
    ["`POST /api/auth/challenges` is implemented in the backend runtime", "docs/platform-auth.md must state POST /api/auth/challenges is implemented in backend runtime"],
    ["password change/reset", "docs/platform-auth.md must document password change/reset contract"],
    ["rate-limit enforcement and redacted audit/error surfaces", "docs/platform-auth.md must document rate-limit and redacted audit/error surfaces"],
    ["always required", "docs/platform-auth.md must document the mandatory challenge baseline"],
    ["rotation/incident/migration governance contract", "docs/platform-auth.md must document password governance"],
    ["write-only process-memory lease for two hours", "docs/platform-auth.md must document bounded retry-target handling"],
    ["message delivery worker", "docs/platform-auth.md must document message delivery worker"],
    ["manual run endpoint", "docs/platform-auth.md must document message-center manual run endpoint"],
    ["application-layer hybrid encryption", "docs/platform-auth.md must document application-layer hybrid encryption"],
    ["GET /api/auth/credential-secret-key", "docs/platform-auth.md must document credential secret key discovery"],
    ["ECDH P-256", "docs/platform-auth.md must document ECDH P-256 key agreement"],
    ["HKDF-SHA256", "docs/platform-auth.md must document HKDF-SHA256 derivation"],
    ["AES-256-GCM/A256GCM", "docs/platform-auth.md must document AES-256-GCM/A256GCM encryption"],
    ["When `RequireEncryptedSecrets=true`, the server rejects plaintext `secret.value` or `secret.code`", "docs/platform-auth.md must document plaintext secret field rejection"],
    ["not the application-layer credential secret encryption mechanism", "docs/platform-auth.md must state HTTPS is not the credential secret encryption mechanism"],
    ["cannot be used as a substitute for `secret.encrypted`", "docs/platform-auth.md must state HTTPS cannot substitute secret.encrypted"],
    ["not a production-complete credential system", "docs/platform-auth.md must state credential-auth is not production-complete"],
  ];
  for (const [snippet, message] of authSnippets) {
    if (!authDoc.includes(snippet)) {
      errors.push(message);
    }
  }

  const capabilitySnippets = [
    ["credential-auth capability rules", "docs/platform-capability-development.md must document credential-auth capability rules"],
    ["resources/platform-credential-auth-v1.json", "docs/platform-capability-development.md must point to the credential-auth v1 contract"],
    ["internal/platform/credentialauth", "docs/platform-capability-development.md must point to the credential-auth service foundation package"],
    ["deliverable-v1", "docs/platform-capability-development.md must document the current v1 runtime status"],
    ["Do not declare provider kind `password`", "docs/platform-capability-development.md must keep provider kind password blocked until implementation"],
    ["`POST /api/auth/challenges` is now a backend runtime endpoint", "docs/platform-capability-development.md must state POST /api/auth/challenges is implemented in backend runtime"],
    ["go-captcha slider material", "docs/platform-capability-development.md must document go-captcha slider UI material"],
    ["digest-only slider X tolerance", "docs/platform-capability-development.md must document digest-only slider X tolerance"],
    ["per-page challenge nonce", "docs/platform-capability-development.md must document per-page challenge nonce binding"],
    ["requires its valid single-use challenge", "docs/platform-capability-development.md must document the mandatory challenge baseline"],
    ["rotation/incident/migration governance contract", "docs/platform-capability-development.md must document password governance"],
    ["write-only process-memory lease", "docs/platform-capability-development.md must document secure retry-target handling"],
    ["password change/reset contract", "docs/platform-capability-development.md must document password change/reset contract"],
    ["message delivery worker contract", "docs/platform-capability-development.md must document message delivery worker contract"],
    ["manual run endpoint `POST /api/admin/message-center/deliveries/run`", "docs/platform-capability-development.md must document message-center delivery run endpoint"],
    ["Aliyun/Tencent live SMS SDK adapters", "docs/platform-capability-development.md must document Aliyun/Tencent live SMS SDK adapters"],
    ["dry-run/config validation", "docs/platform-capability-development.md must document SMS dry-run/config validation"],
    ["ECDH P-256", "docs/platform-capability-development.md must document ECDH P-256 key agreement"],
    ["HKDF-SHA256", "docs/platform-capability-development.md must document HKDF-SHA256 derivation"],
    ["AES-256-GCM/A256GCM", "docs/platform-capability-development.md must document AES-256-GCM/A256GCM encryption"],
    ["When encrypted secrets are required, `secret.value` and `secret.code` are invalid", "docs/platform-capability-development.md must document plaintext secret field rejection"],
    ["not the application-layer credential secret encryption mechanism", "docs/platform-capability-development.md must state HTTPS is not the credential secret encryption mechanism"],
    ["cannot be used as a substitute for `secret.encrypted`", "docs/platform-capability-development.md must state HTTPS cannot substitute secret.encrypted"],
    ["rtk node scripts/validate-platform-credential-auth-v1.mjs", "docs/platform-capability-development.md must document the credential-auth validator"],
  ];
  for (const [snippet, message] of capabilitySnippets) {
    if (!capabilityDoc.includes(snippet)) {
      errors.push(message);
    }
  }

  const dataGovernanceSnippets = [
    ["application-layer hybrid encryption", "docs/platform-data-governance-and-integrations-assessment.md must document application-layer hybrid encryption"],
    ["GET /api/auth/credential-secret-key", "docs/platform-data-governance-and-integrations-assessment.md must document credential secret key discovery"],
    ["ECDH P-256", "docs/platform-data-governance-and-integrations-assessment.md must document ECDH P-256 key agreement"],
    ["HKDF-SHA256", "docs/platform-data-governance-and-integrations-assessment.md must document HKDF-SHA256 derivation"],
    ["AES-256-GCM/A256GCM", "docs/platform-data-governance-and-integrations-assessment.md must document AES-256-GCM/A256GCM encryption"],
    ["HTTPS cannot be used as a substitute for `secret.encrypted`", "docs/platform-data-governance-and-integrations-assessment.md must state HTTPS cannot substitute secret.encrypted"],
  ];
  for (const [snippet, message] of dataGovernanceSnippets) {
    if (!dataGovernanceDoc.includes(snippet)) {
      errors.push(message);
    }
  }
}

function validateChallengeImplementation(credentialService, httpCredentialAuth, adminLoginView, openapi, errors) {
  if (!credentialService.includes("hashChallengeAnswer") || !credentialService.includes("challengeProofDigestSetPrefix")) {
    errors.push("credential-auth service must store slider challenge proof tolerance as a digest-only answer set");
  }
  if (!credentialService.includes("normalizeChallengeProof(challenge.Kind, input.Proof)")) {
    errors.push("credential-auth service must normalize raw challenge proof before hashing");
  }
  if (!credentialService.includes("challenge.Status = StatusDisabled") || !credentialService.includes("s.repository.UpsertSMSOTPChallenge(ctx, challenge)")) {
    errors.push("credential-auth service must disable challenge and SMS OTP records on max-attempt rejection");
  }
  if (!httpCredentialAuth.includes("Proof:                 proof")) {
    errors.push("credential-auth HTTP login must pass raw challenge proof to the service for normalization and tolerance handling");
  }
  if (!httpCredentialAuth.includes("runtime.Service.PutSMSOTPChallenge(ctx.Request.Context(), otpChallenge)") || !httpCredentialAuth.includes("otpChallenge.Status = credentialauth.StatusDisabled")) {
    errors.push("credential-auth HTTP SMS OTP start must persist the OTP digest before send and disable it on send failure");
  }
  if (!httpCredentialAuth.includes("recordCredentialAuthLoginFailure") || !httpCredentialAuth.includes("\"auth.login\"")) {
    errors.push("credential-auth HTTP login must write redacted failure audit events");
  }
  if (!adminLoginView.includes("clientFingerprintRef") || !adminLoginView.includes("clientFingerprint")) {
    errors.push("Admin login view must bind credential challenges with a per-page client fingerprint");
  }
  if (!adminLoginView.includes("loginRateLimited") || !adminLoginView.includes("loginChallengeExpired")) {
    errors.push("Admin login view must expose localized rate-limit and challenge-expiry feedback");
  }
  const challengeProof = openapi.components?.schemas?.AdminCredentialAuthChallengeProof;
  if (challengeProof?.properties?.clientFingerprint?.writeOnly !== true) {
    errors.push("OpenAPI AdminCredentialAuthChallengeProof must include write-only clientFingerprint");
  }
}

function validateSecretTransportImplementation(httpCredentialAuth, adminClient, errors) {
  if (!httpCredentialAuth.includes("credentialAuthPlaintextSecretPresent")) {
    errors.push("internal/platform/httpapi/credential_auth.go must reject plaintext secret fields when encrypted secrets are required");
  }
  if (!httpCredentialAuth.includes("strings.TrimSpace(input.Value) != \"\"") || !httpCredentialAuth.includes("strings.TrimSpace(input.Code) != \"\"")) {
    errors.push("credentialAuthPlaintextSecretPresent must reject both secret.value and secret.code");
  }
  if (!/RequireEncryptedSecrets[\s\S]{0,400}credentialAuthPlaintextSecretPresent/.test(httpCredentialAuth)) {
    errors.push("credential-auth login must call plaintext secret rejection inside RequireEncryptedSecrets handling");
  }
  if (!adminClient.includes("encryptCredentialSecret(input.provider, \"password\"") || !adminClient.includes("encryptCredentialSecret(input.provider, \"sms-otp\"")) {
    errors.push("admin API client must encrypt password and sms-otp secrets before login submission");
  }
  if (!adminClient.includes("secret: { type: \"password\", encrypted }")) {
    errors.push("admin API client must submit password login secrets only as secret.encrypted");
  }
  if (!adminClient.includes("secret: { type: \"sms-otp\", transactionId: secret.transactionId, encrypted }")) {
    errors.push("admin API client must submit sms-otp login secrets only as secret.encrypted plus transactionId");
  }
  for (const snippet of ["ECDH", "P-256", "HKDF", "SHA-256", "AES-GCM"]) {
    if (!adminClient.includes(snippet)) {
      errors.push(`admin API client credential encryption must use ${snippet}`);
    }
  }
}

function validateOpenAPI(openapi, errors) {
  const paths = openapi.paths ?? {};
  for (const [routePath, method] of requiredOpenAPIPaths.entries()) {
    if (!paths[routePath]?.[method]) {
      errors.push(`resources/generated/openapi.admin.json must include ${method.toUpperCase()} ${routePath}`);
    }
  }
  const challenge = paths["/api/auth/challenges"]?.post;
  if (challenge) {
    if (challenge.operationId !== "startAdminCredentialChallenge") {
      errors.push("OpenAPI /api/auth/challenges operationId must be startAdminCredentialChallenge");
    }
    if (challenge["x-platform-runtime-status"] !== "deliverable-v1") {
      errors.push("OpenAPI /api/auth/challenges must keep x-platform-runtime-status deliverable-v1");
    }
  }
  const change = paths["/api/admin/profile/current/password/change"]?.post;
  if (change && change["x-platform-secret-transport"] !== "ECDH-P256-HKDF-SHA256+A256GCM") {
    errors.push("OpenAPI password change must use the credential-auth encrypted secret transport");
  }
  const reset = paths["/api/admin/profile/{id}/password/reset"]?.post;
  if (reset && reset["x-platform-permission"] !== "admin:user:update") {
    errors.push("OpenAPI password reset must require admin:user:update");
  }
  const run = paths["/api/admin/message-center/deliveries/run"]?.post;
  if (run) {
    if (run["x-platform-runtime"] !== "notification-delivery-worker-v1") {
      errors.push("OpenAPI message-center deliveries run must use notification-delivery-worker-v1 runtime marker");
    }
    if (!String(run.description ?? "").includes("Aliyun/Tencent live SMS SDK adapters") || !String(run.description ?? "").includes("dry-run/config validation")) {
      errors.push("OpenAPI message-center deliveries run must describe Aliyun/Tencent live SMS SDK adapters plus dry-run/config validation");
    }
    if (!String(run.description ?? "").includes("In-app, Email and WeChat worker paths use local dry-run receipts")) {
      errors.push("OpenAPI message-center deliveries run must document in-app, Email and WeChat local dry-run worker paths");
    }
    if (/SMTP.*live|WeChat.*live/i.test(String(run.description ?? ""))) {
      errors.push("OpenAPI message-center deliveries run must not claim SMTP/WeChat supplier integration");
    }
  }
}

const errors = [];
const contract = readJSON(contractPath);
const openapiAdmin = readJSON(openapiAdminPath);
const mainGo = readText(mainGoPath);
const authDoc = readText(authDocPath);
const capabilityDoc = readText(capabilityDocPath);
const dataGovernanceDoc = readText(dataGovernanceDocPath);
const httpCredentialAuth = readText(httpCredentialAuthPath);
const credentialService = readText(credentialServicePath);
const adminClient = readText(adminClientPath);
const adminLoginView = readText(adminLoginViewPath);

if (contract.contractVersion !== "0.1.0") {
  errors.push("contractVersion must be 0.1.0");
}
if (contract.$schema !== "https://platform-go.local/schemas/platform-credential-auth-v1.json") {
  errors.push("$schema must point to platform-credential-auth-v1");
}

validateRuntimeBoundary(contract, mainGo, errors);
validateCapabilityBoundary(contract, errors);
validateProviderModes(contract, errors);
validateGenericValuesPolicy(contract, errors);
validateStorageContracts(contract, errors);
validateChallengeAndSecretPolicies(contract, errors);
validateNotificationSms(contract, errors);
validateMessageCenterContract(contract, errors);
validateAPIContract(contract, errors);
validateConfiguration(contract, errors);
validateEvidenceWiring(contract, errors);
validateDocs(authDoc, capabilityDoc, dataGovernanceDoc, errors);
validateChallengeImplementation(credentialService, httpCredentialAuth, adminLoginView, openapiAdmin, errors);
validateSecretTransportImplementation(httpCredentialAuth, adminClient, errors);
validateOpenAPI(openapiAdmin, errors);

if (errors.length > 0) {
  console.error(errors.join("\n"));
  process.exit(1);
}

console.log("Validated platform credential-auth v1 contract.");
