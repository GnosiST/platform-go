import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function run(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-data-lifecycle-retention.mjs", ...args], { cwd: repoRoot, encoding: "utf8" });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-data-lifecycle-"));
  const file = path.join(dir, name);
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
  return file;
}

describe("validate-platform-data-lifecycle-retention", () => {
  it("accepts the implemented lifecycle contract", () => {
    const result = run();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform data lifecycle retention governance/);
  });

  it("rejects deletion policy and public purge drift", () => {
    const contract = readJSON("resources/generated/admin-resource-contract.json");
    contract.resources.find((item) => item.name === "auditLogs").deletion.mode = "hard-delete";
    const contractResult = run(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(contractResult.status, 0);
    assert.match(contractResult.stderr, /auditLogs must preserve its reviewed deletion policy/);

    const openapi = readJSON("resources/generated/openapi.admin.json");
    openapi.paths["/api/admin/resources/files/{id}/purge"] = { post: {} };
    const openapiResult = run(["--openapi", tempJSON("openapi.json", openapi)]);
    assert.notEqual(openapiResult.status, 0);
    assert.match(openapiResult.stderr, /must not expose maintenance purge/);
  });

  it("rejects overstated runtime boundaries", () => {
    const engineering = readJSON("resources/platform-engineering-capabilities.json");
    engineering.capabilities.find((item) => item.id === "data-lifecycle-retention").evidence.runtimeBoundary.dynamicDatasourceRouting = "implemented";
    const result = run(["--engineering", tempJSON("engineering.json", engineering)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /boundary must remain explicit and non-expansive/);
  });

  it("rejects lifecycle closeout without the completed phase cleanup", () => {
    const closeout = readJSON("resources/platform-node-closeout-audit.json");
    const lifecycle = closeout.nodeCloseouts.find((item) => item.taskId === "data-lifecycle-retention");
    lifecycle.neatFreak = false;
    lifecycle.cleanupMode = "focused";

    const result = run(["--closeout", tempJSON("closeout.json", closeout)]);

    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /completed phase-level neat-freak cleanup/);
  });
});
