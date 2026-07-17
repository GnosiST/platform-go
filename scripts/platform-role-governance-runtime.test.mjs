import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";

const runtimeSource = readFileSync(new URL("../admin/src/platform/resources/roleGovernanceRuntime.ts", import.meta.url), "utf8");
const runtimeProbeDirectory = mkdtempSync(join(tmpdir(), "platform-role-governance-runtime-"));
const runtimeProbePath = join(runtimeProbeDirectory, "roleGovernanceRuntime.ts");
writeFileSync(runtimeProbePath, runtimeSource.replace(
  /import \{ resolveRolePermissionWriteMode, type RolePermissionWriteMode \} from "\.\/rolePermissionWriteMode";/,
  `type RolePermissionWriteMode = "legacy-generic" | "target-domain" | "readonly";
function resolveRolePermissionWriteMode(schema) {
  const keys = ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"];
  const fields = keys.map((key) => schema?.fields.find((field) => field.key === key));
  if (fields.some((field) => !field)) return "readonly";
  if (fields.every((field) => field?.inForm !== false && field?.readOnly !== true)) return "legacy-generic";
  if (fields.every((field) => field?.inForm !== true && field?.readOnly === true)) return "target-domain";
  return "readonly";
}`,
));
const {
  buildRoleGovernanceMetadataInput,
  loadRoleAssignmentSearchPages,
  localizedGovernanceDescription,
  localizedGovernanceName,
  projectRoleGovernanceTree,
  resolveRoleGovernanceRuntime,
} = await import(pathToFileURL(runtimeProbePath));
process.on("exit", () => rmSync(runtimeProbeDirectory, { recursive: true, force: true }));

const roleGovernance = readFileSync(new URL("../admin/src/platform/resources/RoleGovernanceConsole.tsx", import.meta.url), "utf8");

