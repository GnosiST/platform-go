import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-organization-rbac-menu-contract.json"));
const topologyPath = path.resolve(repoRoot, argValue("--topology", "resources/platform-governance-topology.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-engineering-capabilities.json"));
const serviceRuntimePath = path.resolve(repoRoot, argValue("--service-runtime", "resources/platform-service-object-runtime.json"));

const taskId = "organization-rbac-menu-contract-and-migration-design";
const downstreamTaskIds = [
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function readText(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(actual, expected) {
  return actual.length === expected.length && actual.every((item, index) => item === expected[index]);
}

function requireEqual(actual, expected, label, errors) {
  if (actual !== expected) errors.push(`${label} must be ${expected}`);
}

function requireIncludes(actual, expected, label, errors) {
  const set = new Set(values(actual));
  for (const item of expected) {
    if (!set.has(item)) errors.push(`${label} must include ${item}`);
  }
}

function requireExisting(relativePath, label, errors) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    errors.push(`${label} must be a repository-relative path`);
    return;
  }
  const absolute = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolute);
  if (!relative || relative.startsWith("..") || !fs.existsSync(absolute)) {
    errors.push(`${label} must exist: ${relativePath}`);
  }
}

function tableByName(contract, logicalName) {
  return values(contract.relationalContract?.tables).find((table) => table.logicalName === logicalName);
}

function serviceObjectKeys(items) {
  return values(items).map((item) => `${item.id}@${item.version}`);
}

