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

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-production-auth-hardening.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const readinessPath = path.resolve(repoRoot, argValue("--production-readiness", "resources/platform-production-readiness.json"));
const capabilityAuditPath = path.resolve(repoRoot, argValue("--capability-audit", "resources/generated/platform-capability-audit.json"));
const refreshTokenFamilyPromotionPath = path.resolve(repoRoot, argValue("--refresh-token-family-promotion", "resources/platform-refresh-token-family-promotion.json"));
const promotionReviewPath = path.resolve(repoRoot, argValue("--promotion-review", "resources/generated/production-auth-promotion-review.json"));
const sessionPolicyDocPath = path.resolve(repoRoot, argValue("--session-policy-doc", "docs/superpowers/specs/2026-07-07-platform-production-session-policy-design.md"));
const oidcDesignDocPath = path.resolve(repoRoot, argValue("--oidc-design-doc", "docs/superpowers/specs/2026-07-11-production-admin-oidc-auth-design.md"));
const adminResourceSchemaDocPath = path.resolve(repoRoot, argValue("--admin-resource-schema-doc", "docs/admin-resource-schema.md"));

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

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function sameSet(actual, expected) {
  if (new Set(actual).size !== actual.length || new Set(expected).size !== expected.length) {
    return false;
  }
  return JSON.stringify([...actual].sort()) === JSON.stringify([...expected].sort());
}

function validateArtifactHashPolicy(schema, label, errors) {
  const policy = schema.artifactHashPolicy ?? {};
  if (policy.algorithm !== "sha256") {
    errors.push(`${label}.artifactHashPolicy.algorithm must be sha256`);
  }
  if (policy.format !== "prefix-hex") {
    errors.push(`${label}.artifactHashPolicy.format must be prefix-hex`);
  }
  if (policy.prefix !== "sha256:") {
    errors.push(`${label}.artifactHashPolicy.prefix must be sha256:`);
  }
  if (policy.hexLength !== 64) {
    errors.push(`${label}.artifactHashPolicy.hexLength must be 64`);
  }
  if (policy.casing !== "lowercase") {
    errors.push(`${label}.artifactHashPolicy.casing must be lowercase`);
  }
}

function validateArtifactURIPolicy(schema, label, errors) {
  const policy = schema.artifactURIPolicy ?? {};
  if (policy.sourceOfTruth !== "external-review-artifacts") {
    errors.push(`${label}.artifactURIPolicy.sourceOfTruth must be external-review-artifacts`);
  }
  requireIncludes(policy.allowedSchemes, ["https", "s3", "gs"], `${label}.artifactURIPolicy.allowedSchemes`, errors);
  requireIncludes(policy.forbiddenSchemes, ["file", "http"], `${label}.artifactURIPolicy.forbiddenSchemes`, errors);
  for (const field of ["forbidRelativePaths", "forbidLocalAbsolutePaths", "forbidLocalhost", "forbidPrivateNetworkHosts"]) {
    if (policy[field] !== true) {
      errors.push(`${label}.artifactURIPolicy.${field} must be true`);
    }
  }
}

function evidenceIDs(items) {
  return values(items)
    .map((item) => (typeof item === "string" ? item : item?.id))
    .filter(Boolean);
}

function validateCredentialClasses(contract, errors) {
  const credentials = values(contract.currentCredentialClasses);
  const ids = new Set(credentials.map((credential) => credential.id));
  for (const id of ["admin-jwt", "app-jwt", "api-token"]) {
    if (!ids.has(id)) {
      errors.push(`currentCredentialClasses must include ${id}`);
    }
  }
  for (const credential of credentials) {
    if (credential.rawSecretMustNotBeLogged !== true) {
      errors.push(`credential ${credential.id ?? "<missing>"} must require raw secret redaction`);
    }
    if (!relativeExistingPath(credential.source)) {
      errors.push(`credential ${credential.id ?? "<missing>"} source is missing or unsafe: ${credential.source ?? "<missing>"}`);
    }
  }
}

