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
const mainGoPath = path.resolve(repoRoot, argValue("--main-go", "cmd/platform-api/main.go"));

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
    ["challengeId", "kind", "purpose", "answerDigest", "expiresAt", "attempts", "usedAt", "clientFingerprintHash"],
  ],
  ["sms_otp_challenges", ["challengeId", "phoneHash", "codeDigest", "expiresAt", "attempts", "messageId", "usedAt"]],
]);

const requiredEndpoints = new Set([
  "GET /api/auth/providers",
  "POST /api/auth/challenges",
  "POST /api/auth/sms-otp/start",
  "POST /api/auth/login",
]);

const requiredDocs = [
  "docs/platform-auth.md",
  "docs/platform-capability-development.md",
];

const requiredValidators = ["scripts/validate-platform-credential-auth-v1.mjs"];
const requiredTests = ["scripts/platform-credential-auth-v1.test.mjs"];
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
  if (boundary.status !== "contract-only") {
    errors.push("runtimeBoundary.status must stay contract-only");
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
  requireIncludes(challenge.modes, ["off", "always", "after-failure", "risk-based"], "challengeContract.modes", errors);
  if (challenge.defaultMode !== "after-failure") {
    errors.push("challengeContract.defaultMode must be after-failure");
  }
  if (challenge.defaultKind !== "slider") {
    errors.push("challengeContract.defaultKind must be slider");
  }
  if (challenge.proofPersistence !== "digest-only") {
    errors.push("challengeContract.proofPersistence must be digest-only");
  }
  if (challenge.singleUse !== true) {
    errors.push("challengeContract.singleUse must be true");
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

  const sms = contract.smsOtpPolicy ?? {};
  if (sms.transactionEndpoint !== "POST /api/auth/sms-otp/start") {
    errors.push("smsOtpPolicy.transactionEndpoint must be POST /api/auth/sms-otp/start");
  }
  if (sms.digestOnly !== true || sms.singleUse !== true) {
    errors.push("smsOtpPolicy must stay digest-only and single-use");
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
}

function validateAPIContract(contract, errors) {
  const api = contract.apiContract ?? {};
  if (api.status !== "specified-not-implemented") {
    errors.push("apiContract.status must stay specified-not-implemented for this work package");
  }
  if (api.providerDriven !== true) {
    errors.push("apiContract.providerDriven must be true");
  }
  if (api.frontEndMustNotHardCodeProviderModes !== true) {
    errors.push("apiContract.frontEndMustNotHardCodeProviderModes must be true");
  }
  const endpoints = new Set(values(api.endpoints).map((item) => `${item.method} ${item.path}`));
  for (const endpoint of requiredEndpoints) {
    if (!endpoints.has(endpoint)) {
      errors.push(`apiContract.endpoints must include ${endpoint}`);
    }
  }
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
      "PLATFORM_CREDENTIAL_AUTH_CHALLENGE_ENABLED",
    ],
    "configurationContract.featureFlags",
    errors,
  );
  requireIncludes(
    config.securityKeys,
    [
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
  for (const id of ["B-backend-repositories-services", "C-notification-sms-adapters", "D-auth-api-compatibility", "E-admin-login-ui", "F-security-governance"]) {
    if (packages.get(id)?.status !== "remaining") {
      errors.push(`implementationPackages.${id} must remain remaining in this first work package`);
    }
  }
}

function validateDocs(authDoc, capabilityDoc, errors) {
  const authSnippets = [
    ["Credential Auth v1", "docs/platform-auth.md must document credential-auth v1"],
    ["resources/platform-credential-auth-v1.json", "docs/platform-auth.md must point to the credential-auth v1 contract"],
    ["does not change the current demo/OIDC runtime", "docs/platform-auth.md must state current demo/OIDC runtime is unchanged"],
    ["password credentials must not be stored in generic `Record.Values`", "docs/platform-auth.md must forbid password credentials in generic Record.Values"],
    ["notification` SMS channel", "docs/platform-auth.md must assign SMS delivery to notification"],
  ];
  for (const [snippet, message] of authSnippets) {
    if (!authDoc.includes(snippet)) {
      errors.push(message);
    }
  }

  const capabilitySnippets = [
    ["credential-auth capability rules", "docs/platform-capability-development.md must document credential-auth capability rules"],
    ["resources/platform-credential-auth-v1.json", "docs/platform-capability-development.md must point to the credential-auth v1 contract"],
    ["Do not declare provider kind `password`", "docs/platform-capability-development.md must keep provider kind password blocked until implementation"],
    ["rtk node scripts/validate-platform-credential-auth-v1.mjs", "docs/platform-capability-development.md must document the credential-auth validator"],
  ];
  for (const [snippet, message] of capabilitySnippets) {
    if (!capabilityDoc.includes(snippet)) {
      errors.push(message);
    }
  }
}

const errors = [];
const contract = readJSON(contractPath);
const mainGo = readText(mainGoPath);
const authDoc = readText(authDocPath);
const capabilityDoc = readText(capabilityDocPath);

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
validateAPIContract(contract, errors);
validateConfiguration(contract, errors);
validateEvidenceWiring(contract, errors);
validateDocs(authDoc, capabilityDoc, errors);

if (errors.length > 0) {
  console.error(errors.join("\n"));
  process.exit(1);
}

console.log("Validated platform credential-auth v1 contract.");
