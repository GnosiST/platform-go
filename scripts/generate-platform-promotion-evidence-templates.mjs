import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "platform-promotion-evidence-templates.json");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const codegenReadinessPath = path.resolve(repoRoot, argValue("--codegen-readiness", "resources/platform-codegen-source-writing-readiness.json"));
const taskExecutionPath = path.resolve(repoRoot, argValue("--task-execution", "resources/platform-task-execution-audit.json"));

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function uniqueSorted(items) {
  return Array.from(new Set(values(items))).sort();
}

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function relativeToRepo(filePath) {
  return toPosix(path.relative(repoRoot, filePath));
}

function emptyValueForField(field) {
  if (
    [
      "verificationCommands",
      "rollbackCommands",
      "auditSampleRefs",
      "providerRotationRunbookRefs",
      "refreshTokenFamilyTestRefs",
      "providerIds",
      "providerControls",
      "runtimeTestRefs",
      "targetFamilies",
      "runtimeTargets",
    ].includes(field)
  ) {
    return [];
  }
  return "";
}

function templateForEvidence(evidence, schema) {
  const template = {
    id: evidence.id ?? "",
    owner: evidence.owner ?? "",
    evidenceKind: evidence.evidenceKind ?? "",
    description: evidence.description ?? "",
    status: "missing",
  };

  for (const field of values(schema.requiredFields)) {
    if (template[field] === undefined) {
      template[field] = emptyValueForField(field);
    }
  }

  return template;
}

function codegenReviewContext(codegenReadiness) {
  return {
    reviewArtifact: "resources/generated/admin-scaffold-promotion-review.json",
    sourceWriting: codegenReadiness.mode?.sourceWriting ?? "",
    runtimeMutation: "disabled",
    promotionDecision: "not-approved",
    targetFamilies: values(codegenReadiness.targetFamilies).map((family) => ({
      id: family.id,
      purpose: family.purpose,
      scaffoldRoles: values(family.scaffoldRoles),
      runtimeTargets: values(family.runtimeTargets),
      testCommands: values(family.testCommands),
    })),
    preflightCommands: values(codegenReadiness.preflightCommands),
    promotionRules: values(codegenReadiness.promotionRules).map((rule) => ({
      id: rule.id,
      description: rule.description,
      required: rule.required === true,
    })),
  };
}

function productionAuthReviewContext(productionAuth) {
  const providers = values(productionAuth.providerPromotionMatrix?.providers);
  return {
    reviewArtifact: productionAuth.promotionReview?.path ?? "",
    runtimeMutation: productionAuth.promotionReview?.runtimeMutation ?? "",
    promotionDecision: productionAuth.promotionReview?.decision ?? "",
    providerIds: providers.map((provider) => provider.id).filter(Boolean),
    requiredRuntimeTests: values(productionAuth.providerRuntimePolicy?.requiredTests),
    requiredProviderControls: uniqueSorted(providers.flatMap((provider) => values(provider.requiredControls))),
  };
}

function packageTemplate({ id, taskId, contractSource, approvalPackage, blocker, reviewContext }) {
  const schema = approvalPackage.completedEvidenceSchema ?? {};
  return {
    id,
    taskId,
    source: contractSource,
    status: "draft-template",
    approvalPackageStatus: approvalPackage.status ?? "",
    defaultRuntimeMutation: approvalPackage.defaultRuntimeMutation ?? "",
    requiredApprovals: values(approvalPackage.requiredApprovals),
    requiredEvidenceCount: values(approvalPackage.requiredEvidence).length,
    missingEvidenceCount: values(approvalPackage.requiredEvidence).length,
    completionBlockers: values(blocker?.runtimeMutationBlockedWhile),
    requiredBeforeCompletion: values(blocker?.requiredEvidenceBeforePromotion),
    completedEvidenceSchema: {
      requiredFields: values(schema.requiredFields),
      approvalRules: values(schema.approvalRules),
      forbiddenFields: values(schema.forbiddenFields),
      artifactHashPolicy: schema.artifactHashPolicy ?? {},
      artifactURIPolicy: schema.artifactURIPolicy ?? {},
    },
    reviewContext,
    evidenceTemplates: values(approvalPackage.requiredEvidence).map((evidence) => templateForEvidence(evidence, schema)),
  };
}

const productionAuth = readJSON(productionAuthPath);
const codegenReadiness = readJSON(codegenReadinessPath);
const taskExecution = readJSON(taskExecutionPath);
const blockersByTask = new Map(values(taskExecution.knownPromotionBlockers).map((blocker) => [blocker.taskId, blocker]));

const productionAuthTask = productionAuth.taskGraph?.taskId ?? "production-auth-provider-hardening";
const codegenTask = "source-writing-codegen-promotion";

const output = {
  generatedBy: "scripts/generate-platform-promotion-evidence-templates.mjs",
  purpose: "Draft-only evidence templates for the controlled platform promotion gates. These templates are not approval evidence and must not enable runtime mutation.",
  mode: {
    templateOnly: true,
    runtimeMutation: "disabled",
    sourceWriting: "disabled",
    approvalState: "not-submitted",
  },
  sources: {
    productionAuthHardening: relativeToRepo(productionAuthPath),
    codegenSourceWritingReadiness: relativeToRepo(codegenReadinessPath),
    taskExecutionAudit: relativeToRepo(taskExecutionPath),
  },
  submittedEvidenceValidator: {
    script: "scripts/validate-platform-promotion-evidence-package.mjs",
    command: "rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>",
    purpose: "Validates externally submitted completed evidence packages against the draft template schema without mutating platform runtime contracts or marking blockers complete.",
    tests: ["scripts/platform-promotion-evidence-package.test.mjs"],
  },
  draftPackageGenerator: {
    script: "scripts/generate-platform-promotion-evidence-package-draft.mjs",
    command: "rtk node scripts/generate-platform-promotion-evidence-package-draft.mjs",
    purpose: "Generates a non-submitted draft evidence package that external reviewers can fill before running the submitted evidence validator.",
    generatedFiles: ["resources/generated/platform-promotion-evidence-package-draft.json"],
    tests: ["scripts/platform-promotion-evidence-package-draft.test.mjs"],
  },
  packages: [
    packageTemplate({
      id: "production-auth-promotion",
      taskId: productionAuthTask,
      contractSource: relativeToRepo(productionAuthPath),
      approvalPackage: productionAuth.productionPromotionApprovalPackage ?? {},
      blocker: blockersByTask.get(productionAuthTask),
      reviewContext: productionAuthReviewContext(productionAuth),
    }),
    packageTemplate({
      id: "source-writing-codegen-promotion",
      taskId: codegenTask,
      contractSource: relativeToRepo(codegenReadinessPath),
      approvalPackage: codegenReadiness.sourceWritingApprovalPackage ?? {},
      blocker: blockersByTask.get(codegenTask),
      reviewContext: codegenReviewContext(codegenReadiness),
    }),
  ],
};

fs.mkdirSync(generatedDir, { recursive: true });
const temporaryPath = `${generatedPath}.tmp-${process.pid}`;
fs.writeFileSync(temporaryPath, `${JSON.stringify(output, null, 2)}\n`);
fs.renameSync(temporaryPath, generatedPath);
console.log(`Generated ${relativeToRepo(generatedPath)}`);
