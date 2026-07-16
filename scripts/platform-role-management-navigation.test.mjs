import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const projectionURL = pathToFileURL(path.join(root, "admin/src/platform/resources/roleManagementNavigation.ts")).href;

function runProjection(testBody) {
  const body = `
    import assert from "node:assert/strict";
    import {
      projectRoleManagementNavigation,
      resolveRoleManagementActiveRoute,
    } from ${JSON.stringify(projectionURL)};

    const title = { zh: "角色管理", en: "Role Management" };
    const resource = (route, name = route.slice(1)) => ({
      route,
      name,
      title: { zh: name, en: name },
      marker: \`marker:\${name}\`,
    });

    ${testBody}
  `;
  const result = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module"], {
    input: body,
    encoding: "utf8",
  });
  assert.equal(result.status, 0, result.error?.message || result.stderr || result.stdout || `signal=${result.signal}`);
}

describe("role management navigation projection", () => {
  it("collapses both role entries at their first position and selects roles", () => {
    runProjection(`
      const input = [
        resource("/overview"),
        resource("/role-groups", "roleGroups"),
        resource("/audit-logs"),
        resource("/roles"),
        resource("/settings"),
      ];
      const projected = projectRoleManagementNavigation(input, title);

      assert.deepEqual(projected.map((item) => item.route), ["/overview", "/roles", "/audit-logs", "/settings"]);
      assert.deepEqual(projected[1].title, title);
      assert.equal(projected[1].marker, "marker:roles");
    `);
  });

  it("keeps a roles-only entry and applies its localized visible label", () => {
    runProjection(`
      const input = [resource("/overview"), resource("/roles"), resource("/settings")];
      const projected = projectRoleManagementNavigation(input, title);

      assert.deepEqual(projected.map((item) => item.route), ["/overview", "/roles", "/settings"]);
      assert.deepEqual(projected[1].title, { zh: "角色管理", en: "Role Management" });
      assert.equal(projected[1].marker, "marker:roles");
    `);
  });

  it("keeps a role-groups-only entry as the selected target", () => {
    runProjection(`
      const input = [resource("/overview"), resource("/role-groups", "roleGroups"), resource("/settings")];
      const projected = projectRoleManagementNavigation(input, title);

      assert.deepEqual(projected.map((item) => item.route), ["/overview", "/role-groups", "/settings"]);
      assert.deepEqual(projected[1].title, title);
      assert.equal(projected[1].marker, "marker:roleGroups");
    `);
  });

  it("preserves a list with no role entries", () => {
    runProjection(`
      const input = [resource("/overview"), resource("/audit-logs"), resource("/settings")];
      const projected = projectRoleManagementNavigation(input, title);

      assert.deepEqual(projected, input);
      assert.notEqual(projected, input);
    `);
  });

  it("does not mutate the input array or its resource objects", () => {
    runProjection(`
      const input = [resource("/role-groups", "roleGroups"), resource("/roles"), resource("/settings")];
      const snapshot = structuredClone(input);
      for (const item of input) {
        Object.freeze(item.title);
        Object.freeze(item);
      }
      Object.freeze(input);

      const projected = projectRoleManagementNavigation(input, title);

      assert.deepEqual(input, snapshot);
      assert.notEqual(projected, input);
      assert.notEqual(projected[0], input[1]);
      assert.deepEqual(projected[0].title, title);
    `);
  });

  it("maps both legacy role routes to the selected projected entry", () => {
    runProjection(`
      const rolesProjected = projectRoleManagementNavigation([resource("/roles"), resource("/role-groups", "roleGroups")], title);
      assert.equal(resolveRoleManagementActiveRoute("/roles", rolesProjected), "/roles");
      assert.equal(resolveRoleManagementActiveRoute("/role-groups", rolesProjected), "/roles");

      const groupsProjected = projectRoleManagementNavigation([resource("/role-groups", "roleGroups")], title);
      assert.equal(resolveRoleManagementActiveRoute("/roles", groupsProjected), "/role-groups");
      assert.equal(resolveRoleManagementActiveRoute("/role-groups", groupsProjected), "/role-groups");
      assert.equal(resolveRoleManagementActiveRoute("/settings", groupsProjected), "/settings");
      assert.equal(resolveRoleManagementActiveRoute("/roles", [resource("/settings")]), "/roles");
    `);
  });
});
