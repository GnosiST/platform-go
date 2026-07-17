import assert from "node:assert/strict";
import { describe, it } from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  dispatchOrganizationUserWrite,
  organizationUserRuntimeCapabilities,
  organizationTreeFieldOption,
  resolveOrganizationUserRuntimeMode,
} from "../admin/src/platform/resources/organizationUserRuntime.ts";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const experienceSource = fs.readFileSync(path.join(root, "admin/src/platform/resources/organizationUserExperience.tsx"), "utf8");
const policyKeys = ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"];

function roleSchema(state) {
  return { fields: policyKeys.map((key) => ({ key, ...state })) };
}

describe("organization user runtime mode", () => {
  it("keeps legacy organization workflows on generic APIs without service objects", () => {
    const mode = resolveOrganizationUserRuntimeMode(roleSchema({ inForm: true, readOnly: false }));
    const capabilities = organizationUserRuntimeCapabilities(mode);

    assert.equal(mode, "legacy");
    assert.equal(capabilities.useServiceObjects, false);
    assert.equal(capabilities.allowGenericWrite, true);
  });

  it("dispatches a legacy submit exclusively to the generic writer", async () => {
    const calls = [];
    const result = await dispatchOrganizationUserWrite("legacy", {
      generic: async () => { calls.push("generic"); return "saved"; },
      target: async () => { calls.push("target"); return "wrong"; },
      readonly: async () => { calls.push("readonly"); return "wrong"; },
    });

    assert.equal(result, "saved");
    assert.deepEqual(calls, ["generic"]);
  });

  it("uses service objects only for a complete target schema", () => {
    const mode = resolveOrganizationUserRuntimeMode(roleSchema({ inForm: false, readOnly: true }));
    const capabilities = organizationUserRuntimeCapabilities(mode);

    assert.equal(mode, "target");
    assert.equal(capabilities.useServiceObjects, true);
    assert.equal(capabilities.allowGenericWrite, false);
  });

  it("fails closed for missing or mixed ownership signals", () => {
    const missing = roleSchema({ inForm: true, readOnly: false });
    missing.fields.pop();
    const mixed = roleSchema({ inForm: true, readOnly: false });
    mixed.fields[0] = { ...mixed.fields[0], inForm: false, readOnly: true };

    for (const schema of [undefined, missing, mixed]) {
      const mode = resolveOrganizationUserRuntimeMode(schema);
      const capabilities = organizationUserRuntimeCapabilities(mode);
      assert.equal(mode, "readonly");
      assert.equal(capabilities.useServiceObjects, false);
      assert.equal(capabilities.allowGenericWrite, false);
    }
  });

  it("preserves organization hierarchy in tree field options", () => {
    const option = organizationTreeFieldOption({
      id: "org-child",
      code: "child",
      name: "Child",
      status: "enabled",
      updatedAt: "2026-07-17T00:00:00Z",
      values: { parentCode: "parent" },
    });

    assert.deepEqual(option, {
      value: "child",
      label: { zh: "Child (child)", en: "Child (child)" },
      parentValue: "parent",
    });
  });

  it("wires service-object reads and writes only through the target mode", () => {
    assert.match(experienceSource, /if \(!active \|\| !runtimeCapabilities\?\.useServiceObjects \|\| resourceKey !== "users"[\s\S]*?getOrganizationRolePool\(selectedOrgUnitCode\)/);
    assert.match(experienceSource, /dispatchOrganizationUserWrite\(runtimeMode, \{[\s\S]*?generic: \(\) => context\.persist\(context\.input\),[\s\S]*?target: \(\) => resourceKey === "org-units"/);
    assert.match(experienceSource, /detailTab: active && runtimeCapabilities\?\.useServiceObjects && resourceKey === "org-units"/);
  });
});
