import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const defaultReadinessPath = path.join(repoRoot, "resources", "platform-production-readiness.json");
const defaultOperationsPlanPath = path.join(repoRoot, "resources", "generated", "platform-operations-plan.json");
const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-production-readiness.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const configPath = path.resolve(repoRoot, argValue("--config", "internal/platform/config/config.go"));
const operationsPlanArgument = argValue("--operations-plan", "");
const operationsPlanPath = path.resolve(repoRoot, operationsPlanArgument || "resources/generated/platform-operations-plan.json");
const requiredOperationPolicies = [
  "config-backup-export",
  "config-import-restore",
  "database-migration",
  "token-rotation",
];
const requiredRuntimeGateSnippets = [
  "production runtime requires PLATFORM_JWT_SECRET to be changed from the development default",
  "production runtime requires PLATFORM_JWT_SECRET to be at least 32 characters",
  "production runtime requires PLATFORM_ADMIN_RESOURCE_DRIVER to be mysql, postgres, or sqlite",
  "production runtime requires PLATFORM_SESSION_DRIVER to be mysql, postgres, or sqlite",
  "production runtime requires PLATFORM_LIFECYCLE_HISTORY_DRIVER to be mysql, postgres, or sqlite",
  "production runtime requires PLATFORM_CACHE_DRIVER=redis",
  "production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY to be at least 32 bytes",
  "production runtime requires PLATFORM_RATE_LIMIT_HMAC_KEY to be distinct from phone and code HMAC keys",
  "production runtime must not enable demo-data capability",
  "production runtime requires PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
  "production runtime requires PLATFORM_PUBLIC_BASE_URL to be an absolute HTTPS origin",
  "production runtime requires a non-empty PLATFORM_TRUSTED_PROXIES policy",
  "PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv4 addresses",
  "PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv6 addresses",
];
const requiredProductionEnv = [
  "PLATFORM_PUBLIC_BASE_URL",
  "PLATFORM_TRUSTED_PROXIES",
  "PLATFORM_HTTP_MAX_BODY_BYTES",
  "PLATFORM_RATE_LIMIT_HMAC_KEY",
];
const requiredPreflightCommands = [
  "production-runtime-tests",
  "production-env-audit",
  "capability-contracts",
  "capability-profiles",
  "admin-resource-contract",
  "governance-topology",
  "form-schema-layout-slots",
  "engineering-capabilities",
  "codegen-source-writing-readiness",
  "reference-discovery",
  "reference-coverage",
  "admin-api-boundary",
  "app-client-api-boundary",
  "personnel-runtime-readiness",
  "foundation-task-graph",
  "foundation-alignment",
  "task-execution-audit",
  "goal-completion-audit",
  "node-closeout-audit",
  "objective-conformance",
  "promotion-evidence-templates",
  "refresh-token-family-promotion",
  "admin-i18n",
  "admin-ui-contracts",
  "admin-ui-contract-tests",
  "admin-build",
  "git-diff-check",
  "platform-capability-audit",
  "production-auth-hardening",
  "production-auth-promotion-review",
  "cache-invalidation",
  "deployment-topology",
  "production-readiness",
  "platform-operations-plan",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const item of required) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function readRelativeFile(relativePath) {
  return fs.readFileSync(path.resolve(repoRoot, relativePath), "utf8");
}

function normalizeRelative(relativePath) {
  return relativePath.split(path.sep).join("/");
}

