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

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-refresh-token-family-promotion.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const readinessPath = path.resolve(repoRoot, argValue("--production-readiness", "resources/platform-production-readiness.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) return false;
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function validateEvidenceSnippets(items, label, errors) {
  for (const item of values(items)) {
    if (!relativeExistingPath(item.path)) {
      errors.push(`${label} evidence path is missing or unsafe: ${item.path ?? "<missing>"}`);
      continue;
    }
    if (!item.contains) {
      errors.push(`${label} evidence for ${item.path} must declare contains`);
      continue;
    }
    const source = fs.readFileSync(path.resolve(repoRoot, item.path), "utf8");
    if (!source.includes(item.contains)) {
      errors.push(`${label} evidence ${item.path} must include ${item.contains}`);
    }
  }
}

function validateCurrentRuntime(contract, errors) {
  const runtime = contract.currentRuntime ?? {};
  if (runtime.status !== "sliding-renewal-only") {
    errors.push("currentRuntime.status must stay sliding-renewal-only");
  }
  if (runtime.endpoint !== "POST /api/auth/refresh") {
    errors.push("currentRuntime.endpoint must stay POST /api/auth/refresh");
  }
  if (runtime.notARefreshTokenFamily !== true) {
    errors.push("currentRuntime.notARefreshTokenFamily must stay true");
  }
  validateEvidenceSnippets(runtime.sourceEvidence, "currentRuntime", errors);
}

function validateProductionAuthLink(contract, productionAuth, errors) {
  const expected = "resources/platform-production-auth-hardening.json";
  if (contract.productionAuthHardening !== expected) {
    errors.push(`productionAuthHardening must point to ${expected}`);
  }
  const refreshFamily = productionAuth.sessionCredentialPolicy?.refreshTokenFamily ?? {};
  if (refreshFamily.status !== contract.promotionState?.refreshTokenFamilyStatus) {
    errors.push("production auth refreshTokenFamily.status must match promotionState.refreshTokenFamilyStatus");
  }
  if (refreshFamily.defaultRuntime !== contract.promotionState?.runtimeDefault) {
    errors.push("production auth refreshTokenFamily.defaultRuntime must match promotionState.runtimeDefault");
  }
  if (refreshFamily.promotionReadinessContract !== "resources/platform-refresh-token-family-promotion.json") {
    errors.push("production auth refreshTokenFamily.promotionReadinessContract must point to resources/platform-refresh-token-family-promotion.json");
  }
  requireIncludes(
    refreshFamily.allowedOnlyWhen,
    values(contract.promotionState?.allowedOnlyWhen),
    "production auth refreshTokenFamily.allowedOnlyWhen",
    errors,
  );
}

function validatePromotionState(contract, errors) {
  const state = contract.promotionState ?? {};
  if (state.refreshTokenFamilyStatus !== "implemented-disabled") {
    errors.push("promotionState.refreshTokenFamilyStatus must stay implemented-disabled");
  }
  if (state.implementationStatus !== "implemented") {
    errors.push("promotionState.implementationStatus must stay implemented");
  }
  if (state.runtimeDefault !== "disabled") {
    errors.push("promotionState.runtimeDefault must stay disabled");
  }
  if (state.approvalRequired !== true) {
    errors.push("promotionState.approvalRequired must stay true");
  }
  if (state.defaultRuntimeMutation !== "forbidden") {
    errors.push("promotionState.defaultRuntimeMutation must stay forbidden");
  }
  if (state.productionEnablementStatus !== "blocked-by-external-approval") {
    errors.push("promotionState.productionEnablementStatus must stay blocked-by-external-approval");
  }
  requireIncludes(
    state.allowedOnlyWhen,
    ["offline-renewal-required", "token-family-reuse-detection-required", "separate-refresh-token-storage-approved", "rotation-replay-audit-approved"],
    "promotionState.allowedOnlyWhen",
    errors,
  );
}

function validateImplementedRuntimeArtifacts(contract, errors) {
  const requiredArtifacts = [
    "refresh-token-family-store",
    "gorm-refresh-token-family-repository",
    "rotation-and-reuse-detection-service",
    "audit-redaction-adapter",
  ];
  const artifacts = new Map(values(contract.implementedRuntimeArtifacts).map((item) => [item.id, item]));
  for (const id of requiredArtifacts) {
    const artifact = artifacts.get(id);
    if (!artifact) {
      errors.push(`implementedRuntimeArtifacts must include ${id}`);
      continue;
    }
    if (artifact.status !== "implemented") {
      errors.push(`implementedRuntimeArtifacts.${id}.status must stay implemented`);
    }
    validateEvidenceSnippets(artifact.evidence, `implementedRuntimeArtifacts.${id}`, errors);
  }
}

function validateDataModel(contract, errors) {
  const model = contract.dataModelContract ?? {};
  if (model.separateFromSessionTable !== true) {
    errors.push("dataModelContract.separateFromSessionTable must stay true");
  }
  if (model.rawTokenPersistenceAllowed !== false) {
    errors.push("dataModelContract.rawTokenPersistenceAllowed must stay false");
  }
  if (model.tokenHashRequired !== true) {
    errors.push("dataModelContract.tokenHashRequired must stay true");
  }
  requireIncludes(
    model.requiredFields,
    ["familyId", "tokenId", "parentTokenId", "sessionId", "username", "tenantId", "tokenType", "issuedAt", "expiresAt", "rotatedAt", "revokedAt", "reusedAt", "replacedByTokenId", "tokenHash"],
    "dataModelContract.requiredFields",
    errors,
  );
  requireIncludes(model.exposedReadFields, ["familyId", "tokenId", "status", "issuedAt", "expiresAt"], "dataModelContract.exposedReadFields", errors);
  requireIncludes(model.forbiddenReadFields, ["refreshToken", "tokenHash", "rawToken", "secret"], "dataModelContract.forbiddenReadFields", errors);
}

function validateRotationAndReuse(contract, errors) {
  const rotation = contract.rotationTransaction ?? {};
  if (rotation.authoritativeStore !== "database") {
    errors.push("rotationTransaction.authoritativeStore must stay database");
  }
  if (rotation.singleTransactionRequired !== true) {
    errors.push("rotationTransaction.singleTransactionRequired must stay true");
  }
  requireIncludes(
    rotation.requiredSteps,
    ["validate-current-token-hash", "mark-current-token-rotated", "insert-next-token-generation", "renew-server-side-session", "publish-sessions-invalidation", "write-redacted-rotation-audit"],
    "rotationTransaction.requiredSteps",
    errors,
  );

  const reuse = contract.reuseDetection ?? {};
  if (reuse.mandatory !== true) {
    errors.push("reuseDetection.mandatory must stay true");
  }
  requireIncludes(reuse.replayInputs, ["unknown-token", "rotated-token", "revoked-token", "expired-token"], "reuseDetection.replayInputs", errors);
  requireIncludes(
    reuse.requiredEffects,
    ["mark-family-compromised", "revoke-related-server-side-session", "publish-sessions-invalidation", "write-redacted-reuse-audit", "reject-with-stable-auth-error"],
    "reuseDetection.requiredEffects",
    errors,
  );
}

function validateRevocationScopes(contract, errors) {
  const scopes = new Map(values(contract.revocationScopeMatrix).map((item) => [item.scope, values(item.requiredEffects)]));
  const requiredScopes = {
    logout: ["revoke-current-session", "revoke-active-refresh-family"],
    "reuse-detection": ["revoke-full-family", "revoke-related-session", "publish-sessions-invalidation"],
    "admin-forced-logout": ["revoke-selected-user-sessions", "revoke-selected-refresh-families"],
    "jwt-signing-secret-rotation": ["document-session-invalidation-decision", "run-token-rotation-preflight"],
    "provider-credential-rotation": ["keep-family-revocation-separate-unless-incident-review-requires-it"],
    "api-token-rotation": ["do-not-touch-human-or-app-session-families"],
  };
  for (const [scope, effects] of Object.entries(requiredScopes)) {
    if (!scopes.has(scope)) {
      errors.push(`revocationScopeMatrix must include ${scope}`);
      continue;
    }
    requireIncludes(scopes.get(scope), effects, `revocationScopeMatrix.${scope}.requiredEffects`, errors);
  }
}

function validateRedisAndAudit(contract, errors) {
  const redis = contract.redisConvergencePolicy ?? {};
  if (redis.redisSourceOfTruth !== false) {
    errors.push("redisConvergencePolicy.redisSourceOfTruth must stay false");
  }
  if (redis.authoritativeSourceOfTruth !== "database") {
    errors.push("redisConvergencePolicy.authoritativeSourceOfTruth must stay database");
  }
  if (redis.invalidationResource !== "sessions") {
    errors.push("redisConvergencePolicy.invalidationResource must stay sessions");
  }
  requireIncludes(redis.requiredEvents, ["refresh-rotation-success", "refresh-reuse-detected", "logout", "admin-forced-logout"], "redisConvergencePolicy.requiredEvents", errors);

  const audit = contract.auditPolicy ?? {};
  requireIncludes(audit.requiredActions, ["auth.refresh", "auth.refresh.rotate", "auth.refresh.reuse_detected", "auth.logout"], "auditPolicy.requiredActions", errors);
  requireIncludes(audit.allowedFields, ["actor", "action", "resource", "familyId", "tokenId", "sessionId", "createdAt"], "auditPolicy.allowedFields", errors);
  requireIncludes(audit.forbiddenRawFields, ["jwt", "bearerToken", "refreshToken", "tokenHash", "apiToken", "openid", "unionid", "phone", "secret"], "auditPolicy.forbiddenRawFields", errors);
  const allowed = new Set(values(audit.allowedFields));
  for (const field of values(audit.forbiddenRawFields)) {
    if (allowed.has(field)) {
      errors.push(`auditPolicy.allowedFields must not include forbidden raw field ${field}`);
    }
  }
}

function validateProviderSeparation(contract, errors) {
  const separation = contract.providerRotationSeparation ?? {};
  if (separation.providerCredentialRotationRevokesFamiliesByDefault !== false) {
    errors.push("providerRotationSeparation.providerCredentialRotationRevokesFamiliesByDefault must stay false");
  }
  if (separation.incidentReviewCanRequireFamilyRevocation !== true) {
    errors.push("providerRotationSeparation.incidentReviewCanRequireFamilyRevocation must stay true");
  }
  if (separation.apiTokenRotationTouchesSessionFamilies !== false) {
    errors.push("providerRotationSeparation.apiTokenRotationTouchesSessionFamilies must stay false");
  }
}

function validatePromotionEvidence(contract, errors) {
  const evidence = contract.requiredPromotionEvidence ?? {};
  for (const relativePath of [...values(evidence.docs), ...values(evidence.validators).filter((item) => item.endsWith(".mjs"))]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`requiredPromotionEvidence path is missing or unsafe: ${relativePath}`);
    }
  }
  requireIncludes(evidence.validators, ["scripts/validate-platform-refresh-token-family-promotion.mjs", "scripts/validate-platform-production-auth-hardening.mjs", "scripts/validate-platform-production-readiness.mjs"], "requiredPromotionEvidence.validators", errors);
  requireIncludes(
    evidence.testsBeforeProductionEnablement,
    ["rotation lineage and reuse-detection tests", "revocation scope tests", "redis session invalidation convergence tests", "audit redaction tests", "provider rotation separation tests"],
    "requiredPromotionEvidence.testsBeforeProductionEnablement",
    errors,
  );
  requireIncludes(
    evidence.remainingBeforeProductionEnablement,
    ["production approval package", "redis session invalidation convergence evidence", "rotation replay audit sample", "provider rotation separation review", "rollback plan"],
    "requiredPromotionEvidence.remainingBeforeProductionEnablement",
    errors,
  );
}

