import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

const approvedBackendStack = ["Gin", "GORM", "Casbin", "JWT"];
const approvedFrontendStack = ["Refine", "React", "Ant Design"];
const allowedStatuses = new Set(["implemented", "pending", "preview", "planned", "deferred"]);
const allowedScopes = new Set(["foundation", "governance", "admin-ui", "business-extension"]);
const allowedLockModes = new Set(["exclusive", "shared"]);
const requiredVisualDesignGate = ["superpowers:brainstorming", "product-design"];
const allowedVisualDesignGates = new Set(requiredVisualDesignGate);
const evidencePathKeys = ["docs", "validators", "tests", "screenshots"];
const requiredAdminUIContractTests = ["scripts/admin-ui-contracts.test.mjs"];
const foundationPromotionGateTaskIDs = new Set(["production-auth-provider-hardening", "source-writing-codegen-promotion"]);

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const graphPath = path.resolve(repoRoot, argValue("--graph", "resources/platform-foundation-task-graph.json"));
const oidcEvidencePath = path.resolve(repoRoot, argValue("--oidc-evidence", "resources/evidence/production-admin-oidc-auth-20260711.json"));

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function hasLocalizedText(value) {
  return typeof value?.zh === "string" && value.zh.trim() !== "" && typeof value?.en === "string" && value.en.trim() !== "";
}

