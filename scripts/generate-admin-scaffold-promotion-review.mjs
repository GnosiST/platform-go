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

const planPath = path.resolve(repoRoot, argValue("--scaffold-plan", "resources/generated/admin-scaffold-plan.json"));
const filesPath = path.resolve(repoRoot, argValue("--scaffold-files", "resources/generated/admin-scaffold-files.json"));
const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-codegen-source-writing-readiness.json"));
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "admin-scaffold-promotion-review.json");

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

function uniqueSorted(items) {
  return Array.from(new Set(items.filter(Boolean))).sort();
}

function rootPolicyForTarget(target) {
  const roots = values(readiness.runtimeTargetPolicy?.roots);
  const targetPath = path.resolve(repoRoot, target);
  return roots.find((root) => {
    const rootPath = path.resolve(repoRoot, root.path);
    const relative = path.relative(rootPath, targetPath);
    return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
  });
}

const plan = readJSON(planPath);
const scaffoldFiles = readJSON(filesPath);
const readiness = readJSON(readinessPath);
const generatedFilesByPath = new Map(values(scaffoldFiles.files).map((file) => [file.path, file]));

const candidateFiles = values(plan.resources).flatMap((resource) =>
  values(resource.candidateFiles).map((file) => ({
    resource: resource.resource,
    code: resource.code,
    role: file.role,
    path: file.path,
    eventualRuntimeTarget: file.eventualRuntimeTarget,
    status: file.status,
    conflict: file.conflict === true,
    generated: file.generated === true,
  })),
);

const targetFamilies = values(readiness.targetFamilies).map((family) => {
  const roleSet = new Set(values(family.scaffoldRoles));
  const reviewItems = candidateFiles
    .filter((file) => roleSet.has(file.role))
    .map((file) => {
      const generatedFile = generatedFilesByPath.get(file.path);
      return {
        resource: file.resource,
        role: file.role,
        scaffoldPath: file.path,
        eventualRuntimeTarget: file.eventualRuntimeTarget,
        status: file.status,
        conflict: file.conflict,
        generatedArtifact: Boolean(generatedFile),
        contentSha256: generatedFile?.contentSha256 ?? "",
        reviewRequired: true,
      };
    });
  return {
    id: family.id,
    purpose: family.purpose,
    scaffoldRoles: values(family.scaffoldRoles),
    runtimeTargets: values(family.runtimeTargets),
    runtimeTargetRoots: values(family.runtimeTargets).map((target) => {
      const root = rootPolicyForTarget(target);
      return {
        target,
        root: root?.path ?? "",
        status: root?.status ?? "",
        owner: root?.owner ?? "",
        requiresSeparateSpec: root?.requiresSeparateSpec === true,
      };
    }),
    testCommands: values(family.testCommands),
    summary: {
      resourceCount: uniqueSorted(reviewItems.map((item) => item.resource)).length,
      reviewItemCount: reviewItems.length,
      generatedArtifactCount: reviewItems.filter((item) => item.generatedArtifact).length,
      conflictCount: reviewItems.filter((item) => item.conflict).length,
    },
    reviewItems,
  };
});

const blockers = [];
if (plan.mode?.sourceWriting !== "disabled") {
  blockers.push("scaffold plan source writing is not disabled");
}
if (plan.mode?.dryRun !== true) {
  blockers.push("scaffold plan is not a dry run");
}
if (scaffoldFiles.mode?.sourceWriting !== "disabled") {
  blockers.push("scaffold files source writing is not disabled");
}
if (readiness.mode?.sourceWriting !== "disabled") {
  blockers.push("readiness contract source writing is not disabled");
}
if ((plan.summary?.conflictCount ?? 0) !== 0) {
  blockers.push(`scaffold plan has ${plan.summary.conflictCount} conflicts`);
}
if ((plan.summary?.unsafePathCount ?? 0) !== 0) {
  blockers.push(`scaffold plan has ${plan.summary.unsafePathCount} unsafe paths`);
}

