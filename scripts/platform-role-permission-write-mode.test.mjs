import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const resolverURL = pathToFileURL(path.join(root, "admin/src/platform/resources/rolePermissionWriteMode.ts")).href;

function source(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function runResolverProbe(body) {
  const result = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module"], {
    cwd: root,
    encoding: "utf8",
    input: `
      import assert from "node:assert/strict";
      import { resolveRolePermissionWriteMode } from ${JSON.stringify(resolverURL)};

      const policyKeys = ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"];
      const fields = (state) => policyKeys.map((key) => ({ key, ...state }));

      ${body}
    `,
  });
  assert.equal(result.status, 0, result.stderr || result.stdout);
}

describe("role permission write mode resolver", () => {
  it("resolves a fully writable legacy schema", () => {
    runResolverProbe(`
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({ inForm: true, readOnly: false }) }), "legacy-generic");
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({}) }), "legacy-generic");
    `);
  });

  it("resolves a fully domain-owned target schema", () => {
    runResolverProbe(`
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({ inForm: false, readOnly: true }) }), "target-domain");
    `);
  });

  it("falls back to readonly when a policy field is missing", () => {
    runResolverProbe(`
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({}).slice(0, -1) }), "readonly");
      assert.equal(resolveRolePermissionWriteMode(undefined), "readonly");
    `);
  });

  it("falls back to readonly for mixed policy field states", () => {
    runResolverProbe(`
      const mixed = fields({ inForm: true, readOnly: false });
      mixed[2] = { ...mixed[2], inForm: false, readOnly: true };
      assert.equal(resolveRolePermissionWriteMode({ fields: mixed }), "readonly");
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({ inForm: false, readOnly: false }) }), "readonly");
      assert.equal(resolveRolePermissionWriteMode({ fields: fields({ inForm: true, readOnly: true }) }), "readonly");
    `);
  });

  it("ignores non-policy field noise", () => {
    runResolverProbe(`
      const noisy = [
        { key: "code", inForm: false, readOnly: true },
        ...fields({ inForm: true, readOnly: false }),
        { key: "description", inForm: false, readOnly: true },
      ];
      assert.equal(resolveRolePermissionWriteMode({ fields: noisy }), "legacy-generic");
    `);
  });
});

describe("role permission write mode integration", () => {
  const roleGovernance = source("admin/src/platform/resources/RoleGovernanceConsole.tsx");

  it("loads the role schema independently with stale and unmount protection", () => {
    assert.match(roleGovernance, /getAdminResourceSchema\("roles"\)/);
    assert.match(roleGovernance, /resolveRolePermissionWriteMode\(schema\)/);
    assert.match(roleGovernance, /const permissionSchemaRequest = useRef\(0\);/);
    assert.match(roleGovernance, /if \(permissionSchemaRequest\.current !== requestID\) return;/);
    assert.match(roleGovernance, /return \(\) => \{ permissionSchemaRequest\.current \+= 1; \};/);

    const schemaLoad = roleGovernance.indexOf('getAdminResourceSchema("roles")');
    const treeLoad = roleGovernance.indexOf("const loadGovernance");
    assert.ok(schemaLoad >= 0 && treeLoad >= 0, "schema and role-tree reads must remain independent");
  });

  it("selects the permission catalog from the permission mode rather than the menu gate", () => {
    const openAuthorization = roleGovernance.slice(roleGovernance.indexOf("const openAuthorization"), roleGovernance.indexOf("const saveAuthorization"));
    assert.match(roleGovernance, /writeMode: RolePermissionWriteMode;/);
    assert.match(openAuthorization, /const writeMode = permissionWriteMode;/);
    assert.match(openAuthorization, /writeMode === "target-domain"\s*\? assignmentPermissionRecords\(role\.code\)\s*:\s*loadAllRecords\("permissions"\)/);
    assert.match(openAuthorization, /setAuthorization\(\{\s*role,\s*writeMode,/);
    assert.doesNotMatch(openAuthorization, /roleMenuMigrationWriteEnabled/);
  });

  it("keeps legacy and target writes mutually exclusive while readonly cannot write", () => {
    const saveAuthorization = roleGovernance.slice(roleGovernance.indexOf("const saveAuthorization"), roleGovernance.indexOf("const openMenus"));
    assert.match(saveAuthorization, /if \(!authorization \|\| !canUpdateRole \|\| authorization\.role\.status !== "enabled" \|\| authorization\.writeMode === "readonly"\) return;/);
    assert.match(saveAuthorization, /if \(authorization\.writeMode === "legacy-generic"\) \{[\s\S]*?await updateAdminResource\("roles", authorization\.role\.id, legacyRolePermissionInput\(authorization\)\);/);
    assert.match(saveAuthorization, /await prepareRolePermissionChange\(authorization\.role\.code/);
    assert.match(saveAuthorization, /await getRolePermissionChangeImpact\(preview\.previewId\)/);
    assert.match(saveAuthorization, /await replaceRolePermissions\(preview\)/);
  });

  it("preserves the complete public role snapshot in legacy generic writes", () => {
    const legacyInput = roleGovernance.slice(roleGovernance.indexOf("function legacyRolePermissionInput"), roleGovernance.indexOf("function selectedRecord"));
    assert.match(legacyInput, /code: authorization\.role\.code/);
    assert.match(legacyInput, /name: authorization\.role\.name/);
    assert.match(legacyInput, /status: authorization\.role\.status/);
    assert.match(legacyInput, /description: authorization\.role\.description/);
    assert.match(legacyInput, /\.\.\.authorization\.role\.values/);
    for (const key of ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"]) {
      assert.match(legacyInput, new RegExp(`${key}:`));
    }
    assert.match(legacyInput, /uniqueSorted\(authorization\.allow\)\.join\(","\)/);
    assert.match(legacyInput, /uniqueSorted\(authorization\.deny\)\.join\(","\)/);
  });

  it("lets disabled roles open for inspection while guarding every save", () => {
    const detail = roleGovernance.slice(roleGovernance.indexOf("function RoleGovernanceDetail"), roleGovernance.indexOf("function AuthorizationModal"));
    assert.match(detail, /canReadAuthorizationInputs \? <Button ref=\{authorizationTriggerRef\} icon=\{<SafetyCertificateOutlined/);
    assert.doesNotMatch(detail, /authorizationTriggerRef\} disabled=\{!canUpdateRole \|\| record\.status !== "enabled"\}/);
    assert.match(roleGovernance, /authorization\.role\.status !== "enabled"/);
  });

  it("renders permission inspection with read-only controls and a close-only footer", () => {
    const modal = roleGovernance.slice(roleGovernance.indexOf("function AuthorizationModal"), roleGovernance.indexOf("function MenuVisibilityModal"));
    assert.match(modal, /footer=\{readOnly \? <Button onClick=\{onCancel\}>\{dictionary\.close\}<\/Button> : undefined\}/);
    assert.match(modal, /<AdminFeedback type="warning" message=\{dictionary\.rolePermissionReadonlyTitle\} description=\{readOnlyReason\} \/>/);
    assert.match(modal, /<PlatformTreeTransfer[\s\S]*?readOnly=\{readOnly\}/);
    assert.match(modal, /<Select[\s\S]*?disabled=\{readOnly\}/);
    assert.match(modal, /<PlatformTreeSelect[\s\S]*?disabled=\{readOnly\}/);
  });
});