function validateSessionPolicy(contract, errors) {
  const policy = contract.sessionCredentialPolicy ?? {};
  if (policy.sessionStore?.productionDriver !== "gorm") {
    errors.push("sessionCredentialPolicy.sessionStore.productionDriver must stay gorm");
  }
  if (policy.sessionStore?.distributedInvalidationRequired !== true) {
    errors.push("sessionCredentialPolicy.sessionStore.distributedInvalidationRequired must stay true");
  }
  if (policy.sessionStore?.tokenPersistence !== "sha256:v1-digest-only") {
    errors.push("sessionCredentialPolicy.sessionStore.tokenPersistence must stay sha256:v1-digest-only");
  }
  if (policy.sessionStore?.rawTokenPersistenceAllowed !== false) {
    errors.push("sessionCredentialPolicy.sessionStore.rawTokenPersistenceAllowed must stay false");
  }
  if (policy.sessionStore?.legacyRawSessionMigration !== "replace-and-revoke") {
    errors.push("sessionCredentialPolicy.sessionStore.legacyRawSessionMigration must stay replace-and-revoke");
  }
  if (policy.slidingRenewal?.status !== "implemented") {
    errors.push("sessionCredentialPolicy.slidingRenewal.status must stay implemented");
  }
  if (policy.slidingRenewal?.notARefreshTokenFamily !== true) {
    errors.push("sessionCredentialPolicy.slidingRenewal.notARefreshTokenFamily must stay true");
  }
  if (policy.productionSessionPolicy?.status !== "specified") {
    errors.push("sessionCredentialPolicy.productionSessionPolicy.status must stay specified");
  }
  if (!relativeExistingPath(policy.productionSessionPolicy?.path)) {
    errors.push(`sessionCredentialPolicy.productionSessionPolicy.path is missing or unsafe: ${policy.productionSessionPolicy?.path ?? "<missing>"}`);
  } else {
    const spec = fs.readFileSync(path.resolve(repoRoot, policy.productionSessionPolicy.path), "utf8");
    for (const phrase of [
      "Current Runtime Boundary",
      "Refresh Token Family Model",
      "Reuse Detection",
      "Revocation Scope Matrix",
      "Redis may speed up invalidation and cache lookups, but it is not the source of truth",
      "provider credential rotation",
      "raw refresh tokens must never be persisted",
    ]) {
      if (!spec.includes(phrase)) {
        errors.push(`sessionCredentialPolicy.productionSessionPolicy spec must include ${phrase}`);
      }
    }
  }
  requireIncludes(
    policy.productionSessionPolicy?.requiredDecisions,
    [
      "current-runtime-sliding-renewal-boundary",
      "refresh-token-family-data-model",
      "reuse-detection-revokes-family-and-session",
      "revocation-scope-matrix",
      "redis-invalidation-not-source-of-truth",
      "audit-redaction-and-replay-evidence",
      "provider-rotation-separation",
    ],
    "sessionCredentialPolicy.productionSessionPolicy.requiredDecisions",
    errors,
  );
  if (policy.productionSessionPolicy?.runtimePromotion !== "blocked-until-production-approval-package-approved") {
    errors.push("sessionCredentialPolicy.productionSessionPolicy.runtimePromotion must stay blocked-until-production-approval-package-approved");
  }
  if (policy.refreshTokenFamily?.status !== "implemented-disabled") {
    errors.push("refreshTokenFamily status must stay implemented-disabled until production approval is attached");
  }
  if (policy.refreshTokenFamily?.defaultRuntime !== "disabled") {
    errors.push("refreshTokenFamily defaultRuntime must stay disabled until production approval is attached");
  }
  if (policy.refreshTokenFamily?.runtimeBinding !== "available-but-not-bound-to-default-refresh-endpoint") {
    errors.push("refreshTokenFamily runtimeBinding must stay available-but-not-bound-to-default-refresh-endpoint");
  }
  if (policy.refreshTokenFamily?.promotionReadinessContract !== "resources/platform-refresh-token-family-promotion.json") {
    errors.push("sessionCredentialPolicy.refreshTokenFamily.promotionReadinessContract must point to resources/platform-refresh-token-family-promotion.json");
  }
  if (!relativeExistingPath(policy.refreshTokenFamily?.promotionReadinessContract)) {
    errors.push(`sessionCredentialPolicy.refreshTokenFamily.promotionReadinessContract is missing or unsafe: ${policy.refreshTokenFamily?.promotionReadinessContract ?? "<missing>"}`);
  }
  if (policy.refreshTokenFamily?.specification !== policy.productionSessionPolicy?.path) {
    errors.push("sessionCredentialPolicy.refreshTokenFamily.specification must match productionSessionPolicy.path");
  }
  requireIncludes(
    policy.refreshTokenFamily?.allowedOnlyWhen,
    ["offline-renewal-required", "token-family-reuse-detection-required", "separate-refresh-token-storage-approved", "rotation-replay-audit-approved"],
    "sessionCredentialPolicy.refreshTokenFamily.allowedOnlyWhen",
    errors,
  );
  requireIncludes(
    policy.refreshTokenFamily?.requiredProductionEnablementEvidence,
    [
      "hashed refresh-token-family storage",
      "rotation lineage and reuse-detection tests",
      "revocation scope tests",
      "redis session invalidation convergence",
      "audit redaction for refresh replay",
      "provider rotation separation",
    ],
    "sessionCredentialPolicy.refreshTokenFamily.requiredProductionEnablementEvidence",
    errors,
  );
  for (const relativePath of values(policy.refreshTokenFamily?.implementedRuntimeEvidence)) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`sessionCredentialPolicy.refreshTokenFamily.implementedRuntimeEvidence path is missing or unsafe: ${relativePath}`);
    }
  }
}

function validateProviderPolicy(contract, errors) {
  const policy = contract.providerAdapterPolicy ?? {};
  if (policy.boundary !== "httpapi.AppIdentityResolver") {
    errors.push("providerAdapterPolicy.boundary must stay httpapi.AppIdentityResolver");
  }
  requireIncludes(
    policy.requiredControls,
    ["secret-rotation-plan", "provider-subject-redaction", "raw-provider-subject-never-in-response", "raw-provider-subject-never-in-audit", "configured-provider-only-login"],
    "providerAdapterPolicy.requiredControls",
    errors,
  );
  requireIncludes(
    policy.productionPromotionRequires,
    ["provider credential owner", "secret storage and rotation runbook", "subject hash and masking proof", "login failure audit policy", "contract tests for unconfigured provider rejection"],
    "providerAdapterPolicy.productionPromotionRequires",
    errors,
  );
}

