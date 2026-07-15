import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

import "./admin-menu-governance-behavior.test.mjs";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-admin-ui-contracts.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempAdminRoot() {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "admin-ui-contracts-"));
  fs.mkdirSync(path.join(tempDir, "admin"), { recursive: true });
  fs.cpSync(path.join(repoRoot, "admin", "src"), path.join(tempDir, "admin", "src"), { recursive: true });
  return tempDir;
}

function replaceInTemp(tempRoot, relativePath, from, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  assert.ok(source.includes(from), `${relativePath} should contain ${from}`);
  fs.writeFileSync(filePath, source.split(from).join(to));
}

function replaceRegexInTemp(tempRoot, relativePath, pattern, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  assert.match(source, pattern);
  fs.writeFileSync(filePath, source.replace(pattern, to));
}

function replaceInTempIfPresent(tempRoot, relativePath, from, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  if (!source.includes(from)) return;
  fs.writeFileSync(filePath, source.split(from).join(to));
}

function runTypeScriptProbe(relativePath, body) {
  const moduleURL = pathToFileURL(path.join(repoRoot, relativePath)).href;
  return spawnSync(
    process.execPath,
    ["--experimental-strip-types", "--input-type=module", "--eval", body(moduleURL)],
    { cwd: repoRoot, encoding: "utf8" },
  );
}