function validateRequiredEnv(readiness, configSource, errors) {
  const seen = new Set();
  for (const item of values(readiness.requiredEnv)) {
    const name = item.name ?? "";
    const prefix = `required env ${name || "<missing>"}`;
    if (!name) {
      errors.push("required env is missing name");
      continue;
    }
    if (seen.has(name)) {
      errors.push(`${prefix} is duplicated`);
    }
    seen.add(name);
    if (!/^PLATFORM_[A-Z0-9_]+$/.test(name)) {
      errors.push(`${prefix} must use PLATFORM_* uppercase naming`);
    }
    if (!item.purpose) {
      errors.push(`${prefix} must declare purpose`);
    }
    if (!configSource.includes(`"${name}"`)) {
      errors.push(`${prefix} is not read by config.Load`);
    }
    const docs = values(item.docs);
    if (docs.length === 0) {
      errors.push(`${prefix} must declare docs`);
    }
    for (const docPath of docs) {
      if (!relativeExistingPath(docPath)) {
        errors.push(`${prefix} doc path is missing or unsafe: ${docPath}`);
        continue;
      }
      if (!readRelativeFile(docPath).includes(name)) {
        errors.push(`${prefix} is missing from ${docPath}`);
      }
    }
  }
  requireIncludes(values(readiness.requiredEnv).map((item) => item.name), requiredProductionEnv, "requiredEnv", errors);
}

function expectedOperationPlanOutput(readinessFilePath) {
  const relativeReadiness = normalizeRelative(path.relative(repoRoot, readinessFilePath));
  const generatorSource = readRelativeFile("scripts/generate-platform-operations-plan.mjs");
  if (!generatorSource.includes("runtimeMutation: \"disabled\"")) {
    return { error: "platform operations plan generator must keep runtimeMutation disabled" };
  }
  const relativeProductionAuth = normalizeRelative(path.relative(repoRoot, productionAuthPath));
  const result = spawnSync(process.execPath, ["scripts/generate-platform-operations-plan.mjs", "--readiness", relativeReadiness, "--production-auth", relativeProductionAuth, "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    return { error: `platform operations plan generator failed\n${result.stdout}${result.stderr}` };
  }
  return { output: result.stdout };
}

function validateRuntimeGate(readiness, configSource, errors) {
  const gate = readiness.runtimeGate ?? {};
  if (gate.source && !relativeExistingPath(gate.source)) {
    errors.push(`runtimeGate source is missing or unsafe: ${gate.source}`);
  }
  if (gate.function && !configSource.includes(gate.function)) {
    errors.push(`runtimeGate function ${gate.function} is missing from config source`);
  }
  for (const snippet of values(gate.requiredSnippets)) {
    if (!configSource.includes(snippet)) {
      errors.push(`runtimeGate source is missing snippet ${snippet}`);
    }
  }
  requireIncludes(gate.requiredSnippets, requiredRuntimeGateSnippets, "runtimeGate.requiredSnippets", errors);
  const tests = values(gate.tests);
  if (tests.length === 0) {
    errors.push("runtimeGate must declare at least one test");
  }
  for (const test of tests) {
    if (!relativeExistingPath(test.path)) {
      errors.push(`runtimeGate test path is missing or unsafe: ${test.path}`);
      continue;
    }
    const source = readRelativeFile(test.path);
    for (const name of values(test.names)) {
      if (!source.includes(name)) {
        errors.push(`runtimeGate test ${test.path} is missing ${name}`);
      }
    }
  }
}

function validateProductionEnvAudit(readiness, errors) {
  const audit = readiness.productionEnvAudit ?? {};
  if (audit.validator !== "scripts/validate-platform-production-env.mjs") {
    errors.push("productionEnvAudit.validator must be scripts/validate-platform-production-env.mjs");
  }
  if (audit.defaultEnvFile !== "deploy/env/production.example.env") {
    errors.push("productionEnvAudit.defaultEnvFile must be deploy/env/production.example.env");
  }
  if (!String(audit.strictCommand ?? "").includes("--strict-secrets")) {
    errors.push("productionEnvAudit.strictCommand must include --strict-secrets");
  }
  for (const relativePath of [audit.validator, audit.defaultEnvFile]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`productionEnvAudit path is missing or unsafe: ${relativePath}`);
    }
  }
  for (const docPath of values(audit.docs)) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`productionEnvAudit doc path is missing or unsafe: ${docPath}`);
      continue;
    }
    const source = readRelativeFile(docPath);
    if (!source.includes("validate-platform-production-env.mjs")) {
      errors.push(`productionEnvAudit is missing from ${docPath}`);
    }
  }
  for (const snippet of values(audit.requiredSourceSnippets)) {
    const snippetPath = snippet.path ?? "";
    const contains = snippet.contains ?? "";
    if (!relativeExistingPath(snippetPath)) {
      errors.push(`productionEnvAudit required snippet path is missing or unsafe: ${snippetPath}`);
      continue;
    }
    if (!contains) {
      errors.push(`productionEnvAudit required snippet for ${snippetPath} is missing contains`);
      continue;
    }
    if (!readRelativeFile(snippetPath).includes(contains)) {
      errors.push(`${snippetPath} must include ${contains}`);
    }
  }
  const preflight = values(readiness.preflightCommands).find((command) => command.id === "production-env-audit");
  if (!preflight) {
    errors.push("production readiness preflight must include production-env-audit");
  } else if (preflight.command !== "rtk node scripts/validate-platform-production-env.mjs") {
    errors.push("production-env-audit preflight command must run scripts/validate-platform-production-env.mjs");
  }
}

