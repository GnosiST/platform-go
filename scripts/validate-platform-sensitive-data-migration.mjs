import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const migrationTaskID = "sensitive-data-historical-migration";
const sourceLockPath = "resources/platform-sensitive-data-migration-source-lock.json";
const sourceLockPurpose = "Fail closed when reviewed sensitive migration safety-boundary AST changes without an explicit lock update, tests and review.";
const sourceLockUpdateCommand = "rtk node scripts/validate-platform-sensitive-data-migration.mjs --print-source-lock";
const expectedModes = ["inventory", "dry-run", "prepare", "apply", "verify", "rehearse-restore", "rollback"];
const requiredApprovalFlags = [
  "run-id",
  "actor",
  "reason",
  "approval-ref",
  "backup-uri",
  "backup-sha256",
  "restore-evidence-ref",
  "maintenance-window-confirmed",
];
const requiredRequestFields = [
  "RunID",
  "ActorID",
  "Reason",
  "ApprovalRef",
  "BackupURI",
  "BackupHash",
  "RestoreEvidence",
  "MaintenanceConfirmed",
];
const requiredTaskEvidence = {
  docs: [
    "docs/platform-sensitive-data-migration.md",
    "docs/superpowers/specs/2026-07-12-sensitive-data-historical-migration-design.md",
    "docs/superpowers/plans/2026-07-12-sensitive-data-historical-migration.md",
    sourceLockPath,
  ],
  validators: ["scripts/validate-platform-sensitive-data-migration.mjs"],
  tests: [
    "internal/platform/sensitivemigration/runner_test.go",
    "internal/platform/adminresource/sensitive_migration_gorm_test.go",
    "cmd/platform-admin/main_test.go",
    "internal/platform/bootstrap/sensitive_migration_test.go",
    "scripts/platform-sensitive-data-migration.test.mjs",
  ],
};
const requiredCloseoutEvidence = [
  "docs/platform-sensitive-data-migration.md",
  "docs/platform-data-governance-and-integrations-assessment.md",
  "docs/superpowers/specs/2026-07-12-sensitive-data-historical-migration-design.md",
  "docs/superpowers/plans/2026-07-12-sensitive-data-historical-migration.md",
  "internal/platform/sensitivemigration/runner_test.go",
  "internal/platform/adminresource/sensitive_migration_gorm_test.go",
  "cmd/platform-admin/main.go",
  "cmd/platform-admin/main_test.go",
  "internal/platform/bootstrap/sensitive_migration.go",
  "internal/platform/bootstrap/sensitive_migration_test.go",
  "scripts/validate-platform-sensitive-data-migration.mjs",
  "scripts/platform-sensitive-data-migration.test.mjs",
  "scripts/platform-foundation-docs-drift.test.mjs",
  sourceLockPath,
];

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function argValues(name) {
  const result = [];
  for (let index = 0; index < process.argv.length; index += 1) {
    if (process.argv[index] === name && process.argv[index + 1]) result.push(process.argv[index + 1]);
  }
  return result;
}

function resolveArg(name, fallback) {
  return path.resolve(repoRoot, argValue(name, fallback));
}

const paths = {
  runbook: resolveArg("--runbook", "docs/platform-sensitive-data-migration.md"),
  taskReport: resolveArg("--task-report", ".superpowers/sdd/sensitive-data-historical-migration-task-7-report.md"),
  model: resolveArg("--model", "internal/platform/sensitivemigration/model.go"),
  cli: resolveArg("--cli", "cmd/platform-admin/main.go"),
  bootstrap: resolveArg("--bootstrap", "internal/platform/bootstrap/sensitive_migration.go"),
  protectionSource: resolveArg("--protection-source", "internal/platform/adminresource/security.go"),
  apiMain: resolveArg("--api-main", "cmd/platform-api/main.go"),
  gormStore: resolveArg("--gorm-store", "internal/platform/adminresource/sensitive_migration_gorm.go"),
  runner: resolveArg("--runner", "internal/platform/sensitivemigration/runner.go"),
  escrow: resolveArg("--escrow", "internal/platform/sensitivemigration/escrow.go"),
  httpAPI: resolveArg("--http-api", "internal/platform/httpapi/server.go"),
  openAPI: resolveArg("--openapi", "resources/generated/openapi.admin.json"),
  graph: resolveArg("--graph", "resources/platform-foundation-task-graph.json"),
  alignment: resolveArg("--alignment", "resources/platform-foundation-alignment-audit.json"),
  goal: resolveArg("--goal-completion", "resources/platform-goal-completion-audit.json"),
  closeout: resolveArg("--closeout", "resources/platform-node-closeout-audit.json"),
  objective: resolveArg("--objective", "resources/platform-objective-conformance.json"),
  execution: resolveArg("--task-execution", "resources/platform-task-execution-audit.json"),
  engineering: resolveArg("--engineering", "resources/platform-engineering-capabilities.json"),
  sourceLock: resolveArg("--source-lock", sourceLockPath),
};

