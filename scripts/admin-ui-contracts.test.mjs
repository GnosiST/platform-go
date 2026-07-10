import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-admin-ui-contracts.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempAdminRoot() {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "admin-ui-contracts-"));
  fs.mkdirSync(path.join(tempDir, "admin"), { recursive: true });
  fs.cpSync(path.join(repoRoot, "admin", "src"), path.join(tempDir, "admin", "src"), { recursive: true });
  return tempDir;
}

function replaceInTemp(tempRoot, relativePath, from, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  assert.ok(source.includes(from), `${relativePath} should contain ${from}`);
  fs.writeFileSync(filePath, source.split(from).join(to));
}

describe("validate-admin-ui-contracts", () => {
  it("accepts the current componentized admin UI contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Admin UI contract validation passed/);
  });

  it("rejects a table fork that drops the shared pagination bar", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "PlatformPaginationBar",
      "LocalPaginationBar",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must use the shared pagination bar/);
  });

  it("rejects a settings drawer that drops import/export configuration support", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "exportConfig",
      "downloadConfig",
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "importConfig",
      "uploadConfig",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /SystemSettingsDrawer must keep exportConfig support/);
    assert.match(result.stderr, /SystemSettingsDrawer must keep importConfig support/);
  });
});
