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

const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/platform-foundation-alignment-audit.json"));
const contractOverrides = {
  taskGraph: argValue("--task-graph", ""),
  engineeringCapabilities: argValue("--engineering-capabilities", ""),
  referenceDiscovery: argValue("--reference-discovery", ""),
  referenceCoverage: argValue("--reference-coverage", ""),
  adminApiBoundary: argValue("--admin-api-boundary", ""),
  appClientApiBoundary: argValue("--app-client-api-boundary", ""),
  governanceTopology: argValue("--governance-topology", ""),
  capabilityContracts: argValue("--capability-contracts", ""),
  capabilityProfiles: argValue("--capability-profiles", ""),
  codegenReadiness: argValue("--codegen-readiness", ""),
  formSchemaLayoutSlots: argValue("--form-schema-layout-slots", ""),
  fileStorageExperience: argValue("--file-storage-experience", ""),
  taskExecutionAudit: argValue("--task-execution-audit", ""),
  productionReadiness: argValue("--production-readiness", ""),
  deploymentTopology: argValue("--deployment-topology", ""),
  productionAuthHardening: argValue("--production-auth-hardening", ""),
  refreshTokenFamilyPromotion: argValue("--refresh-token-family-promotion", ""),
  promotionReview: argValue("--promotion-review", ""),
};

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
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

