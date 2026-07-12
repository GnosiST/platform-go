import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-admin-resources.mjs", ...args], {
    cwd: path.resolve(import.meta.dirname, ".."),
    encoding: "utf8",
  });
}

function writeBrokenManifest(mutator) {
  const sourcePath = path.resolve(import.meta.dirname, "..", "resources", "admin-resources.json");
  const manifest = JSON.parse(fs.readFileSync(sourcePath, "utf8"));
  mutator(manifest);
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-resources-"));
  const targetPath = path.join(tempDir, "admin-resources.json");
  fs.writeFileSync(targetPath, `${JSON.stringify(manifest, null, 2)}\n`);
  return targetPath;
}

function writeBrokenGeneratedContract(mutator) {
  const result = spawnSync(process.execPath, ["scripts/generate-admin-resource-contract.mjs", "--stdout"], {
    cwd: path.resolve(import.meta.dirname, ".."),
    encoding: "utf8",
  });
  assert.equal(result.status, 0, `generate-admin-resource-contract.mjs failed\n${result.stdout}${result.stderr}`);
  const contract = JSON.parse(result.stdout);
  mutator(contract);
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-contract-"));
  const targetPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(targetPath, `${JSON.stringify(contract, null, 2)}\n`);
  return targetPath;
}

describe("validate-admin-resources default gate wiring", () => {
  it("runs capability contract and reference discovery validators in the default admin resource gate", () => {
    const validatorSource = fs.readFileSync(path.resolve(import.meta.dirname, "validate-admin-resources.mjs"), "utf8");

    assert.match(validatorSource, /assertValidatorPass\("validate-platform-capability-contracts\.mjs"\)/);
    assert.match(validatorSource, /assertValidatorPass\("validate-platform-reference-discovery\.mjs"\)/);
    assert.match(validatorSource, /assertValidatorPass\("validate-platform-reference-coverage\.mjs"\)/);
  });

  it("rejects generated default capability resources without usable schema fields", () => {
    const manifestPath = writeBrokenManifest(() => {});
    const contractPath = writeBrokenGeneratedContract((contract) => {
      const schema = contract.schemas["dictionary-parameters"];
      schema.fields = [];
      schema.table = [];
      schema.search = [];
      schema.filter = [];
      schema.sort = [];
    });

    const result = runValidator(["--manifest", manifestPath, "--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /generated resource dictionary-parameters must declare schema fields/);
    assert.match(result.stderr, /generated resource dictionary-parameters must declare table fields/);
    assert.match(result.stderr, /generated resource dictionary-parameters must declare search fields/);
    assert.match(result.stderr, /generated resource dictionary-parameters must declare filter fields/);
    assert.match(result.stderr, /generated resource dictionary-parameters must declare sort fields/);
  });
});

describe("validate-admin-resources field security policies", () => {
  function mutateProtectedField(mutator) {
    return writeBrokenManifest((manifest) => {
      const identities = manifest.resources.find((resource) => resource.code === "app-identities");
      const field = identities.schema.fields.find((candidate) => candidate.key === "providerSubjectHash");
      mutator(field);
    });
  }

  for (const [name, mutate, pattern] of [
    ["sensitivity", (field) => (field.sensitivity = "classified"), /unsupported sensitivity classified/],
    ["storage mode", (field) => (field.storageMode = "digest"), /unsupported storageMode digest/],
    ["response mode", (field) => (field.responseMode = "redacted"), /unsupported responseMode redacted/],
    ["export mode", (field) => (field.exportMode = "redacted"), /unsupported exportMode redacted/],
    ["plain secret", (field) => (field.storageMode = "plain"), /require protected storage/],
    ["hash response", (field) => (field.responseMode = "full"), /must be omitted from response and export/],
  ]) {
    it(`rejects invalid ${name}`, () => {
      const result = runValidator(["--manifest", mutateProtectedField(mutate)]);
      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, pattern);
    });
  }

  for (const [name, field, pattern] of [
    [
      "plain personal storage",
      { key: "contactPhone", sensitivity: "personal", storageMode: "plain", responseMode: "masked", exportMode: "masked" },
      /personal values require masked or protected storage/,
    ],
    [
      "masked storage without personal sensitivity",
      { key: "maskedPhone", sensitivity: "public", storageMode: "masked", responseMode: "masked", exportMode: "masked" },
      /masked storage requires personal sensitivity/,
    ],
    [
      "masked storage with full response",
      { key: "maskedPhone", sensitivity: "personal", storageMode: "masked", responseMode: "full", exportMode: "masked" },
      /masked storage must use masked or omitted response and export/,
    ],
  ]) {
    it(`rejects ${name}`, () => {
      const manifestPath = writeBrokenManifest((manifest) => {
        const identities = manifest.resources.find((resource) => resource.code === "app-identities");
        identities.schema.fields.push({
          label: { zh: "安全字段", en: "Security Field" },
          type: "text",
          source: "values",
          ...field,
        });
      });

      const result = runValidator(["--manifest", manifestPath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, pattern);
    });
  }

  it("accepts explicit public plain fields regardless of security-like names", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const identities = manifest.resources.find((resource) => resource.code === "app-identities");
      for (const field of [
        { key: "contactPhone", sensitivity: "public", storageMode: "plain", responseMode: "full", exportMode: "full" },
        { key: "email", sensitivity: "public", storageMode: "plain", responseMode: "full", exportMode: "full" },
        { key: "address", sensitivity: "public", storageMode: "plain", responseMode: "full", exportMode: "full" },
        { key: "apiToken", sensitivity: "public", storageMode: "plain", responseMode: "full", exportMode: "full" },
      ]) {
        identities.schema.fields.push({
          label: { zh: "安全字段", en: "Security Field" },
          type: "text",
          source: "values",
          ...field,
        });
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  function mutateEncryptedField(mutator = () => {}) {
    return writeBrokenManifest((manifest) => {
      const identities = manifest.resources.find((resource) => resource.code === "app-identities");
      identities.schema.protection = { schemaVersion: 1, scope: "tenant-field", tenantField: "tenantCode" };
      identities.schema.fields.push(
        { key: "tenantCode", label: { zh: "租户", en: "Tenant" }, type: "text", source: "values", required: true },
        {
          key: "governmentReference",
          label: { zh: "政府引用", en: "Government Reference" },
          type: "text",
          source: "values",
          sensitivity: "sensitive",
          storageMode: "encrypted",
          responseMode: "privileged",
          exportMode: "omitted",
          filterable: true,
          protection: {
            format: "aes-256-gcm-v1",
            normalization: "trim-v1",
            blindIndexNamespace: "custom-government-reference",
          },
        },
      );
      mutator(identities, identities.schema.fields.at(-1));
    });
  }

  it("accepts arbitrary custom encrypted fields with explicit protection", () => {
    const result = runValidator(["--manifest", mutateEncryptedField()]);
    assert.equal(result.status, 0, result.stderr);
  });

  for (const [name, mutate, pattern] of [
    ["missing field protection", (_resource, field) => delete field.protection, /encrypted storage requires protection metadata/],
    ["missing format", (_resource, field) => delete field.protection.format, /protection format is required/],
    ["missing normalization", (_resource, field) => delete field.protection.normalization, /protection normalization is required/],
    ["missing resource protection", (resource) => delete resource.schema.protection, /encrypted fields require resource protection metadata/],
    ["missing resource scope", (resource) => delete resource.schema.protection.scope, /protection scope is required/],
    ["missing tenant field", (resource) => delete resource.schema.protection.tenantField, /tenantField is required/],
    ["undeclared tenant field", (resource) => (resource.schema.protection.tenantField = "accountCode"), /tenantField accountCode is not declared/],
    ["protected tenant field", (resource) => (resource.schema.fields.at(-2).storageMode = "masked"), /tenantField tenantCode must use plain storage/],
    ["optional tenant field", (resource) => (resource.schema.fields.at(-2).required = false), /tenantField tenantCode must be required/],
    ["non canonical namespace", (_resource, field) => (field.protection.blindIndexNamespace = "Custom Reference"), /blindIndexNamespace must be canonical/],
    [
      "duplicate namespace",
      (resource, field) => resource.schema.fields.push({ ...field, key: "secondaryReference", label: { zh: "次级引用", en: "Secondary Reference" } }),
      /duplicate blindIndexNamespace custom-government-reference/,
    ],
    ["keyword search", (_resource, field) => (field.searchable = true), /encrypted fields cannot use keyword search/],
    ["range filtering without blind index", (_resource, field) => delete field.protection.blindIndexNamespace, /encrypted filtering requires a blindIndexNamespace/],
    ["sorting", (_resource, field) => (field.sortable = true), /encrypted fields cannot be sorted/],
  ]) {
    it(`rejects encrypted protection with ${name}`, () => {
      const result = runValidator(["--manifest", mutateEncryptedField(mutate)]);
      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, pattern);
    });
  }
});

describe("validate-admin-resources relation contracts", () => {
  it("accepts the side-detail-preview schema form layout preset", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const menus = manifest.resources.find((resource) => resource.code === "menus");
      menus.schema.formLayout = "side-detail-preview";
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects unsupported schema form layouts", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const menus = manifest.resources.find((resource) => resource.code === "menus");
      menus.schema.formLayout = "runtime-component-path";
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resource menus schema\.formLayout has unsupported layout runtime-component-path/);
  });

  it("rejects duplicate custom action keys on one resource", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const menus = manifest.resources.find((resource) => resource.code === "menus");
      menus.actions = [
        {
          key: "copy-config",
          label: { zh: "复制配置", en: "Copy Config" },
          kind: "row",
          permission: "admin:menu:read",
        },
        {
          key: "copy-config",
          label: { zh: "再次复制", en: "Copy Again" },
          kind: "row",
          permission: "admin:menu:read",
        },
      ];
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resource menus actions.key contains duplicate value: copy-config/);
  });

  it("rejects danger custom actions without confirmation metadata", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const menus = manifest.resources.find((resource) => resource.code === "menus");
      menus.actions = [
        {
          key: "danger-close",
          label: { zh: "关闭", en: "Close" },
          kind: "row",
          tone: "danger",
          permission: "admin:menu:update",
          route: "/api/admin/menus/:id/close",
          method: "POST",
        },
      ];
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resource menus action danger-close danger action requires confirmation/);
  });

  it("rejects unsupported custom panel metadata", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const menus = manifest.resources.find((resource) => resource.code === "menus");
      menus.panels = [
        {
          key: "unsafe",
          label: { zh: "不安全", en: "Unsafe" },
          kind: "workflow",
          permission: "admin:menu:read",
          component: "../UnsafePanel.tsx",
        },
      ];
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resource menus panel unsafe has unsupported kind workflow/);
    assert.match(result.stderr, /resource menus panel unsafe component must be a semantic key/);
  });

  it("rejects relation metadata that references fields missing from the target resource", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const users = manifest.resources.find((resource) => resource.code === "users");
      const tenantCode = users.schema.fields.find((field) => field.key === "tenantCode");
      tenantCode.relation.labelField = "displayName";
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /relation\.labelField references unknown tenants\.displayName/);
  });

  it("rejects tenant resources that omit address-code ownership", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const tenants = manifest.resources.find((resource) => resource.code === "tenants");
      tenants.schema.fields = tenants.schema.fields.filter((field) => field.key !== "areaCode");
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        tenants.schema[listName] = (tenants.schema[listName] ?? []).filter((field) => field !== "areaCode");
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /tenants must declare areaCode relation to area-codes/);
  });

  it("rejects user resources that omit organization or area ownership", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const users = manifest.resources.find((resource) => resource.code === "users");
      users.schema.fields = users.schema.fields.filter((field) => field.key !== "orgUnitCode" && field.key !== "areaCode");
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        users.schema[listName] = (users.schema[listName] ?? []).filter((field) => field !== "orgUnitCode" && field !== "areaCode");
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /users must declare orgUnitCode relation to org-units/);
    assert.match(result.stderr, /users must declare areaCode relation to area-codes/);
  });

  it("rejects governance ownership fields with the wrong required policy", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const users = manifest.resources.find((resource) => resource.code === "users");
      users.schema.fields.find((field) => field.key === "tenantCode").required = false;
      users.schema.fields.find((field) => field.key === "orgUnitCode").required = true;
      users.schema.fields.find((field) => field.key === "areaCode").required = true;

      const orgUnits = manifest.resources.find((resource) => resource.code === "org-units");
      delete orgUnits.schema.fields.find((field) => field.key === "tenantCode").required;
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /users tenantCode must be required/);
    assert.match(result.stderr, /users orgUnitCode must stay optional by default/);
    assert.match(result.stderr, /users areaCode must stay optional by default/);
    assert.match(result.stderr, /org-units tenantCode must be required/);
  });

  it("rejects org-unit resources that omit tenant, parent or area structure", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const orgUnits = manifest.resources.find((resource) => resource.code === "org-units");
      const removed = new Set(["type", "tenantCode", "parentCode", "areaCode"]);
      orgUnits.schema.fields = orgUnits.schema.fields.filter((field) => !removed.has(field.key));
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        orgUnits.schema[listName] = (orgUnits.schema[listName] ?? []).filter((field) => !removed.has(field));
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /org-units must declare required type select options/);
    assert.match(result.stderr, /org-units must declare tenantCode relation to tenants/);
    assert.match(result.stderr, /org-units must declare parentCode tree relation to org-units/);
    assert.match(result.stderr, /org-units must declare areaCode relation to area-codes/);
  });

  it("rejects org-unit resources that omit common institution levels", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const orgUnits = manifest.resources.find((resource) => resource.code === "org-units");
      const typeField = orgUnits.schema.fields.find((field) => field.key === "type");
      typeField.options = ["organization", "department", "team"];
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /org-units must declare required type select options: group, company, branch, organization, department, team, store, custom/);
  });

  it("rejects area-code resources that omit tree hierarchy fields", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const areaCodes = manifest.resources.find((resource) => resource.code === "area-codes");
      const removed = new Set(["parentCode", "level", "path"]);
      areaCodes.schema.fields = areaCodes.schema.fields.filter((field) => !removed.has(field.key));
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        areaCodes.schema[listName] = (areaCodes.schema[listName] ?? []).filter((field) => !removed.has(field));
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /area-codes must declare parentCode tree relation to area-codes/);
    assert.match(result.stderr, /area-codes must declare level select options/);
    assert.match(result.stderr, /area-codes must declare path hierarchy field/);
  });

  it("rejects area-code resources that drop street or custom levels", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const areaCodes = manifest.resources.find((resource) => resource.code === "area-codes");
      const level = areaCodes.schema.fields.find((field) => field.key === "level");
      level.options = level.options.filter((option) => option !== "street" && option !== "custom");
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /area-codes must declare level select options/);
  });

  it("rejects role resources that omit role-group or data-scope governance fields", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roles = manifest.resources.find((resource) => resource.code === "roles");
      const removed = new Set(["groupCode", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"]);
      roles.schema.fields = roles.schema.fields.filter((field) => !removed.has(field.key));
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        roles.schema[listName] = (roles.schema[listName] ?? []).filter((field) => !removed.has(field));
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /roles must declare groupCode tree relation to role-groups/);
    assert.match(result.stderr, /roles must declare required dataScope select options/);
    assert.match(result.stderr, /roles must declare dataScopeOrgCodes relation to org-units/);
    assert.match(result.stderr, /roles must declare dataScopeAreaCodes relation to area-codes/);
  });

  it("rejects role resources whose role-group relation is not tree-shaped", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roles = manifest.resources.find((resource) => resource.code === "roles");
      const groupCode = roles.schema.fields.find((field) => field.key === "groupCode");
      groupCode.relation.display = "";
      delete groupCode.relation.parentField;
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /roles must declare groupCode tree relation to role-groups/);
  });

  it("rejects role groups that try to grant permissions or inheritance", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roleGroups = manifest.resources.find((resource) => resource.code === "role-groups");
      roleGroups.schema.fields.push(
        {
          key: "permissions",
          label: { zh: "权限", en: "Permissions" },
          type: "multiselect",
          source: "values",
          inTable: true,
          inForm: true,
        },
        {
          key: "inheritFrom",
          label: { zh: "继承自", en: "Inherit From" },
          type: "text",
          source: "values",
          inTable: false,
          inForm: true,
        },
        {
          key: "memberUserCodes",
          label: { zh: "成员用户", en: "Member Users" },
          type: "multiselect",
          source: "values",
          inTable: false,
          inForm: true,
        },
      );
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /role-groups must stay classification-only/);
    assert.match(result.stderr, /permissions/);
    assert.match(result.stderr, /inheritFrom/);
    assert.match(result.stderr, /memberUserCodes/);
  });

  it("rejects role groups that omit tree classification metadata", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roleGroups = manifest.resources.find((resource) => resource.code === "role-groups");
      roleGroups.schema.fields = roleGroups.schema.fields.filter((field) => field.key !== "parentCode");
      for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
        roleGroups.schema[listName] = (roleGroups.schema[listName] ?? []).filter((field) => field !== "parentCode");
      }
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /role-groups must declare parentCode tree relation to role-groups/);
  });
});
