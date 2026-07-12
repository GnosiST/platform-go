import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const completionProgramTaskIDs = [
  "sensitive-data-protection-runtime",
  "sensitive-data-historical-migration",
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];

const foundationBaselineCloseoutTaskIDs = [
  "stack-alignment-and-architecture",
  "capability-manifest-contract",
  "resource-schema-contract",
  "capability-profile-composition-gate",
  "capability-contract-governance",
  "rbac-menu-data-scope",
  "governance-org-area-role-groups",
  "auth-session-provider-jwt-wechat",
  "gorm-storage-runtime",
  "cache-redis-invalidation",
  "production-persistence-correctness",
  "production-runtime-gate",
  "production-readiness-preflight",
  "openapi-app-contracts",
  "admin-api-boundary-query-security",
  "codegen-preview-scaffold",
  "codegen-source-writing-readiness",
  "admin-ui-shell-and-list-components",
  "branding-demo-data-dashboard",
  "personnel-extension-boundary",
  "notification-extension-boundary",
  "job-extension-boundary",
  "visual-product-design-qa",
  "policy-review-and-audit-workflow",
  "form-schema-layout-and-slots",
  "refine-custom-panels-and-actions",
  "file-storage-preview-and-audit-workflow",
  "policy-review-custom-ui",
  "task-dependency-governance",
  "reference-discovery-classification-gate",
  "reference-coverage-boundary-gate",
  "node-closeout-audit",
  "foundation-alignment-audit",
  "production-auth-provider-hardening",
  "source-writing-codegen-promotion",
  "admin-ui-system-quality-hardening",
  "production-admin-oidc-auth",
];

const foundationBaselineCloseoutDigest = "3f40fe426cb54ca5b75221721158f3f37cac400baa7f32ab680ebe447a082c59";

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

  it("preserves 37 baseline closeouts, closes runtime security and watermark governance, and tracks six pending nodes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const task = graph.tasks.find((item) => item.id === "production-admin-oidc-auth");

    assert.ok(task, "task graph must include production-admin-oidc-auth");
    assert.equal(task.status, "implemented");
    assert.equal(audit.nodeCloseouts.some((item) => item.taskId === task.id), true);
    assert.equal(audit.nodeCloseouts.length, 39);
    assert.deepEqual(audit.nodeCloseouts.slice(0, 37).map((item) => item.taskId), foundationBaselineCloseoutTaskIDs);
    assert.equal(createHash("sha256").update(JSON.stringify(audit.nodeCloseouts.slice(0, 37))).digest("hex"), foundationBaselineCloseoutDigest);
    const runtimeSecurityCloseout = audit.nodeCloseouts[37];
    assert.equal(runtimeSecurityCloseout.taskId, "runtime-security-containment");
    assert.equal(runtimeSecurityCloseout.status, "closed");
    assert.equal(runtimeSecurityCloseout.neatFreak, true);
    assert.ok(runtimeSecurityCloseout.cleanupEvidence.length > 0);
    const watermarkCloseout = audit.nodeCloseouts[38];
    assert.equal(watermarkCloseout.taskId, "admin-watermark-export-governance");
    assert.equal(watermarkCloseout.status, "closed");
    assert.equal(watermarkCloseout.neatFreak, true);
    assert.ok(watermarkCloseout.cleanupEvidence.length > 0);
    assert.ok(watermarkCloseout.visualEvidence.includes("product-design"));
    assert.ok(watermarkCloseout.visualEvidence.includes("ui-ux-pro-max"));
    assert.ok(watermarkCloseout.visualEvidence.includes("browser:control-in-app-browser"));
    assert.deepEqual(audit.pendingNodeEvidence, completionProgramTaskIDs);
  });

  it("rejects watermark closeout without Product Design, UI UX and browser evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((item) => item.taskId === "admin-watermark-export-governance");
    closeout.visualEvidence = ["superpowers:brainstorming"];

    const result = runValidator(["--audit", tempJSON("missing-watermark-visual-evidence.json", audit)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.admin-watermark-export-governance\.visualEvidence must include product-design/);
    assert.match(result.stderr, /nodeCloseouts\.admin-watermark-export-governance\.visualEvidence must include ui-ux-pro-max/);
    assert.match(result.stderr, /nodeCloseouts\.admin-watermark-export-governance\.visualEvidence must include browser:control-in-app-browser/);
  });

  it("rejects omitting runtime security closeout evidence after implementation", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const task = graph.tasks.find((item) => item.id === "runtime-security-containment");
    assert.ok(task, "runtime security completion node must exist before closeout mutation validation");
    assert.equal(task.status, "implemented");
    audit.nodeCloseouts = audit.nodeCloseouts.filter((item) => item.taskId !== task.id);

    const result = runValidator(["--audit", tempJSON("missing-runtime-security-closeout-audit.json", audit)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /implemented task runtime-security-containment must declare node closeout evidence/);
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
