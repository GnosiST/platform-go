import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

const completionProgramCapabilityIDs = [
  "runtime-security-containment",
  "admin-watermark-export-governance",
  "sensitive-data-protection",
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
  "public-documentation-and-release",
];

const partialCapabilityDependencies = {
  "persisted-query-command-object-runtime": ["platform-service-contract-standard"],
  "integration-ports-disabled-default": ["platform-service-contract-standard"],
  "organization-rbac-menu-contract-and-migration-design": ["persisted-query-command-object-runtime"],
  "organization-role-pool-backend-and-migration": ["organization-rbac-menu-contract-and-migration-design"],
  "organization-user-admin-experience": ["organization-role-pool-backend-and-migration"],
  "role-tree-and-authorization-entry": ["organization-user-admin-experience"],
  "menu-tree-and-button-permission-configuration": ["role-tree-and-authorization-entry"],
  "organization-rbac-menu-e2e-qa": [
    "organization-user-admin-experience",
    "role-tree-and-authorization-entry",
    "menu-tree-and-button-permission-configuration",
  ],
  "multi-datasource-contract-and-runtime": ["platform-service-contract-standard"],
  "tenant-placement-and-request-routing": [
    "multi-datasource-contract-and-runtime",
    "organization-role-pool-backend-and-migration",
  ],
  "datasource-read-write-routing": ["tenant-placement-and-request-routing"],
  "sharding-and-tenant-migration": ["datasource-read-write-routing"],
  "federated-read-query": ["sharding-and-tenant-migration", "persisted-query-command-object-runtime"],
  "xa-optional-adapter": ["federated-read-query"],
  "database-certification-matrix": ["xa-optional-adapter"],
  "transactional-outbox-and-one-mq-adapter": [
    "integration-ports-disabled-default",
    "database-certification-matrix",
  ],
  "asynchronous-search-projection": [
    "transactional-outbox-and-one-mq-adapter",
    "persisted-query-command-object-runtime",
  ],
  "open-source-portability": [
    "runtime-security-containment",
    "admin-watermark-export-governance",
    "sensitive-data-protection",
    "organization-rbac-menu-e2e-qa",
    "asynchronous-search-projection",
  ],
  "public-documentation-and-release": ["open-source-portability"],
};
const governedCapabilityDocs = [
  "docs/superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md",
  "docs/platform-data-governance-and-integrations-assessment.md",
  "docs/platform-roadmap.md",
];
const openSourceCapabilityDocs = [
  "docs/superpowers/specs/2026-07-12-open-source-docs-site-design.md",
  "docs/superpowers/plans/2026-07-12-platform-completion-task-graph.md",
  "docs/superpowers/specs/2026-07-14-platform-remaining-task-topology-adjustment.md",
];

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-engineering-capabilities.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-engineering-capabilities-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-engineering-capabilities-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
  return filePath;
}

const stackSourcePaths = [
  "internal/platform/httpapi/server.go",
  "internal/platform/adminresource/gorm_store.go",
  "internal/platform/authz/casbin.go",
  "internal/platform/adminresource/authorization.go",
  "admin/src/App.tsx",
  "admin/src/platform/refine/dataProvider.ts",
  "admin/src/platform/refine/accessControlProvider.ts",
];

function tempStackSourceRoot(mutator) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-stack-sources-"));
  for (const relativePath of stackSourcePaths) {
    const source = path.join(repoRoot, relativePath);
    const target = path.join(tempDir, relativePath);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(source, target);
  }
  mutator(tempDir);
  return tempDir;
}

