import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-datasource-registry.mjs", ...args], { cwd: repoRoot, encoding: "utf8" });
}

function tempJSON(value) {
  const filePath = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "platform-datasource-registry-")), "contract.json");
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-datasource-registry", () => {
  it("accepts the datasource registry contract", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects a client-routable or credential-bearing contract", () => {
    const contract = JSON.parse(fs.readFileSync(path.join(repoRoot, "resources/platform-datasource-registry.json"), "utf8"));
    contract.clientPhysicalRouting = "allowed";
    contract.connectionMaterial = "dsn";
    const result = runValidator(["--contract", tempJSON(contract)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must retain process-owned config references only/);
    assert.match(result.stderr, /must forbid client physical routing/);
  });
});
