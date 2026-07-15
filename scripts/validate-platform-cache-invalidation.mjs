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

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-cache-invalidation.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-engineering-capabilities.json"));
const serverPath = path.resolve(repoRoot, argValue("--server", "internal/platform/httpapi/server.go"));

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

function displayPath(filePath) {
  const relative = path.relative(repoRoot, filePath);
  return relative && !relative.startsWith("..") ? relative : filePath;
}

function extractBraceBlock(source, marker) {
  const markerIndex = source.indexOf(marker);
  if (markerIndex === -1) return null;
  const openingIndex = source.indexOf("{", markerIndex + marker.length);
  if (openingIndex === -1) return null;

  let depth = 0;
  let quote = "";
  let escaped = false;
  let lineComment = false;
  let blockComment = false;
  for (let index = openingIndex; index < source.length; index += 1) {
    const current = source[index];
    const next = source[index + 1];
    if (lineComment) {
      if (current === "\n") lineComment = false;
      continue;
    }
    if (blockComment) {
      if (current === "*" && next === "/") {
        blockComment = false;
        index += 1;
      }
      continue;
    }
    if (quote) {
      if (quote === "`") {
        if (current === "`") quote = "";
        continue;
      }
      if (escaped) {
        escaped = false;
        continue;
      }
      if (current === "\\") {
        escaped = true;
        continue;
      }
      if (current === quote) quote = "";
      continue;
    }
    if (current === "/" && next === "/") {
      lineComment = true;
      index += 1;
      continue;
    }
    if (current === "/" && next === "*") {
      blockComment = true;
      index += 1;
      continue;
    }
    if (current === '"' || current === "'" || current === "`") {
      quote = current;
      continue;
    }
    if (current === "{") {
      depth += 1;
      continue;
    }
    if (current === "}") {
      depth -= 1;
      if (depth === 0) {
        return source.slice(openingIndex + 1, index);
      }
    }
  }
  return null;
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const item of required) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function validateTaskGraph(contract, taskGraph, errors) {
  const taskID = contract.taskGraph?.taskId;
  const task = values(taskGraph.tasks).find((item) => item.id === taskID);
  if (!task) {
    errors.push(`task graph must include ${taskID}`);
    return;
  }
  if (task.status !== contract.taskGraph?.requiredStatus) {
    errors.push(`${taskID} status must stay ${contract.taskGraph?.requiredStatus}`);
  }
}

function validateCacheTargets(contract, errors) {
  const targets = new Map(values(contract.cacheTargets).map((target) => [target.id, target]));
  for (const targetID of ["branding", "principal", "menus", "schemas", "permissions", "auth-providers"]) {
    if (!targets.has(targetID)) {
      errors.push(`cacheTargets must include ${targetID}`);
    }
  }
  if (targets.get("branding")?.key !== "admin:branding") {
    errors.push("cache target branding key must stay admin:branding");
  }
  if (targets.get("principal")?.keyPrefix !== "admin:principal:") {
    errors.push("cache target principal keyPrefix must stay admin:principal:");
  }
  if (targets.get("menus")?.keyPrefix !== "admin:menus:") {
    errors.push("cache target menus keyPrefix must stay admin:menus:");
  }
  if (targets.get("schemas")?.keyPrefix !== "admin:schema:") {
    errors.push("cache target schemas keyPrefix must stay admin:schema:");
  }
  if (targets.get("permissions")?.key !== "admin:permissions:list") {
    errors.push("cache target permissions key must stay admin:permissions:list");
  }
  if (targets.get("auth-providers")?.key !== "admin:auth-providers") {
    errors.push("cache target auth-providers key must stay admin:auth-providers");
  }
}

function validateInvalidationRules(contract, errors) {
  const rules = new Map(values(contract.invalidationRules).map((rule) => [rule.resource, rule]));
  const requiredRules = {
    settings: ["delete:admin:branding"],
    branding: ["delete:admin:branding"],
    menus: ["deletePrefix:admin:menus:"],
    users: ["invalidatePolicyAuthorizer", "deletePrefix:admin:menus:", "deletePrefix:admin:principal:"],
    roles: ["invalidatePolicyAuthorizer", "deletePrefix:admin:menus:", "deletePrefix:admin:principal:"],
    permissions: ["invalidatePolicyAuthorizer", "deletePrefix:admin:menus:", "deletePrefix:admin:principal:", "delete:admin:permissions:list", "deletePrefix:admin:schema:"],
    sessions: ["reloadSessionRepository"],
  };
  for (const [resource, operations] of Object.entries(requiredRules)) {
    if (!rules.has(resource)) {
      errors.push(`invalidationRules must include ${resource}`);
      continue;
    }
    requireIncludes(rules.get(resource).operations, operations, `invalidationRules.${resource}.operations`, errors);
  }
}

