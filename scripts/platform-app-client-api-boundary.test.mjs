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
  it("publishes the unified App OpenAPI error contract", () => {
    const openapi = readJSON("resources/generated/openapi.app.json");
    const registry = readJSON("resources/generated/platform-error-code-contract.json");
    assert.equal(openapi["x-platform-error-registry-source"], "resources/generated/platform-error-code-contract.json");
    assert.equal(openapi["x-platform-error-registry-hash"], registry.contractHash);
    assert.deepEqual(openapi.components.schemas.PlatformErrorCode.enum, registry.definitions.map((definition) => definition.code));
    assert.deepEqual(openapi.components.schemas.ErrorBody.required, ["code", "message", "requestId", "traceId"]);
    assert.equal(openapi.components.schemas.ErrorBody.additionalProperties, false);
    assert.equal(openapi.components.responses.BadRequest.content["application/json"].schema.$ref, "#/components/schemas/ErrorResponse");
    assert.equal(openapi.components.schemas.ErrorResponse.properties.data, undefined);
  });

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

  it("rejects unknown and cross-plane explicit App OpenAPI error codes", () => {
    const openapi = readJSON("resources/generated/openapi.app.json");
    const operation = Object.values(openapi.paths)[0].post ?? Object.values(openapi.paths)[0].get;
    operation.responses["400"]["x-platform-error-codes"] = ["UNKNOWN_PLATFORM_ERROR", "ADMIN_FORBIDDEN"];
    const result = runValidator(["--app-openapi", tempJSON("openapi.app.json", openapi)]);

    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /unknown platform error code UNKNOWN_PLATFORM_ERROR/);
    assert.match(result.stderr, /platform error code ADMIN_FORBIDDEN does not belong to plane app/);
  });

  it("rejects App error responses that reference the wrong OpenAPI component", () => {
    const openapi = readJSON("resources/generated/openapi.app.json");
    openapi.paths["/api/app/auth/login"].post.responses["400"] = {
      $ref: "#/components/schemas/ErrorResponse",
    };
    const result = runValidator(["--app-openapi", tempJSON("openapi.app.json", openapi)]);

    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /POST \/api\/app\/auth\/login response 400 must reference a component response/);
  });
});
