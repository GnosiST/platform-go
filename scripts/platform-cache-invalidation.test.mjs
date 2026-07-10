import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-cache-invalidation.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-cache-invalidation-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-cache-invalidation-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
  return filePath;
}

function serverSourceWithAdminReload() {
  const source = fs.readFileSync(path.join(repoRoot, "internal/platform/httpapi/server.go"), "utf8");
  const reloadGuard = [
    "\t\tif err := s.resources.Reload(); err != nil {",
    "\t\t\treturn",
    "\t\t}",
  ].join("\n");
  if (source.includes(reloadGuard)) {
    return source;
  }
  const invalidation = "\t\ts.invalidateCachesForResourceLocal(ctx, event.Resource)";
  const prepared = source.replace(invalidation, `${reloadGuard}\n${invalidation}`);
  assert.notEqual(prepared, source, "fixture must locate the admin invalidation callback");
  return prepared;
}

describe("validate-platform-cache-invalidation", () => {
  it("accepts the current cache invalidation contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform cache invalidation/);
  });

  it("rejects missing cache targets and invalidation rules", () => {
    const contract = readJSON("resources/platform-cache-invalidation.json");
    contract.cacheTargets = contract.cacheTargets.filter((target) => target.id !== "menus");
    const permissionsRule = contract.invalidationRules.find((rule) => rule.resource === "permissions");
    permissionsRule.operations = permissionsRule.operations.filter((operation) => operation !== "deletePrefix:admin:schema:");
    const contractPath = tempJSON("platform-cache-invalidation.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /cacheTargets must include menus/);
    assert.match(result.stderr, /invalidationRules\.permissions\.operations must include deletePrefix:admin:schema:/);
  });

  it("rejects unsafe bus policy drift", () => {
    const contract = readJSON("resources/platform-cache-invalidation.json");
    contract.busPolicy.redisChannel = "other-channel";
    contract.busPolicy.localInvalidationBeforePublish = false;
    contract.busPolicy.businessCapabilitiesMustNotImportRedis = false;
    const contractPath = tempJSON("platform-cache-invalidation.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /busPolicy\.redisChannel must stay platform:cache:invalidations/);
    assert.match(result.stderr, /busPolicy\.localInvalidationBeforePublish must stay true/);
    assert.match(result.stderr, /busPolicy\.businessCapabilitiesMustNotImportRedis must stay true/);
  });

  it("rejects an admin invalidation callback without Store reload", () => {
    const source = serverSourceWithAdminReload();
    const mutated = source.replace(
      /\n\t\tif err := s\.resources\.Reload\(\); err != nil \{\n\t\t\treturn\n\t\t\}\n/,
      "\n",
    );
    assert.notEqual(mutated, source, "fixture must remove the admin Store reload guard");

    const result = runValidator(["--server", tempText("server.go", mutated)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin invalidation callback must reload resources before derived cache invalidation/);
  });

  it("rejects derived cache invalidation before admin Store reload", () => {
    const source = serverSourceWithAdminReload();
    const reloadThenInvalidate = [
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\treturn",
      "\t\t}",
      "\t\ts.invalidateCachesForResourceLocal(ctx, event.Resource)",
    ].join("\n");
    const invalidateThenReload = [
      "\t\ts.invalidateCachesForResourceLocal(ctx, event.Resource)",
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\treturn",
      "\t\t}",
    ].join("\n");
    const mutated = source.replace(reloadThenInvalidate, invalidateThenReload);
    assert.notEqual(mutated, source, "fixture must move derived cache invalidation before reload");

    const result = runValidator(["--server", tempText("server.go", mutated)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin invalidation callback must reload resources before derived cache invalidation/);
  });

  it("rejects an admin reload error guard that clears derived caches", () => {
    const source = serverSourceWithAdminReload();
    const guardedReload = [
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\treturn",
      "\t\t}",
    ].join("\n");
    const unguardedReload = [
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\ts.invalidateCachesForResourceLocal(ctx, event.Resource)",
      "\t\t\treturn",
      "\t\t}",
    ].join("\n");
    const mutated = source.replace(guardedReload, unguardedReload);
    assert.notEqual(mutated, source, "fixture must clear derived caches inside the reload error guard");

    const result = runValidator(["--server", tempText("server.go", mutated)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin invalidation callback must preserve derived caches when resource reload fails/);
  });

  it("rejects an admin reload error guard with only a conditional return", () => {
    const source = serverSourceWithAdminReload();
    const guardedReload = [
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\treturn",
      "\t\t}",
    ].join("\n");
    const conditionalReturn = [
      "\t\tif err := s.resources.Reload(); err != nil {",
      "\t\t\tif event.Resource != \"\" {",
      "\t\t\t\treturn",
      "\t\t\t}",
      "\t\t}",
    ].join("\n");
    const mutated = source.replace(guardedReload, conditionalReturn);
    assert.notEqual(mutated, source, "fixture must make the reload error return conditional");

    const result = runValidator(["--server", tempText("server.go", mutated)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin invalidation callback must preserve derived caches when resource reload fails/);
  });

  it("rejects engineering matrix entries that do not cite the cache contract validator and tests", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "runtime-cache-invalidation");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-cache-invalidation.json");
    capability.evidence.validators = [];
    capability.evidence.tests = [];
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtime-cache-invalidation must cite resources\/platform-cache-invalidation\.json/);
    assert.match(result.stderr, /runtime-cache-invalidation must cite validate-platform-cache-invalidation\.mjs/);
    assert.match(result.stderr, /runtime-cache-invalidation must cite platform-cache-invalidation\.test\.mjs/);
  });
});