function validateBusPolicy(contract, errors) {
  const policy = contract.busPolicy ?? {};
  if (policy.publishEmptyResource !== "ignore") {
    errors.push("busPolicy.publishEmptyResource must stay ignore");
  }
  if (policy.localInvalidationBeforePublish !== true) {
    errors.push("busPolicy.localInvalidationBeforePublish must stay true");
  }
  if (policy.redisChannel !== "platform:cache:invalidations") {
    errors.push("busPolicy.redisChannel must stay platform:cache:invalidations");
  }
  if (policy.sessionEventResource !== "sessions") {
    errors.push("busPolicy.sessionEventResource must stay sessions");
  }
  if (policy.sessionEventBehavior !== "reloadSessionRepository") {
    errors.push("busPolicy.sessionEventBehavior must stay reloadSessionRepository");
  }
  if (policy.adminEventBehavior !== "reloadAdminResourceStoreBeforeDerivedCacheInvalidation") {
    errors.push("busPolicy.adminEventBehavior must stay reloadAdminResourceStoreBeforeDerivedCacheInvalidation");
  }
  if (policy.adminReloadFailureBehavior !== "preserveDerivedCaches") {
    errors.push("busPolicy.adminReloadFailureBehavior must stay preserveDerivedCaches");
  }
  if (policy.businessCapabilitiesMustNotImportRedis !== true) {
    errors.push("busPolicy.businessCapabilitiesMustNotImportRedis must stay true");
  }
}

function validateNoCachePolicy(contract, errors) {
  requireIncludes(
    contract.noCachePolicy?.forbiddenTargets,
    ["raw tokens", "passwords", "WeChat secrets", "OAuth codes", "audit writes", "unpersisted write payloads"],
    "noCachePolicy.forbiddenTargets",
    errors,
  );
  if (contract.noCachePolicy?.sourceOfTruthRequired !== true) {
    errors.push("noCachePolicy.sourceOfTruthRequired must stay true");
  }
  if (contract.noCachePolicy?.clearInvalidationPointRequired !== true) {
    errors.push("noCachePolicy.clearInvalidationPointRequired must stay true");
  }
}

function validateAdminInvalidationCallback(server, serverLabel, errors) {
  const subscription = extractBraceBlock(server, "func (s *Server) subscribeInvalidations()");
  const callback = subscription && extractBraceBlock(subscription, "func(ctx context.Context, event cache.InvalidationEvent)");
  if (!callback) {
    errors.push(`${serverLabel} must define the invalidation callback inside subscribeInvalidations`);
    return;
  }

  const reloadMarker = "if err := s.resources.Reload(); err != nil";
  const invalidationMarker = "s.invalidateCachesForResourceLocal(ctx, event.Resource)";
  const reloadIndex = callback.indexOf(reloadMarker);
  const invalidationIndex = callback.indexOf(invalidationMarker);
  if (reloadIndex === -1 || invalidationIndex === -1 || reloadIndex > invalidationIndex) {
    errors.push("admin invalidation callback must reload resources before derived cache invalidation");
  }

  if (reloadIndex !== -1) {
    const reloadErrorGuard = extractBraceBlock(callback, reloadMarker);
    const normalizedReloadErrorGuard = reloadErrorGuard?.trim().replace(/\s+/g, " ");
    const returnsWithoutInvalidation =
      normalizedReloadErrorGuard === "return" &&
      !reloadErrorGuard.includes(invalidationMarker);
    if (!returnsWithoutInvalidation) {
      errors.push("admin invalidation callback must preserve derived caches when resource reload fails");
    }
  }
}

