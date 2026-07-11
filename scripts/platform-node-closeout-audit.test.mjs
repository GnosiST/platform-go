import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-node-closeout-audit.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-node-closeout-audit-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-node-closeout-audit", () => {
  it("accepts the current node closeout audit", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform node closeout audit/);
  });

  it("closes the production Admin OIDC node with Task 8 evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const task = graph.tasks.find((item) => item.id === "production-admin-oidc-auth");

    assert.ok(task, "task graph must include production-admin-oidc-auth");
    assert.equal(task.status, "implemented");
    assert.equal(audit.nodeCloseouts.some((item) => item.taskId === task.id), true);
    assert.equal(audit.nodeCloseouts.length, 37);
    assert.deepEqual(audit.pendingNodeEvidence, []);
  });

  it("rejects implemented task nodes without closeout evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    audit.nodeCloseouts = audit.nodeCloseouts.filter((node) => node.taskId !== "resource-schema-contract");
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /implemented task resource-schema-contract must declare node closeout evidence/);
  });

  it("rejects closeout entries that omit neat-freak cleanup evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "admin-ui-shell-and-list-components");
    closeout.neatFreak = false;
    closeout.cleanupEvidence = [];
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.admin-ui-shell-and-list-components\.neatFreak must stay true/);
    assert.match(result.stderr, /nodeCloseouts\.admin-ui-shell-and-list-components\.cleanupEvidence must not be empty/);
  });

  it("rejects visual closeouts that lack brainstorming and product-design evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "form-schema-layout-and-slots");
    closeout.visualEvidence = ["product-design"];
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.form-schema-layout-and-slots\.visualEvidence must include superpowers:brainstorming/);
  });

  it("rejects admin UI closeout without the UI contract drift test", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "admin-ui-shell-and-list-components");
    closeout.cleanupEvidence = closeout.cleanupEvidence.filter((item) => item !== "scripts/admin-ui-contracts.test.mjs");
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.admin-ui-shell-and-list-components\.cleanupEvidence must include scripts\/admin-ui-contracts\.test\.mjs/);
  });

  it("rejects production readiness closeout without operations plan and preflight runner evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "production-readiness-preflight");
    closeout.cleanupEvidence = closeout.cleanupEvidence.filter(
      (item) => item !== "resources/generated/platform-operations-plan.json" && item !== "scripts/platform-production-preflight-runner.test.mjs",
    );
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.production-readiness-preflight\.cleanupEvidence must include resources\/generated\/platform-operations-plan\.json/);
    assert.match(result.stderr, /nodeCloseouts\.production-readiness-preflight\.cleanupEvidence must include scripts\/platform-production-preflight-runner\.test\.mjs/);
  });
});