const review = {
  generatedBy: "scripts/generate-admin-scaffold-promotion-review.mjs",
  sources: {
    scaffoldPlan: relativeToRepo(planPath),
    scaffoldFiles: relativeToRepo(filesPath),
    readiness: relativeToRepo(readinessPath),
  },
  sourceVersion: plan.sourceVersion,
  mode: {
    dryRun: true,
    sourceWriting: "disabled",
    runtimeMutation: "disabled",
    promotion: "manual-review-required",
  },
  summary: {
    targetFamilyCount: targetFamilies.length,
    scaffoldRoleCount: uniqueSorted(targetFamilies.flatMap((family) => family.scaffoldRoles)).length,
    resourceCount: plan.summary?.resourceCount ?? values(plan.resources).length,
    candidateResourceCount: plan.summary?.candidateResourceCount ?? 0,
    manualReviewResourceCount: plan.summary?.manualReviewResourceCount ?? 0,
    candidateFileCount: candidateFiles.length,
    generatedFileCount: values(scaffoldFiles.files).length,
    conflictCount: plan.summary?.conflictCount ?? 0,
    unsafePathCount: plan.summary?.unsafePathCount ?? 0,
    blockerCount: blockers.length,
  },
  reviewGates: {
    requiresExplicitSpec: readiness.mode?.requiresExplicitSpec === true,
    requiresHumanReview: readiness.mode?.requiresHumanReview === true,
    requiresDiffReview: readiness.mode?.requiresDiffReview === true,
    requiresTestMapping: readiness.mode?.requiresTestMapping === true,
    promotionRules: values(readiness.promotionRules).map((rule) => ({
      id: rule.id,
      required: rule.required === true,
    })),
  },
  sourceWritingApprovalPackage: {
    status: readiness.sourceWritingApprovalPackage?.status ?? "",
    sourceOfTruth: readiness.sourceWritingApprovalPackage?.sourceOfTruth ?? "",
    defaultRuntimeMutation: readiness.sourceWritingApprovalPackage?.defaultRuntimeMutation ?? "",
    requiredApprovals: values(readiness.sourceWritingApprovalPackage?.requiredApprovals),
    requiredEvidence: values(readiness.sourceWritingApprovalPackage?.requiredEvidence).map((item) => ({
      id: item.id,
      owner: item.owner,
      evidenceKind: item.evidenceKind,
      description: item.description,
    })),
    completedEvidence: values(readiness.sourceWritingApprovalPackage?.completedEvidence),
    prohibitedEvidence: values(readiness.sourceWritingApprovalPackage?.prohibitedEvidence),
    mustNotEnableSourceWriting: readiness.sourceWritingApprovalPackage?.mustNotEnableSourceWriting === true,
    completedEvidenceSchema: {
      requiredFields: values(readiness.sourceWritingApprovalPackage?.completedEvidenceSchema?.requiredFields),
      approvalRules: values(readiness.sourceWritingApprovalPackage?.completedEvidenceSchema?.approvalRules),
      forbiddenFields: values(readiness.sourceWritingApprovalPackage?.completedEvidenceSchema?.forbiddenFields),
      artifactHashPolicy: readiness.sourceWritingApprovalPackage?.completedEvidenceSchema?.artifactHashPolicy ?? {},
      artifactURIPolicy: readiness.sourceWritingApprovalPackage?.completedEvidenceSchema?.artifactURIPolicy ?? {},
    },
  },
  runtimeTargetPolicy: {
    mode: readiness.runtimeTargetPolicy?.mode ?? "",
    newRootPolicy: readiness.runtimeTargetPolicy?.newRootPolicy ?? "",
    roots: values(readiness.runtimeTargetPolicy?.roots).map((root) => ({
      path: root.path,
      status: root.status,
      owner: root.owner,
      promotionGate: root.promotionGate,
      requiresExistingDirectory: root.requiresExistingDirectory === true,
      requiresSeparateSpec: root.requiresSeparateSpec === true,
      purpose: root.purpose,
    })),
  },
  manualReview: {
    required: true,
    requiredBeforeSourceWriting: true,
    decision: "not-approved",
    reasons: [
      "Runtime source writing is disabled.",
      "Generated scaffold files are review artifacts, not runtime source changes.",
      "Each target family must pass its mapped test commands before any future promotion.",
      "Human review and diff review remain required before enabling source-writing generation.",
    ],
    preflightCommands: values(readiness.preflightCommands),
  },
  targetFamilies,
  resources: values(plan.resources).map((resource) => ({
    resource: resource.resource,
    code: resource.code,
    generationLevel: resource.generationLevel,
    dryRunStatus: resource.dryRunStatus,
    manualReviewReason: resource.manualReviewReason,
    candidateFileCount: values(resource.candidateFiles).length,
    targetFamilies: uniqueSorted(
      values(resource.candidateFiles).flatMap((file) =>
        targetFamilies.filter((family) => family.scaffoldRoles.includes(file.role)).map((family) => family.id),
      ),
    ),
  })),
  blockers,
};

const output = `${JSON.stringify(review, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${toPosix(path.relative(repoRoot, generatedPath))}`);
}
