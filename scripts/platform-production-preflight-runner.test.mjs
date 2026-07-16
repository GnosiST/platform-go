import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runRunner(args = []) {
  return spawnSync(process.execPath, ["scripts/run-platform-production-preflight.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "platform-production-preflight-"));
}

function tempJSON(name, value) {
  const dir = tempDir();
  const filePath = path.join(dir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("platform production preflight runner", () => {
  it("lists current production preflight commands without executing them", () => {
    const result = runRunner(["--list", "--json"]);

    assert.equal(result.status, 0, result.stderr);
    const output = JSON.parse(result.stdout);
    assert.equal(output.generatedBy, "scripts/run-platform-production-preflight.mjs");
    assert.equal(output.mode, "list");
    assert.equal(output.dryRun, true);
    assert.equal(output.source, "resources/platform-production-readiness.json");
    assert.equal(output.commandCount, readJSON("resources/platform-production-readiness.json").preflightCommands.length);
    assert.ok(output.commands.some((command) => command.id === "production-env-audit"));
    assert.ok(output.commands.some((command) => command.id === "platform-operations-plan"));
    assert.ok(output.commands.every((command) => command.status === "pending"));
  });

  it("dry-runs policy and command selections with stable de-duplication", () => {
    const result = runRunner(["--policy", "token-rotation", "--command", "deployment-topology", "--json"]);

    assert.equal(result.status, 0, result.stderr);
    const output = JSON.parse(result.stdout);
    const ids = output.commands.map((command) => command.id);

    assert.equal(output.mode, "dry-run");
    assert.equal(output.dryRun, true);
    assert.deepEqual(output.selection.policies, ["token-rotation"]);
    assert.deepEqual(output.selection.commands, ["deployment-topology"]);
    assert.equal(new Set(ids).size, ids.length);
    assert.equal(ids[0], "production-runtime-tests");
    assert.ok(ids.includes("production-auth-hardening"));
    assert.ok(ids.includes("deployment-topology"));
  });

  it("rejects unknown command and policy identifiers", () => {
    const result = runRunner(["--policy", "missing-policy", "--command", "missing-command", "--json"]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Unknown preflight policy missing-policy/);
    assert.match(result.stderr, /Unknown preflight command missing-command/);
  });

  it("does not execute selected commands unless --run is explicit", () => {
    const dir = tempDir();
    const markerPath = path.join(dir, "preflight-ran.txt");
    const scriptPath = path.join(dir, "write-marker.mjs");
    fs.writeFileSync(scriptPath, `import fs from "node:fs";\nfs.writeFileSync(${JSON.stringify(markerPath)}, "ran");\n`);
    const readinessPath = tempJSON("platform-production-readiness.json", {
      purpose: "Temporary readiness contract for runner execution tests.",
      preflightCommands: [
        {
          id: "write-marker",
          command: `node ${scriptPath}`,
          purpose: "Writes a marker so tests can prove --run is required.",
        },
      ],
      operationPolicies: [],
      policyPreflightRequirements: [],
    });

    const dryRun = runRunner(["--readiness", readinessPath, "--command", "write-marker", "--json"]);
    assert.equal(dryRun.status, 0, dryRun.stderr);
    assert.equal(fs.existsSync(markerPath), false);
    assert.equal(JSON.parse(dryRun.stdout).commands[0].status, "pending");

    const run = runRunner(["--readiness", readinessPath, "--command", "write-marker", "--run", "--json"]);
    assert.equal(run.status, 0, run.stderr);
    assert.equal(fs.readFileSync(markerPath, "utf8"), "ran");
    const output = JSON.parse(run.stdout);
    assert.equal(output.mode, "run");
    assert.equal(output.dryRun, false);
    assert.equal(output.commands[0].status, "passed");
    assert.equal(output.commands[0].exitCode, 0);
  });

  it("adds strict secret validation arguments for the production env audit command", () => {
    const envPath = path.join(tempDir(), "production.env");
    fs.writeFileSync(envPath, "PLATFORM_RUNTIME_ENV=production\n");

    const result = runRunner(["--command", "production-env-audit", "--strict-env-file", envPath, "--json"]);

    assert.equal(result.status, 0, result.stderr);
    const output = JSON.parse(result.stdout);
    assert.equal(output.commands.length, 1);
    assert.equal(
      output.commands[0].command,
      `rtk node scripts/validate-platform-production-env.mjs --env-file ${envPath} --strict-secrets`,
    );
  });
});