function adminSource(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

describe("validate-admin-ui-contracts", () => {
  it("accepts the current componentized admin UI contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Admin UI contract validation passed/);
  });

  it("rejects an Admin runtime that enables Refine third-party telemetry", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "disableTelemetry: true",
      "disableTelemetry: false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must disable Refine third-party telemetry by default/);
  });

  it("rejects organization and user routes without the domain experience", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'experienceKey={resource.route === "/org-units" || resource.route === "/users" ? "organization-user" : undefined}',
      "experienceKey={undefined}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Organization and user routes must inject the shared organization-user experience/);
  });

  it("rejects role routes that bypass the shared governance console", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'resource.route === "/roles" || resource.route === "/role-groups"',
      'resource.route === "/roles"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role and role-group routes must share the role governance console/);
  });

  it("rejects a menus route that falls through to the generic resource console", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'resource.route === "/menus"',
      'resource.route === "/menus-disabled"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menus route must use the dedicated menu governance console/);
  });

  it("rejects menu governance that exposes page metadata while authoring a directory", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "{!directoryMode ? (",
      "{true ? (",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Page-only route, parameter, and button controls must stay hidden during directory authoring/);
  });

  it("rejects menu parameter rows without the forbidden-input guard", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "isForbiddenMenuParameterStringValue(value)",
      "false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must reject scripts, expressions, SQL, and physical routing inputs/);
  });

  it("rejects internal routes that reuse parameter keyword blocking", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "isSafeInternalMenuRoute(route)",
      "!isForbiddenMenuParameterStringValue(route)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must use route-specific validation instead of parameter-value keyword blocking/);
  });

  it("rejects menu page buttons without current-menu metadata binding", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "button.menuCode === values.code.trim()",
      "button.menuCode.length > 0",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Page-button metadata must point to the current menu code/);
  });

  it("rejects menu governance search that can apply a stale response", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "if (menuListRequest.current !== requestID) return;",
      "if (false) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu search must discard stale responses/);
  });

  it("rejects menu governance that requests records without menu read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "if (!canRead || menuListRequest.current !== requestID) return;",
      "if (menuListRequest.current !== requestID) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must fail closed before requesting records without read access/);
  });

  it("rejects menu definition loading that can retain stale detail state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "setDefinitionLoading(false);\n      setSelectedDefinition(null);\n      setSelectedRevision(0);",
      "setSelectedDefinition(null);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Clearing menu selection must also clear loading and revision state/);
  });

  it("rejects menu parent changes without structural confirmation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "await confirmMenuParentChange(modal.confirm, dictionary, editor.definition, definition, records)",
      "true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require an explicit localized structural confirmation/);
  });

  it("rejects page-button rows without duplicate permission-code validation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "duplicateButtonPermission(form, index, dictionary.menuButtonPermissionDuplicate)",
      "safeCodeRule(dictionary.menuButtonPermissionInvalid)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /permission codes must expose duplicate validation/);
  });

  it("rejects menu saves without editor-session isolation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "const editorSession = useRef(0);",
      "const editorSessionMissing = useRef(0);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu saves must ignore stale mutation completions/);
  });

  it("rejects menu saves without a synchronous submission lock", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "const savingRef = useRef(false);",
      "const savingRefMissing = useRef(false);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu saves must acquire a synchronous single-flight lock/);
  });

  it("rejects menu save focus restoration without a stable detail target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "returnFocusRef.current = detailFocusRef.current;",
      "returnFocusRef.current = returnFocusRef.current;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Successful menu saves must restore focus to a stable detail target/);
  });

  it("rejects menu modal controls without complete 44px targets", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".menu-governance-modal .ant-modal-close,",
      ".menu-governance-modal .ant-modal-close-missing,",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu governance modal controls must expose 44px targets/);
  });

  it("rejects tree workbench search without a tablet touch target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/styles.css",
      ".admin-tree-workbench-navigation .admin-list-toolbar .ant-input-affix-wrapper {",
      ".admin-tree-workbench-navigation .admin-list-toolbar .ant-input-affix-wrapper-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree workbench search must expose a 44px tablet touch target/);
  });

  it("rejects tree workbench expanders without a 44px target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".admin-tree-workbench-tree .ant-tree-switcher {",
      ".admin-tree-workbench-tree .ant-tree-switcher-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree workbench expanders must expose a 44px pointer target/);
  });

  it("rejects menu governance that bypasses generated menu-definition service objects", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/organizationRBAC.ts",
      "client.replaceMenuDefinition",
      "transport.post",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menu-definition replace wrapper must use the generated service-object client/);
  });

  it("rejects menu creation that hard-codes the initial global revision", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "createMenuDefinition(definition, selectedRevision)",
      "createMenuDefinition(definition, 0)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu creation must use the most recent trusted global menu revision/);
  });

  it("rejects menu saves that retain a stale selected-definition revision", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "setDefinitionRefresh((current) => current + 1);",
      "void selectedRevision;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must reload the normalized definition and its new global revision/);
  });

  it("rejects role governance that queries a resource the principal cannot read", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'canReadGroups ? loadAllRecords("role-groups") : Promise.resolve([])',
      'loadAllRecords("role-groups")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not request role or role-group resources the current principal cannot read/);
  });

  it("rejects role permission assignment that omits a supporting read permission", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions) && hasPermission(permissions, "admin:area-code:read", deniedPermissions);',
      'const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require every resource read permission used by its editor/);
  });

  it("rejects read-only role menu assignment without menu read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      '{canReadMenus ? <Button ref={menuTriggerRef}',
      '{true ? <Button ref={menuTriggerRef}',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must be hidden when menu records cannot be read/);
  });

  it("requires cutover-gated role menu assignment to persist page leaves only", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "directory-a", name: "Directory A", status: "enabled", nodeType: "directory" },
          { code: "page-a", name: "Page A", status: "enabled", nodeType: "page", parentCode: "directory-a" },
        ],
        [],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      assert.deepEqual(pageMenuCodes(nodes, ["directory-a", "page-a"]), ["page-a"]);
    `);

    assert.match(roleGovernance, /const menuCodes = pageMenuCodes\(menuTreeNodes\(menus, menuAssignment\.menuCodes, dictionary\), menuAssignment\.menuCodes\);/);
    assert.doesNotMatch(roleGovernance, /menuCodes:\s*legacyVisibleMenus/);
    assert.equal(result.status, 0, result.stderr);
  });

  it("requires Tree Transfer directory state to be derived without persisting branch keys", () => {
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(treeTransfer, /const normalizedValue = useMemo\(\(\) => leafValues\(nodes, value\), \[nodes, value\]\);/);
    assert.match(treeTransfer, /halfCheckedKeys/);
    assert.match(treeTransfer, /checkStrictly/);
  });

  it("requires disabled and missing historical role menu selections to stay removable", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "disabled-page", name: "Disabled Page", status: "disabled", nodeType: "page" }],
        ["disabled-page", "missing-page"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      assert.equal(nodes.find((node) => node.key === "disabled-page")?.availableDisabledReason, "Disabled");
      assert.equal(nodes.find((node) => node.key === "missing-page")?.availableDisabledReason, "Missing");
      assert.deepEqual(pageMenuCodes(nodes, ["disabled-page", "missing-page"]), ["disabled-page", "missing-page"]);
    `);

    assert.match(roleGovernance, /menuTreeNodes\(menus, menuAssignment\.menuCodes, dictionary\)/);
    assert.equal(result.status, 0, result.stderr);
  });

  it("requires role update plus menu read permission while the migration write gate stays closed", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const organizationRBAC = adminSource("admin/src/platform/api/organizationRBAC.ts");

    assert.match(roleGovernance, /const canAssignMenus = canReadMenus && canUpdateRole;/);
    assert.match(roleGovernance, /readOnly=\{!roleMenuMigrationWriteEnabled \|\| !canAssignMenus\}/);
    assert.match(roleGovernance, /if \(!menuAssignment \|\| !roleMenuMigrationWriteEnabled \|\| !canAssignMenus\) return;/);
    assert.match(organizationRBAC, /export const roleMenuMigrationWriteEnabled = false;/);
  });

  it("requires one generated-client role menu prepare, impact and apply flow without client chunks", () => {
    const organizationRBAC = adminSource("admin/src/platform/api/organizationRBAC.ts");

    assert.match(organizationRBAC, /client\.getRoleMenus/);
    assert.match(organizationRBAC, /client\.prepareRoleMenuChange/);
    assert.match(organizationRBAC, /client\.getRoleMenuChangeImpact/);
    assert.match(organizationRBAC, /client\.replaceRoleMenus/);
    assert.doesNotMatch(organizationRBAC, /menuCodes\.(?:slice|splice)\(/);
  });

  it("keeps closed-gate legacy menu visibility independent from target assignment reads", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(roleGovernance, /const targetRequest = roleMenuMigrationWriteEnabled \? getRoleMenus\(role\.code\) : Promise\.resolve\(null\);/);
    assert.match(roleGovernance, /menuCodes: targetAssignment \? \[\.\.\.targetAssignment\.menuCodes\] : \[\]/);
    assert.match(roleGovernance, /const value = migrationReadOnly \? legacyVisible : menuAssignment\.menuCodes;/);
  });

  it("requires stale role-menu opens and closes to invalidate older async results", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const openMenus = roleGovernance.slice(roleGovernance.indexOf("const openMenus"), roleGovernance.indexOf("const closeMenus"));

    assert.match(roleGovernance, /const menuRequest = useRef\(0\);/);
    assert.match(openMenus, /const requestID = \+\+menuRequest\.current;/);
    assert.ok((openMenus.match(/if \(menuRequest\.current !== requestID\) return;/g) ?? []).length >= 2);
    assert.match(roleGovernance, /const closeMenus = \(\) => \{\s*menuRequest\.current \+= 1;\s*setMenuAssignment\(null\);\s*\};/);
  });

  it("requires role-menu impact to match the opened snapshot before apply", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const guard = roleGovernance.indexOf("!roleMenuImpactMatches(impact, menuAssignment, menuCodes)");
    const apply = roleGovernance.indexOf("await replaceRoleMenus(preview);");

    assert.match(roleGovernance, /initialMenuCodes: string\[\];/);
    assert.match(roleGovernance, /impact\.expectedRevision === assignment\.revision/);
    assert.match(roleGovernance, /impact\.previewId !== preview\.previewId \|\| impact\.impactHash !== preview\.impactHash/);
    assert.match(roleGovernance, /sameStringSet\(impact\.currentMenuCodes, assignment\.initialMenuCodes\)/);
    assert.match(roleGovernance, /sameStringSet\(impact\.proposedMenuCodes, proposedMenuCodes\)/);
    assert.match(roleGovernance, /impact\.changed === !sameStringSet\(assignment\.initialMenuCodes, proposedMenuCodes\)/);
    assert.match(roleGovernance, /setError\(dictionary\.changePreviewUnavailable\);\s*await openMenus\(menuAssignment\.role\);\s*return;/);
    assert.ok(guard >= 0 && apply > guard, "snapshot guard must run before apply");
  });

  it("requires explicit role-menu impact confirmation before apply", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const confirm = roleGovernance.indexOf("if (!await confirmRoleMenuImpact(modal.confirm, dictionary, impact)) return;");
    const apply = roleGovernance.indexOf("await replaceRoleMenus(preview);");

    assert.match(roleGovernance, /impact\.currentMenuCodes\.join\(", "\)/);
    assert.match(roleGovernance, /impact\.proposedMenuCodes\.join\(", "\)/);
    assert.match(roleGovernance, /dictionary\.currentContext/);
    assert.match(roleGovernance, /dictionary\.reviewAndApply/);
    assert.ok(confirm >= 0 && apply > confirm, "impact cancellation must stop apply");
  });

  it("keeps missing historical menu selections visible under a removable selected branch", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(roleGovernance, /projectMenuTreeNodes/);
    assert.match(roleGovernance, /historicalLabel: dictionary\.permissionTypeHistorical/);
    assert.match(roleGovernance, /missingReason: dictionary\.rolePermissionHistoricalMissing/);
  });

  it("projects a unique historical branch when a real page uses the base branch key", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "menu-history", name: "History Page", status: "enabled", nodeType: "page" },
          { code: "page-a", name: "Page A", status: "enabled", nodeType: "page" },
        ],
        ["menu-history", "missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const keys = nodes.map((node) => node.key);
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const missing = nodes.find((node) => node.key === "missing-a");
      assert.equal(new Set(keys).size, keys.length);
      assert.ok(historyBranch);
      assert.notEqual(historyBranch.key, "menu-history");
      assert.equal(missing?.parentKey, historyBranch.key);
      assert.deepEqual(pageMenuCodes(nodes, keys), ["menu-history", "missing-a", "page-a"]);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("projects a unique historical branch when a missing historical page uses the base branch key", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "page-a", name: "Page A", status: "enabled", nodeType: "page" }],
        ["menu-history"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const keys = nodes.map((node) => node.key);
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const missing = nodes.find((node) => node.key === "menu-history");
      assert.equal(new Set(keys).size, keys.length);
      assert.ok(historyBranch);
      assert.notEqual(historyBranch.key, "menu-history");
      assert.equal(missing?.parentKey, historyBranch.key);
      assert.notEqual(missing?.key, missing?.parentKey);
      assert.deepEqual(pageMenuCodes(nodes, [historyBranch.key, "menu-history"]), ["menu-history"]);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("avoids a dangling parent code when projecting the historical branch", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "orphan-page", name: "Orphan Page", status: "enabled", nodeType: "page", parentCode: "menu-history" }],
        ["missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const orphan = nodes.find((node) => node.key === "orphan-page");
      assert.ok(historyBranch);
      assert.equal(orphan?.parentKey, "menu-history");
      assert.notEqual(historyBranch.key, "menu-history");
      assert.notEqual(orphan?.parentKey, historyBranch.key);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("advances the historical branch suffix past dangling parent candidates", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "orphan-a", name: "Orphan A", status: "enabled", nodeType: "page", parentCode: "menu-history" },
          { code: "orphan-b", name: "Orphan B", status: "enabled", nodeType: "page", parentCode: "menu-history:1" },
        ],
        ["missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const catalogNodes = nodes.filter((node) => node.code === "orphan-a" || node.code === "orphan-b");
      assert.ok(historyBranch);
      assert.equal(historyBranch.key, "menu-history:2");
      assert.deepEqual(catalogNodes.map((node) => node.parentKey), ["menu-history", "menu-history:1"]);
      assert.ok(catalogNodes.every((node) => node.parentKey !== historyBranch.key));
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("derives directory full and half state from each pane eligible descendants", () => {
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(treeTransfer, /const selectedEligibleLeafKeySet = useMemo/);
    assert.match(treeTransfer, /derivedTreeSelection\(nodes, normalizedValue, leafValues\(nodes, visibleCheckedKeys\), filteredKeys, selectableLeafKeySet\)/);
    assert.match(treeTransfer, /derivedTreeSelection\(nodes, normalizedValue, leafValues\(nodes, visibleCheckedKeys\), filteredKeys, selectedEligibleLeafKeySet\)/);
    assert.match(treeTransfer, /leafDescendants\(nodes, node\.key\)\.filter\(\(key\) => eligibleLeafKeys\.has\(key\)\)/);
  });

  it("only renders legacy read-only messaging while role-menu assignment is read-only", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(roleGovernance, /\{migrationReadOnly \? <AdminFeedback type="warning"/);
    assert.match(roleGovernance, /showReadOnlyMessage=\{migrationReadOnly\}/);
    assert.match(treeTransfer, /showReadOnlyMessage && readOnlyMessage/);
    assert.doesNotMatch(roleGovernance, /^\s*<AdminFeedback type="warning" message=\{dictionary\.roleMenuLegacyReadonlyTitle\}/m);
  });

  it("rejects tenant-scoped role-group creation without tenant read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const canCreateGroup = hasPermission(permissions, "admin:role-group:create", deniedPermissions) && canReadTenants;',
      'const canCreateGroup = hasPermission(permissions, "admin:role-group:create", deniedPermissions);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require tenant read access/);
  });

  it("rejects role governance search that can apply a stale response", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "if (governanceRequest.current !== requestID) return;",
      "if (false) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must discard stale role and role-group search responses/);
  });

  it("rejects a stale debounced role-governance request that can restart loading", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const loadGovernance = useCallback(async (query = "", requestID = ++governanceRequest.current) => {\n    if (governanceRequest.current !== requestID) return;\n    setLoading(true);',
      'const loadGovernance = useCallback(async (query = "", requestID = ++governanceRequest.current) => {\n    setLoading(true);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not re-enter the loading state/);
  });

  it("rejects filtered Tree Transfer behavior that drops hidden selections", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const preserved = value.filter((key) => !mutableVisibleSet.has(key));",
      "const preserved: string[] = [];",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must preserve selections outside the current result/);
  });

  it("rejects filtered Tree Transfer rendering that passes hidden selections to Ant Tree", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const visibleCheckedKeys = useMemo(() => value.filter((key) => filteredKeys.has(key)), [filteredKeys, value]);",
      "const visibleCheckedKeys = value;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not pass hidden selections to Ant Tree/);
  });

  it("rejects role move options that cross scope or tenant boundaries", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "sameRoleGroupBoundary(group, moveSourceGroup)",
      "true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must stay inside the current scope and tenant boundary/);
  });

  it("rejects role mutation actions that remain enabled for disabled roles", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'disabled={!canUpdateRole || record.status !== "enabled"}',
      "disabled={!canUpdateRole}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Disabled roles must not expose move or permission mutation actions/);
  });

  it("rejects role detail actions without 44px touch targets", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".role-governance-detail .admin-list-actions .ant-btn,",
      ".role-governance-detail .admin-list-actions-missing .ant-btn,",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role detail actions must expose 44px targets/);
  });

  it("rejects Tree Transfer checkboxes without a real 44px pointer target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".platform-tree-transfer-pane .ant-tree-checkbox {",
      ".platform-tree-transfer-pane .ant-tree-checkbox-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree Transfer checkboxes must expose a real 44px by 44px pointer target/);
  });

  it("rejects Tree Transfer bulk operations that clear disabled selections", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const preservedDisabled = value.filter((key) => !mutableLeafKeySet.has(key));",
      "const preservedDisabled: string[] = [];",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /bulk operations must preserve disabled selections/);
  });

  it("rejects Tree Transfer bulk assignment that can add unavailable historical permissions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      'node.kind === "leaf" && !node.disabledReason && !node.availableDisabledReason',
      'node.kind === "leaf" && !node.disabledReason',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /bulk assignment must exclude unavailable historical selections/);
  });

  it("rejects Tree Transfer that prevents removing historical permissions from the selected pane", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const unavailable = Boolean(node.disabledReason || !selectedOnly && node.availableDisabledReason);",
      "const unavailable = Boolean(node.disabledReason || node.availableDisabledReason);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /historical selections must be disabled only in the available pane/);
  });

  it("rejects role authorization that drops disabled or missing historical permissions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "permissionTreeNodes(permissionCatalog, dictionary, uniqueSorted([...authorization.allow, ...authorization.deny]))",
      "permissionTreeNodes(permissionCatalog, dictionary)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must project disabled and missing historical permissions/);
  });

  it("rejects a second generic role mutation after atomic policy apply", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "await replaceRolePermissions(preview);",
      'await replaceRolePermissions(preview);\n      await updateAdminResource("roles", authorization.role.id, {} as never);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not be followed by a second generic role mutation/);
  });

  it("rejects a new user form that implicitly selects organization context", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      'record ? values : { ...values, tenantCode: "", orgUnitCode: undefined, roles: [] }',
      'record ? values : { ...values, tenantCode: "tenant-default", orgUnitCode: "org-default", roles: ["role-default"] }',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /New organization-scoped users must start without an implicitly selected organization/);
  });

  it("rejects form initialization that reruns when async options change", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "if (initializedFormKeyRef.current === initializationKey)",
      "if (false && initializedFormKeyRef.current === initializationKey)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Async relation and organization context updates must not reset an already initialized form/);
  });

  it("rejects form initialization that bypasses resource experience defaults", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "form.setFieldsValue(activeFormInitialValues)",
      "form.setFieldsValue(defaultFormValues(formFields))",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Form initialization must apply the active resource experience initial values/);
  });

  it("rejects initial value calculation that bypasses the resource experience", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "return experience.initialValues?.(values, editingRecord) ?? values;",
      "return values;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource forms must apply experience-owned initial values/);
  });

  it("rejects a form initialization guard that never records its lifecycle key", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "initializedFormKeyRef.current = initializationKey;",
      "void initializationKey;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Form initialization must record the active modal lifecycle/);
  });

  it("rejects a closed modal that retains its initialization key", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'initializedFormKeyRef.current = "";',
      "void initializedFormKeyRef.current;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Closing a form must clear its initialization key/);
  });

  it("rejects an editable tenant field for organization-scoped users", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      '<Input readOnly aria-readonly="true" placeholder={dictionary.userDerivedTenantPending} />',
      '<Input placeholder={dictionary.userDerivedTenantPending} />',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Derived user tenant must remain visibly and semantically read-only/);
  });

  it("rejects a role selector enabled before organization selection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "disabled={!selectedOrgUnitCode || rolePoolLoading}",
      "disabled={rolePoolLoading}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /User roles must remain disabled until an organization is selected/);
  });

  it("rejects organization role-pool feedback without a polite live region", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      '<div aria-live="polite" id="organization-role-pool-status"',
      '<div aria-live="off" id="organization-role-pool-status"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role-pool status changes must use a polite live region/);
  });

  it("rejects organization role-group bindings in generic CRUD input", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      'values: omitValue(context.input.values, "roleGroupCodes")',
      "values: context.input.values",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic organization CRUD must not carry role-group bindings/);
  });

  it("rejects organization context loading that stops after the first generic-resource page", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "records.length >= result.total",
      "currentPage >= 1",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /context pagination must stop only after the full result set is loaded/);
  });

  it("rejects a generic metadata write after organization role-group apply", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "await replaceOrganizationRoleGroups(preview);\n  return recordWithValues(context.editingRecord, { roleGroupCodes: selectedGroups.join(\",\") });",
      "await replaceOrganizationRoleGroups(preview);\n  return context.persist(input);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Organization role-group apply must not be followed by a second generic metadata write/);
  });

  it("rejects static organization authorization confirmations outside the AntD application context", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "const { modal } = App.useApp();",
      "const modal = { confirm: () => undefined };",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /confirmations must use the active AntD application context/);
  });

  for (const [name, source, replacement, expected] of [
    ["delete actions", "allowDelete: !active", "allowDelete: true", /generic delete actions must stay disabled/],
    ["status toggles", "allowStatusToggle: !active", "allowStatusToggle: true", /generic status toggles must stay disabled/],
  ]) {
    it(`rejects organization and user ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(
        tempRoot,
        "admin/src/platform/resources/organizationUserExperience.tsx",
        source,
        replacement,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, expected);
    });
  }

  it("rejects optional personnel copy that is labeled as the organization capability", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/metadata.ts",
      "人员与岗位",
      "人员组织",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Optional personnel capability must not be labeled as the organization capability/);
  });

  it("rejects a table fork that drops the shared pagination bar", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "PlatformPaginationBar",
      "LocalPaginationBar",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must use the shared pagination bar/);
  });

  it("rejects table column settings without AntD breakpoint state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "Grid.useBreakpoint()",
      "{}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must use AntD breakpoint state for rendered-column clarity/);
  });

  it("rejects table column settings without the current-width hidden state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "labels.hiddenAtCurrentWidth",
      "labels.columnUnavailable",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must explain breakpoint-hidden selected columns/);
  });

  it("rejects a breakpoint-hidden column checkbox without the hidden state in its accessible name", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      /aria-label=\{(?:String\(column\.title\)|checkboxAccessibleLabel)\}/,
      "aria-label={columnLabel}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must include the breakpoint-hidden state in each hidden checkbox accessible name/);
  });

  it("rejects a truncated column label without its full plain-text title", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      /<span className="platform-column-option-label"(?: title=\{columnLabel\})?>/,
      '<span className="platform-column-option-label">',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must expose each full plain-text label through the option title attribute/);
  });

  it("rejects responsive column matching that requires every breakpoint", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      ".some((breakpoint) => screens[breakpoint])",
      ".every((breakpoint) => screens[breakpoint])",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must treat a column as rendered when any responsive breakpoint is active/);
  });

  it("rejects a settings drawer that drops import/export configuration support", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "exportConfig",
      "downloadConfig",
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "importConfig",
      "uploadConfig",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /SystemSettingsDrawer must keep exportConfig support/);
    assert.match(result.stderr, /SystemSettingsDrawer must keep importConfig support/);
  });

  it("normalizes legacy and invalid watermark preferences", () => {
    const result = runTypeScriptProbe("admin/src/platform/ui/adminUIConfig.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      const { defaultAdminUIConfig, normalizeAdminUIConfig } = await import(${JSON.stringify(moduleURL)});
      const legacy = normalizeAdminUIConfig({ watermark: true });
      assert.equal(legacy.watermark, true);
      assert.equal(legacy.watermarkCount, 1);
      assert.deepEqual(legacy.watermarkScopes, ["screen"]);
      const invalid = normalizeAdminUIConfig({
        density: "comfortable",
        watermark: true,
        watermarkCount: 7,
        watermarkScopes: ["screen", "unknown", "export", "screen"],
      });
      assert.equal(invalid.density, "comfortable");
      assert.equal(invalid.watermarkCount, defaultAdminUIConfig.watermarkCount);
      assert.deepEqual(invalid.watermarkScopes, ["screen", "export"]);
      assert.deepEqual(normalizeAdminUIConfig({ watermark: true, watermarkScopes: [] }).watermarkScopes, []);
    `);

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });

  it("keeps accessible watermark scope controls and exact grid rendering", () => {
    const settings = fs.readFileSync(path.join(repoRoot, "admin/src/platform/ui/SystemSettingsDrawer.tsx"), "utf8");
    const shell = fs.readFileSync(path.join(repoRoot, "admin/src/platform/shell/AdminShell.tsx"), "utf8");
    const styles = fs.readFileSync(path.join(repoRoot, "admin/src/styles.css"), "utf8");

    assert.match(settings, /Checkbox\.Group/);
    assert.match(settings, /<Switch aria-label=\{label\}/);
    assert.match(settings, /className="settings-switch-hit-target"/);
    assert.match(settings, /role="group"/);
    assert.match(settings, /watermarkScopes/);
    assert.match(settings, /watermarkCount/);
    assert.match(settings, /options=\{watermarkCounts\}/);
    assert.match(shell, /className="platform-watermark-layer"/);
    assert.match(shell, /aria-hidden="true"/);
    assert.match(shell, /Array\.from\(\{ length: uiConfig\.watermarkCount \}/);
    assert.match(styles, /\.platform-watermark-layer/);
    assert.match(styles, /grid-template-columns/);
    assert.match(styles, /min-height:\s*44px/);
    assert.match(styles, /\.settings-switch-hit-target\s*\{[\s\S]*?min-height:\s*44px[\s\S]*?min-width:\s*44px/);
  });

  it("rejects a screen watermark nested under the scrollable content region", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "      {showScreenWatermark ? (",
      '      <section className="platform-content">\n        {showScreenWatermark ? (',
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      '      ) : null}\n      <aside className="platform-sider"',
      '        ) : null}\n      </section>\n      <aside className="platform-sider"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Screen watermark must be mounted directly under \.platform-shell/);
    assert.match(result.stderr, /Screen watermark must not be nested under \.platform-content/);
  });

  for (const [name, declaration, replacement] of [
    ["fixed viewport positioning", "position: fixed;", "position: absolute;"],
    ["full viewport inset", "inset: 0;", "inset: auto;"],
    ["overlay stacking", "z-index: 2200;", "z-index: 1100;"],
    ["non-interactive pointer handling", "pointer-events: none;", "pointer-events: auto;"],
  ]) {
    it(`rejects a screen watermark without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceRegexInTemp(
        tempRoot,
        "admin/src/styles.css",
        new RegExp(`(\\.platform-watermark-layer\\s*\\{[\\s\\S]*?)${declaration.replace(/[.*+?^${}()|[\\]\\\\]/g, "\\\\$&")}`),
        `$1${replacement}`,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /Screen watermark must remain a fixed, full-viewport, non-interactive overlay above the Admin shell/);
    });
  }

  it("rejects a multi-watermark layout that leaves the topbar visually blank", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      '.platform-watermark-layer[data-count="16"] span:nth-child(-n + 4)',
      '.platform-watermark-layer[data-count="16"] span:nth-child(-n + 0)',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /first row against the viewport edge/);
  });

  it("rejects a sixteen-watermark layout that stays four columns on narrow viewports", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      'grid-template-columns: repeat(2, minmax(0, 1fr));\n    grid-template-rows: repeat(8, minmax(0, 1fr));',
      'grid-template-columns: repeat(4, minmax(0, 1fr));\n    grid-template-rows: repeat(4, minmax(0, 1fr));',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reflow sixteen watermarks to two columns/);
  });

  it("rejects reveal callbacks that can mount the login OIDC consumer", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/App.tsx", "if (sensitiveRevealOIDCCallbackPending) {", "if (false) {");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reveal callbacks must remain isolated from the login OIDC callback consumer/);
  });

  it("rejects sensitive reveal actions outside manifest-governed detail fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "field.inDetail && field.reveal && permissionAllows(permissions, field.reveal.permission, deniedPermissions)",
      "field.reveal && permissionAllows(permissions, field.reveal.permission, deniedPermissions)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Sensitive reveal actions must require detail visibility, manifest declaration, and the declared permission/);
  });

  it("rejects reveal responses that can restore plaintext after close or page hide", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/SensitiveFieldRevealModal.tsx",
      "if (operationGenerationRef.current !== generation) return;",
      "void generation;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reveal responses must be discarded after close, hide, or target changes/);
  });

  it("rejects plaintext copy that bypasses field or policy configuration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/SensitiveFieldRevealModal.tsx",
      "result.copyAllowed && policy?.copyAllowed && field.reveal?.copyAllowed",
      "result.copyAllowed",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Sensitive plaintext copy must require the response, policy, and field contract to allow it/);
  });

  it("rejects OIDC resume without off-page detail hydration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "provider.getOne<AdminResourceRecord>",
      "provider.getMany<AdminResourceRecord>",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /OIDC resume must hydrate a detail record that is outside the current list page/);
  });

  it("passes the shared export watermark intent to policy review JSON exports", () => {
    const client = fs.readFileSync(path.join(repoRoot, "admin/src/platform/api/client.ts"), "utf8");
    const app = fs.readFileSync(path.join(repoRoot, "admin/src/App.tsx"), "utf8");
    const policyReview = fs.readFileSync(path.join(repoRoot, "admin/src/platform/policy-review/PolicyReviewConsole.tsx"), "utf8");

    assert.match(client, /export function exportAdminPolicyReviews\(\{ watermark \}: \{ watermark: boolean \}\)/);
    assert.match(client, /policy-reviews\/export\?watermark=\$\{watermark\}/);
    assert.match(app, /uiConfig\.watermark && uiConfig\.watermarkScopes\.includes\("export"\)/);
    assert.match(policyReview, /exportAdminPolicyReviews\(\{ watermark: exportWatermark \}\)/);
  });

  it("rejects a policy review export control that ignores the dedicated permission", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/policy-review/PolicyReviewConsole.tsx",
      'permissionAllows(permissions, "admin:policy-review:export", deniedPermissions)',
      'permissionAllows(permissions, "admin:policy-review:read", deniedPermissions)',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PolicyReviewConsole must derive export access from the dedicated permission/);
  });

  it("rejects a shell whose skip link does not target main content", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      'href="#platform-main-content"',
      'href="#missing-content"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminShell must expose a skip-to-content link/);
  });

  it("rejects mobile work-tab close handling that leaves the context panel open", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "const handleMobileWorkTabClose = (route: string) => {\n    setOpenContext(null);\n    closeWorkTab(route);\n  };",
      "const handleMobileWorkTabClose = (route: string) => {\n    closeWorkTab(route);\n  };",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Mobile work-tab close must dismiss its context panel before closing the tab/);
  });

  it("rejects mobile work-tab close controls that bypass the context-closing handler", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "onClick={() => handleMobileWorkTabClose(resource.route)}",
      "onClick={() => closeWorkTab(resource.route)}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Mobile work-tab close controls must use the context-closing handler/);
  });

  it("rejects a client that renames the session-expired event contract", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "ADMIN_SESSION_EXPIRED_EVENT",
      "ADMIN_SESSION_INVALID_EVENT",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /The shared client must expose the session-expired event contract/);
  });

  it("rejects session expiry handling that does not match the exact request token", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/sessionExpiry.ts",
      "currentToken === requestToken",
      "Boolean(currentToken)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Session expiry must clear only the exact token used by the failed request/);
  });

  it("keeps the Admin session after sensitive reveal verification fails", () => {
    const result = runTypeScriptProbe("admin/src/platform/api/sessionExpiry.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { shouldExpireAdminSession } from ${JSON.stringify(moduleURL)};

      assert.equal(shouldExpireAdminSession({
        statusCode: 422,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED",
      }), false);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED",
      }), false);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SESSION_INVALID",
      }), true);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "stale-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SESSION_INVALID",
      }), false);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects session expiry that mounts login before reveal callback URL cleanup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "      setSensitiveRevealOIDCResume(null);\n      clearPendingSensitiveRevealOIDC();",
      "      setSensitiveRevealOIDCResume(null);\n      setSensitiveRevealOIDCCallbackPending(false);\n      clearPendingSensitiveRevealOIDC();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reveal callback cleanup remove code and state/);
  });

  it("rejects auth bootstrap calls that use stored-token authentication", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      'auth: "none"',
      'auth: "stored-token"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Auth provider discovery must explicitly avoid stored-token authentication/);
    assert.match(result.stderr, /Auth login must explicitly avoid stored-token authentication/);
  });

  for (const [name, relativePath, from, to, message] of [
    ["provider audiences", "admin/src/platform/api/client.ts", "audiences: string[];", "audienceList: string[];", "AuthProvider must expose its declared audiences"],
    ["Web Crypto verifier", "admin/src/platform/refine/authProvider.ts", "crypto.getRandomValues(new Uint8Array(size))", "new Uint8Array(size)", "generate verifier bytes with Web Crypto"],
    ["S256 challenge", "admin/src/platform/refine/authProvider.ts", 'crypto.subtle.digest("SHA-256"', 'legacyDigest("SHA-256"', "derive an S256 challenge with Web Crypto"],
    ["tab-scoped transaction", "admin/src/platform/refine/authProvider.ts", "window.sessionStorage.setItem", "window.localStorage.setItem", "tab-scoped sessionStorage"],
    ["exact callback state", "admin/src/platform/auth/oidcPolicy.ts", "callbackState !== pending.state", "!callbackState", "exact comparison"],
    ["callback URL cleanup", "admin/src/platform/refine/authProvider.ts", "window.history.replaceState", "window.history.pushState", "remove callback values from browser history before exchange"],
    ["demo-only form", "admin/src/platform/auth/AdminLoginView.tsx", 'selectedProvider.kind === "demo"', 'selectedProvider.kind !== "unknown"', "username form must render only for the demo provider"],
    ["OIDC-only action", "admin/src/platform/auth/AdminLoginView.tsx", 'selectedProvider.kind === "oidc"', 'selectedProvider.kind !== "unknown"', "OIDC action must render only for an OIDC provider"],
    ["OIDC action width hook", "admin/src/platform/auth/AdminLoginView.tsx", 'className="login-oidc-action"', 'className="login-provider-action"', "one full-width login action"],
    ["callback live region", "admin/src/platform/auth/AdminLoginView.tsx", 'aria-live="polite"', 'aria-live="off"', "polite live region"],
    ["error focus", "admin/src/platform/auth/AdminLoginView.tsx", "focus({ preventScroll: true })", "focus()", "focus its heading without a scroll jump"],
    ["recovery action", "admin/src/platform/auth/AdminLoginView.tsx", 'className="login-recovery-action"', 'className="login-reset-action"', "explicit recovery action"],
    ["login reduced motion", "admin/src/styles.css", ".login-page *", ".login-motion-uncovered *", "suppress non-essential login transitions"],
    ["mobile login target", "admin/src/styles.css", ".login-submit,\n  .login-oidc-action,\n  .login-recovery-action", ".login-actions-missing", "Mobile login submit, OIDC, and recovery actions must expose 44px touch targets"],
    ["tablet login target", "admin/src/styles.css", "@media (max-width: 1024px)", "@media (max-width: 1023px)", "Tablet login submit, OIDC, and recovery actions must expose 44px touch targets"],
  ]) {
    it(`rejects Task 6 without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTempIfPresent(tempRoot, relativePath, from, to);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }

  it("executes the Admin audience and authorization URL policy", () => {
    const result = runTypeScriptProbe(
      "admin/src/platform/auth/oidcPolicy.ts",
      (moduleURL) => `
        import assert from "node:assert/strict";
        import {
          assertAdminAuthProvider,
          beginOIDCLoginTransaction,
          consumePendingOIDCLoginTransaction,
          createSingleUseGuard,
          createSubmissionLock,
          filterAdminAuthProviders,
          OIDCCallbackError,
          validateOIDCAuthorizationURL,
        } from ${JSON.stringify(moduleURL)};

        const adminProvider = { id: "oidc", audiences: ["admin"] };
        const appProvider = { id: "wechat", audiences: ["app"] };
        assert.deepEqual(filterAdminAuthProviders([appProvider, adminProvider]), [adminProvider]);
        assert.doesNotThrow(() => assertAdminAuthProvider(adminProvider));
        assert.throws(() => assertAdminAuthProvider(appProvider), /not available for Admin login/);

        for (const url of ["https://id.example/authorize"]) {
          assert.equal(validateOIDCAuthorizationURL(url), new URL(url).toString());
        }

        for (const url of [
          "http://localhost:8080/authorize",
          "http://auth.localhost:8080/authorize",
          "http://127.0.0.1:8080/authorize",
          "http://127.25.3.9:8080/authorize",
          "http://[::1]:8080/authorize",
        ]) {
          assert.throws(() => validateOIDCAuthorizationURL(url), /OIDC authorization URL is not trusted/);
          assert.equal(validateOIDCAuthorizationURL(url, { allowLoopbackHTTP: true }), new URL(url).toString());
        }

        for (const url of [
          "not a URL",
          "javascript:alert(1)",
          "data:text/html,unsafe",
          "http://id.example/authorize",
          "ftp://127.0.0.1/authorize",
        ]) {
          assert.throws(
            () => validateOIDCAuthorizationURL(url),
            (error) => error instanceof Error && error.message === "OIDC authorization URL is not trusted" && !error.message.includes(url),
          );
        }

        const submissionLock = createSubmissionLock();
        assert.equal(submissionLock.acquire(), true);
        assert.equal(submissionLock.acquire(), false);
        submissionLock.release();
        assert.equal(submissionLock.acquire(), true);

        const onceGuard = createSingleUseGuard();
        assert.equal(onceGuard.acquire(), true);
        assert.equal(onceGuard.acquire(), false);

        const beginEvents = [];
        let storedTransaction = "";
        await beginOIDCLoginTransaction(adminProvider, {
          allowLoopbackHTTP: false,
          randomBytes: (size) => { beginEvents.push("random"); return new Uint8Array(size).fill(1); },
          digestSHA256: async (input) => { beginEvents.push("digest"); assert.equal(input.length, 43); return new Uint8Array(32).fill(2); },
          startProvider: async (provider, challenge) => {
            beginEvents.push("start");
            assert.equal(provider, "oidc");
            assert.match(challenge, /^[A-Za-z0-9_-]{43}$/);
            return { authorizationUrl: "https://id.example/authorize", state: "state-exact", expiresAt: "2030-01-01T00:00:00.000Z" };
          },
          storePending: (value) => { beginEvents.push("store"); storedTransaction = value; },
          navigate: (url) => { beginEvents.push("navigate"); assert.equal(url, "https://id.example/authorize"); },
        });
        assert.deepEqual(beginEvents, ["random", "digest", "start", "store", "navigate"]);
        const stored = JSON.parse(storedTransaction);
        assert.deepEqual(Object.keys(stored).sort(), ["codeVerifier", "expiresAt", "provider", "state"]);
        assert.equal(stored.provider, "oidc");
        assert.equal(stored.state, "state-exact");

        const rejectedBeginEvents = [];
        await assert.rejects(
          beginOIDCLoginTransaction(adminProvider, {
            allowLoopbackHTTP: false,
            randomBytes: (size) => new Uint8Array(size),
            digestSHA256: async () => new Uint8Array(32),
            startProvider: async () => ({ authorizationUrl: "http://127.0.0.1:8080/authorize", state: "state", expiresAt: "2030-01-01T00:00:00.000Z" }),
            storePending: () => rejectedBeginEvents.push("store"),
            navigate: () => rejectedBeginEvents.push("navigate"),
          }),
          /OIDC authorization URL is not trusted/,
        );
        assert.deepEqual(rejectedBeginEvents, []);

        const pending = JSON.stringify({ provider: "oidc", state: "state-exact", codeVerifier: "verifier", expiresAt: "2030-01-01T00:00:00.000Z" });
        const makeConsumeDependencies = ({ raw = pending, now = Date.parse("2029-01-01T00:00:00.000Z"), exchangeError = null } = {}) => {
          const events = [];
          const exchanges = [];
          return {
            events,
            exchanges,
            dependencies: {
              cleanupURL: () => events.push("cleanup"),
              readPending: () => { events.push("read"); return raw; },
              removePending: () => events.push("remove"),
              now: () => now,
              exchange: async (input) => {
                events.push("exchange");
                exchanges.push(input);
                if (exchangeError) throw exchangeError;
                return { principal: { user: { id: "admin" } } };
              },
            },
          };
        };

        const noCallback = makeConsumeDependencies();
        assert.equal(await consumePendingOIDCLoginTransaction("", noCallback.dependencies), null);
        assert.deepEqual(noCallback.events, []);

        for (const scenario of [
          { name: "empty code and state", search: "?code=&state=" },
          { name: "empty provider error", search: "?error=" },
          { name: "empty provider error with code and state", search: "?code=x&state=y&error=" },
        ]) {
          const current = makeConsumeDependencies();
          await assert.rejects(
            consumePendingOIDCLoginTransaction(scenario.search, current.dependencies),
            (error) => error instanceof OIDCCallbackError && error.reason === "callback" && error.message === "callback",
            scenario.name,
          );
          assert.deepEqual(current.events, ["cleanup", "read", "remove"], scenario.name);
          assert.deepEqual(current.exchanges, [], scenario.name);
        }

        for (const scenario of [
          { name: "missing code", search: "?state=state-exact" },
          { name: "malformed transaction", search: "?code=code&state=state-exact", raw: "not-json" },
          { name: "state mismatch", search: "?code=code&state=wrong" },
          { name: "expired", search: "?code=code&state=state-exact", now: Date.parse("2031-01-01T00:00:00.000Z") },
        ]) {
          const current = makeConsumeDependencies(scenario);
          await assert.rejects(consumePendingOIDCLoginTransaction(scenario.search, current.dependencies));
          assert.deepEqual(current.events.slice(0, 3), ["cleanup", "read", "remove"], scenario.name);
          assert.equal(current.events.includes("exchange"), false, scenario.name);
        }

        const success = makeConsumeDependencies();
        const successResult = await consumePendingOIDCLoginTransaction("?code=code-exact&state=state-exact", success.dependencies);
        assert.deepEqual(success.events, ["cleanup", "read", "remove", "exchange"]);
        assert.deepEqual(success.exchanges, [{ provider: "oidc", code: "code-exact", state: "state-exact", codeVerifier: "verifier" }]);
        assert.equal(successResult.principal.user.id, "admin");

        const exchangeFailure = makeConsumeDependencies({ exchangeError: new Error("normalized exchange failure") });
        await assert.rejects(consumePendingOIDCLoginTransaction("?code=code&state=state-exact", exchangeFailure.dependencies), /normalized exchange failure/);
        assert.deepEqual(exchangeFailure.events, ["cleanup", "read", "remove", "exchange"]);
      `,
    );

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });

  for (const [name, relativePath, from, to, message] of [
    ["Admin audience filtering", "admin/src/platform/auth/AdminLoginView.tsx", "filterAdminAuthProviders(providers)", "providers", "Admin login must consume provider audiences before selection and rendering"],
    ["Admin audience start guard", "admin/src/platform/auth/oidcPolicy.ts", "assertAdminAuthProvider(provider);", "void provider;", "OIDC start must reject providers without the Admin audience"],
    ["demo synchronous lock", "admin/src/platform/auth/AdminLoginView.tsx", "if (!submissionLockRef.current.acquire()) return;", "if (submitting) return;", "Demo login must acquire the synchronous submission lock"],
    ["OIDC synchronous lock", "admin/src/platform/auth/AdminLoginView.tsx", "if (!submissionLockRef.current.acquire()) return;", "if (submitting) return;", "OIDC start must acquire the synchronous submission lock"],
    ["authorization URL validation", "admin/src/platform/auth/oidcPolicy.ts", "validateOIDCAuthorizationURL(started.authorizationUrl,", "String(started.authorizationUrl,", "OIDC start must validate the authorization URL before browser navigation"],
    ["recovery focus", "admin/src/platform/auth/AdminLoginView.tsx", "loginHeadingRef.current?.focus({ preventScroll: true })", "void loginHeadingRef.current", "Explicit OIDC recovery must restore focus predictably without scrolling"],
    ["localized callback category", "admin/src/platform/auth/AdminLoginView.tsx", "setCallbackFailure(callbackFailureReason(nextError))", "setLoginError(callbackErrorMessage(dictionary, nextError))", "OIDC callback failures must store a stable error category"],
  ]) {
    it(`rejects Task 6 without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTempIfPresent(tempRoot, relativePath, from, to);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }

  it("rejects production wrappers that enable loopback HTTP outside verified development", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/refine/authProvider.ts",
      "allowLoopbackHTTP: import.meta.env.DEV",
      "allowLoopbackHTTP: true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Loopback HTTP authorization URLs must be enabled only in verified development mode/);
  });

  it("rejects callback cleanup after pending transaction access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/oidcPolicy.ts",
      "dependencies.cleanupURL();\n  const rawPending = dependencies.readPending();",
      "const rawPending = dependencies.readPending();\n  dependencies.cleanupURL();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /OIDC callbacks must remove callback values before reading pending transaction state/);
  });

  it("rejects OIDC rendering that restores the disabled password field", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/AdminLoginView.tsx",
      "</Form>",
      "<Input.Password disabled /></Form>",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not retain the disabled password field/);
  });

  it("rejects unauthenticated route normalization during OIDC callback cleanup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/App.tsx",
      "if (!getAuthToken() || !session || loading)",
      "if (loading)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Unauthenticated callback routes must not be normalized before OIDC URL cleanup/);
  });

  it("rejects unmatched Admin OIDC translations", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/i18n.ts",
      'loginOIDCRecovery: "Return to login providers",',
      "",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin login i18n key loginOIDCRecovery must exist in matching Chinese and English dictionaries/);
  });

  it("rejects localized session expiry state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "const [sessionExpired, setSessionExpired] = useState(false);",
      "const [sessionExpired, setSessionExpired] = useState(dictionary.sessionExpired);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /App must keep session expiry in stable non-localized state/);
  });

  it("rejects localized-string equality for session expiry", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      'setAuthError("");',
      'setAuthError((current) => current === dictionary.sessionExpired ? current : "");',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /App must not identify session expiry by comparing localized strings/);
  });

  it("rejects styles that remove reduced-motion support", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      "@media (prefers-reduced-motion: reduce)",
      "@media (prefers-reduced-motion: no-preference)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /styles\.css must respect reduced motion/);
  });

  it("rejects reduced-motion styles that omit body-portaled AntD modals", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".ant-modal-root",
      ".ant-modal-portal-root",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reduced motion must cover used body-portaled AntD motion roots/);
  });

  for (const [name, selector] of [
    ["tooltips", ".ant-tooltip"],
    ["select dropdowns", ".ant-select-dropdown"],
  ]) {
    it(`rejects reduced-motion styles that omit body-portaled AntD ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(
        tempRoot,
        "admin/src/styles.css",
        selector,
        `${selector}-motion-uncovered`,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /Reduced motion must cover used body-portaled AntD motion roots/);
    });
  }

  it("rejects resource modal focus handling outside the AntD open lifecycle", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "focusFirstEditableFormField(form, formFields);",
      "void form;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Resource modals must invoke the shared first-field focus helper/);
  });

  it("rejects encrypted resource filters that are not constrained to exact match", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "operator: isEncryptedExactMatchField(field)",
      "operator: field.type === \"text\"",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted resource filters must submit exact-match conditions/);
  });

  it("rejects query parsing that accepts non-equality operators for encrypted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'isEncryptedExactMatchField(field) && match[2] !== "="',
      "false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted resource query syntax must allow equality only/);
  });

  it("rejects encrypted edit hydration that reuses projected or default values", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'if (field.storageMode === "encrypted") {\n    return undefined;\n  }',
      'if (field.storageMode === "encrypted") {\n    return defaultFieldValue(field);\n  }',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must hydrate blank/);
  });

  it("rejects encrypted edit fields that require resubmitting the current secret", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'field.required && !(editing && field.storageMode === "encrypted")',
      "field.required",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must allow a blank value/);
  });

  it("rejects encrypted edit fields without localized preserve-value guidance", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "parts.push(dictionary.encryptedFieldEditHint);",
      "void dictionary.encryptedFieldEditHint;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must expose the localized blank-preserves-current-value hint/);
  });

  it("rejects an incomplete encrypted edit hint dictionary", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/i18n.ts",
      'encryptedFieldEditHint: "This field is encrypted. Leave it blank while editing to keep the current value.",',
      'encryptedEditHint: "This field is encrypted. Leave it blank while editing to keep the current value.",',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit field guidance must exist in matching Chinese and English dictionaries/);
  });

  it("rejects status updates that can resubmit encrypted, hidden or non-writable values", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      '.filter((field) => field.source === "values" && !field.readOnly && field.sensitivity === "public" && field.storageMode !== "encrypted" && field.responseMode !== "omitted" && field.responseMode !== "privileged")',
      '.filter((field) => field.source === "values" && field.storageMode !== "encrypted")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Status updates must exclude encrypted, hidden and non-writable values/);
  });

  it("rejects status updates that bypass schema-aware filtering", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "inputFromRecord(record, schema.fields, { status: nextStatus })",
      "inputFromRecord(record, [], { status: nextStatus })",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Status updates must use schema-aware record input filtering/);
  });

  it("rejects generic resource tables that render omitted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'schema.fields.filter((field) => field.inTable && field.responseMode !== "omitted")',
      "schema.fields.filter((field) => field.inTable)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource tables must not render omitted response fields/);
  });

  it("rejects generic resource details that render omitted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'schema.fields.filter((field) => field.inDetail && field.responseMode !== "omitted")',
      "schema.fields.filter((field) => field.inDetail)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource details must not render omitted response fields/);
  });

  it("rejects an incomplete TypeScript masking contract", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "masking?: AdminResourceFieldMasking;",
      "masking?: unknown;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminResourceField must carry optional masking metadata/);
  });

  it("rejects TypeScript masking metadata that drops a supported strategy", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      'strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1" | "address-cn-v1";',
      'strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1";',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin field masking metadata must expose every supported versioned strategy/);
  });

  it("rejects resource modal fallback focus without current-visible-modal scoping", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "modal.getClientRects().length > 0",
      "true",
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
      "document.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /currently visible modal/);
    assert.match(result.stderr, /stay scoped to the visible modal/);
  });

  it("rejects resource modal fallback focus that includes disabled or read-only fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'input:not([type="hidden"]):not([disabled]):not([readonly])',
      'input:not([type="hidden"])',
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      '[tabindex]:not([tabindex="-1"]):not([disabled]):not([readonly]):not([aria-disabled="true"])',
      '[tabindex]:not([tabindex="-1"])',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /first enabled editable form control/);
    assert.match(result.stderr, /exclude disabled generic focus targets/);
  });

  it("rejects resource modal fallback focus without preventScroll", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "fieldControl?.focus({ preventScroll: true });",
      "fieldControl?.focus();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /fallback focus must prevent scroll jumps/);
  });

  it("rejects resource modal fallback focus that restores a global field-id lookup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
      "document.getElementById(firstField.key)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not depend on a global field id/);
  });

  for (const [name, selector, message] of [
    ["resource search", ".platform-data-table-panel .platform-table-search {", "Mobile resource search must expose a 44px touch target"],
    ["resource toolbar", ".platform-data-table-panel .table-actions .ant-btn {", "Mobile resource table actions must expose 44px touch targets"],
    ["pagination main controls", ".platform-pagination-main :where(.ant-pagination-prev, .ant-pagination-item, .ant-pagination-jump-prev, .ant-pagination-jump-next, .ant-pagination-next, .ant-pagination-item-link, a) {", "Mobile pagination main controls must expose 44px touch targets"],
    ["pagination quick jumper", ".platform-pagination-jumper .ant-input-number {", "Mobile pagination quick jumper must expose a 44px touch target"],
    ["settings close control", ".system-settings-drawer .ant-drawer-close {", "Mobile settings Drawer close control must expose a 44px touch target"],
    ["settings tab", ".settings-tabs .ant-tabs-tab {", "Mobile settings Drawer tab must expose a 44px touch target"],
    ["settings tab button", ".settings-tabs .ant-tabs-tab-btn {", "Mobile settings Drawer tab button must expose a 44px touch target"],
    ["settings overflow control", ".settings-tabs .ant-tabs-nav-more {", "Mobile settings Drawer overflow control must expose a 44px touch target"],
  ]) {
    it(`rejects mobile styles that shrink the ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(tempRoot, "admin/src/styles.css", selector, `${selector.slice(0, -1)}.missing {`);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }
});
