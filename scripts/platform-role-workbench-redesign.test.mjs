import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

const root = path.resolve(import.meta.dirname, "..");

function source(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

describe("role workbench redesign contracts", () => {
  it("projects explicit selected and hierarchy semantics into tree data", () => {
    const workbench = source("admin/src/platform/ui/AdminTreeWorkbench.tsx");

    assert.match(workbench, /workbenchTreeData\(nodes, selectedKey\)/);
    assert.match(workbench, /"aria-selected": node\.key === selectedKey/);
    assert.match(workbench, /"aria-level": depth/);
    assert.match(workbench, /children\.map\(\(child\) => build\(child, depth \+ 1\)\)/);
    assert.match(workbench, /title=\{typeof node\.label === "string" \? node\.label : node\.searchText\}/);
  });

  it("keeps role detail localized, focused and split by responsibility", () => {
    const role = source("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(role, /className="role-governance-detail-focus-target" tabIndex=\{-1\}/);
    assert.match(role, /<Typography\.Title level=\{4\}>/);
    assert.match(role, /column=\{\{ xs: 1, md: 2 \}\}/);
    assert.match(role, /roleStatusLabel\(record\.status, dictionary\)/);
    assert.match(role, /roleGroupScopeLabel\(valueOf\(record, "scopeType"\), dictionary\)/);
    assert.match(role, /roleDataScopeLabel\(valueOf\(record, "dataScope"\), dictionary\)/);
    assert.match(role, /className="role-governance-access-control"/);
    assert.match(role, /className="role-governance-lifecycle"/);
    assert.doesNotMatch(role, /role-governance-command-bar/);
  });

  it("makes menu assignment editable only for enabled authorized roles after cutover", () => {
    const role = source("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(role, /roleMenuMigrationWriteEnabled && canAssignMenus && record\.status === "enabled"/);
    assert.match(role, /roleMenuMigrationWriteEnabled && canAssignMenus && menuAssignment\.role\.status === "enabled"/);
    assert.match(role, /canEditMenus \? dictionary\.assignMenus : dictionary\.viewMenus/);
    assert.match(role, /if \(role\.status !== "enabled"\) return dictionary\.roleMenuReadonlyDisabledDescription/);
    assert.match(role, /if \(!canAssignMenus\) return dictionary\.roleMenuReadonlyAccessDescription/);
    assert.match(role, /if \(!roleMenuMigrationWriteEnabled\) return dictionary\.roleMenuLegacyReadonlyDescription/);
    assert.match(role, /menuAssignment\.role\.status !== "enabled"\) return/);
  });

  it("uses the platform modal boundary and removes dead Tree Transfer state", () => {
    const primitives = source("admin/src/platform/ui/AdminPrimitives.tsx");
    const role = source("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const transfer = source("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(primitives, /export function AdminModal/);
    assert.match(primitives, /return <AdminModal className=\{cx\("admin-form-modal"/);
    assert.match(role, /<AdminModal/);
    assert.doesNotMatch(role, /<Modal/);
    assert.doesNotMatch(transfer, /visibleCheckedKeys/);
  });

  it("keeps desktop tracks bounded and mobile transfer controls stable", () => {
    const styles = source("admin/src/styles.css");

    assert.match(styles, /grid-template-columns: clamp\(264px, 28vw, 320px\) minmax\(0, 1fr\);/);
    assert.match(styles, /\.role-governance-detail-focus-target,[\s\S]*?min-height: 360px;/);
    assert.match(styles, /\.admin-tree-workbench-node-label,[\s\S]*?text-overflow: ellipsis;[\s\S]*?white-space: nowrap;/);
    assert.match(styles, /@media \(min-width: 1024px\)[\s\S]*?\.admin-tree-workbench-detail[\s\S]*?position: sticky;/);
    assert.match(styles, /@media screen and \(max-width: 767px\)[\s\S]*?\.platform-tree-transfer-toolbar[\s\S]*?position: sticky;[\s\S]*?grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/);
    assert.match(styles, /\.platform-tree-transfer-toolbar \.ant-input-affix-wrapper[\s\S]*?grid-column: 1 \/ -1;[\s\S]*?width: 100%;/);
    assert.match(styles, /\.platform-tree-transfer-toolbar \.ant-space[\s\S]*?grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/);
  });

  it("declares every new role workbench label in both dictionaries", () => {
    const i18n = source("admin/src/platform/i18n.ts");
    for (const key of [
      "viewMenus",
      "roleAccessControl",
      "roleLifecycle",
      "roleMenuReadonlyTitle",
      "roleMenuReadonlyAccessDescription",
      "roleMenuReadonlyDisabledDescription",
    ]) {
      assert.equal((i18n.match(new RegExp(`${key}:`, "g")) ?? []).length, 2, key);
    }
  });
});
