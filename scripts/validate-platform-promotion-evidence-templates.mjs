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

const templatesPath = path.resolve(repoRoot, argValue("--templates", "resources/generated/platform-promotion-evidence-templates.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const codegenReadinessPath = path.resolve(repoRoot, argValue("--codegen-readiness", "resources/platform-codegen-source-writing-readiness.json"));
const taskExecutionPath = path.resolve(repoRoot, argValue("--task-execution", "resources/platform-task-execution-audit.json"));

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

function requireIncludes(items, expected, label, errors) {
  const actual = new Set(values(items));
  for (const value of expected) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function sameJSON(actual, expected) {
  return JSON.stringify(actual ?? {}) === JSON.stringify(expected ?? {});
}

function validateSourceWritingReviewContext(pkg, codegenReadiness, errors) {
  const context = pkg?.reviewContext ?? {};
  const packageId = pkg?.id ?? "source-writing-codegen-promotion";
  const prefix = `${packageId}.reviewContext`;
  if (context.reviewArtifact !== "resources/generated/admin-scaffold-promotion-review.json") {
    errors.push(`${prefix}.reviewArtifact must be resources/generated/admin-scaffold-promotion-review.json`);
  }
  if (context.sourceWriting !== "disabled" || context.sourceWriting !== codegenReadiness.mode?.sourceWriting) {
    errors.push(`${prefix}.sourceWriting must stay disabled`);
  }
  if (context.runtimeMutation !== "disabled") {
    errors.push(`${prefix}.runtimeMutation must stay disabled`);
  }
  if (context.promotionDecision !== "not-approved") {
    errors.push(`${prefix}.promotionDecision must stay not-approved`);
  }
  requireIncludes(context.preflightCommands, values(codegenReadiness.preflightCommands), `${prefix}.preflightCommands`, errors);
  requireIncludes(
    values(context.promotionRules).map((rule) => rule.id),
    values(codegenReadiness.promotionRules).map((rule) => rule.id),
    `${prefix}.promotionRules`,
    errors,
  );

  const contextFamilies = values(context.targetFamilies);
  const familiesByID = new Map(contextFamilies.map((family) => [family.id, family]));
  requireIncludes(
    contextFamilies.map((family) => family.id),
    values(codegenReadiness.targetFamilies).map((family) => family.id),
    `${prefix}.targetFamilies`,
    errors,
  );
  for (const sourceFamily of values(codegenReadiness.targetFamilies)) {
    const family = familiesByID.get(sourceFamily.id);
    if (!family) continue;
    requireIncludes(family.scaffoldRoles, values(sourceFamily.scaffoldRoles), `${prefix}.${sourceFamily.id}.scaffoldRoles`, errors);
    requireIncludes(family.runtimeTargets, values(sourceFamily.runtimeTargets), `${prefix}.${sourceFamily.id}.runtimeTargets`, errors);
    requireIncludes(family.testCommands, values(sourceFamily.testCommands), `${prefix}.${sourceFamily.id}.testCommands`, errors);
  }
}

function validateProductionAuthReviewContext(pkg, productionAuth, errors) {
  const context = pkg?.reviewContext ?? {};
  const packageId = pkg?.id ?? "production-auth-promotion";
  const prefix = `${packageId}.reviewContext`;
  if (context.reviewArtifact !== productionAuth.promotionReview?.path) {
    errors.push(`${prefix}.reviewArtifact must match production auth promotion review path`);
  }
  if (context.runtimeMutation !== "disabled" || context.runtimeMutation !== productionAuth.promotionReview?.runtimeMutation) {
    errors.push(`${prefix}.runtimeMutation must stay disabled`);
  }
  if (context.promotionDecision !== "not-approved" || context.promotionDecision !== productionAuth.promotionReview?.decision) {
    errors.push(`${prefix}.promotionDecision must stay not-approved`);
  }
  requireIncludes(
    context.providerIds,
    values(productionAuth.providerPromotionMatrix?.providers).map((provider) => provider.id),
    `${prefix}.providerIds`,
    errors,
  );
  requireIncludes(
    context.requiredRuntimeTests,
    values(productionAuth.providerRuntimePolicy?.requiredTests),
    `${prefix}.requiredRuntimeTests`,
    errors,
  );
}

function validateMode(templates, errors) {
  if (templates.mode?.templateOnly !== true) {
    errors.push("mode.templateOnly must stay true");
  }
  if (templates.mode?.runtimeMutation !== "disabled") {
    errors.push("mode.runtimeMutation must stay disabled");
  }
  if (templates.mode?.sourceWriting !== "disabled") {
    errors.push("mode.sourceWriting must stay disabled");
  }
  if (templates.mode?.approvalState !== "not-submitted") {
    errors.push("mode.approvalState must stay not-submitted");
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

function validateSubmittedEvidenceValidator(templates, errors) {
  const validator = templates.submittedEvidenceValidator ?? {};
  const expectedScript = "scripts/validate-platform-promotion-evidence-package.mjs";
  const expectedTest = "scripts/platform-promotion-evidence-package.test.mjs";
  if (validator.script !== expectedScript) {
    errors.push(`submittedEvidenceValidator.script must be ${expectedScript}`);
  }
  if (!relativeExistingPath(validator.script)) {
    errors.push("submittedEvidenceValidator.script path is missing or unsafe");
  }
  if (!String(validator.command ?? "").startsWith(`rtk node ${expectedScript}`)) {
    errors.push(`submittedEvidenceValidator.command must start with rtk node ${expectedScript}`);
  }
  if (!String(validator.command ?? "").includes("--package <promotion-evidence-package>")) {
    errors.push("submittedEvidenceValidator.command must include --package <promotion-evidence-package>");
  }
  if (!validator.purpose) {
    errors.push("submittedEvidenceValidator.purpose must be declared");
  }
  if (!values(validator.tests).includes(expectedTest)) {
    errors.push(`submittedEvidenceValidator.tests must include ${expectedTest}`);
  }
  for (const testPath of values(validator.tests)) {
    if (!relativeExistingPath(testPath)) {
      errors.push(`submittedEvidenceValidator test path is missing or unsafe: ${testPath}`);
    }
  }
}

function validateDraftPackageGenerator(templates, errors) {
  const generator = templates.draftPackageGenerator ?? {};
  const expectedScript = "scripts/generate-platform-promotion-evidence-package-draft.mjs";
  const expectedGeneratedFile = "resources/generated/platform-promotion-evidence-package-draft.json";
  const expectedTest = "scripts/platform-promotion-evidence-package-draft.test.mjs";
  if (generator.script !== expectedScript) {
    errors.push(`draftPackageGenerator.script must be ${expectedScript}`);
  }
  if (!relativeExistingPath(generator.script)) {
    errors.push("draftPackageGenerator.script path is missing or unsafe");
  }
  if (!String(generator.command ?? "").startsWith(`rtk node ${expectedScript}`)) {
    errors.push(`draftPackageGenerator.command must start with rtk node ${expectedScript}`);
  }
  if (!generator.purpose) {
    errors.push("draftPackageGenerator.purpose must be declared");
  }
  if (!values(generator.generatedFiles).includes(expectedGeneratedFile)) {
    errors.push(`draftPackageGenerator.generatedFiles must include ${expectedGeneratedFile}`);
  }
  for (const generatedFile of values(generator.generatedFiles)) {
    if (!generatedFile.startsWith("resources/generated/") || path.isAbsolute(generatedFile)) {
      errors.push(`draftPackageGenerator generated file path is unsafe: ${generatedFile}`);
    }
  }
  if (!values(generator.tests).includes(expectedTest)) {
    errors.push(`draftPackageGenerator.tests must include ${expectedTest}`);
  }
  for (const testPath of values(generator.tests)) {
    if (!relativeExistingPath(testPath)) {
      errors.push(`draftPackageGenerator test path is missing or unsafe: ${testPath}`);
    }
  }
}

function validatePackage({ pkg, taskId, source, approvalPackage, blocker, errors }) {
  const prefix = `promotion evidence template package ${pkg?.id ?? "<missing>"}`;
  if (!pkg) {
    errors.push(`missing promotion evidence template package for ${taskId}`);
    return;
  }
  if (pkg.taskId !== taskId) {
    errors.push(`${prefix}.taskId must be ${taskId}`);
  }
  if (pkg.source !== source) {
    errors.push(`${prefix}.source must be ${source}`);
  }
  if (pkg.status !== "draft-template") {
    errors.push(`${prefix}.status must stay draft-template`);
  }
  if (pkg.approvalPackageStatus !== approvalPackage.status) {
    errors.push(`${prefix}.approvalPackageStatus must match source approval package status`);
  }
  if (pkg.defaultRuntimeMutation !== approvalPackage.defaultRuntimeMutation) {
    errors.push(`${prefix}.defaultRuntimeMutation must match source approval package`);
  }
  requireIncludes(pkg.requiredApprovals, values(approvalPackage.requiredApprovals), `${prefix}.requiredApprovals`, errors);
  requireIncludes(pkg.completionBlockers, values(blocker?.runtimeMutationBlockedWhile), `${prefix}.completionBlockers`, errors);
  requireIncludes(pkg.requiredBeforeCompletion, values(blocker?.requiredEvidenceBeforePromotion), `${prefix}.requiredBeforeCompletion`, errors);

  const requiredEvidence = values(approvalPackage.requiredEvidence);
  const requiredEvidenceIDs = requiredEvidence.map((item) => item.id);
  const templateIDs = values(pkg.evidenceTemplates).map((template) => template.id);
  requireIncludes(templateIDs, requiredEvidenceIDs, `${prefix}.evidenceTemplates`, errors);
  requireIncludes(requiredEvidenceIDs, templateIDs, `${prefix}.requiredEvidence`, errors);
  if (pkg.requiredEvidenceCount !== requiredEvidence.length) {
    errors.push(`${prefix}.requiredEvidenceCount must be ${requiredEvidence.length}`);
  }
  if (pkg.missingEvidenceCount !== requiredEvidence.length) {
    errors.push(`${prefix}.missingEvidenceCount must equal required evidence count for draft templates`);
  }

  const schema = approvalPackage.completedEvidenceSchema ?? {};
  requireIncludes(pkg.completedEvidenceSchema?.requiredFields, values(schema.requiredFields), `${prefix}.completedEvidenceSchema.requiredFields`, errors);
  requireIncludes(pkg.completedEvidenceSchema?.approvalRules, values(schema.approvalRules), `${prefix}.completedEvidenceSchema.approvalRules`, errors);
  requireIncludes(pkg.completedEvidenceSchema?.forbiddenFields, values(schema.forbiddenFields), `${prefix}.completedEvidenceSchema.forbiddenFields`, errors);
  if (!sameJSON(pkg.completedEvidenceSchema?.artifactHashPolicy, schema.artifactHashPolicy)) {
    errors.push(`${prefix}.completedEvidenceSchema.artifactHashPolicy must match source approval package`);
  }
  if (!sameJSON(pkg.completedEvidenceSchema?.artifactURIPolicy, schema.artifactURIPolicy)) {
    errors.push(`${prefix}.completedEvidenceSchema.artifactURIPolicy must match source approval package`);
  }

  const evidenceByID = new Map(requiredEvidence.map((item) => [item.id, item]));
  for (const template of values(pkg.evidenceTemplates)) {
    const itemPrefix = `${prefix}.evidenceTemplates.${template.id ?? "<missing>"}`;
    const required = evidenceByID.get(template.id);
    if (!required) {
      errors.push(`${itemPrefix} is not declared in source requiredEvidence`);
      continue;
    }
    if (template.status !== "missing") {
      errors.push(`${itemPrefix}.status must stay missing`);
    }
    if (template.owner !== required.owner) {
      errors.push(`${itemPrefix}.owner must match source requiredEvidence owner`);
    }
    if (template.evidenceKind !== required.evidenceKind) {
      errors.push(`${itemPrefix}.evidenceKind must match source requiredEvidence kind`);
    }
    for (const field of values(schema.requiredFields)) {
      if (!(field in template)) {
        errors.push(`${itemPrefix} is missing required field ${field}`);
      }
    }
    for (const field of values(schema.forbiddenFields)) {
      if (field in template) {
        errors.push(`${itemPrefix} must not include forbidden field ${field}`);
      }
    }
    if (template.approvedBy || template.approvedAt || template.artifactURI || template.artifactHash) {
      errors.push(`${itemPrefix} must not contain completed approval values in the draft template`);
    }
  }
}

function validate() {
  const templates = readJSON(templatesPath);
  const productionAuth = readJSON(productionAuthPath);
  const codegenReadiness = readJSON(codegenReadinessPath);
  const taskExecution = readJSON(taskExecutionPath);
  const blockersByTask = new Map(values(taskExecution.knownPromotionBlockers).map((blocker) => [blocker.taskId, blocker]));
  const errors = [];

  if (templates.generatedBy !== "scripts/generate-platform-promotion-evidence-templates.mjs") {
    errors.push("generatedBy must stay scripts/generate-platform-promotion-evidence-templates.mjs");
  }
  validateMode(templates, errors);
  if (templates.sources?.productionAuthHardening !== relativeToRepo(productionAuthPath)) {
    errors.push("sources.productionAuthHardening must match production auth contract path");
  }
  if (templates.sources?.codegenSourceWritingReadiness !== relativeToRepo(codegenReadinessPath)) {
    errors.push("sources.codegenSourceWritingReadiness must match codegen readiness contract path");
  }
  if (templates.sources?.taskExecutionAudit !== relativeToRepo(taskExecutionPath)) {
    errors.push("sources.taskExecutionAudit must match task execution audit path");
  }
  validateSubmittedEvidenceValidator(templates, errors);
  validateDraftPackageGenerator(templates, errors);

  const packagesByID = new Map(values(templates.packages).map((pkg) => [pkg.id, pkg]));
  const productionAuthTask = productionAuth.taskGraph?.taskId ?? "production-auth-provider-hardening";
  const codegenTask = "source-writing-codegen-promotion";
  validatePackage({
    pkg: packagesByID.get("production-auth-promotion"),
    taskId: productionAuthTask,
    source: relativeToRepo(productionAuthPath),
    approvalPackage: productionAuth.productionPromotionApprovalPackage ?? {},
    blocker: blockersByTask.get(productionAuthTask),
    errors,
  });
  validateProductionAuthReviewContext(packagesByID.get("production-auth-promotion"), productionAuth, errors);
  validatePackage({
    pkg: packagesByID.get("source-writing-codegen-promotion"),
    taskId: codegenTask,
    source: relativeToRepo(codegenReadinessPath),
    approvalPackage: codegenReadiness.sourceWritingApprovalPackage ?? {},
    blocker: blockersByTask.get(codegenTask),
    errors,
  });
  validateSourceWritingReviewContext(packagesByID.get("source-writing-codegen-promotion"), codegenReadiness, errors);

  return { templates, errors };
}

const { templates, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${values(templates.packages).length} platform promotion evidence template packages`);
