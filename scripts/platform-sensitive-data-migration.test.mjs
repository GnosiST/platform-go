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

function assertRejected(result, expected) {
  assert.notEqual(result.status, 0, result.stdout);
  assert.match(result.stderr, expected);
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
    assert.match(evidenceResult.stderr, /evidence artifact must not contain an encrypted value/);
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

  it("scans the full runbook prose for encrypted values", () => {
    const runbook = `${read("docs/platform-sensitive-data-migration.md")}\nObserved value pgo:enc:v1:actual-ciphertext.\n`;

    assertRejected(
      runValidator(["--runbook", tempText("runbook.md", runbook)]),
      /runbook.*encrypted value/i,
    );
  });

  it("allows redacted runbook command placeholders and a bare envelope prefix", () => {
    const runbook = `${read("docs/platform-sensitive-data-migration.md")}\n\`\`\`text\npgo:enc:v1:\nPLATFORM_DATABASE_DSN=<redacted>\n\`\`\`\n\`\`\`bash\nplatform-admin sensitive-data-migrate --backup-uri <redacted>\n\`\`\`\n`;

    const result = runValidator(["--runbook", tempText("safe-runbook.md", runbook)]);
    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects a literal runbook operational evidence value", () => {
    const source = read("docs/platform-sensitive-data-migration.md");
    for (const runbook of [
      source.replace('--backup-uri "$MIGRATION_BACKUP_URI"', "--backup-uri backup-reference-1042"),
      `${source}\n\`\`\`text\nPLATFORM_DATABASE_DSN=opaque-database-target\n\`\`\`\n`,
    ]) {
      assertRejected(
        runValidator(["--runbook", tempText("literal-runbook.md", runbook)]),
        /runbook command examples must use environment references or redacted placeholders/i,
      );
    }
  });

  it("scans the Task 7 report for secret-bearing assignments", () => {
    const report = "# Task 7 Report\nnonce=actual-nonce-value\n";

    assertRejected(
      runValidator(["--task-report", tempText("task-7-report.md", report)]),
      /Task 7 report.*nonce/i,
    );
  });

  it("scans migration closeout document evidence", () => {
    const closeoutEvidence = "Review evidence only. blind-index=actual-index-value\n";

    assertRejected(
      runValidator(["--closeout-evidence-file", tempText("closeout-evidence.md", closeoutEvidence)]),
      /closeout evidence.*blind index/i,
    );
  });

  it("rejects concrete cryptographic material while allowing policy terms and placeholders", () => {
    const safeEvidence = [
      "Keys, nonces, AAD and blind indexes must never be recorded.",
      "pgo:enc:v1:",
      "key=$PLATFORM_DATA_KEY",
      "nonce=<redacted>",
      "aad=${MIGRATION_AAD}",
      "blind-index=<placeholder>",
    ].join("\n");
    const safeResult = runValidator(["--evidence-file", tempText("safe-evidence.md", safeEvidence)]);
    assert.equal(safeResult.status, 0, safeResult.stderr);

    for (const fixture of [
      { name: "ciphertext", value: "ciphertext=actual-ciphertext-value", expected: /ciphertext/i },
      { name: "key", value: "key=actual-key-value", expected: /key material/i },
      { name: "json-key", value: '{"key":"actual-key-value"}', expected: /key material/i },
      { name: "nonce", value: "nonce=actual-nonce-value", expected: /nonce/i },
      { name: "aad", value: "aad=tenant-a\/resource-a\/record-1", expected: /AAD/i },
      { name: "blind-index", value: "blind-index=actual-index-value", expected: /blind index/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects URI DSNs and concrete record or tenant identifiers", () => {
    for (const fixture of [
      { name: "dsn", value: "dsn=postgres://migration:secret@db.internal/platform", expected: /DSN/i },
      { name: "record-id", value: "record-id=record-1042", expected: /record ID/i },
      { name: "record-id-colon", value: "record ID: record-1042", expected: /record ID/i },
      { name: "tenant-id", value: "tenant-id=tenant-north-7", expected: /tenant ID/i },
      { name: "tenant-id-colon", value: "tenant ID: tenant-north-7", expected: /tenant ID/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects numeric, UUID and compact colon-form identifiers while allowing placeholders", () => {
    const safeEvidence = [
      "record ID: $MIGRATION_RECORD_ID",
      "tenant ID: ${MIGRATION_TENANT_ID}",
      "record ID: <redacted>",
      "record ID: the target record coordinate",
      "record ID: run ID plus a domain-separated hash",
    ].join("\n");
    const safeResult = runValidator(["--evidence-file", tempText("safe-identifiers.md", safeEvidence)]);
    assert.equal(safeResult.status, 0, safeResult.stderr);

    for (const fixture of [
      { name: "record-number", value: "record ID: 1042", expected: /record ID/i },
      { name: "record-uuid", value: "record ID: 550e8400-e29b-41d4-a716-446655440000", expected: /record ID/i },
      { name: "tenant-number", value: "tenant ID: 42", expected: /tenant ID/i },
      { name: "tenant-token", value: "tenant ID: north-tenant-7", expected: /tenant ID/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects every single-token colon-form record or tenant identifier", () => {
    for (const fixture of [
      { name: "record-ulid", value: "record ID: 01JABC123XYZ", expected: /record ID/i },
      { name: "record-cuid", value: "record ID: ckx7abc123def", expected: /record ID/i },
      { name: "record-compact", value: "record ID: acme42", expected: /record ID/i },
      { name: "tenant-alpha", value: "tenant ID: ACME", expected: /tenant ID/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects inline single-token record or tenant identifiers", () => {
    const safeEvidence = [
      "Observed record ID: $MIGRATION_RECORD_ID",
      "Target tenant ID: <redacted>",
      "Policy record ID: the target record coordinate",
    ].join("\n");
    const safeResult = runValidator(["--evidence-file", tempText("safe-inline-identifiers.md", safeEvidence)]);
    assert.equal(safeResult.status, 0, safeResult.stderr);

    for (const fixture of [
      { name: "inline-record", value: "Observed record ID: acme42", expected: /record ID/i },
      { name: "inline-tenant", value: "Target tenant ID: ACME", expected: /tenant ID/i },
      { name: "inline-ulid", value: "Audit record ID: 01JABC123XYZ", expected: /record ID/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("allows quoted identifier placeholders and rejects quoted literal identifiers", () => {
    const safeEvidence = [
      'record ID: "$MIGRATION_RECORD_ID"',
      "tenant ID: '<redacted>'",
      '{"record ID":"${MIGRATION_RECORD_ID}"}',
      "tenant ID: '${MIGRATION_TENANT_ID}'",
    ].join("\n");
    const safeResult = runValidator(["--evidence-file", tempText("safe-quoted-identifiers.yaml", safeEvidence)]);
    assert.equal(safeResult.status, 0, safeResult.stderr);

    for (const fixture of [
      { name: "quoted-record", value: 'record ID: "acme42"', expected: /record ID/i },
      { name: "quoted-tenant-json", value: '{"tenant ID":"ACME"}', expected: /tenant ID/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.json`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects email, mainland mobile and Chinese identity PII", () => {
    for (const fixture of [
      { name: "email", value: "owner=alice@example.com", expected: /email/i },
      { name: "phone", value: "phone=13800138000", expected: /phone/i },
      { name: "identity", value: "identity=11010519491231002X", expected: /identity/i },
    ]) {
      assertRejected(
        runValidator(["--evidence-file", tempText(`${fixture.name}.md`, `${fixture.value}\n`)]),
        fixture.expected,
      );
    }
  });

  it("rejects any bootstrap driver beyond mysql, postgres and sqlite", () => {
    const source = read("internal/platform/bootstrap/sensitive_migration.go");
    for (const bootstrap of [
      source.replace(
        '\tcase "mysql", "postgres", "sqlite":\n\t\treturn true',
        '\tcase "mysql", "postgres", "sqlite":\n\t\treturn true\n\tcase "oracle":\n\t\treturn true',
      ),
      source.replace(
        "func sensitiveMigrationGORMDriver(driver string) bool {",
        'func sensitiveMigrationGORMDriver(driver string) bool {\n\tif driver == "kingbase" {\n\t\treturn true\n\t}',
      ),
      source.replace(
        '\tcase "mysql", "postgres", "sqlite":',
        '\tcase "oracle":\n\t\tfallthrough\n\tcase "mysql", "postgres", "sqlite":',
      ),
      source.replace(
        '\tcase "mysql", "postgres", "sqlite":',
        '\tcase oracleDriver, "mysql", "postgres", "sqlite":',
      ),
      source.replace(
        '\tcase "mysql", "postgres", "sqlite":',
        '\tcase configuredDriver(), "mysql", "postgres", "sqlite":',
      ),
      source.replace("\t\treturn false", '\t\treturn false || driver == "oracle"'),
      source.replace("\t\treturn true", '\t\treturn true && driver != "oracle"'),
      source.replace("\t\treturn true", "\t\t_ = driver\n\t\treturn true"),
    ]) {
      assertRejected(
        runValidator(["--bootstrap", tempText("sensitive_migration.go", bootstrap)]),
        /driver gate must allow exactly mysql, postgres and sqlite/i,
      );
    }
  });

  it("rejects mutation or decryption calls from verify", () => {
    for (const call of ["store.ApplyBatch(ctx, BatchMutation{})", "r.runtime.Reveal(ctx, \"value\", dataprotection.FieldPolicy{}, dataprotection.FieldContext{})"]) {
      const runner = read("internal/platform/sensitivemigration/runner.go").replace(
        "func (r *Runner) runVerify(ctx context.Context, store MutatingStore, request RunRequest, batchSize int, state RunState, report Report) (Report, error) {",
        `func (r *Runner) runVerify(ctx context.Context, store MutatingStore, request RunRequest, batchSize int, state RunState, report Report) (Report, error) {\n\t${call}`,
      );
      assertRejected(
        runValidator(["--runner", tempText("runner.go", runner)]),
        /verify path must not call mutation or decryption boundaries/i,
      );
    }
  });

  it("rejects a mutation call from the verify dispatch branch", () => {
    const runner = read("internal/platform/sensitivemigration/runner.go").replace(
      "case ModeVerify:\n\t\tif !validRunIdentity(request)",
      "case ModeVerify:\n\t\t_ = store.FinishRun(ctx, request.RunID, StatusCompleted)\n\t\tif !validRunIdentity(request)",
    );

    assertRejected(
      runValidator(["--runner", tempText("runner.go", runner)]),
      /verify path must not call mutation or decryption boundaries/i,
    );
  });

  it("rejects a write from the verify state loader", () => {
    const gormStore = read("internal/platform/adminresource/sensitive_migration_gorm.go").replace(
      "func (s *GORMProtectedValueMigrationStore) StartOrResume(ctx context.Context, request sensitivemigration.RunRequest) (sensitivemigration.RunState, error) {",
      "func (s *GORMProtectedValueMigrationStore) StartOrResume(ctx context.Context, request sensitivemigration.RunRequest) (sensitivemigration.RunState, error) {\n\t_ = s.db.Save(&gormSensitiveMigrationRun{}).Error",
    );

    assertRejected(
      runValidator(["--gorm-store", tempText("sensitive_migration_gorm.go", gormStore)]),
      /verify state loader must stay read-only/i,
    );
  });

  it("rejects an ordinary Store plaintext fallback", () => {
    const protectionSource = read("internal/platform/adminresource/security.go").replace(
      'return invalidSecurityField(field.Key, "does not contain a valid envelope")',
      "return nil // plaintext fallback",
    );

    assertRejected(
      runValidator(["--protection-source", tempText("security.go", protectionSource)]),
      /ordinary Store must reject plaintext for encrypted fields/i,
    );
  });

  it("rejects migration bootstrap from the API startup composition root", () => {
    const apiMain = read("cmd/platform-api/main.go").replace(
      "func main() {",
      "func main() {\n\t_, _ = bootstrap.OpenSensitiveDataMigration(config.Load())",
    );

    assertRejected(
      runValidator(["--api-main", tempText("main.go", apiMain)]),
      /API startup must not call sensitive data migration entry points/i,
    );
  });
});
