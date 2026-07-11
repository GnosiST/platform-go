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

const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/platform-task-execution-audit.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const alignmentPath = path.resolve(repoRoot, argValue("--alignment", "resources/platform-foundation-alignment-audit.json"));
const engineeringPath = path.resolve(repoRoot, argValue("--engineering", "resources/platform-engineering-capabilities.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const formSlotsPath = path.resolve(repoRoot, argValue("--form-slots", "resources/platform-form-schema-layout-slots.json"));
const codegenReadinessPath = path.resolve(repoRoot, argValue("--codegen-readiness", "resources/platform-codegen-source-writing-readiness.json"));
const codegenReviewPath = path.resolve(repoRoot, argValue("--codegen-review", "resources/generated/admin-scaffold-promotion-review.json"));
const requiredValidatorPaths = [
  "scripts/validate-platform-task-execution-audit.mjs",
  "scripts/validate-platform-foundation-task-graph.mjs",
  "scripts/validate-platform-foundation-alignment.mjs",
  "scripts/validate-platform-engineering-capabilities.mjs",
  "scripts/validate-platform-admin-api-boundary.mjs",
  "scripts/validate-platform-file-storage-experience.mjs",
  "scripts/validate-platform-refresh-token-family-promotion.mjs",
  "scripts/validate-platform-goal-completion-audit.mjs",
  "scripts/validate-platform-node-closeout-audit.mjs",
  "scripts/validate-platform-objective-conformance.mjs",
  "scripts/validate-platform-promotion-evidence-templates.mjs",
  "scripts/validate-platform-promotion-evidence-package.mjs",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

function hasLocalizedText(value) {
  return typeof value?.zh === "string" && value.zh.trim() !== "" && typeof value?.en === "string" && value.en.trim() !== "";
}

function existingRelativePath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function hasDependencyPath(tasksByID, fromTaskID, toTaskID, visited = new Set()) {
  if (visited.has(fromTaskID)) {
    return false;
  }
  visited.add(fromTaskID);
  const task = tasksByID.get(fromTaskID);
  if (!task) {
    return false;
  }
  for (const dependency of values(task.dependsOn)) {
    if (dependency === toTaskID || hasDependencyPath(tasksByID, dependency, toTaskID, visited)) {
      return true;
    }
  }
  return false;
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function unfinishedBlockersStillActive(taskID, contracts) {
  switch (taskID) {
    case "form-schema-layout-and-slots":
      return contracts.formSlots.promotionState?.runtimeSlots === "deferred";
    default:
      return false;
  }
}

function validateStatusPolicy(audit, errors) {
  const policy = audit.statusPolicy ?? {};
  for (const key of [
    "implementedRequiresEvidence",
    "unfinishedRequiresStatusReason",
    "unfinishedRequiresCompletionGate",
    "futureTaskNodesMustNotDisappear",
    "visualTasksRequireProductDesign",
    "resourceLocksMustDeclarePolicies",
    "parallelBatchesMustAvoidResourceLockGroups",
    "parallelBatchesMustAvoidResourceLockConflicts",
    "parallelBatchesMustAvoidDependencyPaths",
  ]) {
    if (policy[key] !== true) {
      errors.push(`statusPolicy.${key} must stay true`);
    }
  }
}

function validateResourceLockPolicies(graph, errors) {
  const locks = new Set(values(graph.resourceLocks));
  const policies = values(graph.resourceLockPolicies);
  const policyLocks = new Set();
  for (const policy of policies) {
    const lock = policy.lock ?? "";
    if (!lock) {
      errors.push("resource lock policy is missing lock");
      continue;
    }
    if (policyLocks.has(lock)) {
      errors.push(`resource lock policy ${lock} is duplicated`);
    }
    policyLocks.add(lock);
    if (!locks.has(lock)) {
      errors.push(`resource lock policy ${lock} references unknown resource lock`);
    }
    if (!hasLocalizedText(policy.reason)) {
      errors.push(`resource lock policy ${lock} must declare zh/en reason`);
    }
  }
  for (const lock of locks) {
    if (!policyLocks.has(lock)) {
      errors.push(`resourceLockPolicies must describe ${lock}`);
    }
  }
  for (const group of values(graph.resourceLockConflictGroups)) {
    if (!group.id) {
      errors.push("resource lock conflict group is missing id");
      continue;
    }
    if (values(group.locks).length < 2) {
      errors.push(`resource lock conflict group ${group.id} must include at least two locks`);
    }
    for (const lock of values(group.locks)) {
      if (!locks.has(lock)) {
        errors.push(`resource lock conflict group ${group.id} references unknown resource lock ${lock}`);
      }
    }
    if (!hasLocalizedText(group.reason)) {
      errors.push(`resource lock conflict group ${group.id} must declare zh/en reason`);
    }
  }
}

function validateKnownPromotionGate(taskID, blocker, errors) {
  if (!blocker) {
    errors.push(`knownPromotionBlockers must describe ${taskID}`);
    return;
  }
  if (values(blocker.runtimeMutationBlockedWhile).length === 0) {
    errors.push(`knownPromotionBlockers.${taskID}.runtimeMutationBlockedWhile must not be empty`);
  }
  if (values(blocker.requiredEvidenceBeforePromotion).length === 0) {
    errors.push(`knownPromotionBlockers.${taskID}.requiredEvidenceBeforePromotion must not be empty`);
  }
  if (taskID === "production-auth-provider-hardening") {
    requireIncludes(
      blocker.runtimeMutationBlockedWhile,
      [
        "resources/platform-production-auth-hardening.json sessionCredentialPolicy.refreshTokenFamily.defaultRuntime is disabled",
        "resources/platform-production-auth-hardening.json productionPromotionApprovalPackage.status is blocked",
      ],
      `knownPromotionBlockers.${taskID}.runtimeMutationBlockedWhile`,
      errors,
    );
    requireIncludes(
      blocker.requiredEvidenceBeforePromotion,
      [
        "runtime test output bundle including refresh-token-family tests",
        "token-family revocation evidence",
        "redis session invalidation convergence evidence",
        "rotation replay audit evidence",
        "security-owner approval",
        "platform-architect approval",
        "operations-owner approval",
        "structured approval evidence package",
        "provider rotation runbook",
        "rollback plan",
        "audit redaction sample",
        "external absolute artifact URI evidence",
      ],
      `knownPromotionBlockers.${taskID}.requiredEvidenceBeforePromotion`,
      errors,
    );
  }
  if (taskID === "source-writing-codegen-promotion") {
    requireIncludes(
      blocker.runtimeMutationBlockedWhile,
      [
        "resources/platform-codegen-source-writing-readiness.json mode.sourceWriting is disabled",
        "resources/platform-codegen-source-writing-readiness.json sourceWritingApprovalPackage.status is blocked",
        "resources/generated/admin-scaffold-promotion-review.json manualReview.decision is not-approved",
      ],
      `knownPromotionBlockers.${taskID}.runtimeMutationBlockedWhile`,
      errors,
    );
    requireIncludes(
      blocker.requiredEvidenceBeforePromotion,
      [
        "source-writing architecture spec",
        "platform-architect approval",
        "codegen-owner approval",
        "runtime-owner approval",
        "approved promotion review packet",
        "reviewed diff",
        "target-family test mapping",
        "target-family test output",
        "runtime target owner approval",
        "rollback plan",
        "structured source-writing approval evidence package",
        "external absolute artifact URI evidence",
      ],
      `knownPromotionBlockers.${taskID}.requiredEvidenceBeforePromotion`,
      errors,
    );
  }
}

function validateTasks(audit, graph, alignment, contracts, errors) {
  const tasks = values(graph.tasks);
  const tasksByID = new Map(tasks.map((task) => [task.id, task]));
  const unfinishedIDs = values(audit.requiredUnfinishedNodes);
  const foundationPromotionGateTasks = new Set(["production-auth-provider-hardening", "source-writing-codegen-promotion"]);

  const graphUnfinishedIDs = tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
  if (!sameList(unfinishedIDs, graphUnfinishedIDs)) {
    errors.push("requiredUnfinishedNodes must exactly match unfinished task graph nodes in graph order");
    const missingIDs = graphUnfinishedIDs.filter((taskID) => !unfinishedIDs.includes(taskID));
    if (missingIDs.length > 0) {
      errors.push(`requiredUnfinishedNodes must match unfinished task graph nodes: ${missingIDs.join(", ")}`);
    }
  }
  if (!sameList(values(alignment.requiredFutureTaskNodes), graphUnfinishedIDs)) {
    errors.push("alignment.requiredFutureTaskNodes must exactly match unfinished task graph nodes in graph order");
  }

  for (const task of tasks) {
    const evidence = task.evidence ?? {};
    const evidenceCount = ["docs", "validators", "tests", "screenshots"].reduce((count, key) => count + values(evidence[key]).length, 0);
    if (task.status === "implemented" && evidenceCount === 0) {
      errors.push(`task ${task.id} is implemented but has no evidence`);
    }
    if (task.status !== "implemented") {
      if (!hasLocalizedText(task.statusReason)) {
        errors.push(`task ${task.id} is unfinished and must declare zh/en statusReason`);
      }
      if (!hasLocalizedText(task.completionGate)) {
        errors.push(`task ${task.id} is unfinished and must declare zh/en completionGate`);
      }
    }
    if (foundationPromotionGateTasks.has(task.id)) {
      if (!hasLocalizedText(task.statusReason)) {
        errors.push(`task ${task.id} must declare zh/en statusReason`);
      }
      if (!hasLocalizedText(task.completionGate)) {
        errors.push(`task ${task.id} must declare zh/en completionGate`);
      }
    }
    if (task.visual === true) {
      const designGate = values(task.designGate);
      if (!designGate.includes("product-design") || !designGate.includes("superpowers:brainstorming")) {
        errors.push(`visual task ${task.id} must require superpowers:brainstorming and product-design`);
      }
    }
  }

  for (const taskID of foundationPromotionGateTasks) {
    const blocker = values(audit.knownPromotionBlockers).find((item) => item.taskId === taskID);
    validateKnownPromotionGate(taskID, blocker, errors);
  }

  for (const taskID of unfinishedIDs) {
    const task = tasksByID.get(taskID);
    if (!task) {
      errors.push(`required unfinished task is missing from task graph: ${taskID}`);
      continue;
    }
    if (unfinishedBlockersStillActive(taskID, contracts) && task.status === "implemented") {
      errors.push(`task ${taskID} must not be implemented while its execution audit blockers are still active`);
    }
  }
}

function validateParallelBatches(graph, errors) {
  const tasksByID = new Map(values(graph.tasks).map((task) => [task.id, task]));
  const lockPolicies = new Map(values(graph.resourceLockPolicies).map((policy) => [policy.lock, policy]));
  for (const batch of values(graph.parallelBatches)) {
    const lockOwner = new Map();
    const groupOwner = new Map();
    for (const taskID of values(batch.taskIds)) {
      const task = tasksByID.get(taskID);
      if (!task) {
        errors.push(`parallel batch ${batch.id} references unknown task ${taskID}`);
        continue;
      }
      const taskLocks = values(task.resourceLocks);
      for (const lock of taskLocks) {
        const owner = lockOwner.get(lock);
        const lockMode = lockPolicies.get(lock)?.mode ?? "exclusive";
        if (owner && lockMode !== "shared") {
          errors.push(`parallel batch ${batch.id} has resource lock conflict ${lock} between ${owner} and ${taskID}`);
        }
        lockOwner.set(lock, taskID);
      }
      for (const group of values(graph.resourceLockConflictGroups)) {
        if (!values(group.locks).some((lock) => taskLocks.includes(lock))) {
          continue;
        }
        const owner = groupOwner.get(group.id);
        if (owner) {
          errors.push(`parallel batch ${batch.id} has resource lock group conflict ${group.id} between ${owner} and ${taskID}`);
        }
        groupOwner.set(group.id, taskID);
      }
    }
    for (const leftTaskID of values(batch.taskIds)) {
      for (const rightTaskID of values(batch.taskIds)) {
        if (leftTaskID !== rightTaskID && hasDependencyPath(tasksByID, leftTaskID, rightTaskID)) {
          errors.push(`parallel batch ${batch.id} contains dependent tasks ${leftTaskID} and ${rightTaskID}`);
        }
      }
    }
  }
}

function validateEngineering(audit, engineering, errors) {
  const capability = values(engineering.capabilities).find((item) => item.id === audit.engineeringCapability);
  if (!capability) {
    errors.push(`engineering capabilities must include ${audit.engineeringCapability}`);
    return;
  }
  if (capability.status !== "implemented") {
    errors.push(`engineering capability ${audit.engineeringCapability} must be implemented`);
  }
  const validators = values(capability.evidence?.validators);
  for (const validator of values(audit.requiredValidators)) {
    if (validator.includes("task-execution-audit") && !validators.includes(validator)) {
      errors.push(`engineering capability ${audit.engineeringCapability} must cite validator ${validator}`);
    }
  }
}

function validate() {
  const audit = readJSON(auditPath);
  const graph = readJSON(taskGraphPath);
  const alignment = readJSON(alignmentPath);
  const engineering = readJSON(engineeringPath);
  const contracts = {
    productionAuth: readJSON(productionAuthPath),
    formSlots: readJSON(formSlotsPath),
    codegenReadiness: readJSON(codegenReadinessPath),
    codegenReview: readJSON(codegenReviewPath),
  };
  const errors = [];

  if (!audit.purpose) {
    errors.push("task execution audit purpose is required");
  }
  for (const relativePath of [audit.taskGraph, audit.alignmentAudit, ...values(audit.requiredValidators), ...values(audit.requiredTests)]) {
    if (!existingRelativePath(relativePath)) {
      errors.push(`task execution audit path is missing or unsafe: ${relativePath}`);
    }
  }
  requireIncludes(values(audit.requiredValidators), requiredValidatorPaths, "requiredValidators", errors);
  validateStatusPolicy(audit, errors);
  validateResourceLockPolicies(graph, errors);
  validateTasks(audit, graph, alignment, contracts, errors);
  validateParallelBatches(graph, errors);
  validateEngineering(audit, engineering, errors);

  return { audit, errors };
}

const { audit, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated platform task execution audit in ${path.relative(repoRoot, auditPath)} (${values(audit.requiredUnfinishedNodes).length} unfinished nodes tracked)`);
