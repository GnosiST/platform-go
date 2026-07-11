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

const boundaryPath = path.resolve(repoRoot, argValue("--boundary", "resources/platform-admin-api-boundary.json"));
const openAPIPath = path.resolve(repoRoot, argValue("--admin-openapi", "resources/generated/openapi.admin.json"));
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
  if (!start?.responses?.["501"] || !login?.responses?.["501"]) {
    errors.push("admin OpenAPI auth operations must declare the missing resolver 501 response");
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
  if (boundary.querySecurity?.sensitiveFieldsAllowed !== false) {
    errors.push("querySecurity.sensitiveFieldsAllowed must stay false");
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
  return { boundary, errors };
}

const { boundary, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated admin API boundary and query security in ${path.relative(repoRoot, boundaryPath)} (${boundary.querySecurity?.transport})`);
