import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function run(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-integration-ports.mjs", ...args], { cwd: repoRoot, encoding: "utf8" });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempFile(name, value) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-integration-ports-"));
  const file = path.join(dir, name);
  fs.writeFileSync(file, typeof value === "string" ? value : `${JSON.stringify(value, null, 2)}\n`);
  return file;
}

describe("validate-platform-integration-ports", () => {
  it("accepts the implemented disabled-by-default contract", () => {
    const result = run();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform integration ports governance/);
  });

  it("rejects an overstated runtime boundary", () => {
    const contract = readJSON("resources/platform-integration-ports.json");
    contract.runtimeBoundary.transactionalOutbox = "implemented";
    const result = run(["--contract", tempFile("contract.json", contract)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /runtime boundary must remain explicit and non-expansive/);
  });

  it("rejects a production template that enables the broker", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/env/production.example.env"), "utf8");
    const result = run(["--env", tempFile("production.env", current.replace("PLATFORM_MESSAGE_BUS_ENABLED=false", "PLATFORM_MESSAGE_BUS_ENABLED=true"))]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /production environment template must include PLATFORM_MESSAGE_BUS_ENABLED=false/);
  });

  it("rejects a silent disabled publisher", () => {
    const current = fs.readFileSync(path.join(repoRoot, "internal/platform/integration/disabled.go"), "utf8");
    const result = run(["--disabled", tempFile("disabled.go", current.replaceAll("return ErrMessageBusDisabled", "return nil"))]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /disabled integrations must include func \(disabledMessageBus\) Publish/);
  });
});
