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

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-file-storage-experience.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));

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

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function validateTaskGraph(contract, taskGraph, errors) {
  const taskID = contract.taskGraph?.taskId;
  const task = values(taskGraph.tasks).find((item) => item.id === taskID);
  if (!task) {
    errors.push(`task graph must include ${taskID}`);
    return;
  }
  if (task.visual !== true) {
    errors.push(`${taskID} must stay a visual workflow task`);
  }
  requireIncludes(task.designGate, ["superpowers:brainstorming", "product-design"], `${taskID}.designGate`, errors);
  const allowedStatuses = new Set(values(contract.taskGraph?.statusBeforePromotion));
  if (!allowedStatuses.has(task.status) && task.status !== contract.taskGraph?.promotionStatus) {
    errors.push(`${taskID} has unsupported status ${task.status}`);
  }
  if (task.status === contract.taskGraph?.promotionStatus && contract.designGate?.implementationStatus === "blocked") {
    errors.push(`${taskID} cannot be implemented while file-storage experience implementationStatus is blocked`);
  }
  requireIncludes(
    task.evidence?.validators,
    ["scripts/validate-platform-file-storage-experience.mjs"],
    `${taskID}.evidence.validators`,
    errors,
  );
  requireIncludes(
    task.evidence?.tests,
    ["scripts/platform-file-storage-experience.test.mjs"],
    `${taskID}.evidence.tests`,
    errors,
  );
}

function validateCurrentRuntime(contract, errors) {
  const runtime = contract.currentRuntime ?? {};
  if (runtime.status !== "implemented-backend-foundation") {
    errors.push("currentRuntime.status must stay implemented-backend-foundation");
  }
  if (runtime.capability !== "file-storage") {
    errors.push("currentRuntime.capability must stay file-storage");
  }
  if (runtime.resource !== "files") {
    errors.push("currentRuntime.resource must stay files");
  }
  requireIncludes(runtime.objectStoreAdapters, ["local", "s3"], "currentRuntime.objectStoreAdapters", errors);
  requireIncludes(
    runtime.adminEndpoints,
    ["POST /api/admin/files/upload", "GET /api/admin/files/:id/content", "DELETE /api/admin/resources/files/:id"],
    "currentRuntime.adminEndpoints",
    errors,
  );
  requireIncludes(runtime.auditActions, ["file.upload", "file.content", "file.delete"], "currentRuntime.auditActions", errors);
  for (const relativePath of [...values(runtime.implementationRoots), ...values(runtime.tests)]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`currentRuntime path is missing or unsafe: ${relativePath}`);
    }
  }

  const serverPath = "internal/platform/httpapi/server.go";
  if (relativeExistingPath(serverPath)) {
    const server = fs.readFileSync(path.resolve(repoRoot, serverPath), "utf8");
    for (const snippet of [
      "func (s *Server) adminFileUpload",
      "func (s *Server) adminFileContent",
      "func (s *Server) deleteAdminFile",
      "TombstoneFileWithAudit",
      "func (s *Server) recordFileAudit",
      'api.POST("/admin/files/upload", s.adminFileUpload)',
      'api.GET("/admin/files/:id/content", s.adminFileContent)',
      "file.upload",
      "file.content",
      "file.delete",
      'Content-Disposition", mime.FormatMediaType("inline"',
    ]) {
      if (!server.includes(snippet)) {
        errors.push(`${serverPath} must include file-storage runtime evidence ${snippet}`);
      }
    }
  }
  const lifecycleApplierPath = "internal/platform/datalifecycle/adminresource_adapter.go";
  if (relativeExistingPath(lifecycleApplierPath)) {
    const lifecycleApplier = fs.readFileSync(path.resolve(repoRoot, lifecycleApplierPath), "utf8");
    if (!lifecycleApplier.includes("PurgeTombstonedFileWithPolicyAndAudit")) {
      errors.push(`${lifecycleApplierPath} must include file-storage lifecycle purge evidence PurgeTombstonedFileWithPolicyAndAudit`);
    }
  }

  const storagePath = "internal/platform/storage/object.go";
  if (relativeExistingPath(storagePath)) {
    const storage = fs.readFileSync(path.resolve(repoRoot, storagePath), "utf8");
    for (const snippet of ["type ObjectStore interface", "type ObjectSaveInput struct", "type ObjectMetadata struct", "NewLocalObjectStore", "NewS3ObjectStore"]) {
      if (!storage.includes(snippet)) {
        errors.push(`${storagePath} must include object-store evidence ${snippet}`);
      }
    }
    for (const frozen of values(runtime.mustNotChangeForUiPromotion)) {
      const typeName = frozen.split(".").pop();
      const hasQualifiedReference = storage.includes(frozen);
      const hasTypeDeclaration = typeName ? storage.includes(`type ${typeName}`) : false;
      if (!hasQualifiedReference && !hasTypeDeclaration) {
        errors.push(`${storagePath} must retain UI-promotion boundary ${frozen}`);
      }
    }
  }

  const capabilitiesPath = "internal/platform/core/capabilities.go";
  if (relativeExistingPath(capabilitiesPath)) {
    const capabilities = fs.readFileSync(path.resolve(repoRoot, capabilitiesPath), "utf8");
    for (const snippet of ["func fileStorageAdminResource", 'Resource:         "files"', 'PermissionPrefix: "admin:file"', 'Route:  "/files"']) {
      if (!capabilities.includes(snippet)) {
        errors.push(`${capabilitiesPath} must include file resource manifest evidence ${snippet}`);
      }
    }
  }
}

