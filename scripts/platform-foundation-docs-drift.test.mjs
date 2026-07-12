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
      assert.doesNotMatch(source, /45 total \/ 37 implemented \/ 8 controlled unfinished|45\/37\/8/);
    }
  });

  it("keeps the remaining completion nodes in task-graph order", () => {
    const source = read("docs/platform-foundation-task-map.md");

    assert.ok(source.includes(`Remaining nodes, in task-graph order: ${orderedRemainingSummary}.`));
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
