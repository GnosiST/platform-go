import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "production-auth-promotion-review.json");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function relativeToRepo(filePath) {
  return toPosix(path.relative(repoRoot, filePath));
}

function evidenceID(item) {
  if (typeof item === "string") {
    return item;
  }
  return item?.id ?? "";
}

const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const refreshTokenFamilyPath = path.resolve(repoRoot, argValue("--refresh-token-family-promotion", "resources/platform-refresh-token-family-promotion.json"));
const readinessPath = path.resolve(repoRoot, argValue("--production-readiness", "resources/platform-production-readiness.json"));
const taskExecutionAuditPath = path.resolve(repoRoot, argValue("--task-execution-audit", "resources/platform-task-execution-audit.json"));

const productionAuth = readJSON(productionAuthPath);
const refreshTokenFamily = readJSON(refreshTokenFamilyPath);
const readiness = readJSON(readinessPath);
const taskExecutionAudit = readJSON(taskExecutionAuditPath);

const approvalPackage = productionAuth.productionPromotionApprovalPackage ?? {};
const requiredEvidence = values(approvalPackage.requiredEvidence).map((item) => ({
  id: item.id,
  owner: item.owner,
  evidenceKind: item.evidenceKind,
  description: item.description,
}));
const completedEvidence = values(approvalPackage.completedEvidence).map((item) => evidenceID(item)).filter(Boolean);
const completedEvidenceSet = new Set(completedEvidence);
const missingEvidence = requiredEvidence.filter((item) => !completedEvidenceSet.has(item.id)).map((item) => item.id);
const productionAuthTaskID = productionAuth.taskGraph?.taskId ?? "production-auth-provider-hardening";
const taskBlocker = values(taskExecutionAudit.knownPromotionBlockers).find((item) => item.taskId === productionAuthTaskID);
const blockers = new Set();

if (productionAuth.sessionCredentialPolicy?.refreshTokenFamily?.defaultRuntime === "disabled") {
  blockers.add("refresh-token-family default runtime is disabled pending production approval");
}
if (refreshTokenFamily.promotionState?.implementationStatus !== "implemented") {
  blockers.add("refresh-token-family promotion implementation is blocked");
}
if (approvalPackage.status === "blocked") {
  blockers.add("production approval package is blocked");
}
if (missingEvidence.length > 0) {
  blockers.add(`production approval package is missing ${missingEvidence.length} required evidence artifacts`);
}
for (const blocker of values(taskBlocker?.runtimeMutationBlockedWhile)) {
  blockers.add(blocker);
}
const blockerList = [...blockers];

const preflightCommandIDs = values(readiness.preflightCommands).map((command) => command.id);
const tokenRotationPolicy = values(readiness.operationPolicies).find((policy) => policy.id === "token-rotation") ?? {};
const providerPromotionProviders = values(productionAuth.providerPromotionMatrix?.providers).map((provider) => ({
  id: provider.id,
  capability: provider.capability,
  kind: provider.kind,
  productionUsage: provider.productionUsage,
  adapterBoundary: provider.adapterBoundary,
  audiences: values(provider.audiences),
  configKeys: values(provider.configKeys),
  requiredControls: values(provider.requiredControls),
  requiresSecretOwner: provider.requiresSecretOwner === true,
  rotationRunbookRequired: provider.rotationRunbookRequired === true,
  subjectRedactionRequired: provider.subjectRedactionRequired === true,
  unconfiguredProviderRejectionRequired: provider.unconfiguredProviderRejectionRequired === true,
  errorNormalizationRequired: provider.errorNormalizationRequired === true,
  productionLikeRehearsalRequired: provider.productionLikeRehearsalRequired === true,
  rawCredentialExposureAllowed: provider.rawCredentialExposureAllowed === true,
  rawSubjectExposureAllowed: provider.rawSubjectExposureAllowed === true,
}));