function readText(filePath) {
  return fs.readFileSync(filePath, "utf8");
}

function readJSON(filePath) {
  return JSON.parse(readText(filePath));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameList(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

function requireIncludes(actual, expected, label, errors) {
  const set = new Set(values(actual));
  for (const item of expected) {
    if (!set.has(item)) errors.push(`${label} must include ${item}`);
  }
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) return false;
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function validateRunbook(runbook, errors) {
  for (const mode of expectedModes) {
    if (!new RegExp(`\\b${mode}\\b`).test(runbook)) errors.push(`runbook must document ${mode}`);
  }
  for (const flag of requiredApprovalFlags) {
    if (!runbook.includes(`--${flag}`)) errors.push(`runbook command contract must include --${flag}`);
  }
  const requiredPhrases = [
    /external backup/i,
    /restore evidence/i,
    /`?prepare`? is the only mode/i,
    /apply never calls `AutoMigrate`/i,
    /incident stop conditions/i,
    /MySQL and PostgreSQL are production targets/i,
    /integration rehearsal and certification evidence/i,
    /SQLite is limited to development or test local rehearsal/i,
    /Oracle and Kingbase are not certified/i,
    /File storage and legacy SQL mutation modes are rejected/i,
    /escrow does not replace an external backup/i,
    /no HTTP route/i,
  ];
  for (const phrase of requiredPhrases) {
    if (!phrase.test(runbook)) errors.push(`runbook is missing required operator policy ${phrase}`);
  }

  const codeBlocks = [...runbook.matchAll(/```(?:bash|text)?\n([\s\S]*?)```/g)].map((match) => match[1]).join("\n");
  const operationalValues = [
    ...codeBlocks.matchAll(/--(?:run-id|actor|reason|approval-ref|backup-uri|backup-sha256|restore-evidence-ref)\s+(?:"([^"]*)"|'([^']*)'|([^\s\\]+))/g),
    ...codeBlocks.matchAll(/PLATFORM_DATABASE_DSN\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s\\]+))/g),
  ];
  if (operationalValues.some((match) => !safeEvidencePlaceholder(match[1] ?? match[2] ?? match[3] ?? ""))) {
    errors.push("runbook command examples must use environment references or redacted placeholders");
  }
}

function cachedASTHelper(errors) {
  const sourcePath = path.join(repoRoot, "internal/tools/sensitivemigrationast/main.go");
  const digest = createHash("sha256")
    .update(readText(sourcePath))
    .update(readText(path.join(repoRoot, "go.mod")))
    .digest("hex")
    .slice(0, 16);
  const cacheDir = path.join(os.tmpdir(), "platform-go-sensitive-migration-ast");
  const helperPath = path.join(cacheDir, `analyze-${digest}${process.platform === "win32" ? ".exe" : ""}`);
  fs.mkdirSync(cacheDir, { recursive: true });
  if (fs.existsSync(helperPath)) return helperPath;

  const lockPath = `${helperPath}.lock`;
  let lock;
  for (let attempt = 0; attempt < 200 && lock === undefined; attempt += 1) {
    try {
      lock = fs.openSync(lockPath, "wx");
    } catch (error) {
      if (error?.code !== "EEXIST") {
        errors.push("sensitive migration Go AST helper cache must be writable");
        return "";
      }
      if (fs.existsSync(helperPath)) return helperPath;
      try {
        const lockAge = Date.now() - fs.statSync(lockPath).mtimeMs;
        if (lockAge > 30_000) fs.rmSync(lockPath, { force: true });
        else Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 50);
      } catch (statError) {
        if (statError?.code !== "ENOENT") throw statError;
      }
    }
  }
  if (lock === undefined) {
    errors.push("sensitive migration Go AST helper cache lock timed out");
    return "";
  }

  const temporaryPath = `${helperPath}.${process.pid}.tmp`;
  try {
    if (!fs.existsSync(helperPath)) {
      const result = spawnSync("go", ["build", "-o", temporaryPath, "./internal/tools/sensitivemigrationast"], {
        cwd: repoRoot,
        encoding: "utf8",
      });
      if (result.status !== 0) {
        errors.push("sensitive migration Go AST helper must build successfully");
        return "";
      }
      fs.renameSync(temporaryPath, helperPath);
    }
    return helperPath;
  } finally {
    fs.rmSync(temporaryPath, { force: true });
    fs.closeSync(lock);
    fs.rmSync(lockPath, { force: true });
  }
}