function validateRequiredOrder(sequence, requiredOrder, label) {
  const errors = [];
  if (requiredOrder.length === 0) {
    return errors;
  }
  let previousIndex = -1;
  for (const item of requiredOrder) {
    const index = sequence.indexOf(item);
    if (index === -1) {
      errors.push(`${label} must include ${item}`);
      continue;
    }
    if (index < previousIndex) {
      errors.push(`${label} must order ${requiredOrder.join(" before ")}`);
      return errors;
    }
    previousIndex = index;
  }
  return errors;
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function existingJSONPath(filePath) {
  if (!filePath) {
    return "";
  }
  const absolutePath = path.resolve(repoRoot, filePath);
  if (!fs.existsSync(absolutePath)) {
    return "";
  }
  return absolutePath;
}

function readContract(audit, key, errors) {
  const overridePath = existingJSONPath(contractOverrides[key]);
  if (overridePath) {
    return readJSON(overridePath);
  }
  const relativePath = audit.contracts?.[key];
  if (!relativeExistingPath(relativePath)) {
    errors.push(`alignment contract ${key} path is missing or unsafe: ${relativePath ?? "<missing>"}`);
    return {};
  }
  return readJSON(path.resolve(repoRoot, relativePath));
}

function runValidator(relativePath) {
  const result = spawnSync(process.execPath, [relativePath], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    return `${relativePath} failed\n${result.stdout}${result.stderr}`;
  }
  return "";
}

function validateStack(audit, taskGraph, engineering, errors) {
  const backend = values(audit.approvedStack?.backend);
  const frontend = values(audit.approvedStack?.frontend);
  if (!sameList(backend, ["Gin", "GORM", "Casbin", "JWT"])) {
    errors.push("alignment approvedStack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (!sameList(frontend, ["Refine", "React", "Ant Design"])) {
    errors.push("alignment approvedStack.frontend must stay Refine + React + Ant Design");
  }
  if (!sameList(values(taskGraph.approvedStack?.backend), backend)) {
    errors.push("task graph approvedStack.backend conflicts with alignment audit");
  }
  if (!sameList(values(taskGraph.approvedStack?.frontend), frontend)) {
    errors.push("task graph approvedStack.frontend conflicts with alignment audit");
  }
  if (!sameList(values(engineering.stack?.backend), backend)) {
    errors.push("engineering matrix stack.backend conflicts with alignment audit");
  }
  if (!sameList(values(engineering.stack?.frontend), frontend)) {
    errors.push("engineering matrix stack.frontend conflicts with alignment audit");
  }
}

function validateTaskGraph(audit, taskGraph, errors) {
  const tasks = values(taskGraph.tasks);
  const tasksByID = new Map(tasks.map((task) => [task.id, task]));
  const promotionGateTaskIDs = new Set(["production-auth-provider-hardening", "source-writing-codegen-promotion"]);
  for (const taskID of values(audit.nonDroppableGoalNodes)) {
    if (!values(audit.requiredTaskNodes).includes(taskID)) {
      errors.push(`requiredTaskNodes is missing required goal node ${taskID}`);
    }
  }
  for (const taskID of values(audit.requiredTaskNodes)) {
    const task = tasksByID.get(taskID);
    if (!task) {
      errors.push(`alignment audit required task node is missing: ${taskID}`);
      continue;
    }
    if (task.status !== "implemented" && task.status !== "preview" && !(taskID === "production-admin-oidc-auth" && task.status === "pending")) {
      errors.push(`alignment audit task ${taskID} has unsupported status ${task.status}`);
    }
    if (promotionGateTaskIDs.has(taskID)) {
      if (!task.statusReason?.zh || !task.statusReason?.en) {
        errors.push(`alignment audit task ${taskID} must declare zh/en statusReason`);
      }
      if (!task.completionGate?.zh || !task.completionGate?.en) {
        errors.push(`alignment audit task ${taskID} must declare zh/en completionGate`);
      }
    }
  }
  for (const taskID of values(audit.requiredFutureTaskNodes)) {
    const task = tasksByID.get(taskID);
    if (!task) {
      errors.push(`alignment audit future task node is missing: ${taskID}`);
      continue;
    }
    if (!["pending", "planned", "deferred", "preview", "implemented"].includes(task.status)) {
      errors.push(`alignment audit future task ${taskID} has unsupported status ${task.status}`);
    }
    if (task.status === "pending" || task.status === "planned" || task.status === "deferred" || task.status === "preview") {
      if (!task.statusReason?.zh || !task.statusReason?.en) {
        errors.push(`alignment audit future task ${taskID} must declare zh/en statusReason`);
      }
      if (!task.completionGate?.zh || !task.completionGate?.en) {
        errors.push(`alignment audit future task ${taskID} must declare zh/en completionGate`);
      }
    }
  }
  const visualRequired = values(audit.visualDesignGate?.requiredForVisualTasks);
  const visualRequiredOrder = values(audit.visualDesignGate?.requiredOrder);
  if (!sameList(visualRequiredOrder, ["superpowers:brainstorming", "product-design"])) {
    errors.push("visualDesignGate.requiredOrder must stay superpowers:brainstorming before product-design");
  }
  errors.push(...validateRequiredOrder(visualRequired, visualRequiredOrder, "visualDesignGate.requiredForVisualTasks"));
  for (const task of tasks.filter((item) => item.visual === true)) {
    const designGate = values(task.designGate);
    for (const gate of visualRequired) {
      if (!designGate.includes(gate)) {
        errors.push(`visual task ${task.id} must require ${gate}`);
      }
    }
    errors.push(...validateRequiredOrder(designGate, visualRequiredOrder, `visual task ${task.id} designGate`));
    const isContractGateOnly = task.contractGateOnly === true;
    if (audit.visualDesignGate?.visualTasksMustHaveScreenshots === true && !isContractGateOnly && (task.status === "implemented" || task.status === "preview") && values(task.evidence?.screenshots).length === 0) {
      errors.push(`visual task ${task.id} must declare screenshot evidence`);
    }
  }
  const previewTask = tasksByID.get(audit.codegenPolicy?.previewTask);
  if (!previewTask || !["implemented", "preview"].includes(previewTask.status)) {
    errors.push(`codegen preview task ${audit.codegenPolicy?.previewTask ?? "<missing>"} must be implemented or preview`);
  }
  const readinessTask = tasksByID.get(audit.codegenPolicy?.readinessTask);
  if (!readinessTask || readinessTask.status !== "implemented") {
    errors.push(`codegen readiness task ${audit.codegenPolicy?.readinessTask ?? "<missing>"} must be implemented`);
  }
  validateObjectiveTaskDependencyPaths(audit, tasksByID, errors);
}

function hasDependencyPath(tasksByID, fromTaskID, toTaskID, visited = new Set()) {
  if (fromTaskID === toTaskID) {
    return true;
  }
  if (visited.has(fromTaskID)) {
    return false;
  }
  visited.add(fromTaskID);
  const task = tasksByID.get(fromTaskID);
  if (!task) {
    return false;
  }
  for (const dependency of values(task.dependsOn)) {
    if (hasDependencyPath(tasksByID, dependency, toTaskID, visited)) {
      return true;
    }
  }
  return false;
}

function validateObjectiveTaskDependencyPaths(audit, tasksByID, errors) {
  const policy = audit.objectiveConflictPolicy ?? {};
  for (const dependencyPath of values(policy.requiredTaskDependencyPaths)) {
    const from = dependencyPath.from;
    const to = dependencyPath.to;
    if (!from || !to) {
      errors.push("objective conflict policy dependency path must declare from and to");
      continue;
    }
    if (!tasksByID.has(from)) {
      errors.push(`objective conflict policy dependency path source task is missing: ${from}`);
      continue;
    }
    if (!tasksByID.has(to)) {
      errors.push(`objective conflict policy dependency path target task is missing: ${to}`);
      continue;
    }
    if (!hasDependencyPath(tasksByID, from, to)) {
      errors.push(`objective conflict policy requires task ${from} to depend on ${to}`);
    }
  }
}

function validateEngineeringCapabilities(audit, engineering, errors) {
  const capabilities = values(engineering.capabilities);
  const capabilityIDs = new Set(capabilities.map((capability) => capability.id));
  for (const capabilityID of values(audit.nonDroppableEngineeringCapabilities)) {
    if (!values(audit.requiredEngineeringCapabilities).includes(capabilityID)) {
      errors.push(`requiredEngineeringCapabilities is missing required goal capability ${capabilityID}`);
    }
  }
  for (const capabilityID of values(audit.requiredEngineeringCapabilities)) {
    if (!capabilityIDs.has(capabilityID)) {
      errors.push(`alignment audit required engineering capability is missing: ${capabilityID}`);
    }
  }
  for (const capabilityID of values(audit.objectiveConflictPolicy?.requiredEngineeringCapabilities)) {
    if (!values(audit.requiredEngineeringCapabilities).includes(capabilityID)) {
      errors.push(`objective conflict policy capability ${capabilityID} must be listed in requiredEngineeringCapabilities`);
    }
    if (!capabilityIDs.has(capabilityID)) {
      errors.push(`objective conflict policy engineering capability is missing: ${capabilityID}`);
    }
  }
  const capabilityByID = new Map(capabilities.map((capability) => [capability.id, capability]));
  const safeCodegen = capabilityByID.get("safe-codegen-scaffold");
  if (safeCodegen?.evidence?.scaffoldPlan?.sourceWriting !== audit.codegenPolicy?.sourceWriting) {
    errors.push("safe-codegen-scaffold sourceWriting conflicts with alignment audit");
  }
  if (safeCodegen?.evidence?.scaffoldPlan?.dryRun !== true) {
    errors.push("safe-codegen-scaffold must remain dry-run");
  }
}

function validateReferenceCoverage(audit, referenceCoverage, referenceDiscovery, errors) {
  const requiredOptionalBoundaries = ["app-phone-identity", "detailed-addresses", "personnel-and-positions"];
  for (const area of requiredOptionalBoundaries) {
    if (!values(audit.requiredOptionalBoundaries).includes(area)) {
      errors.push(`requiredOptionalBoundaries is missing required optional boundary ${area}`);
    }
  }

  const foundationAreas = new Set(values(referenceCoverage.foundation).map((item) => item.area));
  const discoveryFoundationAreas = new Set(values(referenceDiscovery.candidates).filter((candidate) => candidate.classification === "foundation").map((candidate) => candidate.coverageArea));
  for (const area of values(audit.requiredReferenceFoundationAreas)) {
    if (!foundationAreas.has(area)) {
      errors.push(`reference foundation area is missing: ${area}`);
    }
    if (!discoveryFoundationAreas.has(area)) {
      errors.push(`reference discovery foundation candidate is missing for area: ${area}`);
    }
  }
  const businessAreas = new Set(values(referenceCoverage.businessBoundary).map((item) => item.area));
  const discoveryBusinessAreas = new Set(values(referenceDiscovery.candidates).filter((candidate) => candidate.classification === "business").map((candidate) => candidate.businessArea));
  for (const area of values(audit.requiredBusinessBoundaries)) {
    if (!businessAreas.has(area)) {
      errors.push(`reference business boundary is missing: ${area}`);
    }
    if (!discoveryBusinessAreas.has(area)) {
      errors.push(`reference discovery business candidate is missing for boundary: ${area}`);
    }
  }
  for (const boundary of values(referenceCoverage.businessBoundary)) {
    if (boundary.mustStayOutOfDefaultPlatform !== true) {
      errors.push(`reference business boundary ${boundary.area} must stay out of default platform`);
    }
  }
  const extensionAreas = new Set(values(referenceCoverage.extensionBoundary).map((item) => item.area));
  const discoveryExtensionAreas = new Set(values(referenceDiscovery.candidates).filter((candidate) => candidate.classification === "extension").map((candidate) => candidate.extensionArea));
  for (const area of values(audit.requiredOptionalBoundaries)) {
    if (!extensionAreas.has(area)) {
      errors.push(`reference optional extension boundary is missing: ${area}`);
    }
    if (!discoveryExtensionAreas.has(area)) {
      errors.push(`reference discovery optional candidate is missing for boundary: ${area}`);
    }
  }
  const candidatesByID = new Map(values(referenceDiscovery.candidates).map((candidate) => [candidate.id, candidate]));
  const dispatchCandidate = candidatesByID.get("business-dispatch-transfer");
  if (dispatchCandidate?.classification !== "business" || dispatchCandidate?.expectedCapability !== "external-business-capability") {
    errors.push("reference discovery business-dispatch-transfer must stay owned by external-business-capability outside platform-go");
  }
  for (const candidateID of values(referenceDiscovery.mustStayBusinessOnly)) {
    const candidate = candidatesByID.get(candidateID);
    if (candidate?.classification !== "business" || candidate?.expectedCapability !== "external-business-capability") {
      errors.push(`reference discovery ${candidateID} must stay business-only under external-business-capability outside platform-go`);
    }
  }
}

function validateAdminAPIBoundary(audit, boundary, engineering, errors) {
  if (!boundary.purpose) {
    errors.push("admin API boundary contract purpose is required");
  }
  if (boundary.reference?.promotionDecision !== "foundation-gate") {
    errors.push("admin API boundary promotionDecision must stay foundation-gate");
  }
  if (boundary.querySecurity?.rawSQLAllowed !== false) {
    errors.push("admin API boundary must forbid raw SQL");
  }
  if (boundary.querySecurity?.sensitiveFieldsAllowed !== false) {
    errors.push("admin API boundary must forbid sensitive query fields");
  }
  if (boundary.querySecurity?.fieldWhitelistSource !== "resource schema") {
    errors.push("admin API boundary fieldWhitelistSource must stay resource schema");
  }
  if (!values(boundary.requiredValidators).includes("scripts/validate-platform-admin-api-boundary.mjs")) {
    errors.push("admin API boundary requiredValidators must include scripts/validate-platform-admin-api-boundary.mjs");
  }
  if (!values(boundary.requiredTests).includes("scripts/platform-admin-api-boundary.test.mjs")) {
    errors.push("admin API boundary requiredTests must include scripts/platform-admin-api-boundary.test.mjs");
  }
  if (!values(audit.requiredEngineeringCapabilities).includes("admin-api-boundary-query-security")) {
    errors.push("alignment requiredEngineeringCapabilities must include admin-api-boundary-query-security");
  }
  const capability = values(engineering.capabilities).find((item) => item.id === "admin-api-boundary-query-security");
  if (!capability) {
    errors.push("engineering matrix must include admin-api-boundary-query-security");
  } else if (!values(capability.evidence?.sourcePaths).includes("resources/platform-admin-api-boundary.json")) {
    errors.push("admin-api-boundary-query-security must cite resources/platform-admin-api-boundary.json");
  }
}

function validateAppClientAPIBoundary(audit, boundary, engineering, errors) {
  if (!boundary.purpose) {
    errors.push("app client API boundary contract purpose is required");
  }
  if (boundary.reference?.promotionDecision !== "foundation-gate") {
    errors.push("app client API boundary promotionDecision must stay foundation-gate");
  }
  if (boundary.clientBoundary?.contractSource !== "resources/generated/app-route-contract.json") {
    errors.push("app client API boundary contractSource must stay resources/generated/app-route-contract.json");
  }
  if (boundary.clientBoundary?.generatedClientBoundary !== "future-app/src/platform/api/appClient.ts") {
    errors.push("app client API boundary generatedClientBoundary must stay future-app/src/platform/api/appClient.ts");
  }
  if (boundary.clientBoundary?.tokenPolicy?.tokenInjectionOwner !== "generated-app-client-or-app-request-port") {
    errors.push("app client API boundary tokenInjectionOwner must stay generated-app-client-or-app-request-port");
  }
  if (!values(boundary.clientBoundary?.forbiddenPageLevelPatterns).includes("Authorization")) {
    errors.push("app client API boundary must forbid page-level Authorization wiring");
  }
  if (!values(boundary.requiredValidators).includes("scripts/validate-platform-app-client-api-boundary.mjs")) {
    errors.push("app client API boundary requiredValidators must include scripts/validate-platform-app-client-api-boundary.mjs");
  }
  if (!values(boundary.requiredTests).includes("scripts/platform-app-client-api-boundary.test.mjs")) {
    errors.push("app client API boundary requiredTests must include scripts/platform-app-client-api-boundary.test.mjs");
  }
  if (!values(audit.requiredEngineeringCapabilities).includes("app-client-api-boundary")) {
    errors.push("alignment requiredEngineeringCapabilities must include app-client-api-boundary");
  }
  const capability = values(engineering.capabilities).find((item) => item.id === "app-client-api-boundary");
  if (!capability) {
    errors.push("engineering matrix must include app-client-api-boundary");
  } else if (!values(capability.evidence?.sourcePaths).includes("resources/platform-app-client-api-boundary.json")) {
    errors.push("app-client-api-boundary must cite resources/platform-app-client-api-boundary.json");
  }
}

function validateGovernanceAndProfiles(audit, governance, profilesDoc, errors) {
  const defaultProfileID = audit.defaultPlatformBoundary?.defaultProfile;
  const defaultProfile = values(profilesDoc.profiles).find((profile) => profile.id === defaultProfileID);
  if (!defaultProfile) {
    errors.push(`default profile ${defaultProfileID ?? "<missing>"} is missing`);
    return;
  }
  const capabilities = new Set(values(defaultProfile.capabilities));
  const excluded = new Set(values(defaultProfile.mustExcludeCapabilities));
  for (const capability of [...values(audit.defaultPlatformBoundary?.mustExcludeBusinessCapabilities), ...values(audit.defaultPlatformBoundary?.mustExcludeOptionalCapabilities)]) {
    if (capabilities.has(capability)) {
      errors.push(`default profile ${defaultProfileID} must not enable ${capability}`);
    }
    if (!excluded.has(capability)) {
      errors.push(`default profile ${defaultProfileID} must explicitly exclude ${capability}`);
    }
  }
  const defaultResources = new Set(values(defaultProfile.mustIncludeResources));
  for (const resource of values(audit.defaultPlatformBoundary?.requiredGovernanceResources)) {
    if (!defaultResources.has(resource)) {
      errors.push(`default profile ${defaultProfileID} must include governance resource ${resource}`);
    }
    if (!values(governance.defaultFoundation?.mustIncludeResources).includes(resource)) {
      errors.push(`governance topology must include default resource ${resource}`);
    }
  }
  if (governance.areaCodePolicy?.implicitAuthorization !== false) {
    errors.push("governance topology area codes must not imply authorization");
  }
  if (governance.roleGroupPolicy?.mode !== "classification-only") {
    errors.push("governance topology role groups must stay classification-only");
  }
  if (governance.orgUnitPolicy?.mode !== "single-tree") {
    errors.push("governance topology org units must stay single-tree");
  }
  if (!values(governance.orgUnitPolicy?.levels).includes("department")) {
    errors.push("governance topology org units must include department level");
  }
}

function validateCapabilityContracts(audit, capabilityContracts, profilesDoc, engineering, errors) {
  if (!capabilityContracts.purpose) {
    errors.push("capability contracts purpose is required");
  }
  if (capabilityContracts.policies?.defaultProfile !== profilesDoc.runtimeDefault) {
    errors.push("capability contracts defaultProfile must match capability profiles runtimeDefault");
  }
  if (capabilityContracts.policies?.defaultProfileBusinessCapabilitiesAllowed !== false) {
    errors.push("capability contracts must keep defaultProfileBusinessCapabilitiesAllowed=false");
  }
  const contractIDs = new Set(values(capabilityContracts.capabilities).map((capability) => capability.id));
  for (const capabilityID of values(profilesDoc.businessCapabilities)) {
    const contract = values(capabilityContracts.capabilities).find((item) => item.id === capabilityID);
    if (!contract) {
      errors.push(`capability contracts must classify business capability ${capabilityID}`);
      continue;
    }
    if (contract.profilePolicy !== "business-external-only") {
      errors.push(`capability contract ${capabilityID} must stay business-external-only`);
    }
  }
  const defaultProfile = values(profilesDoc.profiles).find((profile) => profile.id === profilesDoc.runtimeDefault);
  for (const capabilityID of values(defaultProfile?.capabilities)) {
    if (!contractIDs.has(capabilityID)) {
      errors.push(`capability contracts must classify default capability ${capabilityID}`);
    }
  }
  if (!values(audit.requiredEngineeringCapabilities).includes("capability-contract-governance")) {
    errors.push("alignment requiredEngineeringCapabilities must include capability-contract-governance");
  }
  const capability = values(engineering.capabilities).find((item) => item.id === "capability-contract-governance");
  if (!capability) {
    errors.push("engineering matrix must include capability-contract-governance");
  } else if (!values(capability.evidence?.sourcePaths).includes("resources/platform-capability-contracts.json")) {
    errors.push("capability-contract-governance must cite resources/platform-capability-contracts.json");
  }
}

function validateCodegenPolicy(audit, codegenReadiness, errors) {
  if (codegenReadiness.mode?.sourceWriting !== audit.codegenPolicy?.sourceWriting) {
    errors.push("codegen readiness sourceWriting conflicts with alignment audit");
  }
  const promotionReviewOverride = existingJSONPath(contractOverrides.promotionReview);
  const promotionReviewPath = promotionReviewOverride || audit.codegenPolicy?.promotionReview;
  if (!promotionReviewOverride && !relativeExistingPath(promotionReviewPath)) {
    errors.push(`codegen promotion review is missing: ${promotionReviewPath ?? "<missing>"}`);
    return;
  }
  const promotionReview = readJSON(promotionReviewOverride || path.resolve(repoRoot, promotionReviewPath));
  if (promotionReview.mode?.runtimeMutation !== audit.codegenPolicy?.runtimeMutation) {
    errors.push("codegen promotion review runtimeMutation conflicts with alignment audit");
  }
  const promotionDecision = promotionReview.decision ?? promotionReview.manualReview?.decision;
  if (promotionDecision !== audit.codegenPolicy?.promotionDecision) {
    errors.push(`codegen source-writing must not be approved; promotion must remain ${audit.codegenPolicy?.promotionDecision}`);
  }
}

function validateFormSchemaLayoutSlots(audit, formSchemaLayoutSlots, errors) {
  if (formSchemaLayoutSlots.status !== "implemented") {
    errors.push("form schema layout slots contract must be implemented");
  }
  if (formSchemaLayoutSlots.promotionState?.runtimeSlots !== "controlled") {
    errors.push("form schema layout slots runtimeSlots must be controlled");
  }
  if (formSchemaLayoutSlots.promotionState?.visualImplementation !== "implemented") {
    errors.push("form schema layout slots visualImplementation must be implemented");
  }
  if (formSchemaLayoutSlots.promotionState?.sourceWriting !== audit.codegenPolicy?.sourceWriting) {
    errors.push("form schema layout slots sourceWriting must match codegen source-writing policy");
  }
  if (!values(formSchemaLayoutSlots.requiredPromotionGates).includes("product-design")) {
    errors.push("form schema layout slots requiredPromotionGates must include product-design");
  }
  if (!values(formSchemaLayoutSlots.requiredPromotionGates).includes("superpowers:brainstorming")) {
    errors.push("form schema layout slots requiredPromotionGates must include superpowers:brainstorming");
  }
  errors.push(...validateRequiredOrder(values(formSchemaLayoutSlots.requiredPromotionGates), values(audit.visualDesignGate?.requiredOrder), "form schema layout slots requiredPromotionGates"));
  if (!values(formSchemaLayoutSlots.requiredPromotionGates).includes("validate-admin-i18n")) {
    errors.push("form schema layout slots requiredPromotionGates must include validate-admin-i18n");
  }
}

function validateFileStorageExperience(audit, fileStorageExperience, errors) {
  if (!fileStorageExperience.purpose) {
    errors.push("file storage experience contract purpose is required");
  }
  const taskID = fileStorageExperience.taskGraph?.taskId;
  if (!taskID) {
    errors.push("file storage experience taskGraph.taskId is required");
  } else if (taskID !== "file-storage-preview-and-audit-workflow") {
    errors.push("file storage experience taskGraph.taskId must stay file-storage-preview-and-audit-workflow");
  }
  if (values(audit.requiredFutureTaskNodes).includes(taskID)) {
    errors.push(`implemented file storage experience task ${taskID} must not stay in requiredFutureTaskNodes`);
  }
  if (!values(audit.requiredEngineeringCapabilities).includes("file-storage-admin-experience")) {
    errors.push("alignment requiredEngineeringCapabilities must include file-storage-admin-experience");
  }
  if (fileStorageExperience.designGate?.recommendedApproach !== "generic-resource-console-extension") {
    errors.push("file storage experience must stay a generic resource console extension");
  }
  if (fileStorageExperience.designGate?.implementationStatus !== "implemented") {
    errors.push("file storage experience implementationStatus must be implemented after product-design and browser evidence");
  }
  if (fileStorageExperience.designGate?.productDesignStatus !== "approved") {
    errors.push("file storage experience productDesignStatus must be approved");
  }
  if (fileStorageExperience.designGate?.browserQaStatus !== "passed") {
    errors.push("file storage experience browserQaStatus must be passed");
  }
  errors.push(
    ...validateRequiredOrder(
      values(fileStorageExperience.designGate?.requiredOrder),
      values(audit.visualDesignGate?.requiredOrder),
      "file storage experience designGate.requiredOrder",
    ),
  );
  for (const action of ["upload", "preview", "download", "delete"]) {
    if (!values(fileStorageExperience.experienceContract?.requiredActions).includes(action)) {
      errors.push(`file storage experience requiredActions must include ${action}`);
    }
  }
  for (const panel of ["metadata", "preview", "audit"]) {
    if (!values(fileStorageExperience.experienceContract?.requiredPanels).includes(panel)) {
      errors.push(`file storage experience requiredPanels must include ${panel}`);
    }
  }
}

function validateProductionReadiness(productionReadiness, errors) {
  const preflight = new Set(values(productionReadiness.preflightCommands).map((item) => (typeof item === "string" ? item : item.command)));
  for (const command of [
    "rtk node scripts/validate-platform-engineering-capabilities.mjs",
    "rtk node scripts/validate-admin-resources.mjs",
  ]) {
    if (!preflight.has(command)) {
      errors.push(`production readiness preflight must include ${command}`);
    }
  }
}

function validateDeploymentTopology(audit, deploymentTopology, errors) {
  const policy = audit.deploymentPolicy ?? {};
  const decision = deploymentTopology.decision ?? {};
  const deploymentPackage = deploymentTopology.deploymentPackage ?? {};
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
  if (decision.vercelRequired !== false) {
    errors.push("deployment topology decision.vercelRequired must stay false");
  }
  if (decision.vercelAdminUsage !== "optional-static-admin") {
    errors.push("deployment topology decision.vercelAdminUsage must stay optional-static-admin");
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
  if (deploymentPackage.status !== "implemented") {
    errors.push("deployment topology package status must stay implemented");
  }
  if (deploymentPackage.defaultTopology !== "single-service-production") {
    errors.push("deployment topology package defaultTopology must stay single-service-production");
  }
  if (deploymentPackage.selectedTopology !== "single-service-production") {
    errors.push("deployment topology package selectedTopology must stay single-service-production");
  }
}

function validateObjectiveConflictPolicy(audit, productionReadiness, errors) {
  const policy = audit.objectiveConflictPolicy ?? {};
  if (!policy.mode) {
    errors.push("objective conflict policy mode is required");
  }
  if (policy.mode !== "fail-fast") {
    errors.push("objective conflict policy mode must stay fail-fast");
  }
  if (!policy.purpose) {
    errors.push("objective conflict policy purpose is required");
  }
  if (policy.referenceExtractionRequiresDiscovery !== true) {
    errors.push("objective conflict policy must require reference discovery before coverage");
  }
  if (policy.visualWorkRequiresProductDesign !== true) {
    errors.push("objective conflict policy must require product-design for visual work");
  }
  if (policy.defaultProfileMustStayBusinessNeutral !== true) {
    errors.push("objective conflict policy must keep the default profile business-neutral");
  }
  for (const validator of values(policy.requiredValidators)) {
    if (!values(audit.requiredValidators).includes(validator)) {
      errors.push(`objective conflict policy validator ${validator} must be listed in requiredValidators`);
    }
  }
  const preflightIDs = new Set(values(productionReadiness.preflightCommands).map((command) => command.id));
  for (const commandID of values(policy.requiredProductionPreflightCommands)) {
    if (!preflightIDs.has(commandID)) {
      errors.push(`objective conflict policy requires production preflight command ${commandID}`);
    }
  }
}

function validateProductionAuthHardening(audit, authHardening, errors) {
  if (!authHardening.purpose) {
    errors.push("production auth hardening contract purpose is required");
  }
  const futureTasks = new Set(values(audit.requiredFutureTaskNodes));
  const taskID = authHardening.taskGraph?.taskId;
  if (!values(audit.requiredTaskNodes).includes(taskID)) {
    errors.push(`production auth hardening task ${taskID ?? "<missing>"} must be listed in requiredTaskNodes`);
  }
  if (authHardening.sessionCredentialPolicy?.refreshTokenFamily?.status !== "implemented-disabled") {
    errors.push("production auth hardening refreshTokenFamily status must stay implemented-disabled");
  }
  if (authHardening.sessionCredentialPolicy?.refreshTokenFamily?.defaultRuntime !== "disabled") {
    errors.push("production auth hardening refreshTokenFamily defaultRuntime must stay disabled");
  }
  if (!values(authHardening.providerAdapterPolicy?.requiredControls).includes("secret-rotation-plan")) {
    errors.push("production auth hardening must require provider secret rotation planning");
  }
  if (authHardening.credentialRotationPolicy?.requiredProductionReadinessPolicy !== "token-rotation") {
    errors.push("production auth hardening must map to token-rotation readiness policy");
  }
}

function validateRefreshTokenFamilyPromotion(audit, refreshTokenFamilyPromotion, authHardening, errors) {
  if (!refreshTokenFamilyPromotion.purpose) {
    errors.push("refresh token family promotion contract purpose is required");
  }
  const taskID = refreshTokenFamilyPromotion.taskGraph?.taskId;
  if (!values(audit.requiredTaskNodes).includes(taskID)) {
    errors.push(`refresh token family promotion task ${taskID ?? "<missing>"} must be listed in requiredTaskNodes`);
  }
  if (refreshTokenFamilyPromotion.currentRuntime?.status !== "sliding-renewal-only") {
    errors.push("refresh token family promotion currentRuntime.status must stay sliding-renewal-only");
  }
  if (refreshTokenFamilyPromotion.currentRuntime?.notARefreshTokenFamily !== true) {
    errors.push("refresh token family promotion currentRuntime.notARefreshTokenFamily must stay true");
  }
  if (refreshTokenFamilyPromotion.promotionState?.implementationStatus !== "implemented") {
    errors.push("refresh token family promotion implementationStatus must stay implemented");
  }
  if (refreshTokenFamilyPromotion.promotionState?.runtimeDefault !== "disabled") {
    errors.push("refresh token family promotion runtimeDefault must stay disabled");
  }
  if (refreshTokenFamilyPromotion.promotionState?.refreshTokenFamilyStatus !== authHardening.sessionCredentialPolicy?.refreshTokenFamily?.status) {
    errors.push("refresh token family promotion status must match production auth refreshTokenFamily.status");
  }
  if (refreshTokenFamilyPromotion.promotionState?.runtimeDefault !== authHardening.sessionCredentialPolicy?.refreshTokenFamily?.defaultRuntime) {
    errors.push("refresh token family promotion runtimeDefault must match production auth refreshTokenFamily.defaultRuntime");
  }
  if (authHardening.sessionCredentialPolicy?.refreshTokenFamily?.promotionReadinessContract !== "resources/platform-refresh-token-family-promotion.json") {
    errors.push("production auth hardening must reference resources/platform-refresh-token-family-promotion.json");
  }
  if (refreshTokenFamilyPromotion.dataModelContract?.rawTokenPersistenceAllowed !== false) {
    errors.push("refresh token family promotion must forbid raw token persistence");
  }
  if (refreshTokenFamilyPromotion.redisConvergencePolicy?.authoritativeSourceOfTruth !== "database") {
    errors.push("refresh token family promotion authoritative source of truth must stay database");
  }
}

function validateTaskExecutionAudit(audit, taskExecutionAudit, engineering, errors) {
  if (!taskExecutionAudit.purpose) {
    errors.push("task execution audit purpose is required");
  }
  if (taskExecutionAudit.taskGraph !== audit.contracts?.taskGraph) {
    errors.push("task execution audit taskGraph must match alignment taskGraph contract");
  }
  if (taskExecutionAudit.alignmentAudit !== "resources/platform-foundation-alignment-audit.json") {
    errors.push("task execution audit alignmentAudit must point to resources/platform-foundation-alignment-audit.json");
  }
  if (JSON.stringify(values(audit.requiredFutureTaskNodes)) !== JSON.stringify(["production-admin-oidc-auth"])) {
    errors.push("alignment requiredFutureTaskNodes must contain only production-admin-oidc-auth during Task 7 evidence collection");
  }
  if (JSON.stringify(values(taskExecutionAudit.requiredUnfinishedNodes)) !== JSON.stringify(["production-admin-oidc-auth"])) {
    errors.push("task execution audit requiredUnfinishedNodes must contain only production-admin-oidc-auth during Task 7 evidence collection");
  }
  for (const taskID of ["production-auth-provider-hardening", "source-writing-codegen-promotion", "production-admin-oidc-auth"]) {
    if (!values(taskExecutionAudit.knownPromotionBlockers).some((blocker) => blocker.taskId === taskID)) {
      errors.push(`task execution audit knownPromotionBlockers must describe ${taskID}`);
    }
  }
  for (const validator of [
    "scripts/validate-platform-task-execution-audit.mjs",
    "scripts/validate-platform-foundation-task-graph.mjs",
    "scripts/validate-platform-file-storage-experience.mjs",
    "scripts/validate-platform-refresh-token-family-promotion.mjs",
  ]) {
    if (!values(taskExecutionAudit.requiredValidators).includes(validator)) {
      errors.push(`task execution audit requiredValidators must include ${validator}`);
    }
    if (!values(audit.requiredValidators).includes(validator)) {
      errors.push(`alignment requiredValidators must include task execution validator ${validator}`);
    }
  }
  if (!values(taskExecutionAudit.requiredTests).includes("scripts/platform-task-execution-audit.test.mjs")) {
    errors.push("task execution audit requiredTests must include scripts/platform-task-execution-audit.test.mjs");
  }
  const capability = values(engineering.capabilities).find((item) => item.id === taskExecutionAudit.engineeringCapability);
  if (!capability) {
    errors.push(`engineering matrix must include task execution capability ${taskExecutionAudit.engineeringCapability}`);
  } else if (!values(capability.evidence?.validators).includes("scripts/validate-platform-task-execution-audit.mjs")) {
    errors.push(`engineering capability ${taskExecutionAudit.engineeringCapability} must cite validate-platform-task-execution-audit.mjs`);
  }
}

function validateDocuments(audit, errors) {
  for (const relativePath of values(audit.documents)) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`alignment document path is missing: ${relativePath}`);
    }
  }
  const readme = fs.readFileSync(path.resolve(repoRoot, "README.md"), "utf8");
  if (!readme.includes("validate-platform-foundation-alignment.mjs")) {
    errors.push("README.md must document validate-platform-foundation-alignment.mjs");
  }
  const agents = fs.readFileSync(path.resolve(repoRoot, "AGENTS.md"), "utf8");
  if (!agents.includes("validate-platform-foundation-alignment.mjs")) {
    errors.push("AGENTS.md must include validate-platform-foundation-alignment.mjs in verification commands");
  }
}

