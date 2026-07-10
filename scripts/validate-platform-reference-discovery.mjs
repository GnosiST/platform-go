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

const discoveryPath = path.resolve(repoRoot, argValue("--discovery", "resources/platform-reference-discovery.json"));
const coveragePath = path.resolve(repoRoot, argValue("--coverage", "resources/platform-reference-coverage.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const engineeringPath = path.resolve(repoRoot, argValue("--engineering", "resources/platform-engineering-capabilities.json"));

const allowedClassifications = new Set(["foundation", "extension", "business", "deferred"]);
const requiredSourceSetIDs = ["admin-resource-manifest", "backend-models", "http-routes", "admin-query-security", "app-client-api-boundary", "architecture-docs", "demo-data-reset"];
const requiredCandidateIDs = [
  "dashboard-shell",
  "identity-tenancy-org",
  "rbac-menu-permissions",
  "api-governance",
  "dictionary-parameters-storage-settings",
  "audit-session-operations",
  "file-storage-assets",
  "auth-provider-wechat-demo",
  "demo-data-reset",
  "app-phone-identity",
  "detailed-addresses",
  "multi-org-membership",
  "role-application-access",
  "business-content-profiles-portfolio-favorites",
  "business-dispatch-transfer",
  "business-fulfillment-confirmations",
  "business-support-tickets",
  "query-security-and-api-boundary",
  "app-client-api-boundary",
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

function relativeExistingPath(root, relativePath) {
  if (!root || !relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(root, relativePath);
  const relative = path.relative(root, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function repoRelativeExistingPath(relativePath) {
  return relativeExistingPath(repoRoot, relativePath);
}

function readReferenceFile(referenceRoot, relativePath) {
  return fs.readFileSync(path.resolve(referenceRoot, relativePath), "utf8");
}

function profileByID(profiles) {
  return new Map(values(profiles.profiles).map((profile) => [profile.id, profile]));
}

function capabilitySetFromProfiles(profiles) {
  const capabilities = new Set(values(profiles.businessCapabilities));
  for (const profile of values(profiles.profiles)) {
    for (const capability of values(profile.capabilities)) {
      capabilities.add(capability);
    }
    for (const capability of values(profile.mustIncludeCapabilities)) {
      capabilities.add(capability);
    }
  }
  return capabilities;
}

function engineeringCapabilityIDs(engineering) {
  return new Set(values(engineering.capabilities).map((capability) => capability.id));
}

function validateSourceSets(discovery, errors) {
  const referenceRoot = discovery.reference?.root ?? "";
  const sourceSets = values(discovery.sourceSets);
  const sourceSetIDs = new Set(sourceSets.map((sourceSet) => sourceSet.id));
  const sourceSetSources = new Map();
  errors.push(...uniqueErrors(sourceSets.map((sourceSet) => sourceSet.id), "sourceSets.id"));

  for (const sourceSetID of requiredSourceSetIDs) {
    if (!sourceSetIDs.has(sourceSetID)) {
      errors.push(`reference discovery must include source set ${sourceSetID}`);
    }
  }

  for (const sourceSet of sourceSets) {
    const prefix = `source set ${sourceSet.id ?? "<missing>"}`;
    if (!sourceSet.id) {
      errors.push("source set is missing id");
      continue;
    }
    if (values(sourceSet.paths).length === 0) {
      errors.push(`${prefix} must declare paths`);
    }
    if (values(sourceSet.requiredSignals).length === 0) {
      errors.push(`${prefix} must declare requiredSignals`);
    }
    const combinedSource = values(sourceSet.paths)
      .map((relativePath) => {
        if (!relativeExistingPath(referenceRoot, relativePath)) {
          errors.push(`${prefix} path is missing or unsafe in reference project: ${relativePath}`);
          return "";
        }
        return readReferenceFile(referenceRoot, relativePath);
      })
      .join("\n");
    sourceSetSources.set(sourceSet.id, combinedSource);
    for (const signal of values(sourceSet.requiredSignals)) {
      if (!combinedSource.includes(signal)) {
        errors.push(`${prefix} is missing required signal ${signal}`);
      }
    }
  }

  return { sourceSetIDs, sourceSetSources };
}

function validateCandidateMappings(discovery, coverage, profiles, engineering, sourceSetIDs, sourceSetSources, errors) {
  const candidates = values(discovery.candidates);
  errors.push(...uniqueErrors(candidates.map((candidate) => candidate.id), "candidates.id"));
  const candidateIDs = new Set(candidates.map((candidate) => candidate.id));
  const foundationAreas = new Set(values(coverage.foundation).map((area) => area.area));
  const businessAreas = new Set(values(coverage.businessBoundary).map((boundary) => boundary.area));
  const extensionAreas = new Set(values(coverage.extensionBoundary).map((boundary) => boundary.area));
  const nonResourceCapabilities = new Set(values(coverage.nonResourceParity).map((item) => item.referenceCapability));
  const profilesByID = profileByID(profiles);
  const knownCapabilities = capabilitySetFromProfiles(profiles);
  const engineeringIDs = engineeringCapabilityIDs(engineering);

  for (const candidateID of requiredCandidateIDs) {
    if (!candidateIDs.has(candidateID)) {
      errors.push(`reference discovery candidate ${candidateID} is missing`);
    }
  }

  for (const candidate of candidates) {
    const prefix = `candidate ${candidate.id ?? "<missing>"}`;
    if (!candidate.id) {
      errors.push("candidate is missing id");
      continue;
    }
    if (!allowedClassifications.has(candidate.classification)) {
      errors.push(`${prefix} has unsupported classification ${candidate.classification}`);
      continue;
    }
    if (values(candidate.sourceSets).length === 0) {
      errors.push(`${prefix} must declare sourceSets`);
    }
    for (const sourceSetID of values(candidate.sourceSets)) {
      if (!sourceSetIDs.has(sourceSetID)) {
        errors.push(`${prefix} references unknown source set ${sourceSetID}`);
      }
    }
    if (values(candidate.evidenceSignals).length === 0) {
      errors.push(`${prefix} must declare evidenceSignals`);
    }
    const candidateSource = values(candidate.sourceSets).map((sourceSetID) => sourceSetSources.get(sourceSetID) ?? "").join("\n");
    for (const signal of values(candidate.evidenceSignals)) {
      if (!candidateSource.includes(signal)) {
        errors.push(`${prefix} evidence signal ${signal} is missing from declared sourceSets`);
      }
    }
    if (!candidate.decision) {
      errors.push(`${prefix} must declare decision`);
    }

    if (candidate.classification === "foundation") {
      if (!foundationAreas.has(candidate.coverageArea)) {
        errors.push(`${prefix} references missing foundation coverage area ${candidate.coverageArea}`);
      }
      if (values(candidate.platformCapabilities).length === 0) {
        errors.push(`${prefix} foundation candidate must declare platformCapabilities`);
      }
      for (const capability of values(candidate.platformCapabilities)) {
        if (!knownCapabilities.has(capability)) {
          errors.push(`${prefix} references unknown platform capability ${capability}`);
        }
      }
    }

    if (candidate.classification === "extension") {
      if (!extensionAreas.has(candidate.extensionArea)) {
        errors.push(`${prefix} references missing extension area ${candidate.extensionArea}`);
      }
      if (!candidate.expectedCapability) {
        errors.push(`${prefix} extension candidate must declare expectedCapability`);
      }
      if (candidate.expectedCapability && candidate.expectedCapability !== "owning-capability" && !knownCapabilities.has(candidate.expectedCapability)) {
        errors.push(`${prefix} references unknown expected capability ${candidate.expectedCapability}`);
      }
      if (candidate.expectedProfile && !profilesByID.has(candidate.expectedProfile)) {
        errors.push(`${prefix} references missing profile ${candidate.expectedProfile}`);
      }
    }

    if (candidate.classification === "business") {
      if (!businessAreas.has(candidate.businessArea)) {
        errors.push(`${prefix} references missing business area ${candidate.businessArea}`);
      }
      if (candidate.expectedCapability !== "external-business-capability") {
        errors.push(`${prefix} must be owned by external-business-capability outside platform-go`);
      }
    }

    for (const nonResourceCapability of values(candidate.nonResourceCapabilities)) {
      if (!nonResourceCapabilities.has(nonResourceCapability)) {
        errors.push(`${prefix} references non-resource capability ${nonResourceCapability} that is missing from platform-reference-coverage`);
      }
    }

    if (candidate.classification === "deferred" && !candidate.reason) {
      errors.push(`${prefix} deferred candidate must declare reason`);
    }
    if (candidate.id === "query-security-and-api-boundary") {
      if (candidate.classification !== "foundation") {
        errors.push(`${prefix} must be promoted as a foundation API-boundary gate`);
      }
      if (candidate.decision !== "reuse-as-foundation-gate") {
        errors.push(`${prefix} decision must stay reuse-as-foundation-gate`);
      }
    }
    if (candidate.id === "app-client-api-boundary") {
      if (candidate.classification !== "foundation") {
        errors.push(`${prefix} must be promoted as a foundation App client API boundary gate`);
      }
      if (candidate.decision !== "reuse-as-foundation-gate") {
        errors.push(`${prefix} decision must stay reuse-as-foundation-gate`);
      }
    }
  }

  if (!engineeringIDs.has("reference-coverage-boundary-gate")) {
    errors.push("engineering matrix must include reference-coverage-boundary-gate");
  }
}

function validateCoverageExplained(discovery, coverage, errors) {
  const explainedFoundation = new Set(values(discovery.candidates).filter((candidate) => candidate.classification === "foundation").map((candidate) => candidate.coverageArea));
  const explainedBusiness = new Set(values(discovery.candidates).filter((candidate) => candidate.classification === "business").map((candidate) => candidate.businessArea));
  const explainedExtension = new Set(values(discovery.candidates).filter((candidate) => candidate.classification === "extension").map((candidate) => candidate.extensionArea));

  for (const area of values(coverage.foundation).map((item) => item.area)) {
    if (!explainedFoundation.has(area)) {
      errors.push(`reference discovery is missing a foundation candidate for coverage area ${area}`);
    }
  }
  for (const area of values(coverage.businessBoundary).map((item) => item.area)) {
    if (!explainedBusiness.has(area)) {
      errors.push(`reference discovery is missing a business candidate for boundary ${area}`);
    }
  }
  for (const area of values(coverage.extensionBoundary).map((item) => item.area)) {
    if (!explainedExtension.has(area)) {
      errors.push(`reference discovery is missing an extension candidate for boundary ${area}`);
    }
  }
}

function validateDefaultProfileBoundaries(discovery, profiles, errors) {
  const defaultProfile = values(profiles.profiles).find((profile) => profile.id === "platform-default");
  if (!defaultProfile) {
    errors.push("platform-default profile is missing");
    return;
  }
  const defaultCapabilities = new Set(values(defaultProfile.capabilities));
  const excluded = new Set(values(defaultProfile.mustExcludeCapabilities));
  for (const capability of values(discovery.mustStayOutOfDefaultProfile)) {
    if (defaultCapabilities.has(capability)) {
      errors.push(`platform-default must not enable ${capability}`);
    }
    if (!excluded.has(capability)) {
      errors.push(`platform-default must explicitly exclude ${capability}`);
    }
  }
}

function validateDocuments(discovery, errors) {
  for (const documentPath of values(discovery.documents)) {
    if (!repoRelativeExistingPath(documentPath)) {
      errors.push(`reference discovery document path is missing or unsafe: ${documentPath}`);
    }
  }
}

function validate() {
  const discovery = readJSON(discoveryPath);
  const coverage = readJSON(coveragePath);
  const profiles = readJSON(profilesPath);
  const engineering = readJSON(engineeringPath);
  const errors = [];

  if (!discovery.purpose) {
    errors.push("reference discovery purpose is required");
  }
  if (discovery.reference?.project !== "zshenmez") {
    errors.push("reference discovery project must stay zshenmez");
  }
  if (!fs.existsSync(discovery.reference?.root ?? "")) {
    errors.push(`reference discovery root is missing: ${discovery.reference?.root ?? "<missing>"}`);
  }
  const { sourceSetIDs, sourceSetSources } = validateSourceSets(discovery, errors);
  validateCandidateMappings(discovery, coverage, profiles, engineering, sourceSetIDs, sourceSetSources, errors);
  validateCoverageExplained(discovery, coverage, errors);
  validateDefaultProfileBoundaries(discovery, profiles, errors);
  validateDocuments(discovery, errors);

  return { discovery, errors };
}

const { discovery, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${discovery.candidates?.length ?? 0} reference discovery candidates in ${path.relative(repoRoot, discoveryPath)}`);
