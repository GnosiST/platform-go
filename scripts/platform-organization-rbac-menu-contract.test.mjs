import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "platform-org-rbac-menu-"));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-organization-rbac-menu-contract.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

describe("platform organization RBAC menu contract", () => {
  it("accepts the frozen contract and controlled downstream boundary", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform organization RBAC menu contract/);
  });

  it("rejects role ownership that regresses to many groups", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.targetModel.roleOwnership = "many-role-groups";
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /targetModel\.roleOwnership must be exactly-one-role-group/);
  });

  it("rejects implicit parent organization role-group inheritance", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.targetModel.organizationBindingInheritance = "ancestor-union";
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /organizationBindingInheritance must be none-direct-bindings-only/);
  });

  it("rejects silent cleanup and stale conflict application", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.conflictContract.silentRoleRemoval = "allowed";
    contract.conflictContract.stalePreview = "apply-best-effort";
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /silentRoleRemoval must be forbidden/);
    assert.match(result.stderr, /stalePreview must be reject-409-and-recompute/);
  });

  it("rejects service object versions and relation commands that do not match the runtime boundary", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.serviceObjectContract.queries[0].version = 1;
    contract.serviceObjectContract.commands[0].executionAdapter = "generic-command-runtime";
    contract.serviceObjectContract.executionBoundary.queryAdapter = "side-effecting-persisted-query";
    contract.serviceObjectContract.executionBoundary.largeSelectionPolicy = "client-chunks-may-partially-commit";
    contract.serviceObjectContract.domainCommandRuntimeExtension.argumentTypes = ["string"];
    contract.serviceObjectContract.domainCommandRuntimeExtension.planType = "CommandAST";
    contract.serviceObjectContract.previewFlow.prepare = "generic-impact-query-persists-change-set";
    contract.serviceObjectContract.previewFlow.genericQuerySideEffects = "allowed";
    const runtime = readJSON("resources/platform-service-object-runtime.json");
    runtime.definitionContract.maximumAffectedRows = 2000;

    const result = runValidator([
      "--contract",
      tempJSON("contract.json", contract),
      "--service-runtime",
      tempJSON("service-runtime.json", runtime),
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /version must use a numeric semver string/);
    assert.match(result.stderr, /executionAdapter must be domain-command-executor/);
    assert.match(result.stderr, /executionBoundary\.queryAdapter must be read-only-persisted-query-runtime-with-scalar-preview-id-and-trusted-principal-scope/);
    assert.match(result.stderr, /largeSelectionPolicy must be server-side-diff-in-one-native-transaction-no-client-chunked-partial-apply/);
    assert.match(result.stderr, /domainCommandRuntimeExtension\.argumentTypes must include string-set/);
    assert.match(result.stderr, /domainCommandRuntimeExtension\.planType must be DomainCommandPlan-independent-from-generic-CommandAST/);
    assert.match(result.stderr, /previewFlow\.prepare must be idempotent-domain-prepare-command-accepts-set-and-remediation-arguments-and-persists-reviewed-change-set/);
    assert.match(result.stderr, /previewFlow\.genericQuerySideEffects must be forbidden/);
    assert.match(result.stderr, /service runtime maximumAffectedRows must be 1000/);
  });

  it("rejects incomplete prepare impact and apply registries", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.serviceObjectContract.queries = contract.serviceObjectContract.queries.filter(
      (definition) => definition.id !== "platform.navigation.role-menu-change.impact",
    );
    contract.serviceObjectContract.commands = contract.serviceObjectContract.commands.filter(
      (definition) => definition.id !== "platform.identity.role-state-or-group-change.prepare",
    );

    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /serviceObjectContract\.queries must exactly match the frozen query registry/);
    assert.match(result.stderr, /serviceObjectContract\.commands must exactly match the frozen prepare and apply command registry/);
  });

  it("rejects authorization lifecycle paths that can bypass impact validation", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.identityContract.allWritePaths = contract.identityContract.allWritePaths.filter(
      (path) => !["delete", "restore", "purge"].includes(path),
    );
    contract.lifecycleAuthorizationContract.lifecycleManagedEntities = ["org-units", "roles"];
    contract.lifecycleAuthorizationContract.dependentAuthorizationRelations = ["user_roles"];
    contract.lifecycleAuthorizationContract.entityRestore = "restore-original-enabled-state";
    contract.lifecycleAuthorizationContract.relationMutation = "independent-soft-delete-and-restore";
    contract.conflictContract.operations.authorizationResourceDelete = "generic-soft-delete";

    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /identityContract\.allWritePaths must include delete/);
    assert.match(result.stderr, /lifecycleAuthorizationContract\.lifecycleManagedEntities must include role-groups/);
    assert.match(result.stderr, /lifecycleAuthorizationContract\.dependentAuthorizationRelations must include role_menu/);
    assert.match(result.stderr, /lifecycleAuthorizationContract\.entityRestore must be restore-disabled-then-revalidate-and-explicitly-enable/);
    assert.match(result.stderr, /lifecycleAuthorizationContract\.relationMutation must be explicit-server-side-atomic-diff-only-no-independent-restore/);
    assert.match(result.stderr, /authorizationResourceDelete must be same-impact-preview-and-remediation-as-disable-before-logical-delete/);
  });

  it("rejects identity migrations that infer platform users or leave orphan roles unresolved", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.identityRbacMigration.migrationManifestRequired = ["roleGroupScopeTenantMap"];
    contract.identityRbacMigration.orphanRoles = "assign-default-group";
    contract.identityRbacMigration.platformPrincipalDetection = "infer-from-empty-organization";
    contract.identityRbacMigration.organizationRoleGroupBackfill.applicationPolicy = "apply-derived-candidates";
    contract.identityRbacMigration.phases = contract.identityRbacMigration.phases.filter(
      (phase) => phase !== "compare-principal-role-permission-data-scope-and-menu-results",
    );

    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /identityRbacMigration\.migrationManifestRequired must include orphanRoleGroupMap/);
    assert.match(result.stderr, /identityRbacMigration\.migrationManifestRequired must include organizationRoleGroupBindingMap/);
    assert.match(result.stderr, /identityRbacMigration\.orphanRoles must be block-until-explicitly-mapped-to-one-existing-or-new-role-group/);
    assert.match(result.stderr, /identityRbacMigration\.platformPrincipalDetection must be explicit-reviewed-allowlist-never-inferred-from-empty-organization/);
    assert.match(result.stderr, /identityRbacMigration\.organizationRoleGroupBackfill\.applicationPolicy must be explicit-versioned-manifest-approval-required-no-inferred-binding-is-applied/);
    assert.match(result.stderr, /identityRbacMigration\.phases must include compare-principal-role-permission-data-scope-and-menu-results/);
  });

  it("rejects directory grants and page nodes that are not leaves", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.menuContract.assignment.storedNodes = "directories-and-pages";
    contract.menuContract.page.mustBeLeaf = false;
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /assignment\.storedNodes must be page-only/);
    assert.match(result.stderr, /page\.mustBeLeaf must be true/);
  });

  it("rejects migration plans without principal-level comparison", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.menuPermissionMigration.phases = contract.menuPermissionMigration.phases.filter(
      (phase) => phase !== "compare-legacy-and-candidate-effective-menus-for-every-active-principal",
    );
    contract.menuPermissionMigration.crossRoleDenyRisk = "ignored";
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must include compare-legacy-and-candidate-effective-menus-for-every-active-principal/);
    assert.match(result.stderr, /crossRoleDenyRisk must be must-be-detected-by-principal-level-comparison/);
  });

  it("rejects a Tree Transfer that drops the large-data and accessibility gates", () => {
    const contract = readJSON("resources/platform-organization-rbac-menu-contract.json");
    contract.treeTransferContract.virtualizeAtVisibleNodes = 500;
    contract.treeTransferContract.accessibility.minimumTargetPixels = 32;
    contract.treeTransferContract.accessibility.keyboard = ["ArrowUp", "ArrowDown"];
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /virtualizeAtVisibleNodes must be 50/);
    assert.match(result.stderr, /minimumTargetPixels must be 44/);
    assert.match(result.stderr, /accessibility\.keyboard must include Space/);
  });

  it("rejects regressing the backend and migration node after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((task) => task.id === "organization-role-pool-backend-and-migration").status = "pending";
    const result = runValidator(["--task-graph", tempJSON("task-graph.json", graph)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /organization-role-pool-backend-and-migration status must be implemented/);
  });

  it("rejects regressing the implemented organization and user Admin node", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((task) => task.id === "organization-user-admin-experience").status = "pending";
    const result = runValidator(["--task-graph", tempJSON("task-graph.json", graph)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /implemented downstream Admin task organization-user-admin-experience must stay implemented/);
  });

  it("rejects closing role, menu or full E2E nodes before their implementation", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((task) => task.id === "role-tree-and-authorization-entry").status = "implemented";
    const result = runValidator(["--task-graph", tempJSON("premature-role-tree.json", graph)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /downstream runtime\/UI task role-tree-and-authorization-entry must remain unfinished/);
  });
});
