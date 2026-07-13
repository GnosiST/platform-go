import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const migrationTaskID = "sensitive-data-historical-migration";
const expectedModes = ["inventory", "dry-run", "prepare", "apply", "verify", "rehearse-restore", "rollback"];
const expectedRemainingTaskIDs = ["open-source-portability", "public-docs-community", "public-docs-site", "github-release-publication"];
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

function goFunction(source, name) {
  const signature = new RegExp(`func\\s+(?:\\([^)]*\\)\\s*)?${name}\\s*\\(`);
  const match = signature.exec(source);
  if (!match) return "";
  const start = source.indexOf("{", match.index);
  if (start === -1) return "";
  let depth = 0;
  for (let index = start; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(match.index, index + 1);
    }
  }
  return "";
}

function goCaseBlock(source, caseName) {
  const match = new RegExp(`case\\s+${caseName}\\s*:`).exec(source);
  if (!match) return "";
  const start = match.index + match[0].length;
  const rest = source.slice(start);
  const end = /\n\s*(?:case\s+|default\s*:)/.exec(rest)?.index ?? rest.length;
  return rest.slice(0, end);
}

function closingBrace(source, openingIndex) {
  let depth = 0;
  for (let index = openingIndex; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}") {
      depth -= 1;
      if (depth === 0) return index;
    }
  }
  return -1;
}

function withoutGoComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\r\n]*/g, "");
}

