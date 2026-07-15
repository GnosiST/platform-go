import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import {
  adminServiceObjectDefinitions,
  validateAdminServiceObjectDefinition,
} from "./admin-service-object-definitions.mjs";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-admin-service-object-definitions.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function temporarySource(name, content) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-object-definitions-"));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, content);
  return filePath;
}

describe("validate-admin-service-object-definitions", () => {
  it("accepts typed menu definitions, string-set results and additional permissions", () => {
    const definition = {
      ...adminServiceObjectDefinitions.queries[0],
      id: "platform.navigation.menu-definition.contract-test",
      codegenName: "MenuDefinitionContractTest",
      clientMethod: "menuDefinitionContractTest",
      arguments: [{ name: "definition", type: "menu-definition", required: true }],
      result: [
        { name: "definition", type: "menu-definition" },
        { name: "permissionCodes", type: "string-set" },
      ],
      additionalPermissions: [{ permission: "admin:permission:read", action: "read" }],
    };

    assert.doesNotThrow(() => validateAdminServiceObjectDefinition("query", definition));
  });

  it("rejects malformed, duplicated and primary-duplicating additional permissions", () => {
    const base = {
      ...adminServiceObjectDefinitions.queries[0],
      id: "platform.navigation.menu-definition.permission-test",
      codegenName: "MenuDefinitionPermissionTest",
      clientMethod: "menuDefinitionPermissionTest",
    };
    for (const additionalPermissions of [
      [{ permission: "", action: "read" }],
      [{ permission: "admin:permission read", action: "read" }],
      [
        { permission: "admin:permission:read", action: "read" },
        { permission: "admin:permission:read", action: "read" },
      ],
      [{ permission: base.permission, action: base.action }],
    ]) {
      assert.throws(
        () => validateAdminServiceObjectDefinition("query", { ...base, additionalPermissions }),
        /additional permission/i,
      );
    }
  });

  it("accepts the Go and JS service-object registry", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated Admin service-object Go\/JS definition consistency/);
  });

  it("rejects a navigation runtime object missing from the JS definition registry", () => {
    const source = fs.readFileSync(
      path.join(repoRoot, "internal/platform/organizationrbac/navigation_service_objects.go"),
      "utf8",
    );
    const changed = `${source}\nvar _ = serviceobject.QueryDefinition{ID: UnmirroredNavigationQueryID}\n`;
    const result = runValidator(["--navigation-source", temporarySource("navigation_service_objects.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /navigation_service_objects\.go service-object IDs are not fully mirrored/);
  });

  it("rejects a Go ID that differs from the generated contract", () => {
    const source = fs.readFileSync(path.join(repoRoot, "internal/platform/organizationrbac/service_objects.go"), "utf8");
    const changed = source.replace(
      'OrganizationRolePoolQueryID                 = "platform.identity.organization-role-pool.get"',
      'OrganizationRolePoolQueryID                 = "platform.identity.organization-role-pool.changed"',
    );
    const result = runValidator(["--organization-source", temporarySource("service_objects.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Go ID constant does not match the JS definition/);
  });

  it("rejects argument type drift between Go and JS", () => {
    const source = fs.readFileSync(path.join(repoRoot, "internal/platform/organizationrbac/service_objects.go"), "utf8");
    const changed = source.replace(
      'Name: "roleGroupCodes", Type: serviceobject.ValueStringSet',
      'Name: "roleGroupCodes", Type: serviceobject.ValueString',
    );
    const result = runValidator(["--organization-source", temporarySource("service_objects.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Go and JS definition fields differ/);
  });

  it("rejects additional permission drift between Go and JS", () => {
    const source = fs.readFileSync(path.join(repoRoot, "internal/platform/serviceobject/reference.go"), "utf8");
    const changed = source.replace(
      'Permission: "admin:reference-records:read", Action: "read",',
      'Permission: "admin:reference-records:read", Action: "read", AdditionalPermissions: []PermissionRequirement{{Permission: "admin:permission:read", Action: "read"}},',
    );
    const result = runValidator(["--reference-source", temporarySource("reference.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Go and JS definition fields differ/);
  });

  it("rejects shared cost policy drift between Go and JS", () => {
    const source = fs.readFileSync(path.join(repoRoot, "internal/platform/organizationrbac/service_objects.go"), "utf8");
    const changed = source.replace(
      "baseCost := serviceobject.CostPolicy{BaseCost: 5, PerRowCost: 1, Limit: 2005}",
      "baseCost := serviceobject.CostPolicy{BaseCost: 5, PerRowCost: 1, Limit: 9999}",
    );
    const result = runValidator(["--organization-source", temporarySource("service_objects.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Go and JS definition fields differ/);
  });

  it("rejects lifecycle integer-bound drift between Go and JS", () => {
    const source = fs.readFileSync(
      path.join(repoRoot, "internal/platform/organizationrbac/lifecycle_service_objects.go"),
      "utf8",
    );
    const changed = source.replace(
      "minimumRetention, maximumRetention := int64(1), int64(36500)",
      "minimumRetention, maximumRetention := int64(1), int64(36501)",
    );
    const result = runValidator(["--lifecycle-source", temporarySource("lifecycle_service_objects.go", changed)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Go and JS definition fields differ/);
  });
});
