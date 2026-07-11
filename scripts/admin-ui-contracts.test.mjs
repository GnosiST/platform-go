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

  it("rejects a shell whose skip link does not target main content", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      'href="#platform-main-content"',
      'href="#missing-content"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminShell must expose a skip-to-content link/);
  });

  it("rejects a client that renames the session-expired event contract", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "ADMIN_SESSION_EXPIRED_EVENT",
      "ADMIN_SESSION_INVALID_EVENT",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /The shared client must expose the session-expired event contract/);
  });

  it("rejects session expiry handling that does not match the exact request token", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "getAuthToken() !== requestToken",
      "Boolean(getAuthToken())",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Session expiry must clear only the exact token used by the failed request/);
  });

  it("rejects auth bootstrap calls that use stored-token authentication", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      'auth: "none"',
      'auth: "stored-token"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Auth provider discovery must explicitly avoid stored-token authentication/);
    assert.match(result.stderr, /Auth login must explicitly avoid stored-token authentication/);
  });

  it("rejects localized session expiry state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "const [sessionExpired, setSessionExpired] = useState(false);",
      "const [sessionExpired, setSessionExpired] = useState(dictionary.sessionExpired);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /App must keep session expiry in stable non-localized state/);
  });

  it("rejects localized-string equality for session expiry", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      'setAuthError("");',
      'setAuthError((current) => current === dictionary.sessionExpired ? current : "");',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /App must not identify session expiry by comparing localized strings/);
  });
});