function validateDesignGate(contract, errors) {
  const gate = contract.designGate ?? {};
  requireIncludes(gate.requiredOrder, ["superpowers:brainstorming", "product-design"], "designGate.requiredOrder", errors);
  if (values(gate.requiredOrder).join(" > ") !== "superpowers:brainstorming > product-design") {
    errors.push("designGate.requiredOrder must keep superpowers:brainstorming before product-design");
  }
  if (gate.recommendedApproach !== "generic-resource-console-extension") {
    errors.push("designGate.recommendedApproach must stay generic-resource-console-extension");
  }
  requireIncludes(gate.rejectedApproaches, ["standalone-file-manager-page", "storage-operations-console"], "designGate.rejectedApproaches", errors);
  if (gate.productDesignStatus !== "approved") {
    errors.push("designGate.productDesignStatus must be approved for implemented file-storage UI promotion");
  }
  if (gate.browserQaStatus !== "passed") {
    errors.push("designGate.browserQaStatus must be passed for implemented file-storage UI promotion");
  }
  if (gate.implementationStatus !== "implemented") {
    errors.push("designGate.implementationStatus must be implemented after visual evidence is complete");
  }
  if (!gate.approvedDesignBrief?.zh || !gate.approvedDesignBrief?.en) {
    errors.push("designGate.approvedDesignBrief must include zh and en summaries");
  }
  validateBrowserEvidence(gate.browserEvidence, errors);
}

function validateExperienceContract(contract, errors) {
  const experience = contract.experienceContract ?? {};
  if (experience.surface !== "files generic resource route") {
    errors.push("experienceContract.surface must stay files generic resource route");
  }
  requireIncludes(
    experience.platformComponents,
    ["AdminPage", "AdminListPanel", "PlatformDataTable", "AdminActionButton", "AdminFeedback", "PlatformOverflowText", "Drawer", "Tabs"],
    "experienceContract.platformComponents",
    errors,
  );
  requireIncludes(experience.requiredPanels, ["metadata", "preview", "audit"], "experienceContract.requiredPanels", errors);
  requireIncludes(experience.requiredActions, ["upload", "preview", "download", "delete"], "experienceContract.requiredActions", errors);
  requireIncludes(experience.previewTypes, ["image", "text", "pdf", "unsupported-fallback"], "experienceContract.previewTypes", errors);
  requireIncludes(
    experience.metadataFields,
    ["name", "mimeType", "size", "storageDriver", "createdAt", "updatedAt"],
    "experienceContract.metadataFields",
    errors,
  );
  if (values(experience.metadataFields).includes("storageKey")) {
    errors.push("experienceContract.metadataFields must not include storageKey");
  }
  requireIncludes(experience.auditVisualization, ["file.upload", "file.content", "file.delete"], "experienceContract.auditVisualization", errors);
  requireIncludes(
    experience.errorStates,
    ["object-not-found", "preview-not-supported", "download-failed", "delete-failed", "permission-denied"],
    "experienceContract.errorStates",
    errors,
  );
  requireIncludes(
    experience.responsiveRequirements,
    ["desktop table with drawer preview", "mobile file cards before pagination", "theme-synced drawer and preview surface", "compact operation bar aligned with PlatformDataTable"],
    "experienceContract.responsiveRequirements",
    errors,
  );
  if (!experience.auditFallback?.zh || !experience.auditFallback?.en) {
    errors.push("experienceContract.auditFallback must document audit-log fallback in zh and en");
  }
}