function validateRuntimeEvidence(contract, errors) {
  for (const relativePath of [...values(contract.runtimeEvidence), ...values(contract.validators), ...values(contract.tests)]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`cache invalidation evidence path is missing or unsafe: ${relativePath}`);
    }
  }

  const serverLabel = displayPath(serverPath);
  if (fs.existsSync(serverPath)) {
    const server = fs.readFileSync(serverPath, "utf8");
    for (const [pattern, label] of [
      [/cacheKeyBranding\s*=\s*"admin:branding"/, 'cacheKeyBranding = "admin:branding"'],
      [/cacheKeyAuthProviders\s*=\s*"admin:auth-providers"/, 'cacheKeyAuthProviders = "admin:auth-providers"'],
      [/cacheKeyMenusPrefix\s*=\s*"admin:menus:"/, 'cacheKeyMenusPrefix = "admin:menus:"'],
      [/cacheKeyPrincipalPrefix\s*=\s*"admin:principal:"/, 'cacheKeyPrincipalPrefix = "admin:principal:"'],
      [/cacheKeySchemaPrefix\s*=\s*"admin:schema:"/, 'cacheKeySchemaPrefix = "admin:schema:"'],
      [/cacheKeyPermissionsList\s*=\s*"admin:permissions:list"/, 'cacheKeyPermissionsList = "admin:permissions:list"'],
      [/sessionInvalidationResource\s*=\s*"sessions"/, 'sessionInvalidationResource = "sessions"'],
      [/case [^:\n]*"roles",\s*"permissions",\s*"users":/, 'authorization cache invalidation case including roles, permissions and users'],
    ]) {
      if (!pattern.test(server)) {
        errors.push(`${serverLabel} must include cache invalidation runtime evidence ${label}`);
      }
    }
    for (const snippet of [
      "s.invalidateCachesForResourceLocal(ctx, resource)",
      "PublishInvalidation(ctx, cache.InvalidationEvent{Resource: resource})",
      "PublishInvalidation(ctx, cache.InvalidationEvent{Resource: sessionInvalidationResource})",
      "s.sessions.Reload()",
      "s.resources.Reload()",
      'case "settings", "branding":',
      "_ = s.cache.Delete(ctx, cacheKeyBranding)",
      'case "menus":',
      "_ = s.cache.DeletePrefix(ctx, cacheKeyMenusPrefix)",
      "s.invalidatePolicyAuthorizer()",
      "_ = s.cache.DeletePrefix(ctx, cacheKeyPrincipalPrefix)",
      "_ = s.cache.Delete(ctx, cacheKeyPermissionsList)",
      "_ = s.cache.DeletePrefix(ctx, cacheKeySchemaPrefix)",
    ]) {
      if (!server.includes(snippet)) {
        errors.push(`${serverLabel} must include cache invalidation runtime evidence ${snippet}`);
      }
    }
    validateAdminInvalidationCallback(server, serverLabel, errors);
  } else {
    errors.push(`cache invalidation server source is missing: ${serverLabel}`);
  }

  const invalidationPath = "internal/platform/cache/invalidation.go";
  if (relativeExistingPath(invalidationPath)) {
    const invalidation = readRelativeFile(invalidationPath);
    for (const snippet of ["type InvalidationBus interface", "strings.TrimSpace(event.Resource) == \"\"", "handlers := append([]InvalidationHandler(nil), b.handlers...)"]) {
      if (!invalidation.includes(snippet)) {
        errors.push(`${invalidationPath} must include invalidation bus evidence ${snippet}`);
      }
    }
  }

  const redisPath = "internal/platform/cache/redis_invalidation.go";
  if (relativeExistingPath(redisPath)) {
    const redis = readRelativeFile(redisPath);
    for (const snippet of ['const defaultInvalidationChannel = "platform:cache:invalidations"', "client.Publish(ctx, b.channel, encoded).Err()", "b.client.Subscribe(ctx, b.channel)"]) {
      if (!redis.includes(snippet)) {
        errors.push(`${redisPath} must include Redis invalidation evidence ${snippet}`);
      }
    }
  }

  const cacheDocPath = "docs/platform-cache.md";
  if (relativeExistingPath(cacheDocPath)) {
    const cacheDoc = readRelativeFile(cacheDocPath);
    for (const phrase of [
      "Redis pub/sub invalidation events on `platform:cache:invalidations`",
      "session issue, renewal or revoke: publish `sessions`",
      "Redis is an optimization, not the source of truth",
    ]) {
      if (!cacheDoc.includes(phrase)) {
        errors.push(`${cacheDocPath} must document cache invalidation phrase: ${phrase}`);
      }
    }
  }
}

function validateEngineeringMatrix(contract, matrix, errors) {
  const capability = values(matrix.capabilities).find((item) => item.id === contract.engineeringCapability);
  if (!capability) {
    errors.push(`engineering matrix must include ${contract.engineeringCapability}`);
    return;
  }
  if (capability.status !== "implemented") {
    errors.push(`${contract.engineeringCapability} status must stay implemented`);
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-cache-invalidation.json")) {
    errors.push(`${contract.engineeringCapability} must cite resources/platform-cache-invalidation.json`);
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-cache-invalidation.mjs")) {
    errors.push(`${contract.engineeringCapability} must cite validate-platform-cache-invalidation.mjs`);
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-cache-invalidation.test.mjs")) {
    errors.push(`${contract.engineeringCapability} must cite platform-cache-invalidation.test.mjs`);
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const taskGraph = readJSON(taskGraphPath);
  const matrix = readJSON(matrixPath);
  const errors = [];

  if (!contract.purpose) {
    errors.push("cache invalidation contract purpose is required");
  }
  validateTaskGraph(contract, taskGraph, errors);
  validateCacheTargets(contract, errors);
  validateInvalidationRules(contract, errors);
  validateBusPolicy(contract, errors);
  validateNoCachePolicy(contract, errors);
  validateRuntimeEvidence(contract, errors);
  validateEngineeringMatrix(contract, matrix, errors);

  return { errors };
}

const { errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated platform cache invalidation in ${path.relative(repoRoot, contractPath)}`);
