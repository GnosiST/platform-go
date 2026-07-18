import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-plugin-management-v1.json"));
const operationPolicyPath = path.resolve(
  repoRoot,
  argValue("--operation-policy", "resources/platform-capability-operation-policy.json"),
);
const protocolPath = path.resolve(
  repoRoot,
  argValue("--protocol", "resources/platform-human-ai-development-protocol.json"),
);
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));

const requiredDesiredSources = new Set([
  "capability-profile",
  "PLATFORM_CAPABILITIES",
  "PLATFORM_CAPABILITY_LOCK_FILE",
  "downstream-composition-root",
]);
const requiredStateFields = new Set([
  "operationMode",
  "activation",
  "source",
  "lockStatus",
  "currentCapabilities",
  "desiredCapabilities",
  "pendingRestart",
  "restartRequiredForChanges",
]);
const requiredLifecycleGates = new Set([
  "declare-desired-state",
  "contract-preflight",
  "contract-regeneration",
  "manual-restart",
  "post-restart-verification",
  "update-detection",
  "destructive-removal-review",
]);
const requiredAcceptedCombinations = new Set([
  "minimal-admin",
  "platform-default",
  "production-admin-oidc-ready",
  "all-optional-platform-without-demo",
  "optional-personnel-disabled-after-profile-removal",
  "external-business-downstream-owned",
]);
const requiredValidators = [
  "scripts/validate-platform-plugin-management-v1.mjs",
  "scripts/validate-platform-capability-operation-policy.mjs",
  "scripts/validate-platform-capability-contracts.mjs",
  "scripts/validate-platform-capability-profiles.mjs",
  "scripts/validate-external-capability-example.mjs",
];
const requiredTests = [
  "scripts/platform-plugin-management-v1.test.mjs",
  "scripts/platform-capability-operation-policy.test.mjs",
  "scripts/platform-capability-contracts.test.mjs",
  "scripts/platform-capability-profiles.test.mjs",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
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

function requireIncludes(items, requiredItems, label, errors) {
  const actual = new Set(values(items));
  for (const item of requiredItems) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function uniqueErrors(items, label) {
  const errors = [];
  const seen = new Set();
  for (const item of items) {
    if (!item) {
      errors.push(`${label} contains an empty value`);
      continue;
    }
    if (seen.has(item)) {
      errors.push(`${label} contains duplicate value ${item}`);
    }
    seen.add(item);
  }
  return errors;
}

function byID(items) {
  return new Map(values(items).map((item) => [item.id, item]));
}

function validateSourceSnippets(contract, errors) {
  for (const snippet of values(contract.requiredSourceSnippets)) {
    const sourcePath = snippet.path ?? "";
    const contains = snippet.contains ?? "";
    if (!relativeExistingPath(sourcePath)) {
      errors.push(`required source snippet path is missing or unsafe: ${sourcePath || "<missing>"}`);
      continue;
    }
    if (!contains) {
      errors.push(`required source snippet ${sourcePath} must declare contains`);
      continue;
    }
    if (!readRelativeFile(sourcePath).includes(contains)) {
      errors.push(`${sourcePath} is missing required snippet ${contains}`);
    }
  }
}

function validateRuntimePolicy(contract, errors) {
  const policy = contract.runtimePolicy ?? {};
  if (policy.operationMode !== "restart-required-desired-state") {
    errors.push("runtimePolicy.operationMode must be restart-required-desired-state");
  }
  if (policy.activation !== "manual-restart") {
    errors.push("runtimePolicy.activation must be manual-restart");
  }
  if (policy.restartRequiredForChanges !== true) {
    errors.push("runtimePolicy.restartRequiredForChanges must stay true");
  }
  if (policy.manualRestartSupported !== true) {
    errors.push("runtimePolicy.manualRestartSupported must stay true");
  }
  for (const key of [
    "runtimeHotInstall",
    "runtimeHotUninstall",
    "remoteRepositoryPull",
    "destructiveUninstall",
    "sourceRemovalSupported",
    "dataPurgeSupported",
    "webSocketRequired",
    "webSocketIntegratedInV1",
    "dirtyWorkAutoRefresh",
  ]) {
    if (policy[key] !== false) {
      errors.push(`runtimePolicy.${key} must stay false`);
    }
  }
  if (policy.progressTransport !== "http-polling") {
    errors.push("runtimePolicy.progressTransport must be http-polling");
  }
  if (policy.updateDetection !== "static-version-json-or-api-version-check") {
    errors.push("runtimePolicy.updateDetection must be static-version-json-or-api-version-check");
  }
}

function validateDesiredState(contract, errors) {
  const desiredState = contract.desiredState ?? {};
  if (!desiredState.currentStateSource || !desiredState.currentStateSource.includes("process startup")) {
    errors.push("desiredState.currentStateSource must describe process startup");
  }
  if (!desiredState.pendingRestartRule || !desiredState.pendingRestartRule.includes("pendingRestart")) {
    errors.push("desiredState.pendingRestartRule must describe pendingRestart");
  }
  requireIncludes(values(desiredState.stateFields), [...requiredStateFields], "desiredState.stateFields", errors);

  const sources = byID(desiredState.sources);
  for (const sourceID of requiredDesiredSources) {
    if (!sources.has(sourceID)) {
      errors.push(`desiredState.sources must include ${sourceID}`);
    }
  }
  for (const source of values(desiredState.sources)) {
    const prefix = `desiredState source ${source.id ?? "<missing>"}`;
    if (!source.id || !source.kind || !source.status) {
      errors.push(`${prefix} must declare id, kind and status`);
    }
    if (source.writesRuntimeSource !== false) {
      errors.push(`${prefix} must not write runtime source`);
    }
    if (source.restartRequired !== true) {
      errors.push(`${prefix} must require restart`);
    }
    if (source.path && !relativeExistingPath(source.path)) {
      errors.push(`${prefix} path is missing or unsafe: ${source.path}`);
    }
  }
}

function validateLifecycleGates(contract, errors) {
  const gates = byID(contract.lifecycleGates);
  for (const gateID of requiredLifecycleGates) {
    if (!gates.has(gateID)) {
      errors.push(`lifecycleGates must include ${gateID}`);
    }
  }
  for (const gate of values(contract.lifecycleGates)) {
    const prefix = `lifecycle gate ${gate.id ?? "<missing>"}`;
    if (!gate.id || !gate.phase) {
      errors.push(`${prefix} must declare id and phase`);
    }
    if (gate.required !== true) {
      errors.push(`${prefix} must be required`);
    }
    for (const command of values(gate.commands)) {
      if (!command.startsWith("rtk ")) {
        errors.push(`${prefix} command must use rtk prefix: ${command}`);
      }
    }
  }
  const updateGate = gates.get("update-detection");
  if (updateGate?.transport !== "http-polling") {
    errors.push("lifecycle gate update-detection must use http-polling");
  }
  const removalGate = gates.get("destructive-removal-review");
  if (removalGate?.mode !== "review-required-outside-v1") {
    errors.push("lifecycle gate destructive-removal-review must be review-required-outside-v1");
  }
  requireIncludes(
    removalGate?.prohibitedInV1,
    ["source package deletion", "persisted data purge", "runtime one-click uninstall"],
    "destructive-removal-review.prohibitedInV1",
    errors,
  );
}

function validateAcceptedCombinations(contract, operationPolicy, profiles, errors) {
  const combinations = byID(contract.acceptedCombinations);
  const operationCombinations = byID(operationPolicy.validatedCombinations);
  const operationCapabilities = byID(operationPolicy.capabilities);
  const profileIDs = new Set(values(profiles.profiles).map((profile) => profile.id));
  const nonRemovable = new Set(values(operationPolicy.nonRemovableCapabilities));

  for (const combinationID of requiredAcceptedCombinations) {
    if (!combinations.has(combinationID)) {
      errors.push(`acceptedCombinations must include ${combinationID}`);
    }
  }
  for (const combination of values(contract.acceptedCombinations)) {
    const prefix = `accepted combination ${combination.id ?? "<missing>"}`;
    if (!combination.id || !combination.type) {
      errors.push(`${prefix} must declare id and type`);
    }
    if (combination.profile && !profileIDs.has(combination.profile)) {
      errors.push(`${prefix} references missing profile ${combination.profile}`);
    }
    if (combination.fromProfile && !profileIDs.has(combination.fromProfile)) {
      errors.push(`${prefix} references missing fromProfile ${combination.fromProfile}`);
    }
    if (combination.toProfile && !profileIDs.has(combination.toProfile)) {
      errors.push(`${prefix} references missing toProfile ${combination.toProfile}`);
    }
    if (combination.operationPolicyCombination && !operationCombinations.has(combination.operationPolicyCombination)) {
      errors.push(`${prefix} references missing operation policy combination ${combination.operationPolicyCombination}`);
    }
    if (combination.capability) {
      const operation = operationCapabilities.get(combination.capability);
      if (!operation) {
        errors.push(`${prefix} references missing operation policy capability ${combination.capability}`);
        continue;
      }
      if (nonRemovable.has(combination.capability)) {
        errors.push(`${prefix} must not model non-removable capability ${combination.capability} as removable`);
      }
      for (const [actualKey, expectedKey] of [
        ["installMode", "expectedInstallMode"],
        ["disableMode", "expectedDisableMode"],
        ["uninstallMode", "expectedUninstallMode"],
        ["dataAfterDisable", "expectedDataAfterDisable"],
      ]) {
        if (combination[expectedKey] && operation[actualKey] !== combination[expectedKey]) {
          errors.push(`${prefix} expected ${actualKey} ${combination[expectedKey]} for ${combination.capability}`);
        }
      }
    }
  }
}

function validatePolicyIntegration(operationPolicy, errors) {
  requireIncludes(operationPolicy.requiredValidators, requiredValidators, "operation policy requiredValidators", errors);
  requireIncludes(operationPolicy.requiredTests, requiredTests, "operation policy requiredTests", errors);
  if (operationPolicy.operationModel?.restartRequiredForChanges !== true) {
    errors.push("operation policy operationModel.restartRequiredForChanges must stay true");
  }
  if (operationPolicy.operationModel?.remoteRepositoryPull !== false) {
    errors.push("operation policy operationModel.remoteRepositoryPull must stay false");
  }
  if (operationPolicy.operationModel?.progressTransport !== "http-polling") {
    errors.push("operation policy operationModel.progressTransport must be http-polling");
  }
  if (operationPolicy.operationModel?.webSocketRequired !== false) {
    errors.push("operation policy operationModel.webSocketRequired must stay false");
  }
}

function validateProtocolIntegration(protocol, errors) {
  requireIncludes(
    protocol.requiredValidators,
    ["scripts/validate-platform-plugin-management-v1.mjs"],
    "protocol requiredValidators",
    errors,
  );
  requireIncludes(protocol.requiredTests, ["scripts/platform-plugin-management-v1.test.mjs"], "protocol requiredTests", errors);
  requireIncludes(
    protocol.minimumAcceptanceCommands,
    ["rtk node scripts/validate-platform-plugin-management-v1.mjs"],
    "protocol minimumAcceptanceCommands",
    errors,
  );
  const lifecycleDomain = values(protocol.domains).find((domain) => domain.id === "capability-lifecycle-operations");
  if (!lifecycleDomain) {
    errors.push("protocol must include capability-lifecycle-operations domain");
    return;
  }
  requireIncludes(
    lifecycleDomain.sourceOfTruth,
    ["resources/platform-plugin-management-v1.json"],
    "capability-lifecycle-operations.sourceOfTruth",
    errors,
  );
  requireIncludes(
    lifecycleDomain.requiredValidators,
    ["scripts/validate-platform-plugin-management-v1.mjs"],
    "capability-lifecycle-operations.requiredValidators",
    errors,
  );
  requireIncludes(
    lifecycleDomain.requiredTests,
    ["scripts/platform-plugin-management-v1.test.mjs"],
    "capability-lifecycle-operations.requiredTests",
    errors,
  );
}

function validatePaths(contract, errors) {
  for (const relativePath of [
    ...values(contract.requiredValidators),
    ...values(contract.requiredTests),
    ...values(contract.requiredDocs),
  ]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`plugin management path is missing or unsafe: ${relativePath}`);
    }
  }
}