const review = {
  generatedBy: "scripts/generate-production-auth-promotion-review.mjs",
  sources: {
    productionAuthHardening: relativeToRepo(productionAuthPath),
    refreshTokenFamilyPromotion: relativeToRepo(refreshTokenFamilyPath),
    productionReadiness: relativeToRepo(readinessPath),
    taskExecutionAudit: relativeToRepo(taskExecutionAuditPath),
  },
  sourceCapturedAt: {
    productionAuthHardening: productionAuth.capturedAt ?? null,
    refreshTokenFamilyPromotion: refreshTokenFamily.capturedAt ?? null,
    productionReadiness: readiness.capturedAt ?? null,
    taskExecutionAudit: taskExecutionAudit.capturedAt ?? null,
  },
  mode: {
    dryRun: true,
    runtimeMutation: "disabled",
    refreshTokenFamilyRuntime: "disabled",
    providerRuntimeMutation: "disabled",
    promotion: "manual-review-required",
  },
  summary: {
    requiredEvidenceCount: requiredEvidence.length,
    completedEvidenceCount: completedEvidence.length,
    missingEvidenceCount: missingEvidence.length,
    blockerCount: blockerList.length,
    providerPromotionCount: providerPromotionProviders.length,
    optionalProductionProviderCount: providerPromotionProviders.filter((provider) => provider.productionUsage === "optional-production-provider").length,
    tokenRotationPreflightCommandCount: values(tokenRotationPolicy.preflightCommands).length,
  },
  currentRuntime: {
    taskId: productionAuthTaskID,
    sessionModel: productionAuth.sessionCredentialPolicy?.slidingRenewal?.model ?? "",
    slidingRenewalStatus: productionAuth.sessionCredentialPolicy?.slidingRenewal?.status ?? "",
    notARefreshTokenFamily: productionAuth.sessionCredentialPolicy?.slidingRenewal?.notARefreshTokenFamily === true,
    refreshTokenFamilyStatus: productionAuth.sessionCredentialPolicy?.refreshTokenFamily?.status ?? "",
    refreshTokenFamilyPromotionStatus: refreshTokenFamily.promotionState?.implementationStatus ?? "",
  },
  approvalPackage: {
    status: approvalPackage.status ?? "",
    sourceOfTruth: approvalPackage.sourceOfTruth ?? "",
    defaultRuntimeMutation: approvalPackage.defaultRuntimeMutation ?? "",
    requiredApprovals: values(approvalPackage.requiredApprovals),
    requiredEvidence,
    completedEvidence,
    missingEvidence,
    prohibitedEvidence: values(approvalPackage.prohibitedEvidence),
    completedEvidenceSchema: {
      requiredFields: values(approvalPackage.completedEvidenceSchema?.requiredFields),
      approvalRules: values(approvalPackage.completedEvidenceSchema?.approvalRules),
      forbiddenFields: values(approvalPackage.completedEvidenceSchema?.forbiddenFields),
      artifactHashPolicy: approvalPackage.completedEvidenceSchema?.artifactHashPolicy ?? {},
      artifactURIPolicy: approvalPackage.completedEvidenceSchema?.artifactURIPolicy ?? {},
    },
  },
  refreshTokenFamilyGate: {
    allowedOnlyWhen: values(refreshTokenFamily.promotionState?.allowedOnlyWhen),
    dataModelRequiredFields: values(refreshTokenFamily.dataModelContract?.requiredFields),
    rotationRequiredSteps: values(refreshTokenFamily.rotationTransaction?.requiredSteps),
    reuseDetectionEffects: values(refreshTokenFamily.reuseDetection?.requiredEffects),
    auditForbiddenRawFields: values(refreshTokenFamily.auditPolicy?.forbiddenRawFields),
  },
  providerPromotionMatrix: {
    source: relativeToRepo(productionAuthPath),
    defaultPolicy: productionAuth.providerPromotionMatrix?.defaultPolicy ?? "",
    newProviderRequirements: values(productionAuth.providerPromotionMatrix?.newProviderRequirements),
    providers: providerPromotionProviders,
  },
  preflight: {
    productionReadinessCommandIds: preflightCommandIDs,
    tokenRotationPolicyCommands: values(tokenRotationPolicy.preflightCommands),
    tokenRotationPolicyRequiresHumanReview: tokenRotationPolicy.requiresHumanReview === true,
    tokenRotationRollbackRequirement: tokenRotationPolicy.rollbackRequirement ?? "",
    tokenRotationAuditRequirement: tokenRotationPolicy.auditRequirement ?? "",
  },
  manualReview: {
    required: true,
    decision: blockerList.length > 0 ? "not-approved" : "review-required",
    reasons: blockerList.length > 0 ? blockerList : ["External production promotion approval has not been attached."],
    requiredBeforeRuntimeMutation: true,
  },
  blockers: blockerList,
};

const output = `${JSON.stringify(review, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${toPosix(path.relative(repoRoot, generatedPath))}`);
}