describe("role governance runtime contracts", () => {
  const field = (key, overrides = {}) => ({ key, source: "values", sensitivity: "public", inForm: true, ...overrides });
  const groupSchema = (target) => ({ fields: [
    field("parentCode", target ? { readOnly: true, inForm: false } : {}),
    field("scopeType", target ? { required: true } : { readOnly: true, inForm: false }),
    field("tenantCode", target ? {} : { readOnly: true, inForm: false }),
    field("sortOrder"),
  ] });
  const roleSchema = (target) => ({ fields: [
    field("groupCode"),
    ...["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"]
      .map((key) => field(key, target ? { readOnly: true, inForm: undefined } : {})),
  ] });
  const menuSchema = (target) => ({ fields: [field("nodeType", { required: target })] });

  it("derives role governance availability from runtime schemas instead of a permanent client gate", () => {
    assert.doesNotMatch(roleGovernance, /roleMenuMigrationWriteEnabled/);
    assert.match(roleGovernance, /getAdminResourceSchema\("role-groups"\)/);
    assert.match(roleGovernance, /getAdminResourceSchema\("roles"\)/);
    assert.match(roleGovernance, /getAdminResourceSchema\("menus"\)/);
    assert.match(roleGovernance, /resolveRoleGovernanceRuntime/);
  });

  it("keeps metadata payloads and lifecycle mutations behind the resolved runtime", () => {
    assert.match(roleGovernance, /buildRoleGovernanceMetadataInput/);
    assert.doesNotMatch(roleGovernance, /values:\s*\{\s*\.\.\.existing/);
    assert.match(roleGovernance, /roleLifecycleTargetEnabled/);
    assert.match(roleGovernance, /if \(!roleLifecycleTargetEnabled\) return/);
  });

  it("paginates target assignment searches at the service limit", () => {
    assert.match(roleGovernance, /loadRoleAssignmentSearchPages/);
    assert.doesNotMatch(roleGovernance, /searchPermissionAssignmentTree\(roleCode, "", 1, 1000\)/);
    assert.doesNotMatch(roleGovernance, /searchMenuAssignmentTree\(roleCode, "", 1, 1000\)/);
  });

  it("uses localized record projections across the role workbench", () => {
    assert.match(roleGovernance, /localizedGovernanceName/);
    assert.match(roleGovernance, /projectRoleGovernanceTree\(groups, roles, search, language/);
    assert.match(roleGovernance, /permissionTreeNodes\(permissionCatalog, dictionary, language/);
    assert.match(roleGovernance, /menuTreeNodes\(menus, historicalCodes, dictionary, language\)/);
  });

  it("opens target lifecycle and menu writes only after all cutover schemas agree", () => {
    assert.deepEqual(resolveRoleGovernanceRuntime(groupSchema(false), roleSchema(false), menuSchema(false)), {
      groupWriteMode: "legacy",
      permissionWriteMode: "legacy-generic",
      roleLifecycleTargetEnabled: false,
      roleMenuTargetEnabled: false,
    });
    assert.deepEqual(resolveRoleGovernanceRuntime(groupSchema(true), roleSchema(true), menuSchema(true)), {
      groupWriteMode: "target",
      permissionWriteMode: "target-domain",
      roleLifecycleTargetEnabled: true,
      roleMenuTargetEnabled: true,
    });
    assert.equal(resolveRoleGovernanceRuntime(groupSchema(true), roleSchema(true), menuSchema(false)).roleMenuTargetEnabled, false);
  });

  it("omits readonly ownership and permission projections while preserving legacy parents", () => {
    const group = { id: "group-ops", code: "ops", name: "Ops", status: "enabled", updatedAt: "now", values: { parentCode: "legacy-parent", scopeType: "tenant", tenantCode: "tenant-a", sortOrder: "1" } };
    const values = { code: "ops", name: "Operations", description: "Operations", scopeType: "platform", tenantCode: "", sortOrder: 3 };
    const legacy = buildRoleGovernanceMetadataInput({ kind: "group", record: group }, values, groupSchema(false), "legacy");
    assert.deepEqual(legacy.values, { parentCode: "legacy-parent", sortOrder: "3" });
    const target = buildRoleGovernanceMetadataInput({ kind: "group", record: group }, values, groupSchema(true), "target");
    assert.deepEqual(target.values, { scopeType: "platform", tenantCode: "", sortOrder: "3" });

    const role = { id: "role-ops", code: "operator", name: "Operator", status: "enabled", updatedAt: "now", values: { groupCode: "ops", permissions: "admin:*", dataScope: "all" } };
    const roleInput = buildRoleGovernanceMetadataInput({ kind: "role", record: role }, { code: "operator", name: "Operator", groupCode: "auditors" }, roleSchema(true), "target");
    assert.deepEqual(roleInput.values, { groupCode: "auditors" });
  });

  it("keeps nested groups and projects placeholders for orphaned roles", () => {
    const groups = [
      { id: "g-root", code: "root", name: "Root", status: "enabled", updatedAt: "", values: { nameZh: "根角色组" } },
      { id: "g-child", code: "child", name: "Child", status: "enabled", updatedAt: "", values: { nameZh: "子角色组", parentCode: "root" } },
    ];
    const roles = [
      { id: "r-child", code: "editor", name: "Editor", status: "enabled", updatedAt: "", values: { nameZh: "编辑员", groupCode: "child" } },
      { id: "r-orphan", code: "orphan", name: "Orphan", status: "enabled", updatedAt: "", values: { nameZh: "孤立角色", groupCode: "missing" } },
    ];
    const nodes = projectRoleGovernanceTree(groups, roles, "", "zh", "未分组");
    assert.equal(nodes.find((node) => node.key === "group:child")?.parentKey, "group:root");
    assert.equal(nodes.find((node) => node.key === "role:editor")?.parentKey, "group:child");
    assert.equal(nodes.find((node) => node.key === "group:missing")?.selectable, false);
    assert.equal(nodes.find((node) => node.key === "role:orphan")?.parentKey, "group:missing");
    assert.equal(localizedGovernanceName(roles[0], "zh"), "编辑员");
    assert.equal(localizedGovernanceName(roles[0], "en"), "Editor");
    const localizedRole = { ...roles[0], description: "English description", values: { ...roles[0].values, descriptionZh: "中文描述", descriptionEn: "English description" } };
    assert.equal(localizedGovernanceDescription(localizedRole, "zh"), "中文描述");
    assert.equal(localizedGovernanceDescription(localizedRole, "en"), "English description");
  });

  it("loads every assignment search page with the maximum page size of 100", async () => {
    const calls = [];
    const records = await loadRoleAssignmentSearchPages("operator", async (...args) => {
      calls.push(args);
      return args[2] === 1 ? Array.from({ length: 100 }, (_, index) => index) : [100];
    });
    assert.equal(records.length, 101);
    assert.deepEqual(calls, [
      ["operator", "", 1, 100],
      ["operator", "", 2, 100],
    ]);
  });
});
