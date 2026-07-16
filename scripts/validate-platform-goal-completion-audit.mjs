import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { isExternalReviewArtifactURI } from "./external-review-artifacts.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/platform-goal-completion-audit.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const taskExecutionPath = path.resolve(repoRoot, argValue("--task-execution", "resources/platform-task-execution-audit.json"));
const engineeringPath = path.resolve(repoRoot, argValue("--engineering", "resources/platform-engineering-capabilities.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const discoveryPath = path.resolve(repoRoot, argValue("--discovery", "resources/platform-reference-discovery.json"));
const coveragePath = path.resolve(repoRoot, argValue("--coverage", "resources/platform-reference-coverage.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const codegenReadinessPath = path.resolve(repoRoot, argValue("--codegen-readiness", "resources/platform-codegen-source-writing-readiness.json"));
const codegenReviewPath = path.resolve(repoRoot, argValue("--codegen-review", "resources/generated/admin-scaffold-promotion-review.json"));
const deploymentTopologyPath = path.resolve(repoRoot, argValue("--deployment-topology", "resources/platform-deployment-topology.json"));
const readmePath = path.resolve(repoRoot, argValue("--readme", "README.md"));
const agentsPath = path.resolve(repoRoot, argValue("--guidance", "docs/platform-capability-development.md"));

const requiredRequirementIDs = [
  "approved-stack-route",
  "business-neutral-reference-boundary",
  "capability-profile-and-plugin-composition",
  "common-admin-resource-schema-api",
  "admin-ui-i18n-design-gates",
  "reference-coverage-floor",
  "task-dependency-conflict-governance",
  "deployment-topology-runtime-boundary",
  "production-auth-promotion-gate",
  "source-writing-codegen-promotion-gate",
  "promotion-evidence-template-gate",
  "completion-state-control",
  "objective-conformance-control",
  "quality-closeout-gate",
];

const futurePromotionGateRequirements = new Set([
  "production-auth-promotion-gate",
  "source-writing-codegen-promotion-gate",
]);
const requiredAdminUIContractTests = ["scripts/admin-ui-contracts.test.mjs"];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function readText(filePath) {
  return fs.readFileSync(filePath, "utf8");
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

function validateTaskSummary(audit, graph, taskExecution, errors) {
  const tasks = values(graph.tasks);
  const implemented = tasks.filter((task) => task.status === "implemented");
  const unfinished = tasks.filter((task) => task.status !== "implemented");
  const summary = audit.taskSummary ?? {};

  if (summary.expectedTotal !== tasks.length) {
    errors.push(`taskSummary.expectedTotal must match task graph total ${tasks.length}`);
  }
  if (summary.expectedImplemented !== implemented.length) {
    errors.push(`taskSummary.expectedImplemented must match implemented task count ${implemented.length}`);
  }
  if (summary.expectedControlledUnfinished !== unfinished.length) {
    errors.push(`taskSummary.expectedControlledUnfinished must match unfinished task count ${unfinished.length}`);
  }

  const controlled = values(audit.completionPolicy?.requiredControlledUnfinishedNodes);
  const unfinishedIDs = unfinished.map((task) => task.id);
  if (!sameList(controlled, unfinishedIDs)) {
    errors.push("completionPolicy.requiredControlledUnfinishedNodes must exactly match unfinished task graph nodes in graph order");
  }
  if (!sameList(values(taskExecution.requiredUnfinishedNodes), unfinishedIDs)) {
    errors.push("task execution requiredUnfinishedNodes must exactly match unfinished task graph nodes in graph order");
  }
}

function validateCompletionStatus(audit, graph, productionAuth, codegenReadiness, codegenReview, errors) {
  const unfinished = values(graph.tasks).filter((task) => task.status !== "implemented");

  if (audit.completionPolicy?.mustNotMarkGoalCompleteWhileUnfinishedNodesExist !== true) {
    errors.push("completionPolicy.mustNotMarkGoalCompleteWhileUnfinishedNodesExist must stay true");
  }
  if (audit.completionPolicy?.mustNotSelfCertifyExternalApprovalEvidence !== true) {
    errors.push("completionPolicy.mustNotSelfCertifyExternalApprovalEvidence must stay true");
  }
  if (audit.completionPolicy?.businessReferenceMode !== "reference-only") {
    errors.push("completionPolicy.businessReferenceMode must stay reference-only");
  }
  if (unfinished.length > 0 && audit.completionStatus !== "not-complete-controlled") {
    errors.push("completionStatus must stay not-complete-controlled while unfinished task nodes or promotion blockers are active");
  }
  if (unfinished.length === 0 && audit.completionStatus !== "complete") {
    errors.push("completionStatus must be complete when all foundation task nodes are implemented");
  }
  if (productionAuth.sessionCredentialPolicy?.refreshTokenFamily?.defaultRuntime !== "disabled") {
    errors.push("production auth refresh-token-family default runtime must stay disabled after foundation completion");
  }
  if (codegenReadiness.mode?.sourceWriting !== "disabled" || codegenReview.mode?.sourceWriting !== "disabled") {
    errors.push("source-writing codegen must stay disabled after foundation completion");
  }
}

function validateBusinessNeutralBoundary({ audit, profiles, discovery, coverage, readme, agents, errors }) {
  if (!readme.includes("not a business migration target")) {
    errors.push("README.md must state platform-go is not a business migration target");
  }
  if (!agents.includes("informed by reusable management patterns observed in `zshenmez`")) {
    errors.push("public capability guidance must describe zshenmez as reusable management pattern reference, not migrated business source");
  }
  requireIncludes(
    discovery.mustStayOutOfDefaultProfile,
    ["external-business-capability"],
    "reference discovery mustStayOutOfDefaultProfile",
    errors,
  );
  for (const profile of values(profiles.profiles)) {
    if (profile.default === true && values(profile.capabilities).includes("external-business-capability")) {
      errors.push(`default profile ${profile.id} must not include external-business-capability`);
    }
    if (profile.default === true && !values(profile.mustExcludeCapabilities).includes("external-business-capability")) {
      errors.push(`default profile ${profile.id} must explicitly exclude external-business-capability`);
    }
  }
  for (const boundary of values(coverage.businessBoundary)) {
    if (boundary.expectedCapability !== "external-business-capability") {
      errors.push(`businessBoundary ${boundary.area} must be owned by external-business-capability`);
    }
    if (boundary.mustStayOutOfDefaultPlatform !== true) {
      errors.push(`businessBoundary ${boundary.area} must stay out of the default platform`);
    }
  }

  const businessRequirement = values(audit.requirements).find((item) => item.id === "business-neutral-reference-boundary");
  if (businessRequirement?.status !== "verified") {
    errors.push("requirement business-neutral-reference-boundary must be verified");
  }
}

function validateRequirements(audit, taskExecution, engineering, errors) {
  const requirements = values(audit.requirements);
  const seen = new Set();
  for (const requirement of requirements) {
    const prefix = `requirement ${requirement.id ?? "<missing>"}`;
    if (!requirement.id) {
      errors.push("requirement is missing id");
      continue;
    }
    if (seen.has(requirement.id)) {
      errors.push(`requirements contains duplicate id ${requirement.id}`);
    }
    seen.add(requirement.id);
    if (!hasLocalizedText(requirement.label)) {
      errors.push(`${prefix} must declare zh/en label`);
    }
    if (!["verified", "controlled-blocker"].includes(requirement.status)) {
      errors.push(`${prefix} has unsupported status ${requirement.status}`);
    }
    if (futurePromotionGateRequirements.has(requirement.id)) {
      if (requirement.status !== "verified") {
        errors.push(`${prefix} must be verified after foundation completion`);
      }
      requireIncludes(
        requirement.requiredBeforeRuntimeMutation,
        ["external absolute artifact URI evidence"],
        `${prefix}.requiredBeforeRuntimeMutation`,
        errors,
      );
    } else if (requirement.status !== "verified") {
      errors.push(`${prefix} must be verified`);
    }

    const evidence = requirement.evidence ?? {};
    for (const relativePath of [
      ...values(evidence.sourcePaths),
      ...values(evidence.generatedFiles),
      ...values(evidence.validators),
      ...values(evidence.tests),
    ]) {
      if (!relativeExistingPath(relativePath)) {
        errors.push(`${prefix} evidence path is missing or unsafe: ${relativePath}`);
      }
    }
    for (const screenshotPath of values(evidence.screenshots)) {
      if (!relativeExistingPath(screenshotPath) && !isExternalReviewArtifactURI(screenshotPath)) {
        errors.push(`${prefix} screenshot evidence path is missing or unsafe: ${screenshotPath}`);
      }
    }
  }
  for (const requirementID of requiredRequirementIDs) {
    if (!seen.has(requirementID)) {
      errors.push(`missing required completion requirement ${requirementID}`);
    }
  }

  const engineeringIDs = new Set(values(engineering.capabilities).map((capability) => capability.id));
  if (!engineeringIDs.has("goal-completion-audit")) {
    errors.push("engineering capability matrix must include goal-completion-audit");
  }
  if (!engineeringIDs.has("node-closeout-audit")) {
    errors.push("engineering capability matrix must include node-closeout-audit");
  }
  if (!engineeringIDs.has("deployment-topology-gate")) {
    errors.push("engineering capability matrix must include deployment-topology-gate");
  }

  const promotionEvidenceGate = requirements.find((item) => item.id === "promotion-evidence-template-gate");
  requireIncludes(
    promotionEvidenceGate?.evidence?.validators,
    ["scripts/validate-platform-promotion-evidence-package.mjs"],
    "promotion-evidence-template-gate evidence.validators",
    errors,
  );

  const closeoutGate = requirements.find((item) => item.id === "quality-closeout-gate");
  requireIncludes(
    closeoutGate?.evidence?.sourcePaths,
    ["resources/platform-node-closeout-audit.json"],
    "quality-closeout-gate evidence.sourcePaths",
    errors,
  );
  requireIncludes(
    closeoutGate?.evidence?.validators,
    ["scripts/validate-platform-node-closeout-audit.mjs"],
    "quality-closeout-gate evidence.validators",
    errors,
  );

  const adminUIGate = requirements.find((item) => item.id === "admin-ui-i18n-design-gates");
  requireIncludes(
    adminUIGate?.evidence?.tests,
    requiredAdminUIContractTests,
    "admin-ui-i18n-design-gates evidence.tests",
    errors,
  );
}

function validateDeploymentTopologyBoundary(audit, deploymentTopology, errors) {
  const requirement = values(audit.requirements).find((item) => item.id === "deployment-topology-runtime-boundary");
  if (requirement?.status !== "verified") {
    errors.push("requirement deployment-topology-runtime-boundary must be verified");
  }
  requireIncludes(
    requirement?.evidence?.sourcePaths,
    ["resources/platform-deployment-topology.json", "docs/platform-deployment.md"],
    "deployment-topology-runtime-boundary evidence.sourcePaths",
    errors,
  );
  requireIncludes(
    requirement?.evidence?.sourcePaths,
    [
      "Dockerfile",
      ".dockerignore",
      "deploy/compose/docker-compose.prod.yml",
      "deploy/nginx/platform.conf",
      "deploy/env/production.example.env",
    ],
    "deployment-topology-runtime-boundary evidence.sourcePaths",
    errors,
  );
  requireIncludes(
    requirement?.evidence?.validators,
    ["scripts/validate-platform-deployment-topology.mjs"],
    "deployment-topology-runtime-boundary evidence.validators",
    errors,
  );
  requireIncludes(
    requirement?.evidence?.tests,
    [
      "scripts/platform-deployment-topology.test.mjs",
      "scripts/platform-production-readiness.test.mjs",
      "scripts/platform-engineering-capabilities.test.mjs",
    ],
    "deployment-topology-runtime-boundary evidence.tests",
    errors,
  );

  const decision = deploymentTopology.decision ?? {};
  if (decision.vercelRequired !== false) {
    errors.push("deployment topology decision.vercelRequired must stay false");
  }
  if (decision.selectedTopology !== "single-service-production") {
    errors.push("deployment topology decision.selectedTopology must stay single-service-production");
  }
  if (decision.defaultApiRuntime !== "long-lived-service") {
    errors.push("deployment topology decision.defaultApiRuntime must stay long-lived-service");
  }
  if (decision.fullStackVercelGo !== "requires-separate-adapter-spec") {
    errors.push("deployment topology decision.fullStackVercelGo must stay requires-separate-adapter-spec");
  }

  const deploymentPackage = deploymentTopology.deploymentPackage ?? {};
  if (deploymentPackage.status !== "implemented") {
    errors.push("deployment package must stay implemented");
  }
  if (deploymentPackage.defaultTopology !== "single-service-production") {
    errors.push("deployment package defaultTopology must stay single-service-production");
  }
  if (deploymentPackage.selectedTopology !== "single-service-production") {
    errors.push("deployment package selectedTopology must stay single-service-production");
  }
  if (deploymentPackage.dockerfile !== "Dockerfile") {
    errors.push("deployment package dockerfile must stay Dockerfile");
  }
  if (deploymentPackage.composeFile !== "deploy/compose/docker-compose.prod.yml") {
    errors.push("deployment package composeFile must stay deploy/compose/docker-compose.prod.yml");
  }
  if (deploymentPackage.adminProxy !== "deploy/nginx/platform.conf") {
    errors.push("deployment package adminProxy must stay deploy/nginx/platform.conf");
  }
  if (deploymentPackage.envTemplate !== "deploy/env/production.example.env") {
    errors.push("deployment package envTemplate must stay deploy/env/production.example.env");
  }
  if (deploymentPackage.dockerTargets?.api !== "api") {
    errors.push("deployment package dockerTargets.api must stay api");
  }
  if (deploymentPackage.dockerTargets?.admin !== "admin-static") {
    errors.push("deployment package dockerTargets.admin must stay admin-static");
  }
}

function validateApprovedStack(audit, engineering, errors) {
  if (values(audit.approvedStack?.backend).join(",") !== "Gin,GORM,Casbin,JWT") {
    errors.push("approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (values(audit.approvedStack?.frontend).join(",") !== "Refine,React,Ant Design") {
    errors.push("approvedStack.frontend must stay Refine + React + Ant Design");
  }
  if (values(engineering.stack?.backend).join(",") !== values(audit.approvedStack?.backend).join(",")) {
    errors.push("goal completion audit backend stack must match engineering matrix");
  }
  if (values(engineering.stack?.frontend).join(",") !== values(audit.approvedStack?.frontend).join(",")) {
    errors.push("goal completion audit frontend stack must match engineering matrix");
  }
}

function validate() {
  const audit = readJSON(auditPath);
  const graph = readJSON(taskGraphPath);
  const taskExecution = readJSON(taskExecutionPath);
  const engineering = readJSON(engineeringPath);
  const profiles = readJSON(profilesPath);
  const discovery = readJSON(discoveryPath);
  const coverage = readJSON(coveragePath);
  const productionAuth = readJSON(productionAuthPath);
  const codegenReadiness = readJSON(codegenReadinessPath);
  const codegenReview = readJSON(codegenReviewPath);
  const deploymentTopology = readJSON(deploymentTopologyPath);
  const readme = readText(readmePath);
  const agents = readText(agentsPath);
  const errors = [];

  validateApprovedStack(audit, engineering, errors);
  validateTaskSummary(audit, graph, taskExecution, errors);
  validateCompletionStatus(audit, graph, productionAuth, codegenReadiness, codegenReview, errors);
  validateBusinessNeutralBoundary({ audit, profiles, discovery, coverage, readme, agents, errors });
  validateRequirements(audit, taskExecution, engineering, errors);
  validateDeploymentTopologyBoundary(audit, deploymentTopology, errors);

  return { audit, errors };
}

const { audit, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(
  `Validated platform goal completion audit in ${path.relative(repoRoot, auditPath)} (${audit.completionStatus})`,
);