function validateProductionPromotionApprovalPackage(contract, errors) {
  const pkg = contract.productionPromotionApprovalPackage ?? {};
  if (pkg.status !== "blocked") {
    errors.push("productionPromotionApprovalPackage.status must stay blocked before promotion");
  }
  if (pkg.sourceOfTruth !== "external-review-artifacts") {
    errors.push("productionPromotionApprovalPackage.sourceOfTruth must stay external-review-artifacts");
  }
  if (pkg.defaultRuntimeMutation !== "forbidden") {
    errors.push("productionPromotionApprovalPackage.defaultRuntimeMutation must stay forbidden");
  }
  if (pkg.mustNotEnableRefreshTokenFamily !== true) {
    errors.push("productionPromotionApprovalPackage.mustNotEnableRefreshTokenFamily must stay true");
  }
  if (pkg.mustNotEnableUnreviewedProvider !== true) {
    errors.push("productionPromotionApprovalPackage.mustNotEnableUnreviewedProvider must stay true");
  }
  if (values(pkg.completedEvidence).length !== 0) {
    errors.push("productionPromotionApprovalPackage.completedEvidence must stay empty before promotion");
  }
  requireIncludes(
    pkg.requiredApprovals,
    ["security-owner", "platform-architect", "operations-owner"],
    "productionPromotionApprovalPackage.requiredApprovals",
    errors,
  );
  requireIncludes(
    pkg.prohibitedEvidence,
    [
      "text-only approval",
      "single-person self approval",
      "missing reuse-detection tests",
      "missing revocation tests",
      "missing Redis/session invalidation convergence",
      "missing audit redaction evidence",
      "missing provider rotation runbook",
      "runtime mutation without rollback plan",
    ],
    "productionPromotionApprovalPackage.prohibitedEvidence",
    errors,
  );

  const requiredEvidence = {
    "session-policy-review": ["security-owner", "signed-security-review"],
    "refresh-token-family-runtime-spec": ["platform-architect", "architecture-spec"],
    "credential-rotation-runbook": ["operations-owner", "rotation-runbook"],
    "provider-secret-rotation-runbook": ["operations-owner", "provider-rotation-runbook"],
    "runtime-test-output": ["security-owner", "test-output"],
    "audit-redaction-sample": ["security-owner", "redacted-audit-sample"],
    "rollback-plan": ["operations-owner", "rollback-runbook"],
  };
  const evidenceByID = new Map(values(pkg.requiredEvidence).map((item) => [item.id, item]));
  for (const [id, [owner, evidenceKind]] of Object.entries(requiredEvidence)) {
    const item = evidenceByID.get(id);
    if (!item) {
      errors.push(`productionPromotionApprovalPackage.requiredEvidence must include ${id}`);
      continue;
    }
    if (item.owner !== owner) {
      errors.push(`productionPromotionApprovalPackage.requiredEvidence.${id}.owner must be ${owner}`);
    }
    if (item.evidenceKind !== evidenceKind) {
      errors.push(`productionPromotionApprovalPackage.requiredEvidence.${id}.evidenceKind must be ${evidenceKind}`);
    }
    if (!item.description) {
      errors.push(`productionPromotionApprovalPackage.requiredEvidence.${id}.description is required`);
    }
  }

  const schema = pkg.completedEvidenceSchema ?? {};
  requireIncludes(
    schema.requiredFields,
    [
      "id",
      "owner",
      "evidenceKind",
      "artifactURI",
      "artifactHash",
      "approvedBy",
      "approvedAt",
      "reviewedCommit",
      "environment",
      "verificationCommands",
      "rollbackCommands",
      "auditSampleRefs",
      "providerRotationRunbookRefs",
      "refreshTokenFamilyTestRefs",
      "providerIds",
      "providerControls",
      "runtimeTestRefs",
    ],
    "productionPromotionApprovalPackage.completedEvidenceSchema.requiredFields",
    errors,
  );
  requireIncludes(
    schema.approvalRules,
    [
      "artifact-id-must-match-requiredEvidence",
      "approvedBy-must-not-equal-owner",
      "all-required-owners-represented",
      "artifact-hash-required",
      "artifact-hash-must-be-sha256-hex",
      "artifact-uri-must-be-external-review-artifact",
      "verification-commands-must-use-rtk",
      "rollback-evidence-required-before-runtime-mutation",
      "provider-rotation-runbook-required-before-provider-promotion",
      "refresh-token-family-tests-required-before-runtime-mutation",
      "redacted-audit-sample-required-before-promotion",
      "provider-controls-covered-before-promotion",
      "provider-runtime-tests-required-before-promotion",
      "production-environment-required",
    ],
    "productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules",
    errors,
  );
  requireIncludes(
    schema.forbiddenFields,
    ["jwt", "bearerToken", "refreshToken", "apiToken", "openid", "unionid", "phone", "secret", "password", "privateKey", "tokenHash", "providerSubject", "rawSubject"],
    "productionPromotionApprovalPackage.completedEvidenceSchema.forbiddenFields",
    errors,
  );
  validateArtifactHashPolicy(pkg.completedEvidenceSchema ?? {}, "productionPromotionApprovalPackage.completedEvidenceSchema", errors);
  validateArtifactURIPolicy(pkg.completedEvidenceSchema ?? {}, "productionPromotionApprovalPackage.completedEvidenceSchema", errors);
}