function sourceASTAnalysis(errors) {
  const helperPath = process.env.PLATFORM_SENSITIVE_MIGRATION_AST_HELPER?.trim() || cachedASTHelper(errors);
  if (!helperPath) return {};
  const sourceArgs = [
    "--bootstrap", paths.bootstrap,
    "--runner", paths.runner,
    "--gorm-store", paths.gormStore,
    "--protection-source", paths.protectionSource,
    "--api-main", paths.apiMain,
    "--cli", paths.cli,
  ];
  const result = spawnSync(helperPath, sourceArgs, { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== 0) {
    errors.push("sensitive migration Go AST analysis must complete successfully");
    return {};
  }
  try {
    return JSON.parse(result.stdout);
  } catch {
    errors.push("sensitive migration Go AST analysis must return structured JSON");
    return {};
  }
}

function sourceLockDocument(fingerprints) {
  return {
    version: 1,
    purpose: sourceLockPurpose,
    updateCommand: sourceLockUpdateCommand,
    entries: values(fingerprints).map((entry) => ({
      path: entry?.path ?? "",
      role: entry?.role ?? "",
      scope: entry?.scope ?? "",
      symbol: entry?.symbol ?? "",
      fingerprint: entry?.fingerprint ?? "",
    })),
  };
}

function sameKeys(value, expected) {
  if (value === null || typeof value !== "object" || Array.isArray(value)) return false;
  return sameList(Object.keys(value).sort(), [...expected].sort());
}

function validateSourceLock(lock, astAnalysis, errors) {
  const expectedDocument = sourceLockDocument(astAnalysis.fingerprints);
  if (!sameKeys(lock, ["version", "purpose", "updateCommand", "entries"]) ||
      lock.version !== expectedDocument.version || lock.purpose !== expectedDocument.purpose ||
      lock.updateCommand !== expectedDocument.updateCommand || !Array.isArray(lock.entries)) {
    errors.push("sensitive migration source lock metadata must match the read-only canonical AST contract");
    return;
  }
  for (const entry of lock.entries) {
    if (!sameKeys(entry, ["path", "role", "scope", "symbol", "fingerprint"]) ||
        typeof entry.path !== "string" || typeof entry.role !== "string" || typeof entry.scope !== "string" ||
        typeof entry.symbol !== "string" || !/^sha256:[0-9a-f]{64}$/.test(entry.fingerprint)) {
      errors.push("sensitive migration source lock entries must use the exact path, role, scope, symbol and SHA-256 shape");
      return;
    }
  }
  if (JSON.stringify(lock.entries) !== JSON.stringify(expectedDocument.entries)) {
    errors.push("sensitive migration source lock must exactly match canonical AST fingerprints; update the lock with tests and review");
  }
}

function validateSourceContract({ model, cli, bootstrap, escrow, httpAPI, openAPI }, astAnalysis, errors) {
  const actualModes = [...model.matchAll(/Mode[A-Za-z]+\s+Mode\s*=\s*"([^"]+)"/g)].map((match) => match[1]);
  if (!sameList(actualModes, expectedModes)) errors.push(`migration modes must exactly match ${expectedModes.join(", ")}`);

  const cliFlags = new Set([...cli.matchAll(/flags\.(?:String|Bool|Int)\("([^"]+)"/g)].map((match) => match[1]));
  for (const flag of requiredApprovalFlags) {
    if (!cliFlags.has(flag)) errors.push(`sensitive migration CLI must declare --${flag}`);
  }
  const requestBlock = /type RunRequest struct \{([\s\S]*?)\n\}/.exec(model)?.[1] ?? "";
  for (const field of requiredRequestFields) {
    if (!new RegExp(`\\b${field}\\b`).test(requestBlock)) errors.push(`RunRequest must declare ${field}`);
  }

  if (astAnalysis.driverGateExact !== true) {
    errors.push("migration bootstrap driver gate must allow exactly mysql, postgres and sqlite");
  }
  if (!/driver == "sqlite" && !sensitiveMigrationLocalEnvironment\(environment\)/.test(bootstrap)) {
    errors.push("migration bootstrap must reject SQLite outside development/test");
  }
  if (!/RuntimeEnvironmentDevelopment\s*\|\|\s*environment == config\.RuntimeEnvironmentTest/.test(bootstrap)) {
    errors.push("migration bootstrap local environment gate must be development/test only");
  }
  if (!/logger\.Discard/.test(bootstrap)) errors.push("migration bootstrap must silence failed storage initialization logs");
  if (!/strings\.TrimSpace\(cfg\.AdminResourceFile\) != ""/.test(bootstrap)) errors.push("migration bootstrap must reject file mutation configuration");

  if (astAnalysis.prepareOwnsJournalSchema !== true) errors.push("prepare must be the only journal schema creation mode");
  if (astAnalysis.readOnlyDispatch !== true) errors.push("inventory and dry-run must stay on the read-only runner path");
  if (astAnalysis.preparedDispatch !== true) {
    errors.push("prepared runner path must contain prepare, apply, verify, rehearse-restore and rollback");
  }
  if (astAnalysis.verifyPathSafe !== true) {
    errors.push("verify path must not call mutation or decryption boundaries");
  }
  if (astAnalysis.verifyStateLoaderReadOnly !== true) {
    errors.push("verify state loader must stay read-only");
  }

  if (astAnalysis.plaintextRejected !== true) {
    errors.push("ordinary Store must reject plaintext for encrypted fields");
  }

  if (astAnalysis.apiStartupSafe !== true) {
    errors.push("API startup must not call sensitive data migration entry points");
  }
  if (!/MigratedValueHash/.test(escrow) || !/migration-rollback/.test(escrow)) errors.push("rollback escrow must retain reserved context and migrated-value hash guards");

  const reportBlock = /type Report struct \{([\s\S]*?)\n\}/.exec(model)?.[1] ?? "";
  const reportFields = [...reportBlock.matchAll(/json:"([^",]+)/g)].map((match) => match[1]);
  if (!sameList(reportFields, ["runId", "mode", "status", "counts", "checkpoints", "eventChainHead"])) {
    errors.push("migration report must remain the value-free single JSON summary contract");
  }
  if (astAnalysis.cliReportBeforeClose !== true) {
    errors.push("migration CLI must emit one JSON report before closing storage");
  }
  if (astAnalysis.cliErrorsValueFree !== true) errors.push("migration CLI errors must remain normalized and value-free");

  if (/sensitive-data-migrat|sensitive_data_migrat/i.test(httpAPI)) errors.push("sensitive data migration must not expose an HTTP route");
  if (Object.keys(openAPI.paths ?? {}).some((route) => /sensitive-data-migrat/i.test(route))) {
    errors.push("sensitive data migration must not appear in Admin OpenAPI routes");
  }
}

function validateGovernance({ graph, alignment, goal, closeout, objective, execution, engineering }, errors) {
  const tasks = values(graph.tasks);
  const migrationTask = tasks.find((task) => task.id === migrationTaskID);
  const unfinished = tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
  const implementedCount = tasks.filter((task) => task.status === "implemented").length;
  if (tasks.length === 0 || implementedCount + unfinished.length !== tasks.length) errors.push("official task graph projection must account for every task node");
  if (migrationTask?.status !== "implemented") errors.push("sensitive-data-historical-migration must stay implemented after closeout");
  for (const [kind, required] of Object.entries(requiredTaskEvidence)) {
    requireIncludes(migrationTask?.evidence?.[kind], required, `migration task evidence.${kind}`, errors);
  }
  for (const kind of ["docs", "validators", "tests", "screenshots"]) {
    for (const relativePath of values(migrationTask?.evidence?.[kind])) {
      if (!relativeExistingPath(relativePath)) errors.push(`migration task evidence path is missing or unsafe: ${relativePath}`);
    }
  }

  if (!values(alignment.requiredTaskNodes).includes(migrationTaskID)) errors.push("alignment requiredTaskNodes must include sensitive-data-historical-migration");
  if (!sameList(values(alignment.requiredFutureTaskNodes), unfinished)) errors.push("alignment requiredFutureTaskNodes must match unfinished task graph nodes");
  requireIncludes(alignment.requiredValidators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "alignment requiredValidators", errors);
  requireIncludes(alignment.documents, ["docs/platform-sensitive-data-migration.md"], "alignment documents", errors);

  if (goal.taskSummary?.expectedTotal !== tasks.length || goal.taskSummary?.expectedImplemented !== implementedCount || goal.taskSummary?.expectedControlledUnfinished !== unfinished.length) {
    errors.push("goal completion taskSummary must match the current task graph projection");
  }
  if (!sameList(values(goal.completionPolicy?.requiredControlledUnfinishedNodes), unfinished)) {
    errors.push("goal completion controlled unfinished nodes must match unfinished task graph nodes");
  }

  const migrationCloseout = values(closeout.nodeCloseouts).find((item) => item.taskId === migrationTaskID);
  if (migrationCloseout?.status !== "closed" || migrationCloseout?.neatFreak !== true) errors.push("migration closeout must be closed with neat-freak evidence");
  if (Object.hasOwn(migrationCloseout ?? {}, "visualEvidence")) errors.push("migration closeout is non-visual and must not declare visualEvidence");
  requireIncludes(migrationCloseout?.cleanupEvidence, requiredCloseoutEvidence, "migration closeout cleanupEvidence", errors);
  for (const relativePath of values(migrationCloseout?.cleanupEvidence)) {
    if (!relativeExistingPath(relativePath)) errors.push(`migration closeout evidence path is missing or unsafe: ${relativePath}`);
  }
  if (!sameList(values(closeout.pendingNodeEvidence), unfinished)) errors.push("node closeout pending evidence must match unfinished task graph nodes");

  if (!sameList(values(objective.taskControlPolicy?.requiredUnfinishedNodes), unfinished) ||
      !sameList(values(objective.completionPolicy?.controlledBlockers), unfinished)) {
    errors.push("objective conformance unfinished projections must match unfinished task graph nodes");
  }
  requireIncludes(objective.evidence?.validators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "objective evidence.validators", errors);
  requireIncludes(objective.evidence?.tests, ["scripts/platform-sensitive-data-migration.test.mjs"], "objective evidence.tests", errors);
  requireIncludes(objective.evidence?.docs, ["docs/platform-sensitive-data-migration.md"], "objective evidence.docs", errors);

  if (!sameList(values(execution.requiredUnfinishedNodes), unfinished)) errors.push("task execution unfinished projection must match unfinished task graph nodes");
  requireIncludes(execution.requiredValidators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "task execution requiredValidators", errors);
  requireIncludes(execution.requiredTests, ["scripts/platform-sensitive-data-migration.test.mjs"], "task execution requiredTests", errors);

  const capability = values(engineering.capabilities).find((item) => item.id === "sensitive-data-protection");
  if (capability?.status !== "implemented") errors.push("sensitive-data-protection engineering capability must stay implemented after migration closeout");
  requireIncludes(capability?.evidence?.sourcePaths, ["docs/platform-sensitive-data-migration.md", sourceLockPath], "sensitive-data-protection evidence.sourcePaths", errors);
  requireIncludes(capability?.evidence?.validators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "sensitive-data-protection evidence.validators", errors);
  requireIncludes(capability?.evidence?.tests, ["scripts/platform-sensitive-data-migration.test.mjs"], "sensitive-data-protection evidence.tests", errors);
}

const evidenceTextExtensions = new Set([".json", ".log", ".md", ".text", ".txt", ".yaml", ".yml"]);

function isEvidenceTextPath(relativePath) {
  return evidenceTextExtensions.has(path.extname(relativePath).toLowerCase());
}

function trackedEvidenceFiles() {
  const result = spawnSync("git", ["ls-files", "resources/evidence", "tmp"], { cwd: repoRoot, encoding: "utf8" });
  if (result.status !== 0) return [];
  return result.stdout.split("\n").filter(Boolean).filter(isEvidenceTextPath).sort();
}

function migrationTaskReports() {
  const reportDir = path.join(repoRoot, ".superpowers/sdd");
  if (!fs.existsSync(reportDir)) return [];
  return fs.readdirSync(reportDir, { withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => {
      const match = /^sensitive-data-historical-migration-task-([1-8])(?:-[^.]+)?-report\.(?:json|log|md|text|txt|yaml|yml)$/i.exec(entry.name);
      if (match) return { filePath: path.join(reportDir, entry.name), task: match[1] };
      const legacyTaskOne = /^task-1-report\.(?:json|log|md|text|txt|yaml|yml)$/i.test(entry.name);
      return legacyTaskOne ? { filePath: path.join(reportDir, entry.name), task: "1" } : null;
    })
    .filter(Boolean)
    .sort((left, right) => left.filePath.localeCompare(right.filePath));
}

function safeEvidencePlaceholder(value) {
  const normalized = value.trim().replace(/[.;]+$/, "");
  return normalized === "" ||
    /^\$(?:\{[A-Z][A-Z0-9_]*\}|[A-Z][A-Z0-9_]*)$/.test(normalized) ||
    /^<[^>]+>$/.test(normalized) ||
    /^\{\{[^}]+\}\}$/.test(normalized) ||
    /^\[(?:redacted|placeholder)\]$/i.test(normalized) ||
    /^(?:redacted|placeholder|example|none|not-set|n\/a)$/i.test(normalized);
}

function sensitiveAssignment(source, aliases, { allowColon = true, colonRequiresQuoted = false, unquotedColonPattern = null } = {}) {
  const separators = allowColon ? "=|:" : "=";
  const pattern = new RegExp(
    `(?:^|[\\s,{;])(?:["']?(?:${aliases})["']?)\\s*(${separators})\\s*(?:"([^"\\r\\n]*)"|'([^'\\r\\n]*)'|(\\$\\{[^}\\r\\n]+\\}|[^\\s,;}\\r\\n]+))`,
    "gim",
  );
  for (const match of source.matchAll(pattern)) {
    const value = match[2] ?? match[3] ?? match[4] ?? "";
    if (match[1] === ":" && colonRequiresQuoted && match[2] === undefined && match[3] === undefined &&
        !(unquotedColonPattern?.test(value))) continue;
    if (!safeEvidencePlaceholder(value)) return true;
  }
  return false;
}

function concreteDSN(source) {
  const assignments = /(?:^|\n)\s*(?:export\s+)?(?:[A-Z][A-Z0-9_]*_DSN|DSN)\s*=\s*([^\r\n#]+)/gi;
  for (const match of source.matchAll(assignments)) {
    const raw = match[1].trim();
    const quoted = /^(["'])(.*)\1$/.exec(raw);
    if (!safeEvidencePlaceholder(quoted?.[2] ?? raw)) return true;
  }
  if (/(?:mysql|postgres(?:ql)?|oracle|kingbase(?:es)?|sqlite):\/\/[^\s`"'<>$]+/i.test(source)) return true;
  if (/(?:^|[\s="'`])[^\s:"'<>$]+:[^\s@"'<>$]+@(?:tcp|tcp4|tcp6|unix)\([^\r\n)]+\)\/[^\s`"'<>$]+/im.test(source)) return true;
  for (const line of source.split(/\r?\n/)) {
    if (!/\b(?:host|user|dbname|database)\s*=/.test(line)) continue;
    const password = /\bpassword\s*=\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|([^\s,;}\r\n]+))/i.exec(line);
    if (password && !safeEvidencePlaceholder(password[1] ?? password[2] ?? password[3] ?? "")) return true;
  }
  const keywordValues = new Map();
  const keywordAssignment = /\b(host|user|password|dbname|database)\s*=\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|(\$\{[^}\r\n]+\}|\$[A-Z][A-Z0-9_]*|<[^>\r\n]+>|[^\s,;}"'\r\n]+))/gi;
  for (const match of source.matchAll(keywordAssignment)) {
    keywordValues.set(match[1].toLowerCase(), match[2] ?? match[3] ?? match[4] ?? "");
  }
  if (keywordValues.has("host") && keywordValues.has("user") && keywordValues.has("password") &&
      (keywordValues.has("dbname") || keywordValues.has("database")) &&
      !safeEvidencePlaceholder(keywordValues.get("password"))) return true;
  return false;
}

function plainKeyAssignment(source) {
  if (sensitiveAssignment(source, "key", { allowColon: false })) return true;
  const patterns = [
    /(?:^|[{,]\s*)"key"\s*:\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|(\$\{[^}\r\n]+\}|[^\s,;}\r\n]+))/gm,
    /(?:^|\n)\s*key\s*:\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|(\$\{[^}\r\n]+\}|[^\s,;}\r\n]+))/gm,
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) {
      const value = match[1] ?? match[2] ?? match[3] ?? "";
      if (!safeEvidencePlaceholder(value)) return true;
    }
  }
  return false;
}