function validateContract(contract, serviceRuntime, errors) {
  requireEqual(contract.status, "design-frozen-runtime-pending", "status", errors);
  requireEqual(contract.taskId, taskId, "taskId", errors);
  requireIncludes(contract.designEvidence, ["superpowers:brainstorming", "product-design", "ui-ux-pro-max"], "designEvidence", errors);

  const target = contract.targetModel ?? {};
  requireEqual(target.organizationResource, "org-units", "targetModel.organizationResource", errors);
  requireEqual(target.organizationBindingInheritance, "none-direct-bindings-only", "targetModel.organizationBindingInheritance", errors);
  requireEqual(target.roleGroupHierarchy, "non-nested-role-group-to-role", "targetModel.roleGroupHierarchy", errors);
  requireEqual(target.roleOwnership, "exactly-one-role-group", "targetModel.roleOwnership", errors);
  if (!sameList(values(target.roleGroupScopes), ["platform", "tenant"])) {
    errors.push("targetModel.roleGroupScopes must be platform then tenant");
  }
  requireEqual(target.organizationRolePool, "distinct-enabled-roles-in-enabled-directly-bound-enabled-tenant-groups", "targetModel.organizationRolePool", errors);
  requireEqual(target.userRoleConstraint, "subset-of-effective-role-pool", "targetModel.userRoleConstraint", errors);
  requireEqual(target.roleMenuBinding, "page-leaves-only-with-derived-directory-ancestors", "targetModel.roleMenuBinding", errors);
  requireEqual(target.apiSecurityBoundary, "casbin-protected-api-permission", "targetModel.apiSecurityBoundary", errors);
  requireEqual(target.federationAndXa, "forbidden-for-authorization", "targetModel.federationAndXa", errors);

  requireEqual(contract.identityContract?.tenantUser?.tenantCode, "server-derived-redundantly-persisted-read-only", "identityContract.tenantUser.tenantCode", errors);
  requireEqual(contract.identityContract?.platformUser?.orgUnitCode, "must-be-empty", "identityContract.platformUser.orgUnitCode", errors);
  requireEqual(contract.identityContract?.platformUser?.roleSource, "enabled-platform-role-groups", "identityContract.platformUser.roleSource", errors);
  requireIncludes(contract.identityContract?.allWritePaths, ["create", "update", "bulk-assign", "import", "service-command", "delete", "restore", "purge"], "identityContract.allWritePaths", errors);

  const requiredTables = ["role_groups", "roles", "org_unit_role_groups", "users", "user_roles", "menus", "role_menu", "page_buttons", "permissions"];
  for (const logicalName of requiredTables) {
    if (!tableByName(contract, logicalName)) errors.push(`relationalContract.tables must include ${logicalName}`);
  }
  requireEqual(contract.relationalContract?.codeIdentityPolicy, "globally-unique-stable-codes-retained-for-compatibility", "relationalContract.codeIdentityPolicy", errors);
  requireIncludes(tableByName(contract, "org_unit_role_groups")?.invariants, [
    "organization tenant equals role group tenant",
    "platform role groups cannot be bound",
  ], "org_unit_role_groups invariants", errors);
  requireIncludes(tableByName(contract, "role_menu")?.invariants, [
    "menu_code references an enabled page node",
    "directory visibility is derived from assigned descendant pages",
  ], "role_menu invariants", errors);
  requireEqual(contract.repositoryBoundary?.snapshotDeleteAndRecreate, "forbidden-for-new-authorization-relations", "repositoryBoundary.snapshotDeleteAndRecreate", errors);

  const lifecycle = contract.lifecycleAuthorizationContract ?? {};
  requireEqual(lifecycle.activeEligibility, "not-logically-deleted-and-enabled", "lifecycleAuthorizationContract.activeEligibility", errors);
  requireIncludes(lifecycle.lifecycleManagedEntities, ["org-units", "role-groups", "roles", "users", "menus", "permissions"], "lifecycleAuthorizationContract.lifecycleManagedEntities", errors);
  requireIncludes(lifecycle.dependentAuthorizationRelations, ["org_unit_role_groups", "user_roles", "role_menu", "page_buttons"], "lifecycleAuthorizationContract.dependentAuthorizationRelations", errors);
  requireEqual(lifecycle.entityDelete, "impact-preview-and-explicit-remediation-before-logical-delete", "lifecycleAuthorizationContract.entityDelete", errors);
  requireEqual(lifecycle.entityRestore, "restore-disabled-then-revalidate-and-explicitly-enable", "lifecycleAuthorizationContract.entityRestore", errors);
  requireEqual(lifecycle.relationMutation, "explicit-server-side-atomic-diff-only-no-independent-restore", "lifecycleAuthorizationContract.relationMutation", errors);
  requireEqual(lifecycle.relationReferencePolicy, "participates-in-impact-preview-purge-blocking-and-audit", "lifecycleAuthorizationContract.relationReferencePolicy", errors);
  requireEqual(lifecycle.scheduledRunnerBoundary, "must-use-the-same-domain-validator-and-native-transactional-repository", "lifecycleAuthorizationContract.scheduledRunnerBoundary", errors);

  const queryKeys = serviceObjectKeys(contract.serviceObjectContract?.queries);
  const commandKeys = serviceObjectKeys(contract.serviceObjectContract?.commands);
  const expectedQueryKeys = [
    "platform.identity.organization-role-pool.get@1.0.0",
    "platform.identity.organization-role-group-change.impact@1.0.0",
    "platform.identity.user-organization-change.impact@1.0.0",
    "platform.identity.role-state-or-group-change.impact@1.0.0",
    "platform.authorization.resource-lifecycle.impact@1.0.0",
    "platform.navigation.role-menu-change.impact@1.0.0",
    "platform.authorization.role-permission-change.impact@1.0.0",
    "platform.navigation.role-menu-migration.compare@1.0.0",
  ];
  const expectedCommandKeys = [
    "platform.identity.organization-role-group-change.prepare@1.0.0",
    "platform.identity.user-organization-change.prepare@1.0.0",
    "platform.identity.role-state-or-group-change.prepare@1.0.0",
    "platform.authorization.resource-lifecycle.prepare@1.0.0",
    "platform.navigation.role-menu-change.prepare@1.0.0",
    "platform.authorization.role-permission-change.prepare@1.0.0",
    "platform.identity.organization-role-groups.replace@1.0.0",
    "platform.identity.user-organization.change@1.0.0",
    "platform.identity.role.move@1.0.0",
    "platform.identity.role.disable@1.0.0",
    "platform.authorization.resource-lifecycle.apply@1.0.0",
    "platform.navigation.role-menus.replace@1.0.0",
    "platform.authorization.role-permissions.replace@1.0.0",
  ];
  if (!sameList(queryKeys, expectedQueryKeys)) {
    errors.push("serviceObjectContract.queries must exactly match the frozen query registry");
  }
  if (!sameList(commandKeys, expectedCommandKeys)) {
    errors.push("serviceObjectContract.commands must exactly match the frozen prepare and apply command registry");
  }
  for (const definition of [...values(contract.serviceObjectContract?.queries), ...values(contract.serviceObjectContract?.commands)]) {
    if (typeof definition.version !== "string" || !/^\d+\.\d+\.\d+$/.test(definition.version)) {
      errors.push(`service object ${definition.id ?? "<missing>"} version must use a numeric semver string`);
    }
  }
  for (const command of values(contract.serviceObjectContract?.commands)) {
    requireEqual(command.executionAdapter, "domain-command-executor", `service object command ${command.id} executionAdapter`, errors);
  }
  if (new Set([...queryKeys, ...commandKeys]).size !== queryKeys.length + commandKeys.length) {
    errors.push("service object id/version pairs must be unique");
  }
  requireEqual(contract.serviceObjectContract?.runtimeReference, "resources/platform-service-object-runtime.json", "serviceObjectContract.runtimeReference", errors);
  requireEqual(contract.serviceObjectContract?.versionFormat, "numeric-semver-string", "serviceObjectContract.versionFormat", errors);
  requireEqual(serviceRuntime.definitionContract?.maximumAffectedRows, 1000, "service runtime maximumAffectedRows", errors);
  requireIncludes(serviceRuntime.definitionContract?.logicalAST, ["query", "insert", "update"], "service runtime logicalAST", errors);
  requireEqual(contract.serviceObjectContract?.executionBoundary?.queryAdapter, "read-only-persisted-query-runtime-with-scalar-preview-id-and-trusted-principal-scope", "serviceObjectContract.executionBoundary.queryAdapter", errors);
  requireEqual(contract.serviceObjectContract?.executionBoundary?.genericCommandAdapter, "single-resource-insert-or-update-maximum-1000-affected-rows", "serviceObjectContract.executionBoundary.genericCommandAdapter", errors);
  requireEqual(contract.serviceObjectContract?.executionBoundary?.domainCommandAdapter, "required-for-atomic-multi-relation-replace-move-disable-and-lifecycle-operations", "serviceObjectContract.executionBoundary.domainCommandAdapter", errors);
  requireEqual(contract.serviceObjectContract?.executionBoundary?.largeSelectionPolicy, "server-side-diff-in-one-native-transaction-no-client-chunked-partial-apply", "serviceObjectContract.executionBoundary.largeSelectionPolicy", errors);
  requireEqual(contract.serviceObjectContract?.executionBoundary?.implementedBy, "organization-role-pool-backend-and-migration", "serviceObjectContract.executionBoundary.implementedBy", errors);
  const domainRuntime = contract.serviceObjectContract?.domainCommandRuntimeExtension ?? {};
  requireEqual(domainRuntime.registry, "versioned-domain-command-handler-registry", "serviceObjectContract.domainCommandRuntimeExtension.registry", errors);
  requireIncludes(domainRuntime.argumentTypes, ["string-set", "typed-remediation-list"], "serviceObjectContract.domainCommandRuntimeExtension.argumentTypes", errors);
  requireEqual(domainRuntime.planType, "DomainCommandPlan-independent-from-generic-CommandAST", "serviceObjectContract.domainCommandRuntimeExtension.planType", errors);
  requireIncludes(domainRuntime.sharedGuards, ["authentication", "authorization", "trusted-tenant-and-data-scope", "cost", "timeout", "idempotency", "audit"], "serviceObjectContract.domainCommandRuntimeExtension.sharedGuards", errors);
  requireEqual(domainRuntime.previewPersistence, "server-persists-normalized-full-change-set-with-owner-scope-expiry-revision-and-impact-hash", "serviceObjectContract.domainCommandRuntimeExtension.previewPersistence", errors);
  requireIncludes(domainRuntime.applyArguments, ["previewId", "expectedRevision", "impactHash", "idempotencyKey"], "serviceObjectContract.domainCommandRuntimeExtension.applyArguments", errors);
  requireEqual(domainRuntime.maximumSelectedLeafKeys, 2000, "serviceObjectContract.domainCommandRuntimeExtension.maximumSelectedLeafKeys", errors);
  requireEqual(domainRuntime.genericMaximumAffectedRowsUnchanged, 1000, "serviceObjectContract.domainCommandRuntimeExtension.genericMaximumAffectedRowsUnchanged", errors);
  const previewFlow = contract.serviceObjectContract?.previewFlow ?? {};
  requireEqual(previewFlow.prepare, "idempotent-domain-prepare-command-accepts-set-and-remediation-arguments-and-persists-reviewed-change-set", "serviceObjectContract.previewFlow.prepare", errors);
  requireIncludes(previewFlow.prepareResult, ["previewId", "expectedRevision", "impactHash", "expiresAt"], "serviceObjectContract.previewFlow.prepareResult", errors);
  requireEqual(previewFlow.impactQuery, "read-only-persisted-query-loads-owner-scoped-preview-by-previewId", "serviceObjectContract.previewFlow.impactQuery", errors);
  requireEqual(previewFlow.apply, "domain-apply-command-reloads-preview-revalidates-and-commits-atomically", "serviceObjectContract.previewFlow.apply", errors);
  requireEqual(previewFlow.genericQuerySideEffects, "forbidden", "serviceObjectContract.previewFlow.genericQuerySideEffects", errors);
  requireIncludes(contract.serviceObjectContract?.highRiskRequestFields, ["previewId", "expectedRevision", "impactHash", "idempotencyKey"], "serviceObjectContract.highRiskRequestFields", errors);

  const conflict = contract.conflictContract ?? {};
  requireEqual(conflict.defaultStrategy, "reject", "conflictContract.defaultStrategy", errors);
  requireEqual(conflict.silentRoleRetention, "forbidden", "conflictContract.silentRoleRetention", errors);
  requireEqual(conflict.silentRoleRemoval, "forbidden", "conflictContract.silentRoleRemoval", errors);
  requireIncludes(conflict.preview?.requiredFields, ["previewId", "expectedRevision", "impactHash", "expiresAt"], "conflictContract.preview.requiredFields", errors);
  requireEqual(conflict.stalePreview, "reject-409-and-recompute", "conflictContract.stalePreview", errors);
  requireEqual(conflict.operations?.emergencyRoleDisable, "separate-high-permission-immediate-deny-with-unresolved-conflict-and-blocked-reenable", "conflictContract.operations.emergencyRoleDisable", errors);
  requireEqual(conflict.operations?.authorizationResourceDelete, "same-impact-preview-and-remediation-as-disable-before-logical-delete", "conflictContract.operations.authorizationResourceDelete", errors);
  requireEqual(conflict.operations?.authorizationResourceRestore, "restore-disabled-with-no-effective-authorization-then-explicitly-revalidate-and-enable", "conflictContract.operations.authorizationResourceRestore", errors);
  requireEqual(conflict.operations?.authorizationResourcePurge, "reject-until-retention-and-all-live-or-unresolved-reference-gates-pass", "conflictContract.operations.authorizationResourcePurge", errors);
  requireIncludes(conflict.audit?.requiredFields, ["actorType", "beforeRevision", "afterRevision", "requestId", "traceId", "conflictCount", "changeSetHash"], "conflictContract.audit.requiredFields", errors);

  requireEqual(contract.menuContract?.directory?.route, "must-be-empty", "menuContract.directory.route", errors);
  requireEqual(contract.menuContract?.directory?.interaction, "expand-collapse-only", "menuContract.directory.interaction", errors);
  requireEqual(contract.menuContract?.page?.mustBeLeaf, true, "menuContract.page.mustBeLeaf", errors);
  requireEqual(contract.menuContract?.assignment?.storedNodes, "page-only", "menuContract.assignment.storedNodes", errors);
  requireEqual(contract.permissionContract?.pageButton?.apiPermissionStillRequired, true, "permissionContract.pageButton.apiPermissionStillRequired", errors);
  requireEqual(contract.permissionContract?.owners?.menuVisibility, "role_menu", "permissionContract.owners.menuVisibility", errors);

  const migration = contract.menuPermissionMigration ?? {};
  requireEqual(migration.legacyField, "menus.permission", "menuPermissionMigration.legacyField", errors);
  requireEqual(migration.cutoverGate, "zero-unapproved-principal-diffs-and-reviewed-rollback-plan", "menuPermissionMigration.cutoverGate", errors);
  requireIncludes(migration.phases, [
    "compare-legacy-and-candidate-effective-menus-for-every-active-principal",
    "dual-read-return-legacy-and-record-value-free-diffs",
    "bounded-cutover-observation-with-auth-writes-frozen",
    "enable-role-menu-writes-and-require-checkpoint-for-legacy-rollback",
  ], "menuPermissionMigration.phases", errors);
  requireEqual(migration.crossRoleDenyRisk, "must-be-detected-by-principal-level-comparison", "menuPermissionMigration.crossRoleDenyRisk", errors);

  const identityMigration = contract.identityRbacMigration ?? {};
  requireIncludes(identityMigration.migrationManifestRequired, [
    "roleGroupScopeTenantMap",
    "orphanRoleGroupMap",
    "tenantUserOrganizationMap",
    "organizationRoleGroupBindingMap",
    "platformPrincipalAllowlist",
    "rolePoolConflictRemediations",
  ], "identityRbacMigration.migrationManifestRequired", errors);
  requireEqual(identityMigration.legacyNestedRoleGroups, "flatten-to-top-level-with-stable-group-identity-no-role-move-and-explicit-scope-tenant-map", "identityRbacMigration.legacyNestedRoleGroups", errors);
  requireEqual(identityMigration.orphanRoles, "block-until-explicitly-mapped-to-one-existing-or-new-role-group", "identityRbacMigration.orphanRoles", errors);
  requireEqual(identityMigration.usersWithoutOrganization, "block-unless-explicitly-assigned-one-organization-or-listed-as-platform-principal", "identityRbacMigration.usersWithoutOrganization", errors);
  requireEqual(identityMigration.platformPrincipalDetection, "explicit-reviewed-allowlist-never-inferred-from-empty-organization", "identityRbacMigration.platformPrincipalDetection", errors);
  requireEqual(identityMigration.organizationRoleGroupBackfill?.candidateAlgorithm, "per-organization-distinct-owner-groups-of-existing-user-roles", "identityRbacMigration.organizationRoleGroupBackfill.candidateAlgorithm", errors);
  requireEqual(identityMigration.organizationRoleGroupBackfill?.applicationPolicy, "explicit-versioned-manifest-approval-required-no-inferred-binding-is-applied", "identityRbacMigration.organizationRoleGroupBackfill.applicationPolicy", errors);
  requireEqual(identityMigration.organizationRoleGroupBackfill?.expansionReview, "record-and-approve-every-newly-assignable-role-before-cutover", "identityRbacMigration.organizationRoleGroupBackfill.expansionReview", errors);
  requireIncludes(identityMigration.phases, [
    "inventory-and-export-all-identity-rbac-conflicts",
    "approve-versioned-migration-manifest",
    "map-or-block-every-orphan-role",
    "create-explicit-platform-principals-from-reviewed-allowlist",
    "derive-minimal-organization-role-group-candidates-and-approve-expansion-diffs",
    "compare-principal-role-permission-data-scope-and-menu-results",
    "freeze-legacy-authorization-writes-for-final-diff",
    "cut-over-atomically-with-database-checkpoint",
  ], "identityRbacMigration.phases", errors);
  requireEqual(identityMigration.cutoverGate, "zero-unresolved-identity-rbac-conflicts-and-zero-unapproved-principal-authorization-diffs", "identityRbacMigration.cutoverGate", errors);

  const transfer = contract.treeTransferContract ?? {};
  requireEqual(transfer.component, "PlatformTreeTransfer", "treeTransferContract.component", errors);
  requireEqual(transfer.workbenchComponent, "AdminTreeWorkbench", "treeTransferContract.workbenchComponent", errors);
  requireEqual(transfer.value, "assignable-leaf-keys-plus-revision", "treeTransferContract.value", errors);
  requireEqual(transfer.virtualizeAtVisibleNodes, 50, "treeTransferContract.virtualizeAtVisibleNodes", errors);
  requireEqual(transfer.largeDatasetAcceptance?.nodes, 10000, "treeTransferContract.largeDatasetAcceptance.nodes", errors);
  requireEqual(transfer.largeDatasetAcceptance?.selected, 2000, "treeTransferContract.largeDatasetAcceptance.selected", errors);
  requireEqual(transfer.accessibility?.mixedState, "aria-checked=mixed", "treeTransferContract.accessibility.mixedState", errors);
  requireEqual(transfer.accessibility?.minimumTargetPixels, 44, "treeTransferContract.accessibility.minimumTargetPixels", errors);
  requireIncludes(transfer.accessibility?.keyboard, ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End", "Space", "Enter"], "treeTransferContract.accessibility.keyboard", errors);
  requireEqual(transfer.responsive?.mobileMode, "available-selected-single-pane-tabs-with-sticky-actions", "treeTransferContract.responsive.mobileMode", errors);

  requireIncludes(contract.browserAcceptance?.viewports, ["375x812", "390x844", "768x1024", "1024x768", "1280x720", "1440x1024"], "browserAcceptance.viewports", errors);
  requireIncludes(contract.browserAcceptance?.scenarios, [
    "user-organization-change-preview-and-explicit-invalid-role-remediation",
    "menu-and-permission-save-refresh-and-separate-audit",
    "large-tree-search-scroll-half-select-and-save",
    "legacy-role-menu-dual-read-equivalence-and-rollback",
  ], "browserAcceptance.scenarios", errors);

  requireEqual(contract.runtimeBoundary?.implementationStatus, "contract-only", "runtimeBoundary.implementationStatus", errors);
  if (!sameList(values(contract.runtimeBoundary?.deferredTaskIds), downstreamTaskIds)) {
    errors.push("runtimeBoundary.deferredTaskIds must match the five downstream organization/RBAC/menu nodes");
  }
  requireIncludes(contract.runtimeBoundary?.notOwned, ["datasource-routing", "federated-query", "xa", "outbox", "mq", "search-projection", "workload-identity"], "runtimeBoundary.notOwned", errors);

  for (const [kind, paths] of Object.entries(contract.evidence ?? {})) {
    for (const relativePath of values(paths)) requireExisting(relativePath, `evidence.${kind}`, errors);
  }
}

function validateTopology(contract, topology, errors) {
  const migration = topology.organizationRbacMenuMigration ?? {};
  requireEqual(migration.status, "planned", "topology organization migration status", errors);
  requireEqual(migration.designStatus, "frozen", "topology organization migration designStatus", errors);
  requireEqual(migration.designContract, "resources/platform-organization-rbac-menu-contract.json", "topology organization migration designContract", errors);
  requireEqual(migration.targetModel?.roleOwnership, contract.targetModel?.roleOwnership, "topology target roleOwnership", errors);
  requireEqual(migration.targetModel?.userRoleConstraint, "subset-of-organization-role-pool", "topology target userRoleConstraint", errors);
  requireEqual(migration.targetModel?.roleMenuBinding, "role_menu", "topology target roleMenuBinding", errors);
}

function validateTaskGovernance(contract, graph, matrix, errors) {
  const task = values(graph.tasks).find((item) => item.id === taskId);
  if (!task) {
    errors.push(`task graph must include ${taskId}`);
  } else {
    requireEqual(task.status, "implemented", `task ${taskId} status`, errors);
    requireEqual(task.contractGateOnly, true, `task ${taskId} contractGateOnly`, errors);
    requireIncludes(task.designGate, ["superpowers:brainstorming", "product-design"], `task ${taskId} designGate`, errors);
    requireIncludes(task.evidence?.docs, contract.evidence?.docs, `task ${taskId} evidence.docs`, errors);
    requireIncludes(task.evidence?.validators, contract.evidence?.validators, `task ${taskId} evidence.validators`, errors);
    requireIncludes(task.evidence?.tests, contract.evidence?.tests, `task ${taskId} evidence.tests`, errors);
  }
  const backendTask = values(graph.tasks).find((item) => item.id === "organization-role-pool-backend-and-migration");
  requireIncludes(backendTask?.resourceLocks, ["query-command-contract"], "organization-role-pool-backend-and-migration resourceLocks", errors);
  for (const id of downstreamTaskIds) {
    const downstream = values(graph.tasks).find((item) => item.id === id);
    if (!downstream || downstream.status === "implemented") {
      errors.push(`downstream runtime/UI task ${id} must remain unfinished`);
    }
  }

  const capability = values(matrix.capabilities).find((item) => item.id === taskId);
  if (!capability) {
    errors.push(`engineering matrix must include ${taskId}`);
  } else {
    requireEqual(capability.status, "implemented", `engineering capability ${taskId} status`, errors);
    requireIncludes(capability.evidence?.sourcePaths, [
      "resources/platform-organization-rbac-menu-contract.json",
      "docs/platform-organization-rbac-menu-contract.md",
      "docs/superpowers/specs/2026-07-15-organization-rbac-menu-contract-and-migration-design.md",
    ], `engineering capability ${taskId} sourcePaths`, errors);
    requireIncludes(capability.evidence?.validators, ["scripts/validate-platform-organization-rbac-menu-contract.mjs"], `engineering capability ${taskId} validators`, errors);
    requireIncludes(capability.evidence?.tests, ["scripts/platform-organization-rbac-menu-contract.test.mjs"], `engineering capability ${taskId} tests`, errors);
    requireEqual(capability.evidence?.runtimeBoundary?.implementationStatus, "contract-only", `engineering capability ${taskId} runtimeBoundary.implementationStatus`, errors);
  }
  for (const id of downstreamTaskIds) {
    const capabilityItem = values(matrix.capabilities).find((item) => item.id === id);
    requireEqual(capabilityItem?.status, "partial", `engineering capability ${id} status`, errors);
  }
}

function validateDocs(errors) {
  const durable = readText("docs/platform-organization-rbac-menu-contract.md");
  const design = readText("docs/superpowers/specs/2026-07-15-organization-rbac-menu-contract-and-migration-design.md");
  for (const phrase of [
    "server derives and redundantly persists `tenantCode`",
    "`role_menu` persists page leaves only",
    "zero unapproved principal differences",
    "Emergency role disable",
    "10,000 nodes with 2,000 selected items",
    "numeric SemVer string",
    "restore into a disabled state",
    "versioned migration manifest",
    "domain command executor",
  ]) {
    if (!durable.includes(phrase)) errors.push(`organization RBAC menu doc must include ${phrase}`);
  }
  if (!design.includes("Runtime implementation is intentionally deferred")) {
    errors.push("design spec must state runtime implementation is deferred");
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const topology = readJSON(topologyPath);
  const graph = readJSON(taskGraphPath);
  const matrix = readJSON(matrixPath);
  const serviceRuntime = readJSON(serviceRuntimePath);
  const errors = [];

  validateContract(contract, serviceRuntime, errors);
  validateTopology(contract, topology, errors);
  validateTaskGovernance(contract, graph, matrix, errors);
  validateDocs(errors);

  if (errors.length > 0) {
    for (const error of errors) process.stderr.write(`- ${error}\n`);
    process.exit(1);
  }
  process.stdout.write(`Validated platform organization RBAC menu contract: ${contract.status}\n`);
}

validate();
