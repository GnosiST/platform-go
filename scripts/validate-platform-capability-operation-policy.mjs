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

const policyPath = path.resolve(repoRoot, argValue("--policy", "resources/platform-capability-operation-policy.json"));
const contractsPath = path.resolve(repoRoot, argValue("--contracts", "resources/platform-capability-contracts.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));

const requiredInstallModes = new Set([
  "required-foundation",
  "default-profile",
  "development-default",
  "profile-only",
  "external-business",
]);
const requiredDisableModes = new Set(["blocked-foundation", "config-remove-retain-data", "downstream-owned"]);
const requiredUninstallModes = new Set([
  "not-supported-foundation",
  "disable-only-retain-data",
  "downstream-owned-reviewed-removal",
]);
const requiredDataModes = new Set(["required-foundation", "retained-unreachable", "downstream-owned"]);
const requiredValidators = [
  "scripts/validate-platform-capability-operation-policy.mjs",
  "scripts/validate-platform-capability-contracts.mjs",
  "scripts/validate-platform-capability-profiles.mjs",
  "scripts/validate-external-capability-example.mjs",
];
const requiredTests = [
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

function requireAllowedModes(policy, key, required, errors) {
  const actual = new Set(values(policy.allowedModes?.[key]));
  for (const mode of required) {
    if (!actual.has(mode)) {
      errors.push(`allowedModes.${key} must include ${mode}`);
    }
  }
}

function profileByID(profiles) {
  return new Map(values(profiles.profiles).map((profile) => [profile.id, profile]));
}

function contractByID(contracts) {
  return new Map(values(contracts.capabilities).map((capability) => [capability.id, capability]));
}

function runAudit(combination, profilesByID) {
  let capabilities = values(combination.capabilities);
  if (combination.profile) {
    const profile = profilesByID.get(combination.profile);
    if (!profile) {
      return { error: `combination ${combination.id} references missing profile ${combination.profile}` };
    }
    capabilities = values(profile.capabilities);
  }
  if (capabilities.length === 0) {
    return { error: `combination ${combination.id} must declare capabilities or profile` };
  }
  const result = spawnSync("go", ["run", "./cmd/platform-contracts", "audit", "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      PLATFORM_CAPABILITIES: capabilities.join(","),
    },
  });
  if (result.status !== 0) {
    return { error: `combination ${combination.id} failed capability audit\n${result.stdout}${result.stderr}` };
  }
  try {
    return { audit: JSON.parse(result.stdout), capabilities };
  } catch (error) {
    return { error: `combination ${combination.id} audit output is not valid JSON: ${error.message}` };
  }
}

function flattenedAuditSet(audit, key) {
  const items = [];
  for (const capability of values(audit.capabilities)) {
    for (const item of values(capability[key])) {
      items.push(item);
    }
  }
  return new Set(items);
}

function validateCombination(combination, profilesByID) {
  const errors = [];
  const prefix = `combination ${combination.id ?? "<missing>"}`;
  if (!combination.id) {
    return ["validated combination is missing id"];
  }
  if (!combination.purpose) {
    errors.push(`${prefix} must declare purpose`);
  }
  const { audit, error } = runAudit(combination, profilesByID);
  if (error) {
    return [error];
  }
  const capabilityIDs = new Set(values(audit.capabilities).map((capability) => capability.id));
  const resourceIDs = flattenedAuditSet(audit, "adminResources");
  const routeIDs = flattenedAuditSet(audit, "appRoutes");
  const authProviders = flattenedAuditSet(audit, "authProviders");

  for (const capability of values(combination.expectedCapabilities)) {
    if (!capabilityIDs.has(capability)) {
      errors.push(`${prefix} missing expected capability ${capability}`);
    }
  }
  for (const capability of values(combination.expectedExcludedCapabilities)) {
    if (capabilityIDs.has(capability)) {
      errors.push(`${prefix} must exclude capability ${capability}`);
    }
  }
  for (const resource of values(combination.expectedAdminResources)) {
    if (!resourceIDs.has(resource)) {
      errors.push(`${prefix} missing expected admin resource ${resource}`);
    }
  }
  for (const route of values(combination.expectedAppRoutes)) {
    if (!routeIDs.has(route)) {
      errors.push(`${prefix} missing expected app route ${route}`);
    }
  }
  for (const provider of values(combination.expectedAuthProviders)) {
    if (!authProviders.has(provider)) {
      errors.push(`${prefix} missing expected auth provider ${provider}`);
    }
  }
  return errors;
}

function validateSourceSnippets(policy, errors) {
  for (const snippet of values(policy.requiredSourceSnippets)) {
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

function validatePolicyCapability(operation, contract, nonRemovable) {
  const errors = [];
  const prefix = `capability ${operation.id ?? "<missing>"}`;
  if (!operation.id) {
    return ["operation capability is missing id"];
  }
  if (!operation.reason) {
    errors.push(`${prefix} must declare reason`);
  }
  if (!requiredInstallModes.has(operation.installMode)) {
    errors.push(`${prefix} has unsupported installMode ${operation.installMode ?? "<missing>"}`);
  }
  if (!requiredDisableModes.has(operation.disableMode)) {
    errors.push(`${prefix} has unsupported disableMode ${operation.disableMode ?? "<missing>"}`);
  }
  if (!requiredUninstallModes.has(operation.uninstallMode)) {
    errors.push(`${prefix} has unsupported uninstallMode ${operation.uninstallMode ?? "<missing>"}`);
  }
  if (!requiredDataModes.has(operation.dataAfterDisable)) {
    errors.push(`${prefix} has unsupported dataAfterDisable ${operation.dataAfterDisable ?? "<missing>"}`);
  }
  if (!contract) {
    return errors;
  }

  if (contract.classification === "foundation-core") {
    if (!nonRemovable.has(operation.id)) {
      errors.push(`${prefix} is foundation-core and must be listed in nonRemovableCapabilities`);
    }
    if (operation.installMode !== "required-foundation") {
      errors.push(`${prefix} is foundation-core and must use installMode required-foundation`);
    }
    if (operation.disableMode !== "blocked-foundation") {
      errors.push(`${prefix} is foundation-core and must use disableMode blocked-foundation`);
    }
    if (operation.uninstallMode !== "not-supported-foundation") {
      errors.push(`${prefix} is foundation-core and must use uninstallMode not-supported-foundation`);
    }
    if (operation.dataAfterDisable !== "required-foundation") {
      errors.push(`${prefix} is foundation-core and must use dataAfterDisable required-foundation`);
    }
  }
  if (contract.profilePolicy === "default-enabled" && contract.classification !== "foundation-core") {
    if (operation.installMode !== "default-profile") {
      errors.push(`${prefix} is default-enabled and must use installMode default-profile`);
    }
    if (operation.disableMode !== "config-remove-retain-data") {
      errors.push(`${prefix} is default-enabled and must use disableMode config-remove-retain-data`);
    }
    if (operation.uninstallMode !== "disable-only-retain-data") {
      errors.push(`${prefix} is default-enabled and must use uninstallMode disable-only-retain-data`);
    }
  }
  if (contract.profilePolicy === "default-development-only") {
    if (operation.installMode !== "development-default") {
      errors.push(`${prefix} is development-only and must use installMode development-default`);
    }
    if (operation.productionForbidden !== true) {
      errors.push(`${prefix} is development-only and must declare productionForbidden=true`);
    }
    if (operation.uninstallMode !== "disable-only-retain-data") {
      errors.push(`${prefix} is development-only and must use uninstallMode disable-only-retain-data`);
    }
  }
  if (contract.profilePolicy === "profile-only") {
    if (operation.installMode !== "profile-only") {
      errors.push(`${prefix} is profile-only and must use installMode profile-only`);
    }
    if (operation.disableMode !== "config-remove-retain-data") {
      errors.push(`${prefix} is profile-only and must use disableMode config-remove-retain-data`);
    }
    if (operation.uninstallMode !== "disable-only-retain-data") {
      errors.push(`${prefix} is profile-only and must use uninstallMode disable-only-retain-data`);
    }
  }
  if (contract.profilePolicy === "business-external-only") {
    if (operation.installMode !== "external-business") {
      errors.push(`${prefix} is business-external-only and must use installMode external-business`);
    }
    if (operation.disableMode !== "downstream-owned") {
      errors.push(`${prefix} is business-external-only and must use disableMode downstream-owned`);
    }
    if (operation.uninstallMode !== "downstream-owned-reviewed-removal") {
      errors.push(`${prefix} is business-external-only and must use uninstallMode downstream-owned-reviewed-removal`);
    }
    if (operation.dataAfterDisable !== "downstream-owned") {
      errors.push(`${prefix} is business-external-only and must use dataAfterDisable downstream-owned`);
    }
  }
  return errors;
}

function validate() {
  const policy = readJSON(policyPath);
  const contracts = readJSON(contractsPath);
  const profiles = readJSON(profilesPath);
  const errors = [];
  const contractsByID = contractByID(contracts);
  const profilesByID = profileByID(profiles);
  const operations = values(policy.capabilities);
  const operationsByID = new Map(operations.map((capability) => [capability.id, capability]));
  const nonRemovable = new Set(values(policy.nonRemovableCapabilities));

  if (!policy.purpose || !policy.purpose.includes("install, disable, uninstall")) {
    errors.push("operation policy purpose must describe install, disable and uninstall");
  }
  if (policy.operationModel?.runtimeHotInstall !== false) {
    errors.push("operationModel.runtimeHotInstall must stay false");
  }
  if (policy.operationModel?.runtimeHotUninstall !== false) {
    errors.push("operationModel.runtimeHotUninstall must stay false");
  }
  if (policy.operationModel?.destructiveUninstallRequiresReview !== true) {
    errors.push("operationModel.destructiveUninstallRequiresReview must stay true");
  }
  if (policy.operationModel?.dataAfterDisableDefault !== "retained-unreachable") {
    errors.push("operationModel.dataAfterDisableDefault must stay retained-unreachable");
  }

  requireAllowedModes(policy, "installMode", requiredInstallModes, errors);
  requireAllowedModes(policy, "disableMode", requiredDisableModes, errors);
  requireAllowedModes(policy, "uninstallMode", requiredUninstallModes, errors);
  requireAllowedModes(policy, "dataAfterDisable", requiredDataModes, errors);
  requireIncludes(policy.requiredValidators, requiredValidators, "requiredValidators", errors);
  requireIncludes(policy.requiredTests, requiredTests, "requiredTests", errors);
  for (const relativePath of [...values(policy.requiredValidators), ...values(policy.requiredTests)]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`operation policy path is missing or unsafe: ${relativePath}`);
    }
  }
  validateSourceSnippets(policy, errors);

  errors.push(...uniqueErrors(operations.map((capability) => capability.id), "capabilities.id"));
  errors.push(...uniqueErrors(values(policy.nonRemovableCapabilities), "nonRemovableCapabilities"));
  for (const capabilityID of contractsByID.keys()) {
    if (!operationsByID.has(capabilityID)) {
      errors.push(`capability ${capabilityID} from contracts is missing from operation policy`);
    }
  }
  for (const capabilityID of operationsByID.keys()) {
    if (!contractsByID.has(capabilityID)) {
      errors.push(`capability ${capabilityID} from operation policy is missing from contracts`);
    }
  }
  for (const capabilityID of nonRemovable) {
    const contract = contractsByID.get(capabilityID);
    if (!contract) {
      errors.push(`nonRemovable capability ${capabilityID} is missing from contracts`);
      continue;
    }
    if (contract.classification !== "foundation-core") {
      errors.push(`nonRemovable capability ${capabilityID} must be foundation-core`);
    }
  }

  const oidcProfile = profilesByID.get("production-admin-oidc-ready");
  if (!oidcProfile) {
    errors.push("profile production-admin-oidc-ready is required");
  } else {
    if (!values(oidcProfile.capabilities).includes("admin-oidc")) {
      errors.push("profile production-admin-oidc-ready must include required capability admin-oidc");
    }
    if (values(oidcProfile.capabilities).includes("demo-data")) {
      errors.push("profile production-admin-oidc-ready must not include demo-data");
    }
  }

  for (const operation of operations) {
    errors.push(...validatePolicyCapability(operation, contractsByID.get(operation.id), nonRemovable));
  }
  for (const combination of values(policy.validatedCombinations)) {
    errors.push(...validateCombination(combination, profilesByID));
  }

  return { policy, errors };
}

const { policy, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

const relativePolicyPath = path.relative(repoRoot, policyPath).split(path.sep).join("/");
console.log(`Validated ${policy.capabilities?.length ?? 0} capability operation policies in ${relativePolicyPath}`);