function singleTokenColonIdentifier(source, aliases) {
  const pattern = new RegExp(`["']?(?:${aliases})["']?\\s*:\\s*([^\\r\\n]*)`, "gim");
  for (const match of source.matchAll(pattern)) {
    const remainder = match[1].trim();
    const quoted = /^(["'])(.*?)\1(?=\s*(?:[,}\]]|#|$))/.exec(remainder);
    const value = quoted?.[2] ?? remainder;
    if (!safeEvidencePlaceholder(value) && /^\S+$/.test(value)) return true;
  }
  return false;
}

function validateEvidenceText(label, source, errors) {
  if (/\bpgo:enc:v\d+:[A-Za-z0-9+/_=-]{4,}/i.test(source)) errors.push(`${label} must not contain an encrypted value`);
  if (/\bfixture[-_](?:plaintext|secret)(?:[-_][A-Za-z0-9][A-Za-z0-9._-]*)?/i.test(source) ||
      sensitiveAssignment(source, "fixture[-_ ](?:plaintext|secret)")) {
    errors.push(`${label} must not contain fixture plaintext or secret material`);
  }

  const assignments = [
    ["ciphertext", "ciphertext|encrypted[-_ ]?value"],
    ["key material", "(?:encryption[-_ ]?|data[-_ ]?|blind[-_ ]?index[-_ ]?)key|keyring"],
    ["nonce", "nonce"],
    ["AAD", "aad"],
    ["blind index", "blind[-_ ]?index"],
    ["plaintext", "plaintext"],
    ["secret material", "secret"],
  ];
  for (const [kind, aliases] of assignments) {
    if (sensitiveAssignment(source, aliases)) errors.push(`${label} must not contain a concrete ${kind}`);
  }
  if (plainKeyAssignment(source)) errors.push(`${label} must not contain concrete key material`);
  if (concreteDSN(source)) errors.push(`${label} must not contain a concrete DSN`);
  const recordID = "record[-_ ]?id";
  if (sensitiveAssignment(source, recordID, { colonRequiresQuoted: true }) || singleTokenColonIdentifier(source, recordID)) {
    errors.push(`${label} must not contain a concrete record ID`);
  }
  const tenantID = "tenant[-_ ]?id(?:entifier)?";
  if (sensitiveAssignment(source, tenantID, { colonRequiresQuoted: true }) || singleTokenColonIdentifier(source, tenantID)) {
    errors.push(`${label} must not contain a concrete tenant ID`);
  }

  if (/\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/i.test(source)) errors.push(`${label} must not contain an email address`);
  if (/(?:^|\D)1[3-9]\d{9}(?!\d)/.test(source)) errors.push(`${label} must not contain a mainland phone number`);
  if (/(?:^|\D)\d{17}[\dXx](?![\dXx])/.test(source)) errors.push(`${label} must not contain a Chinese identity number`);
}

function addEvidenceCarrier(carriers, label, filePath) {
  const absolutePath = path.resolve(repoRoot, filePath);
  if (!fs.existsSync(absolutePath) || !fs.statSync(absolutePath).isFile()) return;
  if (!carriers.has(absolutePath)) carriers.set(absolutePath, label);
}

function evidenceCarriers(graph, closeout) {
  const carriers = new Map();
  addEvidenceCarrier(carriers, "runbook", paths.runbook);
  addEvidenceCarrier(carriers, "Task 7 report", paths.taskReport);
  for (const report of migrationTaskReports()) addEvidenceCarrier(carriers, `Task ${report.task} report`, report.filePath);

  const migrationTask = values(graph.tasks).find((task) => task.id === migrationTaskID);
  for (const relativePath of values(migrationTask?.evidence?.docs).filter(isEvidenceTextPath).sort()) {
    addEvidenceCarrier(carriers, `migration task evidence ${relativePath}`, relativePath);
  }

  const migrationCloseout = values(closeout.nodeCloseouts).find((item) => item.taskId === migrationTaskID);
  for (const relativePath of values(migrationCloseout?.cleanupEvidence).filter(isEvidenceTextPath).sort()) {
    addEvidenceCarrier(carriers, `migration closeout evidence ${relativePath}`, relativePath);
  }

  for (const relativePath of trackedEvidenceFiles()) {
    addEvidenceCarrier(carriers, `tracked evidence ${relativePath}`, relativePath);
  }
  for (const filePath of argValues("--task-evidence-file")) addEvidenceCarrier(carriers, "migration task evidence", filePath);
  for (const filePath of argValues("--closeout-evidence-file")) addEvidenceCarrier(carriers, "migration closeout evidence", filePath);
  for (const filePath of argValues("--tracked-evidence-file")) addEvidenceCarrier(carriers, "tracked evidence", filePath);
  for (const filePath of argValues("--evidence-file")) addEvidenceCarrier(carriers, "evidence artifact", filePath);
  return carriers;
}

function validateEvidenceFiles(carriers, errors) {
  for (const [filePath, label] of carriers) validateEvidenceText(label, readText(filePath), errors);
}

function validate() {
  const source = {
    model: readText(paths.model),
    cli: readText(paths.cli),
    bootstrap: readText(paths.bootstrap),
    protectionSource: readText(paths.protectionSource),
    apiMain: readText(paths.apiMain),
    gormStore: readText(paths.gormStore),
    runner: readText(paths.runner),
    escrow: readText(paths.escrow),
    httpAPI: readText(paths.httpAPI),
    openAPI: readJSON(paths.openAPI),
  };
  const governance = {
    graph: readJSON(paths.graph),
    alignment: readJSON(paths.alignment),
    goal: readJSON(paths.goal),
    closeout: readJSON(paths.closeout),
    objective: readJSON(paths.objective),
    execution: readJSON(paths.execution),
    engineering: readJSON(paths.engineering),
    sourceLock: readJSON(paths.sourceLock),
  };
  const errors = [];
  const astAnalysis = sourceASTAnalysis(errors);
  validateRunbook(readText(paths.runbook), errors);
  validateSourceContract(source, astAnalysis, errors);
  validateSourceLock(governance.sourceLock, astAnalysis, errors);
  validateGovernance(governance, errors);
  validateEvidenceFiles(evidenceCarriers(governance.graph, governance.closeout), errors);
  return errors;
}

if (process.argv.includes("--print-source-lock")) {
  const printErrors = [];
  const astAnalysis = sourceASTAnalysis(printErrors);
  if (printErrors.length > 0 || !Array.isArray(astAnalysis.fingerprints)) {
    console.error([...new Set([...printErrors, "sensitive migration source lock fingerprints must be available"])].map((error) => `- ${error}`).join("\n"));
    process.exit(1);
  }
  console.log(`${JSON.stringify(sourceLockDocument(astAnalysis.fingerprints), null, 2)}\n`);
  process.exit(0);
}

const errors = validate();
if (errors.length > 0) {
  console.error([...new Set(errors)].map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

const validatedTasks = values(readJSON(paths.graph).tasks);
const validatedImplemented = validatedTasks.filter((task) => task.status === "implemented").length;
console.log(`Validated platform sensitive data migration governance (${validatedTasks.length}/${validatedImplemented}/${validatedTasks.length - validatedImplemented})`);
