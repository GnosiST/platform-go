import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { after, describe, it } from "node:test";

import "./admin-menu-governance-behavior.test.mjs";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const tempRoots = new Set();

function makeTempRoot(prefix) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  tempRoots.add(tempDir);
  return tempDir;
}

function cleanupTempRoot(tempDir) {
  fs.rmSync(tempDir, { recursive: true, force: true });
  tempRoots.delete(tempDir);
}

after(() => {
  for (const tempRoot of [...tempRoots]) {
    cleanupTempRoot(tempRoot);
  }
});

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-admin-ui-contracts.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempAdminRoot() {
  const tempDir = makeTempRoot("admin-ui-contracts-");
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

function replaceRegexInTemp(tempRoot, relativePath, pattern, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  assert.match(source, pattern);
  fs.writeFileSync(filePath, source.replace(pattern, to));
}

function replaceInTempIfPresent(tempRoot, relativePath, from, to) {
  const filePath = path.join(tempRoot, relativePath);
  const source = fs.readFileSync(filePath, "utf8");
  if (!source.includes(from)) return;
  fs.writeFileSync(filePath, source.split(from).join(to));
}

function runTypeScriptProbe(relativePath, body) {
  const moduleURL = pathToFileURL(path.join(repoRoot, relativePath)).href;
  return spawnSync(
    process.execPath,
    ["--experimental-strip-types", "--input-type=module", "--eval", body(moduleURL)],
    { cwd: repoRoot, encoding: "utf8" },
  );
}

function relationSearchSchedulerProbe(body) {
  return runTypeScriptProbe("admin/src/platform/resources/relationOptionSearch.ts", (moduleURL) => `
    import assert from "node:assert/strict";
    import { createRelationOptionSearchScheduler } from ${JSON.stringify(moduleURL)};

    function createFakeTimers() {
      let nextID = 1;
      const tasks = new Map();
      return {
        clearTimer(id) {
          tasks.delete(id);
        },
        pendingCount() {
          return tasks.size;
        },
        runAll() {
          const pending = [...tasks.values()];
          tasks.clear();
          for (const task of pending) task();
        },
        setTimer(task, delay) {
          assert.equal(delay, 250);
          const id = nextID++;
          tasks.set(id, task);
          return id;
        },
      };
    }

    ${body}
  `);
}

function runAdminClientProbe(body) {
  const tempDir = makeTempRoot("admin-api-client-probe-");
  try {
    const apiDir = path.join(tempDir, "admin", "src", "platform", "api");
    const errorSDKDir = path.join(tempDir, "resources", "generated", "error-sdk");
    fs.mkdirSync(apiDir, { recursive: true });
    const generator = spawnSync(
      process.execPath,
      ["scripts/generate-platform-error-code-artifacts.mjs", "--output-dir", errorSDKDir],
      { cwd: repoRoot, encoding: "utf8" },
    );
    assert.equal(generator.status, 0, generator.stderr);
    const clientSource = adminSource("admin/src/platform/api/client.ts")
      .replace('const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "/api";', 'const API_BASE = "/api";');
    const sessionExpirySource = adminSource("admin/src/platform/api/sessionExpiry.ts");
    fs.writeFileSync(path.join(apiDir, "client.ts"), clientSource);
    fs.writeFileSync(path.join(apiDir, "sessionExpiry.ts"), sessionExpirySource);
    const buildDir = path.join(tempDir, "build");
    const tsc = path.join(repoRoot, "admin", "node_modules", ".bin", "tsc");
    const compile = spawnSync(
      tsc,
      [
        "--target", "ES2022",
        "--module", "CommonJS",
        "--moduleResolution", "node",
        "--outDir", buildDir,
        path.join(apiDir, "client.ts"),
        path.join(apiDir, "sessionExpiry.ts"),
        path.join(errorSDKDir, "typescript", "errorContract.ts"),
      ],
      { cwd: tempDir, encoding: "utf8" },
    );
    assert.equal(compile.status, 0, compile.stderr || compile.stdout);
    const moduleURL = pathToFileURL(path.join(buildDir, "admin", "src", "platform", "api", "client.js")).href;
    return spawnSync(
      process.execPath,
      ["--input-type=module", "--eval", body(moduleURL)],
      { cwd: tempDir, encoding: "utf8" },
    );
  } finally {
    cleanupTempRoot(tempDir);
  }
}

function adminSource(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

describe("validate-admin-ui-contracts", () => {
  it("accepts the current componentized admin UI contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Admin UI contract validation passed/);
  });

  it("rejects restoring hover-open behavior on the profile dropdown", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      'trigger={["click"]}',
      'trigger={["click", "hover"]}',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile dropdown must not open on hover/);
  });

  it("rejects rendering the user name inside the avatar trigger", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "<Avatar size={28} className=\"admin-avatar\" src={avatarUrl || undefined}>\n                  {avatarLetter}\n                </Avatar>",
      "<Avatar size={28} className=\"admin-avatar\" src={avatarUrl || undefined}>\n                  {avatarLetter}\n                </Avatar>\n                <span className=\"profile-menu-name\">{displayName}</span>",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile avatar trigger must not render the user name/);
  });

  it("rejects showing a role count instead of role names in the profile panel", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "roles.map((role) => <Tag key={role}>{role}</Tag>)",
      "String(roles.length)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile summary must display role names instead of a role count/);
  });

  it("rejects removing outside-click close handling from the profile dropdown", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      'document.addEventListener("pointerdown", handleProfileOutsidePointerDown, true);',
      'document.addEventListener("pointerdown", () => undefined, true);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile outside-click handling must run before portal overlays swallow blank-area clicks/);
  });

  it("rejects clipping the profile dropdown instead of scrolling its body", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".profile-summary-body {",
      ".profile-summary-body.missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile summary body must remain scrollable/);
  });

  it("rejects profile editor modals that can grow past the viewport", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".profile-editor-modal .ant-modal-body {",
      ".profile-editor-modal .ant-modal-body.missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile editor modal body must stay scrollable within the viewport/);
  });

  it("rejects a theme provider that leaves body-portaled overlays outside platform tokens", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/AdminDesignProvider.tsx",
      "document.body.dataset.theme = themeName;",
      "void themeName;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin theme tokens must propagate to body-portaled overlays/);
  });

  it("rejects passing the complete role resource list to visible shell navigation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/App.tsx", "resources={navigationResources}", "resources={resources}");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminShell must receive only the projected role-management navigation resources/);
  });

  it("rejects highlighting a legacy role route without mapping it to the projected entry", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/App.tsx", "activeRoute={navigationActiveRoute}", "activeRoute={activeRoute}");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminShell must highlight the projected role-management route/);
  });

  it("rejects a dashboard role-management action that drops the role-groups-only fallback", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/dashboard/DashboardHome.tsx", "route: roleManagementResource.route", 'route: "/roles"');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Dashboard role management must target the projected authorized role resource/);
  });

  it("rejects dashboard role management without the shared deterministic selector", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/dashboard/DashboardHome.tsx",
      "const roleManagementResource = selectRoleManagementNavigationResource(",
      "const roleManagementResource = Array.from(",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Dashboard role management must use the shared deterministic resource selector/);
  });

  it("rejects using projected navigation resources for route page registration", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/App.tsx",
      /(<PlatformRoutePages[\s\S]*?resources=)\{resources\}/,
      "$1{navigationResources}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Platform route pages must retain the complete authorized resource list/);
  });

  it("rejects routing permissions back through the generic resource console", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      /if \(resource\.route === "\/permissions"\) \{[\s\S]*?\n  \}\n\n  if \(resource\.route === "\/sessions"\)/,
      'if (resource.route === "/sessions")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /permissions route must use the dedicated permission governance console/);
  });

  it("rejects permission governance without controlled custom API permission creation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      'await createAdminResource("permissions", input)',
      'await queryAdminResource("permissions", { page: 1, pageSize: 1 })',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must support controlled custom API permission creation/);
  });

  it("rejects permission governance that does not guard edits to custom API permissions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      "isCustomAPIPermission",
      "isPermissionEditable",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must guard edit actions to custom API permission records/);
  });

  it("rejects permission governance that does not guard the custom permission submit path", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      "if ((editor.record && !canUpdate) || (!editor.record && !canCreate)) {\n      setError(dictionary.noPermission);\n      return;\n    }\n    savingRef.current = true;",
      "savingRef.current = true;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must guard the custom permission submit path/);
  });

  it("rejects custom permission forms that do not validate resource/code mismatches", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      "resourceMatchesPermissionCodeRule(form, dictionary.permissionCustomCodeResourceMismatch),",
      "",
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      "if (parts.resource !== resource) {\n    throw new Error(dictionary.permissionCustomCodeResourceMismatch);\n  }\n  ",
      "",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must validate that custom permission codes match the declared resource/);
  });

  it("rejects tree workbenches without shared summaries and controlled expansion", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/ui/AdminTreeWorkbench.tsx", "summary?: ReactNode;", "");
    replaceInTemp(tempRoot, "admin/src/platform/ui/AdminTreeWorkbench.tsx", "expandedKeys={expandedKeys}", "defaultExpandedKeys={expandedKeys}");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must expose a shared summary slot/);
    assert.match(result.stderr, /must use controlled expansion/);
  });

  it("rejects permission tree sorting that infers hierarchy from colon-delimited permission codes", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/PermissionGovernanceConsole.tsx",
      "const leftLevel = permissionTreeNodeLevel(left);\n  const rightLevel = permissionTreeNodeLevel(right);",
      'const leftLevel = String(left.key).split(":").length;\n  const rightLevel = String(right.key).split(":").length;',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not derive hierarchy depth from colon-delimited permission codes/);
  });

  it("rejects system settings that restore the unclassified status grid", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "settings-summary-groups",
      "settings-status-grid",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /old cramped status grid/);
  });

  it("rejects settings center resources that are no longer projected from manifests", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/settings/SettingsCenterConsole.tsx",
      "projectSettingsResourceConfigs(runtimeItems, resources, dictionary, language)",
      "projectSettingsResourceConfigs([], resources, dictionary, language)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must project configuration entries from settings runtime items/);
  });

  it("rejects settings center pages that drop dictionary parameters from system configuration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/settings/SettingsCenterConsole.tsx",
      'route: "/dictionary-parameters"',
      'route: "/dictionary-links"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /dictionary parameters as a system-level configuration entry/);
  });

  it("rejects settings center pages that drop manifest-backed capability configuration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/settings/SettingsCenterConsole.tsx",
      '"manifest" as const',
      '"catalog" as const',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /dynamic manifest-backed configuration path/);
  });

  it("rejects removing the formal system settings route", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/App.tsx", 'path="/settings"', 'path="/interface-preferences"');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /System settings must be a first-class route/);
  });

  it("rejects settings center pages that blur system settings with topbar preferences", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/settings/SettingsCenterConsole.tsx",
      "dictionary.settingsCenterInterfacePreferenceBoundary",
      "dictionary.settingsCenterSystemSettingsDescription",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /topbar settings scoped to interface preferences/);
  });

  it("rejects settings center pages that hide writable and schema footprint state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/settings/SettingsCenterConsole.tsx",
      "item.schema?.fields?.length ?? 0",
      "0",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /schema field footprint/);
  });

  it("rejects message center pages that drop common SMS, email, and WeChat channels", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      '["in_app", "sms", "email", "wechat_official", "wechat_miniapp"]',
      '["in_app"]',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Message center must expose the common in-app, SMS, email, and WeChat channel set/);
  });

  it("rejects message center pages that remove the operating loop workbench", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "<MessageCenterClosedLoop",
      "<MessageCenterResourceTabs",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must render the operating loop/);
  });

  it("rejects message center pages that reduce the runtime loop to a partial resource list", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "messageCenterClosedLoopSteps(resourceConfigs, records, resourceRoutes, dictionary)",
      "messageCenterClosedLoopSteps([resourceConfigs[0]], records, resourceRoutes, dictionary)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /full notification resource sequence/);
  });

  it("rejects message center pages that present dry run without runtime connected explanation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "dictionary.messageCenterTrialReadyTitle",
      "dictionary.messageCenterRuntimeOverview",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /test send is connected to the runtime endpoint/);
  });

  it("rejects message center channel cards that collapse runtime readiness into generic configured state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "<Tag color={card.statusTone}>{card.statusLabel}</Tag>",
      '<Tag color={card.ready ? "success" : "default"}>{card.ready ? dictionary.configured : dictionary.notConfigured}</Tag>',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /generic configured tag/);
  });

  it("rejects message center channel cards that hide the multi-channel dry-run path", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "testConnectEnabled: true",
      'testConnectEnabled: channel === "sms"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /dry-run path for all common channels/);
  });

  it("rejects message center placeholder channels labeled as real external test-connect", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "dictionary.messageCenterLocalDryRun",
      "dictionary.messageCenterTestConnect",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /local dry-run action label/);
  });

  it("rejects message center channel cards that do not prefill the clicked channel", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/message-center/MessageCenterConsole.tsx",
      "openTestSend(undefined, undefined, normalizeMessageCenterChannel(card.key))",
      "openTestSend()",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /prefill dry-run with the clicked channel/);
  });

  it("rejects capability details that no longer open from the list into a modal", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/CapabilityConsole.tsx",
      'className="capability-detail-modal"',
      'className="capability-detail-drawer"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Capability detail must render in a modal card/);
  });

  it("rejects capability details that hide install impact configuration resources", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/CapabilityConsole.tsx",
      "dictionary.capabilityConfigResources",
      "dictionary.capabilityResources",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Capability detail must show contributed configuration resources/);
  });

  it("rejects capability restart state that only handles install-pending changes", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/CapabilityConsole.tsx",
      "const pendingRestart = enabled !== desired;",
      "const pendingRestart = !enabled && desired;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /cover both enable and disable pending changes/);
  });

  it("rejects optional notification metadata without pre-install impact preview", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/metadata.ts",
      'menuContribution("/message-center"',
      'menuContribution("/notifications"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Optional notification capability must preview the message-center route before installation/);
  });

  it("rejects restoring the verbose Role Management navigation label", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/i18n.ts", 'roleManagement: "Roles"', 'roleManagement: "Role Management"');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role navigation must declare the concise English label/);
  });

  it("rejects restoring the legacy Role Governance page H1", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/i18n.ts", 'roleGovernanceTitle: "Role Management"', 'roleGovernanceTitle: "Role Governance"');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Shared role governance page must display Role Management as its English H1/);
  });

  it("rejects global search copy that implies unavailable API or document search", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/i18n.ts", 'topSearch: "Search accessible pages..."', 'topSearch: "Search capabilities, APIs, docs..."');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Global search copy must describe page navigation/);
  });

  it("rejects generic session table action labels", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/SessionConsole.tsx", "refresh: dictionary.sessionRefreshList", "refresh: dictionary.refresh");
    replaceInTemp(tempRoot, "admin/src/platform/resources/SessionConsole.tsx", "columns: dictionary.sessionColumnSettings", "columns: dictionary.tableColumns");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Session console refresh action must use read-only session-specific copy/);
  });

  it("rejects resource writes that narrow schema values to strings", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "return isSchemaValue(value) ? value : undefined;",
      'return typeof value === "string" ? value : undefined;',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /valid arrays, booleans or numbers/);
  });

  it("rejects resource writes that stringify array boundaries", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "const value = schemaValueFromFormValue(raw);",
      'const value = Array.isArray(raw) ? JSON.stringify(raw.map((item) => String(item))) : schemaValueFromFormValue(raw);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not stringify multiselect arrays/);
  });

  it("rejects relation option loading without stale-response protection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "relationSearchScheduler.isCurrent",
      "removedRelationSearchScheduler.isCurrent",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must discard stale responses/);
  });

  it("executes only the latest pending relation search for one field", () => {
    const result = relationSearchSchedulerProbe(`
      const timers = createFakeTimers();
      const scheduler = createRelationOptionSearchScheduler({
        delayMs: 250,
        setTimer: timers.setTimer,
        clearTimer: timers.clearTimer,
      });
      const calls = [];
      scheduler.schedule("orgId", () => calls.push("first"));
      const generation = scheduler.schedule("orgId", () => calls.push("second"));
      assert.equal(timers.pendingCount(), 1);
      timers.runAll();
      assert.deepEqual(calls, ["second"]);
      assert.equal(scheduler.isCurrent("orgId", generation), true);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("keeps pending relation searches isolated by field", () => {
    const result = relationSearchSchedulerProbe(`
      const timers = createFakeTimers();
      const scheduler = createRelationOptionSearchScheduler({
        delayMs: 250,
        setTimer: timers.setTimer,
        clearTimer: timers.clearTimer,
      });
      const calls = [];
      scheduler.schedule("orgId", () => calls.push("orgId"));
      scheduler.schedule("roleId", () => calls.push("roleId"));
      assert.equal(timers.pendingCount(), 2);
      timers.runAll();
      assert.deepEqual(calls.sort(), ["orgId", "roleId"]);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("invalidates pending relation searches when the form session closes", () => {
    const result = relationSearchSchedulerProbe(`
      const timers = createFakeTimers();
      const scheduler = createRelationOptionSearchScheduler({
        delayMs: 250,
        setTimer: timers.setTimer,
        clearTimer: timers.clearTimer,
      });
      const calls = [];
      scheduler.schedule("orgId", () => calls.push("stale"));
      scheduler.invalidateAll();
      assert.equal(timers.pendingCount(), 0);
      timers.runAll();
      assert.deepEqual(calls, []);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects in-flight relation search generations after invalidation", () => {
    const result = relationSearchSchedulerProbe(`
      const timers = createFakeTimers();
      const scheduler = createRelationOptionSearchScheduler({
        delayMs: 250,
        setTimer: timers.setTimer,
        clearTimer: timers.clearTimer,
      });
      let staleGeneration = 0;
      scheduler.schedule("orgId", (generation) => {
        staleGeneration = generation;
      });
      timers.runAll();
      assert.equal(scheduler.isCurrent("orgId", staleGeneration), true);
      scheduler.invalidateAll();
      assert.equal(scheduler.isCurrent("orgId", staleGeneration), false);
      const nextGeneration = scheduler.schedule("orgId", () => {});
      assert.notEqual(nextGeneration, staleGeneration);
      assert.equal(scheduler.isCurrent("orgId", staleGeneration), false);
      assert.equal(scheduler.isCurrent("orgId", nextGeneration), true);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects closing a resource form without invalidating relation searches", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "const closeFormModal = () => {\n    relationSearchScheduler.invalidateAll();",
      "const closeFormModal = () => {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /form close must invalidate relation searches/);
  });

  it("rejects a successful save that bypasses relation search session cleanup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "setSelectedID(String(result.id));\n      closeFormModal();",
      "setSelectedID(String(result.id));\n      setModalOpen(false);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /successful save must close the relation search session/);
  });

  it("rejects resource cleanup that leaves relation searches active", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "return () => {\n      relationSearchScheduler.invalidateAll();\n    };\n  }, [relationSearchScheduler, resourceKey]);",
      "return () => {};\n  }, [relationSearchScheduler, resourceKey]);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Resource cleanup must invalidate relation searches/);
  });

  it("rejects interactive hover feedback on read-only runtime context chips", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".context-chip:not(.context-readonly):hover",
      ".context-chip:hover",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not expose interactive hover feedback/);
  });

  it("rejects an Admin runtime that enables Refine third-party telemetry", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "disableTelemetry: true",
      "disableTelemetry: false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must disable Refine third-party telemetry by default/);
  });

  it("rejects organization and user routes without the domain experience", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'experienceKey={resource.route === "/org-units" || resource.route === "/users" ? "organization-user" : undefined}',
      "experienceKey={undefined}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Organization and user routes must inject the shared organization-user experience/);
  });

  it("rejects role routes that bypass the shared governance console", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'resource.route === "/roles" || resource.route === "/role-groups"',
      'resource.route === "/roles"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role and role-group routes must share the role governance console/);
  });

  it("rejects a menus route that falls through to the generic resource console", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/refine/ResourceRoutePage.tsx",
      'resource.route === "/menus"',
      'resource.route === "/menus-disabled"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menus route must use the dedicated menu governance console/);
  });

  it("rejects menu governance that exposes page metadata while authoring a directory", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "{!directoryMode ? (",
      "{true ? (",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Page-only route, parameter, and button controls must stay hidden during directory authoring/);
  });

  it("rejects menu parameter rows without the forbidden-input guard", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "isForbiddenMenuParameterStringValue(value)",
      "false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must reject scripts, expressions, SQL, and physical routing inputs/);
  });

  it("rejects internal routes that reuse parameter keyword blocking", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "isSafeInternalMenuRoute(route)",
      "!isForbiddenMenuParameterStringValue(route)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must use route-specific validation instead of parameter-value keyword blocking/);
  });

  it("rejects menu page buttons without current-menu metadata binding", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "button.menuCode === values.code.trim()",
      "button.menuCode.length > 0",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Page-button metadata must point to the current menu code/);
  });

  it("rejects menu governance search that can apply a stale response", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "if (menuListRequest.current !== requestID) return;",
      "if (false) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu search must discard stale responses/);
  });

  it("rejects menu governance that requests records without menu read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "if (!canRead || menuListRequest.current !== requestID) return;",
      "if (menuListRequest.current !== requestID) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must fail closed before requesting records without read access/);
  });

  it("rejects menu definition loading that can retain stale detail state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "setDefinitionLoading(false);\n      setSelectedDefinition(null);\n      setSelectedRevision(0);",
      "setSelectedDefinition(null);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Clearing menu selection must also clear loading and revision state/);
  });

  it("rejects menu parent changes without structural confirmation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "await confirmMenuParentChange(modal.confirm, dictionary, editor.definition, definition, records)",
      "true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require an explicit localized structural confirmation/);
  });

  it("rejects page-button rows without duplicate permission-code validation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "duplicateButtonPermission(form, index, dictionary.menuButtonPermissionDuplicate)",
      "safeCodeRule(dictionary.menuButtonPermissionInvalid)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /permission codes must expose duplicate validation/);
  });

  it("rejects menu saves without editor-session isolation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "const editorSession = useRef(0);",
      "const editorSessionMissing = useRef(0);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu saves must ignore stale mutation completions/);
  });

  it("rejects menu saves without a synchronous submission lock", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "const savingRef = useRef(false);",
      "const savingRefMissing = useRef(false);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu saves must acquire a synchronous single-flight lock/);
  });

  it("rejects menu save focus restoration without a stable detail target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "returnFocusRef.current = detailFocusRef.current;",
      "returnFocusRef.current = returnFocusRef.current;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Successful menu saves must restore focus to a stable detail target/);
  });

  it("rejects menu modal controls without complete 44px targets", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".menu-governance-modal .ant-modal-close,",
      ".menu-governance-modal .ant-modal-close-missing,",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu governance modal controls must expose 44px targets/);
  });

  it("rejects tree workbench search without a tablet touch target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/styles.css",
      ".admin-tree-workbench-navigation .admin-list-toolbar .ant-input-affix-wrapper {",
      ".admin-tree-workbench-navigation .admin-list-toolbar .ant-input-affix-wrapper-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree workbench search must expose a 44px tablet touch target/);
  });

  it("rejects tree workbench expanders without a 44px target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".admin-tree-workbench-tree .ant-tree-switcher {",
      ".admin-tree-workbench-tree .ant-tree-switcher-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree workbench expanders must expose a 44px pointer target/);
  });

  it("rejects menu tree keyboard handling without a selected active node", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/AdminTreeWorkbench.tsx",
      "activeKey={activeKey}",
      "activeKey={null}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must expose the selected menu node to Ant Tree keyboard handling/);
  });

  it("rejects menu governance that bypasses generated menu-definition service objects", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/organizationRBAC.ts",
      "client.replaceMenuDefinition",
      "transport.post",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menu-definition replace wrapper must use the generated service-object client/);
  });

  it("rejects menu governance that drops the runtime legacy branch", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      'if (menuWriteMode === "legacy")',
      'if (menuWriteMode === "target")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must keep legacy updates separate from target service-object writes/);
  });

  it("rejects menu governance that drops missing legacy directory projection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "projectMenuGovernanceRecords(rawRecords, nextWriteMode, menuDirectoryLabels())",
      "rawRecords",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must project missing directory ancestors/);
  });

  it("rejects layout options without an accessible selected state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/SystemSettingsDrawer.tsx",
      "aria-pressed={active === mode}",
      "aria-pressed={false}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Layout-mode options must expose their selected state/);
  });

  it("rejects menu creation that hard-codes the initial global revision", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "createMenuDefinition(definition, selectedRevision)",
      "createMenuDefinition(definition, 0)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Menu creation must use the most recent trusted global menu revision/);
  });

  it("rejects menu saves that retain a stale selected-definition revision", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/MenuGovernanceConsole.tsx",
      "setDefinitionRefresh((current) => current + 1);",
      "void selectedRevision;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must reload the normalized definition and its new global revision/);
  });

  it("rejects role governance that queries a resource the principal cannot read", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'canReadGroups ? loadAllRecords("role-groups") : Promise.resolve([])',
      'loadAllRecords("role-groups")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not request role or role-group resources the current principal cannot read/);
  });

  it("rejects role permission assignment that omits a supporting read permission", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions) && hasPermission(permissions, "admin:area-code:read", deniedPermissions);',
      'const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require every resource read permission used by its editor/);
  });

  it("rejects read-only role menu assignment without menu read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      '{canReadMenus ? <AdminActionButton ref={menuTriggerRef}',
      '{true ? <AdminActionButton ref={menuTriggerRef}',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must be hidden when menu records cannot be read/);
  });

  it("requires cutover-gated role menu assignment to persist page leaves only", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "directory-a", name: "Directory A", status: "enabled", nodeType: "directory" },
          { code: "page-a", name: "Page A", status: "enabled", nodeType: "page", parentCode: "directory-a" },
        ],
        [],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      assert.deepEqual(pageMenuCodes(nodes, ["directory-a", "page-a"]), ["page-a"]);
    `);

    assert.match(roleGovernance, /const menuCodes = pageMenuCodes\(menuTreeNodes\(menus, menuAssignment\.menuCodes, dictionary, language\), menuAssignment\.menuCodes\);/);
    assert.doesNotMatch(roleGovernance, /menuCodes:\s*legacyVisibleMenus/);
    assert.equal(result.status, 0, result.stderr);
  });

  it("requires Tree Transfer directory state to be derived without persisting branch keys", () => {
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(treeTransfer, /const index = useMemo\(\(\) => buildTreeTransferIndex\(nodes\), \[nodes\]\);/);
    assert.match(treeTransfer, /const normalizedValue = useMemo\(\(\) => leafValues\(index, value\), \[index, value\]\);/);
    assert.match(treeTransfer, /halfCheckedKeys/);
    assert.match(treeTransfer, /checkStrictly/);
  });

  it("requires disabled and missing historical role menu selections to stay removable", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "disabled-page", name: "Disabled Page", status: "disabled", nodeType: "page" }],
        ["disabled-page", "missing-page"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      assert.equal(nodes.find((node) => node.key === "disabled-page")?.availableDisabledReason, "Disabled");
      assert.equal(nodes.find((node) => node.key === "missing-page")?.availableDisabledReason, "Missing");
      assert.deepEqual(pageMenuCodes(nodes, ["disabled-page", "missing-page"]), ["disabled-page", "missing-page"]);
    `);

    assert.match(roleGovernance, /menuTreeNodes\(menus, historicalCodes, dictionary, language\)/);
    assert.equal(result.status, 0, result.stderr);
  });

  it("requires role update plus menu read permission while schema cutover controls menu writes", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const roleRuntime = adminSource("admin/src/platform/resources/roleGovernanceRuntime.ts");

    assert.match(roleGovernance, /const canAssignMenus = canReadMenus && canUpdateRole;/);
    assert.match(roleGovernance, /resolveRoleMenuAccess\(roleMenuTargetEnabled, canAssignMenus, menuAssignment\?\.role\.status \?\? ""\)/);
    assert.match(roleGovernance, /readOnly=\{menuAccess\.readOnly\}/);
    assert.match(roleGovernance, /!resolveRoleMenuAccess\(roleMenuTargetEnabled, canAssignMenus, menuAssignment\.role\.status\)\.editable/);
    assert.match(roleRuntime, /roleMenuTargetEnabled: targetIdentityRuntime && targetMenuSchema/);
  });

  it("requires one generated-client role menu prepare, impact and apply flow without client chunks", () => {
    const organizationRBAC = adminSource("admin/src/platform/api/organizationRBAC.ts");

    assert.match(organizationRBAC, /client\.getRoleMenus/);
    assert.match(organizationRBAC, /client\.prepareRoleMenuChange/);
    assert.match(organizationRBAC, /client\.getRoleMenuChangeImpact/);
    assert.match(organizationRBAC, /client\.replaceRoleMenus/);
    assert.doesNotMatch(organizationRBAC, /menuCodes\.(?:slice|splice)\(/);
  });

  it("keeps legacy menu visibility independent from target assignment reads", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(roleGovernance, /const targetRequest = roleMenuTargetEnabled \? getRoleMenus\(role\.code\) : Promise\.resolve\(null\);/);
    assert.match(roleGovernance, /menuCodes: targetAssignment \? \[\.\.\.targetAssignment\.menuCodes\] : \[\]/);
    assert.match(roleGovernance, /const value = migrationReadOnly \? legacyVisible : menuAssignment\?\.menuCodes \?\? \[\];/);
  });

  it("selects permission catalogs by permission write mode while menus use the schema runtime", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const permissionWorkflow = adminSource("admin/src/platform/resources/rolePermissionWorkflow.ts");
    const openAuthorization = roleGovernance.slice(roleGovernance.indexOf("const openAuthorization"), roleGovernance.indexOf("const saveAuthorization"));

    assert.match(openAuthorization, /const writeMode = permissionWriteMode;/);
    assert.match(openAuthorization, /loadRolePermissionCatalog\(writeMode, role\.code/);
    assert.match(permissionWorkflow, /writeMode === "target-domain" \? sources\.target\(roleCode\) : sources\.generic\(\)/);
    assert.doesNotMatch(openAuthorization, /roleMenuMigrationWriteEnabled/);
    assert.match(roleGovernance, /menus\.length > 0 \? Promise\.resolve\(menus\) : roleMenuTargetEnabled \? assignmentMenuRecords\(role\.code\) : loadAllRecords\("menus"\)/);
    assert.match(roleGovernance, /const targetRequest = roleMenuTargetEnabled \? getRoleMenus\(role\.code\) : Promise\.resolve\(null\);/);
  });

  it("requires stale role-menu opens and closes to invalidate older async results", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const openMenus = roleGovernance.slice(roleGovernance.indexOf("const openMenus"), roleGovernance.indexOf("const closeMenus"));

    assert.match(roleGovernance, /const menuRequest = useRef\(0\);/);
    assert.match(openMenus, /const requestID = \+\+menuRequest\.current;/);
    assert.ok((openMenus.match(/if \(menuRequest\.current !== requestID\) return;/g) ?? []).length >= 2);
    assert.match(roleGovernance, /const closeMenus = \(\) => \{\s*menuRequest\.current \+= 1;\s*setMenuAssignment\(null\);\s*\};/);
  });

  it("requires role-menu impact to match the opened snapshot before apply", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const guard = roleGovernance.indexOf("!roleMenuImpactMatches(impact, menuAssignment, menuCodes)");
    const apply = roleGovernance.indexOf("await replaceRoleMenus(preview);");

    assert.match(roleGovernance, /initialMenuCodes: string\[\];/);
    assert.match(roleGovernance, /impact\.expectedRevision === assignment\.revision/);
    assert.match(roleGovernance, /impact\.previewId !== preview\.previewId \|\| impact\.impactHash !== preview\.impactHash/);
    assert.match(roleGovernance, /sameStringSet\(impact\.currentMenuCodes, assignment\.initialMenuCodes\)/);
    assert.match(roleGovernance, /sameStringSet\(impact\.proposedMenuCodes, proposedMenuCodes\)/);
    assert.match(roleGovernance, /impact\.changed === !sameStringSet\(assignment\.initialMenuCodes, proposedMenuCodes\)/);
    assert.match(roleGovernance, /setError\(dictionary\.changePreviewUnavailable\);\s*await openMenus\(menuAssignment\.role\);\s*return;/);
    assert.ok(guard >= 0 && apply > guard, "snapshot guard must run before apply");
  });

  it("requires explicit role-menu impact confirmation before apply", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const confirm = roleGovernance.indexOf("if (!await confirmRoleMenuImpact(modal.confirm, dictionary, impact)) return;");
    const apply = roleGovernance.indexOf("await replaceRoleMenus(preview);");

    assert.match(roleGovernance, /impact\.currentMenuCodes\.join\(", "\)/);
    assert.match(roleGovernance, /impact\.proposedMenuCodes\.join\(", "\)/);
    assert.match(roleGovernance, /dictionary\.currentContext/);
    assert.match(roleGovernance, /dictionary\.reviewAndApply/);
    assert.ok(confirm >= 0 && apply > confirm, "impact cancellation must stop apply");
  });

  it("keeps missing historical menu selections visible under a removable selected branch", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");

    assert.match(roleGovernance, /projectMenuTreeNodes/);
    assert.match(roleGovernance, /historicalLabel: dictionary\.permissionTypeHistorical/);
    assert.match(roleGovernance, /missingReason: dictionary\.rolePermissionHistoricalMissing/);
  });

  it("projects a unique historical branch when a real page uses the base branch key", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "menu-history", name: "History Page", status: "enabled", nodeType: "page" },
          { code: "page-a", name: "Page A", status: "enabled", nodeType: "page" },
        ],
        ["menu-history", "missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const keys = nodes.map((node) => node.key);
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const missing = nodes.find((node) => node.key === "missing-a");
      assert.equal(new Set(keys).size, keys.length);
      assert.ok(historyBranch);
      assert.notEqual(historyBranch.key, "menu-history");
      assert.equal(missing?.parentKey, historyBranch.key);
      assert.deepEqual(pageMenuCodes(nodes, keys), ["menu-history", "missing-a", "page-a"]);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("projects a unique historical branch when a missing historical page uses the base branch key", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "page-a", name: "Page A", status: "enabled", nodeType: "page" }],
        ["menu-history"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const keys = nodes.map((node) => node.key);
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const missing = nodes.find((node) => node.key === "menu-history");
      assert.equal(new Set(keys).size, keys.length);
      assert.ok(historyBranch);
      assert.notEqual(historyBranch.key, "menu-history");
      assert.equal(missing?.parentKey, historyBranch.key);
      assert.notEqual(missing?.key, missing?.parentKey);
      assert.deepEqual(pageMenuCodes(nodes, [historyBranch.key, "menu-history"]), ["menu-history"]);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("avoids a dangling parent code when projecting the historical branch", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [{ code: "orphan-page", name: "Orphan Page", status: "enabled", nodeType: "page", parentCode: "menu-history" }],
        ["missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const orphan = nodes.find((node) => node.key === "orphan-page");
      assert.ok(historyBranch);
      assert.equal(orphan?.parentKey, "menu-history");
      assert.notEqual(historyBranch.key, "menu-history");
      assert.notEqual(orphan?.parentKey, historyBranch.key);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("advances the historical branch suffix past dangling parent candidates", () => {
    const result = runTypeScriptProbe("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};
      const nodes = projectMenuTreeNodes(
        [
          { code: "orphan-a", name: "Orphan A", status: "enabled", nodeType: "page", parentCode: "menu-history" },
          { code: "orphan-b", name: "Orphan B", status: "enabled", nodeType: "page", parentCode: "menu-history:1" },
        ],
        ["missing-a"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const historyBranch = nodes.find((node) => node.kind === "branch" && node.code === undefined);
      const catalogNodes = nodes.filter((node) => node.code === "orphan-a" || node.code === "orphan-b");
      assert.ok(historyBranch);
      assert.equal(historyBranch.key, "menu-history:2");
      assert.deepEqual(catalogNodes.map((node) => node.parentKey), ["menu-history", "menu-history:1"]);
      assert.ok(catalogNodes.every((node) => node.parentKey !== historyBranch.key));
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("derives directory full and half state from each pane eligible descendants", () => {
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(treeTransfer, /const selectedEligibleLeafKeySet = useMemo/);
    assert.match(treeTransfer, /deriveTreeTransferSelection\(index, normalizedValue, filteredKeys, selectableLeafKeySet\)/);
    assert.match(treeTransfer, /deriveTreeTransferSelection\(index, normalizedValue, filteredKeys, selectedEligibleLeafKeySet\)/);
    assert.match(treeTransfer, /index\.leafDescendantsByBranch\.get\(key\)/);
  });

  it("renders one state-specific role-menu read-only explanation", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const treeTransfer = adminSource("admin/src/platform/ui/PlatformTreeTransfer.tsx");

    assert.match(roleGovernance, /menuAccess\.readOnly \? <AdminFeedback type="warning" message=\{roleMenuReadOnlyTitle/);
    assert.match(roleGovernance, /description=\{readOnlyReason\}/);
    assert.match(roleGovernance, /showReadOnlyMessage=\{false\}/);
    assert.match(treeTransfer, /showReadOnlyMessage && readOnlyMessage/);
    assert.match(roleGovernance, /if \(reason === "disabled"\) return dictionary\.roleMenuReadonlyDisabledDescription;/);
    assert.match(roleGovernance, /if \(reason === "access"\) return dictionary\.roleMenuReadonlyAccessDescription;/);
    assert.match(roleGovernance, /if \(reason === "legacy"\) return dictionary\.roleMenuLegacyReadonlyDescription;/);
  });

  it("rejects tenant-scoped role-group creation without tenant read access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      '&& (groupWriteMode !== "target" || canReadTenants);',
      ';',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must require tenant read access/);
  });

  it("rejects role governance search that can apply a stale response", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "if (governanceRequest.current !== requestID) return;",
      "if (false) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must discard stale role and role-group search responses/);
  });

  it("rejects a stale debounced role-governance request that can restart loading", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const loadGovernance = useCallback(async (query = "", requestID = ++governanceRequest.current) => {\n    if (governanceRequest.current !== requestID) return;\n    setLoading(true);',
      'const loadGovernance = useCallback(async (query = "", requestID = ++governanceRequest.current) => {\n    setLoading(true);',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not re-enter the loading state/);
  });

  it("rejects filtered Tree Transfer behavior that drops hidden selections", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const preserved = value.filter((key) => !mutableVisibleSet.has(key));",
      "const preserved: string[] = [];",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must preserve selections outside the current result/);
  });

  it("rejects restoring the unused filtered Tree Transfer checked-key projection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const filteredKeys = useMemo(() => filteredNodeKeys(index, search), [index, search]);",
      "const filteredKeys = useMemo(() => filteredNodeKeys(index, search), [index, search]);\n  const visibleCheckedKeys = value.filter((key) => filteredKeys.has(key));",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not retain the unused visibleCheckedKeys projection/);
  });

  it("rejects tree workbench data without explicit selected and level semantics", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/ui/AdminTreeWorkbench.tsx", '"aria-selected": node.key === selectedKey,', '"aria-selected": false,');
    replaceInTemp(tempRoot, "admin/src/platform/ui/AdminTreeWorkbench.tsx", '"aria-level": depth,', '"aria-level": 1,');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /explicit aria-selected state/);
    assert.match(result.stderr, /explicit hierarchy depth/);
  });

  it("rejects role governance without a stable detail focus target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", 'className="role-governance-detail-focus-target" tabIndex={-1}', 'className="role-governance-detail-focus-target"');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /stable programmatic detail focus target/);
  });

  it("rejects role menu entry points that bypass the shared access resolver", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "resolveRoleMenuAccess(roleMenuTargetEnabled, canAssignMenus, record.status)",
      "resolveRoleMenuAccess(roleMenuTargetEnabled, canAssignMenus, \"enabled\")",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /entry points must use the shared access resolver/);
  });

  it("rejects a generic assign label for read-only role menu inspection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", "menuAccess.editable ? dictionary.assignMenus : dictionary.viewMenus", "dictionary.assignMenus");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /localized View Menus label/);
  });

  it("rejects role modal focus restoration without the detail fallback", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "afterClose={() => restoreRoleModalFocus(moveTriggerRef.current, detailFocusRef.current)}",
      "afterClose={() => restoreRoleModalFocus(moveTriggerRef.current, null)}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /restore focus to its connected trigger or the detail fallback/);
  });

  it("rejects Ant automatic trigger focus in explicitly restored role modals", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", "focusTriggerAfterClose={false}", "focusTriggerAfterClose");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /disable Ant automatic trigger focus/);
  });

  it("rejects untranslated role modal cancel actions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", "cancelText={dictionary.cancel}", "cancelText=\"Cancel\"");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must use the localized cancel label/);
  });

  it("rejects large role modals without a dynamic viewport height bound", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "max-height: calc(100dvh - 32px);", "max-height: none;");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must fit inside the dynamic viewport/);
  });

  it("rejects raw role status, role-group scope and role data-scope summaries", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", "roleStatusLabel(record.status, dictionary)", "record.status");
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", 'roleGroupScopeLabel(valueOf(record, "scopeType"), dictionary)', 'valueOf(record, "scopeType")');
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", 'roleDataScopeLabel(valueOf(record, "dataScope"), dictionary)', 'valueOf(record, "dataScope")');

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /status summaries must be localized/);
    assert.match(result.stderr, /scope summaries must be localized/);
  });

  it("rejects unbounded role workbench tracks and collapsing role detail", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "grid-template-columns: clamp(360px, 36vw, 520px) minmax(0, 1fr);", "grid-template-columns: 1fr 1fr;");
    replaceInTemp(tempRoot, "admin/src/styles.css", "min-height: 360px;", "min-height: 0;");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /wide enough for structured tree nodes/);
    assert.match(result.stderr, /stable minimum height/);
  });

  it("rejects tree workbenches that let long trees stretch the whole page", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "max-height: min(640px, calc(100vh - 280px));", "max-height: none;");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must scroll internally instead of stretching long trees/);
  });

  it("rejects tree labels that truncate structured node context", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "white-space: normal;", "white-space: nowrap;");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must wrap long names/);
  });

  it("rejects a mobile Tree Transfer toolbar without sticky two-column actions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "position: sticky;", "position: static;");
    replaceInTemp(tempRoot, "admin/src/styles.css", "grid-template-columns: repeat(2, minmax(0, 1fr));", "grid-template-columns: minmax(0, 1fr);");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Mobile Tree Transfer toolbar must stay visible/);
    assert.match(result.stderr, /bulk actions must stay in a two-column row/);
  });

  it("rejects role dialogs that bypass the shared AdminModal wrapper", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/resources/RoleGovernanceConsole.tsx", "<AdminModal", "<Modal");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not bypass the shared AdminModal wrapper/);
  });

  it("rejects product dialogs that bypass the shared AdminModal wrapper", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/platform/shell/AdminShell.tsx", "<AdminModal", "<Modal");
    replaceInTemp(tempRoot, "admin/src/platform/policy-review/PolicyReviewConsole.tsx", "<AdminModal", "<Modal");
    replaceInTemp(tempRoot, "admin/src/platform/resources/GenericResourceConsole.tsx", "<AdminModal", "<Modal");
    replaceInTemp(tempRoot, "admin/src/platform/resources/SensitiveFieldRevealModal.tsx", "<AdminModal", "<Modal");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Profile editor must not bypass the shared AdminModal wrapper/);
    assert.match(result.stderr, /Policy review must not bypass the shared AdminModal wrapper/);
    assert.match(result.stderr, /Generic resource dialogs must not bypass the shared AdminModal wrapper/);
    assert.match(result.stderr, /Sensitive reveal dialogs must not bypass the shared AdminModal wrapper/);
  });

  it("rejects shared AdminModal regressions without product sizing and scroll-safe layout", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/AdminPrimitives.tsx",
      'export type AdminModalSize = "sm" | "md" | "lg" | "xl";',
      'export type AdminModalSize = "md";',
    );
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".admin-modal .ant-modal-body {",
      ".admin-modal .ant-modal-body.missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminModal must expose sm\/md\/lg\/xl product sizes/);
    assert.match(result.stderr, /AdminModal body must own viewport-bounded scrolling/);
  });

  it("rejects role move options that cross scope or tenant boundaries", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "sameRoleGroupBoundary(group, moveSourceGroup)",
      "true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must stay inside the current scope and tenant boundary/);
  });

  it("rejects role mutation actions that remain enabled for disabled roles", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'const lifecycleDisabled = !roleLifecycleTargetEnabled || !canUpdateRole || record.status !== "enabled";',
      "const lifecycleDisabled = !roleLifecycleTargetEnabled || !canUpdateRole;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Disabled roles and non-target runtimes must not expose lifecycle mutations/);
  });

  it("rejects role detail actions without 44px touch targets", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".role-governance-detail .admin-list-actions .ant-btn,",
      ".role-governance-detail .admin-list-actions-missing .ant-btn,",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role detail actions must expose 44px targets/);
  });

  it("keeps role and organization workbench hierarchy localized and contract-bound", () => {
    const roleGovernance = adminSource("admin/src/platform/resources/RoleGovernanceConsole.tsx");
    const organizationExperience = adminSource("admin/src/platform/resources/organizationUserExperience.tsx");
    const styles = adminSource("admin/src/styles.css");

    assert.match(roleGovernance, /role-governance-action-groups/);
    assert.match(roleGovernance, /roleGovernanceAuthorizationActions/);
    assert.match(roleGovernance, /roleGovernanceLifecycleActions/);
    assert.match(organizationExperience, /organization-role-pool-metrics/);
    assert.match(styles, /\.organization-role-pool-panel\s*\{/);
  });

  it("rejects Tree Transfer checkboxes without a real 44px pointer target", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      ".platform-tree-transfer-pane .ant-tree-checkbox {",
      ".platform-tree-transfer-pane .ant-tree-checkbox-missing {",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Tree Transfer checkboxes must expose a real 44px by 44px pointer target/);
  });

  it("rejects Tree Transfer bulk operations that clear disabled selections", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const preservedDisabled = value.filter((key) => !mutableLeafKeySet.has(key));",
      "const preservedDisabled: string[] = [];",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /bulk operations must preserve disabled selections/);
  });

  it("rejects Tree Transfer bulk assignment that can add unavailable historical permissions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      'node.kind === "leaf" && !node.disabledReason && !node.availableDisabledReason',
      'node.kind === "leaf" && !node.disabledReason',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /bulk assignment must exclude unavailable historical selections/);
  });

  it("rejects Tree Transfer that prevents removing historical permissions from the selected pane", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformTreeTransfer.tsx",
      "const unavailable = Boolean(node.disabledReason || !selectedOnly && node.availableDisabledReason);",
      "const unavailable = Boolean(node.disabledReason || node.availableDisabledReason);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /historical selections must be disabled only in the available pane/);
  });

  it("rejects role authorization that drops disabled or missing historical permissions", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "permissionTreeNodes(permissionCatalog, dictionary, language, uniqueSorted([...authorization.allow, ...authorization.deny]))",
      "permissionTreeNodes(permissionCatalog, dictionary, language, [])",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must project disabled and missing historical permissions/);
  });

  it("rejects a second generic role mutation after atomic policy apply", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/rolePermissionWorkflow.ts",
      "await clients.replace(preview);",
      'await clients.replace(preview);\n  await clients.updateAdminResource("roles", authorization.role.id, {});',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not be followed by a second generic role mutation/);
  });

  it("rejects permission catalogs that reuse the menu migration gate", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/rolePermissionWorkflow.ts",
      'writeMode === "target-domain" ? sources.target(roleCode) : sources.generic()',
      'roleMenuMigrationWriteEnabled ? sources.target(roleCode) : sources.generic()',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Permission catalogs must select their source from the snapshotted role permission mode/);
  });

  it("rejects permission saves without readonly, update-permission and disabled-role guards", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      'if (!authorization || !canUpdateRole || authorization.role.status !== "enabled" || authorization.writeMode === "readonly") return;',
      "if (!authorization) return;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role permission saves must reject readonly, unauthorized and disabled-role writes/);
  });

  it("rejects legacy role permission writes that drop the existing public values snapshot", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/rolePermissionWorkflow.ts",
      "...authorization.role.values,",
      "",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Legacy role permission writes must preserve the complete public values snapshot/);
  });

  it("rejects read-only permission inspection without a close-only footer", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/RoleGovernanceConsole.tsx",
      "footer={readOnly ? <Button onClick={onCancel}>{dictionary.close}</Button> : undefined}",
      "footer={undefined}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Read-only role permission inspection must expose a close-only footer/);
  });

  it("rejects a permission write-mode resolver that omits a policy field", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/rolePermissionWriteMode.ts",
      '"dataScopeAreaCodes"] as const',
      '"areaCodes"] as const',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role permission write-mode resolver must inspect dataScopeAreaCodes/);
  });

  it("rejects a new user form that implicitly selects organization context", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      'record ? values : { ...values, tenantCode: "", orgUnitCode: undefined, roles: [] }',
      'record ? values : { ...values, tenantCode: "tenant-default", orgUnitCode: "org-default", roles: ["role-default"] }',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /New organization-scoped users must start without an implicitly selected organization/);
  });

  it("rejects form initialization that reruns when async options change", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "if (initializedFormKeyRef.current === initializationKey)",
      "if (false && initializedFormKeyRef.current === initializationKey)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Async relation and organization context updates must not reset an already initialized form/);
  });

  it("rejects form initialization that bypasses resource experience defaults", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "form.setFieldsValue(activeFormInitialValues)",
      "form.setFieldsValue(defaultFormValues(formFields))",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Form initialization must apply the active resource experience initial values/);
  });

  it("rejects initial value calculation that bypasses the resource experience", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "return experience.initialValues?.(values, editingRecord) ?? values;",
      "return values;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource forms must apply experience-owned initial values/);
  });

  it("rejects a form initialization guard that never records its lifecycle key", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "initializedFormKeyRef.current = initializationKey;",
      "void initializationKey;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Form initialization must record the active modal lifecycle/);
  });

  it("rejects a closed modal that retains its initialization key", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'initializedFormKeyRef.current = "";',
      "void initializedFormKeyRef.current;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Closing a form must clear its initialization key/);
  });

  it("rejects an editable tenant field for organization-scoped users", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      '<Input readOnly aria-readonly="true" placeholder={dictionary.userDerivedTenantPending} />',
      '<Input placeholder={dictionary.userDerivedTenantPending} />',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Derived user tenant must remain visibly and semantically read-only/);
  });

  it("rejects a role selector enabled before organization selection", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "disabled={!selectedOrgUnitCode || rolePoolLoading}",
      "disabled={rolePoolLoading}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /User roles must remain disabled until an organization is selected/);
  });

  it("rejects organization role-pool feedback without a polite live region", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      '<div aria-live="polite" id="organization-role-pool-status"',
      '<div aria-live="off" id="organization-role-pool-status"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Role-pool status changes must use a polite live region/);
  });

  it("rejects organization role-group bindings in generic CRUD input", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      'values: omitValue(context.input.values, "roleGroupCodes")',
      "values: context.input.values",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic organization CRUD must not carry role-group bindings/);
  });

  it("rejects organization context loading that stops after the first generic-resource page", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "records.length >= result.total",
      "currentPage >= 1",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /context pagination must stop only after the full result set is loaded/);
  });

  it("rejects a generic metadata write after organization role-group apply", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "await replaceOrganizationRoleGroups(preview);\n  return recordWithValues(context.editingRecord, { roleGroupCodes: selectedGroups.join(\",\") });",
      "await replaceOrganizationRoleGroups(preview);\n  return context.persist(input);",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Organization role-group apply must not be followed by a second generic metadata write/);
  });

  it("rejects static organization authorization confirmations outside the AntD application context", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/organizationUserExperience.tsx",
      "const { modal } = App.useApp();",
      "const modal = { confirm: () => undefined };",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /confirmations must use the active AntD application context/);
  });

  for (const [name, source, replacement, expected] of [
    ["delete actions", "allowDelete: !active", "allowDelete: true", /generic delete actions must stay disabled/],
    ["status toggles", "allowStatusToggle: !active", "allowStatusToggle: true", /generic status toggles must stay disabled/],
  ]) {
    it(`rejects organization and user ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(
        tempRoot,
        "admin/src/platform/resources/organizationUserExperience.tsx",
        source,
        replacement,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, expected);
    });
  }

  it("rejects optional personnel copy that is labeled as the organization capability", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/capabilities/metadata.ts",
      "人员与岗位",
      "人员组织",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Optional personnel capability must not be labeled as the organization capability/);
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

  it("rejects table column settings without AntD breakpoint state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "Grid.useBreakpoint()",
      "{}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must use AntD breakpoint state for rendered-column clarity/);
  });

  it("rejects table column settings without the current-width hidden state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      "labels.hiddenAtCurrentWidth",
      "labels.columnUnavailable",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must explain breakpoint-hidden selected columns/);
  });

  it("rejects a breakpoint-hidden column checkbox without the hidden state in its accessible name", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      /aria-label=\{(?:String\(column\.title\)|checkboxAccessibleLabel)\}/,
      "aria-label={columnLabel}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must include the breakpoint-hidden state in each hidden checkbox accessible name/);
  });

  it("rejects a truncated column label without its full plain-text title", () => {
    const tempRoot = tempAdminRoot();
    replaceRegexInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      /<span className="platform-column-option-label"(?: title=\{columnLabel\})?>/,
      '<span className="platform-column-option-label">',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Column settings must expose each full plain-text label through the option title attribute/);
  });

  it("rejects responsive column matching that requires every breakpoint", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/ui/PlatformDataTable.tsx",
      ".some((breakpoint) => screens[breakpoint])",
      ".every((breakpoint) => screens[breakpoint])",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PlatformDataTable must treat a column as rendered when any responsive breakpoint is active/);
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

  it("normalizes legacy and invalid watermark preferences", () => {
    const result = runTypeScriptProbe("admin/src/platform/ui/adminUIConfig.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      const { defaultAdminUIConfig, normalizeAdminUIConfig } = await import(${JSON.stringify(moduleURL)});
      const legacy = normalizeAdminUIConfig({ watermark: true });
      assert.equal(legacy.watermark, true);
      assert.equal(legacy.watermarkCount, 1);
      assert.deepEqual(legacy.watermarkScopes, ["screen"]);
      const invalid = normalizeAdminUIConfig({
        density: "comfortable",
        watermark: true,
        watermarkCount: 7,
        watermarkScopes: ["screen", "unknown", "export", "screen"],
      });
      assert.equal(invalid.density, "comfortable");
      assert.equal(invalid.watermarkCount, defaultAdminUIConfig.watermarkCount);
      assert.deepEqual(invalid.watermarkScopes, ["screen", "export"]);
      assert.deepEqual(normalizeAdminUIConfig({ watermark: true, watermarkScopes: [] }).watermarkScopes, []);
    `);

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });

  it("keeps accessible watermark scope controls and exact grid rendering", () => {
    const settings = fs.readFileSync(path.join(repoRoot, "admin/src/platform/ui/SystemSettingsDrawer.tsx"), "utf8");
    const shell = fs.readFileSync(path.join(repoRoot, "admin/src/platform/shell/AdminShell.tsx"), "utf8");
    const styles = fs.readFileSync(path.join(repoRoot, "admin/src/styles.css"), "utf8");

    assert.match(settings, /Checkbox\.Group/);
    assert.match(settings, /<Switch aria-label=\{label\}/);
    assert.match(settings, /className="settings-switch-hit-target"/);
    assert.match(settings, /role="group"/);
    assert.match(settings, /watermarkScopes/);
    assert.match(settings, /watermarkCount/);
    assert.match(settings, /options=\{watermarkCounts\}/);
    assert.match(shell, /className="platform-watermark-layer"/);
    assert.match(shell, /aria-hidden="true"/);
    assert.match(shell, /Array\.from\(\{ length: uiConfig\.watermarkCount \}/);
    assert.match(styles, /\.platform-watermark-layer/);
    assert.match(styles, /grid-template-columns/);
    assert.match(styles, /min-height:\s*44px/);
    assert.match(styles, /\.settings-switch-hit-target\s*\{[\s\S]*?min-height:\s*44px[\s\S]*?min-width:\s*44px/);
  });

  it("rejects a screen watermark nested under the scrollable content region", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "      {showScreenWatermark ? (",
      '      <section className="platform-content">\n        {showScreenWatermark ? (',
    );
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      '      ) : null}\n      <aside className="platform-sider"',
      '        ) : null}\n      </section>\n      <aside className="platform-sider"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Screen watermark must be mounted directly under \.platform-shell/);
    assert.match(result.stderr, /Screen watermark must not be nested under \.platform-content/);
  });

  for (const [name, declaration, replacement] of [
    ["fixed viewport positioning", "position: fixed;", "position: absolute;"],
    ["full viewport inset", "inset: 0;", "inset: auto;"],
    ["overlay stacking", "z-index: 2200;", "z-index: 1100;"],
    ["non-interactive pointer handling", "pointer-events: none;", "pointer-events: auto;"],
  ]) {
    it(`rejects a screen watermark without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceRegexInTemp(
        tempRoot,
        "admin/src/styles.css",
        new RegExp(`(\\.platform-watermark-layer\\s*\\{[\\s\\S]*?)${declaration.replace(/[.*+?^${}()|[\\]\\\\]/g, "\\\\$&")}`),
        `$1${replacement}`,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /Screen watermark must remain a fixed, full-viewport, non-interactive overlay above the Admin shell/);
    });
  }

  it("rejects a multi-watermark layout that leaves the topbar visually blank", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      '.platform-watermark-layer[data-count="16"] span:nth-child(-n + 4)',
      '.platform-watermark-layer[data-count="16"] span:nth-child(-n + 0)',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /first row against the viewport edge/);
  });

  it("rejects a sixteen-watermark layout that stays four columns on narrow viewports", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/styles.css",
      'grid-template-columns: repeat(2, minmax(0, 1fr));\n    grid-template-rows: repeat(8, minmax(0, 1fr));',
      'grid-template-columns: repeat(4, minmax(0, 1fr));\n    grid-template-rows: repeat(4, minmax(0, 1fr));',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reflow sixteen watermarks to two columns/);
  });

  it("rejects reveal callbacks that can mount the login OIDC consumer", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/App.tsx", "if (sensitiveRevealOIDCCallbackPending) {", "if (false) {");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reveal callbacks must remain isolated from the login OIDC callback consumer/);
  });

  it("rejects sensitive reveal actions outside manifest-governed detail fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "field.inDetail && field.reveal && permissionAllows(permissions, field.reveal.permission, deniedPermissions)",
      "field.reveal && permissionAllows(permissions, field.reveal.permission, deniedPermissions)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Sensitive reveal actions must require detail visibility, manifest declaration, and the declared permission/);
  });

  it("rejects reveal responses that can restore plaintext after close or page hide", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/SensitiveFieldRevealModal.tsx",
      "if (operationGenerationRef.current !== generation) return;",
      "void generation;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Reveal responses must be discarded after close, hide, or target changes/);
  });

  it("rejects plaintext copy that bypasses field or policy configuration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/SensitiveFieldRevealModal.tsx",
      "result.copyAllowed && policy?.copyAllowed && field.reveal?.copyAllowed",
      "result.copyAllowed",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Sensitive plaintext copy must require the response, policy, and field contract to allow it/);
  });

  it("rejects OIDC resume without off-page detail hydration", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "provider.getOne<AdminResourceRecord>",
      "provider.getMany<AdminResourceRecord>",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /OIDC resume must hydrate a detail record that is outside the current list page/);
  });

  it("passes the shared export watermark intent to policy review JSON exports", () => {
    const client = fs.readFileSync(path.join(repoRoot, "admin/src/platform/api/client.ts"), "utf8");
    const app = fs.readFileSync(path.join(repoRoot, "admin/src/App.tsx"), "utf8");
    const policyReview = fs.readFileSync(path.join(repoRoot, "admin/src/platform/policy-review/PolicyReviewConsole.tsx"), "utf8");

    assert.match(client, /export function exportAdminPolicyReviews\(\{ watermark \}: \{ watermark: boolean \}\)/);
    assert.match(client, /policy-reviews\/export\?watermark=\$\{watermark\}/);
    assert.match(app, /uiConfig\.watermark && uiConfig\.watermarkScopes\.includes\("export"\)/);
    assert.match(policyReview, /exportAdminPolicyReviews\(\{ watermark: exportWatermark \}\)/);
  });

  it("rejects a policy review export control that ignores the dedicated permission", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/policy-review/PolicyReviewConsole.tsx",
      'permissionAllows(permissions, "admin:policy-review:export", deniedPermissions)',
      'permissionAllows(permissions, "admin:policy-review:read", deniedPermissions)',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PolicyReviewConsole must derive export access from the dedicated permission/);
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

  it("rejects mobile work-tab close handling that leaves the context panel open", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "const handleMobileWorkTabClose = (route: string) => {\n    setOpenContext(null);\n    closeWorkTab(route);\n  };",
      "const handleMobileWorkTabClose = (route: string) => {\n    closeWorkTab(route);\n  };",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Mobile work-tab close must dismiss its context panel before closing the tab/);
  });

  it("rejects mobile work-tab close controls that bypass the context-closing handler", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "onClick={() => handleMobileWorkTabClose(resource.route)}",
      "onClick={() => closeWorkTab(resource.route)}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Mobile work-tab close controls must use the context-closing handler/);
  });

  it("keeps one shell collapse control and gives mixed layout distinct navigation roles", () => {
    const shell = adminSource("admin/src/platform/shell/AdminShell.tsx");

    assert.equal((shell.match(/className="brand-collapse-button"/g) ?? []).length, 1);
    assert.doesNotMatch(shell, /className="desktop-sider-toggle"/);
    assert.match(shell, /groupedResources=\{layoutMode === "mixed" && activeGroup \? \[activeGroup\] : groupedResources\}/);
    assert.match(shell, /compact=\{layoutMode === "mixed"\}/);
    assert.match(shell, /aria-current=\{active \? "page" : undefined\}/);
  });

  it("keeps work tabs visible with overflow controls, wheel scrolling, and active-tab reveal", () => {
    const shell = adminSource("admin/src/platform/shell/AdminShell.tsx");
    const styles = adminSource("admin/src/styles.css");
    const dictionary = adminSource("admin/src/platform/i18n.ts");

    assert.match(shell, /className="platform-work-tabs-shell"/);
    assert.match(shell, /ResizeObserver/);
    assert.match(shell, /MutationObserver/);
    assert.match(shell, /onWheel=\{handleWorkTabsWheel\}/);
    assert.match(shell, /event\.currentTarget\.scrollBy/);
    assert.match(shell, /scrollIntoView\(\{ block: "nearest", inline: "nearest" \}\)/);
    assert.match(shell, /className="work-tabs-scroll-button"/);
    assert.match(shell, /disabled=\{!workTabsScroll\.left\}/);
    assert.match(shell, /disabled=\{!workTabsScroll\.right\}/);
    assert.match(styles, /\.platform-work-tabs-shell\s*\{/);
    assert.match(styles, /\.work-tab\s*\{[\s\S]*?flex:\s*0 0 auto;/);
    assert.match(styles, /\.platform-work-tabs\s*\{[\s\S]*?overflow-x:\s*auto;/);
    assert.match(dictionary, /scrollWorkTabsLeft: /);
    assert.match(dictionary, /scrollWorkTabsRight: /);
  });

  it("keeps the resource page heading semantic while removing its duplicate list title", () => {
    const page = adminSource("admin/src/platform/ui/AdminPage.tsx");
    const resource = adminSource("admin/src/platform/resources/GenericResourceConsole.tsx");
    const styles = adminSource("admin/src/styles.css");

    assert.match(page, /<ResourcePageHeader className="page-heading"/);
    assert.match(adminSource("admin/src/platform/ui/ResourcePageHeader.tsx"), /<Typography\.Title level=\{1\}>\{title\}<\/Typography\.Title>/);
    assert.match(resource, /<AdminPage\s+[\s\S]*title=\{resource\.title\[language\]\}/);
    assert.match(styles, /\.generic-resource-console \.platform-data-table-panel \.admin-list-title\s*\{[\s\S]*?display:\s*none;/);
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
      "admin/src/platform/api/sessionExpiry.ts",
      "currentToken === requestToken",
      "Boolean(currentToken)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Session expiry must clear only the exact token used by the failed request/);
  });

  it("keeps the Admin session after sensitive reveal verification fails", () => {
    const result = runTypeScriptProbe("admin/src/platform/api/sessionExpiry.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { shouldExpireAdminSession } from ${JSON.stringify(moduleURL)};

      assert.equal(shouldExpireAdminSession({
        statusCode: 422,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED",
      }), false);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED",
      }), false);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "current-token",
        currentToken: "current-token",
        errorCode: "AUTH_UNAUTHORIZED",
      }), true);
      assert.equal(shouldExpireAdminSession({
        statusCode: 401,
        requestToken: "stale-token",
        currentToken: "current-token",
        errorCode: "AUTH_UNAUTHORIZED",
      }), false);
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("keeps Admin session expiry codes typed by the generated registry", () => {
    assert.match(
      adminSource("admin/src/platform/api/sessionExpiry.ts"),
      /errorCode\?: PlatformErrorCode/,
    );
  });

  it("normalizes malformed and unknown Admin errors with header correlation", () => {
    const result = runAdminClientProbe((moduleURL) => `
      import assert from "node:assert/strict";
      import { AdminAPIError, parsePlatformResponse } from ${JSON.stringify(moduleURL)};

      const requestId = "req_0123456789abcdef0123456789abcdef";
      const traceId = "0123456789abcdef0123456789abcdef";
      const headers = {
        "X-Request-ID": requestId,
        traceparent: "00-" + traceId + "-0123456789abcdef-01",
      };
      for (const body of ["not-json", JSON.stringify({ error: { code: "UNKNOWN_ERROR", message: "upstream failure" } })]) {
        await assert.rejects(
          parsePlatformResponse(new Response(body, { status: 500, headers }), ""),
          (error) => error instanceof AdminAPIError
            && error.code === "INTERNAL_ERROR"
            && error.requestId === requestId
            && error.traceId === traceId,
        );
      }
      for (const [traceparent, expectedTraceId] of [
        ["00-" + traceId + "-0123456789abcdef-01", traceId],
        ["00-00000000000000000000000000000000-0123456789abcdef-01", ""],
        ["00-" + traceId + "-0000000000000000-01", ""],
        ["invalid", ""],
      ]) {
        await assert.rejects(
          parsePlatformResponse(new Response("not-json", {
            status: 500,
            headers: { "X-Request-ID": "caller-owned", traceparent },
          }), ""),
          (error) => error instanceof AdminAPIError
            && error.code === "INTERNAL_ERROR"
            && error.requestId === ""
            && error.traceId === expectedTraceId,
        );
      }
      await assert.rejects(
        parsePlatformResponse(new Response(JSON.stringify({
          error: { code: "ADMIN_FORBIDDEN", message: "permission denied", requestId, traceId },
        }), { status: 403 }), ""),
        (error) => error instanceof AdminAPIError
          && error.code === "ADMIN_FORBIDDEN"
          && error.requestId === requestId
          && error.traceId === traceId,
      );
    `);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects session expiry that mounts login before reveal callback URL cleanup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/App.tsx",
      "      setSensitiveRevealOIDCResume(null);\n      clearPendingSensitiveRevealOIDC();",
      "      setSensitiveRevealOIDCResume(null);\n      setSensitiveRevealOIDCCallbackPending(false);\n      clearPendingSensitiveRevealOIDC();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reveal callback cleanup remove code and state/);
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

  for (const [name, relativePath, from, to, message] of [
    ["provider audiences", "admin/src/platform/api/client.ts", "audiences: string[];", "audienceList: string[];", "AuthProvider must expose its declared audiences"],
    ["configured provider filtering", "admin/src/platform/auth/AdminLoginView.tsx", "adminProviders.filter((provider) => provider.enabled && provider.configured)", "adminProviders", "Admin login must render only enabled and configured providers"],
    ["provider tabs", "admin/src/platform/auth/AdminLoginView.tsx", 'className="login-provider-tabs"', 'className="login-provider-list"', "Admin login providers must render as tabs instead of disabled option cards"],
    ["Web Crypto verifier", "admin/src/platform/refine/authProvider.ts", "crypto.getRandomValues(new Uint8Array(size))", "new Uint8Array(size)", "generate verifier bytes with Web Crypto"],
    ["S256 challenge", "admin/src/platform/refine/authProvider.ts", 'crypto.subtle.digest("SHA-256"', 'legacyDigest("SHA-256"', "derive an S256 challenge with Web Crypto"],
    ["tab-scoped transaction", "admin/src/platform/refine/authProvider.ts", "window.sessionStorage.setItem", "window.localStorage.setItem", "tab-scoped sessionStorage"],
    ["exact callback state", "admin/src/platform/auth/oidcPolicy.ts", "callbackState !== pending.state", "!callbackState", "exact comparison"],
    ["callback URL cleanup", "admin/src/platform/refine/authProvider.ts", "window.history.replaceState", "window.history.pushState", "remove callback values from browser history before exchange"],
    ["demo-only form", "admin/src/platform/auth/AdminLoginView.tsx", 'selectedProvider.kind === "demo"', 'selectedProvider.kind !== "unknown"', "username form must render only for the demo provider"],
    ["OIDC-only action", "admin/src/platform/auth/AdminLoginView.tsx", 'selectedProvider.kind === "oidc"', 'selectedProvider.kind !== "unknown"', "OIDC action must render only for an OIDC provider"],
    ["OIDC action width hook", "admin/src/platform/auth/AdminLoginView.tsx", 'className="login-oidc-action"', 'className="login-provider-action"', "one full-width login action"],
    ["callback live region", "admin/src/platform/auth/AdminLoginView.tsx", 'aria-live="polite"', 'aria-live="off"', "polite live region"],
    ["error focus", "admin/src/platform/auth/AdminLoginView.tsx", "focus({ preventScroll: true })", "focus()", "focus its heading without a scroll jump"],
    ["recovery action", "admin/src/platform/auth/AdminLoginView.tsx", 'className="login-recovery-action"', 'className="login-reset-action"', "explicit recovery action"],
    ["login reduced motion", "admin/src/styles.css", ".login-page *", ".login-motion-uncovered *", "suppress non-essential login transitions"],
    ["mobile login target", "admin/src/styles.css", ".login-submit,\n  .login-oidc-action,\n  .login-recovery-action", ".login-actions-missing", "Mobile login submit, OIDC, and recovery actions must expose 44px touch targets"],
    ["tablet login target", "admin/src/styles.css", "@media (max-width: 1024px)", "@media (max-width: 1023px)", "Tablet login submit, OIDC, and recovery actions must expose 44px touch targets"],
  ]) {
    it(`rejects Task 6 without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTempIfPresent(tempRoot, relativePath, from, to);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }

  it("executes the Admin audience and authorization URL policy", () => {
    const result = runTypeScriptProbe(
      "admin/src/platform/auth/oidcPolicy.ts",
      (moduleURL) => `
        import assert from "node:assert/strict";
        import {
          assertAdminAuthProvider,
          beginOIDCLoginTransaction,
          consumePendingOIDCLoginTransaction,
          createSingleUseGuard,
          createSubmissionLock,
          filterAdminAuthProviders,
          OIDCCallbackError,
          validateOIDCAuthorizationURL,
        } from ${JSON.stringify(moduleURL)};

        const adminProvider = { id: "oidc", audiences: ["admin"] };
        const appProvider = { id: "wechat", audiences: ["app"] };
        assert.deepEqual(filterAdminAuthProviders([appProvider, adminProvider]), [adminProvider]);
        assert.doesNotThrow(() => assertAdminAuthProvider(adminProvider));
        assert.throws(() => assertAdminAuthProvider(appProvider), /not available for Admin login/);

        for (const url of ["https://id.example/authorize"]) {
          assert.equal(validateOIDCAuthorizationURL(url), new URL(url).toString());
        }

        for (const url of [
          "http://localhost:8080/authorize",
          "http://auth.localhost:8080/authorize",
          "http://127.0.0.1:8080/authorize",
          "http://127.25.3.9:8080/authorize",
          "http://[::1]:8080/authorize",
        ]) {
          assert.throws(() => validateOIDCAuthorizationURL(url), /OIDC authorization URL is not trusted/);
          assert.equal(validateOIDCAuthorizationURL(url, { allowLoopbackHTTP: true }), new URL(url).toString());
        }

        for (const url of [
          "not a URL",
          "javascript:alert(1)",
          "data:text/html,unsafe",
          "http://id.example/authorize",
          "ftp://127.0.0.1/authorize",
        ]) {
          assert.throws(
            () => validateOIDCAuthorizationURL(url),
            (error) => error instanceof Error && error.message === "OIDC authorization URL is not trusted" && !error.message.includes(url),
          );
        }

        const submissionLock = createSubmissionLock();
        assert.equal(submissionLock.acquire(), true);
        assert.equal(submissionLock.acquire(), false);
        submissionLock.release();
        assert.equal(submissionLock.acquire(), true);

        const onceGuard = createSingleUseGuard();
        assert.equal(onceGuard.acquire(), true);
        assert.equal(onceGuard.acquire(), false);

        const beginEvents = [];
        let storedTransaction = "";
        await beginOIDCLoginTransaction(adminProvider, {
          allowLoopbackHTTP: false,
          randomBytes: (size) => { beginEvents.push("random"); return new Uint8Array(size).fill(1); },
          digestSHA256: async (input) => { beginEvents.push("digest"); assert.equal(input.length, 43); return new Uint8Array(32).fill(2); },
          startProvider: async (provider, challenge) => {
            beginEvents.push("start");
            assert.equal(provider, "oidc");
            assert.match(challenge, /^[A-Za-z0-9_-]{43}$/);
            return { authorizationUrl: "https://id.example/authorize", state: "state-exact", expiresAt: "2030-01-01T00:00:00.000Z" };
          },
          storePending: (value) => { beginEvents.push("store"); storedTransaction = value; },
          navigate: (url) => { beginEvents.push("navigate"); assert.equal(url, "https://id.example/authorize"); },
        });
        assert.deepEqual(beginEvents, ["random", "digest", "start", "store", "navigate"]);
        const stored = JSON.parse(storedTransaction);
        assert.deepEqual(Object.keys(stored).sort(), ["codeVerifier", "expiresAt", "provider", "state"]);
        assert.equal(stored.provider, "oidc");
        assert.equal(stored.state, "state-exact");

        const rejectedBeginEvents = [];
        await assert.rejects(
          beginOIDCLoginTransaction(adminProvider, {
            allowLoopbackHTTP: false,
            randomBytes: (size) => new Uint8Array(size),
            digestSHA256: async () => new Uint8Array(32),
            startProvider: async () => ({ authorizationUrl: "http://127.0.0.1:8080/authorize", state: "state", expiresAt: "2030-01-01T00:00:00.000Z" }),
            storePending: () => rejectedBeginEvents.push("store"),
            navigate: () => rejectedBeginEvents.push("navigate"),
          }),
          /OIDC authorization URL is not trusted/,
        );
        assert.deepEqual(rejectedBeginEvents, []);

        const pending = JSON.stringify({ provider: "oidc", state: "state-exact", codeVerifier: "verifier", expiresAt: "2030-01-01T00:00:00.000Z" });
        const makeConsumeDependencies = ({ raw = pending, now = Date.parse("2029-01-01T00:00:00.000Z"), exchangeError = null } = {}) => {
          const events = [];
          const exchanges = [];
          return {
            events,
            exchanges,
            dependencies: {
              cleanupURL: () => events.push("cleanup"),
              readPending: () => { events.push("read"); return raw; },
              removePending: () => events.push("remove"),
              now: () => now,
              exchange: async (input) => {
                events.push("exchange");
                exchanges.push(input);
                if (exchangeError) throw exchangeError;
                return { principal: { user: { id: "admin" } } };
              },
            },
          };
        };

        const noCallback = makeConsumeDependencies();
        assert.equal(await consumePendingOIDCLoginTransaction("", noCallback.dependencies), null);
        assert.deepEqual(noCallback.events, []);

        for (const scenario of [
          { name: "empty code and state", search: "?code=&state=" },
          { name: "empty provider error", search: "?error=" },
          { name: "empty provider error with code and state", search: "?code=x&state=y&error=" },
        ]) {
          const current = makeConsumeDependencies();
          await assert.rejects(
            consumePendingOIDCLoginTransaction(scenario.search, current.dependencies),
            (error) => error instanceof OIDCCallbackError && error.reason === "callback" && error.message === "callback",
            scenario.name,
          );
          assert.deepEqual(current.events, ["cleanup", "read", "remove"], scenario.name);
          assert.deepEqual(current.exchanges, [], scenario.name);
        }

        for (const scenario of [
          { name: "missing code", search: "?state=state-exact" },
          { name: "malformed transaction", search: "?code=code&state=state-exact", raw: "not-json" },
          { name: "state mismatch", search: "?code=code&state=wrong" },
          { name: "expired", search: "?code=code&state=state-exact", now: Date.parse("2031-01-01T00:00:00.000Z") },
        ]) {
          const current = makeConsumeDependencies(scenario);
          await assert.rejects(consumePendingOIDCLoginTransaction(scenario.search, current.dependencies));
          assert.deepEqual(current.events.slice(0, 3), ["cleanup", "read", "remove"], scenario.name);
          assert.equal(current.events.includes("exchange"), false, scenario.name);
        }

        const success = makeConsumeDependencies();
        const successResult = await consumePendingOIDCLoginTransaction("?code=code-exact&state=state-exact", success.dependencies);
        assert.deepEqual(success.events, ["cleanup", "read", "remove", "exchange"]);
        assert.deepEqual(success.exchanges, [{ provider: "oidc", code: "code-exact", state: "state-exact", codeVerifier: "verifier" }]);
        assert.equal(successResult.principal.user.id, "admin");

        const exchangeFailure = makeConsumeDependencies({ exchangeError: new Error("normalized exchange failure") });
        await assert.rejects(consumePendingOIDCLoginTransaction("?code=code&state=state-exact", exchangeFailure.dependencies), /normalized exchange failure/);
        assert.deepEqual(exchangeFailure.events, ["cleanup", "read", "remove", "exchange"]);
      `,
    );

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });

  for (const [name, relativePath, from, to, message] of [
    ["Admin audience filtering", "admin/src/platform/auth/AdminLoginView.tsx", "filterAdminAuthProviders(providers)", "providers", "Admin login must consume provider audiences before selection and rendering"],
    ["configured-provider-only login selector", "admin/src/platform/auth/AdminLoginView.tsx", "adminProviders.filter((provider) => provider.enabled && provider.configured)", "adminProviders", "Admin login must render only enabled and configured providers"],
    ["Admin audience start guard", "admin/src/platform/auth/oidcPolicy.ts", "assertAdminAuthProvider(provider);", "void provider;", "OIDC start must reject providers without the Admin audience"],
    ["demo synchronous lock", "admin/src/platform/auth/AdminLoginView.tsx", "if (!submissionLockRef.current.acquire()) return;", "if (submitting) return;", "Demo login must acquire the synchronous submission lock"],
    ["OIDC synchronous lock", "admin/src/platform/auth/AdminLoginView.tsx", "if (!submissionLockRef.current.acquire()) return;", "if (submitting) return;", "OIDC start must acquire the synchronous submission lock"],
    ["authorization URL validation", "admin/src/platform/auth/oidcPolicy.ts", "validateOIDCAuthorizationURL(started.authorizationUrl,", "String(started.authorizationUrl,", "OIDC start must validate the authorization URL before browser navigation"],
    ["recovery focus", "admin/src/platform/auth/AdminLoginView.tsx", "loginHeadingRef.current?.focus({ preventScroll: true })", "void loginHeadingRef.current", "Explicit OIDC recovery must restore focus predictably without scrolling"],
    ["localized callback category", "admin/src/platform/auth/AdminLoginView.tsx", "setCallbackFailure(callbackFailureReason(nextError))", "setLoginError(callbackErrorMessage(dictionary, nextError))", "OIDC callback failures must store a stable error category"],
  ]) {
    it(`rejects Task 6 without ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTempIfPresent(tempRoot, relativePath, from, to);

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(message));
    });
  }

  it("rejects production wrappers that enable loopback HTTP outside verified development", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/refine/authProvider.ts",
      "allowLoopbackHTTP: import.meta.env.DEV",
      "allowLoopbackHTTP: true",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Loopback HTTP authorization URLs must be enabled only in verified development mode/);
  });

  it("rejects callback cleanup after pending transaction access", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/oidcPolicy.ts",
      "dependencies.cleanupURL();\n  const rawPending = dependencies.readPending();",
      "const rawPending = dependencies.readPending();\n  dependencies.cleanupURL();",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /OIDC callbacks must remove callback values before reading pending transaction state/);
  });

  it("rejects login views that remove credential password rendering", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/AdminLoginView.tsx",
      'credentialSpec?.mode === "password"',
      'credentialSpec?.mode === "never"',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /password form must render only for credential password providers/);
  });

  it("rejects login views that expose unconfigured providers as selector options", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/AdminLoginView.tsx",
      'className="login-provider-tabs"',
      'className="login-provider-tabs" data-regression={dictionary.notConfigured}',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin login must not show unconfigured providers in the login selector/);
  });

  it("rejects credential login clients that serialize raw form input", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "const body = await withEncryptedCredentialSecret(input);",
      "const body = input;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Credential login must encrypt secrets before request serialization/);
  });

  it("rejects password mutation clients that drop encrypted transport request contracts", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "satisfies AdminCurrentPasswordChangeRequest",
      "as unknown",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Current-user password changes must serialize encrypted secret envelopes/);
  });

  it("rejects admin password reset clients that drop encrypted transport request contracts", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "satisfies AdminProfilePasswordResetRequest",
      "as unknown",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin password resets must serialize encrypted secret envelopes/);
  });

  it("rejects unauthenticated route normalization during OIDC callback cleanup", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/App.tsx",
      "if (!getAuthToken() || !session || loading)",
      "if (loading)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Unauthenticated callback routes must not be normalized before OIDC URL cleanup/);
  });

  it("rejects unmatched Admin OIDC translations", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/i18n.ts",
      'loginOIDCRecovery: "Return to login providers",',
      "",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin login i18n key loginOIDCRecovery must exist in matching Chinese and English dictionaries/);
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
    assert.match(result.stderr, /Reduced motion must cover used body-portaled AntD motion roots/);
  });

  for (const [name, selector] of [
    ["tooltips", ".ant-tooltip"],
    ["select dropdowns", ".ant-select-dropdown"],
  ]) {
    it(`rejects reduced-motion styles that omit body-portaled AntD ${name}`, () => {
      const tempRoot = tempAdminRoot();
      replaceInTemp(
        tempRoot,
        "admin/src/styles.css",
        selector,
        `${selector}-motion-uncovered`,
      );

      const result = runValidator(["--root", tempRoot]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /Reduced motion must cover used body-portaled AntD motion roots/);
    });
  }

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

  it("rejects encrypted resource filters that are not constrained to exact match", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "operator: isEncryptedExactMatchField(field)",
      "operator: field.type === \"text\"",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted resource filters must submit exact-match conditions/);
  });

  it("rejects query parsing that accepts non-equality operators for encrypted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'isEncryptedExactMatchField(field) && match[2] !== "="',
      "false",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted resource query syntax must allow equality only/);
  });

  it("rejects encrypted edit hydration that reuses projected or default values", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'if (field.storageMode === "encrypted") {\n    return undefined;\n  }',
      'if (field.storageMode === "encrypted") {\n    return defaultFieldValue(field);\n  }',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must hydrate blank/);
  });

  it("rejects encrypted edit fields that require resubmitting the current secret", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'field.required && !(editing && field.storageMode === "encrypted")',
      "field.required",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must allow a blank value/);
  });

  it("rejects encrypted edit fields without localized preserve-value guidance", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "parts.push(dictionary.encryptedFieldEditHint);",
      "void dictionary.encryptedFieldEditHint;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit fields must expose the localized blank-preserves-current-value hint/);
  });

  it("rejects an incomplete encrypted edit hint dictionary", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/i18n.ts",
      'encryptedFieldEditHint: "This field is encrypted. Leave it blank while editing to keep the current value.",',
      'encryptedEditHint: "This field is encrypted. Leave it blank while editing to keep the current value.",',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Encrypted edit field guidance must exist in matching Chinese and English dictionaries/);
  });

  it("rejects status updates that can resubmit encrypted, hidden or non-writable values", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      '.filter((field) => field.source === "values" && !field.readOnly && field.sensitivity === "public" && field.storageMode !== "encrypted" && field.responseMode !== "omitted" && field.responseMode !== "privileged")',
      '.filter((field) => field.source === "values" && field.storageMode !== "encrypted")',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Status updates must exclude encrypted, hidden and non-writable values/);
  });

  it("rejects status updates that bypass schema-aware filtering", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "inputFromRecord(record, schema.fields, { status: nextStatus })",
      "inputFromRecord(record, [], { status: nextStatus })",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Status updates must use schema-aware record input filtering/);
  });

  it("rejects generic resource tables that render omitted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'schema.fields.filter((field) => field.inTable && field.responseMode !== "omitted")',
      "schema.fields.filter((field) => field.inTable)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource tables must not render omitted response fields/);
  });

  it("rejects generic resource details that render omitted fields", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      'schema.fields.filter((field) => field.inDetail && field.responseMode !== "omitted")',
      "schema.fields.filter((field) => field.inDetail)",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Generic resource details must not render omitted response fields/);
  });

  it("rejects an incomplete TypeScript masking contract", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      "masking?: AdminResourceFieldMasking;",
      "masking?: unknown;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminResourceField must carry optional masking metadata/);
  });

  it("rejects TypeScript masking metadata that drops a supported strategy", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/api/client.ts",
      'strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1" | "address-cn-v1";',
      'strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1";',
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin field masking metadata must expose every supported versioned strategy/);
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

  it("rejects global search that hides the no-results state", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "open={Boolean(globalSearchQuery.trim())}",
      "open={Boolean(globalSearchQuery.trim()) && globalSearchResults.length > 0}",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /no-results feedback/);
  });

  it("rejects a desktop sidebar that grows past the viewport instead of scrolling navigation", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(tempRoot, "admin/src/styles.css", "height: 100dvh;\n  min-height: 0;", "min-height: 100vh;");

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /expanded navigation can scroll/);
  });

  it("rejects removing the legacy operations directory localization", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/shell/AdminShell.tsx",
      "operations: dictionary.operations,",
      "operations: \"Operations\",",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /localize the legacy operations navigation directory/);
  });

  it("rejects relation option search without the platform debounce interval", () => {
    const tempRoot = tempAdminRoot();
    replaceInTemp(
      tempRoot,
      "admin/src/platform/resources/GenericResourceConsole.tsx",
      "const RELATION_OPTION_SEARCH_DELAY_MS = 250;",
      "const RELATION_OPTION_SEARCH_DELAY_MS = 0;",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /250ms debounce interval/);
  });

  for (const [name, selector, message] of [
    ["profile trigger", ".platform-topbar .profile-menu-trigger {", "Mobile profile trigger must expose a 44px touch target"],
    ["settings trigger", ".platform-topbar .settings-trigger-button {", "Mobile settings trigger must expose a 44px touch target"],
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
