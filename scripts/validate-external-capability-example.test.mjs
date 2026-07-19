import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-external-capability-example.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function copyExampleFixture(name) {
  const target = fs.mkdtempSync(path.join(os.tmpdir(), name));
  fs.rmSync(target, { recursive: true, force: true });
  fs.cpSync(path.join(repoRoot, "examples/external-capability"), target, { recursive: true });
  return target;
}

describe("validate-external-capability-example", () => {
  it("runs the external capability example through public contracts", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated external capability example/);
  });

  it("rejects a missing example directory", () => {
    const result = runValidator(["--example-dir", "examples/missing-external-capability"]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /external capability example directory is missing/);
  });

  it("rejects platform internal imports in the example", () => {
    const fixture = copyExampleFixture("platform-go-external-capability-internal-");
    try {
      fs.writeFileSync(
        path.join(fixture, "internal_import.go"),
        'package main\n\nimport _ "github.com/GnosiST/platform-go/internal/platform/config"\n',
      );
      const result = runValidator(["--example-dir", fixture]);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /must not import platform internal packages/);
    } finally {
      fs.rmSync(fixture, { recursive: true, force: true });
    }
  });

  it("rejects a template without a settings resource", () => {
    const fixture = copyExampleFixture("platform-go-external-capability-template-");
    try {
      const templatePath = path.join(fixture, "business-project-template.json");
      const template = JSON.parse(fs.readFileSync(templatePath, "utf8"));
      template.adminSurface.resources = template.adminSurface.resources.filter((resource) => resource.resource !== "catalog-settings");
      fs.writeFileSync(templatePath, `${JSON.stringify(template, null, 2)}\n`);
      const result = runValidator(["--example-dir", fixture]);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /catalog-settings/);
    } finally {
      fs.rmSync(fixture, { recursive: true, force: true });
    }
  });
});