function validatePreflightRunner(readiness, errors) {
  const runner = readiness.preflightRunner ?? {};
  const expectedScript = "scripts/run-platform-production-preflight.mjs";
  const commandPrefix = `rtk node ${expectedScript}`;

  if (runner.script !== expectedScript) {
    errors.push(`preflightRunner.script must be ${expectedScript}`);
  }
  if (!relativeExistingPath(runner.script)) {
    errors.push("preflightRunner.script path is missing or unsafe");
  }
  const commandExpectations = [
    ["listCommand", `${commandPrefix} --list`, "--list"],
    ["dryRunCommand", commandPrefix, ""],
    ["policyCommand", `${commandPrefix} --policy <policy-id>`, "--policy <policy-id>"],
    ["commandCommand", `${commandPrefix} --command <command-id>`, "--command <command-id>"],
    ["runCommand", `${commandPrefix} --run`, "--run"],
  ];
  for (const [field, expected, requiredSnippet] of commandExpectations) {
    const actual = runner[field] ?? "";
    if (actual !== expected) {
      errors.push(`preflightRunner.${field} must use ${expected}`);
    }
    if (requiredSnippet && !actual.includes(requiredSnippet)) {
      errors.push(`preflightRunner.${field} must include ${requiredSnippet}`);
    }
  }
  if (!String(runner.strictEnvCommand ?? "").startsWith(`${commandPrefix} --command production-env-audit`)) {
    errors.push("preflightRunner.strictEnvCommand must select production-env-audit");
  }
  if (!String(runner.strictEnvCommand ?? "").includes("--strict-env-file")) {
    errors.push("preflightRunner.strictEnvCommand must include --strict-env-file");
  }
  for (const docPath of values(runner.docs)) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`preflightRunner doc path is missing or unsafe: ${docPath}`);
      continue;
    }
    if (!readRelativeFile(docPath).includes(expectedScript)) {
      errors.push(`preflightRunner is missing from ${docPath}`);
    }
  }
  for (const testPath of values(runner.tests)) {
    if (!relativeExistingPath(testPath)) {
      errors.push(`preflightRunner test path is missing or unsafe: ${testPath}`);
      continue;
    }
    if (!readRelativeFile(testPath).includes("platform production preflight runner")) {
      errors.push(`preflightRunner test ${testPath} must cover the platform production preflight runner`);
    }
  }
}