function validatePromotionEvidence(contract, errors) {
  const evidence = contract.promotionEvidenceRequired ?? {};
  for (const relativePath of [...values(evidence.docs), ...values(evidence.validators).filter((item) => item.endsWith(".mjs"))]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`promotion evidence path is missing or unsafe: ${relativePath}`);
    }
  }
  requireIncludes(evidence.validators, ["scripts/validate-admin-i18n.mjs", "scripts/validate-admin-ui-contracts.mjs"], "promotionEvidenceRequired.validators", errors);
  requireIncludes(evidence.tests, ["scripts/platform-file-storage-experience.test.mjs", "rtk npm --prefix admin run build"], "promotionEvidenceRequired.tests", errors);
  requireIncludes(
    evidence.screenshots,
    ["desktop files list with preview drawer", "desktop files audit panel", "mobile files card and preview flow"],
    "promotionEvidenceRequired.screenshots",
    errors,
  );
}

function validateBrowserEvidence(evidence, errors) {
  if (!evidence || evidence.tool !== "chrome-devtools") {
    errors.push("designGate.browserEvidence.tool must be chrome-devtools");
    return;
  }
  if (!evidence.capturedAt) {
    errors.push("designGate.browserEvidence.capturedAt is required");
  }
  const screenshots = values(evidence.screenshots);
  const labels = screenshots.map((item) => item.label);
  requireIncludes(
    labels,
    ["desktop files list with preview drawer", "desktop files audit panel", "mobile files card and pagination", "mobile files preview drawer"],
    "designGate.browserEvidence.screenshots.label",
    errors,
  );
  for (const screenshot of screenshots) {
    if (!screenshot.path || (!relativeExistingPath(screenshot.path) && !isExternalReviewArtifactURI(screenshot.path))) {
      errors.push(`designGate.browserEvidence screenshot path is missing or unsafe: ${screenshot.path}`);
    }
    if (!screenshot.viewport) {
      errors.push(`designGate.browserEvidence screenshot ${screenshot.label} must include viewport`);
    }
    if (values(screenshot.assertions).length === 0) {
      errors.push(`designGate.browserEvidence screenshot ${screenshot.label} must include assertions`);
    }
  }
  requireIncludes(
    evidence.runtimeChecks,
    [
      "Demo login opened /files and uploaded platform-preview.txt through the hidden file input path.",
      "Current navigation console errors and warnings: none.",
      "Current navigation fetch/xhr requests returned 200, including /api/admin/files/:id/content.",
      "Audit-log queries are skipped when /audit-logs is not exposed in the current resource list.",
    ],
    "designGate.browserEvidence.runtimeChecks",
    errors,
  );
}

function validate() {
  const contract = readJSON(contractPath);
  const taskGraph = readJSON(taskGraphPath);
  const errors = [];

  if (!contract.purpose) {
    errors.push("file-storage experience purpose is required");
  }
  validateTaskGraph(contract, taskGraph, errors);
  validateCurrentRuntime(contract, errors);
  validateDesignGate(contract, errors);
  validateExperienceContract(contract, errors);
  validatePromotionEvidence(contract, errors);

  return { contract, errors };
}

const { contract, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated file-storage experience gate in ${path.relative(repoRoot, contractPath)}`);
