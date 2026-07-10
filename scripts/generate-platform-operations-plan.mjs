import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "platform-operations-plan.json");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

function relativePath(filePath) {
  return path.relative(repoRoot, filePath).split(path.sep).join("/");
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-production-readiness.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const readiness = JSON.parse(fs.readFileSync(readinessPath, "utf8"));
const productionAuth = JSON.parse(fs.readFileSync(productionAuthPath, "utf8"));
const preflightCommands = values(readiness.preflightCommands).map((command) => ({
  id: command.id,
  command: command.command,
  purpose: command.purpose,
}));
const commandsByID = new Map(preflightCommands.map((command) => [command.id, command]));
const policyPreflightRequirements = new Map(
  values(readiness.policyPreflightRequirements).map((requirement) => [requirement.policy, values(requirement.requiredCommands)]),
);
const policies = values(readiness.operationPolicies).map((policy) => ({
  id: policy.id,
  purpose: policy.purpose,
  docs: values(policy.docs),
  preflightCommands: values(policy.preflightCommands),
  requiredPreflightCommands: values(policyPreflightRequirements.get(policy.id)),
  missingRequiredPreflightCommands: values(policyPreflightRequirements.get(policy.id)).filter(
    (commandID) => !values(policy.preflightCommands).includes(commandID),
  ),
  preflightCommandDetails: values(policy.preflightCommands).map((id) => commandsByID.get(id) ?? { id, missing: true }),
  requiresHumanReview: policy.requiresHumanReview === true,
  rollbackRequirement: policy.rollbackRequirement ?? "",
  auditRequirement: policy.auditRequirement ?? "",
  prohibitedActions: values(policy.prohibitedActions),
}));
const providerPromotionMatrix = productionAuth.providerPromotionMatrix ?? {};
const providerPromotionProviders = values(providerPromotionMatrix.providers).map((provider) => ({
  id: provider.id,
  capability: provider.capability,
  kind: provider.kind,
  productionUsage: provider.productionUsage,
  adapterBoundary: provider.adapterBoundary,
  configKeys: values(provider.configKeys),
  requiredControls: values(provider.requiredControls),
  requiresSecretOwner: provider.requiresSecretOwner === true,
  rotationRunbookRequired: provider.rotationRunbookRequired === true,
  subjectRedactionRequired: provider.subjectRedactionRequired === true,
  unconfiguredProviderRejectionRequired: provider.unconfiguredProviderRejectionRequired === true,
  errorNormalizationRequired: provider.errorNormalizationRequired === true,
  rawCredentialExposureAllowed: provider.rawCredentialExposureAllowed === true,
  rawSubjectExposureAllowed: provider.rawSubjectExposureAllowed === true,
}));
const approvalPackage = productionAuth.productionPromotionApprovalPackage ?? {};
const productionPromotionApprovalPackage = {
  source: relativePath(productionAuthPath),
  status: approvalPackage.status ?? "",
  sourceOfTruth: approvalPackage.sourceOfTruth ?? "",
  defaultRuntimeMutation: approvalPackage.defaultRuntimeMutation ?? "",
  requiredApprovals: values(approvalPackage.requiredApprovals),
  requiredEvidence: values(approvalPackage.requiredEvidence).map((item) => ({
    id: item.id,
    owner: item.owner,
    evidenceKind: item.evidenceKind,
    description: item.description,
  })),
  completedEvidence: values(approvalPackage.completedEvidence),
  prohibitedEvidence: values(approvalPackage.prohibitedEvidence),
  completedEvidenceSchema: {
    requiredFields: values(approvalPackage.completedEvidenceSchema?.requiredFields),
    approvalRules: values(approvalPackage.completedEvidenceSchema?.approvalRules),
    forbiddenFields: values(approvalPackage.completedEvidenceSchema?.forbiddenFields),
    artifactHashPolicy: approvalPackage.completedEvidenceSchema?.artifactHashPolicy ?? {},
    artifactURIPolicy: approvalPackage.completedEvidenceSchema?.artifactURIPolicy ?? {},
  },
  mustNotEnableRefreshTokenFamily: approvalPackage.mustNotEnableRefreshTokenFamily === true,
  mustNotEnableUnreviewedProvider: approvalPackage.mustNotEnableUnreviewedProvider === true,
};

const plan = {
  generatedBy: "scripts/generate-platform-operations-plan.mjs",
  source: relativePath(readinessPath),
  productionAuthHardeningSource: relativePath(productionAuthPath),
  sourceCapturedAt: readiness.capturedAt ?? null,
  productionAuthHardeningCapturedAt: productionAuth.capturedAt ?? null,
  mode: {
    dryRun: true,
    runtimeMutation: "disabled",
    sourceWriting: "disabled",
  },
  summary: {
    policyCount: policies.length,
    preflightCommandCount: preflightCommands.length,
    humanReviewPolicyCount: policies.filter((policy) => policy.requiresHumanReview).length,
    prohibitedActionCount: policies.reduce((count, policy) => count + policy.prohibitedActions.length, 0),
    providerPromotionCount: providerPromotionProviders.length,
    optionalProductionProviderCount: providerPromotionProviders.filter((provider) => provider.productionUsage === "optional-production-provider").length,
    productionPromotionRequiredEvidenceCount: productionPromotionApprovalPackage.requiredEvidence.length,
  },
  guardrails: [
    "This plan is generated from the production readiness contract and is non-mutating.",
    "It must not add import, restore, migration, credential or source-writing runtime APIs.",
    "Production operators must run the referenced preflight commands and keep review, rollback and audit evidence outside this generated artifact.",
  ],
  preflightRunner: readiness.preflightRunner ?? {},
  preflightCommands,
  policies,
  providerPromotionMatrix: {
    source: relativePath(productionAuthPath),
    defaultPolicy: providerPromotionMatrix.defaultPolicy ?? "",
    newProviderRequirements: values(providerPromotionMatrix.newProviderRequirements),
    providers: providerPromotionProviders,
  },
  productionPromotionApprovalPackage,
};

const output = `${JSON.stringify(plan, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${relativePath(generatedPath)}`);
}
