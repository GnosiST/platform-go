import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const resolverURL = pathToFileURL(path.join(root, "admin/src/platform/resources/rolePermissionWriteMode.ts")).href;
const workflowURL = pathToFileURL(path.join(root, "admin/src/platform/resources/rolePermissionWorkflow.ts")).href;

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

function runWorkflowProbe(body) {
  const result = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module"], {
    cwd: root,
    encoding: "utf8",
    input: `
      import assert from "node:assert/strict";
      import { executeRolePermissionWrite, loadRolePermissionCatalog } from ${JSON.stringify(workflowURL)};

      const role = {
        id: "role-1",
        code: "ops",
        name: "Operations",
        status: "enabled",
        description: "Operations role",
        updatedAt: "2026-07-17T00:00:00Z",
        values: { groupCode: "platform-ops", retained: "yes", permissions: "old" },
      };
      const authorization = {
        role,
        writeMode: "legacy-generic",
        allow: ["admin:z", "admin:a", "admin:a"],
        deny: ["admin:d"],
        dataScope: "custom_orgs",
        dataScopeOrgCodes: ["org-b", "org-a", "org-a"],
        dataScopeAreaCodes: ["area-unused"],
      };

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
  const rolePermissionWorkflow = source("admin/src/platform/resources/rolePermissionWorkflow.ts");

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
    assert.match(openAuthorization, /const writeMode = permissionWriteMode;/);
    assert.match(openAuthorization, /loadRolePermissionCatalog\(writeMode, role\.code, \{[\s\S]*?target: assignmentPermissionRecords,[\s\S]*?generic: \(\) => loadAllRecords\("permissions"\),[\s\S]*?\}\)/);
    assert.match(openAuthorization, /setAuthorization\(\{\s*role,\s*writeMode,/);
    assert.doesNotMatch(openAuthorization, /roleMenuMigrationWriteEnabled/);
  });

  it("connects the component to the executable permission write dispatcher", () => {
    const saveAuthorization = roleGovernance.slice(roleGovernance.indexOf("const saveAuthorization"), roleGovernance.indexOf("const openMenus"));
    assert.match(saveAuthorization, /if \(!authorization \|\| !canUpdateRole \|\| authorization\.role\.status !== "enabled" \|\| authorization\.writeMode === "readonly"\) return;/);
    assert.match(saveAuthorization, /executeRolePermissionWrite\(authorization, canUpdateRole, \{[\s\S]*?updateAdminResource,[\s\S]*?prepare: prepareRolePermissionChange,[\s\S]*?impact: getRolePermissionChangeImpact,[\s\S]*?replace: replaceRolePermissions,/);
    assert.match(rolePermissionWorkflow, /if \(!canUpdateRole \|\| authorization\.role\.status !== "enabled" \|\| authorization\.writeMode === "readonly"\) return "blocked"/);
  });

  it("preserves the complete public role snapshot in legacy generic writes", () => {
    const legacyInput = rolePermissionWorkflow.slice(rolePermissionWorkflow.indexOf("function legacyRolePermissionInput"));
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
    assert.match(detail, /canReadAuthorizationInputs \? <AdminActionButton ref=\{authorizationTriggerRef\} icon=\{<SafetyCertificateOutlined/);
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

describe("role permission workflow executable branches", () => {
  it("performs one exact legacy generic update and no domain commands", () => {
    runWorkflowProbe(`
      const calls = [];
      const result = await executeRolePermissionWrite(authorization, true, {
        updateAdminResource: async (resource, id, input) => { calls.push({ type: "update", resource, id, input }); },
        prepare: async () => { calls.push({ type: "prepare" }); throw new Error("unexpected prepare"); },
        impact: async () => { calls.push({ type: "impact" }); throw new Error("unexpected impact"); },
        confirm: async () => { calls.push({ type: "confirm" }); throw new Error("unexpected confirm"); },
        replace: async () => { calls.push({ type: "replace" }); throw new Error("unexpected replace"); },
      });

      assert.equal(result, "applied");
      assert.deepEqual(calls, [{
        type: "update",
        resource: "roles",
        id: "role-1",
        input: {
          code: "ops",
          name: "Operations",
          status: "enabled",
          description: "Operations role",
          values: {
            groupCode: "platform-ops",
            retained: "yes",
            permissions: "admin:a,admin:z",
            denyPermissions: "admin:d",
            dataScope: "custom_orgs",
            dataScopeOrgCodes: "org-a,org-b",
            dataScopeAreaCodes: "",
          },
        },
      }]);
    `);
  });

  it("performs target prepare, impact, confirmation and replace without a generic update", () => {
    runWorkflowProbe(`
      const targetAuthorization = {
        ...authorization,
        writeMode: "target-domain",
        dataScope: "custom_areas",
        dataScopeOrgCodes: ["org-unused"],
        dataScopeAreaCodes: ["area-b", "area-a"],
      };
      const preview = { previewId: "preview-1", expectedRevision: 7, impactHash: "hash-1" };
      const impact = { affectedUsers: 3, conflictCount: 0 };
      const calls = [];
      const result = await executeRolePermissionWrite(targetAuthorization, true, {
        updateAdminResource: async () => { calls.push({ type: "update" }); throw new Error("unexpected update"); },
        prepare: async (roleCode, policy) => { calls.push({ type: "prepare", roleCode, policy }); return preview; },
        impact: async (previewId) => { calls.push({ type: "impact", previewId }); return impact; },
        confirm: async (nextImpact) => { calls.push({ type: "confirm", impact: nextImpact }); return true; },
        replace: async (nextPreview) => { calls.push({ type: "replace", preview: nextPreview }); },
      });

      assert.equal(result, "applied");
      assert.deepEqual(calls, [
        {
          type: "prepare",
          roleCode: "ops",
          policy: {
            allowPermissionCodes: ["admin:z", "admin:a", "admin:a"],
            denyPermissionCodes: ["admin:d"],
            dataScope: "custom_areas",
            dataScopeOrgCodes: [],
            dataScopeAreaCodes: ["area-b", "area-a"],
          },
        },
        { type: "impact", previewId: "preview-1" },
        { type: "confirm", impact },
        { type: "replace", preview },
      ]);
    `);
  });

  it("performs zero writes for readonly, disabled and unauthorized states", () => {
    runWorkflowProbe(`
      for (const [candidate, canUpdateRole] of [
        [{ ...authorization, writeMode: "readonly" }, true],
        [{ ...authorization, writeMode: "target-domain", role: { ...role, status: "disabled" } }, true],
        [{ ...authorization, writeMode: "target-domain" }, false],
      ]) {
        const calls = [];
        const clients = {
          updateAdminResource: async () => { calls.push("update"); },
          prepare: async () => { calls.push("prepare"); return { previewId: "unexpected" }; },
          impact: async () => { calls.push("impact"); return {}; },
          confirm: async () => { calls.push("confirm"); return true; },
          replace: async () => { calls.push("replace"); },
        };
        assert.equal(await executeRolePermissionWrite(candidate, canUpdateRole, clients), "blocked");
        assert.deepEqual(calls, []);
      }
    `);
  });

  it("loads the catalog from the snapshotted permission mode", () => {
    runWorkflowProbe(`
      const calls = [];
      const sources = {
        target: async (roleCode) => { calls.push(["target", roleCode]); return ["target-record"]; },
        generic: async () => { calls.push(["generic"]); return ["generic-record"]; },
      };
      let currentMode = "target-domain";
      const snapshottedMode = currentMode;
      const targetResult = loadRolePermissionCatalog(snapshottedMode, "ops", sources);
      currentMode = "legacy-generic";

      assert.deepEqual(await targetResult, ["target-record"]);
      assert.deepEqual(await loadRolePermissionCatalog(currentMode, "ops", sources), ["generic-record"]);
      assert.deepEqual(await loadRolePermissionCatalog("readonly", "ops", sources), ["generic-record"]);
      assert.deepEqual(calls, [["target", "ops"], ["generic"], ["generic"]]);
    `);
  });
});
