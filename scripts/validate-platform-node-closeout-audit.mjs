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
  [
    "data-lifecycle-retention",
    [
      "docs/platform-data-lifecycle-retention.md",
      "internal/platform/datalifecycle/runner_test.go",
      "scripts/validate-platform-data-lifecycle-retention.mjs",
      "scripts/platform-data-lifecycle-retention.test.mjs",
    ],
  ],
  [
    "platform-service-contract-standard",
    [
      "docs/platform-service-contract-standard.md",
      "internal/platform/capability/service_contract_test.go",
      "resources/generated/platform-service-contract.json",
      "resources/generated/asyncapi.events.json",
      "resources/generated/service-sdk/go/service_contract_sdk.go",
      "resources/generated/service-sdk/typescript/serviceContractSDK.ts",
      "scripts/validate-platform-service-contract-standard.mjs",
      "scripts/platform-service-contract-standard.test.mjs",
    ],
  ],
  [
    "persisted-query-command-object-runtime",
    [
      "docs/platform-service-objects.md",
      "internal/platform/serviceobject/runtime_test.go",
      "internal/platform/serviceobject/idempotency_gorm_test.go",
      "internal/platform/httpapi/service_objects_test.go",
      "resources/platform-service-object-runtime.json",
      "scripts/validate-platform-service-object-runtime.mjs",
      "scripts/platform-service-object-runtime.test.mjs",
    ],
  ],
  [
    "integration-ports-disabled-default",
    [
      "docs/platform-integration-ports.md",
      "internal/platform/integration/integration_test.go",
      "resources/platform-integration-ports.json",
      "scripts/validate-platform-integration-ports.mjs",
      "scripts/platform-integration-ports.test.mjs",
    ],
  ],
  [
    "organization-rbac-menu-contract-and-migration-design",
    [
      "resources/platform-organization-rbac-menu-contract.json",
      "docs/platform-organization-rbac-menu-contract.md",
      "docs/superpowers/specs/2026-07-15-organization-rbac-menu-contract-and-migration-design.md",
      "scripts/validate-platform-organization-rbac-menu-contract.mjs",
      "scripts/platform-organization-rbac-menu-contract.test.mjs",
    ],
  ],
  [
    "organization-role-pool-backend-and-migration",
    [
      "docs/platform-organization-rbac-menu-contract.md",
      "internal/platform/organizationrbac/validation_test.go",
      "internal/platform/organizationrbac/gorm_repository_test.go",
      "internal/platform/organizationrbac/migration_test.go",
      "internal/platform/bootstrap/organization_rbac_test.go",
      "scripts/validate-admin-service-object-definitions.mjs",
      "scripts/validate-platform-organization-rbac-menu-contract.mjs",
      "scripts/platform-organization-rbac-menu-contract.test.mjs",
    ],
  ],
  [
    "organization-user-admin-experience",
    [
      "docs/superpowers/specs/2026-07-15-organization-user-admin-experience-design.md",
      "docs/superpowers/plans/2026-07-15-organization-user-admin-experience.md",
      "resources/evidence/organization-user-admin-experience-20260715.json",
      "scripts/validate-admin-ui-contracts.mjs",
      "scripts/admin-ui-contracts.test.mjs",
      "scripts/validate-platform-organization-rbac-menu-contract.mjs",
      "scripts/platform-organization-rbac-menu-contract.test.mjs",
    ],
  ],
  [
    "role-tree-and-authorization-entry",
    [
      "docs/superpowers/specs/2026-07-15-role-tree-and-authorization-entry-design.md",
      "docs/superpowers/plans/2026-07-15-role-tree-and-authorization-entry.md",
      "resources/evidence/role-tree-and-authorization-entry-20260715.json",
      "scripts/validate-admin-ui-contracts.mjs",
      "scripts/admin-ui-contracts.test.mjs",
      "scripts/validate-platform-organization-rbac-menu-contract.mjs",
      "scripts/platform-organization-rbac-menu-contract.test.mjs",
    ],
  ],
  [
    "menu-tree-and-button-permission-configuration",
    [
      "docs/platform-organization-rbac-menu-contract.md",
      "docs/admin-rbac-menu.md",
      "docs/platform-ui-optimization-assessment.md",
      "resources/evidence/menu-tree-and-button-permission-configuration-20260715.json",
      "internal/platform/organizationrbac/menu_repository_test.go",
      "internal/platform/organizationrbac/navigation_service_objects_test.go",
      "scripts/validate-platform-organization-rbac-menu-contract.mjs",
      "scripts/platform-organization-rbac-menu-contract.test.mjs",
      "scripts/validate-admin-i18n.mjs",
      "scripts/validate-admin-ui-contracts.mjs",
      "scripts/admin-ui-contracts.test.mjs",
    ],
  ],
]);

const requiredVisualEvidenceByTask = new Map([
  [
    "menu-tree-and-button-permission-configuration",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "browser:control-in-app-browser"],
  ],
  [
    "admin-watermark-export-governance",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "browser:control-in-app-browser"],
  ],
  [
    "sensitive-data-reveal-step-up",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "browser:control-in-app-browser"],
  ],
  [
    "organization-rbac-menu-contract-and-migration-design",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max"],
  ],
  [
    "organization-user-admin-experience",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "playwright-1.55-local-fallback"],
  ],
  [
    "role-tree-and-authorization-entry",
    ["superpowers:brainstorming", "product-design", "ui-ux-pro-max", "playwright-1.55-local-fallback"],
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
  for (const key of ["implementedTasksRequireCloseout", "cleanupEvidenceRequired", "unfinishedTasksMustNotHaveCloseout"]) {
    if (policy[key] !== true) {
      errors.push(`policy.${key} must stay true`);
    }
  }
  if (policy.neatFreakRequired !== false) {
    errors.push("policy.neatFreakRequired must stay false");
  }
  const neatFreakInvocationPolicy = policy.neatFreakInvocationPolicy ?? {};
  requireIncludes(
    neatFreakInvocationPolicy.requiredFor,
    ["phase-closeout", "major-cross-module-task", "release-preparation"],
    "policy.neatFreakInvocationPolicy.requiredFor",
    errors,
  );
  requireIncludes(
    neatFreakInvocationPolicy.notRequiredFor,
    ["small-node", "routine-sub-agent-task"],
    "policy.neatFreakInvocationPolicy.notRequiredFor",
    errors,
  );

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
    if (typeof closeout.neatFreak !== "boolean") {
      errors.push(`${prefix}.neatFreak must be a boolean`);
    }
    if (closeout.neatFreak === false && closeout.cleanupMode !== "focused") {
      errors.push(`${prefix}.cleanupMode must be focused when neat-freak was not invoked`);
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