function validateProductionPromotionReview(contract, refreshTokenFamilyPromotion, readiness, review, errors) {
  const expectedProductionAuthSource = path.relative(repoRoot, contractPath).split(path.sep).join("/");
  const expectedRefreshTokenFamilySource = path.relative(repoRoot, refreshTokenFamilyPromotionPath).split(path.sep).join("/");
  const expectedReadinessSource = path.relative(repoRoot, readinessPath).split(path.sep).join("/");
  const approvalPackage = contract.productionPromotionApprovalPackage ?? {};
  const requiredEvidenceIDs = evidenceIDs(approvalPackage.requiredEvidence).sort();
  const completedEvidenceIDs = evidenceIDs(approvalPackage.completedEvidence).sort();
  const missingEvidenceIDs = requiredEvidenceIDs.filter((id) => !completedEvidenceIDs.includes(id)).sort();
  const providerCount = values(contract.providerPromotionMatrix?.providers).length;
  const optionalProviderCount = values(contract.providerPromotionMatrix?.providers).filter((provider) => provider.productionUsage === "optional-production-provider").length;

  if (contract.promotionReview?.path !== "resources/generated/production-auth-promotion-review.json") {
    errors.push("production auth hardening promotionReview.path must point to resources/generated/production-auth-promotion-review.json");
  }
  if (contract.promotionReview?.generator !== "scripts/generate-production-auth-promotion-review.mjs") {
    errors.push("production auth hardening promotionReview.generator must be scripts/generate-production-auth-promotion-review.mjs");
  }
  if (contract.promotionReview?.decision !== "not-approved") {
    errors.push("production auth hardening promotionReview.decision must stay not-approved before external approval");
  }
  if (contract.promotionReview?.runtimeMutation !== "disabled") {
    errors.push("production auth hardening promotionReview.runtimeMutation must stay disabled");
  }
  if (review.generatedBy !== "scripts/generate-production-auth-promotion-review.mjs") {
    errors.push("production auth promotion review must be generated by scripts/generate-production-auth-promotion-review.mjs");
  }
  if (review.sources?.productionAuthHardening !== expectedProductionAuthSource) {
    errors.push(`production auth promotion review sources.productionAuthHardening must point to ${expectedProductionAuthSource}`);
  }
  if (review.sources?.refreshTokenFamilyPromotion !== expectedRefreshTokenFamilySource) {
    errors.push(`production auth promotion review sources.refreshTokenFamilyPromotion must point to ${expectedRefreshTokenFamilySource}`);
  }
  if (review.sources?.productionReadiness !== expectedReadinessSource) {
    errors.push(`production auth promotion review sources.productionReadiness must point to ${expectedReadinessSource}`);
  }
  if (review.mode?.dryRun !== true) {
    errors.push("production auth promotion review mode.dryRun must stay true");
  }
  if (review.mode?.runtimeMutation !== "disabled") {
    errors.push("production auth promotion review mode.runtimeMutation must stay disabled");
  }
  if (review.mode?.refreshTokenFamilyRuntime !== "disabled") {
    errors.push("production auth promotion review mode.refreshTokenFamilyRuntime must stay disabled");
  }
  if (review.mode?.providerRuntimeMutation !== "disabled") {
    errors.push("production auth promotion review mode.providerRuntimeMutation must stay disabled");
  }
  if (review.currentRuntime?.refreshTokenFamilyStatus !== contract.sessionCredentialPolicy?.refreshTokenFamily?.status) {
    errors.push("production auth promotion review currentRuntime.refreshTokenFamilyStatus must match production auth hardening contract");
  }
  if (review.currentRuntime?.refreshTokenFamilyPromotionStatus !== refreshTokenFamilyPromotion.promotionState?.implementationStatus) {
    errors.push("production auth promotion review currentRuntime.refreshTokenFamilyPromotionStatus must match refresh-token family promotion contract");
  }
  if (review.currentRuntime?.notARefreshTokenFamily !== true) {
    errors.push("production auth promotion review currentRuntime.notARefreshTokenFamily must stay true");
  }
  if (review.approvalPackage?.status !== approvalPackage.status) {
    errors.push("production auth promotion review approvalPackage.status must match production auth hardening contract");
  }
  if (!sameSet(values(review.approvalPackage?.requiredApprovals).sort(), values(approvalPackage.requiredApprovals).sort())) {
    errors.push("production auth promotion review approvalPackage.requiredApprovals must match production auth hardening contract");
  }
  if (!sameSet(evidenceIDs(review.approvalPackage?.requiredEvidence).sort(), requiredEvidenceIDs)) {
    errors.push("production auth promotion review approvalPackage.requiredEvidence must match production auth hardening contract");
  }
  if (!sameSet(evidenceIDs(review.approvalPackage?.completedEvidence).sort(), completedEvidenceIDs)) {
    errors.push("production auth promotion review approvalPackage.completedEvidence must match production auth hardening contract");
  }
  if (!sameSet(values(review.approvalPackage?.missingEvidence).sort(), missingEvidenceIDs)) {
    errors.push("production auth promotion review approvalPackage.missingEvidence must reflect incomplete approval evidence");
  }
  for (const field of ["requiredFields", "approvalRules", "forbiddenFields"]) {
    if (!sameSet(values(review.approvalPackage?.completedEvidenceSchema?.[field]).sort(), values(approvalPackage.completedEvidenceSchema?.[field]).sort())) {
      errors.push(`production auth promotion review approvalPackage.completedEvidenceSchema.${field} must match production auth hardening contract`);
    }
  }
  validateArtifactHashPolicy(
    review.approvalPackage?.completedEvidenceSchema ?? {},
    "production auth promotion review approvalPackage.completedEvidenceSchema",
    errors,
  );
  validateArtifactURIPolicy(
    review.approvalPackage?.completedEvidenceSchema ?? {},
    "production auth promotion review approvalPackage.completedEvidenceSchema",
    errors,
  );
  if (review.summary?.requiredEvidenceCount !== requiredEvidenceIDs.length) {
    errors.push(`production auth promotion review summary.requiredEvidenceCount must be ${requiredEvidenceIDs.length}`);
  }
  if (review.summary?.completedEvidenceCount !== completedEvidenceIDs.length) {
    errors.push(`production auth promotion review summary.completedEvidenceCount must be ${completedEvidenceIDs.length}`);
  }
  if (review.summary?.missingEvidenceCount !== missingEvidenceIDs.length) {
    errors.push(`production auth promotion review summary.missingEvidenceCount must be ${missingEvidenceIDs.length}`);
  }
  if (review.summary?.providerPromotionCount !== providerCount) {
    errors.push(`production auth promotion review summary.providerPromotionCount must be ${providerCount}`);
  }
  if (review.summary?.optionalProductionProviderCount !== optionalProviderCount) {
    errors.push(`production auth promotion review summary.optionalProductionProviderCount must be ${optionalProviderCount}`);
  }
  const expectedProviders = values(contract.providerPromotionMatrix?.providers);
  const reviewProviders = values(review.providerPromotionMatrix?.providers);
  if (!sameSet(reviewProviders.map((provider) => provider.id).sort(), expectedProviders.map((provider) => provider.id).sort())) {
    errors.push("production auth promotion review providerPromotionMatrix.providers must match production auth hardening contract");
  }
  const reviewProviderByID = new Map(reviewProviders.map((provider) => [provider.id, provider]));
  for (const expected of expectedProviders) {
    const actual = reviewProviderByID.get(expected.id);
    if (!actual) {
      continue;
    }
    for (const field of ["capability", "kind", "productionUsage", "adapterBoundary"]) {
      if (actual[field] !== expected[field]) {
        errors.push(`production auth promotion review provider ${expected.id} ${field} must match production auth hardening contract`);
      }
    }
    if (!sameSet(values(actual.audiences).sort(), values(expected.audiences).sort())) {
      errors.push(`production auth promotion review provider ${expected.id} audiences must match production auth hardening contract`);
    }
    for (const field of ["configKeys", "requiredControls"]) {
      if (!sameSet(values(actual[field]).sort(), values(expected[field]).sort())) {
        errors.push(`production auth promotion review provider ${expected.id} ${field} must match production auth hardening contract`);
      }
    }
    for (const field of [
      "requiresSecretOwner",
      "rotationRunbookRequired",
      "subjectRedactionRequired",
      "unconfiguredProviderRejectionRequired",
      "errorNormalizationRequired",
      "productionLikeRehearsalRequired",
      "rawCredentialExposureAllowed",
      "rawSubjectExposureAllowed",
    ]) {
      if (actual[field] !== expected[field]) {
        errors.push(`production auth promotion review provider ${expected.id} ${field} must match production auth hardening contract`);
      }
    }
  }
  requireIncludes(
    review.preflight?.tokenRotationPolicyCommands,
    ["production-auth-hardening", "refresh-token-family-promotion", "production-auth-promotion-review", "cache-invalidation", "task-execution-audit"],
    "production auth promotion review preflight.tokenRotationPolicyCommands",
    errors,
  );
  if (review.preflight?.tokenRotationPolicyRequiresHumanReview !== true) {
    errors.push("production auth promotion review preflight.tokenRotationPolicyRequiresHumanReview must stay true");
  }
  const blockers = values(review.blockers);
  requireIncludes(
    blockers,
    ["refresh-token-family default runtime is disabled pending production approval", "production approval package is blocked"],
    "production auth promotion review blockers",
    errors,
  );
  if (approvalPackage.status === "blocked" || contract.sessionCredentialPolicy?.refreshTokenFamily?.defaultRuntime === "disabled") {
    if (review.manualReview?.decision !== "not-approved") {
      errors.push("production auth promotion review manualReview.decision must stay not-approved while blockers are active");
    }
    if (review.summary?.blockerCount !== blockers.length || blockers.length === 0) {
      errors.push("production auth promotion review summary.blockerCount must reflect active blockers");
    }
  }
}

