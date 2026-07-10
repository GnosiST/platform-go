import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-app-client-api-boundary.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-app-client-api-boundary-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-app-client-api-boundary", () => {
  it("accepts the current app client API boundary contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated app client API boundary/);
  });

  it("rejects weakening the generated client and token injection boundary", () => {
    const boundary = readJSON("resources/platform-app-client-api-boundary.json");
    boundary.clientBoundary.generatedClientBoundary = "future-app/src/api/raw.ts";
    boundary.clientBoundary.tokenPolicy.tokenInjectionOwner = "business-page";
    const boundaryPath = tempJSON("platform-app-client-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /clientBoundary\.generatedClientBoundary must stay future-app\/src\/platform\/api\/appClient\.ts/);
    assert.match(result.stderr, /tokenPolicy\.tokenInjectionOwner must stay generated-app-client-or-app-request-port/);
  });

  it("rejects contracts that stop forbidding page-level request, upload and Authorization wiring", () => {
    const boundary = readJSON("resources/platform-app-client-api-boundary.json");
    boundary.clientBoundary.forbiddenPageLevelPatterns = ["console.log"];
    boundary.clientBoundary.forbiddenApiTargets = [];
    const boundaryPath = tempJSON("platform-app-client-api-boundary.json", boundary);

    const result = runValidator(["--boundary", boundaryPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /clientBoundary\.forbiddenPageLevelPatterns must include uni\.request/);
    assert.match(result.stderr, /clientBoundary\.forbiddenPageLevelPatterns must include Authorization/);
    assert.match(result.stderr, /clientBoundary\.forbiddenApiTargets must include \/api\/admin/);
  });

  it("rejects app codegen previews that do not route through the app client boundary", () => {
    const preview = readJSON("resources/generated/app-codegen-preview.json");
    preview.routes[0].client.apiClientFile = "future-app/src/pages/rawRequest.ts";
    preview.guardrails = [];
    const previewPath = tempJSON("app-codegen-preview.json", preview);

    const result = runValidator(["--app-codegen-preview", previewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /app codegen preview guardrails must include generated clients must stay behind the app API boundary/);
    assert.match(result.stderr, /app codegen route appAuthLogin client\.apiClientFile must stay future-app\/src\/platform\/api\/appClient\.ts/);
  });

  it("rejects app OpenAPI contracts that expose admin paths or drop app bearer security", () => {
    const openapi = readJSON("resources/generated/openapi.app.json");
    openapi.paths["/api/admin/resources/users/query"] = {};
    delete openapi.components.securitySchemes.appBearerAuth;
    const openapiPath = tempJSON("openapi.app.json", openapi);

    const result = runValidator(["--app-openapi", openapiPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /app OpenAPI must declare appBearerAuth security scheme/);
    assert.match(result.stderr, /app OpenAPI path must stay under \/api\/app: \/api\/admin\/resources\/users\/query/);
  });
});
