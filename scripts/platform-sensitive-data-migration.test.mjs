import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");
const migrationTaskID = "sensitive-data-historical-migration";
const remainingTaskIDs = [
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];
const modes = ["inventory", "dry-run", "prepare", "apply", "verify", "rehearse-restore", "rollback"];

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-sensitive-data-migration.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

function readJSON(relativePath) {
  return JSON.parse(read(relativePath));
}

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-sensitive-data-migration-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
  return filePath;
}

function tempJSON(name, value) {
  return tempText(name, `${JSON.stringify(value, null, 2)}\n`);
}

describe("validate-platform-sensitive-data-migration", () => {
  it("accepts the implemented offline migration governance contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform sensitive data migration governance/);
  });

  it("documents all seven modes, the operator sequence and the promotion driver policy", () => {
    const runbookPath = path.join(repoRoot, "docs/platform-sensitive-data-migration.md");
    assert.equal(fs.existsSync(runbookPath), true, "operator migration runbook must exist");
    const runbook = fs.readFileSync(runbookPath, "utf8");

    for (const mode of modes) {
      assert.match(runbook, new RegExp(`\\b${mode.replace("-", "\\-")}\\b`));
    }
    for (const step of [
      "external backup",
      "restore evidence",
      "inventory",
      "dry-run",
      "prepare",
      "apply",
      "verify",
      "rehearse-restore",
      "rollback",
      "resume",
      "incident stop",
    ]) {
      assert.match(runbook, new RegExp(step, "i"), `runbook must document ${step}`);
    }

    assert.match(runbook, /MySQL and PostgreSQL[^\n]*production targets/i);
    assert.match(runbook, /integration rehearsal[^\n]*certification evidence/i);
    assert.match(runbook, /SQLite[^\n]*(?:development|test)[^\n]*local rehearsal/i);
    assert.match(runbook, /Oracle and Kingbase[^\n]*not certified/i);
    assert.match(runbook, /file[^\n]*legacy SQL[^\n]*reject/i);
    assert.match(runbook, /escrow[^\n]*does not replace[^\n]*external backup/i);
  });

  it("keeps the CLI and source contract value-free and maintenance-only", () => {
    const model = read("internal/platform/sensitivemigration/model.go");
    const cli = read("cmd/platform-admin/main.go");
    const bootstrap = read("internal/platform/bootstrap/sensitive_migration.go");
    const httpAPI = read("internal/platform/httpapi/server.go");
    const openAPI = readJSON("resources/generated/openapi.admin.json");

    for (const mode of modes) {
      assert.match(model, new RegExp(`Mode[A-Za-z]+\\s+Mode\\s+=\\s+"${mode}"`));
    }
    for (const flag of [
      "--run-id",
      "--actor",
      "--reason",
      "--approval-ref",
      "--backup-uri",
      "--backup-sha256",
      "--restore-evidence-ref",
      "--maintenance-window-confirmed",
    ]) {
      assert.ok(cli.includes(flag.slice(2)), `CLI must declare ${flag}`);
    }
    assert.match(bootstrap, /case "mysql", "postgres", "sqlite":/);
    assert.match(bootstrap, /driver == "sqlite" && !sensitiveMigrationLocalEnvironment\(environment\)/);
    assert.match(
      bootstrap,
      /environment == config\.RuntimeEnvironmentDevelopment \|\| environment == config\.RuntimeEnvironmentTest/,
    );
    assert.match(bootstrap, /logger\.Discard/);
    assert.doesNotMatch(httpAPI, /sensitive-data-migrat|sensitive_data_migrat/i);
    assert.ok(!Object.keys(openAPI.paths ?? {}).some((route) => /sensitive-data-migrat/i.test(route)));
  });

  it("projects 45 total, 41 implemented and four controlled unfinished nodes with a non-visual closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const alignment = readJSON("resources/platform-foundation-alignment-audit.json");
    const goal = readJSON("resources/platform-goal-completion-audit.json");
    const closeout = readJSON("resources/platform-node-closeout-audit.json");
    const objective = readJSON("resources/platform-objective-conformance.json");
    const execution = readJSON("resources/platform-task-execution-audit.json");
    const engineering = readJSON("resources/platform-engineering-capabilities.json");
    const migrationTask = graph.tasks.find((task) => task.id === migrationTaskID);
    const migrationCloseout = closeout.nodeCloseouts.find((item) => item.taskId === migrationTaskID);
    const migrationCapability = engineering.capabilities.find((item) => item.id === "sensitive-data-protection");

    assert.equal(graph.tasks.length, 45);
    assert.equal(graph.tasks.filter((task) => task.status === "implemented").length, 41);
    assert.equal(migrationTask?.status, "implemented");
    assert.deepEqual(graph.tasks.filter((task) => task.status !== "implemented").map((task) => task.id), remainingTaskIDs);
    assert.ok(alignment.requiredTaskNodes.includes(migrationTaskID));
    assert.deepEqual(alignment.requiredFutureTaskNodes, remainingTaskIDs);
    assert.deepEqual(goal.taskSummary, { expectedTotal: 45, expectedImplemented: 41, expectedControlledUnfinished: 4 });
    assert.deepEqual(goal.completionPolicy.requiredControlledUnfinishedNodes, remainingTaskIDs);
    assert.deepEqual(closeout.pendingNodeEvidence, remainingTaskIDs);
    assert.equal(migrationCloseout?.status, "closed");
    assert.equal(migrationCloseout?.neatFreak, true);
    assert.equal("visualEvidence" in (migrationCloseout ?? {}), false);
    assert.deepEqual(objective.taskControlPolicy.requiredUnfinishedNodes, remainingTaskIDs);
    assert.deepEqual(objective.completionPolicy.controlledBlockers, remainingTaskIDs);
    assert.deepEqual(execution.requiredUnfinishedNodes, remainingTaskIDs);
    assert.equal(migrationCapability?.status, "implemented");
  });

  it("rejects a mode, approval or redaction regression", () => {
    const model = read("internal/platform/sensitivemigration/model.go").replace(
      'ModeRollback        Mode = "rollback"',
      'ModeRollback        Mode = "removed"',
    );
    const modeResult = runValidator(["--model", tempText("model.go", model)]);
    assert.notEqual(modeResult.status, 0, modeResult.stdout);
    assert.match(modeResult.stderr, /migration modes must exactly match/);

    const runbook = read("docs/platform-sensitive-data-migration.md").replaceAll("--approval-ref", "--approval-removed");
    const approvalResult = runValidator(["--runbook", tempText("runbook.md", runbook)]);
    assert.notEqual(approvalResult.status, 0, approvalResult.stdout);
    assert.match(approvalResult.stderr, /runbook command contract must include --approval-ref/);

    const evidenceResult = runValidator([
      "--evidence-file",
      tempText("unsafe-evidence.json", '{"value":"pgo:enc:v1:fixture-ciphertext"}\n'),
    ]);
    assert.notEqual(evidenceResult.status, 0, evidenceResult.stdout);
    assert.match(evidenceResult.stderr, /evidence artifact must not contain fixture plaintext or encrypted values/);
  });

  it("rejects HTTP exposure and a visual-evidence claim for the non-visual migration node", () => {
    const httpAPI = `${read("internal/platform/httpapi/server.go")}\nconst migrationRoute = "/api/admin/sensitive-data-migrate";\n`;
    const routeResult = runValidator(["--http-api", tempText("server.go", httpAPI)]);
    assert.notEqual(routeResult.status, 0, routeResult.stdout);
    assert.match(routeResult.stderr, /sensitive data migration must not expose an HTTP route/);

    const closeout = readJSON("resources/platform-node-closeout-audit.json");
    closeout.nodeCloseouts.find((item) => item.taskId === migrationTaskID).visualEvidence = ["product-design"];
    const closeoutResult = runValidator(["--closeout", tempJSON("closeout.json", closeout)]);
    assert.notEqual(closeoutResult.status, 0, closeoutResult.stdout);
    assert.match(closeoutResult.stderr, /migration closeout is non-visual and must not declare visualEvidence/);
  });
});
