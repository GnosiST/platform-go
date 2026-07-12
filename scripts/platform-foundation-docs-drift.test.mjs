import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

function readJSON(relativePath) {
  return JSON.parse(read(relativePath));
}

const graph = readJSON("resources/platform-foundation-task-graph.json");
const implementedCount = graph.tasks.filter((task) => task.status === "implemented").length;
const remainingTaskIDs = graph.tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
const graphSummary = `${graph.tasks.length} total / ${implementedCount} implemented / ${remainingTaskIDs.length} controlled unfinished`;
const orderedRemainingSummary = remainingTaskIDs.map((taskID) => `\`${taskID}\``).join(", ");

describe("platform foundation documentation drift", () => {
  it("keeps completion counts aligned with the task graph", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
      "docs/platform-ui-optimization-assessment.md",
    ]) {
      const source = read(relativePath);
      assert.ok(source.includes(graphSummary), `${relativePath} must include ${graphSummary}`);
      assert.doesNotMatch(source, /45 total \/ (?:37|38) implemented \/ (?:8|7) controlled unfinished|45\/(?:37\/8|38\/7)/);
    }
  });

  it("records watermark export governance as implemented", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
      "docs/platform-ui-optimization-assessment.md",
    ]) {
      assert.match(read(relativePath), /`admin-watermark-export-governance` is `implemented`/);
    }
  });

  it("keeps the remaining completion nodes in task-graph order", () => {
    const source = read("docs/platform-foundation-task-map.md");

    assert.ok(source.includes(`Remaining nodes, in task-graph order: ${orderedRemainingSummary}.`));
    assert.doesNotMatch(source, /ordered seven-node remainder/);
  });

  it("keeps completed plan and sensitive-data dependency wording honest", () => {
    const plan = read("docs/superpowers/plans/2026-07-12-platform-completion-task-graph.md");
    const assessment = read("docs/platform-data-governance-and-integrations-assessment.md");

    assert.doesNotMatch(plan, /- \[ \]/);
    assert.doesNotMatch(
      assessment,
      /Depends on the existing `sensitive-data-protection-runtime` field, encryption and key-provider contracts\./,
    );
  });

  it("records verified masking, step-up, database and integration limits", () => {
    const assessment = read("docs/platform-data-governance-and-integrations-assessment.md");

    assert.match(assessment, /`masked` projection returns the stored value unchanged/);
    assert.match(assessment, /There are currently zero reveal-capable step-up factors/);
    assert.match(assessment, /`PLATFORM_DATABASE_DRIVER` and `PLATFORM_DATABASE_DSN` are not wired into process composition/);
    assert.doesNotMatch(assessment, /Redis Pub\/Sub is used only for cache invalidation/);
  });

  it("records runtime security containment as implemented", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
      "docs/platform-ui-optimization-assessment.md",
    ]) {
      assert.match(read(relativePath), /`runtime-security-containment` is `implemented`/);
    }
    assert.match(read(".superpowers/sdd/runtime-security-progress.md"), /Task 7: complete/);
  });

  it("documents the policy review workflow routes", () => {
    const source = read("README.md");

    for (const route of [
      "POST /api/admin/policy-reviews/:id/request",
      "POST /api/admin/policy-reviews/:id/reject",
      "POST /api/admin/policy-reviews/:id/approve",
      "GET /api/admin/policy-reviews/export",
    ]) {
      assert.ok(source.includes(route), `README.md must include ${route}`);
    }
  });

  it("documents the production runtime security environment", () => {
    const source = read("docs/platform-deployment.md");

    for (const setting of [
      "PLATFORM_RATE_LIMIT_HMAC_KEY=",
      "PLATFORM_FILE_MAX_UPLOAD_BYTES=",
      "PLATFORM_FILE_ALLOWED_MIME_TYPES=",
      "PLATFORM_PHONE_HMAC_KEY=",
      "PLATFORM_PHONE_CODE_HMAC_KEY=",
      "PLATFORM_PHONE_VERIFICATION_PROVIDER=",
    ]) {
      assert.ok(source.includes(setting), `docs/platform-deployment.md must include ${setting}`);
    }
    assert.match(source, /phone and verification-code HMAC keys.*distinct/i);
    assert.match(source, /verification provider.*must not be `debug`/i);
  });

  it("keeps the production env validator in the broad verification rules", () => {
    const source = read("AGENTS.md");

    assert.ok(source.includes("rtk node scripts/validate-platform-production-env.mjs"));
    assert.match(source, /Run `rtk node scripts\/validate-platform-production-env\.mjs` when changing production environment/);
  });
});
