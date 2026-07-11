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

  it("rejects styles that remove reduced-motion support", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      "@media (prefers-reduced-motion: reduce)",
      "@media (prefers-reduced-motion: no-preference)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /styles\.css must respect reduced motion/);
  });

  it("rejects reduced-motion styles that omit body-portaled AntD modals", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".ant-modal-root",
      ".ant-modal-portal-root",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reduced motion must cover body-portaled AntD modal, drawer, dropdown, and popover roots/);
  });

  it("rejects resource modal focus handling outside the AntD open lifecycle", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "focusFirstEditableFormField(form, formFields);",
      "void form;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Resource modals must invoke the shared first-field focus helper/);
  });

  it("rejects resource modal fallback focus without current-visible-modal scoping", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "modal.getClientRects().length > 0",
      "true",
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
      "document.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /currently visible modal/);
    assert.match(result.stderr, /stay scoped to the visible modal/);
  });

  it("rejects resource modal fallback focus that includes disabled or read-only fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'input:not([type="hidden"]):not([disabled]):not([readonly])',
      'input:not([type="hidden"])',
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      '[tabindex]:not([tabindex="-1"]):not([disabled]):not([readonly]):not([aria-disabled="true"])',
      '[tabindex]:not([tabindex="-1"])',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /first enabled editable form control/);
    assert.match(result.stderr, /exclude disabled generic focus targets/);
  });

  it("rejects resource modal fallback focus without preventScroll", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "fieldControl?.focus({ preventScroll: true });",
      "fieldControl?.focus();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /fallback focus must prevent scroll jumps/);
  });

  it("rejects resource modal fallback focus that restores a global field-id lookup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)",
      "document.getElementById(firstField.key)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not depend on a global field id/);
  });

  for (const [name, selector, message] of [
    ["resource search", ".platform-data-table-panel .platform-table-search {", "Mobile resource search must expose a 44px touch target"],
    ["resource toolbar", ".platform-data-table-panel .table-actions .ant-btn {", "Mobile resource table actions must expose 44px touch targets"],
    ["pagination main controls", ".platform-pagination-main :where(.ant-pagination-prev, .ant-pagination-item, .ant-pagination-jump-prev, .ant-pagination-jump-next, .ant-pagination-next, .ant-pagination-item-link, a) {", "Mobile pagination main controls must expose 44px touch targets"],
    ["pagination quick jumper", ".platform-pagination-jumper .ant-input-number {", "Mobile pagination quick jumper must expose a 44px touch target"],
    ["settings close control", ".system-settings-drawer .ant-drawer-close {", "Mobile settings Drawer close control must expose a 44px touch target"],
    ["settings tab", ".settings-tabs .ant-tabs-tab {", "Mobile settings Drawer tab must expose a 44px touch target"],
    ["settings tab button", ".settings-tabs .ant-tabs-tab-btn {", "Mobile settings Drawer tab button must expose a 44px touch target"],
    ["settings overflow control", ".settings-tabs .ant-tabs-nav-more {", "Mobile settings Drawer overflow control must expose a 44px touch target"],
  ]) {
    it(`rejects mobile styles that shrink the ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(tempRoot, "admin/src/styles.css", selector, `${selector.slice(0, -1)}.missing {`);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }
});