function commandPathCandidates(command) {
  return command
    .split(/\s+/)
    .map((token) => token.replace(/^['"]|['"]$/g, ""))
    .filter((token) => token.startsWith("scripts/") || token.startsWith("resources/") || token.startsWith("docs/") || token.startsWith("internal/") || token.startsWith("cmd/") || token.startsWith("./"));
}

function normalizeCommandPath(candidate) {
  if (candidate.startsWith("./")) {
    return candidate.slice(2);
  }
  return candidate;
}

function validatePreflightCommands(readiness, errors) {
  const commands = values(readiness.preflightCommands);
  const seen = new Set();
  if (commands.length === 0) {
    errors.push("preflightCommands must not be empty");
  }
  for (const item of commands) {
    const id = item.id ?? "";
    if (!id) {
      errors.push("preflight command is missing id");
      continue;
    }
    if (seen.has(id)) {
      errors.push(`preflight command ${id} is duplicated`);
    }
    seen.add(id);
    if (!item.command) {
      errors.push(`preflight command ${id} must declare command`);
    }
    if (!item.command.startsWith("rtk ")) {
      errors.push(`preflight command ${id} must use rtk prefix`);
    }
    if (!item.purpose) {
      errors.push(`preflight command ${id} must declare purpose`);
    }
    for (const candidate of commandPathCandidates(item.command)) {
      const relativePath = normalizeCommandPath(candidate);
      if (!relativeExistingPath(relativePath)) {
        errors.push(`preflight command ${id} references missing path ${relativePath}`);
      }
    }
  }
  for (const requiredCommand of requiredPreflightCommands) {
    if (!seen.has(requiredCommand)) {
      errors.push(`missing required preflight command ${requiredCommand}`);
    }
  }
  const productionAuth = commands.find((command) => command.id === "production-auth-hardening");
  if (productionAuth && !productionAuth.purpose.includes("Admin OIDC")) {
    errors.push("production-auth-hardening purpose must mention Admin OIDC");
  }
}

function validatePolicyPreflightRequirements(readiness, errors) {
  const requirements = values(readiness.policyPreflightRequirements);
  const requirementIDs = new Set();
  const commandIDs = new Set(values(readiness.preflightCommands).map((command) => command.id));
  if (requirements.length === 0) {
    errors.push("policyPreflightRequirements must not be empty");
  }
  for (const requirement of requirements) {
    const policyID = requirement.policy ?? "";
    const prefix = `policy preflight requirement ${policyID || "<missing>"}`;
    if (!policyID) {
      errors.push("policy preflight requirement is missing policy");
      continue;
    }
    if (requirementIDs.has(policyID)) {
      errors.push(`${prefix} is duplicated`);
    }
    requirementIDs.add(policyID);
    const requiredCommands = values(requirement.requiredCommands);
    if (requiredCommands.length === 0) {
      errors.push(`${prefix} must declare requiredCommands`);
    }
    if (!requirement.reason) {
      errors.push(`${prefix} must declare reason`);
    }
    for (const commandID of requiredCommands) {
      if (!commandIDs.has(commandID)) {
        errors.push(`${prefix} references unknown preflight command ${commandID}`);
      }
    }
  }
  for (const requiredPolicy of requiredOperationPolicies) {
    if (!requirementIDs.has(requiredPolicy)) {
      errors.push(`missing policy preflight requirement ${requiredPolicy}`);
    }
  }
  return new Map(requirements.map((requirement) => [requirement.policy, values(requirement.requiredCommands)]));
}

function validateOperationPolicies(readiness, errors) {
  const policies = values(readiness.operationPolicies);
  const policiesByID = new Map();
  const requiredPolicyPreflightCommands = validatePolicyPreflightRequirements(readiness, errors);
  for (const policy of policies) {
    const id = policy.id ?? "";
    const prefix = `operation policy ${id || "<missing>"}`;
    if (!id) {
      errors.push("operation policy is missing id");
      continue;
    }
    if (policiesByID.has(id)) {
      errors.push(`${prefix} is duplicated`);
    }
    policiesByID.set(id, policy);
    if (!policy.purpose) {
      errors.push(`${prefix} must declare purpose`);
    }
    const docs = values(policy.docs);
    if (docs.length === 0) {
      errors.push(`${prefix} must declare docs`);
    }
    for (const docPath of docs) {
      if (!relativeExistingPath(docPath)) {
        errors.push(`${prefix} doc path is missing or unsafe: ${docPath}`);
        continue;
      }
      const source = readRelativeFile(docPath);
      if (!source.includes(id)) {
        errors.push(`${prefix} is missing from ${docPath}`);
      }
    }
    const preflightCommands = values(policy.preflightCommands);
    if (preflightCommands.length === 0) {
      errors.push(`${prefix} must declare preflightCommands`);
    }
    for (const commandID of preflightCommands) {
      if (!values(readiness.preflightCommands).some((command) => command.id === commandID)) {
        errors.push(`${prefix} references unknown preflight command ${commandID}`);
      }
    }
    for (const requiredCommandID of values(requiredPolicyPreflightCommands.get(id))) {
      if (!preflightCommands.includes(requiredCommandID)) {
        errors.push(`${prefix} must include required preflight command ${requiredCommandID}`);
      }
    }
    if (policy.requiresHumanReview !== true) {
      errors.push(`${prefix} must require human review`);
    }
    if (values(policy.prohibitedActions).length === 0) {
      errors.push(`${prefix} must declare prohibitedActions`);
    }
    if (!policy.rollbackRequirement) {
      errors.push(`${prefix} must declare rollbackRequirement`);
    }
    if (!policy.auditRequirement) {
      errors.push(`${prefix} must declare auditRequirement`);
    }
    if (id === "token-rotation") {
      if (!policy.purpose.includes("OIDC client credentials")) {
        errors.push("token-rotation purpose must mention OIDC client credentials");
      }
      if (!policy.rollbackRequirement.includes("OIDC provider rollback")) {
        errors.push("token-rotation rollbackRequirement must mention OIDC provider rollback");
      }
      if (!values(policy.prohibitedActions).includes("promote Admin OIDC without production-like rehearsal, six-viewport browser acceptance and cleanup evidence")) {
        errors.push("token-rotation prohibitedActions must include production-like Admin OIDC evidence gate");
      }
    }
  }
  for (const requiredPolicy of requiredOperationPolicies) {
    if (!policiesByID.has(requiredPolicy)) {
      errors.push(`missing required operation policy ${requiredPolicy}`);
    }
  }
}

function sameSet(actual, expected) {
  if (new Set(actual).size !== actual.length || new Set(expected).size !== expected.length) {
    return false;
  }
  return JSON.stringify([...actual].sort()) === JSON.stringify([...expected].sort());
}

function sameJSON(actual, expected) {
  return JSON.stringify(actual ?? {}) === JSON.stringify(expected ?? {});
}

function validateProviderPromotionPlan(plan, productionAuth, errors) {
  const source = normalizeRelative(path.relative(repoRoot, productionAuthPath));
  if (plan.productionAuthHardeningSource !== source) {
    errors.push(`platform operations plan productionAuthHardeningSource must point to ${source}`);
  }
  const matrix = productionAuth.providerPromotionMatrix ?? {};
  const planMatrix = plan.providerPromotionMatrix ?? {};
  if (planMatrix.source !== source) {
    errors.push(`platform operations plan providerPromotionMatrix.source must point to ${source}`);
  }
  if (planMatrix.defaultPolicy !== matrix.defaultPolicy) {
    errors.push("platform operations plan providerPromotionMatrix.defaultPolicy must match production auth hardening contract");
  }
  if (!sameSet(values(planMatrix.newProviderRequirements).sort(), values(matrix.newProviderRequirements).sort())) {
    errors.push("platform operations plan providerPromotionMatrix.newProviderRequirements must match production auth hardening contract");
  }
  const expectedProviders = values(matrix.providers);
  const planProviders = values(planMatrix.providers);
  if (plan.summary?.providerPromotionCount !== expectedProviders.length) {
    errors.push(`platform operations plan summary.providerPromotionCount must be ${expectedProviders.length}`);
  }
  const expectedOptionalCount = expectedProviders.filter((provider) => provider.productionUsage === "optional-production-provider").length;
  if (plan.summary?.optionalProductionProviderCount !== expectedOptionalCount) {
    errors.push(`platform operations plan summary.optionalProductionProviderCount must be ${expectedOptionalCount}`);
  }
  if (!sameSet(planProviders.map((provider) => provider.id).sort(), expectedProviders.map((provider) => provider.id).sort())) {
    errors.push("platform operations plan providerPromotionMatrix.providers must match production auth hardening contract");
  }
  const planProviderByID = new Map(planProviders.map((provider) => [provider.id, provider]));
  for (const expected of expectedProviders) {
    const actual = planProviderByID.get(expected.id);
    if (!actual) {
      continue;
    }
    for (const field of ["capability", "kind", "productionUsage", "adapterBoundary"]) {
      if (actual[field] !== expected[field]) {
        errors.push(`platform operations plan provider ${expected.id} ${field} must match production auth hardening contract`);
      }
    }
    if (!sameSet(values(actual.audiences).sort(), values(expected.audiences).sort())) {
      errors.push(`platform operations plan provider ${expected.id} audiences must match production auth hardening contract`);
    }
    if (!sameSet(values(actual.configKeys).sort(), values(expected.configKeys).sort())) {
      errors.push(`platform operations plan provider ${expected.id} configKeys must match production auth hardening contract`);
    }
    if (!sameSet(values(actual.requiredControls).sort(), values(expected.requiredControls).sort())) {
      errors.push(`platform operations plan provider ${expected.id} requiredControls must match production auth hardening contract`);
    }
    for (const field of [
      "requiresSecretOwner",
      "rotationRunbookRequired",
      "subjectRedactionRequired",
      "unconfiguredProviderRejectionRequired",
      "errorNormalizationRequired",
      "productionLikeRehearsalRequired",
      "rawCredentialExposureAllowed",
      "rawSubjectExposureAllowed",
    ]) {
      if (actual[field] !== expected[field]) {
        errors.push(`platform operations plan provider ${expected.id} ${field} must match production auth hardening contract`);
      }
    }
    if (actual.rawCredentialExposureAllowed !== false) {
      errors.push(`platform operations plan provider ${expected.id} rawCredentialExposureAllowed must stay false`);
    }
    if (actual.rawSubjectExposureAllowed !== false) {
      errors.push(`platform operations plan provider ${expected.id} rawSubjectExposureAllowed must stay false`);
    }
  }
}

function validateProductionPromotionApprovalPlan(plan, productionAuth, errors) {
  const source = normalizeRelative(path.relative(repoRoot, productionAuthPath));
  const expected = productionAuth.productionPromotionApprovalPackage ?? {};
  const actual = plan.productionPromotionApprovalPackage ?? {};
  if (actual.source !== source) {
    errors.push(`platform operations plan productionPromotionApprovalPackage.source must point to ${source}`);
  }
  for (const field of ["status", "sourceOfTruth", "defaultRuntimeMutation"]) {
    if (actual[field] !== expected[field]) {
      errors.push(`platform operations plan productionPromotionApprovalPackage.${field} must match production auth hardening contract`);
    }
  }
  for (const field of ["mustNotEnableRefreshTokenFamily", "mustNotEnableUnreviewedProvider"]) {
    if ((actual[field] === true) !== (expected[field] === true)) {
      errors.push(`platform operations plan productionPromotionApprovalPackage.${field} must match production auth hardening contract`);
    }
  }
  if (!sameSet(values(actual.requiredApprovals).sort(), values(expected.requiredApprovals).sort())) {
    errors.push("platform operations plan productionPromotionApprovalPackage.requiredApprovals must match production auth hardening contract");
  }
  if (!sameSet(values(actual.completedEvidence).sort(), values(expected.completedEvidence).sort())) {
    errors.push("platform operations plan productionPromotionApprovalPackage.completedEvidence must match production auth hardening contract");
  }
  if (!sameSet(values(actual.prohibitedEvidence).sort(), values(expected.prohibitedEvidence).sort())) {
    errors.push("platform operations plan productionPromotionApprovalPackage.prohibitedEvidence must match production auth hardening contract");
  }
  const schemaFields = ["requiredFields", "approvalRules", "forbiddenFields"];
  for (const field of schemaFields) {
    if (!sameSet(values(actual.completedEvidenceSchema?.[field]).sort(), values(expected.completedEvidenceSchema?.[field]).sort())) {
      errors.push(`platform operations plan productionPromotionApprovalPackage.completedEvidenceSchema.${field} must match production auth hardening contract`);
    }
  }
  if (!sameJSON(actual.completedEvidenceSchema?.artifactHashPolicy, expected.completedEvidenceSchema?.artifactHashPolicy)) {
    errors.push("platform operations plan productionPromotionApprovalPackage.completedEvidenceSchema.artifactHashPolicy must match production auth hardening contract");
  }
  if (!sameJSON(actual.completedEvidenceSchema?.artifactURIPolicy, expected.completedEvidenceSchema?.artifactURIPolicy)) {
    errors.push("platform operations plan productionPromotionApprovalPackage.completedEvidenceSchema.artifactURIPolicy must match production auth hardening contract");
  }
  const normalizeEvidence = (items) =>
    values(items)
      .map((item) => `${item.id ?? ""}|${item.owner ?? ""}|${item.evidenceKind ?? ""}|${item.description ?? ""}`)
      .sort();
  if (!sameSet(normalizeEvidence(actual.requiredEvidence), normalizeEvidence(expected.requiredEvidence))) {
    errors.push("platform operations plan productionPromotionApprovalPackage.requiredEvidence must match production auth hardening contract");
  }
  if (plan.summary?.productionPromotionRequiredEvidenceCount !== values(expected.requiredEvidence).length) {
    errors.push(`platform operations plan summary.productionPromotionRequiredEvidenceCount must be ${values(expected.requiredEvidence).length}`);
  }
}

function validateOperationsPlan(readiness, productionAuth, errors) {
  if (!fs.existsSync(operationsPlanPath)) {
    errors.push(`platform operations plan is missing: ${normalizeRelative(path.relative(repoRoot, operationsPlanPath))}`);
    return;
  }
  const plan = readJSON(operationsPlanPath);
  const planLabel = `platform operations plan ${normalizeRelative(path.relative(repoRoot, operationsPlanPath))}`;

  if (plan.generatedBy !== "scripts/generate-platform-operations-plan.mjs") {
    errors.push(`${planLabel} must be generated by scripts/generate-platform-operations-plan.mjs`);
  }
  if (plan.source !== normalizeRelative(path.relative(repoRoot, readinessPath))) {
    errors.push(`${planLabel} source must point to ${normalizeRelative(path.relative(repoRoot, readinessPath))}`);
  }
  if (plan.mode?.dryRun !== true) {
    errors.push("platform operations plan must run in dryRun mode");
  }
  if (plan.mode?.runtimeMutation !== "disabled") {
    errors.push("platform operations plan must keep runtimeMutation disabled");
  }
  if (plan.mode?.sourceWriting !== "disabled") {
    errors.push("platform operations plan must keep sourceWriting disabled");
  }
  if (JSON.stringify(plan.preflightRunner ?? {}) !== JSON.stringify(readiness.preflightRunner ?? {})) {
    errors.push("platform operations plan preflightRunner must match production readiness preflightRunner");
  }

  const readinessPolicyIDs = values(readiness.operationPolicies).map((policy) => policy.id).sort();
  const planPolicyIDs = values(plan.policies).map((policy) => policy.id).sort();
  if (!sameSet(planPolicyIDs, readinessPolicyIDs)) {
    errors.push("platform operations plan policies must match production readiness operationPolicies");
  }
  if (plan.summary?.policyCount !== readinessPolicyIDs.length) {
    errors.push(`platform operations plan summary.policyCount must be ${readinessPolicyIDs.length}`);
  }

  const readinessCommandIDs = new Set(values(readiness.preflightCommands).map((command) => command.id));
  const readinessRequirementIDs = new Map(values(readiness.policyPreflightRequirements).map((requirement) => [requirement.policy, values(requirement.requiredCommands)]));
  const planCommandIDs = values(plan.preflightCommands).map((command) => command.id).sort();
  if (!sameSet(planCommandIDs, Array.from(readinessCommandIDs).sort())) {
    errors.push("platform operations plan preflightCommands must match production readiness preflightCommands");
  }
  for (const policy of values(plan.policies)) {
    const requiredCommands = values(readinessRequirementIDs.get(policy.id));
    if (!sameSet(values(policy.requiredPreflightCommands).sort(), requiredCommands.sort())) {
      errors.push(`platform operations plan policy ${policy.id} requiredPreflightCommands must match production readiness policyPreflightRequirements`);
    }
    const missingRequiredCommands = requiredCommands.filter((commandID) => !values(policy.preflightCommands).includes(commandID));
    if (!sameSet(values(policy.missingRequiredPreflightCommands).sort(), missingRequiredCommands.sort())) {
      errors.push(`platform operations plan policy ${policy.id} missingRequiredPreflightCommands must match policy coverage`);
    }
    for (const commandID of values(policy.preflightCommands)) {
      if (!readinessCommandIDs.has(commandID)) {
        errors.push(`platform operations plan policy ${policy.id} references unknown preflight command ${commandID}`);
      }
    }
    if (policy.requiresHumanReview !== true) {
      errors.push(`platform operations plan policy ${policy.id} must require human review`);
    }
    if (values(policy.prohibitedActions).length === 0) {
      errors.push(`platform operations plan policy ${policy.id} must include prohibited actions`);
    }
    if (!policy.rollbackRequirement) {
      errors.push(`platform operations plan policy ${policy.id} must include rollback requirement`);
    }
    if (!policy.auditRequirement) {
      errors.push(`platform operations plan policy ${policy.id} must include audit requirement`);
    }
  }
  validateProviderPromotionPlan(plan, productionAuth, errors);
  validateProductionPromotionApprovalPlan(plan, productionAuth, errors);

  const expected = expectedOperationPlanOutput(readinessPath);
  if (expected.error) {
    errors.push(expected.error);
    return;
  }
  const actual = fs.readFileSync(operationsPlanPath, "utf8");
  if (actual !== expected.output) {
    errors.push("resources/generated/platform-operations-plan.json is stale; rerun scripts/generate-platform-operations-plan.mjs");
  }
}

function validate() {
  const readiness = readJSON(readinessPath);
  const productionAuth = readJSON(productionAuthPath);
  const configSource = fs.readFileSync(configPath, "utf8");
  const errors = [];
  if (!readiness.purpose) {
    errors.push("production readiness contract must declare purpose");
  }
  validateRuntimeGate(readiness, configSource, errors);
  validateProductionEnvAudit(readiness, errors);
  validatePreflightRunner(readiness, errors);
  validateRequiredEnv(readiness, configSource, errors);
  validatePreflightCommands(readiness, errors);
  validateOperationPolicies(readiness, errors);
  if (readinessPath === defaultReadinessPath || operationsPlanArgument) {
    validateOperationsPlan(readiness, productionAuth, errors);
  }
  return { readiness, errors };
}

const { readiness, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

const count =
  values(readiness.requiredEnv).length +
  values(readiness.preflightCommands).length +
  values(readiness.runtimeGate?.tests).length +
  values(readiness.operationPolicies).length +
  (readiness.preflightRunner ? 1 : 0);
console.log(`Validated ${count} production readiness checks in ${path.relative(repoRoot, readinessPath)}`);