function validate() {
  const contract = readJSON(contractPath);
  const operationPolicy = readJSON(operationPolicyPath);
  const protocol = readJSON(protocolPath);
  const profiles = readJSON(profilesPath);
  const errors = [];

  if (contract.id !== "platform-plugin-management-v1") {
    errors.push("id must be platform-plugin-management-v1");
  }
  if (!contract.purpose || !contract.purpose.includes("restart-required desired-state")) {
    errors.push("purpose must describe restart-required desired-state");
  }
  if (!contract.businessBoundary?.platformRole || !contract.businessBoundary?.businessRole) {
    errors.push("businessBoundary must declare platformRole and businessRole");
  }
  requireIncludes(contract.businessBoundary?.forbids, [
    "hard-coding business menus, handlers or storage into platform core",
  ], "businessBoundary.forbids", errors);
  requireIncludes(contract.requiredValidators, requiredValidators, "requiredValidators", errors);
  requireIncludes(contract.requiredTests, requiredTests, "requiredTests", errors);
  errors.push(...uniqueErrors(values(contract.requiredValidators), "requiredValidators"));
  errors.push(...uniqueErrors(values(contract.requiredTests), "requiredTests"));
  errors.push(...uniqueErrors(values(contract.acceptedCombinations).map((combination) => combination.id), "acceptedCombinations.id"));

  validateRuntimePolicy(contract, errors);
  validateDesiredState(contract, errors);
  validateLifecycleGates(contract, errors);
  validateAcceptedCombinations(contract, operationPolicy, profiles, errors);
  validatePolicyIntegration(operationPolicy, errors);
  validateProtocolIntegration(protocol, errors);
  validatePaths(contract, errors);
  validateSourceSnippets(contract, errors);

  return { contract, errors };
}

const { contract, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

const relativeContractPath = path.relative(repoRoot, contractPath).split(path.sep).join("/");
console.log(`Validated plugin management v1 contract in ${relativeContractPath}`);
