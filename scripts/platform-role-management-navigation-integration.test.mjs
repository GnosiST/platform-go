import assert from "node:assert/strict";
import fs from "node:fs";
import { describe, it } from "node:test";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

function source(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

describe("role management navigation integration", () => {
  const app = source("admin/src/App.tsx");
  const dashboard = source("admin/src/platform/dashboard/DashboardHome.tsx");
  const i18n = source("admin/src/platform/i18n.ts");
  const resourceRoute = source("admin/src/platform/refine/ResourceRoutePage.tsx");
  const roleConsole = source("admin/src/platform/resources/RoleGovernanceConsole.tsx");
  const shell = source("admin/src/platform/shell/AdminShell.tsx");

  it("projects only the shell navigation while preserving the complete routing resource contract", () => {
    assert.match(app, /projectRoleManagementNavigation/);
    assert.match(app, /resolveRoleManagementActiveRoute/);
    assert.match(app, /const navigationResources = useMemo\([\s\S]*?projectRoleManagementNavigation\(resources,/);
    assert.match(app, /const navigationActiveRoute = resolveRoleManagementActiveRoute\(activeRoute, navigationResources\);/);
    assert.match(app, /<AdminShell[\s\S]*?resources=\{navigationResources\}[\s\S]*?activeRoute=\{navigationActiveRoute\}/);

    assert.match(app, /const refineResources = useMemo\(\(\) => resources\.map\(resourceDefinitionToRefineResource\), \[resources\]\);/);
    assert.match(app, /<PlatformRoutePages[\s\S]*?activeRoute=\{activeRoute\}[\s\S]*?resources=\{resources\}/);
    assert.match(app, /const resourceRoutes = resources\.filter/);
    assert.match(resourceRoute, /resource\.route === "\/roles" \|\| resource\.route === "\/role-groups"/);
  });

  it("keeps every shell-owned visible navigation surface on its projected resources prop", () => {
    assert.match(shell, /new Map\(resources\.map/);
    assert.match(shell, /resources\.filter\(\(resource\) => resource\.group === group\)/);
    assert.match(shell, /return resources\.filter\(\(resource\) =>/);
    assert.match(shell, /buildNavigationTree\(group\.resources,/);
  });

  it("uses the projection target and shared label for the dashboard role-management action", () => {
    assert.match(dashboard, /projectRoleManagementNavigation\(resources,/);
    assert.match(dashboard, /const roleManagementResource = selectRoleManagementNavigationResource\(/);
    assert.match(dashboard, /route: roleManagementResource\.route/);
    assert.match(dashboard, /label: dictionary\.roleManagement/);
    assert.doesNotMatch(dashboard, /\.find\(\(resource\) => resource\.route === "\/roles"/);
    assert.doesNotMatch(dashboard, /\{ key: "roles", label: dictionary\.roles, route: "\/roles"/);
  });

  it("declares concise bilingual navigation labels and keeps Role Management as the shared page H1", () => {
    assert.equal((i18n.match(/roleManagement: "角色"/g) ?? []).length, 1);
    assert.equal((i18n.match(/roleManagement: "Roles"/g) ?? []).length, 1);
    assert.match(i18n, /roleGovernanceTitle: "角色管理"/);
    assert.match(i18n, /roleGovernanceTitle: "Role Management"/);
    assert.match(roleConsole, /<AdminPage title=\{dictionary\.roleGovernanceTitle\} description=\{dictionary\.roleGovernanceDescription\}>/);
  });
});
