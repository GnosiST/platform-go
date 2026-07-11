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

const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/platform-node-closeout-audit.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));

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

function requireIncludes(items, expected, label, errors) {
  const actual = new Set(values(items));
  for (const value of expected) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function hasAnyEvidenceKind(closeout) {
  return values(closeout.cleanupEvidence).some((item) => {
    return (
      item.startsWith("docs/") ||
      item.startsWith("scripts/") ||
      item.endsWith("_test.go") ||
      item.endsWith(".test.mjs") ||
      item === "README.md" ||
      item === "AGENTS.md" ||
      item === "admin/package.json" ||
      item.startsWith("resources/")
    );
  });
}

const requiredCleanupEvidenceByTask = new Map([
  ["admin-ui-shell-and-list-components", ["scripts/admin-ui-contracts.test.mjs"]],
  [
    "production-readiness-preflight",
    [
      "resources/generated/platform-operations-plan.json",
      "scripts/platform-operations-plan.test.mjs",
      "scripts/platform-production-preflight-runner.test.mjs",
    ],
  ],
]);

function validate() {
  const audit = readJSON(auditPath);
  const taskGraph = readJSON(taskGraphPath);
  const errors = [];
  const policy = audit.policy ?? {};

  if (!audit.purpose) {
    errors.push("node closeout audit purpose is required");
  }
  if (policy.taskGraph !== "resources/platform-foundation-task-graph.json") {
    errors.push("policy.taskGraph must stay resources/platform-foundation-task-graph.json");
  }
  for (const key of ["implementedTasksRequireCloseout", "neatFreakRequired", "cleanupEvidenceRequired", "unfinishedTasksMustNotHaveCloseout"]) {
    if (policy[key] !== true) {
      errors.push(`policy.${key} must stay true`);
    }
  }

  const requiredDimensions = ["docs", "tests-or-validators", "resource-lock-review", "objective-conflict-review"];
  requireIncludes(policy.requiredDimensions, requiredDimensions, "policy.requiredDimensions", errors);
  requireIncludes(policy.visualEvidenceRequiredForLocks, ["admin-ui", "browser-qa"], "policy.visualEvidenceRequiredForLocks", errors);
  requireIncludes(policy.visualEvidenceRequired, ["superpowers:brainstorming", "product-design"], "policy.visualEvidenceRequired", errors);

  const tasks = values(taskGraph.tasks);
  const taskByID = new Map(tasks.map((task) => [task.id, task]));
  const implementedTasks = tasks.filter((task) => task.status === "implemented");
  const unfinishedTasks = tasks.filter((task) => task.status !== "implemented");
  const closeouts = values(audit.nodeCloseouts);
  const closeoutByTaskID = new Map();

  for (const closeout of closeouts) {
    const taskID = closeout.taskId;
    if (!taskID) {
      errors.push("nodeCloseouts entry is missing taskId");
      continue;
    }
    if (closeoutByTaskID.has(taskID)) {
      errors.push(`nodeCloseouts contains duplicate taskId ${taskID}`);
    }
    closeoutByTaskID.set(taskID, closeout);

    const task = taskByID.get(taskID);
    const prefix = `nodeCloseouts.${taskID}`;
    if (!task) {
      errors.push(`${prefix} references unknown task`);
      continue;
    }
    if (policy.unfinishedTasksMustNotHaveCloseout === true && task.status !== "implemented") {
      errors.push(`${prefix} must reference an implemented task`);
    }
    if (closeout.status !== "closed") {
      errors.push(`${prefix}.status must be closed`);
    }
    if (policy.neatFreakRequired === true && closeout.neatFreak !== true) {
      errors.push(`${prefix}.neatFreak must stay true`);
    }
    if (policy.cleanupEvidenceRequired === true && values(closeout.cleanupEvidence).length === 0) {
      errors.push(`${prefix}.cleanupEvidence must not be empty`);
    }
    if (values(closeout.cleanupEvidence).length > 0 && !hasAnyEvidenceKind(closeout)) {
      errors.push(`${prefix}.cleanupEvidence must cite docs, tests, validators, resources or package evidence`);
    }
    for (const evidencePath of values(closeout.cleanupEvidence)) {
      if (!relativeExistingPath(evidencePath)) {
        errors.push(`${prefix}.cleanupEvidence path is missing or unsafe: ${evidencePath}`);
      }
    }
    requireIncludes(closeout.dimensions, requiredDimensions, `${prefix}.dimensions`, errors);
    const requiredCleanupEvidence = requiredCleanupEvidenceByTask.get(taskID);
    if (requiredCleanupEvidence) {
      requireIncludes(closeout.cleanupEvidence, requiredCleanupEvidence, `${prefix}.cleanupEvidence`, errors);
    }

    const visualLocks = values(policy.visualEvidenceRequiredForLocks);
    const requiresVisualEvidence = values(task.resourceLocks).some((lock) => visualLocks.includes(lock));
    if (requiresVisualEvidence) {
      requireIncludes(closeout.visualEvidence, values(policy.visualEvidenceRequired), `${prefix}.visualEvidence`, errors);
    }
  }

  for (const task of implementedTasks) {
    if (!closeoutByTaskID.has(task.id)) {
      errors.push(`implemented task ${task.id} must declare node closeout evidence`);
    }
  }

  const pendingEvidence = values(audit.pendingNodeEvidence);
  if (JSON.stringify(unfinishedTasks.map((task) => task.id)) !== JSON.stringify(["production-admin-oidc-auth"])) {
    errors.push("node closeout audit must track only production-admin-oidc-auth as unfinished during Task 7 evidence collection");
  }
  if (pendingEvidence.length !== 1 || pendingEvidence[0]?.taskId !== "production-admin-oidc-auth") {
    errors.push("pendingNodeEvidence must describe production-admin-oidc-auth");
  } else {
    const pending = pendingEvidence[0];
    if (pending.status !== "pending" || pending.closeoutAllowed !== false) {
      errors.push("pendingNodeEvidence.production-admin-oidc-auth must stay pending with closeoutAllowed false");
    }
    requireIncludes(
      pending.missingEvidence,
      ["production-like OIDC provider rehearsal", "six-viewport browser acceptance", "neat-freak cleanup evidence"],
      "pendingNodeEvidence.production-admin-oidc-auth.missingEvidence",
      errors,
    );
  }

  return { audit, implementedCount: implementedTasks.length, errors };
}

const { audit, implementedCount, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(
  `Validated platform node closeout audit in ${path.relative(repoRoot, auditPath)} (${implementedCount} implemented task nodes)`,
);
