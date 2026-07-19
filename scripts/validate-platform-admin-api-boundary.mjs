import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { loadPlatformErrorContract, validateOpenAPIErrorContract } from "./platform-error-contract.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const boundaryPath = path.resolve(repoRoot, argValue("--boundary", "resources/platform-admin-api-boundary.json"));
const openAPIPath = path.resolve(repoRoot, argValue("--admin-openapi", "resources/generated/openapi.admin.json"));
const errorContractPath = path.resolve(repoRoot, argValue("--error-contract", "resources/generated/platform-error-code-contract.json"));
const allowedExtensions = new Set([".js", ".jsx", ".mjs", ".ts", ".tsx"]);

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function readRelativeFile(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function normalizeRelative(value) {
  return value.split(path.sep).join("/");
}

function shouldScan(relativePath, absolutePath, ignoredFiles) {
  if (!allowedExtensions.has(path.extname(absolutePath))) {
    return false;
  }
  if (ignoredFiles.has(relativePath)) {
    return false;
  }
  if (relativePath.includes("/__tests__/") || relativePath.includes(".test.") || relativePath.includes(".spec.")) {
    return false;
  }
  return true;
}

function* walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const childPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      yield* walk(childPath);
      continue;
    }
    if (entry.isFile()) {
      yield childPath;
    }
  }
}

function validateRequiredSnippets(boundary, errors) {
  for (const snippet of values(boundary.requiredSourceSnippets)) {
    const sourcePath = snippet.path ?? "";
    if (!relativeExistingPath(sourcePath)) {
      errors.push(`required source snippet path is missing or unsafe: ${sourcePath || "<missing>"}`);
      continue;
    }
    if (!snippet.contains) {
      errors.push(`required source snippet ${sourcePath} must declare contains`);
      continue;
    }
    const source = readRelativeFile(sourcePath);
    if (!source.includes(snippet.contains)) {
      errors.push(`${sourcePath} is missing required snippet ${snippet.contains}`);
    }
  }
}

