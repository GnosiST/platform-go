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

  it("requires the admin oidc start and exchange OpenAPI contract", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    const start = openAPI.paths?.["/api/auth/providers/{provider}/start"]?.post;
    const login = openAPI.paths?.["/api/auth/login"]?.post;

    assert.equal(start?.operationId, "startAdminAuthProvider");
    assert.deepEqual(start?.security, []);
    assert.equal(
      start?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminAuthProviderStartRequest",
    );
    assert.equal(
      start?.responses?.["200"]?.content?.["application/json"]?.schema?.properties?.data?.$ref,
      "#/components/schemas/AdminAuthProviderStartData",
    );
    assert.ok(start?.responses?.["501"]);
    assert.equal(login?.operationId, "adminAuthLogin");
    assert.deepEqual(login?.security, []);
    assert.equal(
      login?.requestBody?.content?.["application/json"]?.schema?.$ref,
      "#/components/schemas/AdminAuthLoginRequest",
    );
    assert.ok(login?.responses?.["501"]);
    const challenge = openAPI.components?.schemas?.AdminAuthProviderStartRequest?.properties?.codeChallenge;
    assert.equal(challenge?.minLength, 43);
    assert.equal(challenge?.maxLength, 43);
    assert.equal(challenge?.pattern, "^[A-Za-z0-9_-]{43}$");
    const loginProperties = openAPI.components?.schemas?.AdminAuthLoginRequest?.properties ?? {};
    assert.ok(loginProperties.state);
    assert.ok(loginProperties.codeVerifier);
    assert.equal(loginProperties.codeVerifier.writeOnly, true);
    assert.deepEqual(
      Object.keys(openAPI.components?.schemas?.AdminAuthProviderStartData?.properties ?? {}).sort(),
      ["authorizationUrl", "expiresAt", "state"],
    );
  });

  it("rejects secret fields in the admin oidc start response", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    openAPI.components.schemas.AdminAuthProviderStartData ??= { type: "object", properties: {} };
    openAPI.components.schemas.AdminAuthProviderStartData.properties.clientSecret = { type: "string" };
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminAuthProviderStartData must not expose sensitive response field clientSecret/);
  });

  it("rejects a weakened admin oidc challenge or missing resolver response", () => {
    const openAPI = readJSON("resources/generated/openapi.admin.json");
    openAPI.components.schemas.AdminAuthProviderStartRequest.properties.codeChallenge.maxLength = 128;
    delete openAPI.paths["/api/auth/providers/{provider}/start"].post.responses["501"];
    const openAPIPath = tempJSON("openapi.admin.json", openAPI);

    const result = runValidator(["--admin-openapi", openAPIPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /AdminAuthProviderStartRequest must require an exact S256 codeChallenge/);
    assert.match(result.stderr, /admin OpenAPI auth operations must declare the missing resolver 501 response/);
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