function acceptedDriverCases(source) {
  const gate = goFunction(source, "sensitiveMigrationGORMDriver");
  if (!gate || /\b(?:fallthrough|goto)\b/.test(gate) || /default\s*:\s*return\s+true/.test(gate)) return [];
  const functionOpening = gate.indexOf("{");
  const functionBody = gate.slice(functionOpening + 1, -1).trim();
  const switchMatch = /^switch\s+driver\s*\{/.exec(functionBody);
  if (!switchMatch) return [];
  const switchOpening = functionBody.indexOf("{", switchMatch.index);
  const switchClosing = closingBrace(functionBody, switchOpening);
  if (switchClosing === -1 || functionBody.slice(switchClosing + 1).trim() !== "") return [];
  const switchBody = functionBody.slice(switchOpening + 1, switchClosing);
  const caseClauses = [...switchBody.matchAll(/case\s+([^:]+):([\s\S]*?)(?=\n\s*(?:case\s+|default\s*:|$))/g)];
  if (caseClauses.length === 0 || caseClauses.some((match) => withoutGoComments(match[2]).trim() !== "return true")) return [];
  const defaultBody = /default\s*:([\s\S]*)$/.exec(switchBody)?.[1] ?? "";
  if (withoutGoComments(defaultBody).trim() !== "return false") return [];
  const caseOperands = caseClauses.flatMap((match) => match[1].split(",").map((operand) => operand.trim()));
  if (caseOperands.some((operand) => !/^"(?:mysql|postgres|sqlite)"$/.test(operand))) return [];
  const allCases = caseOperands.map((operand) => operand.slice(1, -1)).sort();
  if (!sameList(allCases, ["mysql", "postgres", "sqlite"])) return [];
  return allCases;
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

function validateSourceContract({ model, cli, bootstrap, protectionSource, apiMain, gormStore, runner, escrow, httpAPI, openAPI }, errors) {
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

  if (!sameList(acceptedDriverCases(bootstrap), ["mysql", "postgres", "sqlite"])) {
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

  const prepare = goFunction(gormStore, "Prepare");
  if (!prepare.includes("AutoMigrate")) errors.push("prepare must be the journal schema creation mode");
  if (gormStore.replace(prepare, "").includes("AutoMigrate")) errors.push("prepare must be the only mode that creates journal tables");
  const run = goFunction(runner, "Run");
  if (!/case ModeInventory, ModeDryRun:\s*return r\.runReadOnly/.test(run)) errors.push("inventory and dry-run must stay on the read-only runner path");
  if (!/case ModePrepare, ModeApply, ModeVerify, ModeRehearseRestore, ModeRollback:/.test(run)) {
    errors.push("prepared runner path must contain prepare, apply, verify, rehearse-restore and rollback");
  }
  const prepared = goFunction(runner, "runPrepared");
  const verifyDispatch = goCaseBlock(prepared, "ModeVerify");
  const verify = goFunction(runner, "runVerify");
  const forbiddenVerifyCall = /\.(?:Prepare|ApplyBatch|FinishRun|AutoMigrate|Protect|Reveal)\s*\(/;
  const forbiddenVerifyBodyCall = /\.(?:Prepare|StartOrResume|ApplyBatch|FinishRun|AutoMigrate|Protect|Reveal)\s*\(/;
  if (!verifyDispatch.includes("store.StartOrResume") || !verifyDispatch.includes("return r.runVerify") ||
      forbiddenVerifyCall.test(verifyDispatch) || !verify || forbiddenVerifyBodyCall.test(verify)) {
    errors.push("verify path must not call mutation or decryption boundaries");
  }
  const verifyStateLoader = goFunction(gormStore, "StartOrResume");
  if (!verifyStateLoader || /\.(?:AutoMigrate|Create|Delete|Exec|Save|Update|Updates)\s*\(/.test(verifyStateLoader)) {
    errors.push("verify state loader must stay read-only");
  }

  const protectedLoad = goFunction(protectionSource, "validateProtectedRecord");
  if (!protectedLoad || !/if\s+!dataprotection\.IsEnvelope\(envelope\)\s*\{\s*return\s+invalidSecurityField\(/.test(protectedLoad) ||
      /if\s+!dataprotection\.IsEnvelope\(envelope\)[\s\S]*?\.(?:Protect|Reveal)\s*\(/.test(protectedLoad)) {
    errors.push("ordinary Store must reject plaintext for encrypted fields");
  }

  const apiStartup = goFunction(apiMain, "main");
  if (!apiStartup || /OpenSensitiveDataMigration|NewRunner|sensitive-data-migrate/.test(apiStartup)) {
    errors.push("API startup must not call sensitive data migration entry points");
  }
  if (!/MigratedValueHash/.test(escrow) || !/migration-rollback/.test(escrow)) errors.push("rollback escrow must retain reserved context and migrated-value hash guards");

  const reportBlock = /type Report struct \{([\s\S]*?)\n\}/.exec(model)?.[1] ?? "";
  const reportFields = [...reportBlock.matchAll(/json:"([^",]+)/g)].map((match) => match[1]);
  if (!sameList(reportFields, ["runId", "mode", "status", "counts", "checkpoints", "eventChainHead"])) {
    errors.push("migration report must remain the value-free single JSON summary contract");
  }
  const cliRun = goFunction(cli, "runSensitiveDataMigration");
  const encodeIndex = cliRun.indexOf("json.NewEncoder(stdout).Encode(report)");
  const closeIndex = cliRun.lastIndexOf("session.Close()");
  if (encodeIndex === -1 || closeIndex === -1 || encodeIndex > closeIndex) {
    errors.push("migration CLI must emit one JSON report before closing storage");
  }
  if (/fmt\.Errorf|%v/.test(cliRun)) errors.push("migration CLI errors must remain normalized and value-free");

  if (/sensitive-data-migrat|sensitive_data_migrat/i.test(httpAPI)) errors.push("sensitive data migration must not expose an HTTP route");
  if (Object.keys(openAPI.paths ?? {}).some((route) => /sensitive-data-migrat/i.test(route))) {
    errors.push("sensitive data migration must not appear in Admin OpenAPI routes");
  }
}

function validateGovernance({ graph, alignment, goal, closeout, objective, execution, engineering }, errors) {
  const tasks = values(graph.tasks);
  const migrationTask = tasks.find((task) => task.id === migrationTaskID);
  const unfinished = tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
  if (tasks.length !== 45 || tasks.filter((task) => task.status === "implemented").length !== 41 || !sameList(unfinished, expectedRemainingTaskIDs)) {
    errors.push("official task graph projection must stay 45 total / 41 implemented / 4 controlled unfinished");
  }
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
  if (!sameList(values(alignment.requiredFutureTaskNodes), expectedRemainingTaskIDs)) errors.push("alignment requiredFutureTaskNodes must match the four-node remainder");
  requireIncludes(alignment.requiredValidators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "alignment requiredValidators", errors);
  requireIncludes(alignment.documents, ["docs/platform-sensitive-data-migration.md"], "alignment documents", errors);

  if (goal.taskSummary?.expectedTotal !== 45 || goal.taskSummary?.expectedImplemented !== 41 || goal.taskSummary?.expectedControlledUnfinished !== 4) {
    errors.push("goal completion taskSummary must stay 45/41/4");
  }
  if (!sameList(values(goal.completionPolicy?.requiredControlledUnfinishedNodes), expectedRemainingTaskIDs)) {
    errors.push("goal completion controlled unfinished nodes must match the four-node remainder");
  }

  const migrationCloseout = values(closeout.nodeCloseouts).find((item) => item.taskId === migrationTaskID);
  if (migrationCloseout?.status !== "closed" || migrationCloseout?.neatFreak !== true) errors.push("migration closeout must be closed with neat-freak evidence");
  if (Object.hasOwn(migrationCloseout ?? {}, "visualEvidence")) errors.push("migration closeout is non-visual and must not declare visualEvidence");
  requireIncludes(migrationCloseout?.cleanupEvidence, requiredCloseoutEvidence, "migration closeout cleanupEvidence", errors);
  for (const relativePath of values(migrationCloseout?.cleanupEvidence)) {
    if (!relativeExistingPath(relativePath)) errors.push(`migration closeout evidence path is missing or unsafe: ${relativePath}`);
  }
  if (!sameList(values(closeout.pendingNodeEvidence), expectedRemainingTaskIDs)) errors.push("node closeout pending evidence must match the four-node remainder");

  if (!sameList(values(objective.taskControlPolicy?.requiredUnfinishedNodes), expectedRemainingTaskIDs) ||
      !sameList(values(objective.completionPolicy?.controlledBlockers), expectedRemainingTaskIDs)) {
    errors.push("objective conformance unfinished projections must match the four-node remainder");
  }
  requireIncludes(objective.evidence?.validators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "objective evidence.validators", errors);
  requireIncludes(objective.evidence?.tests, ["scripts/platform-sensitive-data-migration.test.mjs"], "objective evidence.tests", errors);
  requireIncludes(objective.evidence?.docs, ["docs/platform-sensitive-data-migration.md"], "objective evidence.docs", errors);

  if (!sameList(values(execution.requiredUnfinishedNodes), expectedRemainingTaskIDs)) errors.push("task execution unfinished projection must match the four-node remainder");
  requireIncludes(execution.requiredValidators, ["scripts/validate-platform-sensitive-data-migration.mjs"], "task execution requiredValidators", errors);
  requireIncludes(execution.requiredTests, ["scripts/platform-sensitive-data-migration.test.mjs"], "task execution requiredTests", errors);

  const capability = values(engineering.capabilities).find((item) => item.id === "sensitive-data-protection");
  if (capability?.status !== "implemented") errors.push("sensitive-data-protection engineering capability must stay implemented after migration closeout");
  requireIncludes(capability?.evidence?.sourcePaths, ["docs/platform-sensitive-data-migration.md"], "sensitive-data-protection evidence.sourcePaths", errors);
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
    ["DSN", "dsn"],
  ];
  for (const [kind, aliases] of assignments) {
    if (sensitiveAssignment(source, aliases)) errors.push(`${label} must not contain a concrete ${kind}`);
  }
  if (plainKeyAssignment(source)) errors.push(`${label} must not contain concrete key material`);
  const recordID = "record[-_ ]?id";
  if (sensitiveAssignment(source, recordID, { colonRequiresQuoted: true }) || singleTokenColonIdentifier(source, recordID)) {
    errors.push(`${label} must not contain a concrete record ID`);
  }
  const tenantID = "tenant[-_ ]?id(?:entifier)?";
  if (sensitiveAssignment(source, tenantID, { colonRequiresQuoted: true }) || singleTokenColonIdentifier(source, tenantID)) {
    errors.push(`${label} must not contain a concrete tenant ID`);
  }

  if (/(?:mysql|postgres(?:ql)?|oracle|kingbase(?:es)?|sqlite):\/\/[^\s`"'<>$]+/i.test(source)) {
    errors.push(`${label} must not contain a URI DSN`);
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
  };
  const errors = [];
  validateRunbook(readText(paths.runbook), errors);
  validateSourceContract(source, errors);
  validateGovernance(governance, errors);
  validateEvidenceFiles(evidenceCarriers(governance.graph, governance.closeout), errors);
  return errors;
}

const errors = validate();
if (errors.length > 0) {
  console.error([...new Set(errors)].map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated platform sensitive data migration governance (45/41/4)");