function validateAdminSourceBoundary(boundary, errors) {
  const sourceRoot = boundary.adminSourceBoundary?.root ?? "admin/src";
  if (!relativeExistingPath(sourceRoot)) {
    errors.push(`admin source root is missing or unsafe: ${sourceRoot}`);
    return;
  }

  const rootPath = path.resolve(repoRoot, sourceRoot);
  const allowedDirectFetchFiles = new Set(values(boundary.adminSourceBoundary?.allowedDirectFetchFiles));
  const ignoredFiles = new Set(values(boundary.adminSourceBoundary?.ignoredFiles));
  const forbiddenLinePatterns = [
    { name: "app API absolute path", pattern: /\/api\/app(?:\/|[\s'"`?#)]|$)/i },
    { name: "app API relative path", pattern: /(^|[\s'"`(])\/app(?:\/|[\s'"`?#)]|$)/i },
    { name: "encoded app API path", pattern: /(?:%2f|%5c)(?:api(?:%2f|%5c))?app(?:%2f|%5c|$)/i },
    { name: "query-string collection filters", pattern: /\?filters|\bfilters\[/i },
  ];

  for (const absolutePath of walk(rootPath)) {
    const relativePath = normalizeRelative(path.relative(repoRoot, absolutePath));
    if (!shouldScan(relativePath, absolutePath, ignoredFiles)) {
      continue;
    }
    const allowedDirectFetch = allowedDirectFetchFiles.has(relativePath);
    const source = fs.readFileSync(absolutePath, "utf8");
    source.split(/\r?\n/).forEach((line, index) => {
      for (const { name, pattern } of forbiddenLinePatterns) {
        if (pattern.test(line)) {
          errors.push(`${relativePath}:${index + 1} contains ${name}`);
        }
      }
      if (!allowedDirectFetch && /\bfetch\s*\(/.test(line)) {
        errors.push(`${relativePath}:${index + 1} calls fetch() outside the platform API client`);
      }
      if (!allowedDirectFetch && /\bAuthorization\s*:/.test(line)) {
        errors.push(`${relativePath}:${index + 1} manages Authorization outside the platform API client`);
      }
    });
  }
}

function validateOpenAPIQueryContract(errors) {
  if (!fs.existsSync(openAPIPath)) {
    errors.push(`admin OpenAPI file is missing: ${path.relative(repoRoot, openAPIPath)}`);
    return;
  }
  const openAPI = readJSON(openAPIPath);
  const querySchema = openAPI.components?.schemas?.AdminQueryRequest;
  if (!querySchema?.description?.includes("raw input must never be concatenated into SQL")) {
    errors.push("AdminQueryRequest must document that raw input is never concatenated into SQL");
  }
  if (querySchema?.additionalProperties !== false) {
    errors.push("AdminQueryRequest must forbid additionalProperties");
  }

  const queryPaths = Object.entries(openAPI.paths ?? {}).filter(([apiPath]) => /\/api\/admin\/resources\/[^/]+\/query$/.test(apiPath));
  if (queryPaths.length === 0) {
    errors.push("admin OpenAPI must expose resource query POST paths");
  }
  for (const [apiPath, methods] of queryPaths) {
    const post = methods.post;
    if (!post) {
      errors.push(`${apiPath} must use POST for resource queries`);
      continue;
    }
    const schema = post.requestBody?.content?.["application/json"]?.schema;
    if (!Array.isArray(schema?.["x-platform-allowed-fields"])) {
      errors.push(`${apiPath} query schema must declare x-platform-allowed-fields`);
    }
    if (!Array.isArray(schema?.["x-platform-filter-fields"])) {
      errors.push(`${apiPath} query schema must declare x-platform-filter-fields`);
    }
    if (!Array.isArray(schema?.["x-platform-sort-fields"])) {
      errors.push(`${apiPath} query schema must declare x-platform-sort-fields`);
    }
  }
}

function validateAdminOIDCOpenAPIContract(errors) {
  if (!fs.existsSync(openAPIPath)) {
    return;
  }
  const openAPI = readJSON(openAPIPath);
  const start = openAPI.paths?.["/api/auth/providers/{provider}/start"]?.post;
  const login = openAPI.paths?.["/api/auth/login"]?.post;
  if (start?.operationId !== "startAdminAuthProvider" || !Array.isArray(start?.security) || start.security.length !== 0) {
    errors.push("admin OpenAPI must expose the public Admin OIDC start operation");
  }
  if (login?.operationId !== "adminAuthLogin" || !Array.isArray(login?.security) || login.security.length !== 0) {
    errors.push("admin OpenAPI must expose the public Admin login exchange operation");
  }
  const smsOTPStart = openAPI.paths?.["/api/auth/sms-otp/start"]?.post;
  if (smsOTPStart?.operationId !== "startAdminCredentialSMSOTP" || !Array.isArray(smsOTPStart?.security) || smsOTPStart.security.length !== 0) {
    errors.push("admin OpenAPI must expose the public credential-auth SMS OTP start operation");
  }
  if (smsOTPStart?.["x-platform-runtime-status"] !== "persistent-runtime-p0") {
    errors.push("credential-auth SMS OTP OpenAPI operation must declare the persistent P0 runtime status");
  }
  const credentialSecretKey = openAPI.paths?.["/api/auth/credential-secret-key"]?.get;
  if (credentialSecretKey?.operationId !== "getAdminCredentialSecretKey" || !Array.isArray(credentialSecretKey?.security) || credentialSecretKey.security.length !== 0) {
    errors.push("admin OpenAPI must expose the public credential-auth secret key operation");
  }
  if (credentialSecretKey?.["x-platform-runtime-status"] !== "persistent-runtime-p0") {
    errors.push("credential-auth secret key OpenAPI operation must declare the persistent P0 runtime status");
  }
  if (!start?.responses?.["501"] || !login?.responses?.["501"] || !smsOTPStart?.responses?.["501"] || !credentialSecretKey?.responses?.["501"]) {
    errors.push("admin OpenAPI auth operations must declare the missing resolver 501 response");
  }
  const settingsRuntime = openAPI.paths?.["/api/admin/settings"]?.get;
  if (settingsRuntime?.operationId !== "getAdminSettingsRuntime" || settingsRuntime?.["x-platform-runtime"] !== "settings-runtime-v1.1") {
    errors.push("admin OpenAPI must expose the settings runtime v1.1 aggregation operation");
  }
  if (settingsRuntime?.["x-platform-permission"] !== "admin:settings:read") {
    errors.push("settings runtime read operation must require admin:settings:read");
  }
  const settingsUpdate = openAPI.paths?.["/api/admin/settings/{resource}/{id}"]?.put;
  if (settingsUpdate?.operationId !== "updateAdminSettingsResource" || settingsUpdate?.["x-platform-runtime"] !== "settings-runtime-v1.1") {
    errors.push("admin OpenAPI must expose the settings runtime v1.1 update operation");
  }
  if (settingsUpdate?.["x-platform-permission"] !== "admin:settings:update") {
    errors.push("settings runtime update operation must require admin:settings:update");
  }
  if (settingsUpdate?.requestBody?.content?.["application/json"]?.schema?.$ref !== "#/components/schemas/AdminSettingsUpdateRequest") {
    errors.push("settings runtime update operation must use AdminSettingsUpdateRequest");
  }
  const settingsItem = openAPI.components?.schemas?.AdminSettingsResourceItem;
  if (settingsItem?.["x-platform-secret-projection"] !== "response projection must mask or omit protected provider secrets") {
    errors.push("AdminSettingsResourceItem must declare masked/omitted secret projection");
  }
  const settingsRequired = settingsItem?.required ?? [];
  for (const required of ["runtimeApplyMode", "restartRequired", "pendingRestart"]) {
    if (!Array.isArray(settingsRequired) || !settingsRequired.includes(required)) {
      errors.push(`AdminSettingsResourceItem must require ${required}`);
    }
  }
  const settingsMutationRequired = openAPI.components?.schemas?.AdminSettingsMutationData?.required ?? [];
  for (const required of ["restartRequired", "pendingRestart"]) {
    if (!Array.isArray(settingsMutationRequired) || !settingsMutationRequired.includes(required)) {
      errors.push(`AdminSettingsMutationData must require ${required}`);
    }
  }
  const settingsValidate = openAPI.paths?.["/api/admin/settings/{resource}/{id}/validate-config"]?.post;
  if (settingsValidate?.operationId !== "validateAdminSettingsResourceConfig" || settingsValidate?.["x-platform-runtime"] !== "settings-runtime-v1.1") {
    errors.push("admin OpenAPI must expose the settings runtime v1.1 validate-config operation");
  }
  if (settingsValidate?.["x-platform-permission"] !== "admin:settings:update") {
    errors.push("settings validate-config operation must require admin:settings:update");
  }
  if (settingsValidate?.responses?.["200"]?.content?.["application/json"]?.schema?.properties?.data?.$ref !== "#/components/schemas/AdminSettingsValidationData") {
    errors.push("settings validate-config operation must return AdminSettingsValidationData");
  }
  const settingsTestConnect = openAPI.paths?.["/api/admin/settings/{resource}/{id}/test-connect"]?.post;
  if (settingsTestConnect?.operationId !== "testConnectAdminSettingsResource" || settingsTestConnect?.["x-platform-runtime"] !== "settings-runtime-v1.1") {
    errors.push("admin OpenAPI must expose the settings runtime v1.1 test-connect operation");
  }
  if (settingsTestConnect?.["x-platform-permission"] !== "admin:settings:update") {
    errors.push("settings test-connect operation must require admin:settings:update");
  }
  if (settingsTestConnect?.responses?.["200"]?.content?.["application/json"]?.schema?.properties?.data?.$ref !== "#/components/schemas/AdminSettingsTestConnectionData") {
    errors.push("settings test-connect operation must return AdminSettingsTestConnectionData");
  }
  const messageCenterTestSend = openAPI.paths?.["/api/admin/message-center/test-send"]?.post;
  if (messageCenterTestSend?.operationId !== "testSendMessageCenter" || messageCenterTestSend?.["x-platform-runtime"] !== "notification-message-center-test-send-v1") {
    errors.push("admin OpenAPI must expose the message-center test-send runtime operation");
  }
  if (messageCenterTestSend?.["x-platform-permission"] !== "admin:message-center:update") {
    errors.push("message-center test-send operation must require admin:message-center:update");
  }
  if (messageCenterTestSend?.requestBody?.content?.["application/json"]?.schema?.$ref !== "#/components/schemas/AdminMessageCenterTestSendRequest") {
    errors.push("message-center test-send operation must use AdminMessageCenterTestSendRequest");
  }
  const unavailableCodes = messageCenterTestSend?.responses?.["503"]?.["x-platform-error-codes"] ?? [];
  if (!Array.isArray(unavailableCodes) || !unavailableCodes.includes("ADMIN_MESSAGE_CENTER_UNAVAILABLE")) {
    errors.push("message-center test-send operation must declare ADMIN_MESSAGE_CENTER_UNAVAILABLE for unavailable runtime");
  }
  const messageCenterRequest = openAPI.components?.schemas?.AdminMessageCenterTestSendRequest;
  if (
    messageCenterRequest?.properties?.recipient?.writeOnly !== true ||
    messageCenterRequest?.properties?.recipient?.["x-platform-response-policy"] !== "redacted target only" ||
    messageCenterRequest?.properties?.templateParams?.writeOnly !== true
  ) {
    errors.push("AdminMessageCenterTestSendRequest must keep recipient and template params write-only with redacted response policy");
  }
  const messageCenterReceipt = openAPI.components?.schemas?.AdminMessageCenterDeliveryReceipt;
  if (messageCenterReceipt?.["x-platform-secret-projection"] !== "only redacted target and provider receipt metadata are returned") {
    errors.push("AdminMessageCenterDeliveryReceipt must declare redacted target projection");
  }
  const receiptRequired = messageCenterReceipt?.required ?? [];
  if (!Array.isArray(receiptRequired) || !receiptRequired.includes("channel")) {
    errors.push("AdminMessageCenterDeliveryReceipt must include channel in required fields");
  }
  const startRequest = openAPI.components?.schemas?.AdminAuthProviderStartRequest;
  const challenge = startRequest?.properties?.codeChallenge;
  if (
    startRequest?.additionalProperties !== false ||
    !Array.isArray(startRequest?.required) ||
    !startRequest.required.includes("codeChallenge") ||
    challenge?.minLength !== 43 ||
    challenge?.maxLength !== 43 ||
    challenge?.pattern !== "^[A-Za-z0-9_-]{43}$"
  ) {
    errors.push("AdminAuthProviderStartRequest must require an exact S256 codeChallenge");
  }
  const loginRequest = openAPI.components?.schemas?.AdminAuthLoginRequest;
  if (
    loginRequest?.additionalProperties !== false ||
    !loginRequest?.properties?.state ||
    !loginRequest?.properties?.codeVerifier ||
    loginRequest.properties.state.writeOnly !== true ||
    loginRequest.properties.codeVerifier.writeOnly !== true
  ) {
    errors.push("AdminAuthLoginRequest must declare write-only state and codeVerifier exchange fields");
  }
  if (loginRequest?.properties?.code?.writeOnly !== true) {
    errors.push("AdminAuthLoginRequest code must stay writeOnly");
  }
  if (
    loginRequest?.properties?.identifier?.$ref !== "#/components/schemas/AdminCredentialAuthIdentifier" ||
    loginRequest?.properties?.secret?.$ref !== "#/components/schemas/AdminCredentialAuthSecret" ||
    loginRequest?.properties?.challenge?.$ref !== "#/components/schemas/AdminCredentialAuthChallengeProof"
  ) {
    errors.push("AdminAuthLoginRequest must include structured credential-auth identifier, secret and challenge fields");
  }
  const credentialSecret = openAPI.components?.schemas?.AdminCredentialAuthSecret;
  if (
    credentialSecret?.additionalProperties !== false ||
    !credentialSecret?.properties?.type?.enum?.includes("password") ||
    !credentialSecret?.properties?.type?.enum?.includes("sms-otp") ||
    credentialSecret.properties?.value?.writeOnly !== true ||
    credentialSecret.properties?.code?.writeOnly !== true ||
    credentialSecret.properties?.encrypted?.$ref !== "#/components/schemas/AdminCredentialAuthSecretEnvelope"
  ) {
    errors.push("AdminCredentialAuthSecret must constrain password and SMS OTP secrets as encrypted write-only request fields");
  }
  const credentialEnvelope = openAPI.components?.schemas?.AdminCredentialAuthSecretEnvelope;
  if (
    credentialEnvelope?.additionalProperties !== false ||
    !credentialEnvelope?.required?.includes("clientPublicKey") ||
    !credentialEnvelope?.required?.includes("ciphertext") ||
    credentialEnvelope?.properties?.ciphertext?.writeOnly !== true
  ) {
    errors.push("AdminCredentialAuthSecretEnvelope must require client public key and write-only ciphertext");
  }
  const credentialKey = openAPI.components?.schemas?.AdminCredentialAuthSecretKeyData;
  if (
    credentialKey?.additionalProperties !== false ||
    !credentialKey?.required?.includes("publicKey") ||
    !credentialKey?.required?.includes("expiresAt") ||
    !credentialKey?.properties?.algorithm?.enum?.includes("ECDH-P256-HKDF-SHA256+A256GCM")
  ) {
    errors.push("AdminCredentialAuthSecretKeyData must expose ECDH/AES public key metadata");
  }
  const smsStartRequest = openAPI.components?.schemas?.AdminCredentialSMSOTPStartRequest;
  const smsStartData = openAPI.components?.schemas?.AdminCredentialSMSOTPStartData;
  if (
    smsStartRequest?.additionalProperties !== false ||
    !smsStartRequest?.required?.includes("provider") ||
    !smsStartRequest?.required?.includes("identifier") ||
    smsStartRequest?.properties?.identifier?.$ref !== "#/components/schemas/AdminCredentialAuthIdentifier"
  ) {
    errors.push("AdminCredentialSMSOTPStartRequest must require provider and structured phone identifier");
  }
  if (
    smsStartData?.additionalProperties !== false ||
    !smsStartData?.required?.includes("transactionId") ||
    !smsStartData?.required?.includes("maskedIdentifier") ||
    !smsStartData?.required?.includes("expiresAt") ||
    smsStartData?.properties?.debugCode?.["x-platform-development-only"] !== true ||
    smsStartData?.properties?.debugCode?.["x-platform-sensitivity"] !== "secret"
  ) {
    errors.push("AdminCredentialSMSOTPStartData must expose transaction metadata and mark debugCode development-only secret");
  }
  const startData = openAPI.components?.schemas?.AdminAuthProviderStartData;
  const responseFields = Object.keys(startData?.properties ?? {});
  const allowedResponseFields = new Set(["authorizationUrl", "state", "expiresAt"]);
  for (const field of responseFields) {
    if (!allowedResponseFields.has(field)) {
      errors.push(`AdminAuthProviderStartData must not expose sensitive response field ${field}`);
    }
  }
  if (
    startData?.additionalProperties !== false ||
    responseFields.length !== allowedResponseFields.size ||
    !responseFields.every((field) => allowedResponseFields.has(field))
  ) {
    errors.push("AdminAuthProviderStartData must expose only authorizationUrl, state and expiresAt");
  }
}

function validateBoundaryPolicy(boundary, errors) {
  if (boundary.reference?.promotionDecision !== "foundation-gate") {
    errors.push("admin API boundary promotionDecision must stay foundation-gate");
  }
  if (boundary.querySecurity?.transport !== "POST /api/admin/resources/:resource/query") {
    errors.push("querySecurity.transport must stay POST /api/admin/resources/:resource/query");
  }
  if (boundary.querySecurity?.payload !== "structured-json") {
    errors.push("querySecurity.payload must stay structured-json");
  }
  if (boundary.querySecurity?.rawSQLAllowed !== false) {
    errors.push("querySecurity.rawSQLAllowed must stay false");
  }
  if (boundary.querySecurity?.fieldWhitelistSource !== "resource schema") {
    errors.push("querySecurity.fieldWhitelistSource must stay resource schema");
  }
  if (boundary.querySecurity?.sensitivityPolicySource !== "capability manifest") {
    errors.push("querySecurity.sensitivityPolicySource must stay capability manifest");
  }
  if (boundary.querySecurity?.fieldNameInferenceAllowed !== false) {
    errors.push("querySecurity.fieldNameInferenceAllowed must stay false");
  }
  if (boundary.querySecurity?.encryptedFieldQueryPolicy !== "declared-blind-index-exact-match-only") {
    errors.push("querySecurity.encryptedFieldQueryPolicy must stay declared-blind-index-exact-match-only");
  }
  if (boundary.querySecurity?.encryptedFieldSortAllowed !== false) {
    errors.push("querySecurity.encryptedFieldSortAllowed must stay false");
  }
  if (boundary.querySecurity?.sortWhitelistRequired !== true) {
    errors.push("querySecurity.sortWhitelistRequired must stay true");
  }
  if (boundary.querySecurity?.maxValueLength !== 256) {
    errors.push("querySecurity.maxValueLength must stay 256");
  }
  for (const validator of values(boundary.requiredValidators)) {
    if (!relativeExistingPath(validator)) {
      errors.push(`admin API boundary required validator is missing: ${validator}`);
    }
  }
  for (const test of values(boundary.requiredTests)) {
    if (!relativeExistingPath(test)) {
      errors.push(`admin API boundary required test is missing: ${test}`);
    }
  }
}

function validate() {
  const boundary = readJSON(boundaryPath);
  const errors = [];
  if (!boundary.purpose) {
    errors.push("admin API boundary purpose is required");
  }
  validateBoundaryPolicy(boundary, errors);
  validateRequiredSnippets(boundary, errors);
  validateAdminSourceBoundary(boundary, errors);
  validateOpenAPIQueryContract(errors);
  validateAdminOIDCOpenAPIContract(errors);
  if (fs.existsSync(openAPIPath)) {
    errors.push(...validateOpenAPIErrorContract(readJSON(openAPIPath), loadPlatformErrorContract(repoRoot, errorContractPath), {
      label: "admin OpenAPI",
      planes: ["admin"],
    }));
  }
  return { boundary, errors };
}

const { boundary, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated admin API boundary and query security in ${path.relative(repoRoot, boundaryPath)} (${boundary.querySecurity?.transport})`);