function sameList(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
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

function requireIncludes(items, expected, label, errors) {
  const actual = new Set(values(items));
  for (const item of expected) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function validateResourceLockPolicies(graph, context, errors) {
  const policies = values(graph.resourceLockPolicies);
  const locksWithPolicies = new Set();
  if (policies.length === 0) {
    errors.push("resourceLockPolicies must not be empty");
  }
  for (const policy of policies) {
    const lock = policy.lock ?? "";
    const prefix = `resource lock policy ${lock || "<missing>"}`;
    if (!lock) {
      errors.push("resource lock policy is missing lock");
      continue;
    }
    if (locksWithPolicies.has(lock)) {
      errors.push(`${prefix} is duplicated`);
    }
    locksWithPolicies.add(lock);
    if (!context.resourceLocks.has(lock)) {
      errors.push(`${prefix} references unknown resource lock ${lock}`);
    }
    if (!allowedLockModes.has(policy.mode)) {
      errors.push(`${prefix} has unsupported mode ${policy.mode ?? "<missing>"}`);
    }
    if (!hasLocalizedText(policy.reason)) {
      errors.push(`${prefix} must declare zh/en reason`);
    }
  }
  for (const lock of context.resourceLocks) {
    if (!locksWithPolicies.has(lock)) {
      errors.push(`resourceLockPolicies must describe ${lock}`);
    }
  }
}

function validateResourceLockConflictGroups(graph, context, errors) {
  const groups = values(graph.resourceLockConflictGroups);
  errors.push(...uniqueErrors(groups.map((group) => group.id), "resourceLockConflictGroups.id"));
  for (const group of groups) {
    const prefix = `resource lock conflict group ${group.id ?? "<missing>"}`;
    if (!group.id) {
      errors.push("resource lock conflict group is missing id");
      continue;
    }
    const locks = values(group.locks);
    if (locks.length < 2) {
      errors.push(`${prefix} must include at least two locks`);
    }
    errors.push(...uniqueErrors(locks, `${prefix}.locks`));
    for (const lock of locks) {
      if (!context.resourceLocks.has(lock)) {
        errors.push(`${prefix} references unknown resource lock ${lock}`);
      }
    }
    if (!hasLocalizedText(group.reason)) {
      errors.push(`${prefix} must declare zh/en reason`);
    }
  }
}

function validateApprovedStack(graph, errors) {
  const backend = values(graph.approvedStack?.backend);
  const frontend = values(graph.approvedStack?.frontend);
  if (!sameList(backend, approvedBackendStack)) {
    errors.push("approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(frontend, approvedFrontendStack)) {
    errors.push("approvedStack.frontend must stay Refine + React + Ant Design");
  }
}

function detectCycles(tasksByID) {
  const visiting = new Set();
  const visited = new Set();
  const cycles = [];

  function visit(task, chain) {
    if (visited.has(task.id)) {
      return;
    }
    if (visiting.has(task.id)) {
      const start = chain.indexOf(task.id);
      cycles.push([...chain.slice(start), task.id].join(" -> "));
      return;
    }
    visiting.add(task.id);
    for (const dependency of values(task.dependsOn)) {
      const next = tasksByID.get(dependency);
      if (next) {
        visit(next, [...chain, task.id]);
      }
    }
    visiting.delete(task.id);
    visited.add(task.id);
  }

  for (const task of tasksByID.values()) {
    visit(task, []);
  }
  return cycles;
}

function validateTask(task, context, errors) {
  const prefix = `task ${task.id ?? "<missing>"}`;
  if (!task.id) {
    errors.push("task is missing id");
    return;
  }
  if (!hasLocalizedText(task.title)) {
    errors.push(`${prefix} must declare zh/en title`);
  }
  if (!context.phaseIDs.has(task.phase)) {
    errors.push(`${prefix} has unknown phase ${task.phase}`);
  }
  if (!allowedScopes.has(task.scope)) {
    errors.push(`${prefix} has unsupported scope ${task.scope}`);
  }
  if (!allowedStatuses.has(task.status)) {
    errors.push(`${prefix} has unsupported status ${task.status}`);
  }
  const resourceLocks = values(task.resourceLocks);
  if (resourceLocks.length === 0) {
    errors.push(`${prefix} must declare at least one resource lock`);
  }
  errors.push(...uniqueErrors(resourceLocks, `${prefix}.resourceLocks`));
  for (const lock of resourceLocks) {
    if (!context.resourceLocks.has(lock)) {
      errors.push(`${prefix} uses unknown resource lock ${lock}`);
    }
  }
  const dependencies = values(task.dependsOn);
  for (const dependency of dependencies) {
    if (dependency === task.id) {
      errors.push(`${prefix} cannot depend on itself`);
    } else if (!context.taskIDs.has(dependency)) {
      errors.push(`${prefix} depends on unknown task ${dependency}`);
    } else {
      const dependencyTask = context.tasksByID.get(dependency);
      const taskPhaseOrder = context.phaseOrders.get(task.phase);
      const dependencyPhaseOrder = context.phaseOrders.get(dependencyTask?.phase);
      if (taskPhaseOrder != null && dependencyPhaseOrder != null && dependencyPhaseOrder > taskPhaseOrder && !hasLaterPhaseDependencyException(task, dependency)) {
        errors.push(`${prefix} in phase ${task.phase} cannot depend on later-phase task ${dependency} in phase ${dependencyTask.phase}`);
      }
    }
  }
  for (const exception of values(task.phaseDependencyExceptions)) {
    const exceptionDependency = exception.dependency;
    if (!dependencies.includes(exceptionDependency)) {
      errors.push(`${prefix} phaseDependencyExceptions references non-dependency ${exception.dependency ?? "<missing>"}`);
    } else {
      const dependencyTask = context.tasksByID.get(exceptionDependency);
      const taskPhaseOrder = context.phaseOrders.get(task.phase);
      const dependencyPhaseOrder = context.phaseOrders.get(dependencyTask?.phase);
      if (taskPhaseOrder != null && dependencyPhaseOrder != null && dependencyPhaseOrder <= taskPhaseOrder) {
        errors.push(`${prefix} phaseDependencyExceptions for ${exceptionDependency} must reference a later-phase dependency`);
      }
    }
    if (!hasLocalizedText(exception.reason)) {
      errors.push(`${prefix} phaseDependencyExceptions for ${exception.dependency ?? "<missing>"} must declare zh/en reason`);
    }
  }
  if (task.visual === true) {
    const designGate = values(task.designGate);
    for (const gate of designGate) {
      if (!allowedVisualDesignGates.has(gate)) {
        errors.push(`visual task ${task.id} has unsupported design gate ${gate}`);
      }
    }
    for (const requiredGate of requiredVisualDesignGate) {
      if (!designGate.includes(requiredGate)) {
        errors.push(`visual task ${task.id} must require ${requiredGate}`);
      }
    }
    const isContractGateOnly = task.contractGateOnly === true;
    if (!isContractGateOnly && (task.status === "implemented" || task.status === "preview") && values(task.evidence?.screenshots).length === 0) {
      errors.push(`visual task ${task.id} with status ${task.status} must declare screenshot evidence`);
    }
  }
  if (task.id === "admin-ui-shell-and-list-components") {
    requireIncludes(task.evidence?.tests, requiredAdminUIContractTests, `${task.id} evidence.tests`, errors);
  }
  if (task.status === "implemented" || task.status === "preview") {
    const evidence = task.evidence ?? {};
    const evidencePaths = evidencePathKeys.flatMap((key) => values(evidence[key]));
    if (evidencePaths.length === 0) {
      errors.push(`${prefix} must declare evidence paths`);
    }
    for (const relativePath of evidencePaths) {
      if (!relativeExistingPath(relativePath)) {
        errors.push(`${prefix} evidence path is missing or unsafe: ${relativePath}`);
      }
    }
  }
  if (task.status === "pending" || task.status === "preview" || task.status === "planned" || task.status === "deferred") {
    if (!hasLocalizedText(task.statusReason)) {
      errors.push(`${prefix} with status ${task.status} must declare zh/en statusReason`);
    }
    if (!hasLocalizedText(task.completionGate)) {
      errors.push(`${prefix} with status ${task.status} must declare zh/en completionGate`);
    }
    const docs = values(task.evidence?.docs);
    if (docs.length === 0) {
      errors.push(`${prefix} with status ${task.status} must declare at least one evidence.docs path`);
    }
    for (const relativePath of docs) {
      if (!relativeExistingPath(relativePath)) {
        errors.push(`${prefix} evidence doc is missing or unsafe: ${relativePath}`);
      }
    }
  }
  if (foundationPromotionGateTaskIDs.has(task.id)) {
    if (!hasLocalizedText(task.statusReason)) {
      errors.push(`${prefix} must declare zh/en statusReason`);
    }
    if (!hasLocalizedText(task.completionGate)) {
      errors.push(`${prefix} must declare zh/en completionGate`);
    }
    const docs = values(task.evidence?.docs);
    if (docs.length === 0) {
      errors.push(`${prefix} must declare at least one evidence.docs path`);
    }
    for (const relativePath of docs) {
      if (!relativeExistingPath(relativePath)) {
        errors.push(`${prefix} evidence doc is missing or unsafe: ${relativePath}`);
      }
    }
  }
}

function validateProductionAdminOIDCNode(tasks, evidence, errors) {
  const task = tasks.find((item) => item.id === "production-admin-oidc-auth");
  if (!task) {
    errors.push("task graph must include production-admin-oidc-auth");
    return;
  }
  if (task.status !== "implemented") {
    errors.push("production-admin-oidc-auth must be implemented after Task 8 evidence closeout");
  }
  const implemented = tasks.filter((item) => item.status === "implemented");
  const pending = tasks.filter((item) => item.status === "pending");
  const blocked = tasks.filter((item) => item.status === "blocked");
  if (tasks.length !== 37 || implemented.length !== 37 || pending.length !== 0 || blocked.length !== 0) {
    errors.push("Task 8 task graph counts must stay 37 total, 37 implemented, 0 pending and 0 blocked");
  }
  if (!sameList(values(task.dependsOn), ["production-auth-provider-hardening", "production-persistence-correctness", "admin-ui-system-quality-hardening"])) {
    errors.push("production-admin-oidc-auth dependencies must stay production auth, persistence correctness and Admin UI hardening");
  }
  const requirements = values(task.completionEvidence);
  const requiredIDs = ["production-like-oidc-rehearsal", "six-viewport-browser-acceptance", "neat-freak-cleanup-closeout"];
  if (!sameList(requirements.map((item) => item.id), requiredIDs)) {
    errors.push("production-admin-oidc-auth completionEvidence must name production-like, six-viewport browser and neat-freak cleanup evidence");
  }
  if (requirements.some((item) => item.status !== "verified" || item.requiredIn !== "Task 8")) {
    errors.push("production-admin-oidc-auth completion evidence must be verified by Task 8");
  }
  const browser = requirements.find((item) => item.id === "six-viewport-browser-acceptance");
  if (!sameList(values(browser?.viewports), ["375x812", "390x844", "768x1024", "1024x768", "1280x720", "1440x1024"])) {
    errors.push("production-admin-oidc-auth browser evidence must require all six approved viewports");
  }
  if (!sameList(values(task.evidence?.screenshots), ["resources/evidence/production-admin-oidc-auth-20260711.json"])) {
    errors.push("production-admin-oidc-auth screenshot evidence must use the tracked Task 8 evidence manifest");
  }
  if (evidence.redaction?.scanPassed !== true || evidence.redaction?.screenshotsInspected !== true) {
    errors.push("production-admin-oidc-auth evidence redaction scan must pass");
  }
  if (!sameList(values(evidence.browser?.viewports), ["375x812", "390x844", "768x1024", "1024x768", "1280x720", "1440x1024"])) {
    errors.push("production-admin-oidc-auth evidence manifest must cover all six viewports");
  }
  requireIncludes(
    evidence.browser?.scenarios,
    ["login", "success", "protected-navigation-refresh", "cancellation-recovery", "invalid-state-recovery", "expired-transaction-recovery", "missing-binding-recovery", "disabled-user-recovery", "keyboard-focus"],
    "production-admin-oidc-auth evidence scenarios",
    errors,
  );
  const screenshots = values(evidence.screenshots);
  if (screenshots.length < 15) {
    errors.push("production-admin-oidc-auth evidence manifest must integrity-address at least 15 screenshots");
  }
  for (const screenshot of screenshots) {
    if (screenshot.redacted !== true || !/^sha256:[0-9a-f]{64}$/.test(screenshot.sha256 ?? "")) {
      errors.push(`production-admin-oidc-auth screenshot ${screenshot.scenario ?? "<missing>"} must be redacted and carry a sha256 hash`);
    }
    if (!String(screenshot.localPath ?? "").startsWith("tmp/product-design/production-admin-oidc-auth-20260711/screenshots/")) {
      errors.push(`production-admin-oidc-auth screenshot ${screenshot.scenario ?? "<missing>"} has an invalid local evidence path`);
    }
  }
  if (evidence.runtime?.expiredTransactionRejected !== true || evidence.runtime?.stdinOnlyBinding !== true) {
    errors.push("production-admin-oidc-auth evidence manifest must verify expiry and stdin-only binding");
  }
  if (
    evidence.promotionBoundary?.foundationNodeClosed !== true ||
    evidence.promotionBoundary?.productionPromotionApproved !== false ||
    evidence.promotionBoundary?.runtimeMutation !== "disabled" ||
    evidence.promotionBoundary?.refreshTokenFamilyDefaultRuntime !== "disabled" ||
    evidence.promotionBoundary?.sourceWriting !== "disabled"
  ) {
    errors.push("production-admin-oidc-auth evidence manifest must preserve promotion and runtime boundaries");
  }
}

function hasLaterPhaseDependencyException(task, dependency) {
  return values(task.phaseDependencyExceptions).some((exception) => exception.dependency === dependency && hasLocalizedText(exception.reason));
}

function validateParallelBatches(graph, tasksByID, context, errors) {
  errors.push(...uniqueErrors(values(graph.parallelBatches).map((batch) => batch.id), "parallelBatches.id"));
  for (const batch of values(graph.parallelBatches)) {
    const taskIDs = values(batch.taskIds);
    errors.push(...uniqueErrors(taskIDs, `parallel batch ${batch.id}.taskIds`));
    const lockOwner = new Map();
    const groupOwner = new Map();
    for (const taskID of taskIDs) {
      const task = tasksByID.get(taskID);
      if (!task) {
        errors.push(`parallel batch ${batch.id} references unknown task ${taskID}`);
        continue;
      }
      const taskLocks = values(task.resourceLocks);
      for (const lock of taskLocks) {
        const owner = lockOwner.get(lock);
        const lockMode = context.lockPolicies.get(lock)?.mode ?? "exclusive";
        if (owner && lockMode !== "shared") {
          errors.push(`parallel batch ${batch.id} has resource lock conflict ${lock} between ${owner} and ${taskID}`);
        } else {
          lockOwner.set(lock, taskID);
        }
      }
      for (const group of values(context.conflictGroups)) {
        if (!values(group.locks).some((lock) => taskLocks.includes(lock))) {
          continue;
        }
        const owner = groupOwner.get(group.id);
        if (owner) {
          errors.push(`parallel batch ${batch.id} has resource lock group conflict ${group.id} between ${owner} and ${taskID}`);
        } else {
          groupOwner.set(group.id, taskID);
        }
      }
    }
    for (const leftTaskID of taskIDs) {
      for (const rightTaskID of taskIDs) {
        if (leftTaskID === rightTaskID) {
          continue;
        }
        if (hasDependencyPath(tasksByID, leftTaskID, rightTaskID)) {
          errors.push(`parallel batch ${batch.id} contains dependent tasks ${leftTaskID} and ${rightTaskID}`);
        }
      }
    }
  }
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
    if (dependency === toTaskID) {
      return true;
    }
    if (hasDependencyPath(tasksByID, dependency, toTaskID, visited)) {
      return true;
    }
  }
  return false;
}

function validate() {
  const graph = readJSON(graphPath);
  let oidcEvidence = {};
  const errors = [];
  try {
    oidcEvidence = readJSON(oidcEvidencePath);
  } catch (error) {
    errors.push(`production-admin-oidc-auth evidence manifest could not be read: ${error.message}`);
  }
  validateApprovedStack(graph, errors);

  const phases = values(graph.phases);
  const tasks = values(graph.tasks);
  const phaseIDs = new Set(phases.map((phase) => phase.id));
  const phaseOrders = new Map(phases.map((phase) => [phase.id, phase.order]));
  const resourceLocks = new Set(values(graph.resourceLocks));
  const lockPolicies = new Map(values(graph.resourceLockPolicies).map((policy) => [policy.lock, policy]));
  const conflictGroups = values(graph.resourceLockConflictGroups);
  const taskIDs = new Set(tasks.map((task) => task.id));
  const tasksByID = new Map(tasks.map((task) => [task.id, task]));

  errors.push(...uniqueErrors(phases.map((phase) => phase.id), "phases.id"));
  errors.push(...uniqueErrors(values(graph.resourceLocks), "resourceLocks"));
  errors.push(...uniqueErrors(tasks.map((task) => task.id), "tasks.id"));

  for (const phase of phases) {
    if (!hasLocalizedText(phase.label)) {
      errors.push(`phase ${phase.id ?? "<missing>"} must declare zh/en label`);
    }
    if (!Number.isInteger(phase.order)) {
      errors.push(`phase ${phase.id ?? "<missing>"} must declare integer order`);
    }
  }

  const context = { phaseIDs, phaseOrders, resourceLocks, lockPolicies, conflictGroups, taskIDs, tasksByID };
  validateResourceLockPolicies(graph, context, errors);
  validateResourceLockConflictGroups(graph, context, errors);
  for (const task of tasks) {
    validateTask(task, context, errors);
  }
  validateProductionAdminOIDCNode(tasks, oidcEvidence, errors);

  for (const cycle of detectCycles(tasksByID)) {
    errors.push(`dependency cycle detected: ${cycle}`);
  }
  validateParallelBatches(graph, tasksByID, context, errors);
  return { graph, errors };
}

const { graph, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${graph.tasks?.length ?? 0} platform foundation task nodes in ${path.relative(repoRoot, graphPath)}`);