function validateProviderRuntimePolicy(contract, errors) {
  const policy = contract.providerRuntimePolicy ?? {};
  if (policy.adapterRegistration !== "manifest-declared-and-composition-root-injected") {
    errors.push("providerRuntimePolicy.adapterRegistration must stay manifest-declared-and-composition-root-injected");
  }
  if (policy.defaultDenyUnconfiguredProviders !== true) {
    errors.push("providerRuntimePolicy.defaultDenyUnconfiguredProviders must stay true");
  }
  if (policy.configuredProviderOnlyLogin !== true) {
    errors.push("providerRuntimePolicy.configuredProviderOnlyLogin must stay true");
  }
  if (policy.rawSubjectStorage !== "hash-and-mask-only") {
    errors.push("providerRuntimePolicy.rawSubjectStorage must stay hash-and-mask-only");
  }
  if (policy.responseRawSubjectAllowed !== false) {
    errors.push("providerRuntimePolicy.responseRawSubjectAllowed must stay false");
  }
  if (policy.auditRawSubjectAllowed !== false) {
    errors.push("providerRuntimePolicy.auditRawSubjectAllowed must stay false");
  }
  if (policy.genericResourceRawSubjectAllowed !== false) {
    errors.push("providerRuntimePolicy.genericResourceRawSubjectAllowed must stay false");
  }
  requireIncludes(
    policy.requiredTests,
    ["unconfigured-provider-rejection", "subject-redaction", "configured-provider-only-login", "provider-error-normalization", "production-demo-capability-rejection"],
    "providerRuntimePolicy.requiredTests",
    errors,
  );
  requireIncludes(
    policy.productionForbiddenCapabilities,
    ["demo-data"],
    "providerRuntimePolicy.productionForbiddenCapabilities",
    errors,
  );
  requireIncludes(
    policy.productionForbiddenAuthProviders,
    ["demo"],
    "providerRuntimePolicy.productionForbiddenAuthProviders",
    errors,
  );
  for (const relativePath of values(policy.runtimeEvidence)) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`providerRuntimePolicy.runtimeEvidence path is missing or unsafe: ${relativePath}`);
    }
  }

  const serverPath = "internal/platform/httpapi/server.go";
  const identityPath = "internal/platform/httpapi/app_identity.go";
  const wechatResolverPath = "internal/platform/authprovider/wechat/resolver.go";
  const wechatResolverTestPath = "internal/platform/authprovider/wechat/resolver_test.go";
  if (relativeExistingPath(serverPath)) {
    const server = fs.readFileSync(path.resolve(repoRoot, serverPath), "utf8");
    for (const snippet of [
      "APP_AUTH_PROVIDER_NOT_CONFIGURED",
      "APP_AUTH_PROVIDER_RESOLVE_FAILED",
      "APP_AUTH_IDENTITY_INVALID",
      "ProviderSubject: strings.TrimSpace(identity.ProviderSubject)",
    ]) {
      if (!server.includes(snippet)) {
        errors.push(`${serverPath} must include provider runtime evidence ${snippet}`);
      }
    }
  }
  const configPath = "internal/platform/config/config.go";
  if (relativeExistingPath(configPath)) {
    const config = fs.readFileSync(path.resolve(repoRoot, configPath), "utf8");
    for (const snippet of [
      "hasCapability(c.Capabilities, \"demo-data\")",
      "production runtime must not enable demo-data capability",
      "DisableDemoAuthProvider",
      "production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
    ]) {
      if (!config.includes(snippet)) {
        errors.push(`${configPath} must include production demo guard ${snippet}`);
      }
    }
  }
  const mainPath = "cmd/platform-api/main.go";
  if (relativeExistingPath(mainPath)) {
    const main = fs.readFileSync(path.resolve(repoRoot, mainPath), "utf8");
    if (!main.includes("DisableDemoAuthProvider: cfg.DisableDemoAuthProvider")) {
      errors.push(`${mainPath} must pass DisableDemoAuthProvider into httpapi.ServerOptions`);
    }
  }
  if (relativeExistingPath(serverPath)) {
    const server = fs.readFileSync(path.resolve(repoRoot, serverPath), "utf8");
    for (const snippet of [
      "DisableDemoAuthProvider bool",
      "disableDemoAuthProvider bool",
      "authProviderAvailable(provider)",
      "provider.Kind == \"demo\"",
    ]) {
      if (!server.includes(snippet)) {
        errors.push(`${serverPath} must include production demo auth provider guard ${snippet}`);
      }
    }
  }
  if (relativeExistingPath(identityPath)) {
    const identity = fs.readFileSync(path.resolve(repoRoot, identityPath), "utf8");
    for (const snippet of ["providerSubjectHash", "maskedSubject", "appProviderSubjectHash", "maskProviderSubject"]) {
      if (!identity.includes(snippet)) {
        errors.push(`${identityPath} must include provider subject redaction evidence ${snippet}`);
      }
    }
  }
  if (relativeExistingPath(wechatResolverPath)) {
    const resolver = fs.readFileSync(path.resolve(repoRoot, wechatResolverPath), "utf8");
    for (const snippet of ["ErrProviderResolveFailed", "%w: provider code %d", "%w: request failed", "%w: invalid response"]) {
      if (!resolver.includes(snippet)) {
        errors.push(`${wechatResolverPath} must include provider error normalization evidence ${snippet}`);
      }
    }
  }
  if (relativeExistingPath(wechatResolverTestPath)) {
    const resolverTest = fs.readFileSync(path.resolve(repoRoot, wechatResolverTestPath), "utf8");
    for (const snippet of ["TestResolverNormalizesProviderErrorForWechatErrorCode", "ErrProviderResolveFailed", "ResolveAppIdentity() error leaked"]) {
      if (!resolverTest.includes(snippet)) {
        errors.push(`${wechatResolverTestPath} must include provider error normalization test evidence ${snippet}`);
      }
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

function auditAuthProviders(audit) {
  const providers = new Set();
  for (const capability of values(audit.capabilities)) {
    for (const provider of values(capability.authProviders)) {
      providers.add(provider);
    }
  }
  return providers;
}

function validateProviderPromotionMatrix(contract, audit, errors) {
  const matrix = contract.providerPromotionMatrix ?? {};
  if (!matrix.defaultPolicy) {
    errors.push("providerPromotionMatrix.defaultPolicy is required");
  }
  if (matrix.manifestCoverage?.source !== "resources/generated/platform-capability-audit.json") {
    errors.push("providerPromotionMatrix.manifestCoverage.source must stay resources/generated/platform-capability-audit.json");
  }
  if (matrix.manifestCoverage?.required !== true) {
    errors.push("providerPromotionMatrix.manifestCoverage.required must stay true");
  }
  if (!matrix.manifestCoverage?.policy) {
    errors.push("providerPromotionMatrix.manifestCoverage.policy is required");
  }
  if (!relativeExistingPath(matrix.manifestCoverage?.source)) {
    errors.push(`providerPromotionMatrix.manifestCoverage.source is missing or unsafe: ${matrix.manifestCoverage?.source ?? "<missing>"}`);
  }
  const providers = values(matrix.providers);
  const byID = new Map(providers.map((provider) => [provider.id, provider]));
  for (const providerID of ["demo", "wechat", "oidc"]) {
    if (!byID.has(providerID)) {
      errors.push(`providerPromotionMatrix.providers must include ${providerID}`);
    }
  }
  for (const providerID of auditAuthProviders(audit)) {
    if (!byID.has(providerID)) {
      errors.push(`providerPromotionMatrix must include manifest-declared provider ${providerID}`);
    }
  }
  requireIncludes(
    matrix.newProviderRequirements,
    ["manifest-declared-provider", "composition-root-injected-resolver", "config-key-contract", "secret-owner", "rotation-runbook", "hash-and-mask-subject-storage", "unconfigured-provider-rejection-test", "configured-provider-only-login-test", "provider-error-normalization-test", "audit-redaction-test"],
    "providerPromotionMatrix.newProviderRequirements",
    errors,
  );

  for (const provider of providers) {
    const label = `providerPromotionMatrix provider ${provider.id ?? "<missing>"}`;
    if (!provider.id || !provider.capability || !provider.kind || !provider.productionUsage || !provider.adapterBoundary) {
      errors.push(`${label} must declare id, capability, kind, productionUsage and adapterBoundary`);
    }
    if (provider.rawCredentialExposureAllowed !== false) {
      errors.push(`${label} rawCredentialExposureAllowed must stay false`);
    }
    if (provider.rawSubjectExposureAllowed !== false) {
      errors.push(`${label} rawSubjectExposureAllowed must stay false`);
    }
    if (provider.subjectRedactionRequired !== true) {
      errors.push(`${label} subjectRedactionRequired must stay true`);
    }
    if (provider.errorNormalizationRequired !== true) {
      errors.push(`${label} errorNormalizationRequired must stay true`);
    }
    if (values(provider.audiences).length === 0 || values(provider.audiences).some((audience) => !["admin", "app"].includes(audience))) {
      errors.push(`${label} audiences must declare at least one typed audience`);
    }
    if (typeof provider.productionLikeRehearsalRequired !== "boolean") {
      errors.push(`${label} productionLikeRehearsalRequired must be boolean`);
    }
    validateEvidenceSnippets(provider.manifestEvidence, `${label} manifest`, errors);
    validateEvidenceSnippets(provider.runtimeEvidence, `${label} runtime`, errors);
  }

  const demo = byID.get("demo");
  if (demo) {
    if (demo.productionUsage !== "local-harness-only") {
      errors.push("providerPromotionMatrix provider demo productionUsage must stay local-harness-only");
    }
    if (demo.requiresSecretOwner !== false) {
      errors.push("providerPromotionMatrix provider demo requiresSecretOwner must stay false");
    }
    if (values(demo.configKeys).length !== 0) {
      errors.push("providerPromotionMatrix provider demo configKeys must stay empty");
    }
    if (!sameSet(values(demo.audiences).sort(), ["admin", "app"])) {
      errors.push("providerPromotionMatrix provider demo audiences must stay admin-and-app");
    }
    if (demo.productionLikeRehearsalRequired !== false) {
      errors.push("providerPromotionMatrix provider demo productionLikeRehearsalRequired must stay false");
    }
  }

  const wechat = byID.get("wechat");
  if (wechat) {
    if (wechat.productionUsage !== "optional-production-provider") {
      errors.push("providerPromotionMatrix provider wechat productionUsage must stay optional-production-provider");
    }
    if (wechat.adapterBoundary !== "httpapi.AppIdentityResolver") {
      errors.push("providerPromotionMatrix provider wechat adapterBoundary must stay httpapi.AppIdentityResolver");
    }
    if (!sameSet(values(wechat.audiences).sort(), ["app"])) {
      errors.push("providerPromotionMatrix provider wechat audiences must stay app-only");
    }
    if (wechat.productionLikeRehearsalRequired !== false) {
      errors.push("providerPromotionMatrix provider wechat productionLikeRehearsalRequired must stay false");
    }
    requireIncludes(
      wechat.configKeys,
      ["PLATFORM_WECHAT_MINIAPP_APP_ID", "PLATFORM_WECHAT_MINIAPP_SECRET", "PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT"],
      "providerPromotionMatrix provider wechat configKeys",
      errors,
    );
    requireIncludes(
      wechat.requiredControls,
      contract.providerAdapterPolicy?.requiredControls,
      "providerPromotionMatrix provider wechat requiredControls",
      errors,
    );
    for (const [field, expected] of [
      ["requiresSecretOwner", true],
      ["rotationRunbookRequired", true],
      ["unconfiguredProviderRejectionRequired", true],
    ]) {
      if (wechat[field] !== expected) {
        errors.push(`providerPromotionMatrix provider wechat ${field} must stay ${expected}`);
      }
    }
  }

  const oidc = byID.get("oidc");
  if (oidc) {
    if (oidc.capability !== "admin-oidc") {
      errors.push("providerPromotionMatrix provider oidc capability must stay admin-oidc");
    }
    if (oidc.kind !== "oidc") {
      errors.push("providerPromotionMatrix provider oidc kind must stay oidc");
    }
    if (oidc.productionUsage !== "optional-production-provider") {
      errors.push("providerPromotionMatrix provider oidc productionUsage must stay optional-production-provider");
    }
    if (oidc.adapterBoundary !== "httpapi.AdminIdentityResolver") {
      errors.push("providerPromotionMatrix provider oidc adapterBoundary must stay httpapi.AdminIdentityResolver");
    }
    if (JSON.stringify(values(oidc.audiences)) !== JSON.stringify(["admin"])) {
      errors.push("providerPromotionMatrix provider oidc audiences must stay admin-only");
    }
    const approvedOIDCConfigKeys = ["PLATFORM_ADMIN_OIDC_ISSUER_URL", "PLATFORM_ADMIN_OIDC_CLIENT_ID", "PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "PLATFORM_ADMIN_OIDC_REDIRECT_URL", "PLATFORM_ADMIN_OIDC_SCOPES"];
    if (!sameSet(values(oidc.configKeys).sort(), approvedOIDCConfigKeys.sort())) {
      errors.push("providerPromotionMatrix provider oidc configKeys must exactly match the approved Admin OIDC keys");
    }
    requireIncludes(
      oidc.requiredControls,
      [
        "admin-audience-only",
        "configured-provider-only-discovery-and-exchange",
        "issuer-validation",
        "signature-validation",
        "audience-validation",
        "nonce-validation",
        "state-validation",
        "pkce-s256-validation",
        "exact-redirect-url-validation",
        "explicit-identity-binding",
        "disabled-user-rejection",
        "provider-subject-redaction",
        "raw-provider-subject-never-in-response",
        "raw-provider-subject-never-in-audit",
        "provider-specific-error-normalization",
        "audit-redaction",
        "production-like-runtime-rehearsal",
      ],
      "providerPromotionMatrix provider oidc requiredControls",
      errors,
    );
    for (const [field, expected] of [
      ["requiresSecretOwner", true],
      ["rotationRunbookRequired", true],
      ["unconfiguredProviderRejectionRequired", true],
      ["productionLikeRehearsalRequired", true],
    ]) {
      if (oidc[field] !== expected) {
        errors.push(`providerPromotionMatrix provider oidc ${field} must stay ${expected}`);
      }
    }
  }
}

function validateRotationPolicy(contract, readiness, errors) {
  const requiredPolicyID = contract.credentialRotationPolicy?.requiredProductionReadinessPolicy;
  const operationPolicies = new Map(values(readiness.operationPolicies).map((policy) => [policy.id, policy]));
  const policy = operationPolicies.get(requiredPolicyID);
  if (!policy) {
    errors.push(`production readiness must include operation policy ${requiredPolicyID}`);
    return;
  }
  if (policy.requiresHumanReview !== true) {
    errors.push(`production readiness policy ${requiredPolicyID} must require human review`);
  }
  const policyText = [policy.rollbackRequirement, policy.auditRequirement, ...(policy.prohibitedActions ?? [])].join(" ");
  for (const tokenClass of values(contract.credentialRotationPolicy?.affectedCredentialClasses)) {
    if (!policyText.includes(tokenClass.replace("-jwt", " JWT")) && !policyText.includes(tokenClass.replace("api-token", "API token"))) {
      errors.push(`production readiness policy ${requiredPolicyID} must mention affected credential class ${tokenClass}`);
    }
  }
  requireIncludes(
    contract.credentialRotationPolicy?.requiredPlanFields,
    ["overlapWindow", "invalidationDecision", "affectedTokenClasses", "rollbackHandling", "verificationCommands"],
    "credentialRotationPolicy.requiredPlanFields",
    errors,
  );
  if (contract.credentialRotationPolicy?.secretRedactionRequired !== true) {
    errors.push("credentialRotationPolicy.secretRedactionRequired must stay true");
  }
}

function validateTaskGraph(contract, graph, errors) {
  const taskID = contract.taskGraph?.taskId;
  const task = values(graph.tasks).find((item) => item.id === taskID);
  if (!task) {
    errors.push(`task graph must include ${taskID}`);
    return;
  }
  if (!values(contract.taskGraph?.allowedStatusesBeforePromotion).includes(task.status) && task.status !== contract.taskGraph?.promotionStatus) {
    errors.push(`${taskID} has unsupported status ${task.status}`);
  }
  for (const field of ["statusReason", "completionGate"]) {
    if (!task[field]?.zh || !task[field]?.en) {
      errors.push(`${taskID} must declare zh/en ${field}`);
    }
  }
}

function validateDocuments(contract, errors) {
  for (const relativePath of [...values(contract.documents), ...values(contract.validators)]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`production auth hardening path is missing or unsafe: ${relativePath}`);
    }
  }
  const authDoc = fs.readFileSync(path.resolve(repoRoot, "docs/platform-auth.md"), "utf8");
  for (const phrase of ["sliding session renewal, not a separate refresh-token rotation model", "Token Rotation Policy", "Provider Promotion Matrix", "httpapi.AppIdentityResolver", "ErrProviderResolveFailed"]) {
    if (!authDoc.includes(phrase)) {
      errors.push(`docs/platform-auth.md must mention ${phrase}`);
    }
  }
}

function validateAuditPolicy(contract, errors) {
  requireIncludes(
    contract.auditPolicy?.requiredActions,
    ["auth.login", "auth.refresh", "auth.logout", "app.auth.login", "app.auth.logout", "api_token.create", "api_token.update", "api_token.revoke"],
    "auditPolicy.requiredActions",
    errors,
  );
  requireIncludes(
    contract.auditPolicy?.forbiddenRawFields,
    ["jwt", "bearerToken", "refreshToken", "apiToken", "openid", "unionid", "phone", "secret", "sessionId", "sessionDigest", "tokenDigest"],
    "auditPolicy.forbiddenRawFields",
    errors,
  );
  requireIncludes(
    contract.auditPolicy?.allowedAuthAuditFields,
    ["actor", "action", "resource", "provider", "createdAt"],
    "auditPolicy.allowedAuthAuditFields",
    errors,
  );
  const allowedFields = new Set(values(contract.auditPolicy?.allowedAuthAuditFields));
  for (const field of values(contract.auditPolicy?.forbiddenRawFields)) {
    if (allowedFields.has(field)) {
      errors.push(`auditPolicy.allowedAuthAuditFields must not include forbidden raw field ${field}`);
    }
  }
  if (contract.auditPolicy?.sessionIdentifier !== "none") {
    errors.push("auditPolicy.sessionIdentifier must stay none");
  }
  for (const relativePath of values(contract.auditPolicy?.runtimeEvidence)) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`auditPolicy.runtimeEvidence path is missing or unsafe: ${relativePath}`);
    }
  }
  const serverPath = "internal/platform/httpapi/server.go";
  if (relativeExistingPath(serverPath)) {
    const server = fs.readFileSync(path.resolve(repoRoot, serverPath), "utf8");
    const recordAuditStart = server.indexOf("func (s *Server) recordAudit(");
    const nextFunctionStart = server.indexOf("func newAuthAuditCode(", recordAuditStart);
    const recordAuditBody = recordAuditStart >= 0 && nextFunctionStart > recordAuditStart ? server.slice(recordAuditStart, nextFunctionStart) : "";
    if (!recordAuditBody) {
      errors.push(`${serverPath} must expose recordAudit before newAuthAuditCode`);
    }
    if (!recordAuditBody.includes("func (s *Server) recordAudit(code string, name string, username string, provider string) error")) {
      errors.push(`${serverPath} recordAudit must not accept a session credential parameter`);
    }
    for (const snippet of [
      '"actor":     username',
      '"action":    code',
      '"resource":  "auth"',
      '"createdAt": s.now().UTC().Format(time.RFC3339)',
      'values["provider"] = provider',
    ]) {
      if (!recordAuditBody.includes(snippet)) {
        errors.push(`${serverPath} recordAudit must include audit whitelist evidence ${snippet}`);
      }
    }
    for (const field of values(contract.auditPolicy?.forbiddenRawFields)) {
      const quotedField = `"${field}"`;
      if (recordAuditBody.includes(quotedField)) {
        errors.push(`${serverPath} recordAudit must not store forbidden raw audit field ${field}`);
      }
    }
    if (server.includes("func shortSessionID(")) {
      errors.push(`${serverPath} must not expose shortSessionID`);
    }
  }
  const authDocPath = "docs/platform-auth.md";
  if (relativeExistingPath(authDocPath)) {
    const authDoc = fs.readFileSync(path.resolve(repoRoot, authDocPath), "utf8");
    for (const phrase of ["does not store the raw session handle, its digest or a shortened derivative", "omitting raw JWT secrets, raw bearer tokens, OpenID, UnionID, phone numbers and API token values"]) {
      if (!authDoc.includes(phrase)) {
        errors.push(`${authDocPath} must document auth audit redaction phrase: ${phrase}`);
      }
    }
  }
}