function validateRequiredValidators(audit, errors) {
  errors.push(...uniqueErrors(values(audit.requiredValidators), "requiredValidators"));
  for (const validator of values(audit.requiredValidators)) {
    if (!relativeExistingPath(validator)) {
      errors.push(`required validator path is missing: ${validator}`);
      continue;
    }
    const failure = runValidator(validator);
    if (failure) {
      errors.push(failure);
    }
  }
}

function validate() {
  const audit = readJSON(auditPath);
  const errors = [];
  const taskGraph = readContract(audit, "taskGraph", errors);
  const engineering = readContract(audit, "engineeringCapabilities", errors);
  const referenceDiscovery = readContract(audit, "referenceDiscovery", errors);
  const referenceCoverage = readContract(audit, "referenceCoverage", errors);
  const adminApiBoundary = readContract(audit, "adminApiBoundary", errors);
  const appClientApiBoundary = readContract(audit, "appClientApiBoundary", errors);
  const governance = readContract(audit, "governanceTopology", errors);
  const capabilityContracts = readContract(audit, "capabilityContracts", errors);
  const profiles = readContract(audit, "capabilityProfiles", errors);
  const codegenReadiness = readContract(audit, "codegenReadiness", errors);
  const formSchemaLayoutSlots = readContract(audit, "formSchemaLayoutSlots", errors);
  const fileStorageExperience = readContract(audit, "fileStorageExperience", errors);
  const taskExecutionAudit = readContract(audit, "taskExecutionAudit", errors);
  const productionReadiness = readContract(audit, "productionReadiness", errors);
  const deploymentTopology = readContract(audit, "deploymentTopology", errors);
  const productionAuthHardening = readContract(audit, "productionAuthHardening", errors);
  const refreshTokenFamilyPromotion = readContract(audit, "refreshTokenFamilyPromotion", errors);

  if (!audit.purpose) {
    errors.push("alignment audit purpose is required");
  }
  errors.push(...uniqueErrors(values(audit.requiredTaskNodes), "requiredTaskNodes"));
  errors.push(...uniqueErrors(values(audit.requiredEngineeringCapabilities), "requiredEngineeringCapabilities"));
  errors.push(...uniqueErrors(values(audit.nonDroppableEngineeringCapabilities), "nonDroppableEngineeringCapabilities"));
  validateStack(audit, taskGraph, engineering, errors);
  validateTaskGraph(audit, taskGraph, errors);
  validateEngineeringCapabilities(audit, engineering, errors);
  validateReferenceCoverage(audit, referenceCoverage, referenceDiscovery, errors);
  validateAdminAPIBoundary(audit, adminApiBoundary, engineering, errors);
  validateAppClientAPIBoundary(audit, appClientApiBoundary, engineering, errors);
  validateGovernanceAndProfiles(audit, governance, profiles, errors);
  validateCapabilityContracts(audit, capabilityContracts, profiles, engineering, errors);
  validateCodegenPolicy(audit, codegenReadiness, errors);
  validateFormSchemaLayoutSlots(audit, formSchemaLayoutSlots, errors);
  validateFileStorageExperience(audit, fileStorageExperience, errors);
  validateObjectiveConflictPolicy(audit, productionReadiness, errors);
  validateDeploymentTopology(audit, deploymentTopology, errors);
  validateTaskExecutionAudit(audit, taskExecutionAudit, engineering, errors);
  validateProductionReadiness(productionReadiness, errors);
  validateProductionAuthHardening(audit, productionAuthHardening, errors);
  validateRefreshTokenFamilyPromotion(audit, refreshTokenFamilyPromotion, productionAuthHardening, errors);
  validateDocuments(audit, errors);
  validateRequiredValidators(audit, errors);

  return { audit, errors };
}

const { audit, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated platform foundation alignment in ${path.relative(repoRoot, auditPath)} (${values(audit.requiredTaskNodes).length} task nodes, ${values(audit.requiredEngineeringCapabilities).length} engineering capabilities)`);
