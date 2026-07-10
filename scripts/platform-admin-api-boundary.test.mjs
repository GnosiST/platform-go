import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-admin-api-boundary.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-api-boundary-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-admin-api-boundary", () => {
  it("accepts the current admin API boundary contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated admin API boundary and query security/);
  });

  it("rejects direct fetch outside the platform API client allowlist", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.adminSourceBoundary.allowedDirectFetchFiles = [];
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin\/src\/platform\/api\/client\.ts:\d+ calls fetch\(\) outside the platform API client/);
  });

  it("rejects missing backend query safety snippets", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.requiredSourceSnippets.push({
      path: "internal/platform/adminresource/query.go",
      contains: "raw sql concatenation is allowed",
    });
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /internal\/platform\/adminresource\/query\.go is missing required snippet raw sql concatenation is allowed/);
  });

  it("rejects weakening the structured query policy", () => {
    const boundary = readJSON("resources/platform-admin-api-boundary.json");
    boundary.querySecurity.rawSQLAllowed = true;
    boundary.querySecurity.sensitiveFieldsAllowed = true;
    const boundaryPath = tempJSON("platform-admin-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /querySecurity\.rawSQLAllowed must stay false/);
    assert.match(result.stderr, /querySecurity\.sensitiveFieldsAllowed must stay false/);
  });
});
