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
    "admin-watermark-export-governance",
    [
      "docs/superpowers/plans/2026-07-12-admin-watermark-export-governance.md",
      "scripts/admin-ui-contracts.test.mjs",
      "internal/platform/httpapi/server_test.go",
    ],
  ],
  [
    "production-readiness-preflight",
    [
      "resources/generated/platform-operations-plan.json",
      "scripts/platform-operations-plan.test.mjs",
      "scripts/platform-production-preflight-runner.test.mjs",
    ],
  ],
]);

const requiredVisualEvidenceByTask = new Map([
  [
    "admin-watermark-export-governance",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "browser:control-in-app-browser"],
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
      errors.push(`${task.status} task ${taskID} must not have closeout evidence`);
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
    const requiredTaskVisualEvidence = requiredVisualEvidenceByTask.get(taskID);
    if (requiredTaskVisualEvidence) {
      requireIncludes(closeout.visualEvidence, requiredTaskVisualEvidence, `${prefix}.visualEvidence`, errors);
    }
  }

  for (const task of implementedTasks) {
    if (!closeoutByTaskID.has(task.id)) {
      errors.push(`implemented task ${task.id} must declare node closeout evidence`);
    }
  }

  const unfinishedTaskIDs = unfinishedTasks.map((task) => task.id);
  if (!sameList(values(audit.pendingNodeEvidence), unfinishedTaskIDs)) {
    errors.push("pendingNodeEvidence must exactly match unfinished task graph nodes in graph order");
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
