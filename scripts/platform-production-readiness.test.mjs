import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-production-readiness.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-production-readiness-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-production-readiness", () => {
  it("accepts the current production readiness contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ production readiness checks/);
  });

  it("requires the retention runner control contract", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.conditionalEnv = readiness.conditionalEnv.filter((item) => item.name !== "PLATFORM_RETENTION_RUNNER_ENABLED");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /conditionalEnv must include PLATFORM_RETENTION_RUNNER_ENABLED/);
  });

  it("rejects production environment variables that are not loaded by config", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.requiredEnv.push({
      name: "PLATFORM_MISSING_PRODUCTION_ENV",
      purpose: "Should fail when not read by config.Load.",
      docs: ["README.md"],
      validateRuntime: true,
    });
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required env PLATFORM_MISSING_PRODUCTION_ENV is not read by config.Load/);
  });

  it("rejects production runtime variables that are not documented", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    const jwt = readiness.requiredEnv.find((item) => item.name === "PLATFORM_JWT_SECRET");
    jwt.docs = ["docs/platform-cache.md"];
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /required env PLATFORM_JWT_SECRET is missing from docs\/platform-cache\.md/);
  });

  it("rejects missing production env audit preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "production-env-audit");
    readiness.productionEnvAudit.strictCommand = "rtk node scripts/validate-platform-production-env.mjs";
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command production-env-audit/);
    assert.match(result.stderr, /productionEnvAudit\.strictCommand must include --strict-secrets/);
    assert.match(result.stderr, /production readiness preflight must include production-env-audit/);
  });

  it("rejects runtime gates that stop requiring demo auth provider disablement", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.runtimeGate.requiredSnippets = readiness.runtimeGate.requiredSnippets.filter(
      (snippet) => snippet !== "production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
    );
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true/);
  });

  it("rejects readiness contracts that omit transport security configuration", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.requiredEnv = readiness.requiredEnv.filter(
      (item) => !["PLATFORM_PUBLIC_BASE_URL", "PLATFORM_TRUSTED_PROXIES", "PLATFORM_EDGE_TRUSTED_PROXY", "PLATFORM_HTTP_MAX_BODY_BYTES"].includes(item.name),
    );
    readiness.runtimeGate.requiredSnippets = readiness.runtimeGate.requiredSnippets.filter(
      (snippet) => !snippet.includes("PLATFORM_PUBLIC_BASE_URL") && !snippet.includes("PLATFORM_TRUSTED_PROXIES") && !snippet.includes("PLATFORM_EDGE_TRUSTED_PROXY"),
    );
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredEnv must include PLATFORM_PUBLIC_BASE_URL/);
    assert.match(result.stderr, /requiredEnv must include PLATFORM_TRUSTED_PROXIES/);
    assert.match(result.stderr, /requiredEnv must include PLATFORM_HTTP_MAX_BODY_BYTES/);
    assert.match(result.stderr, /requiredEnv must include PLATFORM_EDGE_TRUSTED_PROXY/);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires PLATFORM_PUBLIC_BASE_URL/);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires a non-empty PLATFORM_TRUSTED_PROXIES policy/);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv4 addresses/);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires PLATFORM_EDGE_TRUSTED_PROXY to be one canonical IP address/);
  });

	it("rejects readiness contracts that omit shared rate-limit key configuration", () => {
		const readiness = readJSON("resources/platform-production-readiness.json");
		readiness.requiredEnv = readiness.requiredEnv.filter((item) => item.name !== "PLATFORM_RATE_LIMIT_HMAC_KEY");
		readiness.runtimeGate.requiredSnippets = readiness.runtimeGate.requiredSnippets.filter(
			(snippet) => !snippet.includes("PLATFORM_RATE_LIMIT_HMAC_KEY"),
		);
		const readinessPath = tempJSON("platform-production-readiness.json", readiness);

		const result = runValidator(["--readiness", readinessPath]);

		assert.notEqual(result.status, 0, result.stdout);
		assert.match(result.stderr, /requiredEnv must include PLATFORM_RATE_LIMIT_HMAC_KEY/);
		assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY/);
		assert.match(result.stderr, /runtimeGate\.requiredSnippets must include production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY to be distinct/);
	});

  it("rejects readiness contracts that omit conditional sensitive reveal configuration", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.conditionalEnv = [];
    readiness.runtimeGate.requiredSnippets = readiness.runtimeGate.requiredSnippets.filter(
      (snippet) => !snippet.includes("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY"),
    );
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /conditionalEnv must include PLATFORM_SENSITIVE_REVEAL_HMAC_KEY/);
    assert.match(result.stderr, /conditionalEnv must include PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD/);
    assert.match(result.stderr, /runtimeGate\.requiredSnippets must include PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must be distinct/);
  });

  it("rejects readiness commands whose executable script is missing", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands.push({
      id: "missing-script",
      command: "rtk node scripts/missing-production-check.mjs",
      purpose: "Should fail because the script does not exist.",
    });
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /preflight command missing-script references missing path scripts\/missing-production-check\.mjs/);
  });

  it("rejects missing production auth hardening preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "production-auth-hardening");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command production-auth-hardening/);
  });

  it("keeps Admin OIDC promotion evidence visible in production readiness", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    const productionAuth = readiness.preflightCommands.find((command) => command.id === "production-auth-hardening");
    const tokenRotation = readiness.operationPolicies.find((policy) => policy.id === "token-rotation");

    assert.match(productionAuth.purpose, /Admin OIDC/);
    assert.match(tokenRotation.purpose, /OIDC client credentials/);
    assert.match(tokenRotation.rollbackRequirement, /OIDC provider rollback/);
    assert.ok(tokenRotation.prohibitedActions.includes("promote Admin OIDC without production-like rehearsal, six-viewport browser acceptance and cleanup evidence"));
  });

  it("rejects production readiness that drops Admin OIDC promotion controls", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    const productionAuth = readiness.preflightCommands.find((command) => command.id === "production-auth-hardening");
    const tokenRotation = readiness.operationPolicies.find((policy) => policy.id === "token-rotation");
    productionAuth.purpose = "Generic auth check.";
    tokenRotation.purpose = "Generic token rotation.";
    tokenRotation.rollbackRequirement = "Generic rollback.";
    tokenRotation.prohibitedActions = tokenRotation.prohibitedActions.filter((item) => !item.startsWith("promote Admin OIDC"));
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-hardening purpose must mention Admin OIDC/);
    assert.match(result.stderr, /token-rotation purpose must mention OIDC client credentials/);
    assert.match(result.stderr, /token-rotation rollbackRequirement must mention OIDC provider rollback/);
    assert.match(result.stderr, /token-rotation prohibitedActions must include production-like Admin OIDC evidence gate/);
  });

  it("rejects missing refresh-token family promotion preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "refresh-token-family-promotion");
    const tokenRotation = readiness.operationPolicies.find((policy) => policy.id === "token-rotation");
    tokenRotation.preflightCommands = tokenRotation.preflightCommands.filter((command) => command !== "refresh-token-family-promotion");
    const requirement = readiness.policyPreflightRequirements.find((item) => item.policy === "token-rotation");
    requirement.requiredCommands = requirement.requiredCommands.filter((command) => command !== "refresh-token-family-promotion");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command refresh-token-family-promotion/);
  });

  it("rejects missing production auth promotion review preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "production-auth-promotion-review");
    const tokenRotation = readiness.operationPolicies.find((policy) => policy.id === "token-rotation");
    tokenRotation.preflightCommands = tokenRotation.preflightCommands.filter((command) => command !== "production-auth-promotion-review");
    const requirement = readiness.policyPreflightRequirements.find((item) => item.policy === "token-rotation");
    requirement.requiredCommands = requirement.requiredCommands.filter((command) => command !== "production-auth-promotion-review");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command production-auth-promotion-review/);
  });

  it("rejects missing cache invalidation preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "cache-invalidation");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command cache-invalidation/);
  });

  it("rejects missing deployment topology preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "deployment-topology");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command deployment-topology/);
  });

  it("rejects missing governance topology preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "governance-topology");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command governance-topology/);
  });

  it("rejects missing capability contracts preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "capability-contracts");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command capability-contracts/);
  });

  it("rejects missing form schema layout slots preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "form-schema-layout-slots");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command form-schema-layout-slots/);
  });

  it("rejects missing codegen source-writing readiness preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "codegen-source-writing-readiness");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command codegen-source-writing-readiness/);
  });

  it("rejects missing admin API boundary preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "admin-api-boundary");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command admin-api-boundary/);
  });

  it("rejects missing app client API boundary preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "app-client-api-boundary");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command app-client-api-boundary/);
  });

  it("rejects missing optional personnel preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "personnel-runtime-readiness");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command personnel-runtime-readiness/);
  });

  it("rejects missing foundation task graph preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "foundation-task-graph");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command foundation-task-graph/);
  });

  it("rejects missing objective conflict or admin UI preflight commands", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter(
      (command) =>
        command.id !== "foundation-alignment" &&
        command.id !== "task-execution-audit" &&
        command.id !== "objective-conformance" &&
        command.id !== "admin-ui-contract-tests" &&
        command.id !== "admin-build",
    );
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command foundation-alignment/);
    assert.match(result.stderr, /missing required preflight command task-execution-audit/);
    assert.match(result.stderr, /missing required preflight command objective-conformance/);
    assert.match(result.stderr, /missing required preflight command admin-ui-contract-tests/);
    assert.match(result.stderr, /missing required preflight command admin-build/);
  });

  it("rejects missing goal completion audit preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "goal-completion-audit");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command goal-completion-audit/);
  });

  it("rejects missing node closeout audit preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "node-closeout-audit");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command node-closeout-audit/);
  });

  it("rejects missing promotion evidence template preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "promotion-evidence-templates");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required preflight command promotion-evidence-templates/);
  });

  it("rejects missing or unsafe production preflight runner metadata", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightRunner = {
      script: "scripts/missing-production-preflight-runner.mjs",
      listCommand: "node scripts/missing-production-preflight-runner.mjs --list",
      dryRunCommand: "node scripts/missing-production-preflight-runner.mjs",
      runCommand: "node scripts/missing-production-preflight-runner.mjs --run",
      strictEnvCommand: "node scripts/missing-production-preflight-runner.mjs --command production-env-audit",
      docs: ["README.md"],
    };
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /preflightRunner\.script must be scripts\/run-platform-production-preflight\.mjs/);
    assert.match(result.stderr, /preflightRunner\.script path is missing or unsafe/);
    assert.match(result.stderr, /preflightRunner\.listCommand must use rtk node scripts\/run-platform-production-preflight\.mjs --list/);
    assert.match(result.stderr, /preflightRunner\.strictEnvCommand must include --strict-env-file/);
  });

  it("rejects production runtime checks without test coverage", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.runtimeGate.tests = [];
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeGate must declare at least one test/);
  });

  it("rejects missing production operation policies", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.operationPolicies = (readiness.operationPolicies ?? []).filter((policy) => policy.id !== "config-import-restore");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required operation policy config-import-restore/);
  });

  it("rejects missing policy-level preflight requirements", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.policyPreflightRequirements = readiness.policyPreflightRequirements.filter((requirement) => requirement.policy !== "token-rotation");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing policy preflight requirement token-rotation/);
  });

  it("rejects malformed policy-level preflight requirements", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    const requirement = readiness.policyPreflightRequirements.find((item) => item.policy === "database-migration");
    requirement.requiredCommands = [];
    requirement.reason = "";
    readiness.policyPreflightRequirements.push({
      policy: "config-backup-export",
      requiredCommands: ["missing-command"],
      reason: "Duplicate policy should fail.",
    });
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /policy preflight requirement database-migration must declare requiredCommands/);
    assert.match(result.stderr, /policy preflight requirement database-migration must declare reason/);
    assert.match(result.stderr, /policy preflight requirement config-backup-export is duplicated/);
    assert.match(result.stderr, /policy preflight requirement config-backup-export references unknown preflight command missing-command/);
  });

  it("rejects production operation policies without review, docs, preflight or prohibited actions", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.operationPolicies = readiness.operationPolicies ?? [
      {
        id: "config-backup-export",
        purpose: "Should fail when required governance fields are missing.",
        docs: ["README.md"],
        preflightCommands: ["rtk node scripts/validate-platform-production-readiness.mjs"],
        requiresHumanReview: true,
        prohibitedActions: ["overwrite runtime data without a reviewed backup artifact"],
      },
      {
        id: "config-import-restore",
        purpose: "Placeholder required policy.",
        docs: ["README.md"],
        preflightCommands: ["rtk node scripts/validate-platform-production-readiness.mjs"],
        requiresHumanReview: true,
        prohibitedActions: ["restore without dry-run"],
      },
      {
        id: "database-migration",
        purpose: "Placeholder required policy.",
        docs: ["README.md"],
        preflightCommands: ["rtk node scripts/validate-platform-production-readiness.mjs"],
        requiresHumanReview: true,
        prohibitedActions: ["run destructive migrations without review"],
      },
      {
        id: "token-rotation",
        purpose: "Placeholder required policy.",
        docs: ["README.md"],
        preflightCommands: ["rtk node scripts/validate-platform-production-readiness.mjs"],
        requiresHumanReview: true,
        prohibitedActions: ["rotate secrets without session impact review"],
      },
    ];
    const policy = readiness.operationPolicies.find((item) => item.id === "config-backup-export");
    policy.docs = [];
    policy.preflightCommands = [];
    policy.requiresHumanReview = false;
    policy.prohibitedActions = [];
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operation policy config-backup-export must declare docs/);
    assert.match(result.stderr, /operation policy config-backup-export must declare preflightCommands/);
    assert.match(result.stderr, /operation policy config-backup-export must require human review/);
    assert.match(result.stderr, /operation policy config-backup-export must declare prohibitedActions/);
  });

  it("rejects cache-sensitive operation policies without cache invalidation preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    for (const policyID of ["config-backup-export", "config-import-restore", "token-rotation"]) {
      const policy = readiness.operationPolicies.find((item) => item.id === policyID);
      policy.preflightCommands = policy.preflightCommands.filter((command) => command !== "cache-invalidation");
    }
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operation policy config-backup-export must include required preflight command cache-invalidation/);
    assert.match(result.stderr, /operation policy config-import-restore must include required preflight command cache-invalidation/);
    assert.match(result.stderr, /operation policy token-rotation must include required preflight command cache-invalidation/);
  });

  it("rejects operation policies that bypass task execution or capability profile gates", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    for (const policyID of ["config-backup-export", "config-import-restore", "database-migration", "token-rotation"]) {
      const policy = readiness.operationPolicies.find((item) => item.id === policyID);
      policy.preflightCommands = policy.preflightCommands.filter((command) => command !== "task-execution-audit");
    }
    for (const policyID of ["config-backup-export", "config-import-restore", "database-migration"]) {
      const policy = readiness.operationPolicies.find((item) => item.id === policyID);
      policy.preflightCommands = policy.preflightCommands.filter((command) => command !== "capability-profiles");
      policy.preflightCommands = policy.preflightCommands.filter((command) => command !== "capability-contracts");
    }
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operation policy config-backup-export must include required preflight command capability-profiles/);
    assert.match(result.stderr, /operation policy config-backup-export must include required preflight command capability-contracts/);
    assert.match(result.stderr, /operation policy config-backup-export must include required preflight command task-execution-audit/);
    assert.match(result.stderr, /operation policy config-import-restore must include required preflight command capability-profiles/);
    assert.match(result.stderr, /operation policy config-import-restore must include required preflight command capability-contracts/);
    assert.match(result.stderr, /operation policy database-migration must include required preflight command capability-profiles/);
    assert.match(result.stderr, /operation policy database-migration must include required preflight command capability-contracts/);
    assert.match(result.stderr, /operation policy token-rotation must include required preflight command task-execution-audit/);
  });

  it("rejects operation policies that bypass objective conformance", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    for (const policyID of ["config-backup-export", "config-import-restore", "database-migration", "token-rotation"]) {
      const policy = readiness.operationPolicies.find((item) => item.id === policyID);
      policy.preflightCommands = policy.preflightCommands.filter((command) => command !== "objective-conformance");
    }
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /operation policy config-backup-export must include required preflight command objective-conformance/);
    assert.match(result.stderr, /operation policy config-import-restore must include required preflight command objective-conformance/);
    assert.match(result.stderr, /operation policy database-migration must include required preflight command objective-conformance/);
    assert.match(result.stderr, /operation policy token-rotation must include required preflight command objective-conformance/);
  });
});
