import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const completionProgramTaskIDs = [
  "platform-service-contract-standard",
  "persisted-query-command-object-runtime",
  "integration-ports-disabled-default",
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
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

  it("preserves 37 baseline closeouts, closes seven completion nodes, and tracks 22 pending nodes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const task = graph.tasks.find((item) => item.id === "production-admin-oidc-auth");

    assert.ok(task, "task graph must include production-admin-oidc-auth");
    assert.equal(task.status, "implemented");
    assert.equal(audit.nodeCloseouts.some((item) => item.taskId === task.id), true);
    assert.equal(audit.nodeCloseouts.length, 44);
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
    const sensitiveDataCloseout = audit.nodeCloseouts[39];
    assert.equal(sensitiveDataCloseout.taskId, "sensitive-data-protection-runtime");
    assert.equal(sensitiveDataCloseout.status, "closed");
    assert.equal(sensitiveDataCloseout.neatFreak, true);
    assert.ok(sensitiveDataCloseout.cleanupEvidence.includes("internal/platform/adminresource/protection_test.go"));
    const migrationCloseout = audit.nodeCloseouts.find((item) => item.taskId === "sensitive-data-historical-migration");
    assert.equal(migrationCloseout.status, "closed");
    assert.equal(migrationCloseout.neatFreak, true);
    assert.equal("visualEvidence" in migrationCloseout, false);
    assert.ok(migrationCloseout.cleanupEvidence.includes("docs/platform-sensitive-data-migration.md"));
    assert.ok(migrationCloseout.cleanupEvidence.includes("docs/platform-data-governance-and-integrations-assessment.md"));
    assert.ok(migrationCloseout.cleanupEvidence.includes("scripts/validate-platform-sensitive-data-migration.mjs"));
    assert.ok(migrationCloseout.cleanupEvidence.includes("scripts/platform-foundation-docs-drift.test.mjs"));
    const maskCloseout = audit.nodeCloseouts.find((item) => item.taskId === "mask-strategy-runtime");
    assert.equal(maskCloseout.status, "closed");
    assert.equal(maskCloseout.neatFreak, false);
    assert.equal(maskCloseout.cleanupMode, "focused");
    assert.equal("visualEvidence" in maskCloseout, false);
    assert.ok(maskCloseout.cleanupEvidence.includes("internal/platform/masking/runtime_test.go"));
    assert.ok(maskCloseout.cleanupEvidence.includes("internal/platform/httpapi/projection_test.go"));
    const revealCloseout = audit.nodeCloseouts.find((item) => item.taskId === "sensitive-data-reveal-step-up");
    assert.equal(revealCloseout.status, "closed");
    assert.equal(revealCloseout.neatFreak, true);
    assert.ok(revealCloseout.cleanupEvidence.includes("internal/platform/httpapi/sensitive_reveal_test.go"));
    assert.ok(revealCloseout.visualEvidence.includes("product-design"));
    assert.ok(revealCloseout.visualEvidence.includes("ui-ux-pro-max"));
    assert.ok(revealCloseout.visualEvidence.includes("browser:control-in-app-browser"));
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

  it("rejects reveal closeout without Product Design, UI UX and browser evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((item) => item.taskId === "sensitive-data-reveal-step-up");
    closeout.visualEvidence = ["superpowers:brainstorming"];

    const result = runValidator(["--audit", tempJSON("missing-reveal-visual-evidence.json", audit)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.sensitive-data-reveal-step-up\.visualEvidence must include product-design/);
    assert.match(result.stderr, /nodeCloseouts\.sensitive-data-reveal-step-up\.visualEvidence must include ui-ux-pro-max/);
    assert.match(result.stderr, /nodeCloseouts\.sensitive-data-reveal-step-up\.visualEvidence must include browser:control-in-app-browser/);
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

  it("rejects closeout entries that omit cleanup evidence", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "admin-ui-shell-and-list-components");
    closeout.cleanupEvidence = [];
    const auditPath = tempJSON("platform-node-closeout-audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.admin-ui-shell-and-list-components\.cleanupEvidence must not be empty/);
  });

  it("allows focused cleanup for a small node and rejects missing cleanup mode", () => {
    const audit = readJSON("resources/platform-node-closeout-audit.json");
    const closeout = audit.nodeCloseouts.find((node) => node.taskId === "mask-strategy-runtime");
    assert.equal(closeout.neatFreak, false);
    assert.equal(closeout.cleanupMode, "focused");

    delete closeout.cleanupMode;
    const result = runValidator(["--audit", tempJSON("missing-focused-cleanup-mode.json", audit)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /nodeCloseouts\.mask-strategy-runtime\.cleanupMode must be focused when neat-freak was not invoked/);
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
