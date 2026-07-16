import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const foundationBaselineTaskIDs = [
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
  "production-auth-provider-hardening",
  "form-schema-layout-and-slots",
  "refine-custom-panels-and-actions",
  "file-storage-preview-and-audit-workflow",
  "policy-review-custom-ui",
  "source-writing-codegen-promotion",
  "task-dependency-governance",
  "reference-discovery-classification-gate",
  "reference-coverage-boundary-gate",
  "node-closeout-audit",
  "foundation-alignment-audit",
  "admin-ui-system-quality-hardening",
  "production-admin-oidc-auth",
];

const completionProgramTaskIDs = [
  "runtime-security-containment",
  "admin-watermark-export-governance",
  "sensitive-data-protection-runtime",
  "sensitive-data-historical-migration",
  "mask-strategy-runtime",
  "sensitive-data-reveal-step-up",
  "data-lifecycle-retention",
  "platform-service-contract-standard",
  "persisted-query-command-object-runtime",
  "integration-ports-disabled-default",
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
  "unified-error-code-governance",
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

const releaseBlockingNodes = [
  "github-release-publication",
];

const postReleaseOptionalNodes = [
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
];

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-foundation-task-graph.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-foundation-task-graph-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-foundation-task-graph", () => {
  it("accepts the current foundation task dependency graph", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform foundation task nodes/);
  });

  it("rejects unknown task dependencies", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks[1].dependsOn.push("missing-task");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /depends on unknown task missing-task/);
  });

  it("rejects dependency cycles", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks[0].dependsOn.push(graph.tasks.at(-1).id);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /dependency cycle detected/);
  });

  it("rejects tasks that depend on later phases", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const stackTask = graph.tasks.find((task) => task.id === "stack-alignment-and-architecture");
    stackTask.dependsOn.push("production-runtime-gate");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task stack-alignment-and-architecture in phase stack-and-contracts cannot depend on later-phase task production-runtime-gate in phase production-governance/);
  });

  it("rejects phase dependency exceptions without localized rationale", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const openAPITask = graph.tasks.find((task) => task.id === "openapi-app-contracts");
    openAPITask.phaseDependencyExceptions[0].reason = { zh: "" };
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task openapi-app-contracts in phase stack-and-contracts cannot depend on later-phase task auth-session-provider-jwt-wechat in phase runtime-and-security/);
    assert.match(result.stderr, /task openapi-app-contracts phaseDependencyExceptions for auth-session-provider-jwt-wechat must declare zh\/en reason/);
  });

  it("rejects phase dependency exceptions for same-phase dependencies", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const openAPITask = graph.tasks.find((task) => task.id === "openapi-app-contracts");
    openAPITask.phaseDependencyExceptions.push({
      dependency: "resource-schema-contract",
      reason: { zh: "同阶段依赖不需要阶段例外。", en: "Same-phase dependencies do not need phase exceptions." },
    });
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task openapi-app-contracts phaseDependencyExceptions for resource-schema-contract must reference a later-phase dependency/);
  });

  it("rejects same-batch tasks that share resource locks", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "non-visual-contract-gates");
    batch.taskIds = ["resource-schema-contract", "codegen-preview-scaffold"];
    const codegenTask = graph.tasks.find((task) => task.id === "codegen-preview-scaffold");
    codegenTask.resourceLocks.push("admin-resource-contract");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch non-visual-contract-gates has resource lock conflict admin-resource-contract/);
  });

  it("rejects resource locks without policy definitions", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.resourceLockPolicies = graph.resourceLockPolicies.filter((policy) => policy.lock !== "admin-ui");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resourceLockPolicies must describe admin-ui/);
  });

  it("rejects same-batch tasks whose locks are in the same conflict group", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "non-visual-contract-gates");
    batch.taskIds = ["openapi-app-contracts", "cache-redis-invalidation"];
    const cacheTask = graph.tasks.find((task) => task.id === "cache-redis-invalidation");
    cacheTask.resourceLocks = ["codegen"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch non-visual-contract-gates has resource lock group conflict contract-generation-surface/);
  });

  it("rejects same-batch tasks with dependency paths", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "non-visual-contract-gates");
    batch.taskIds = ["resource-schema-contract", "openapi-app-contracts"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch non-visual-contract-gates contains dependent tasks openapi-app-contracts and resource-schema-contract/);
  });

  it("rejects deferred tasks in active parallel batches", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const batch = graph.parallelBatches.find((item) => item.id === "service-contract-extension-lanes");
    batch.taskIds[1] = "multi-datasource-contract-and-runtime";
    const graphPath = tempJSON("deferred-parallel-batch.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /parallel batch service-contract-extension-lanes must not schedule deferred task multi-datasource-contract-and-runtime/);
  });

  it("rejects missing screenshot evidence paths", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "visual-product-design-qa");
    visualTask.evidence.screenshots[0] = "tmp/product-design/missing-screenshot.png";
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task visual-product-design-qa evidence path is missing or unsafe: tmp\/product-design\/missing-screenshot\.png/);
  });

  it("accepts portable external screenshot evidence URIs", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "visual-product-design-qa");
    visualTask.evidence.screenshots[0] = "external-review-artifacts://platform-go/visual-product-design-qa/2026-07-06/evidence.png";
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects visual tasks that skip the product design gate", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "admin-ui-shell-and-list-components");
    visualTask.designGate = ["superpowers:brainstorming"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visual task admin-ui-shell-and-list-components must require product-design/);
  });

  it("rejects visual tasks that declare unsupported design gates", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "admin-ui-shell-and-list-components");
    visualTask.designGate = ["superpowers:brainstorming", "product-design", "ad-hoc-design-review"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visual task admin-ui-shell-and-list-components has unsupported design gate ad-hoc-design-review/);
  });

  it("rejects implemented visual tasks without screenshot evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "admin-ui-shell-and-list-components");
    visualTask.evidence.screenshots = [];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /visual task admin-ui-shell-and-list-components with status implemented must declare screenshot evidence/);
  });

  it("rejects the admin UI componentization task without its drift test", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const visualTask = graph.tasks.find((task) => task.id === "admin-ui-shell-and-list-components");
    visualTask.evidence.tests = [];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-ui-shell-and-list-components evidence\.tests must include scripts\/admin-ui-contracts\.test\.mjs/);
  });

  it("rejects stack drift away from the approved route", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.approvedStack.backend = ["Gin", "GORM", "JWT"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /approvedStack.backend must stay Gin \+ GORM \+ Casbin \+ JWT/);
  });

  it("rejects planned tasks without status rationale and completion gates", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const plannedTask = graph.tasks.find((task) => task.id === "codegen-preview-scaffold");
    plannedTask.status = "planned";
    delete plannedTask.statusReason;
    delete plannedTask.completionGate;
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task codegen-preview-scaffold with status planned must declare zh\/en statusReason/);
    assert.match(result.stderr, /task codegen-preview-scaffold with status planned must declare zh\/en completionGate/);
  });

  it("rejects implemented promotion-gate tasks without rationale, completion gates and docs", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const previewTask = graph.tasks.find((task) => task.id === "production-auth-provider-hardening");
    delete previewTask.statusReason;
    delete previewTask.completionGate;
    previewTask.evidence.docs = [];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task production-auth-provider-hardening must declare zh\/en statusReason/);
    assert.match(result.stderr, /task production-auth-provider-hardening must declare zh\/en completionGate/);
    assert.match(result.stderr, /task production-auth-provider-hardening must declare at least one evidence\.docs path/);
  });

  it("enforces the v0.1 release blocker and post-release optional partition", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const menuTask = graph.tasks.find((item) => item.id === "menu-tree-and-button-permission-configuration");
    const publicationTask = graph.tasks.find((item) => item.id === "github-release-publication");

    assert.deepEqual(graph.releaseBlockingNodes, releaseBlockingNodes);
    assert.deepEqual(graph.postReleaseOptionalNodes, postReleaseOptionalNodes);
    assert.equal(menuTask?.status, "implemented");
    assert.doesNotMatch(menuTask?.statusReason?.zh ?? "", /尚未区分|未实现/);
    assert.doesNotMatch(menuTask?.statusReason?.en ?? "", /do not distinguish|not implemented/i);
    assert.deepEqual(menuTask?.implementationBoundary, {
      implementedScope: ["directory-page-menu", "route-metadata", "page-buttons", "role-menu-assignment-contract"],
      closedGates: ["target-menu-serving", "role-menu-migration-writes", "all-principal-dual-read", "cutover-rollback"],
      ownerTask: "organization-rbac-menu-e2e-qa",
    });
    assert.equal(graph.tasks.find((item) => item.id === "unified-error-code-governance")?.status, "implemented");
    assert.ok(postReleaseOptionalNodes.every((id) => graph.tasks.find((item) => item.id === id)?.status === "deferred"));
    assert.equal(publicationTask?.status, "implemented");
    assert.match(publicationTask?.statusReason?.zh ?? "", /已发布并验证/);
    assert.match(publicationTask?.statusReason?.en ?? "", /published and verified/i);
    assert.deepEqual(publicationTask?.evidence?.artifacts, ["resources/evidence/github-release-publication-20260716.json"]);
    const publicationEvidence = readJSON("resources/evidence/github-release-publication-20260716.json");
    assert.equal(publicationEvidence.status, "verified");
    assert.equal(publicationEvidence.release, "https://github.com/GnosiST/platform-go/releases/tag/v0.1.0");
    for (const field of ["tagCommit", "releaseCommit", "ciHeadSha", "pagesHeadSha"]) {
      assert.equal(publicationEvidence[field], "5821d10ced3ba438e814201ef0ca32cda096c941");
    }
    assert.deepEqual(graph.tasks.find((item) => item.id === "open-source-portability")?.dependsOn, [
      "admin-watermark-export-governance",
      "organization-rbac-menu-e2e-qa",
      "unified-error-code-governance",
    ]);
  });

  it("rejects unknown, duplicate, overlapping or incomplete release lanes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const cases = [
      ["unknown", (value) => value.releaseBlockingNodes.push("missing-task"), /releaseBlockingNodes references unknown task missing-task/],
      ["duplicate", (value) => value.releaseBlockingNodes.push(value.releaseBlockingNodes[0]), /releaseBlockingNodes contains duplicate value/],
      ["overlap", (value) => value.releaseBlockingNodes.push(value.postReleaseOptionalNodes[0]), /release lanes overlap at/],
      ["omitted", (value) => value.releaseBlockingNodes.shift(), /releaseBlockingNodes must include github-release-publication/],
    ];

    for (const [name, mutate, expected] of cases) {
      const value = structuredClone(graph);
      mutate(value);
      const result = runValidator(["--graph", tempJSON(`${name}-release-lane.json`, value)]);
      assert.notEqual(result.status, 0, `${name}: ${result.stdout}`);
      assert.match(result.stderr, expected);
    }
  });

  it("rejects invalid deferred status and release dependencies", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");

    const optionalPending = structuredClone(graph);
    optionalPending.tasks.find((item) => item.id === postReleaseOptionalNodes[0]).status = "pending";
    let result = runValidator(["--graph", tempJSON("optional-pending.json", optionalPending)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /post-release optional task multi-datasource-contract-and-runtime must be deferred/);

    const blockerDeferred = structuredClone(graph);
    blockerDeferred.tasks.find((item) => item.id === releaseBlockingNodes[0]).status = "deferred";
    result = runValidator(["--graph", tempJSON("blocker-deferred.json", blockerDeferred)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /release blocker github-release-publication must not be deferred/);

    const transitive = structuredClone(graph);
    transitive.tasks.find((item) => item.id === "open-source-portability").dependsOn.push("multi-datasource-contract-and-runtime");
    result = runValidator(["--graph", tempJSON("blocker-depends-on-optional.json", transitive)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /release blocker github-release-publication must not depend on post-release optional task multi-datasource-contract-and-runtime/);
  });

  it("rejects publication evidence whose release source SHAs are missing or inconsistent", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const publicationTask = graph.tasks.find((item) => item.id === "github-release-publication");
    publicationTask.status = "implemented";
    publicationTask.evidence.artifacts = ["resources/evidence/github-release-publication-20260716.json"];
    const graphPath = tempJSON("implemented-publication-graph.json", graph);
    const evidence = readJSON("resources/evidence/github-release-publication-20260716.json");
    const releaseCommit = "a".repeat(40);
    Object.assign(evidence, {
      status: "verified",
      tagCommit: releaseCommit,
      releaseCommit,
      ciHeadSha: releaseCommit,
      pagesHeadSha: "b".repeat(40),
    });

    const mismatch = runValidator(["--graph", graphPath, "--release-evidence", tempJSON("release-evidence-mismatch.json", evidence)]);
    assert.notEqual(mismatch.status, 0, mismatch.stdout);
    assert.match(mismatch.stderr, /publication evidence source SHAs must all equal releaseCommit/);

    delete evidence.tagCommit;
    const missing = runValidator(["--graph", graphPath, "--release-evidence", tempJSON("release-evidence-missing.json", evidence)]);
    assert.notEqual(missing.status, 0, missing.stdout);
    assert.match(missing.stderr, /publication evidence tagCommit must be a full lowercase Git commit SHA/);
  });

  it("accepts publication evidence submitted after release without requiring its own commit SHA", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const publicationTask = graph.tasks.find((item) => item.id === "github-release-publication");
    publicationTask.status = "implemented";
    publicationTask.evidence.artifacts = ["resources/evidence/github-release-publication-20260716.json"];
    const evidence = readJSON("resources/evidence/github-release-publication-20260716.json");
    const releaseCommit = "a".repeat(40);
    Object.assign(evidence, {
      status: "verified",
      tagCommit: releaseCommit,
      releaseCommit,
      ciHeadSha: releaseCommit,
      pagesHeadSha: releaseCommit,
    });

    const result = runValidator([
      "--graph",
      tempJSON("implemented-publication-graph.json", graph),
      "--release-evidence",
      tempJSON("post-release-evidence.json", evidence),
    ]);
    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects stale unimplemented rationale on the implemented menu node", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const menuTask = graph.tasks.find((item) => item.id === "menu-tree-and-button-permission-configuration");
    menuTask.statusReason = {
      zh: "菜单尚未区分目录与页面，相关合同未实现。",
      en: "Directory and page menus are not implemented.",
    };

    const result = runValidator(["--graph", tempJSON("stale-menu-status-reason.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menu-tree-and-button-permission-configuration statusReason contradicts its implemented state/);
  });

  it("rejects menu closeout without a structured implementation boundary", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const menuTask = graph.tasks.find((item) => item.id === "menu-tree-and-button-permission-configuration");
    delete menuTask.implementationBoundary;

    const result = runValidator(["--graph", tempJSON("missing-menu-implementation-boundary.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /menu-tree-and-button-permission-configuration implementationBoundary must preserve implemented scope, closed gates and owner task/);
  });

  it("preserves the closed 37-node baseline, implements 21 completion nodes, and tracks nine controlled unfinished nodes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "production-admin-oidc-auth");
    const implemented = graph.tasks.filter((item) => item.status === "implemented");
    const pending = graph.tasks.filter((item) => item.status === "pending");
    const deferred = graph.tasks.filter((item) => item.status === "deferred");
    const blocked = graph.tasks.filter((item) => item.status === "blocked");

    assert.ok(task, "task graph must include production-admin-oidc-auth");
    assert.equal(task.status, "implemented");
    assert.equal(task.visual, true);
    assert.deepEqual(task.dependsOn, [
      "production-auth-provider-hardening",
      "production-persistence-correctness",
      "admin-ui-system-quality-hardening",
    ]);
    assert.deepEqual(task.designGate, ["superpowers:brainstorming", "product-design"]);
    assert.deepEqual(task.evidence.screenshots, ["resources/evidence/production-admin-oidc-auth-20260711.json"]);
    assert.deepEqual(
      task.completionEvidence.map((item) => item.id),
      ["production-like-oidc-rehearsal", "six-viewport-browser-acceptance", "neat-freak-cleanup-closeout"],
    );
    assert.deepEqual(task.completionEvidence[1].viewports, [
      "375x812",
      "390x844",
      "768x1024",
      "1024x768",
      "1280x720",
      "1440x1024",
    ]);
    assert.ok(task.completionEvidence.every((item) => item.status === "verified"));
    assert.deepEqual(graph.tasks.slice(0, foundationBaselineTaskIDs.length).map((item) => item.id), foundationBaselineTaskIDs);
    assert.ok(graph.tasks.slice(0, foundationBaselineTaskIDs.length).every((item) => item.status === "implemented"));
    assert.equal(graph.tasks.length, 67);
    assert.equal(implemented.length, 58);
    assert.equal(graph.tasks.find((item) => item.id === "runtime-security-containment")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "admin-watermark-export-governance")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "sensitive-data-protection-runtime")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "sensitive-data-historical-migration")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "mask-strategy-runtime")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "sensitive-data-reveal-step-up")?.status, "implemented");
    assert.equal(graph.tasks.find((item) => item.id === "data-lifecycle-retention")?.status, "implemented");
    const serviceContract = graph.tasks.find((item) => item.id === "platform-service-contract-standard");
    assert.equal(serviceContract?.status, "implemented");
    assert.deepEqual(serviceContract?.dependsOn, ["data-lifecycle-retention", "capability-contract-governance"]);
    assert.ok(serviceContract?.evidence?.validators?.includes("scripts/validate-platform-service-contract-standard.mjs"));
    const serviceObjects = graph.tasks.find((item) => item.id === "persisted-query-command-object-runtime");
    assert.equal(serviceObjects?.status, "implemented");
    assert.deepEqual(serviceObjects?.dependsOn, ["platform-service-contract-standard", "admin-api-boundary-query-security"]);
    assert.ok(serviceObjects?.evidence?.validators?.includes("scripts/validate-platform-service-object-runtime.mjs"));
    const menuTask = graph.tasks.find((item) => item.id === "menu-tree-and-button-permission-configuration");
    assert.equal(menuTask?.status, "implemented");
    assert.deepEqual(menuTask?.evidence?.screenshots, ["resources/evidence/menu-tree-and-button-permission-configuration-20260715.json"]);
    assert.equal(graph.tasks.find((item) => item.id === "unified-error-code-governance")?.status, "implemented");
    const integrationPorts = graph.tasks.find((item) => item.id === "integration-ports-disabled-default");
    assert.equal(integrationPorts?.status, "implemented");
    assert.ok(integrationPorts?.evidence?.validators?.includes("scripts/validate-platform-integration-ports.mjs"));
    const organizationContract = graph.tasks.find((item) => item.id === "organization-rbac-menu-contract-and-migration-design");
    assert.equal(organizationContract?.status, "implemented");
    assert.equal(organizationContract?.contractGateOnly, true);
    assert.ok(organizationContract?.evidence?.validators?.includes("scripts/validate-platform-organization-rbac-menu-contract.mjs"));
    const organizationBackend = graph.tasks.find((item) => item.id === "organization-role-pool-backend-and-migration");
    assert.equal(organizationBackend?.status, "implemented");
    assert.ok(organizationBackend?.evidence?.tests?.includes("internal/platform/organizationrbac/migration_test.go"));
    assert.ok(
      graph.tasks
        .find((item) => item.id === "organization-role-pool-backend-and-migration")
        ?.resourceLocks?.includes("query-command-contract"),
    );
    const organizationAdmin = graph.tasks.find((item) => item.id === "organization-user-admin-experience");
    assert.equal(organizationAdmin?.status, "implemented");
    assert.ok(organizationAdmin?.evidence?.validators?.includes("scripts/validate-admin-ui-contracts.mjs"));
    assert.deepEqual(organizationAdmin?.evidence?.screenshots, ["resources/evidence/organization-user-admin-experience-20260715.json"]);
    assert.ok(organizationAdmin?.evidence?.skills?.includes("ui-ux-pro-max"));
    const roleTree = graph.tasks.find((item) => item.id === "role-tree-and-authorization-entry");
    assert.equal(roleTree?.status, "implemented");
    assert.ok(roleTree?.evidence?.validators?.includes("scripts/validate-admin-ui-contracts.mjs"));
    assert.deepEqual(roleTree?.evidence?.screenshots, ["resources/evidence/role-tree-and-authorization-entry-20260715.json"]);
    assert.ok(roleTree?.evidence?.skills?.includes("ui-ux-pro-max"));
    assert.deepEqual(graph.tasks.find((item) => item.id === "integration-ports-disabled-default")?.dependsOn, [
      "platform-service-contract-standard",
      "notification-extension-boundary",
      "job-extension-boundary",
    ]);
    assert.deepEqual(graph.tasks.find((item) => item.id === "tenant-placement-and-request-routing")?.dependsOn, [
      "multi-datasource-contract-and-runtime",
      "organization-role-pool-backend-and-migration",
    ]);
    assert.deepEqual(graph.tasks.find((item) => item.id === "federated-read-query")?.dependsOn, [
      "sharding-and-tenant-migration",
      "persisted-query-command-object-runtime",
    ]);
    assert.deepEqual(graph.tasks.find((item) => item.id === "transactional-outbox-and-one-mq-adapter")?.dependsOn, [
      "integration-ports-disabled-default",
      "database-certification-matrix",
    ]);
    assert.deepEqual(graph.tasks.find((item) => item.id === "open-source-portability")?.dependsOn, [
      "admin-watermark-export-governance",
      "organization-rbac-menu-e2e-qa",
      "unified-error-code-governance",
    ]);
    const organizationE2ETask = graph.tasks.find((item) => item.id === "organization-rbac-menu-e2e-qa");
    for (const lock of ["service-contract", "query-command-contract", "admin-resource-api", "openapi", "codegen", "audit-policy", "docs"]) {
      assert.ok(organizationE2ETask?.resourceLocks.includes(lock), `organization-rbac-menu-e2e-qa must declare real shared lock ${lock}`);
    }
    assert.deepEqual(graph.parallelBatches.find((item) => item.id === "service-contract-extension-lanes")?.taskIds, [
      "persisted-query-command-object-runtime",
      "integration-ports-disabled-default",
    ]);
    assert.ok(
      graph.parallelBatches.every((batch) => batch.taskIds.every((id) => graph.tasks.find((item) => item.id === id)?.status !== "deferred")),
      "active parallel batches must not schedule deferred tasks",
    );
    assert.equal(graph.parallelBatches.find((item) => item.id === "release-blocker-contract-lanes"), undefined);
    assert.deepEqual(pending.map((item) => item.id), releaseBlockingNodes.filter((id) => graph.tasks.find((task) => task.id === id)?.status !== "implemented"));
    assert.deepEqual(deferred.map((item) => item.id), postReleaseOptionalNodes);
    assert.equal(blocked.length, 0);
  });

  it("rejects organization backend migration without the query command contract lock", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "organization-role-pool-backend-and-migration");
    task.resourceLocks = task.resourceLocks.filter((lock) => lock !== "query-command-contract");
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /organization-role-pool-backend-and-migration resourceLocks must include query-command-contract/);
  });

  it("rejects regressing mask strategy runtime after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((item) => item.id === "mask-strategy-runtime").status = "pending";
    const graphPath = tempJSON("pending-mask-strategy-runtime.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /approved implemented task mask-strategy-runtime must stay implemented/);
  });

  it("rejects regressing persisted query runtime after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((item) => item.id === "persisted-query-command-object-runtime").status = "pending";
    const graphPath = tempJSON("pending-persisted-query-runtime.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /persisted-query-command-object-runtime must stay implemented after closeout/);
  });

  it("rejects regressing sensitive data protection after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((item) => item.id === "sensitive-data-protection-runtime").status = "pending";
    const graphPath = tempJSON("pending-sensitive-data-protection.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sensitive-data-protection-runtime must stay implemented after closeout/);
  });

  it("rejects regressing sensitive data historical migration after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((item) => item.id === "sensitive-data-historical-migration").status = "pending";
    const graphPath = tempJSON("pending-sensitive-data-historical-migration.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sensitive-data-historical-migration must stay implemented after closeout/);
  });

  it("rejects watermark closeout without UI UX and browser evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "admin-watermark-export-governance");
    assert.equal(task.status, "implemented");

    task.evidence.skills = task.evidence.skills.filter((skill) => skill !== "ui-ux-pro-max");
    task.evidence.screenshots = [];
    const result = runValidator(["--graph", tempJSON("missing-watermark-design-evidence.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-watermark-export-governance evidence\.skills must include ui-ux-pro-max/);
    assert.match(result.stderr, /visual task admin-watermark-export-governance with status implemented must declare screenshot evidence/);
  });

  it("requires tracked evidence manifests for current watermark, sensitive reveal, organization Admin and role Admin visual nodes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const watermarkTask = graph.tasks.find((item) => item.id === "admin-watermark-export-governance");
    const revealTask = graph.tasks.find((item) => item.id === "sensitive-data-reveal-step-up");
    const organizationTask = graph.tasks.find((item) => item.id === "organization-user-admin-experience");
    const roleTask = graph.tasks.find((item) => item.id === "role-tree-and-authorization-entry");

    assert.deepEqual(watermarkTask.evidence.screenshots, ["resources/evidence/admin-watermark-export-governance-20260713.json"]);
    assert.deepEqual(revealTask.evidence.screenshots, ["resources/evidence/sensitive-data-reveal-step-up-20260713.json"]);
    assert.deepEqual(organizationTask.evidence.screenshots, ["resources/evidence/organization-user-admin-experience-20260715.json"]);
    assert.deepEqual(roleTask.evidence.screenshots, ["resources/evidence/role-tree-and-authorization-entry-20260715.json"]);

    watermarkTask.evidence.screenshots = ["docs/platform-roadmap.md"];
    revealTask.evidence.screenshots = ["docs/platform-roadmap.md"];
    organizationTask.evidence.screenshots = ["docs/platform-roadmap.md"];
    roleTask.evidence.screenshots = ["docs/platform-roadmap.md"];
    const result = runValidator(["--graph", tempJSON("ignored-current-visual-evidence.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin-watermark-export-governance evidence\.screenshots must include resources\/evidence\/admin-watermark-export-governance-20260713\.json/);
    assert.match(result.stderr, /sensitive-data-reveal-step-up evidence\.screenshots must include resources\/evidence\/sensitive-data-reveal-step-up-20260713\.json/);
    assert.match(result.stderr, /organization-user-admin-experience evidence\.screenshots must include resources\/evidence\/organization-user-admin-experience-20260715\.json/);
    assert.match(result.stderr, /role-tree-and-authorization-entry evidence\.screenshots must include resources\/evidence\/role-tree-and-authorization-entry-20260715\.json/);
  });

  it("rejects organization Admin closeout without UI UX and browser evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "organization-user-admin-experience");
    task.evidence.skills = [];
    task.evidence.screenshots = [];

    const result = runValidator(["--graph", tempJSON("missing-organization-admin-evidence.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /organization-user-admin-experience evidence\.skills must include ui-ux-pro-max/);
    assert.match(result.stderr, /organization-user-admin-experience evidence\.screenshots must include resources\/evidence\/organization-user-admin-experience-20260715\.json/);
  });

  it("rejects role Admin closeout without UI UX and browser evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "role-tree-and-authorization-entry");
    task.evidence.skills = [];
    task.evidence.screenshots = [];

    const result = runValidator(["--graph", tempJSON("missing-role-admin-evidence.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /role-tree-and-authorization-entry evidence\.skills must include ui-ux-pro-max/);
    assert.match(result.stderr, /role-tree-and-authorization-entry evidence\.screenshots must include resources\/evidence\/role-tree-and-authorization-entry-20260715\.json/);
  });

  it("rejects missing or reordered completion program task IDs", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const programTasks = graph.tasks.filter((task) => completionProgramTaskIDs.includes(task.id));
    assert.equal(programTasks.length, completionProgramTaskIDs.length, "completion program nodes must exist before mutation validation");

    const missingGraph = structuredClone(graph);
    missingGraph.tasks = missingGraph.tasks.filter((task) => task.id !== completionProgramTaskIDs[0]);
    const missingResult = runValidator(["--graph", tempJSON("missing-completion-task.json", missingGraph)]);
    assert.notEqual(missingResult.status, 0, missingResult.stdout);
    assert.match(missingResult.stderr, /approved completion program task is missing: runtime-security-containment/);

    const reorderedGraph = structuredClone(graph);
    const indexes = completionProgramTaskIDs.slice(0, 2).map((id) => reorderedGraph.tasks.findIndex((task) => task.id === id));
    [reorderedGraph.tasks[indexes[0]], reorderedGraph.tasks[indexes[1]]] = [reorderedGraph.tasks[indexes[1]], reorderedGraph.tasks[indexes[0]]];
    const reorderedResult = runValidator(["--graph", tempJSON("reordered-completion-task.json", reorderedGraph)]);
    assert.notEqual(reorderedResult.status, 0, reorderedResult.stdout);
    assert.match(reorderedResult.stderr, /completion program task order must match approved order/);
  });

  it("rejects production Admin OIDC evidence manifests without a completed redaction scan", () => {
    const evidence = readJSON("resources/evidence/production-admin-oidc-auth-20260711.json");
    evidence.redaction.scanPassed = false;
    const evidencePath = tempJSON("production-admin-oidc-auth-20260711.json", evidence);

    const result = runValidator(["--oidc-evidence", evidencePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-admin-oidc-auth evidence redaction scan must pass/);
  });

  it("tracks completed platform foundation work and promoted visual task nodes", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const closedTasks = [
      "production-auth-provider-hardening",
      "source-writing-codegen-promotion",
    ];

    for (const taskID of closedTasks) {
      const task = graph.tasks.find((item) => item.id === taskID);
      assert.ok(task, `expected ${taskID} in task graph`);
      assert.equal(task.status, "implemented");
      assert.ok(task.statusReason?.zh && task.statusReason?.en, `${taskID} must declare statusReason`);
      assert.ok(task.completionGate?.zh && task.completionGate?.en, `${taskID} must declare completionGate`);
      assert.ok(task.evidence?.docs?.length > 0, `${taskID} must cite docs`);
    }

    const implementedFormSlotTask = graph.tasks.find((item) => item.id === "form-schema-layout-and-slots");
    assert.equal(implementedFormSlotTask?.status, "implemented");
    assert.equal(implementedFormSlotTask?.contractGateOnly, false);
    assert.ok(implementedFormSlotTask.evidence?.screenshots?.length >= 4, "form layout slots must keep dense and side-preview browser screenshot evidence");
    assert.ok(implementedFormSlotTask.evidence?.validators?.includes("scripts/validate-platform-form-schema-layout-slots.mjs"));
    assert.ok(implementedFormSlotTask.evidence?.validators?.includes("scripts/validate-admin-i18n.mjs"));
    assert.ok(implementedFormSlotTask.evidence?.validators?.includes("scripts/validate-admin-ui-contracts.mjs"));

    const implementedFileStorageTask = graph.tasks.find((item) => item.id === "file-storage-preview-and-audit-workflow");
    assert.equal(implementedFileStorageTask?.status, "implemented");
    assert.ok(implementedFileStorageTask.evidence?.screenshots?.length >= 4, "file-storage experience must keep browser screenshot evidence");

    const implementedPolicyReviewTask = graph.tasks.find((item) => item.id === "policy-review-custom-ui");
    assert.equal(implementedPolicyReviewTask?.status, "implemented");
    assert.ok(implementedPolicyReviewTask.evidence?.screenshots?.length >= 4, "policy-review custom UI must keep browser screenshot evidence");
    assert.ok(implementedPolicyReviewTask.evidence?.validators?.includes("scripts/validate-admin-i18n.mjs"));
    assert.ok(implementedPolicyReviewTask.evidence?.validators?.includes("scripts/validate-admin-ui-contracts.mjs"));
    assert.ok(implementedPolicyReviewTask.evidence?.checks?.includes("rtk npm --prefix admin run build"));

    assert.ok(!graph.tasks.some((item) => item.id === "reference-business-boundary-and-parity-gate"));
    assert.ok(!graph.tasks.some((item) => item.id === "external-business-boundary"));
  });

  it("keeps reference discovery as the predecessor of reference coverage", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const discoveryTask = graph.tasks.find((task) => task.id === "reference-discovery-classification-gate");
    const coverageTask = graph.tasks.find((task) => task.id === "reference-coverage-boundary-gate");

    assert.equal(discoveryTask.status, "implemented");
    assert.ok(discoveryTask.evidence.validators.includes("scripts/validate-platform-reference-discovery.mjs"));
    assert.ok(discoveryTask.evidence.tests.includes("scripts/platform-reference-discovery.test.mjs"));
    assert.ok(coverageTask.dependsOn.includes("reference-discovery-classification-gate"));
  });

  it("keeps policy review workflow implemented with backend and contract evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const policyReviewTask = graph.tasks.find((task) => task.id === "policy-review-and-audit-workflow");

    assert.equal(policyReviewTask.status, "implemented");
    assert.ok(policyReviewTask.evidence.docs.includes("docs/platform-foundation-task-map.md"));
    assert.ok(policyReviewTask.evidence.docs.includes("docs/platform-capability-development.md"));
    assert.ok(policyReviewTask.evidence.tests.includes("internal/platform/adminresource/policy_review_test.go"));
    assert.ok(policyReviewTask.evidence.tests.includes("internal/platform/httpapi/server_test.go"));
    assert.ok(policyReviewTask.evidence.tests.includes("scripts/admin-resource-contract-generators.test.mjs"));
  });

  it("keeps codegen scaffold implemented without enabling source writing", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const scaffoldTask = graph.tasks.find((task) => task.id === "codegen-preview-scaffold");
    const readinessTask = graph.tasks.find((task) => task.id === "codegen-source-writing-readiness");
    const promotionTask = graph.tasks.find((task) => task.id === "source-writing-codegen-promotion");

    assert.equal(scaffoldTask.status, "implemented");
    assert.equal(readinessTask.status, "implemented");
    assert.equal(promotionTask.status, "implemented");
    assert.ok(scaffoldTask.evidence.validators.includes("scripts/generate-admin-scaffold-files.mjs"));
    assert.ok(scaffoldTask.evidence.validators.includes("scripts/generate-admin-scaffold-promotion-review.mjs"));
    assert.ok(scaffoldTask.evidence.tests.includes("scripts/admin-scaffold-plan.test.mjs"));
    assert.ok(scaffoldTask.evidence.docs.includes("docs/platform-roadmap.md"));
  });

  it("rejects implemented policy review tasks without any evidence", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const policyReviewTask = graph.tasks.find((task) => task.id === "policy-review-and-audit-workflow");
    policyReviewTask.evidence = {};
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /task policy-review-and-audit-workflow must declare evidence paths/);
  });
});
