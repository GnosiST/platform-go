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
      assert.doesNotMatch(source, /45 total \/ (?:37|38) implemented \/ (?:8|7) controlled unfinished|45\/(?:37\/8|38\/7)|66 total \/ (?:51|52) implemented \/ (?:15|14) controlled unfinished/);
    }
  });

  it("records watermark export governance as implemented", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
      "docs/platform-ui-optimization-assessment.md",
    ]) {
      assert.match(read(relativePath), /`admin-watermark-export-governance`[^.\n]*`implemented`/);
    }
  });

  it("keeps the remaining completion nodes in task-graph order", () => {
    const source = read("docs/platform-foundation-task-map.md");

    assert.ok(source.includes(`Remaining nodes, in task-graph order: ${orderedRemainingSummary}.`));
    assert.doesNotMatch(source, /ordered seven-node remainder/);
  });

  it("documents the v0.1 release lanes and support boundary", () => {
    const readme = read("README.md");
    const topology = read("docs/superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md");
    const release = graph.releaseBlockingNodes.map((id) => `\`${id}\``).join(", ");
    const optional = graph.postReleaseOptionalNodes.map((id) => `\`${id}\``).join(", ");

    assert.ok(topology.includes(`v0.1.0 release blockers: ${release}.`));
    assert.ok(topology.includes(`Post-release optional deferred nodes: ${optional}.`));
    assert.match(readme, /one datasource and one native transaction boundary/);
    assert.match(readme, /SQLite is development\/test-only by support policy/);
    assert.match(readme, /Oracle and KingbaseES are unsupported/);
    assert.match(readme, /`alibaba\/page-agent` is only a default-off optional `public-docs-site` sub-capability/);
  });

  it("records organization and user Admin experience as implemented", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
      "docs/platform-ui-optimization-assessment.md",
      "docs/platform-data-governance-and-integrations-assessment.md",
      "docs/platform-organization-rbac-menu-contract.md",
      "docs/admin-rbac-menu.md",
      "docs/admin-resource-schema.md",
      "design-qa.md",
    ]) {
      const source = read(relativePath);
      assert.match(source, /organization-user-admin-experience|organization\/user Admin/i);
    }
    assert.match(read("design-qa.md"), /playwright 1\.55 fallback/i);
    assert.doesNotMatch(read("docs/platform-foundation-task-map.md"), /organization\/user Admin experience is the next organization-lane node/);
  });

  it("keeps completed plan and sensitive-data dependency wording honest", () => {
    const plan = read("docs/superpowers/plans/2026-07-12-platform-completion-task-graph.md");
    const assessment = read("docs/platform-data-governance-and-integrations-assessment.md");
    const trackedSteps = plan.split("\n").filter((line) => /^- \[[ x]\] \*\*Step /.test(line));

    assert.match(plan, /> \*\*Status:\*\* Completed\./);
    assert.ok(trackedSteps.length > 0, "completed plan must retain tracked steps");
    assert.ok(trackedSteps.every((line) => line.startsWith("- [x]")), "every tracked step must be checked");
    assert.doesNotMatch(
      assessment,
      /Depends on the existing `sensitive-data-protection-runtime` field, encryption and key-provider contracts\./,
    );
  });

  it("records verified masking, step-up, database and integration limits", () => {
    const assessment = read("docs/platform-data-governance-and-integrations-assessment.md");

    assert.match(
      assessment,
      /Encrypted `masked` projections return only the masked value/,
    );
    assert.doesNotMatch(assessment, /`masked` projection returns the stored value unchanged/);
    assert.match(assessment, /initial reveal-capable factor set is implemented as OIDC reauthentication and Admin SMS OTP/);
    assert.doesNotMatch(assessment, /There are currently zero reveal-capable step-up factors/);
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
      assert.match(read(relativePath), /`runtime-security-containment`[^.\n]*`implemented`/);
    }
    assert.match(read(".superpowers/sdd/runtime-security-progress.md"), /Task 7: complete/);
  });

  it("records configurable sensitive data protection as implemented", () => {
    for (const relativePath of ["README.md", "docs/platform-foundation-task-map.md", "docs/platform-roadmap.md"]) {
      assert.match(read(relativePath), /`sensitive-data-protection-runtime`[^.\n]*`implemented`/);
    }
    assert.match(read("docs/admin-resource-schema.md"), /Sensitive fields are not identified by a built-in list of names/);
    assert.match(read(".superpowers/sdd/sensitive-data-progress.md"), /Task 4: complete/);
  });

  it("records sensitive data historical migration as implemented", () => {
    for (const relativePath of [
      "README.md",
      "docs/platform-data-governance-and-integrations-assessment.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-roadmap.md",
    ]) {
      assert.match(read(relativePath), /`sensitive-data-historical-migration`[^.\n]*`implemented`/);
    }
    assert.match(read("docs/platform-sensitive-data-migration.md"), /platform-admin sensitive-data-migrate/);
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
