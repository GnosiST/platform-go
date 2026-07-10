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

const boundaryPath = path.resolve(repoRoot, argValue("--boundary", "resources/platform-app-client-api-boundary.json"));
const appRouteContractPath = path.resolve(repoRoot, argValue("--app-route-contract", "resources/generated/app-route-contract.json"));
const appOpenAPIPath = path.resolve(repoRoot, argValue("--app-openapi", "resources/generated/openapi.app.json"));
const appCodegenPreviewPath = path.resolve(repoRoot, argValue("--app-codegen-preview", "resources/generated/app-codegen-preview.json"));

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
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

function normalizeRelative(filePath) {
  return path.relative(repoRoot, filePath).split(path.sep).join("/");
}

function readRelativeFile(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function validateBoundaryPolicy(boundary, errors) {
  if (!boundary.purpose) {
    errors.push("app client API boundary purpose is required");
  }
  if (boundary.reference?.promotionDecision !== "foundation-gate") {
    errors.push("reference.promotionDecision must stay foundation-gate");
  }
  const client = boundary.clientBoundary ?? {};
  if (client.contractSource !== "resources/generated/app-route-contract.json") {
    errors.push("clientBoundary.contractSource must stay resources/generated/app-route-contract.json");
  }
  if (client.openapiSource !== "resources/generated/openapi.app.json") {
    errors.push("clientBoundary.openapiSource must stay resources/generated/openapi.app.json");
  }
  if (client.codegenPreview !== "resources/generated/app-codegen-preview.json") {
    errors.push("clientBoundary.codegenPreview must stay resources/generated/app-codegen-preview.json");
  }
  if (client.generatedClientBoundary !== "future-app/src/platform/api/appClient.ts") {
    errors.push("clientBoundary.generatedClientBoundary must stay future-app/src/platform/api/appClient.ts");
  }
  requireIncludes(client.targetClients, ["wechat-miniprogram", "h5", "native-app"], "clientBoundary.targetClients", errors);
  requireIncludes(client.allowedTransportPorts, ["generated-app-client", "appRequest", "appUpload"], "clientBoundary.allowedTransportPorts", errors);
  requireIncludes(
    client.forbiddenPageLevelPatterns,
    ["uni.request", "wx.request", "Taro.request", "uni.uploadFile", "wx.uploadFile", "Taro.uploadFile", "Authorization"],
    "clientBoundary.forbiddenPageLevelPatterns",
    errors,
  );
  requireIncludes(
    client.forbiddenApiTargets,
    ["/api/admin", "/admin", "absolute admin URL", "query-string hand-written paths", "encoded admin API path"],
    "clientBoundary.forbiddenApiTargets",
    errors,
  );
  const token = client.tokenPolicy ?? {};
  if (token.securityDomain !== "app") {
    errors.push("tokenPolicy.securityDomain must stay app");
  }
  if (token.tokenType !== "app") {
    errors.push("tokenPolicy.tokenType must stay app");
  }
  if (token.tokenInjectionOwner !== "generated-app-client-or-app-request-port") {
    errors.push("tokenPolicy.tokenInjectionOwner must stay generated-app-client-or-app-request-port");
  }
  if (token.callerAuthorizationHeaderAllowed !== false) {
    errors.push("tokenPolicy.callerAuthorizationHeaderAllowed must stay false");
  }
  if (token.adminTokenAccepted !== false) {
    errors.push("tokenPolicy.adminTokenAccepted must stay false");
  }
  if (token.apiTokenAccepted !== false) {
    errors.push("tokenPolicy.apiTokenAccepted must stay false");
  }
  const upload = client.uploadPolicy ?? {};
  if (upload.uploadPort !== "appUpload") {
    errors.push("uploadPolicy.uploadPort must stay appUpload");
  }
  if (upload.pageLevelUploadAllowed !== false) {
    errors.push("uploadPolicy.pageLevelUploadAllowed must stay false");
  }
  if (upload.uploadTargetPrefix !== "/api/app") {
    errors.push("uploadPolicy.uploadTargetPrefix must stay /api/app");
  }
}

function validateRequiredPaths(boundary, errors) {
  for (const relativePath of [
    ...values(boundary.requiredGeneratedEvidence),
    ...values(boundary.requiredValidators),
    ...values(boundary.requiredTests),
  ]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`app client API boundary path is missing or unsafe: ${relativePath}`);
    }
  }
  for (const snippet of values(boundary.requiredSourceSnippets)) {
    const sourcePath = snippet.path ?? "";
    const contains = snippet.contains ?? "";
    if (!relativeExistingPath(sourcePath)) {
      errors.push(`required source snippet path is missing or unsafe: ${sourcePath || "<missing>"}`);
      continue;
    }
    if (!contains) {
      errors.push(`required source snippet ${sourcePath} must declare contains`);
      continue;
    }
    if (!readRelativeFile(sourcePath).includes(contains)) {
      errors.push(`${sourcePath} is missing required snippet ${contains}`);
    }
  }
}