function validateAuditDocumentation(errors) {
  const documents = [
    {
      path: sessionPolicyDocPath,
      requirements: [
        ["Persisted session identifiers use the canonical `sha256:v1:` prefix followed by exactly 64 lowercase hexadecimal characters.", "session policy must define canonical sha256:v1 digests with 64 lowercase hexadecimal characters"],
        ["Audit records must not store the raw session handle, its digest, or any shortened derivative.", "session policy must forbid raw session handles, digests and shortened derivatives in audits"],
        ["The generic audit schema has no `sessionId` field", "session policy must state that generic audit schema has no sessionId field"],
      ],
    },
    {
      path: oidcDesignDocPath,
      requirements: [
        ["OIDC audit records must not store the raw session handle, its digest, or any shortened derivative.", "OIDC design must forbid raw session handles, digests and shortened derivatives in audits"],
        ["The persisted OIDC audit schema does not expose a `sessionId` field.", "OIDC design must state that persisted audit schema has no sessionId field"],
      ],
    },
    {
      path: adminResourceSchemaDocPath,
      requirements: [
        ["The audit schema does not expose `sessionId`", "admin resource schema must state that audit schema has no sessionId field"],
        ["raw session handles, session digests or shortened derivatives", "admin resource schema must forbid session credentials and derivatives in audit schema"],
      ],
    },
  ];

  for (const document of documents) {
    if (!fs.existsSync(document.path)) {
      errors.push(`audit documentation path is missing: ${document.path}`);
      continue;
    }
    const content = fs.readFileSync(document.path, "utf8");
    for (const [phrase, error] of document.requirements) {
      if (!content.includes(phrase)) {
        errors.push(error);
      }
    }
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const graph = readJSON(taskGraphPath);
  const readiness = readJSON(readinessPath);
  const capabilityAudit = readJSON(capabilityAuditPath);
  const refreshTokenFamilyPromotion = readJSON(refreshTokenFamilyPromotionPath);
  const promotionReview = readJSON(promotionReviewPath);
  const errors = [];

  if (!contract.purpose) {
    errors.push("production auth hardening purpose is required");
  }
  validateCredentialClasses(contract, errors);
  validateSessionPolicy(contract, errors);
  validateProviderPolicy(contract, errors);
  validateProductionPromotionApprovalPackage(contract, errors);
  validateProductionPromotionReview(contract, refreshTokenFamilyPromotion, readiness, promotionReview, errors);
  validateProviderRuntimePolicy(contract, errors);
  validateProviderPromotionMatrix(contract, capabilityAudit, errors);
  validateRotationPolicy(contract, readiness, errors);
  validateTaskGraph(contract, graph, errors);
  validateAuditPolicy(contract, errors);
  validateAuditDocumentation(errors);
  validateDocuments(contract, errors);

  return { contract, errors };
}

const { errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated platform production auth hardening in ${path.relative(repoRoot, contractPath)}`);
