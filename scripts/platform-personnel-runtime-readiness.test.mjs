import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-personnel-runtime-readiness.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-personnel-runtime-readiness-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-personnel-runtime-readiness", () => {
  it("accepts the current personnel optional runtime readiness contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated optional personnel runtime readiness/);
  });

  it("rejects default contracts that leak personnel resources", () => {
    const defaultContract = readJSON("resources/generated/admin-resource-contract.json");
    defaultContract.resources.push({
      name: "personnel-profiles",
      code: "personnel-profiles",
      permissions: { read: "admin:personnel-profile:read" },
    });
    const defaultContractPath = tempJSON("default-admin-resource-contract.json", defaultContract);

    const result = runValidator(["--default-admin-contract", defaultContractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /default admin contract must not include optional personnel resource personnel-profiles/);
  });

  it("rejects default capability audits that enable personnel", () => {
    const defaultAudit = readJSON("resources/generated/platform-capability-audit.json");
    defaultAudit.capabilities.push({
      id: "personnel",
      name: "Personnel",
      version: "0.1.0",
      adminResources: ["personnel-profiles", "positions", "position-assignments"],
    });
    const defaultAuditPath = tempJSON("default-platform-capability-audit.json", defaultAudit);

    const result = runValidator(["--default-audit", defaultAuditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /default capability audit must not enable personnel/);
  });

  it("rejects personnel-enabled contracts without OpenAPI, frontend or permission coverage", () => {
    const enabledContract = readJSON("resources/generated/admin-resource-contract.json");
    enabledContract.resources.push({
      name: "personnel-profiles",
      code: "personnel-profiles",
      label: { zh: "人员档案", en: "Personnel Profiles" },
      group: "foundation",
      menu: { path: "/personnel-profiles" },
      refine: { resource: "personnel-profiles", list: "/personnel-profiles", component: "ResourceTablePage" },
      permissions: { read: "admin:personnel-profile:read" },
      routes: [{ method: "POST", path: "/api/admin/resources/personnel-profiles/query", permission: "admin:personnel-profile:read" }],
      schema: { fields: [] },
    });
    enabledContract.permissions = (enabledContract.permissions ?? []).filter((permission) => permission !== "admin:personnel-profile:read");
    enabledContract.frontend = (enabledContract.frontend ?? []).filter((resource) => resource.resource !== "personnel-profiles");
    const enabledContractPath = tempJSON("enabled-admin-resource-contract.json", enabledContract);
    const enabledOpenAPI = readJSON("resources/generated/openapi.admin.json");
    const enabledOpenAPIPath = tempJSON("enabled-openapi.admin.json", enabledOpenAPI);

    const result = runValidator(["--personnel-admin-contract", enabledContractPath, "--personnel-openapi", enabledOpenAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /personnel contract missing permission admin:personnel-profile:read/);
    assert.match(result.stderr, /personnel contract frontend must include personnel-profiles/);
    assert.match(result.stderr, /personnel OpenAPI must include path \/api\/admin\/resources\/personnel-profiles\/query/);
  });
});