function validateAppRouteContract(contract, errors) {
  if (contract.source !== "capability.Manifest.App.Routes") {
    errors.push("app route contract source must stay capability.Manifest.App.Routes");
  }
  for (const route of values(contract.routes)) {
    const label = `${route.method ?? "<method>"} ${route.path ?? "<path>"}`;
    if (!String(route.path ?? "").startsWith("/api/app/")) {
      errors.push(`app route contract path must stay under /api/app: ${label}`);
    }
    if (!["public", "session"].includes(route.auth)) {
      errors.push(`app route contract ${label} auth must be public or session`);
    }
    if (route.permission && !String(route.permission).startsWith("app:")) {
      errors.push(`app route contract ${label} permission must use app: prefix`);
    }
  }
}

function validateAppOpenAPI(openapi, errors) {
  const scheme = openapi.components?.securitySchemes?.appBearerAuth;
  if (!scheme || scheme.type !== "http" || scheme.scheme !== "bearer") {
    errors.push("app OpenAPI must declare appBearerAuth security scheme");
  }
  for (const [apiPath, methods] of Object.entries(openapi.paths ?? {})) {
    if (!apiPath.startsWith("/api/app/")) {
      errors.push(`app OpenAPI path must stay under /api/app: ${apiPath}`);
      continue;
    }
    for (const [method, operation] of Object.entries(methods ?? {})) {
      const auth = operation?.["x-platform-auth"];
      if (auth === "session") {
        const security = JSON.stringify(operation.security ?? []);
        if (!security.includes("appBearerAuth")) {
          errors.push(`app OpenAPI ${method.toUpperCase()} ${apiPath} must use appBearerAuth for session routes`);
        }
      }
      if (auth === "public" && values(operation.security).length !== 0) {
        errors.push(`app OpenAPI ${method.toUpperCase()} ${apiPath} public routes must not require security`);
      }
    }
  }
}

function validateAppCodegenPreview(preview, expectedClientFile, errors) {
  if (preview.source !== "resources/generated/app-route-contract.json") {
    errors.push("app codegen preview source must stay resources/generated/app-route-contract.json");
  }
  if (preview.securityDomain !== "app") {
    errors.push("app codegen preview securityDomain must stay app");
  }
  const guardrails = values(preview.guardrails);
  if (!guardrails.some((guardrail) => guardrail.toLowerCase().includes("generated clients must stay behind the app api boundary"))) {
    errors.push("app codegen preview guardrails must include generated clients must stay behind the app API boundary");
  }
  if (!guardrails.some((guardrail) => guardrail.includes("tokenType=app"))) {
    errors.push("app codegen preview guardrails must include tokenType=app");
  }
  for (const route of values(preview.routes)) {
    const operationID = route.operationId ?? `${route.method ?? ""} ${route.path ?? ""}`;
    if (!String(route.path ?? "").startsWith("/api/app/")) {
      errors.push(`app codegen route ${operationID} path must stay under /api/app`);
    }
    if (route.securityDomain !== "app") {
      errors.push(`app codegen route ${operationID} securityDomain must stay app`);
    }
    if (route.client?.apiClientFile !== expectedClientFile) {
      errors.push(`app codegen route ${operationID} client.apiClientFile must stay ${expectedClientFile}`);
    }
    const expectedTokenType = route.auth === "public" ? "none" : "app";
    if (route.client?.tokenType !== expectedTokenType) {
      errors.push(`app codegen route ${operationID} client.tokenType must be ${expectedTokenType}`);
    }
  }
}

function validate() {
  const boundary = readJSON(boundaryPath);
  const appRouteContract = readJSON(appRouteContractPath);
  const appOpenAPI = readJSON(appOpenAPIPath);
  const appCodegenPreview = readJSON(appCodegenPreviewPath);
  const errors = [];

  validateBoundaryPolicy(boundary, errors);
  validateRequiredPaths(boundary, errors);
  validateAppRouteContract(appRouteContract, errors);
  validateAppOpenAPI(appOpenAPI, errors);
  validateAppCodegenPreview(appCodegenPreview, boundary.clientBoundary?.generatedClientBoundary, errors);

  return { boundary, errors };
}

const { boundary, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated app client API boundary in ${normalizeRelative(boundaryPath)} (${boundary.clientBoundary?.generatedClientBoundary})`);
