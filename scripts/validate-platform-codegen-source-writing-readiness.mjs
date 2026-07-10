import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-codegen-source-writing-readiness.json"));
const scaffoldPlanPath = path.resolve(repoRoot, argValue("--scaffold-plan", "resources/generated/admin-scaffold-plan.json"));
const promotionReviewPath = path.resolve(repoRoot, argValue("--promotion-review", "resources/generated/admin-scaffold-promotion-review.json"));

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

function safeRelativePrefix(value) {
  if (!value || path.isAbsolute(value)) return false;
  const absolute = path.resolve(repoRoot, value);
  const relative = path.relative(repoRoot, absolute);
  return relative !== "" && !relative.startsWith("..");
}

function scriptExistsForCommand(command) {
  const parts = command.trim().split(/\s+/);
  const script = parts.find((part) => part.startsWith("scripts/"));
  return !script || relativeExistingPath(script);
}

function uniqueErrors(items, label) {
  const errors = [];
  const seen = new Set();
  for (const item of items) {
    if (!item) {
      errors.push(`${label} contains an empty value`);
      continue;
    }
    if (seen.has(item)) {
      errors.push(`${label} contains duplicate value ${item}`);
    }
    seen.add(item);
  }
  return errors;
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function isUnderAnyPrefix(target, prefixes) {
  const targetPath = path.resolve(repoRoot, target);
  return prefixes.some((prefix) => {
    const prefixPath = path.resolve(repoRoot, prefix);
    const relative = path.relative(prefixPath, targetPath);
    return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
  });
}

function rootPolicyForTarget(target, rootPolicies) {
  const targetPath = path.resolve(repoRoot, target);
  return rootPolicies.find((policy) => {
    const rootPath = path.resolve(repoRoot, policy.path);
    const relative = path.relative(rootPath, targetPath);
    return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
  });
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

function validateRuntimeTargetPolicy(readiness, errors) {
  const policy = readiness.runtimeTargetPolicy ?? {};
  if (policy.mode !== "explicit-root-registry") {
    errors.push("runtimeTargetPolicy.mode must be explicit-root-registry");
  }
  if (policy.newRootPolicy !== "requires-separate-architecture-spec") {
    errors.push("runtimeTargetPolicy.newRootPolicy must be requires-separate-architecture-spec");
  }

  const roots = values(policy.roots);
  if (roots.length === 0) {
    errors.push("runtimeTargetPolicy.roots must not be empty");
    return [];
  }
  errors.push(...uniqueErrors(roots.map((root) => root.path), "runtimeTargetPolicy.roots.path"));

  const allowedTargets = values(readiness.allowedRuntimeTargets);
  const rootPaths = new Set(roots.map((root) => root.path));
  for (const target of allowedTargets) {
    if (!rootPaths.has(target)) {
      errors.push(`allowed runtime target ${target} must be declared in runtimeTargetPolicy.roots`);
    }
  }
  for (const root of roots) {
    const prefix = `runtime target policy root ${root.path ?? "<missing>"}`;
    if (!safeRelativePrefix(root.path)) {
      errors.push(`${prefix} path is unsafe`);
      continue;
    }
    if (!allowedTargets.includes(root.path)) {
      errors.push(`${prefix} must also be listed in allowedRuntimeTargets`);
    }
    if (!["existing", "proposed"].includes(root.status)) {
      errors.push(`${prefix} status must be existing or proposed`);
    }
    if (!root.owner) {
      errors.push(`${prefix} must declare owner`);
    }
    if (root.promotionGate !== "source-writing-codegen-promotion") {
      errors.push(`${prefix} promotionGate must stay source-writing-codegen-promotion`);
    }
    if (root.requiresSeparateSpec !== true) {
      errors.push(`${prefix} must require a separate source-writing spec`);
    }
    if (!root.purpose) {
      errors.push(`${prefix} must declare purpose`);
    }
    if (root.status === "existing" || root.requiresExistingDirectory === true) {
      const absolute = path.resolve(repoRoot, root.path);
      if (!fs.existsSync(absolute) || !fs.statSync(absolute).isDirectory()) {
        errors.push(`${prefix} must point to an existing directory`);
      }
    }
    if (root.status === "proposed" && root.requiresExistingDirectory === true) {
      errors.push(`${prefix} proposed roots must not require an existing directory before promotion`);
    }
  }
  return roots;
}

function scaffoldRoles(scaffoldPlan) {
  return new Set(
    values(scaffoldPlan.resources)
      .flatMap((resource) => values(resource.candidateFiles))
      .map((file) => file.role)
      .filter(Boolean),
  );
}

function validateCommands(commands, errors, label) {
  for (const command of values(commands)) {
    if (!command.startsWith("rtk ")) {
      errors.push(`${label} command must start with rtk: ${command}`);
    }
    if (!scriptExistsForCommand(command)) {
      errors.push(`${label} command references a missing script: ${command}`);
    }
  }
}

function validateSourceWritingApprovalPackage(readiness, promotionReview, errors) {
  const pkg = readiness.sourceWritingApprovalPackage ?? {};
  if (pkg.status !== "blocked") {
    errors.push("sourceWritingApprovalPackage.status must stay blocked before promotion");
  }
  if (pkg.sourceOfTruth !== "external-review-artifacts") {
    errors.push("sourceWritingApprovalPackage.sourceOfTruth must stay external-review-artifacts");
  }
  if (pkg.defaultRuntimeMutation !== "forbidden") {
    errors.push("sourceWritingApprovalPackage.defaultRuntimeMutation must stay forbidden");
  }
  if (pkg.mustNotEnableSourceWriting !== true) {
    errors.push("sourceWritingApprovalPackage.mustNotEnableSourceWriting must stay true");
  }
  if (values(pkg.completedEvidence).length !== 0) {
    errors.push("sourceWritingApprovalPackage.completedEvidence must stay empty before promotion");
  }
  requireIncludes(
    pkg.requiredApprovals,
    ["platform-architect", "codegen-owner", "runtime-owner", "operations-owner"],
    "sourceWritingApprovalPackage.requiredApprovals",
    errors,
  );
  requireIncludes(
    pkg.prohibitedEvidence,
    [
      "text-only approval",
      "missing reviewed diff",
      "missing rollback plan",
      "missing target-family test output",
      "runtime mutation enabled in review packet",
    ],
    "sourceWritingApprovalPackage.prohibitedEvidence",
    errors,
  );

  const requiredEvidence = [
    ["explicit-source-writing-spec", "platform-architect", "architecture-spec"],
    ["approved-promotion-review-packet", "codegen-owner", "signed-review-record"],
    ["diff-review", "runtime-owner", "reviewed-diff"],
    ["rollback-plan", "operations-owner", "rollback-runbook"],
    ["target-family-test-run", "codegen-owner", "test-output"],
    ["runtime-target-owner-approval", "runtime-owner", "root-owner-approval"],
  ];
  const items = values(pkg.requiredEvidence);
  for (const [id, owner, evidenceKind] of requiredEvidence) {
    const item = items.find((candidate) => candidate.id === id);
    if (!item) {
      errors.push(`sourceWritingApprovalPackage.requiredEvidence must include ${id}`);
      continue;
    }
    if (item.owner !== owner) {
      errors.push(`sourceWritingApprovalPackage.requiredEvidence.${id}.owner must be ${owner}`);
    }
    if (item.evidenceKind !== evidenceKind) {
      errors.push(`sourceWritingApprovalPackage.requiredEvidence.${id}.evidenceKind must be ${evidenceKind}`);
    }
    if (!item.description) {
      errors.push(`sourceWritingApprovalPackage.requiredEvidence.${id}.description is required`);
    }
  }

  const schema = pkg.completedEvidenceSchema ?? {};
  requireIncludes(
    schema.requiredFields,
    ["id", "owner", "evidenceKind", "artifactURI", "artifactHash", "approvedBy", "approvedAt", "reviewedCommit", "targetFamilies", "runtimeTargets", "verificationCommands", "rollbackCommands"],
    "sourceWritingApprovalPackage.completedEvidenceSchema.requiredFields",
    errors,
  );
  requireIncludes(
    schema.approvalRules,
    [
      "artifact-id-must-match-requiredEvidence",
      "approvedBy-must-not-equal-owner",
      "all-required-owners-represented",
      "all-target-families-covered",
      "runtime-target-owner-approval-required",
      "reviewed-diff-required-per-runtime-target",
      "artifact-hash-must-be-sha256-hex",
      "artifact-uri-must-be-external-review-artifact",
      "verification-commands-must-use-rtk",
      "rollback-commands-required-before-runtime-mutation",
    ],
    "sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules",
    errors,
  );
  requireIncludes(
    schema.forbiddenFields,
    ["secret", "token", "password", "credential", "privateKey"],
    "sourceWritingApprovalPackage.completedEvidenceSchema.forbiddenFields",
    errors,
  );
  validateArtifactHashPolicy(schema, "sourceWritingApprovalPackage.completedEvidenceSchema", errors);
  validateArtifactURIPolicy(schema, "sourceWritingApprovalPackage.completedEvidenceSchema", errors);

  const reviewPackage = promotionReview.sourceWritingApprovalPackage ?? {};
  if (reviewPackage.status !== pkg.status || reviewPackage.sourceOfTruth !== pkg.sourceOfTruth) {
    errors.push("admin scaffold promotion review must carry sourceWritingApprovalPackage from readiness");
  }
  if (values(reviewPackage.requiredEvidence).length !== values(pkg.requiredEvidence).length) {
    errors.push("admin scaffold promotion review sourceWritingApprovalPackage.requiredEvidence must match readiness");
  }
  if (values(reviewPackage.completedEvidence).length !== 0) {
    errors.push("admin scaffold promotion review sourceWritingApprovalPackage.completedEvidence must stay empty before promotion");
  }
  requireIncludes(
    reviewPackage.completedEvidenceSchema?.requiredFields,
    ["id", "owner", "evidenceKind", "artifactURI", "artifactHash", "approvedBy", "approvedAt", "reviewedCommit", "targetFamilies", "runtimeTargets", "verificationCommands", "rollbackCommands"],
    "admin scaffold promotion review sourceWritingApprovalPackage.completedEvidenceSchema.requiredFields",
    errors,
  );
  requireIncludes(
    reviewPackage.completedEvidenceSchema?.approvalRules,
    [
      "artifact-id-must-match-requiredEvidence",
      "approvedBy-must-not-equal-owner",
      "all-required-owners-represented",
      "all-target-families-covered",
      "runtime-target-owner-approval-required",
      "reviewed-diff-required-per-runtime-target",
      "artifact-hash-must-be-sha256-hex",
      "artifact-uri-must-be-external-review-artifact",
      "verification-commands-must-use-rtk",
      "rollback-commands-required-before-runtime-mutation",
    ],
    "admin scaffold promotion review sourceWritingApprovalPackage.completedEvidenceSchema.approvalRules",
    errors,
  );
  requireIncludes(
    reviewPackage.completedEvidenceSchema?.forbiddenFields,
    ["secret", "token", "password", "credential", "privateKey"],
    "admin scaffold promotion review sourceWritingApprovalPackage.completedEvidenceSchema.forbiddenFields",
    errors,
  );
  validateArtifactHashPolicy(
    reviewPackage.completedEvidenceSchema ?? {},
    "admin scaffold promotion review sourceWritingApprovalPackage.completedEvidenceSchema",
    errors,
  );
  validateArtifactURIPolicy(
    reviewPackage.completedEvidenceSchema ?? {},
    "admin scaffold promotion review sourceWritingApprovalPackage.completedEvidenceSchema",
    errors,
  );
}

function assertGeneratedFresh(script, outputPath, errors) {
  const result = spawnSync(process.execPath, [path.join(repoRoot, script), "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    errors.push(`${script} --stdout failed\n${result.stdout}${result.stderr}`);
    return;
  }
  if (!fs.existsSync(outputPath)) {
    errors.push(`${path.relative(repoRoot, outputPath)} is missing; run ${script}`);
    return;
  }
  const actual = fs.readFileSync(outputPath, "utf8");
  if (actual !== result.stdout) {
    errors.push(`${path.relative(repoRoot, outputPath)} is stale; rerun ${script}`);
  }
}

function validateTargetFamilies(readiness, scaffoldPlan, errors) {
  const families = values(readiness.targetFamilies);
  if (families.length === 0) {
    errors.push("targetFamilies must not be empty");
    return;
  }
  errors.push(...uniqueErrors(families.map((family) => family.id), "targetFamilies.id"));

  const knownScaffoldRoles = scaffoldRoles(scaffoldPlan);
  const allowedTargets = values(readiness.allowedRuntimeTargets);
  const blockedTargets = values(readiness.blockedRuntimeTargets);
  const rootPolicies = validateRuntimeTargetPolicy(readiness, errors);
  for (const family of families) {
    const prefix = `target family ${family.id ?? "<missing>"}`;
    if (!family.id) {
      errors.push("target family is missing id");
      continue;
    }
    if (!family.purpose) {
      errors.push(`${prefix} must declare purpose`);
    }
    const familyRoles = values(family.scaffoldRoles);
    if (familyRoles.length === 0) {
      errors.push(`${prefix} must declare scaffoldRoles`);
    }
    for (const role of familyRoles) {
      if (!knownScaffoldRoles.has(role)) {
        errors.push(`${prefix} references unknown scaffold role ${role}`);
      }
    }

    const runtimeTargets = values(family.runtimeTargets);
    if (runtimeTargets.length === 0) {
      errors.push(`${prefix} must declare runtimeTargets`);
    }
    for (const target of runtimeTargets) {
      if (!safeRelativePrefix(target)) {
        errors.push(`${prefix} runtime target is unsafe: ${target}`);
        continue;
      }
      if (!isUnderAnyPrefix(target, allowedTargets)) {
        errors.push(`${prefix} runtime target ${target} is not covered by allowedRuntimeTargets`);
      }
      if (isUnderAnyPrefix(target, blockedTargets)) {
        errors.push(`${prefix} runtime target ${target} is blocked`);
      }
      const rootPolicy = rootPolicyForTarget(target, rootPolicies);
      if (!rootPolicy) {
        errors.push(`${prefix} runtime target ${target} must be declared in runtimeTargetPolicy.roots`);
      } else if (rootPolicy.requiresSeparateSpec !== true) {
        errors.push(`${prefix} runtime target ${target} root must require a separate source-writing spec`);
      }
    }

    const commands = values(family.testCommands);
    if (commands.length === 0) {
      errors.push(`${prefix} must declare testCommands`);
    }
    validateCommands(commands, errors, `${prefix} test`);
  }
}

function validate() {
  const readiness = readJSON(readinessPath);
  const scaffoldPlan = readJSON(scaffoldPlanPath);
  const promotionReview = fs.existsSync(promotionReviewPath) ? readJSON(promotionReviewPath) : {};
  const errors = [];

  if (readiness.mode?.sourceWriting !== "disabled") {
    errors.push("codegen source-writing readiness must keep mode.sourceWriting disabled");
  }
  for (const flag of ["requiresExplicitSpec", "requiresHumanReview", "requiresDiffReview", "requiresTestMapping"]) {
    if (readiness.mode?.[flag] !== true) {
      errors.push(`codegen source-writing readiness must set mode.${flag}=true`);
    }
  }
  validateSourceWritingApprovalPackage(readiness, promotionReview, errors);

  for (const artifact of values(readiness.requiredSourceArtifacts)) {
    if (!relativeExistingPath(artifact)) {
      errors.push(`required source artifact is missing or unsafe: ${artifact}`);
    }
  }

  const allowedTargets = values(readiness.allowedRuntimeTargets);
  if (allowedTargets.length === 0) {
    errors.push("allowedRuntimeTargets must not be empty");
  }
  for (const target of allowedTargets) {
    if (!safeRelativePrefix(target)) {
      errors.push(`allowed runtime target is unsafe: ${target}`);
    }
    if (target.startsWith("resources/generated/") || target === "docs/" || target === "scripts/") {
      errors.push(`allowed runtime target cannot be generated/docs/scripts root: ${target}`);
    }
  }

  const blockedTargets = values(readiness.blockedRuntimeTargets);
  for (const requiredBlocked of [".git/", ".codegraph/", "node_modules/", "resources/generated/", "docs/", "scripts/"]) {
    if (!blockedTargets.includes(requiredBlocked)) {
      errors.push(`blockedRuntimeTargets must include ${requiredBlocked}`);
    }
  }
  for (const target of blockedTargets) {
    if (!safeRelativePrefix(target)) {
      errors.push(`blocked runtime target is unsafe: ${target}`);
    }
  }

  const promotionRuleIDs = new Set(values(readiness.promotionRules).map((rule) => rule.id));
  for (const requiredRule of ["generated-marker-required", "no-handwritten-overwrite", "review-scaffold-first", "test-command-required"]) {
    if (!promotionRuleIDs.has(requiredRule)) {
      errors.push(`promotionRules must include ${requiredRule}`);
    }
  }
  for (const rule of values(readiness.promotionRules)) {
    if (rule.required !== true) {
      errors.push(`promotion rule ${rule.id ?? "<missing>"} must be required`);
    }
    if (!rule.description) {
      errors.push(`promotion rule ${rule.id ?? "<missing>"} must declare description`);
    }
  }

  validateCommands(values(readiness.preflightCommands), errors, "preflight");
  validateTargetFamilies(readiness, scaffoldPlan, errors);

  if (scaffoldPlan.mode?.sourceWriting !== "disabled") {
    errors.push("admin scaffold plan must keep sourceWriting disabled before readiness can pass");
  }
  if (scaffoldPlan.mode?.dryRun !== true) {
    errors.push("admin scaffold plan must stay in dry-run mode before readiness can pass");
  }
  if (scaffoldPlan.summary?.conflictCount !== 0) {
    errors.push(`admin scaffold plan conflictCount must be 0, got ${scaffoldPlan.summary?.conflictCount}`);
  }
  if (scaffoldPlan.summary?.unsafePathCount !== 0) {
    errors.push(`admin scaffold plan unsafePathCount must be 0, got ${scaffoldPlan.summary?.unsafePathCount}`);
  }

  assertGeneratedFresh("scripts/generate-admin-scaffold-promotion-review.mjs", promotionReviewPath, errors);

  if (promotionReview.mode?.sourceWriting !== "disabled") {
    errors.push("admin scaffold promotion review must keep sourceWriting disabled");
  }
  if (promotionReview.mode?.runtimeMutation !== "disabled") {
    errors.push("admin scaffold promotion review must keep runtimeMutation disabled");
  }
  if (promotionReview.mode?.promotion !== "manual-review-required") {
    errors.push("admin scaffold promotion review must require manual review before promotion");
  }
  if (promotionReview.manualReview?.required !== true || promotionReview.manualReview?.decision !== "not-approved") {
    errors.push("admin scaffold promotion review must remain not-approved until human review is complete");
  }
  if ((promotionReview.summary?.targetFamilyCount ?? 0) !== values(readiness.targetFamilies).length) {
    errors.push("admin scaffold promotion review targetFamilyCount must match readiness targetFamilies");
  }
  if ((promotionReview.summary?.blockerCount ?? 0) !== values(promotionReview.blockers).length) {
    errors.push("admin scaffold promotion review blockerCount must match blockers length");
  }
  if ((promotionReview.summary?.conflictCount ?? 0) !== 0) {
    errors.push(`admin scaffold promotion review conflictCount must be 0, got ${promotionReview.summary?.conflictCount}`);
  }
  if ((promotionReview.summary?.unsafePathCount ?? 0) !== 0) {
    errors.push(`admin scaffold promotion review unsafePathCount must be 0, got ${promotionReview.summary?.unsafePathCount}`);
  }
  if (promotionReview.runtimeTargetPolicy?.mode !== readiness.runtimeTargetPolicy?.mode) {
    errors.push("admin scaffold promotion review must include runtimeTargetPolicy");
  }
  if (promotionReview.runtimeTargetPolicy?.newRootPolicy !== readiness.runtimeTargetPolicy?.newRootPolicy) {
    errors.push("admin scaffold promotion review runtimeTargetPolicy.newRootPolicy must match readiness");
  }
  const reviewRoots = new Set(values(promotionReview.runtimeTargetPolicy?.roots).map((root) => root.path));
  for (const root of values(readiness.runtimeTargetPolicy?.roots)) {
    if (!reviewRoots.has(root.path)) {
      errors.push(`admin scaffold promotion review runtimeTargetPolicy must include root ${root.path}`);
    }
  }
  for (const family of values(promotionReview.targetFamilies)) {
    if ((family.reviewItems ?? []).length === 0) {
      errors.push(`admin scaffold promotion review target family ${family.id ?? "<missing>"} must declare reviewItems`);
    }
    if (values(family.testCommands).length === 0) {
      errors.push(`admin scaffold promotion review target family ${family.id ?? "<missing>"} must declare testCommands`);
    }
    if (values(family.runtimeTargetRoots).length === 0) {
      errors.push(`admin scaffold promotion review target family ${family.id ?? "<missing>"} must declare runtimeTargetRoots`);
    }
  }

  return { readiness, errors };
}

const { readiness, errors } = validate();
if (errors.length > 0) {
  console.error("Codegen source-writing readiness validation failed:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exit(1);
}

console.log(`Validated codegen source-writing readiness in ${path.relative(repoRoot, readinessPath)} with ${readiness.preflightCommands.length} preflight commands`);
