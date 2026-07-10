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

const contractsPath = path.resolve(repoRoot, argValue("--contracts", "resources/platform-capability-contracts.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const configPath = path.resolve(repoRoot, argValue("--config", "internal/platform/config/config.go"));

const allowedProfilePolicies = new Set(["default-enabled", "default-development-only", "profile-only", "business-external-only"]);
const allowedClassifications = new Set([
  "foundation-core",
  "foundation-default",
  "optional-platform",
  "local-demo",
  "external-business-boundary",
]);
const allowedSourceKinds = new Set(["core-manifest", "platform-extension-manifest", "external-business-boundary"]);

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function hasLocalizedText(value) {
  return typeof value?.zh === "string" && value.zh.trim() !== "" && typeof value?.en === "string" && value.en.trim() !== "";
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

function sameSet(left, right) {
  const leftValues = values(left).slice().sort();
  const rightValues = values(right).slice().sort();
  return leftValues.length === rightValues.length && leftValues.every((item, index) => item === rightValues[index]);
}

function readDefaultCapabilities() {
  const source = fs.readFileSync(configPath, "utf8");
  const match = source.match(/var\s+defaultCapabilities\s*=\s*\[\]string\s*{([\s\S]*?)\n}/);
  if (!match) {
    throw new Error("cannot find defaultCapabilities in internal/platform/config/config.go");
  }
  return [...match[1].matchAll(/"([^"]+)"/g)].map((item) => item[1]);
}

function runProfileAudit(profile) {
  const result = spawnSync("go", ["run", "./cmd/platform-contracts", "audit", "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      PLATFORM_CAPABILITIES: values(profile.capabilities).join(","),
    },
  });
  if (result.status !== 0) {
    return { error: `profile ${profile.id} failed capability audit\n${result.stdout}${result.stderr}` };
  }
  try {
    return { audit: JSON.parse(result.stdout) };
  } catch (error) {
    return { error: `profile ${profile.id} audit output is not valid JSON: ${error.message}` };
  }
}

function collectProfileAudits(profiles) {
  const errors = [];
  const byProfile = new Map();
  const capabilityByID = new Map();
  for (const profile of profiles) {
    const { audit, error } = runProfileAudit(profile);
    if (error) {
      errors.push(error);
      continue;
    }
    byProfile.set(profile.id, audit);
    for (const capability of values(audit.capabilities)) {
      if (!capabilityByID.has(capability.id)) {
        capabilityByID.set(capability.id, capability);
      }
    }
  }
  return { byProfile, capabilityByID, errors };
}

function validateDeclaredSurface(contract, actual, prefix, errors) {
  const comparisons = [
    ["dependencies", "dependencies"],
    ["adminResources", "adminResources"],
    ["appRoutes", "appRoutes"],
    ["authProviders", "authProviders"],
    ["demoDataSets", "demoDataSets"],
  ];
  for (const [contractKey, actualKey] of comparisons) {
    if (contract[contractKey] === undefined) {
      continue;
    }
    if (!sameSet(contract[contractKey], actual?.[actualKey])) {
      errors.push(`${prefix}.${contractKey} must match capability audit output`);
    }
  }
}

function validateProfilePolicy(contract, context, errors) {
  const prefix = `capability contract ${contract.id}`;
  const {
    defaultCapabilities,
    runtimeDefaultProfile,
    businessCapabilities,
    nonBusinessProfiles,
    profileCapabilityIDs,
    profileByID,
    capabilityByID,
  } = context;
  const defaultSet = new Set(defaultCapabilities);
  const runtimeDefaultSet = new Set(values(runtimeDefaultProfile?.capabilities));
  const businessSet = new Set(businessCapabilities);
  const appearsInProfiles = profileCapabilityIDs.has(contract.id);

  if (contract.profilePolicy === "default-enabled") {
    if (!defaultSet.has(contract.id) || !runtimeDefaultSet.has(contract.id)) {
      errors.push(`${prefix} is default-enabled and must be in config defaults and runtime default profile`);
    }
    if (businessSet.has(contract.id)) {
      errors.push(`${prefix} is default-enabled and must not be a business capability`);
    }
    if (contract.productionPolicy === "forbidden") {
      errors.push(`${prefix} must not be production-forbidden when default-enabled`);
    }
  } else if (contract.profilePolicy === "default-development-only") {
    if (!defaultSet.has(contract.id) || !runtimeDefaultSet.has(contract.id)) {
      errors.push(`${prefix} is default-development-only and must stay visible in development defaults`);
    }
    if (contract.productionPolicy !== "forbidden") {
      errors.push(`${prefix} is default-development-only and must declare productionPolicy=forbidden`);
    }
  } else if (contract.profilePolicy === "profile-only") {
    if (defaultSet.has(contract.id) || runtimeDefaultSet.has(contract.id)) {
      errors.push(`${prefix} is profile-only and must not be enabled by default`);
    }
    if (!appearsInProfiles) {
      errors.push(`${prefix} is profile-only and must appear in at least one non-default profile`);
    }
  } else if (contract.profilePolicy === "business-external-only") {
    if (!businessSet.has(contract.id)) {
      errors.push(`${prefix} must be listed in capability profiles businessCapabilities`);
    }
    for (const profile of nonBusinessProfiles) {
      if (values(profile.capabilities).includes(contract.id)) {
        errors.push(`${prefix} must not appear in non-business profile ${profile.id}`);
      }
    }
    if (contract.source?.kind !== "external-business-boundary") {
      errors.push(`${prefix} business-external-only source.kind must be external-business-boundary`);
    }
    return;
  }

  if (contract.profilePolicy !== "business-external-only" && !capabilityByID.has(contract.id)) {
    errors.push(`${prefix} must be backed by at least one audited platform manifest`);
  }

  for (const profileID of values(contract.requiredInProfiles)) {
    const profile = profileByID.get(profileID);
    if (!profile) {
      errors.push(`${prefix}.requiredInProfiles references unknown profile ${profileID}`);
      continue;
    }
    if (!values(profile.capabilities).includes(contract.id)) {
      errors.push(`${prefix} must be included in required profile ${profileID}`);
    }
  }
  for (const profileID of values(contract.includedInProfiles)) {
    const profile = profileByID.get(profileID);
    if (!profile) {
      errors.push(`${prefix}.includedInProfiles references unknown profile ${profileID}`);
      continue;
    }
    if (!values(profile.capabilities).includes(contract.id)) {
      errors.push(`${prefix} must be included in profile ${profileID}`);
    }
  }
  for (const profileID of values(contract.excludedFromProfiles)) {
    const profile = profileByID.get(profileID);
    if (!profile) {
      errors.push(`${prefix}.excludedFromProfiles references unknown profile ${profileID}`);
      continue;
    }
    if (values(profile.capabilities).includes(contract.id)) {
      errors.push(`${prefix} must be excluded from profile ${profileID}`);
    }
  }
}

function validateContract(contract, context) {
  const errors = [];
  const prefix = `capability contract ${contract.id ?? "<missing>"}`;
  if (!contract.id) {
    return ["capability contract is missing id"];
  }
  if (!hasLocalizedText(contract.label)) {
    errors.push(`${prefix} must declare zh/en label`);
  }
  if (!allowedClassifications.has(contract.classification)) {
    errors.push(`${prefix} has unsupported classification ${contract.classification ?? "<missing>"}`);
  }
  if (!allowedProfilePolicies.has(contract.profilePolicy)) {
    errors.push(`${prefix} has unsupported profilePolicy ${contract.profilePolicy ?? "<missing>"}`);
  }
  if (!allowedSourceKinds.has(contract.source?.kind)) {
    errors.push(`${prefix} has unsupported source.kind ${contract.source?.kind ?? "<missing>"}`);
  }
  for (const field of ["ownership", "boundary", "replaceability"]) {
    if (!contract[field]) {
      errors.push(`${prefix} must declare ${field}`);
    }
  }
  const docs = values(contract.docs);
  if (docs.length === 0) {
    errors.push(`${prefix} must declare docs`);
  }
  for (const docPath of docs) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`${prefix} doc path is missing or unsafe: ${docPath}`);
    }
  }
  errors.push(...uniqueErrors(docs, `${prefix}.docs`));
  errors.push(...uniqueErrors(values(contract.requiredInProfiles), `${prefix}.requiredInProfiles`));
  errors.push(...uniqueErrors(values(contract.includedInProfiles), `${prefix}.includedInProfiles`));
  errors.push(...uniqueErrors(values(contract.excludedFromProfiles), `${prefix}.excludedFromProfiles`));

  validateProfilePolicy(contract, context, errors);
  if (contract.profilePolicy !== "business-external-only") {
    validateDeclaredSurface(contract, context.capabilityByID.get(contract.id), prefix, errors);
  }
  if (contract.classification === "external-business-boundary" && contract.profilePolicy !== "business-external-only") {
    errors.push(`${prefix} external-business-boundary must use business-external-only profilePolicy`);
  }
  return errors;
}

