import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const completedProgramTaskIDs = [
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
];

const remainingCompletionProgramTaskIDs = [
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
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

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-foundation-alignment.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-foundation-alignment-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-foundation-alignment", () => {
  it("migrates ten completed program nodes to required work and tracks 19 future nodes", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const engineering = readJSON("resources/platform-engineering-capabilities.json");

    assert.ok(audit.requiredTaskNodes.includes("production-admin-oidc-auth"));
    for (const taskID of completedProgramTaskIDs) {
      assert.ok(audit.requiredTaskNodes.includes(taskID), `${taskID} must be required work`);
    }
    assert.deepEqual(audit.requiredFutureTaskNodes, remainingCompletionProgramTaskIDs);
    for (const taskID of [...completedProgramTaskIDs, ...remainingCompletionProgramTaskIDs]) {
      assert.ok(audit.nonDroppableGoalNodes.includes(taskID), `${taskID} must be non-droppable`);
    }
    assert.equal(audit.requiredEngineeringCapabilities.length, engineering.capabilities.length);
    for (const capability of engineering.capabilities) {
      assert.ok(audit.requiredEngineeringCapabilities.includes(capability.id), `${capability.id} must be required`);
    }
    for (const capability of engineering.capabilities.filter((item) => item.status === "partial")) {
      assert.ok(audit.nonDroppableEngineeringCapabilities.includes(capability.id), `${capability.id} must be non-droppable`);
    }
  });

  it("rejects omitting the implemented runtime security node from required task tracking", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredTaskNodes = audit.requiredTaskNodes.filter((taskID) => taskID !== completedProgramTaskIDs[0]);

    const result = runValidator(["--audit", tempJSON("missing-runtime-security-required-task.json", audit)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredTaskNodes is missing required goal node runtime-security-containment/);
  });

  it("rejects missing required tracking or stale future tracking for implemented watermark governance", () => {
    const missingRequiredAudit = readJSON("resources/platform-foundation-alignment-audit.json");
    missingRequiredAudit.requiredTaskNodes = missingRequiredAudit.requiredTaskNodes.filter(
      (taskID) => taskID !== "admin-watermark-export-governance",
    );
    const missingRequiredResult = runValidator([
      "--audit",
      tempJSON("missing-required-watermark-governance.json", missingRequiredAudit),
    ]);

    assert.notEqual(missingRequiredResult.status, 0, missingRequiredResult.stdout);
    assert.match(missingRequiredResult.stderr, /requiredTaskNodes is missing required goal node admin-watermark-export-governance/);

    const staleFutureAudit = readJSON("resources/platform-foundation-alignment-audit.json");
    staleFutureAudit.requiredFutureTaskNodes = ["admin-watermark-export-governance", ...remainingCompletionProgramTaskIDs];
    const staleFutureResult = runValidator([
      "--audit",
      tempJSON("future-watermark-governance.json", staleFutureAudit),
    ]);

    assert.notEqual(staleFutureResult.status, 0, staleFutureResult.stdout);
    assert.match(staleFutureResult.stderr, /alignment requiredFutureTaskNodes must exactly match unfinished task graph nodes in graph order/);
  });

  it("rejects regressing production Admin OIDC to pending after Task 8 closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "production-admin-oidc-auth");
    task.status = "pending";
    task.statusReason = { zh: "测试待完成。", en: "Test pending." };
    task.completionGate = { zh: "完成测试。", en: "Complete the test." };
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /alignment audit task production-admin-oidc-auth must be implemented after Task 8 closeout/);
  });

  it("accepts the current platform foundation alignment audit", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform foundation alignment/);
  });

  it("rejects stack drift away from the approved route", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.approvedStack.backend = ["Gin", "GORM", "JWT"];
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /approvedStack\.backend must stay Gin \+ GORM \+ Casbin \+ JWT/);
  });

  it("rejects dropping completed foundation promotion-gate task nodes from the task graph", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks = graph.tasks.filter((task) => task.id !== "production-auth-provider-hardening");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--audit", auditPath, "--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /alignment audit required task node is missing: production-auth-provider-hardening/);
  });

  it("rejects completed promotion-gate task nodes without promotion gate rationale", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const futureTask = graph.tasks.find((task) => task.id === "production-auth-provider-hardening");
    delete futureTask.statusReason;
    delete futureTask.completionGate;
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--audit", auditPath, "--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /alignment audit task production-auth-provider-hardening must declare zh\/en statusReason/);
    assert.match(result.stderr, /alignment audit task production-auth-provider-hardening must declare zh\/en completionGate/);
  });

  it("rejects visual gate order drift in the alignment audit", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.visualDesignGate.requiredOrder = ["product-design", "superpowers:brainstorming"];
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visualDesignGate\.requiredOrder must stay superpowers:brainstorming before product-design/);
  });

  it("rejects visual tasks that run product-design before brainstorming", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.visual === true);
    visualTask.designGate = ["product-design", "superpowers:brainstorming"];
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--audit", auditPath, "--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visual task .* designGate must order superpowers:brainstorming before product-design/);
  });

  it("rejects production auth hardening contracts detached from required task tracking", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredTaskNodes = audit.requiredTaskNodes.filter((task) => task !== "production-auth-provider-hardening");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production auth hardening task production-auth-provider-hardening must be listed in requiredTaskNodes/);
  });

  it("rejects refresh-token family promotion contracts detached from production auth policy", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const promotion = readJSON("resources/platform-refresh-token-family-promotion.json");
    promotion.taskGraph.taskId = "detached-refresh-token-family-task";
    promotion.currentRuntime.status = "refresh-token-family-enabled";
    promotion.currentRuntime.notARefreshTokenFamily = false;
    promotion.promotionState.implementationStatus = "ready";
    promotion.promotionState.runtimeDefault = "enabled";
    promotion.dataModelContract.rawTokenPersistenceAllowed = true;
    promotion.redisConvergencePolicy.authoritativeSourceOfTruth = "redis";
    const productionAuth = readJSON("resources/platform-production-auth-hardening.json");
    delete productionAuth.sessionCredentialPolicy.refreshTokenFamily.promotionReadinessContract;
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const promotionPath = tempJSON("platform-refresh-token-family-promotion.json", promotion);
    const productionAuthPath = tempJSON("platform-production-auth-hardening.json", productionAuth);

    const result = runValidator(["--audit", auditPath, "--refresh-token-family-promotion", promotionPath, "--production-auth-hardening", productionAuthPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /refresh token family promotion task detached-refresh-token-family-task must be listed in requiredTaskNodes/);
    assert.match(result.stderr, /refresh token family promotion currentRuntime.status must stay sliding-renewal-only/);
    assert.match(result.stderr, /refresh token family promotion currentRuntime.notARefreshTokenFamily must stay true/);
    assert.match(result.stderr, /refresh token family promotion implementationStatus must stay implemented/);
    assert.match(result.stderr, /refresh token family promotion runtimeDefault must stay disabled/);
    assert.match(result.stderr, /production auth hardening must reference resources\/platform-refresh-token-family-promotion\.json/);
    assert.match(result.stderr, /refresh token family promotion must forbid raw token persistence/);
    assert.match(result.stderr, /refresh token family promotion authoritative source of truth must stay database/);
  });

  it("rejects source-writing codegen approval", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const promotionReview = readJSON("resources/generated/admin-scaffold-promotion-review.json");
    promotionReview.decision = "approved";
    const promotionReviewPath = tempJSON("admin-scaffold-promotion-review.json", promotionReview);
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath, "--promotion-review", promotionReviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /codegen source-writing must not be approved/);
  });

  it("rejects form schema layout slots that bypass product-design or source-writing policy", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const formSlots = readJSON("resources/platform-form-schema-layout-slots.json");
    formSlots.status = "contract-gate";
    formSlots.promotionState.runtimeSlots = "deferred";
    formSlots.promotionState.visualImplementation = "requires-product-design";
    formSlots.promotionState.sourceWriting = "enabled";
    formSlots.requiredPromotionGates = formSlots.requiredPromotionGates.filter((gate) => gate !== "superpowers:brainstorming" && gate !== "product-design" && gate !== "validate-admin-i18n");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const formSlotsPath = tempJSON("platform-form-schema-layout-slots.json", formSlots);

    const result = runValidator(["--audit", auditPath, "--form-schema-layout-slots", formSlotsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /form schema layout slots contract must be implemented/);
    assert.match(result.stderr, /form schema layout slots runtimeSlots must be controlled/);
    assert.match(result.stderr, /form schema layout slots visualImplementation must be implemented/);
    assert.match(result.stderr, /form schema layout slots sourceWriting must match codegen source-writing policy/);
    assert.match(result.stderr, /form schema layout slots requiredPromotionGates must include superpowers:brainstorming/);
    assert.match(result.stderr, /form schema layout slots requiredPromotionGates must include product-design/);
    assert.match(result.stderr, /form schema layout slots requiredPromotionGates must include validate-admin-i18n/);
  });

  it("rejects file storage experience contracts that bypass the generic resource extension gate", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const fileStorageExperience = readJSON("resources/platform-file-storage-experience.json");
    fileStorageExperience.taskGraph.taskId = "detached-file-storage-task";
    fileStorageExperience.designGate.requiredOrder = ["product-design", "superpowers:brainstorming"];
    fileStorageExperience.designGate.recommendedApproach = "standalone-file-manager-page";
    fileStorageExperience.designGate.implementationStatus = "blocked";
    fileStorageExperience.designGate.productDesignStatus = "pending";
    fileStorageExperience.designGate.browserQaStatus = "pending";
    fileStorageExperience.experienceContract.requiredActions = ["upload"];
    fileStorageExperience.experienceContract.requiredPanels = ["metadata"];
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const fileStorageExperiencePath = tempJSON("platform-file-storage-experience.json", fileStorageExperience);

    const result = runValidator(["--audit", auditPath, "--file-storage-experience", fileStorageExperiencePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /file storage experience taskGraph\.taskId must stay file-storage-preview-and-audit-workflow/);
    assert.match(result.stderr, /file storage experience must stay a generic resource console extension/);
    assert.match(result.stderr, /file storage experience implementationStatus must be implemented/);
    assert.match(result.stderr, /file storage experience productDesignStatus must be approved/);
    assert.match(result.stderr, /file storage experience browserQaStatus must be passed/);
    assert.match(result.stderr, /file storage experience designGate\.requiredOrder must order superpowers:brainstorming before product-design/);
    assert.match(result.stderr, /file storage experience requiredActions must include preview/);
    assert.match(result.stderr, /file storage experience requiredPanels must include audit/);
  });

  it("rejects default profile leakage of business capabilities", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const profiles = readJSON("resources/platform-capability-profiles.json");
    const defaultProfile = profiles.profiles.find((profile) => profile.id === "platform-default");
    defaultProfile.capabilities.push("external-business-capability");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath, "--capability-profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /default profile platform-default must not enable external-business-capability/);
  });

  it("rejects dropping optional app phone or detailed address boundaries from the alignment audit", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredOptionalBoundaries = audit.requiredOptionalBoundaries.filter((boundary) => boundary !== "app-phone-identity");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredOptionalBoundaries is missing required optional boundary app-phone-identity/);
  });

  it("rejects reference discovery that does not explain required boundaries", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const discovery = readJSON("resources/platform-reference-discovery.json");
    discovery.candidates = discovery.candidates.filter((candidate) => candidate.id !== "business-dispatch-transfer");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

    const result = runValidator(["--audit", auditPath, "--reference-discovery", discoveryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference discovery business candidate is missing for boundary: business-dispatch/);
    assert.match(result.stderr, /reference discovery business-dispatch-transfer must stay owned by external-business-capability outside platform-go/);
  });

  it("rejects removing the reference discovery gate from required engineering capabilities", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredEngineeringCapabilities = audit.requiredEngineeringCapabilities.filter((capability) => capability !== "reference-discovery-gate");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEngineeringCapabilities is missing required goal capability reference-discovery-gate/);
  });

  it("rejects removing the admin API boundary gate from alignment coverage", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredEngineeringCapabilities = audit.requiredEngineeringCapabilities.filter((capability) => capability !== "admin-api-boundary-query-security");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-admin-api-boundary.mjs");
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.querySecurity.rawSQLAllowed = true;
    boundary.querySecurity.sensitivityPolicySource = "field-name-list";
    boundary.querySecurity.fieldNameInferenceAllowed = true;
    boundary.querySecurity.encryptedFieldQueryPolicy = "all-operators";
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--audit", auditPath, "--admin-api-boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEngineeringCapabilities is missing required goal capability admin-api-boundary-query-security/);
    assert.match(result.stderr, /admin API boundary must forbid raw SQL/);
    assert.match(result.stderr, /admin API boundary sensitivityPolicySource must stay capability manifest/);
    assert.match(result.stderr, /admin API boundary must forbid field-name sensitivity inference/);
    assert.match(result.stderr, /admin API boundary encrypted field query policy must stay declared-blind-index-exact-match-only/);
    assert.match(result.stderr, /admin API boundary requiredValidators must include scripts\/validate-platform-admin-api-boundary\.mjs|objective conflict policy validator scripts\/validate-platform-admin-api-boundary\.mjs must be listed in requiredValidators/);
  });

  it("rejects dropping file operation audit from the engineering objective lock", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredEngineeringCapabilities = audit.requiredEngineeringCapabilities.filter((capability) => capability !== "file-operation-audit-contract");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEngineeringCapabilities is missing required goal capability file-operation-audit-contract/);
  });

  it("rejects dropping deployment topology from alignment coverage", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredEngineeringCapabilities = audit.requiredEngineeringCapabilities.filter((capability) => capability !== "deployment-topology-gate");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-deployment-topology.mjs");
    audit.objectiveConflictPolicy.requiredEngineeringCapabilities = audit.objectiveConflictPolicy.requiredEngineeringCapabilities.filter(
      (capability) => capability !== "deployment-topology-gate",
    );
    audit.objectiveConflictPolicy.requiredValidators = audit.objectiveConflictPolicy.requiredValidators.filter(
      (validator) => validator !== "scripts/validate-platform-deployment-topology.mjs",
    );
    audit.objectiveConflictPolicy.requiredProductionPreflightCommands = audit.objectiveConflictPolicy.requiredProductionPreflightCommands.filter(
      (command) => command !== "deployment-topology",
    );
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "deployment-topology");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--audit", auditPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEngineeringCapabilities is missing required goal capability deployment-topology-gate/);
  });

  it("rejects deployment topology drift away from selected scheme A", () => {
    const deploymentTopology = readJSON("resources/platform-deployment-topology.json");
    deploymentTopology.decision.selectedTopology = "split-admin-vercel-api-service";
    deploymentTopology.decision.defaultApiRuntime = "vercel-go-runtime";
    deploymentTopology.deploymentPackage.selectedTopology = "split-admin-vercel-api-service";
    const deploymentTopologyPath = tempJSON("platform-deployment-topology.json", deploymentTopology);

    const result = runValidator(["--deployment-topology", deploymentTopologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deployment topology decision\.selectedTopology must stay single-service-production/);
    assert.match(result.stderr, /deployment topology decision\.defaultApiRuntime must stay long-lived-service/);
    assert.match(result.stderr, /deployment topology package selectedTopology must stay single-service-production/);
  });

  it("rejects dropping capability contracts from alignment coverage", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredEngineeringCapabilities = audit.requiredEngineeringCapabilities.filter((capability) => capability !== "capability-contract-governance");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-capability-contracts.mjs");
    audit.objectiveConflictPolicy.requiredEngineeringCapabilities = audit.objectiveConflictPolicy.requiredEngineeringCapabilities.filter(
      (capability) => capability !== "capability-contract-governance",
    );
    audit.objectiveConflictPolicy.requiredValidators = audit.objectiveConflictPolicy.requiredValidators.filter(
      (validator) => validator !== "scripts/validate-platform-capability-contracts.mjs",
    );
    audit.objectiveConflictPolicy.requiredProductionPreflightCommands = audit.objectiveConflictPolicy.requiredProductionPreflightCommands.filter(
      (command) => command !== "capability-contracts",
    );
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "capability-contracts");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--audit", auditPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEngineeringCapabilities is missing required goal capability capability-contract-governance/);
  });

  it("rejects reference coverage that drops non-resource extension boundaries", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.extensionBoundary = coverage.extensionBoundary.filter((boundary) => boundary.area !== "detailed-addresses");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const coveragePath = tempJSON("platform-reference-coverage.json", coverage);

    const result = runValidator(["--audit", auditPath, "--reference-coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference optional extension boundary is missing: detailed-addresses/);
  });

  it("rejects objective conflict policies with missing dependency paths or disabled gates", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.objectiveConflictPolicy.mode = "warn-only";
    audit.objectiveConflictPolicy.referenceExtractionRequiresDiscovery = false;
    audit.objectiveConflictPolicy.requiredValidators = audit.objectiveConflictPolicy.requiredValidators.filter(
      (validator) => validator !== "scripts/validate-admin-i18n.mjs",
    );
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const alignmentTask = graph.tasks.find((task) => task.id === "foundation-alignment-audit");
    alignmentTask.dependsOn = alignmentTask.dependsOn.filter((task) => task !== "reference-coverage-boundary-gate");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--audit", auditPath, "--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conflict policy mode must stay fail-fast/);
    assert.match(result.stderr, /objective conflict policy must require reference discovery before coverage/);
    assert.match(result.stderr, /objective conflict policy requires task foundation-alignment-audit to depend on reference-coverage-boundary-gate/);
  });

  it("rejects missing task execution audit integration", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-task-execution-audit.mjs");
    const taskExecution = readJSON("resources/platform-task-execution-audit.json");
    taskExecution.requiredUnfinishedNodes = ["source-writing-codegen-promotion"];
    const engineering = readJSON("resources/platform-engineering-capabilities.json");
    const capability = engineering.capabilities.find((item) => item.id === "task-dependency-governance");
    capability.evidence.validators = capability.evidence.validators.filter((validator) => validator !== "scripts/validate-platform-task-execution-audit.mjs");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const taskExecutionPath = tempJSON("platform-task-execution-audit.json", taskExecution);
    const engineeringPath = tempJSON("platform-engineering-capabilities.json", engineering);

    const result = runValidator(["--audit", auditPath, "--task-execution-audit", taskExecutionPath, "--engineering-capabilities", engineeringPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task execution audit requiredUnfinishedNodes/);
    assert.match(result.stderr, /alignment requiredValidators must include task execution validator scripts\/validate-platform-task-execution-audit\.mjs/);
    assert.match(result.stderr, /engineering capability task-dependency-governance must cite validate-platform-task-execution-audit\.mjs/);
  });

  it("rejects missing file storage experience validator integration", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-file-storage-experience.mjs");
    const taskExecution = readJSON("resources/platform-task-execution-audit.json");
    taskExecution.requiredValidators = taskExecution.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-file-storage-experience.mjs");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const taskExecutionPath = tempJSON("platform-task-execution-audit.json", taskExecution);

    const result = runValidator(["--audit", auditPath, "--task-execution-audit", taskExecutionPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task execution audit requiredValidators must include scripts\/validate-platform-file-storage-experience\.mjs/);
    assert.match(result.stderr, /alignment requiredValidators must include task execution validator scripts\/validate-platform-file-storage-experience\.mjs/);
  });

  it("rejects missing refresh-token family promotion validator integration", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    audit.requiredValidators = audit.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-refresh-token-family-promotion.mjs");
    const taskExecution = readJSON("resources/platform-task-execution-audit.json");
    taskExecution.requiredValidators = taskExecution.requiredValidators.filter((validator) => validator !== "scripts/validate-platform-refresh-token-family-promotion.mjs");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const taskExecutionPath = tempJSON("platform-task-execution-audit.json", taskExecution);

    const result = runValidator(["--audit", auditPath, "--task-execution-audit", taskExecutionPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task execution audit requiredValidators must include scripts\/validate-platform-refresh-token-family-promotion\.mjs/);
    assert.match(result.stderr, /alignment requiredValidators must include task execution validator scripts\/validate-platform-refresh-token-family-promotion\.mjs/);
  });

  it("rejects production preflight that omits objective conflict policy commands", () => {
    const audit = readJSON("resources/platform-foundation-alignment-audit.json");
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "admin-build" && command.id !== "admin-ui-contract-tests");
    const auditPath = tempJSON("platform-foundation-alignment-audit.json", audit);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--audit", auditPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /objective conflict policy requires production preflight command admin-ui-contract-tests/);
    assert.match(result.stderr, /objective conflict policy requires production preflight command admin-build/);
  });
});
