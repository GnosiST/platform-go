import assert from "node:assert/strict";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-external-capability-example.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

describe("validate-external-capability-example", () => {
  it("runs the external capability example through public contracts", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated external capability example/);
  });

  it("rejects a missing example directory", () => {
    const result = runValidator(["--example-dir", "examples/missing-external-capability"]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /external capability example directory is missing/);
  });
});