function validateTaskAndReadiness(contract, taskGraph, readiness, errors) {
  const taskID = contract.taskGraph?.taskId;
  const task = values(taskGraph.tasks).find((item) => item.id === taskID);
  if (!task) {
    errors.push(`task graph must include ${taskID}`);
  } else {
    const allowed = values(contract.taskGraph?.allowedStatusesBeforePromotion);
    if (!allowed.includes(task.status) && task.status !== contract.taskGraph?.promotionStatus) {
      errors.push(`${taskID} has unsupported status ${task.status}`);
    }
  }
  const commands = new Set(values(readiness.preflightCommands).map((command) => command.id));
  if (!commands.has("refresh-token-family-promotion")) {
    errors.push("production readiness preflightCommands must include refresh-token-family-promotion");
  }
  const tokenRotation = values(readiness.operationPolicies).find((policy) => policy.id === "token-rotation");
  if (!values(tokenRotation?.preflightCommands).includes("refresh-token-family-promotion")) {
    errors.push("token-rotation operation policy must include refresh-token-family-promotion preflight");
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const productionAuth = readJSON(productionAuthPath);
  const readiness = readJSON(readinessPath);
  const taskGraph = readJSON(taskGraphPath);
  const errors = [];

  if (!contract.purpose) {
    errors.push("refresh token family promotion purpose is required");
  }
  validateCurrentRuntime(contract, errors);
  validateProductionAuthLink(contract, productionAuth, errors);
  validatePromotionState(contract, errors);
  validateImplementedRuntimeArtifacts(contract, errors);
  validateDataModel(contract, errors);
  validateRotationAndReuse(contract, errors);
  validateRevocationScopes(contract, errors);
  validateRedisAndAudit(contract, errors);
  validateProviderSeparation(contract, errors);
  validatePromotionEvidence(contract, errors);
  validateTaskAndReadiness(contract, taskGraph, readiness, errors);

  return { errors };
}

const { errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated refresh-token family promotion gate in ${path.relative(repoRoot, contractPath)}`);
