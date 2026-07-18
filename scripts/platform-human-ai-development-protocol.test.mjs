import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-human-ai-development-protocol.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-human-ai-development-protocol-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-human-ai-development-protocol", () => {
  it("accepts the current human + AI development protocol", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated human \+ AI development protocol/);
  });

  it("rejects a protocol that drops core development domains", () => {
    const protocol = readJSON("resources/platform-human-ai-development-protocol.json");
    protocol.domains = protocol.domains.filter((domain) => domain.id !== "code-generation");
    const result = runValidator(["--protocol", tempJSON("protocol.json", protocol)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /domains must include code-generation/);
  });

  it("rejects a protocol that drops capability lifecycle operations", () => {
    const protocol = readJSON("resources/platform-human-ai-development-protocol.json");
    protocol.domains = protocol.domains.filter((domain) => domain.id !== "capability-lifecycle-operations");
    const result = runValidator(["--protocol", tempJSON("protocol.json", protocol)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /domains must include capability-lifecycle-operations/);
  });

  it("rejects a protocol that removes required validator gates", () => {
    const protocol = readJSON("resources/platform-human-ai-development-protocol.json");
    protocol.minimumAcceptanceCommands = ["rtk go test ./..."];
    protocol.requiredValidators = ["scripts/validate-platform-human-ai-development-protocol.mjs"];
    protocol.requiredTests = ["scripts/platform-human-ai-development-protocol.test.mjs"];
    const result = runValidator(["--protocol", tempJSON("protocol.json", protocol)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /minimumAcceptanceCommands must include rtk node scripts\/validate-platform-capability-operation-policy\.mjs/);
    assert.match(result.stderr, /minimumAcceptanceCommands must include rtk node scripts\/validate-platform-plugin-management-v1\.mjs/);
    assert.match(result.stderr, /minimumAcceptanceCommands must include rtk node scripts\/validate-external-capability-example\.mjs/);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-plugin-management-v1\.mjs/);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-platform-capability-operation-policy\.mjs/);
    assert.match(result.stderr, /requiredValidators must include scripts\/validate-admin-ui-contracts\.mjs/);
    assert.match(result.stderr, /requiredTests must include scripts\/platform-plugin-management-v1\.test\.mjs/);
    assert.match(result.stderr, /requiredTests must include scripts\/platform-capability-operation-policy\.test\.mjs/);
  });

  it("rejects missing source snippets in contributor-facing docs", () => {
    const protocol = readJSON("resources/platform-human-ai-development-protocol.json");
    protocol.requiredSourceSnippets = [
      {
        path: "README.md",
        contains: "this snippet should not exist in README",
      },
    ];
    const result = runValidator(["--protocol", tempJSON("protocol.json", protocol)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /README\.md is missing required snippet/);
  });
});
