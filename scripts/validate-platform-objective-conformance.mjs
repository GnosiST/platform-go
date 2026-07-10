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

const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/platform-objective-conformance.json"));
const alignmentPath = path.resolve(repoRoot, argValue("--alignment", "resources/platform-foundation-alignment-audit.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const taskExecutionPath = path.resolve(repoRoot, argValue("--task-execution", "resources/platform-task-execution-audit.json"));
const goalPath = path.resolve(repoRoot, argValue("--goal", "resources/platform-goal-completion-audit.json"));
const engineeringPath = path.resolve(repoRoot, argValue("--engineering", "resources/platform-engineering-capabilities.json"));
const capabilityContractsPath = path.resolve(repoRoot, argValue("--capability-contracts", "resources/platform-capability-contracts.json"));
const productionReadinessPath = path.resolve(repoRoot, argValue("--production-readiness", "resources/platform-production-readiness.json"));
const deploymentTopologyPath = path.resolve(repoRoot, argValue("--deployment-topology", "resources/platform-deployment-topology.json"));
const readmePath = path.resolve(repoRoot, argValue("--readme", "README.md"));
const agentsPath = path.resolve(repoRoot, argValue("--agents", "AGENTS.md"));

const approvedBackendStack = ["Gin", "GORM", "Casbin", "JWT"];
const approvedFrontendStack = ["Refine", "React", "Ant Design"];
const visualGateOrder = ["superpowers:brainstorming", "product-design"];
const requiredFuturePromotionGates = ["production-auth-provider-hardening", "source-writing-codegen-promotion"];
const requiredDocsRoots = ["README.md", "AGENTS.md", "docs/"];
const requiredEvidencePaths = [
  "resources/platform-objective-conformance.json",
  "resources/platform-foundation-task-graph.json",
  "resources/platform-foundation-alignment-audit.json",
  "resources/platform-task-execution-audit.json",
  "resources/platform-goal-completion-audit.json",
  "resources/platform-engineering-capabilities.json",
  "resources/platform-capability-contracts.json",
  "resources/platform-production-readiness.json",
  "resources/platform-deployment-topology.json",
  "resources/platform-node-closeout-audit.json",
  "scripts/validate-platform-objective-conformance.mjs",
  "scripts/validate-platform-foundation-task-graph.mjs",
  "scripts/validate-platform-node-closeout-audit.mjs",
  "scripts/validate-platform-capability-contracts.mjs",
  "scripts/platform-capability-contracts.test.mjs",
  "scripts/platform-foundation-task-graph.test.mjs",
  "scripts/platform-objective-conformance.test.mjs",
  "scripts/platform-foundation-alignment.test.mjs",
  "scripts/platform-goal-completion-audit.test.mjs",
  "scripts/platform-task-execution-audit.test.mjs",
  "scripts/platform-node-closeout-audit.test.mjs",
  "scripts/platform-deployment-topology.test.mjs",
  "scripts/platform-engineering-capabilities.test.mjs",
  "scripts/platform-production-readiness.test.mjs",
  "scripts/admin-ui-contracts.test.mjs",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function readText(filePath) {
  return fs.readFileSync(filePath, "utf8");
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(actual, expected) {
  return values(actual).join("\u0000") === values(expected).join("\u0000");
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const item of required) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function validateApprovedStack(audit, alignment, goal, engineering, errors) {
  if (!sameList(audit.approvedStack?.backend, approvedBackendStack)) {
    errors.push("approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(audit.approvedStack?.frontend, approvedFrontendStack)) {
    errors.push("approvedStack.frontend must stay Refine + React + Ant Design");
  }
  if (!sameList(alignment.approvedStack?.backend, approvedBackendStack)) {
    errors.push("alignment approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(alignment.approvedStack?.frontend, approvedFrontendStack)) {
    errors.push("alignment approvedStack.frontend must stay Refine + React + Ant Design");
  }
  if (!sameList(goal.approvedStack?.backend, approvedBackendStack)) {
    errors.push("goal approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(goal.approvedStack?.frontend, approvedFrontendStack)) {
    errors.push("goal approvedStack.frontend must stay Refine + React + Ant Design");
  }
  if (!sameList(engineering.stack?.backend, approvedBackendStack)) {
    errors.push("engineering stack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(engineering.stack?.frontend, approvedFrontendStack)) {
    errors.push("engineering stack.frontend must stay Refine + React + Ant Design");
  }
}

function validateReferencePolicy(audit, readme, agents, errors) {
  const policy = audit.referenceProjectPolicy ?? {};
  if (policy.referenceProject !== "/Users/irainbow/Documents/DevelopmentSpace/myProject/zshenmez") {
    errors.push("referenceProjectPolicy.referenceProject must stay the zshenmez reference path");
  }
  if (policy.mode !== "reference-only") {
    errors.push("referenceProjectPolicy.mode must stay reference-only");
  }
  if (policy.defaultPlatformBusinessMigration !== "forbidden") {
    errors.push("referenceProjectPolicy.defaultPlatformBusinessMigration must stay forbidden");
  }
  if (policy.businessCapabilityOwnership !== "external-business-capability") {
    errors.push("referenceProjectPolicy.businessCapabilityOwnership must stay external-business-capability");
  }
  if (policy.foundationExtractionRequiresDiscovery !== true) {
    errors.push("referenceProjectPolicy.foundationExtractionRequiresDiscovery must stay true");
  }
  if (!readme.includes("not a business migration target")) {
    errors.push("README.md must state platform-go is not a business migration target");
  }
  if (!agents.includes("informed by reusable management patterns observed in `zshenmez`")) {
    errors.push("AGENTS.md must keep zshenmez as reusable management pattern reference");
  }
}

function validateQualityPolicy(audit, alignment, errors) {
  const policy = audit.qualityPolicy ?? {};
  for (const key of ["architectureFirst", "noArtificialStacking", "noCompromiseImplementation", "businessNeutralFoundation"]) {
    if (policy[key] !== true) {
      errors.push(`qualityPolicy.${key} must stay true`);
    }
  }
  if (policy.conflictResolution !== "objective-wins") {
    errors.push("qualityPolicy.conflictResolution must stay objective-wins");
  }
  if (alignment.objectiveConflictPolicy?.mode !== "fail-fast") {
    errors.push("alignment objectiveConflictPolicy.mode must stay fail-fast");
  }
}

function validateTaskControl(audit, alignment, taskGraph, taskExecution, goal, errors) {
  const policy = audit.taskControlPolicy ?? {};
  if (policy.taskGraph !== "resources/platform-foundation-task-graph.json") {
    errors.push("taskControlPolicy.taskGraph must stay resources/platform-foundation-task-graph.json");
  }
  if (policy.taskExecutionAudit !== "resources/platform-task-execution-audit.json") {
    errors.push("taskControlPolicy.taskExecutionAudit must stay resources/platform-task-execution-audit.json");
  }
  for (const key of ["requiresResourceLocks", "requiresDependencyConflictChecks", "requiresTaskGraphBeforeNewNode"]) {
    if (policy[key] !== true) {
      errors.push(`taskControlPolicy.${key} must stay true`);
    }
  }

  if (values(policy.requiredUnfinishedNodes).length !== 0) {
    errors.push("taskControlPolicy.requiredUnfinishedNodes must be empty after foundation completion");
  }
  if (values(taskExecution.requiredUnfinishedNodes).length !== 0) {
    errors.push("task execution requiredUnfinishedNodes must be empty after foundation completion");
  }
  if (values(alignment.requiredFutureTaskNodes).length !== 0) {
    errors.push("alignment.requiredFutureTaskNodes must be empty after foundation completion");
  }
  if (values(goal.completionPolicy?.requiredControlledUnfinishedNodes).length !== 0) {
    errors.push("goal completionPolicy.requiredControlledUnfinishedNodes must be empty after foundation completion");
  }

  const unfinishedTaskIDs = values(taskGraph.tasks).filter((task) => task.status !== "implemented").map((task) => task.id);
  if (unfinishedTaskIDs.length !== 0) {
    errors.push(`unfinished task graph nodes must be empty after foundation completion: ${unfinishedTaskIDs.join(", ")}`);
  }
  if (values(taskGraph.resourceLocks).length === 0) {
    errors.push("task graph resourceLocks must not be empty");
  }
  if (values(taskGraph.resourceLockConflictGroups).length === 0) {
    errors.push("task graph resourceLockConflictGroups must not be empty");
  }
}

function validateVisualPolicy(audit, alignment, taskExecution, errors) {
  const policy = audit.visualPolicy ?? {};
  if (!sameList(policy.requiredOrder, visualGateOrder)) {
    errors.push("visualPolicy.requiredOrder must stay superpowers:brainstorming before product-design");
  }
  if (policy.requiredForVisualTasks !== true) {
    errors.push("visualPolicy.requiredForVisualTasks must stay true");
  }
  if (policy.productDesignRequiredForVisualImplementation !== true) {
    errors.push("visualPolicy.productDesignRequiredForVisualImplementation must stay true");
  }
  if (policy.i18nRequiredForSharedAdminComponents !== true) {
    errors.push("visualPolicy.i18nRequiredForSharedAdminComponents must stay true");
  }
  if (!sameList(alignment.visualDesignGate?.requiredOrder, visualGateOrder)) {
    errors.push("alignment visualDesignGate.requiredOrder must stay superpowers:brainstorming before product-design");
  }
  if (taskExecution.statusPolicy?.visualTasksRequireProductDesign !== true) {
    errors.push("task execution statusPolicy.visualTasksRequireProductDesign must stay true");
  }
}

function validateCapabilityPolicy(audit, capabilityContracts, engineering, productionReadiness, errors) {
  const policy = audit.capabilityPolicy ?? {};
  if (policy.contract !== "resources/platform-capability-contracts.json") {
    errors.push("capabilityPolicy.contract must stay resources/platform-capability-contracts.json");
  }
  if (policy.profiles !== "resources/platform-capability-profiles.json") {
    errors.push("capabilityPolicy.profiles must stay resources/platform-capability-profiles.json");
  }
  if (policy.defaultProfile !== "platform-default") {
    errors.push("capabilityPolicy.defaultProfile must stay platform-default");
  }
  if (policy.businessCapabilityOwnership !== "external-business-capability") {
    errors.push("capabilityPolicy.businessCapabilityOwnership must stay external-business-capability");
  }
  if (policy.runtimeManifestMutation !== "forbidden") {
    errors.push("capabilityPolicy.runtimeManifestMutation must stay forbidden");
  }
  if (policy.defaultProfileBusinessCapabilitiesAllowed !== false) {
    errors.push("capabilityPolicy.defaultProfileBusinessCapabilitiesAllowed must stay false");
  }
  if (!relativeExistingPath(policy.contract)) {
    errors.push(`capabilityPolicy.contract path is missing or unsafe: ${policy.contract}`);
  }
  if (!relativeExistingPath(policy.profiles)) {
    errors.push(`capabilityPolicy.profiles path is missing or unsafe: ${policy.profiles}`);
  }
  if (capabilityContracts.policies?.defaultProfile !== policy.defaultProfile) {
    errors.push("capability contracts defaultProfile must match objective capabilityPolicy.defaultProfile");
  }
  if (capabilityContracts.policies?.runtimeManifestMutation !== policy.runtimeManifestMutation) {
    errors.push("capability contracts runtimeManifestMutation must match objective capabilityPolicy");
  }
  if (capabilityContracts.policies?.defaultProfileBusinessCapabilitiesAllowed !== policy.defaultProfileBusinessCapabilitiesAllowed) {
    errors.push("capability contracts defaultProfileBusinessCapabilitiesAllowed must match objective capabilityPolicy");
  }
  const externalBusinessContract = values(capabilityContracts.capabilities).find(
    (capability) => capability.id === policy.businessCapabilityOwnership,
  );
  if (!externalBusinessContract) {
    errors.push(`capability contracts must classify ${policy.businessCapabilityOwnership}`);
  } else if (externalBusinessContract.profilePolicy !== "business-external-only") {
    errors.push(`${policy.businessCapabilityOwnership} must stay business-external-only`);
  }
  const capabilityIDs = new Set(values(engineering.capabilities).map((capability) => capability.id));
  if (!capabilityIDs.has("capability-contract-governance")) {
    errors.push("engineering capability matrix must include capability-contract-governance");
  }
  const preflightIDs = new Set(values(productionReadiness.preflightCommands).map((command) => command.id));
  if (!preflightIDs.has("capability-contracts")) {
    errors.push("production readiness preflightCommands must include capability-contracts");
  }
}

function validateCloseoutPolicy(audit, errors) {
  const policy = audit.closeoutPolicy ?? {};
  if (policy.neatFreakRequiredForNodeCloseout !== true) {
    errors.push("closeoutPolicy.neatFreakRequiredForNodeCloseout must stay true");
  }
  if (policy.knowledgeCleanupBeforeNodeCloseout !== true) {
    errors.push("closeoutPolicy.knowledgeCleanupBeforeNodeCloseout must stay true");
  }
  if (policy.nodeCloseoutAudit !== "resources/platform-node-closeout-audit.json") {
    errors.push("closeoutPolicy.nodeCloseoutAudit must be resources/platform-node-closeout-audit.json");
  }
  if (!relativeExistingPath(policy.nodeCloseoutAudit)) {
    errors.push(`closeoutPolicy.nodeCloseoutAudit path is missing or unsafe: ${policy.nodeCloseoutAudit}`);
  }
  requireIncludes(policy.docsRoots, requiredDocsRoots, "closeoutPolicy.docsRoots", errors);
  for (const docsRoot of values(policy.docsRoots)) {
    if (!relativeExistingPath(docsRoot)) {
      errors.push(`closeoutPolicy.docsRoots path is missing or unsafe: ${docsRoot}`);
    }
  }
}

function validateDeploymentPolicy(audit, alignment, deploymentTopology, errors) {
  const policy = audit.deploymentPolicy ?? {};
  const alignmentPolicy = alignment.deploymentPolicy ?? {};
  const decision = deploymentTopology.decision ?? {};
  if (policy.vercelRequired !== false) {
    errors.push("deploymentPolicy.vercelRequired must stay false");
  }
  if (policy.vercelAdminUsage !== "optional-static-admin") {
    errors.push("deploymentPolicy.vercelAdminUsage must stay optional-static-admin");
  }
  if (policy.selectedTopology !== "single-service-production") {
    errors.push("deploymentPolicy.selectedTopology must stay single-service-production");
  }
  if (policy.defaultApiRuntime !== "long-lived-service") {
    errors.push("deploymentPolicy.defaultApiRuntime must stay long-lived-service");
  }
  if (policy.fullStackVercelGo !== "requires-separate-adapter-spec") {
    errors.push("deploymentPolicy.fullStackVercelGo must stay requires-separate-adapter-spec");
  }
  if (alignmentPolicy.vercelRequired !== false) {
    errors.push("alignment deploymentPolicy.vercelRequired must stay false");
  }
  if (alignmentPolicy.vercelAdminUsage !== "optional-static-admin") {
    errors.push("alignment deploymentPolicy.vercelAdminUsage must stay optional-static-admin");
  }
  if (alignmentPolicy.selectedTopology !== "single-service-production") {
    errors.push("alignment deploymentPolicy.selectedTopology must stay single-service-production");
  }
  if (alignmentPolicy.defaultApiRuntime !== "long-lived-service") {
    errors.push("alignment deploymentPolicy.defaultApiRuntime must stay long-lived-service");
  }
  if (alignmentPolicy.fullStackVercelGo !== "requires-separate-adapter-spec") {
    errors.push("alignment deploymentPolicy.fullStackVercelGo must stay requires-separate-adapter-spec");
  }
  if (decision.vercelRequired !== false) {
    errors.push("deployment decision.vercelRequired must stay false");
  }
  if (decision.vercelAdminUsage !== "optional-static-admin") {
    errors.push("deployment decision.vercelAdminUsage must stay optional-static-admin");
  }
  if (decision.selectedTopology !== "single-service-production") {
    errors.push("deployment decision.selectedTopology must stay single-service-production");
  }
  if (decision.defaultApiRuntime !== "long-lived-service") {
    errors.push("deployment decision.defaultApiRuntime must stay long-lived-service");
  }
  if (decision.fullStackVercelGo !== "requires-separate-adapter-spec") {
    errors.push("deployment decision.fullStackVercelGo must stay requires-separate-adapter-spec");
  }
}

function validateCompletionPolicy(audit, goal, taskExecution, errors) {
  const policy = audit.completionPolicy ?? {};
  if (policy.goalCompletionStatus !== "complete") {
    errors.push("completionPolicy.goalCompletionStatus must be complete after foundation completion");
  }
  if (goal.completionStatus !== "complete") {
    errors.push("goal completionStatus must be complete after foundation completion");
  }
  if (policy.mustNotMarkCompleteWhileBlockersActive !== true) {
    errors.push("completionPolicy.mustNotMarkCompleteWhileBlockersActive must stay true");
  }
  if (policy.mustNotSelfCertifyExternalApprovalEvidence !== true) {
    errors.push("completionPolicy.mustNotSelfCertifyExternalApprovalEvidence must stay true");
  }
  if (values(policy.controlledBlockers).length !== 0) {
    errors.push("completionPolicy.controlledBlockers must be empty after foundation completion");
  }
  if (values(taskExecution.requiredUnfinishedNodes).length !== 0) {
    errors.push("task execution requiredUnfinishedNodes must be empty after foundation completion");
  }

  const gateIDs = values(audit.futurePromotionGates).map((gate) => gate.taskId);
  requireIncludes(gateIDs, requiredFuturePromotionGates, "futurePromotionGates", errors);
  for (const gate of values(audit.futurePromotionGates)) {
    if (!relativeExistingPath(gate.contract)) {
      errors.push(`futurePromotionGates.${gate.taskId ?? "<missing>"}.contract path is missing or unsafe: ${gate.contract}`);
    }
    requireIncludes(
      gate.requiredEvidenceBeforeRuntimeMutation,
      ["external absolute artifact URI evidence"],
      `futurePromotionGates.${gate.taskId ?? "<missing>"}.requiredEvidenceBeforeRuntimeMutation`,
      errors,
    );
  }
}

function validateEvidence(audit, engineering, productionReadiness, errors) {
  const evidence = audit.evidence ?? {};
  for (const relativePath of [
    ...values(evidence.contracts),
    ...values(evidence.validators),
    ...values(evidence.tests),
    ...values(evidence.docs),
  ]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`evidence path is missing or unsafe: ${relativePath}`);
    }
  }
  requireIncludes(
    [...values(evidence.contracts), ...values(evidence.validators), ...values(evidence.tests)],
    requiredEvidencePaths,
    "objective conformance evidence",
    errors,
  );

  const capabilities = values(engineering.capabilities);
  const objectiveCapability = capabilities.find((capability) => capability.id === "objective-conformance-gate");
  if (!objectiveCapability) {
    errors.push("engineering capability matrix must include objective-conformance-gate");
  } else {
    if (objectiveCapability.status !== "implemented") {
      errors.push("objective-conformance-gate status must be implemented");
    }
    requireIncludes(
      objectiveCapability.evidence?.sourcePaths,
      ["resources/platform-objective-conformance.json"],
      "objective-conformance-gate evidence.sourcePaths",
      errors,
    );
    requireIncludes(
      objectiveCapability.evidence?.validators,
      ["scripts/validate-platform-objective-conformance.mjs"],
      "objective-conformance-gate evidence.validators",
      errors,
    );
    requireIncludes(
      objectiveCapability.evidence?.tests,
      ["scripts/platform-objective-conformance.test.mjs"],
      "objective-conformance-gate evidence.tests",
      errors,
    );
  }

  const preflightIDs = new Set(values(productionReadiness.preflightCommands).map((command) => command.id));
  if (!preflightIDs.has("engineering-capabilities")) {
    errors.push("production readiness preflightCommands must include engineering-capabilities");
  }
  if (!preflightIDs.has("foundation-task-graph")) {
    errors.push("production readiness preflightCommands must include foundation-task-graph");
  }
  if (!preflightIDs.has("objective-conformance")) {
    errors.push("production readiness preflightCommands must include objective-conformance");
  }
}

function validate() {
  const audit = readJSON(auditPath);
  const alignment = readJSON(alignmentPath);
  const taskGraph = readJSON(taskGraphPath);
  const taskExecution = readJSON(taskExecutionPath);
  const goal = readJSON(goalPath);
  const engineering = readJSON(engineeringPath);
  const capabilityContracts = readJSON(capabilityContractsPath);
  const productionReadiness = readJSON(productionReadinessPath);
  const deploymentTopology = readJSON(deploymentTopologyPath);
  const readme = readText(readmePath);
  const agents = readText(agentsPath);
  const errors = [];

  if (!audit.purpose) {
    errors.push("objective conformance purpose is required");
  }
  if (audit.objectiveMode !== "persistent-full-scope") {
    errors.push("objectiveMode must stay persistent-full-scope");
  }

  validateApprovedStack(audit, alignment, goal, engineering, errors);
  validateReferencePolicy(audit, readme, agents, errors);
  validateQualityPolicy(audit, alignment, errors);
  validateTaskControl(audit, alignment, taskGraph, taskExecution, goal, errors);
  validateCapabilityPolicy(audit, capabilityContracts, engineering, productionReadiness, errors);
  validateVisualPolicy(audit, alignment, taskExecution, errors);
  validateCloseoutPolicy(audit, errors);
  validateDeploymentPolicy(audit, alignment, deploymentTopology, errors);
  validateCompletionPolicy(audit, goal, taskExecution, errors);
  validateEvidence(audit, engineering, productionReadiness, errors);

  return { audit, errors };
}

const { audit, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(
  `Validated platform objective conformance in ${path.relative(repoRoot, auditPath)} (${audit.objectiveMode})`,
);
