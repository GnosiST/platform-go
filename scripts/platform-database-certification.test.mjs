import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-database-certification.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "platform-database-certification-"));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-database-certification", () => {
  it("accepts the current experimental and unsupported support boundary", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated database certification matrix/);
  });

  it("rejects a missing experimental lane", () => {
    const matrix = readJSON("resources/platform-database-certification-matrix.json");
    matrix.entries = matrix.entries.filter((entry) => entry.driver !== "postgres");
    const result = runValidator(["--matrix", tempJSON("matrix.json", matrix)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must include postgres\/16\/gorm-storage-smoke as experimental/);
  });

  it("rejects unsafe evidence references", () => {
    const matrix = readJSON("resources/platform-database-certification-matrix.json");
    matrix.entries[0].evidenceRef = "/tmp/secret";
    const result = runValidator(["--matrix", tempJSON("matrix.json", matrix)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /evidenceRef is missing or unsafe/);
  });
});
