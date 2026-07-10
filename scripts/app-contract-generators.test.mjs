import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

function runScript(script, args = ["--stdout"]) {
  const result = spawnSync(process.execPath, [`scripts/${script}`, ...args], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
  });
  assert.equal(result.status, 0, `${script} failed\n${result.stdout}${result.stderr}`);
  return JSON.parse(result.stdout);
}

describe("app contract generators", () => {
  it("generates app OpenAPI from the manifest-derived app route contract", () => {
    const openapi = runScript("generate-app-openapi.mjs");

    assert.equal(openapi.openapi, "3.1.0");
    assert.equal(openapi.info.title, "Platform App API");
    assert.equal(openapi["x-source"], "resources/generated/app-route-contract.json");
    assert.equal(openapi["x-security-domain"], "app");
    assert.deepEqual(openapi.components.securitySchemes.appBearerAuth, {
      type: "http",
      scheme: "bearer",
      bearerFormat: "JWT",
      description: "App bearer token with tokenType=app.",
    });

    const login = openapi.paths["/api/app/auth/login"].post;
    assert.deepEqual(login.security, []);
    assert.equal(login.operationId, "appAuthLogin");
    assert.equal(login["x-platform-capability"], "session");

    const current = openapi.paths["/api/app/session/current"].get;
    assert.deepEqual(current.security, [{ appBearerAuth: [] }]);
    assert.equal(current.operationId, "appSessionCurrent");

    const appFileUpload = openapi.paths["/api/app/files"].post;
    assert.deepEqual(appFileUpload.security, [{ appBearerAuth: [] }]);
    assert.equal(appFileUpload.operationId, "appFilesCreate");
    assert.equal(appFileUpload["x-platform-capability"], "file-storage");

    const appFileContent = openapi.paths["/api/app/files/{id}/content"].get;
    assert.deepEqual(appFileContent.security, [{ appBearerAuth: [] }]);
    assert.equal(appFileContent.operationId, "appFilesContent");
    assert.equal(appFileContent["x-platform-capability"], "file-storage");
  });

  it("generates app codegen preview from the same app route contract", () => {
    const preview = runScript("generate-app-codegen-preview.mjs");

    assert.equal(preview.generatedBy, "scripts/generate-app-codegen-preview.mjs");
    assert.equal(preview.source, "resources/generated/app-route-contract.json");
    assert.equal(preview.securityDomain, "app");
    assert.equal(preview.summary.routeCount, 5);
    assert.deepEqual(preview.summary.capabilities, ["file-storage", "session"]);
    assert.deepEqual(preview.summary.authModes, { public: 1, session: 4 });

    const login = preview.routes.find((route) => route.path === "/api/app/auth/login");
    assert.ok(login, "expected login route in app codegen preview");
    assert.equal(login.operationId, "appAuthLogin");
    assert.equal(login.auth, "public");
    assert.equal(login.client.tokenType, "none");

    const current = preview.routes.find((route) => route.path === "/api/app/session/current");
    assert.ok(current, "expected current session route in app codegen preview");
    assert.equal(current.client.tokenType, "app");

    const appFileUpload = preview.routes.find(
      (route) => route.method === "POST" && route.path === "/api/app/files",
    );
    assert.ok(appFileUpload, "expected app file upload route in app codegen preview");
    assert.equal(appFileUpload.operationId, "appFilesCreate");
    assert.equal(appFileUpload.capabilityId, "file-storage");
    assert.equal(appFileUpload.client.tokenType, "app");

    const appFileContent = preview.routes.find(
      (route) => route.method === "GET" && route.path === "/api/app/files/:id/content",
    );
    assert.ok(appFileContent, "expected app file content route in app codegen preview");
    assert.equal(appFileContent.operationId, "appFilesContent");
    assert.equal(appFileContent.capabilityId, "file-storage");
    assert.equal(appFileContent.client.tokenType, "app");
  });
});
