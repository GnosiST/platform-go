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
const requiredWatermarkEvidenceManifest = "resources/evidence/admin-watermark-export-governance-20260713.json";
const requiredSensitiveRevealEvidenceManifest = "resources/evidence/sensitive-data-reveal-step-up-20260713.json";
const requiredOrganizationUserEvidenceManifest = "resources/evidence/organization-user-admin-experience-20260715.json";
const requiredRoleTreeEvidenceManifest = "resources/evidence/role-tree-and-authorization-entry-20260715.json";
const requiredMenuEvidenceManifest = "resources/evidence/menu-tree-and-button-permission-configuration-20260715.json";
const releaseBlockingNodes = [
  "organization-rbac-menu-e2e-qa",
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];
const postReleaseOptionalNodes = [
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
];
const foundationPromotionGateTaskIDs = new Set(["production-auth-provider-hardening", "source-writing-codegen-promotion"]);
const foundationBaselineTaskIDs = [
  "stack-alignment-and-architecture",
  "capability-manifest-contract",
  "resource-schema-contract",
  "capability-profile-composition-gate",
  "capability-contract-governance",
  "rbac-menu-data-scope",
  "governance-org-area-role-groups",
  "auth-session-provider-jwt-wechat",
  "gorm-storage-runtime",
  "cache-redis-invalidation",
  "production-persistence-correctness",
  "production-runtime-gate",
  "production-readiness-preflight",
  "openapi-app-contracts",
  "admin-api-boundary-query-security",
  "codegen-preview-scaffold",
  "codegen-source-writing-readiness",
  "admin-ui-shell-and-list-components",
  "branding-demo-data-dashboard",
  "personnel-extension-boundary",
  "notification-extension-boundary",
  "job-extension-boundary",
  "visual-product-design-qa",
  "policy-review-and-audit-workflow",
  "production-auth-provider-hardening",
  "form-schema-layout-and-slots",
  "refine-custom-panels-and-actions",
  "file-storage-preview-and-audit-workflow",
  "policy-review-custom-ui",
  "source-writing-codegen-promotion",
  "task-dependency-governance",
  "reference-discovery-classification-gate",
  "reference-coverage-boundary-gate",
  "node-closeout-audit",
  "foundation-alignment-audit",
  "admin-ui-system-quality-hardening",
  "production-admin-oidc-auth",
];
const approvedCompletionProgramTaskIDs = [
  "runtime-security-containment",
  "admin-watermark-export-governance",
  "sensitive-data-protection-runtime",
  "sensitive-data-historical-migration",
  "mask-strategy-runtime",
  "sensitive-data-reveal-step-up",
  "data-lifecycle-retention",
  "platform-service-contract-standard",
  "persisted-query-command-object-runtime",
  "integration-ports-disabled-default",
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
  "unified-error-code-governance",
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];
const requiredRemainingTaskDependencies = new Map([
  ["platform-service-contract-standard", ["data-lifecycle-retention", "capability-contract-governance"]],
  ["persisted-query-command-object-runtime", ["platform-service-contract-standard", "admin-api-boundary-query-security"]],
  ["integration-ports-disabled-default", ["platform-service-contract-standard", "notification-extension-boundary", "job-extension-boundary"]],
  ["organization-rbac-menu-contract-and-migration-design", ["persisted-query-command-object-runtime", "governance-org-area-role-groups", "rbac-menu-data-scope"]],
  ["organization-role-pool-backend-and-migration", ["organization-rbac-menu-contract-and-migration-design", "production-persistence-correctness"]],
  ["organization-user-admin-experience", ["organization-role-pool-backend-and-migration", "admin-ui-system-quality-hardening"]],
  ["role-tree-and-authorization-entry", ["organization-user-admin-experience"]],
  ["menu-tree-and-button-permission-configuration", ["role-tree-and-authorization-entry"]],
  ["organization-rbac-menu-e2e-qa", ["organization-user-admin-experience", "role-tree-and-authorization-entry", "menu-tree-and-button-permission-configuration"]],
  ["unified-error-code-governance", ["platform-service-contract-standard", "persisted-query-command-object-runtime", "admin-api-boundary-query-security", "openapi-app-contracts", "runtime-security-containment"]],
  ["multi-datasource-contract-and-runtime", ["platform-service-contract-standard", "data-lifecycle-retention", "production-persistence-correctness"]],
  ["tenant-placement-and-request-routing", ["multi-datasource-contract-and-runtime", "organization-role-pool-backend-and-migration"]],
  ["datasource-read-write-routing", ["tenant-placement-and-request-routing"]],
  ["sharding-and-tenant-migration", ["datasource-read-write-routing"]],
  ["federated-read-query", ["sharding-and-tenant-migration", "persisted-query-command-object-runtime"]],
  ["xa-optional-adapter", ["federated-read-query"]],
  ["database-certification-matrix", ["xa-optional-adapter"]],
  ["transactional-outbox-and-one-mq-adapter", ["integration-ports-disabled-default", "database-certification-matrix"]],
  ["asynchronous-search-projection", ["transactional-outbox-and-one-mq-adapter", "persisted-query-command-object-runtime"]],
  ["open-source-portability", ["admin-watermark-export-governance", "organization-rbac-menu-e2e-qa", "unified-error-code-governance"]],
  ["public-docs-community", ["open-source-portability"]],
  ["public-docs-site", ["public-docs-community"]],
  ["github-release-publication", ["public-docs-site", "open-source-portability"]],
]);
const requiredRemainingResourceLocks = [
  "service-contract",
  "query-command-contract",
  "tenant-context",
  "organization-rbac-contract",
  "organization-rbac-migration",
  "menu-permission-contract",
  "datasource-registry",
  "routing-runtime",
  "sharding-runtime",
  "federated-query",
  "xa-runtime",
  "event-contract",
  "transaction-outbox",
  "database-certification",
  "data-plane-docs",
  "identity-governance-docs",
  "integration-docs",
];

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
  if (task.id === "admin-watermark-export-governance" && task.status === "implemented") {
    requireIncludes(task.evidence?.skills, ["ui-ux-pro-max"], `${task.id} evidence.skills`, errors);
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/httpapi/server_test.go", "scripts/admin-ui-contracts.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-admin-i18n.mjs", "scripts/validate-admin-ui-contracts.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(task.evidence?.screenshots, [requiredWatermarkEvidenceManifest], `${task.id} evidence.screenshots`, errors);
  }
  if (task.id === "sensitive-data-protection-runtime") {
    if (task.status !== "implemented") {
      errors.push("sensitive-data-protection-runtime must stay implemented after closeout");
    }
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/adminresource/protection_test.go", "internal/platform/dataprotection/runtime_test.go"],
      `${task.id} evidence.tests`,
      errors,
    );
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-platform-production-env.mjs", "scripts/validate-platform-node-closeout-audit.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
  }
  if (task.id === "sensitive-data-historical-migration") {
    if (task.status !== "implemented") {
      errors.push("sensitive-data-historical-migration must stay implemented after closeout");
    }
    requireIncludes(
      task.evidence?.docs,
      ["docs/platform-sensitive-data-migration.md", "docs/superpowers/specs/2026-07-12-sensitive-data-historical-migration-design.md"],
      `${task.id} evidence.docs`,
      errors,
    );
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/sensitivemigration/runner_test.go", "scripts/platform-sensitive-data-migration.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-platform-sensitive-data-migration.mjs", "scripts/validate-platform-node-closeout-audit.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
  }
  if (task.id === "mask-strategy-runtime") {
    if (task.status !== "implemented") {
      errors.push("approved implemented task mask-strategy-runtime must stay implemented");
    }
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/masking/runtime_test.go", "internal/platform/httpapi/projection_test.go", "scripts/admin-ui-contracts.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-admin-resources.mjs", "scripts/validate-admin-ui-contracts.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
  }
  if (task.id === "sensitive-data-reveal-step-up" && task.status === "implemented") {
    requireIncludes(task.evidence?.screenshots, [requiredSensitiveRevealEvidenceManifest], `${task.id} evidence.screenshots`, errors);
  }
  if (task.id === "data-lifecycle-retention") {
    if (task.status !== "implemented") {
      errors.push("data-lifecycle-retention must stay implemented after closeout");
    }
    requireIncludes(task.evidence?.docs, ["docs/platform-data-lifecycle-retention.md"], `${task.id} evidence.docs`, errors);
    requireIncludes(task.evidence?.validators, ["scripts/validate-platform-data-lifecycle-retention.mjs"], `${task.id} evidence.validators`, errors);
    requireIncludes(task.evidence?.tests, ["internal/platform/datalifecycle/runner_test.go", "scripts/platform-data-lifecycle-retention.test.mjs"], `${task.id} evidence.tests`, errors);
  }
  if (task.id === "platform-service-contract-standard") {
    if (task.status !== "implemented") {
      errors.push("platform-service-contract-standard must stay implemented after closeout");
    }
    requireIncludes(
      task.evidence?.docs,
      ["docs/platform-service-contract-standard.md"],
      `${task.id} evidence.docs`,
      errors,
    );
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-platform-service-contract-standard.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/capability/service_contract_test.go", "scripts/platform-service-contract-standard.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
  }
  if (task.id === "persisted-query-command-object-runtime") {
    if (task.status !== "implemented") {
      errors.push("persisted-query-command-object-runtime must stay implemented after closeout");
    }
    requireIncludes(task.evidence?.docs, ["docs/platform-service-objects.md"], `${task.id} evidence.docs`, errors);
    requireIncludes(
      task.evidence?.validators,
      ["scripts/validate-platform-service-object-runtime.mjs", "scripts/validate-platform-admin-api-boundary.mjs"],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/serviceobject/runtime_test.go", "internal/platform/httpapi/service_objects_test.go", "scripts/platform-service-object-runtime.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
  }
  if (task.id === "integration-ports-disabled-default") {
    if (task.status !== "implemented") {
      errors.push("integration-ports-disabled-default must stay implemented after closeout");
    }
    requireIncludes(task.evidence?.docs, ["docs/platform-integration-ports.md"], `${task.id} evidence.docs`, errors);
    requireIncludes(task.evidence?.validators, ["scripts/validate-platform-integration-ports.mjs"], `${task.id} evidence.validators`, errors);
    requireIncludes(
      task.evidence?.tests,
      ["internal/platform/integration/integration_test.go", "scripts/platform-integration-ports.test.mjs"],
      `${task.id} evidence.tests`,
      errors,
    );
  }
  if (task.id === "organization-rbac-menu-contract-and-migration-design") {
    if (task.status !== "implemented") {
      errors.push("organization-rbac-menu-contract-and-migration-design must stay implemented after contract closeout");
    }
    if (task.contractGateOnly !== true) {
      errors.push("organization-rbac-menu-contract-and-migration-design must stay contractGateOnly");
    }
    requireIncludes(task.evidence?.docs, ["docs/platform-organization-rbac-menu-contract.md"], `${task.id} evidence.docs`, errors);
    requireIncludes(task.evidence?.validators, ["scripts/validate-platform-organization-rbac-menu-contract.mjs"], `${task.id} evidence.validators`, errors);
    requireIncludes(task.evidence?.tests, ["scripts/platform-organization-rbac-menu-contract.test.mjs"], `${task.id} evidence.tests`, errors);
  }
  if (task.id === "organization-role-pool-backend-and-migration") {
    if (task.status !== "implemented") {
      errors.push("organization-role-pool-backend-and-migration must stay implemented after backend and migration closeout");
    }
    requireIncludes(task.resourceLocks, ["query-command-contract"], `${task.id} resourceLocks`, errors);
    requireIncludes(
      task.evidence?.tests,
      [
        "internal/platform/organizationrbac/validation_test.go",
        "internal/platform/organizationrbac/gorm_repository_test.go",
        "internal/platform/organizationrbac/migration_test.go",
      ],
      `${task.id} evidence.tests`,
      errors,
    );
  }
  if (task.id === "organization-user-admin-experience") {
    if (task.status !== "implemented") {
      errors.push("organization-user-admin-experience must stay implemented after Admin experience closeout");
    }
    requireIncludes(
      task.evidence?.validators,
      [
        "scripts/validate-platform-organization-rbac-menu-contract.mjs",
        "scripts/validate-platform-admin-api-boundary.mjs",
        "scripts/validate-admin-i18n.mjs",
        "scripts/validate-admin-ui-contracts.mjs",
      ],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(task.evidence?.tests, requiredAdminUIContractTests, `${task.id} evidence.tests`, errors);
    requireIncludes(task.evidence?.screenshots, [requiredOrganizationUserEvidenceManifest], `${task.id} evidence.screenshots`, errors);
    requireIncludes(task.evidence?.skills, ["ui-ux-pro-max"], `${task.id} evidence.skills`, errors);
  }
  if (task.id === "role-tree-and-authorization-entry") {
    if (task.status !== "implemented") {
      errors.push("role-tree-and-authorization-entry must stay implemented after role Admin closeout");
    }
    requireIncludes(
      task.evidence?.validators,
      [
        "scripts/validate-platform-organization-rbac-menu-contract.mjs",
        "scripts/validate-platform-governance-topology.mjs",
        "scripts/validate-platform-admin-api-boundary.mjs",
        "scripts/validate-admin-service-object-definitions.mjs",
        "scripts/validate-admin-i18n.mjs",
        "scripts/validate-admin-ui-contracts.mjs",
      ],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(task.evidence?.tests, requiredAdminUIContractTests, `${task.id} evidence.tests`, errors);
    requireIncludes(task.evidence?.screenshots, [requiredRoleTreeEvidenceManifest], `${task.id} evidence.screenshots`, errors);
    requireIncludes(task.evidence?.skills, ["ui-ux-pro-max"], `${task.id} evidence.skills`, errors);
  }
  if (task.id === "menu-tree-and-button-permission-configuration") {
    if (task.status !== "implemented") {
      errors.push("menu-tree-and-button-permission-configuration must stay implemented after menu governance closeout");
    }
    const statusReasonZH = task.statusReason?.zh ?? "";
    const statusReasonEN = task.statusReason?.en ?? "";
    if (/尚未区分|未实现/.test(statusReasonZH) || /do not distinguish|not implemented/i.test(statusReasonEN)) {
      errors.push("menu-tree-and-button-permission-configuration statusReason contradicts its implemented state");
    }
    const implementationBoundary = task.implementationBoundary ?? {};
    if (
      !sameList(values(implementationBoundary.implementedScope), [
        "directory-page-menu",
        "route-metadata",
        "page-buttons",
        "role-menu-assignment-contract",
      ])
      || !sameList(values(implementationBoundary.closedGates), [
        "target-menu-serving",
        "role-menu-migration-writes",
        "all-principal-dual-read",
        "cutover-rollback",
      ])
      || implementationBoundary.ownerTask !== "organization-rbac-menu-e2e-qa"
    ) {
      errors.push("menu-tree-and-button-permission-configuration implementationBoundary must preserve implemented scope, closed gates and owner task");
    }
    requireIncludes(
      task.evidence?.validators,
      [
        "scripts/validate-platform-organization-rbac-menu-contract.mjs",
        "scripts/validate-admin-i18n.mjs",
        "scripts/validate-admin-ui-contracts.mjs",
      ],
      `${task.id} evidence.validators`,
      errors,
    );
    requireIncludes(task.evidence?.tests, requiredAdminUIContractTests, `${task.id} evidence.tests`, errors);
    requireIncludes(task.evidence?.screenshots, [requiredMenuEvidenceManifest], `${task.id} evidence.screenshots`, errors);
    requireIncludes(task.evidence?.skills, ["ui-ux-pro-max"], `${task.id} evidence.skills`, errors);
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
  const baselineTasks = tasks.slice(0, foundationBaselineTaskIDs.length);
  if (!sameList(baselineTasks.map((item) => item.id), foundationBaselineTaskIDs) || baselineTasks.some((item) => item.status !== "implemented")) {
    errors.push("Task 8 baseline must preserve the original 37 implemented task nodes in order");
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

function validateCompletionProgram(tasks, errors) {
  const taskIDs = tasks.map((task) => task.id);
  for (const taskID of approvedCompletionProgramTaskIDs) {
    if (!taskIDs.includes(taskID)) {
      errors.push(`approved completion program task is missing: ${taskID}`);
    }
  }
  const completionProgramTaskIDs = taskIDs.filter((taskID) => approvedCompletionProgramTaskIDs.includes(taskID));
  if (completionProgramTaskIDs.length === approvedCompletionProgramTaskIDs.length && !sameList(completionProgramTaskIDs, approvedCompletionProgramTaskIDs)) {
    errors.push("completion program task order must match approved order");
  }
  const tasksByID = new Map(tasks.map((task) => [task.id, task]));
  for (const [taskID, dependencies] of requiredRemainingTaskDependencies) {
    if (!sameList(values(tasksByID.get(taskID)?.dependsOn), dependencies)) {
      errors.push(`task ${taskID} dependencies must match the approved remaining topology`);
    }
  }
}

function validateReleaseLanes(graph, tasks, tasksByID, errors) {
  const release = values(graph.releaseBlockingNodes);
  const optional = values(graph.postReleaseOptionalNodes);
  errors.push(...uniqueErrors(release, "releaseBlockingNodes"));
  errors.push(...uniqueErrors(optional, "postReleaseOptionalNodes"));

  for (const taskID of release) {
    if (!tasksByID.has(taskID)) {
      errors.push(`releaseBlockingNodes references unknown task ${taskID}`);
    }
  }
  for (const taskID of optional) {
    if (!tasksByID.has(taskID)) {
      errors.push(`postReleaseOptionalNodes references unknown task ${taskID}`);
    }
  }

  const releaseSet = new Set(release);
  const optionalSet = new Set(optional);
  for (const taskID of releaseSet) {
    if (optionalSet.has(taskID)) {
      errors.push(`release lanes overlap at ${taskID}`);
    }
  }

  const orderedRelease = tasks.filter((task) => releaseSet.has(task.id)).map((task) => task.id);
  const orderedOptional = tasks.filter((task) => optionalSet.has(task.id)).map((task) => task.id);
  if (!sameList(release, orderedRelease)) {
    errors.push("releaseBlockingNodes must preserve task graph order");
  }
  if (!sameList(optional, orderedOptional)) {
    errors.push("postReleaseOptionalNodes must preserve task graph order");
  }

  const unfinished = tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
  const laneUnion = tasks.filter((task) => releaseSet.has(task.id) || optionalSet.has(task.id)).map((task) => task.id);
  if (!sameList(laneUnion, unfinished) || release.length + optional.length !== unfinished.length) {
    errors.push("release lane union must exactly match unfinished task graph nodes in graph order");
  }

  for (const taskID of postReleaseOptionalNodes) {
    const task = tasksByID.get(taskID);
    if (!optionalSet.has(taskID)) {
      errors.push(`postReleaseOptionalNodes must include ${taskID}`);
    }
    if (task?.status !== "deferred") {
      errors.push(`post-release optional task ${taskID} must be deferred`);
    }
  }
  for (const taskID of releaseBlockingNodes) {
    const task = tasksByID.get(taskID);
    if (!releaseSet.has(taskID)) {
      errors.push(`releaseBlockingNodes must include ${taskID}`);
    }
    if (task?.status === "deferred") {
      errors.push(`release blocker ${taskID} must not be deferred`);
    }
    if (task?.status === "implemented") {
      errors.push(`release blocker ${taskID} must remain unfinished while listed`);
    }
    for (const optionalTaskID of postReleaseOptionalNodes) {
      if (hasDependencyPath(tasksByID, taskID, optionalTaskID)) {
        errors.push(`release blocker ${taskID} must not depend on post-release optional task ${optionalTaskID}`);
      }
    }
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
      if (task.status === "deferred") {
        errors.push(`parallel batch ${batch.id} must not schedule deferred task ${taskID}`);
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
  requireIncludes(graph.resourceLocks, requiredRemainingResourceLocks, "resourceLocks", errors);

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
  validateCompletionProgram(tasks, errors);
  validateReleaseLanes(graph, tasks, tasksByID, errors);
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