describe("validate-platform-engineering-capabilities", () => {
  it("tracks runtime security and watermark governance as implemented", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capabilities = matrix.capabilities.filter((item) => completionProgramCapabilityIDs.includes(item.id));

    assert.equal(matrix.capabilities.length, 57);
    assert.deepEqual(capabilities.map((item) => item.id), completionProgramCapabilityIDs);
    for (const capability of capabilities.slice(0, 7)) {
      assert.equal(capability.status, "implemented");
      assert.ok(capability.evidence.sourcePaths.length > 0);
      assert.ok(capability.evidence.tests.length > 0);
      assert.ok(capability.evidence.validators.length > 0);
    }
    assert.ok(capabilities.slice(7).every((item) => item.status === "partial"));

    for (const [capabilityID, expectedDependencies] of Object.entries(partialCapabilityDependencies)) {
      const capability = capabilities.find((item) => item.id === capabilityID);
      assert.equal(capability.status, "partial");
      assert.deepEqual(capability.dependsOn, expectedDependencies);
      assert.deepEqual(
        capability.evidence.sourcePaths,
        ["open-source-portability", "public-documentation-and-release"].includes(capabilityID)
          ? openSourceCapabilityDocs
          : governedCapabilityDocs,
      );
    }

    for (const capability of capabilities) {
      if (!["sensitive-data-protection", "public-documentation-and-release"].includes(capability.id)) {
        assert.deepEqual(capability.evidence.taskIds, [capability.id]);
      }
    }

    const publication = capabilities.find((item) => item.id === "public-documentation-and-release");
    assert.deepEqual(publication.evidence.taskIds, [
      "public-docs-community",
      "public-docs-site",
      "github-release-publication",
    ]);

    const reveal = capabilities.find((item) => item.id === "sensitive-data-reveal-step-up");
    assert.ok(reveal.evidence.sourcePaths.includes("internal/platform/httpapi/sensitive_reveal.go"));
    assert.ok(reveal.evidence.sourcePaths.includes("admin/src/platform/resources/SensitiveFieldRevealModal.tsx"));
    assert.ok(reveal.evidence.tests.includes("internal/platform/httpapi/sensitive_reveal_test.go"));
    assert.ok(reveal.evidence.validators.includes("scripts/validate-admin-ui-contracts.mjs"));

    const serviceContract = capabilities.find((item) => item.id === "platform-service-contract-standard");
    assert.equal(serviceContract.status, "implemented");
    assert.deepEqual(serviceContract.dependsOn, ["data-lifecycle-retention", "capability-contract-governance"]);
    assert.ok(serviceContract.evidence.sourcePaths.includes("resources/platform-service-contract-standard.json"));
    assert.ok(serviceContract.evidence.generatedFiles.includes("resources/generated/asyncapi.events.json"));
    assert.ok(serviceContract.evidence.tests.includes("scripts/platform-service-contract-standard.test.mjs"));
  });

  it("rejects regressing the platform service contract after closeout", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "platform-service-contract-standard");
    capability.status = "partial";
    capability.dependsOn = ["data-lifecycle-retention"];

    const result = runValidator(["--matrix", tempJSON("partial-platform-service-contract.json", matrix)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required implemented capability platform-service-contract-standard must stay implemented/);
    assert.match(result.stderr, /platform-service-contract-standard dependsOn must equal/);
  });

  it("rejects regressing watermark governance to partial after closeout", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities.find((item) => item.id === "admin-watermark-export-governance").status = "partial";

    const result = runValidator(["--matrix", tempJSON("partial-watermark-capability.json", matrix)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required implemented capability admin-watermark-export-governance must stay implemented/);
  });

  it("rejects regressing runtime security capability to partial after closeout", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities.find((item) => item.id === "runtime-security-containment").status = "partial";

    const result = runValidator(["--matrix", tempJSON("partial-runtime-security-capability.json", matrix)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required implemented capability runtime-security-containment must stay implemented/);
  });

  it("rejects regressing mask strategy runtime to partial after closeout", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities.find((item) => item.id === "mask-strategy-runtime").status = "partial";

    const result = runValidator(["--matrix", tempJSON("partial-mask-strategy-runtime.json", matrix)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required implemented capability mask-strategy-runtime must stay implemented/);
  });

  it("rejects regressing sensitive reveal step-up to partial after implementation", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities.find((item) => item.id === "sensitive-data-reveal-step-up").status = "partial";

    const result = runValidator(["--matrix", tempJSON("partial-sensitive-reveal-step-up.json", matrix)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required implemented capability sensitive-data-reveal-step-up must stay implemented/);
  });

  it("keeps sensitive data migration evidence implemented after closeout", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "sensitive-data-protection");

    assert.equal(capability.status, "implemented");
    assert.ok(capability.evidence.sourcePaths.includes("docs/platform-sensitive-data-migration.md"));
    assert.ok(capability.evidence.tests.includes("scripts/platform-sensitive-data-migration.test.mjs"));
    assert.ok(capability.evidence.validators.includes("scripts/validate-platform-sensitive-data-migration.mjs"));
  });

  it("rejects dropping an approved completion program capability", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((item) => item.id !== "platform-service-contract-standard");

    const result = runValidator(["--matrix", tempJSON("missing-completion-capability.json", matrix)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability platform-service-contract-standard/);
  });

  it("rejects changing the approved completion program dependency topology", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities.find((item) => item.id === "federated-read-query").dependsOn = ["sharding-and-tenant-migration"];

    const result = runValidator(["--matrix", tempJSON("invalid-completion-dependencies.json", matrix)]);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /approved completion program capability federated-read-query dependsOn must equal/);
  });

  it("accepts current engineering capability coverage", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform engineering capabilities/);
  });

  it("rejects missing evidence paths", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities[0].evidence.sourcePaths.push("missing/not-real.ts");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /evidence path is missing or unsafe: missing\/not-real\.ts/);
  });

  it("rejects missing test evidence paths", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const referenceDiscovery = matrix.capabilities.find((capability) => capability.id === "reference-discovery-gate");
    referenceDiscovery.evidence.tests.push("missing/not-real.test.go");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /evidence path is missing or unsafe: missing\/not-real\.test\.go/);
  });

  it("rejects resources missing from the admin contract", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities[0].evidence.adminResources.push("missing-resource");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing admin resource missing-resource/);
  });

  it("rejects required engineering capability gaps", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "production-runtime-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability production-runtime-gate/);
  });

  it("rejects missing task dependency governance", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "task-dependency-governance");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability task-dependency-governance/);
  });

  it("rejects missing reference coverage boundary gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "reference-coverage-boundary-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability reference-coverage-boundary-gate/);
  });

  it("rejects missing reference discovery gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "reference-discovery-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability reference-discovery-gate/);
  });

  it("rejects missing personnel runtime readiness gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "personnel-runtime-readiness");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability personnel-runtime-readiness/);
  });

  it("rejects missing production readiness preflight gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "production-readiness-preflight");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability production-readiness-preflight/);
  });

  it("rejects missing deployment topology gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "deployment-topology-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability deployment-topology-gate/);
  });

  it("rejects missing app client API boundary gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "app-client-api-boundary");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability app-client-api-boundary/);
  });

  it("rejects missing goal completion audit gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "goal-completion-audit");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability goal-completion-audit/);
  });

  it("rejects missing node closeout audit gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "node-closeout-audit");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability node-closeout-audit/);
  });

  it("rejects missing production auth hardening gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "production-auth-hardening-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability production-auth-hardening-gate/);
  });

  it("keeps production Admin OIDC governance evidence in the production auth hardening gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "production-auth-hardening-gate");

    assert.ok(capability.evidence.sourcePaths.includes("docs/superpowers/specs/2026-07-11-production-admin-oidc-auth-design.md"));
    assert.ok(capability.evidence.sourcePaths.includes("resources/platform-foundation-task-graph.json"));
    assert.ok(capability.evidence.sourcePaths.includes("resources/evidence/production-admin-oidc-auth-20260711.json"));
    assert.ok(capability.evidence.tests.includes("internal/platform/authprovider/oidc/resolver_test.go"));
    assert.ok(capability.evidence.tests.includes("scripts/platform-foundation-task-graph.test.mjs"));
    assert.deepEqual(capability.completedEvidence, [
      "production-like-oidc-rehearsal",
      "six-viewport-browser-acceptance",
      "neat-freak-cleanup-closeout",
    ]);
  });

  it("rejects production auth hardening gates that drop completed OIDC evidence controls", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "production-auth-hardening-gate");
    capability.completedEvidence = [];
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-foundation-task-graph.json");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/evidence/production-admin-oidc-auth-20260711.json");
    capability.evidence.tests = capability.evidence.tests.filter((item) => item !== "internal/platform/authprovider/oidc/resolver_test.go");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-hardening-gate must cite resources\/platform-foundation-task-graph\.json/);
    assert.match(result.stderr, /production-auth-hardening-gate must cite the tracked production Admin OIDC evidence manifest/);
    assert.match(result.stderr, /production-auth-hardening-gate must cite internal\/platform\/authprovider\/oidc\/resolver_test\.go/);
    assert.match(result.stderr, /production-auth-hardening-gate completedEvidence must include production-like-oidc-rehearsal/);
  });

  it("rejects cache invalidation capabilities that do not cite the cache contract gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "runtime-cache-invalidation");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-cache-invalidation.json");
    capability.evidence.validators = [];
    capability.evidence.tests = [];
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtime-cache-invalidation must cite resources\/platform-cache-invalidation\.json/);
    assert.match(result.stderr, /runtime-cache-invalidation must cite validate-platform-cache-invalidation\.mjs/);
    assert.match(result.stderr, /runtime-cache-invalidation must cite platform-cache-invalidation\.test\.mjs/);
  });

  it("rejects deployment topology gates without deployment evidence", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "deployment-topology-gate");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-deployment-topology.json");
    capability.evidence.validators = [];
    capability.evidence.tests = [];
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deployment-topology-gate must cite resources\/platform-deployment-topology\.json/);
    assert.match(result.stderr, /deployment-topology-gate must cite validate-platform-deployment-topology\.mjs/);
    assert.match(result.stderr, /deployment-topology-gate must cite platform-deployment-topology\.test\.mjs/);
  });

  it("rejects app client API boundary gates without contract evidence", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "app-client-api-boundary");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-app-client-api-boundary.json");
    capability.evidence.validators = [];
    capability.evidence.tests = [];
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /app-client-api-boundary must cite resources\/platform-app-client-api-boundary\.json/);
    assert.match(result.stderr, /app-client-api-boundary must cite validate-platform-app-client-api-boundary\.mjs/);
    assert.match(result.stderr, /app-client-api-boundary must cite platform-app-client-api-boundary\.test\.mjs/);
  });

  it("rejects missing org, role group and area governance gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "governance-org-area-role-groups");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability governance-org-area-role-groups/);
  });

  it("rejects missing form schema layout slot gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "form-schema-layout-slot-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability form-schema-layout-slot-gate/);
  });

  it("rejects missing capability profile composition gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "capability-profile-composition-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability capability-profile-composition-gate/);
  });

  it("rejects capability contract governance gates without contract evidence", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "capability-contract-governance");
    capability.evidence.sourcePaths = capability.evidence.sourcePaths.filter((item) => item !== "resources/platform-capability-contracts.json");
    capability.evidence.validators = [];
    capability.evidence.tests = [];
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability-contract-governance must cite resources\/platform-capability-contracts\.json/);
    assert.match(result.stderr, /capability-contract-governance must cite validate-platform-capability-contracts\.mjs/);
    assert.match(result.stderr, /capability-contract-governance must cite platform-capability-contracts\.test\.mjs/);
  });

  it("rejects missing codegen source-writing readiness gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "codegen-source-writing-readiness");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability codegen-source-writing-readiness/);
  });

  it("keeps safe codegen scaffold implemented while source writing is disabled", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "safe-codegen-scaffold");

    assert.equal(capability.status, "implemented");
    assert.equal(capability.evidence.scaffoldPlan.sourceWriting, "disabled");
    assert.equal(capability.evidence.scaffoldPlan.dryRun, true);
    assert.ok(capability.evidence.generatedFiles.includes("resources/generated/admin-scaffold-promotion-review.json"));
  });

  it("rejects safe codegen scaffold status drift back to preview", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    const capability = matrix.capabilities.find((item) => item.id === "safe-codegen-scaffold");
    capability.status = "preview-scaffold";
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /safe-codegen-scaffold status must be implemented/);
  });

  it("rejects missing file operation audit contract", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "file-operation-audit-contract");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required capability file-operation-audit-contract/);
  });

  it("rejects scaffold plans that enable source writing", () => {
    const scaffoldPlan = readJSON("resources/generated/admin-scaffold-plan.json");
    scaffoldPlan.mode.sourceWriting = "enabled";
    const scaffoldPlanPath = tempJSON("admin-scaffold-plan.json", scaffoldPlan);

    const result = runValidator(["--scaffold-plan", scaffoldPlanPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /scaffold plan mode\.sourceWriting must be disabled/);
  });

  it("rejects backend stack dependency drift", () => {
    const goMod = fs.readFileSync(path.join(repoRoot, "go.mod"), "utf8").replace("github.com/casbin/casbin/v2 v2.135.0\n", "");
    const goModPath = tempText("go.mod", goMod);

    const result = runValidator(["--go-mod", goModPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /backend stack dependency Casbin is missing from go\.mod/);
  });

  it("rejects frontend stack dependency drift", () => {
    const adminPackage = readJSON("admin/package.json");
    delete adminPackage.dependencies["@refinedev/core"];
    const adminPackagePath = tempJSON("package.json", adminPackage);

    const result = runValidator(["--admin-package", adminPackagePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /frontend stack dependency Refine core is missing from admin\/package\.json/);
  });

  it("rejects target stack source wiring drift even when dependencies remain installed", () => {
    const stackSourceRoot = tempStackSourceRoot((root) => {
      const appPath = path.join(root, "admin/src/App.tsx");
      const source = fs.readFileSync(appPath, "utf8").replace("accessControlProvider={accessControlProvider}", "accessControlProvider={undefined}");
      fs.writeFileSync(appPath, source);
    });

    const result = runValidator(["--stack-source-root", stackSourceRoot]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Refine runtime providers must include accessControlProvider=\{accessControlProvider\}/);
  });
});