function validate() {
  const contracts = readJSON(contractsPath);
  const profilesDoc = readJSON(profilesPath);
  const defaultCapabilities = readDefaultCapabilities();
  const errors = [];
  const capabilities = values(contracts.capabilities);
  const profiles = values(profilesDoc.profiles);
  const profileByID = new Map(profiles.map((profile) => [profile.id, profile]));
  const runtimeDefaultProfile = profileByID.get(profilesDoc.runtimeDefault);
  const nonBusinessProfiles = profiles.filter((profile) => profile.business !== true);
  const profileCapabilityIDs = new Set(profiles.flatMap((profile) => values(profile.capabilities)));
  const businessCapabilities = values(profilesDoc.businessCapabilities);

  if (!contracts.purpose) {
    errors.push("capability contracts purpose is required");
  }
  if (!contracts.contractVersion) {
    errors.push("capability contracts contractVersion is required");
  }
  if (contracts.policies?.runtimeManifestMutation !== "forbidden") {
    errors.push("policies.runtimeManifestMutation must stay forbidden");
  }
  if (contracts.policies?.defaultProfileBusinessCapabilitiesAllowed !== false) {
    errors.push("policies.defaultProfileBusinessCapabilitiesAllowed must stay false");
  }
  if (contracts.policies?.defaultProfile !== profilesDoc.runtimeDefault) {
    errors.push("policies.defaultProfile must match capability profiles runtimeDefault");
  }
  for (const policy of allowedProfilePolicies) {
    if (!values(contracts.policies?.profilePolicies).includes(policy)) {
      errors.push(`policies.profilePolicies must include ${policy}`);
    }
  }
  for (const classification of allowedClassifications) {
    if (!values(contracts.policies?.classifications).includes(classification)) {
      errors.push(`policies.classifications must include ${classification}`);
    }
  }
  for (const sourceKind of allowedSourceKinds) {
    if (!values(contracts.policies?.sourceKinds).includes(sourceKind)) {
      errors.push(`policies.sourceKinds must include ${sourceKind}`);
    }
  }

  errors.push(...uniqueErrors(capabilities.map((capability) => capability.id), "capabilities.id"));
  const contractIDs = new Set(capabilities.map((capability) => capability.id));
  for (const capabilityID of [...profileCapabilityIDs, ...businessCapabilities]) {
    if (!contractIDs.has(capabilityID)) {
      errors.push(`capability ${capabilityID} appears in profiles but is missing from platform capability contracts`);
    }
  }
  for (const capabilityID of defaultCapabilities) {
    if (!contractIDs.has(capabilityID)) {
      errors.push(`default capability ${capabilityID} is missing from platform capability contracts`);
    }
  }

  const auditContext = collectProfileAudits(profiles);
  errors.push(...auditContext.errors);
  const context = {
    defaultCapabilities,
    runtimeDefaultProfile,
    businessCapabilities,
    nonBusinessProfiles,
    profileCapabilityIDs,
    profileByID,
    capabilityByID: auditContext.capabilityByID,
  };
  for (const contract of capabilities) {
    errors.push(...validateContract(contract, context));
  }
  return { contracts, errors };
}

const { contracts, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${contracts.capabilities?.length ?? 0} platform capability contracts in ${path.relative(repoRoot, contractsPath)}`);
