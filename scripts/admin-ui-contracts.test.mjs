import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";

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

describe("validate-admin-ui-contracts", () => {
  it("accepts the current componentized admin UI contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Admin UI contract validation passed/);
  });

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

  for (const [name, relativePath, from, to, message] of [
    ["provider audiences", "admin/src/platform/api/client.ts", "audiences: string[];", "audienceList: string[];", "AuthProvider must expose its declared audiences"],
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

  it("rejects OIDC rendering that restores the disabled password field", () => {
    const tempRoot = tempAdminRoot();
    replaceInTempIfPresent(
      tempRoot,
      "admin/src/platform/auth/AdminLoginView.tsx",
      "</Form>",
      "<Input.Password disabled /></Form>",
    );

    const result = runValidator(["--root", tempRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /must not retain the disabled password field/);
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
