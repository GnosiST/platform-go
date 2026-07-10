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

const coveragePath = path.resolve(repoRoot, argValue("--coverage", "resources/platform-reference-coverage.json"));
const adminContractPath = path.resolve(repoRoot, argValue("--admin-contract", "resources/generated/admin-resource-contract.json"));
const auditPath = path.resolve(repoRoot, argValue("--audit", "resources/generated/platform-capability-audit.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const referenceDiscoveryPath = path.resolve(repoRoot, argValue("--reference-discovery", "resources/platform-reference-discovery.json"));
const referenceManifestArg = argValue("--reference-manifest", "");
const requiredFoundationAreas = [
  "dashboard",
  "identity-and-tenancy",
  "rbac-and-menu",
  "api-governance",
  "dictionary-parameters-branding",
  "audit-and-operations",
  "file-storage",
  "auth-providers",
  "demo-data",
];
const requiredNonResourceCapabilities = ["storage-settings", "admin-api-boundary-query-security", "app-phone-binding", "user-addresses", "user-org-memberships"];
const requiredFoundationAppRoutes = new Map([
  ["file-storage", ["POST /api/app/files", "GET /api/app/files/:id/content"]],
]);
const referenceBusinessCapabilityOwner = "external-business-capability";

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function normalizeResourceKey(value) {
  return String(value ?? "")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/_/g, "-")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}

function resolveRelativeExistingPath(root, relativePath, errors, label) {
  if (!root || !relativePath || path.isAbsolute(relativePath)) {
    errors.push(`${label} must be a relative path under the reference root`);
    return "";
  }
  const absolutePath = path.resolve(root, relativePath);
  const relative = path.relative(root, absolutePath);
  if (relative === "" || relative.startsWith("..") || !fs.existsSync(absolutePath)) {
    errors.push(`${label} is missing or unsafe: ${relativePath}`);
    return "";
  }
  return absolutePath;
}

function resolveReferenceManifestPath(coverage, errors) {
  if (referenceManifestArg) {
    return path.resolve(repoRoot, referenceManifestArg);
  }

  if (!fs.existsSync(referenceDiscoveryPath)) {
    errors.push(`reference discovery file is missing: ${path.relative(repoRoot, referenceDiscoveryPath)}`);
    return "";
  }

  const discovery = readJSON(referenceDiscoveryPath);
  if (discovery.reference?.project !== coverage.reference?.project) {
    errors.push("reference discovery project must match reference coverage project");
  }

  return resolveRelativeExistingPath(
    discovery.reference?.root ?? "",
    coverage.reference?.resourceManifest ?? "",
    errors,
    "reference coverage resourceManifest",
  );
}

function resourceKeysFromAdminContract(adminContract) {
  const keys = new Set();
  for (const resource of values(adminContract.resources)) {
    for (const key of [resource.name, resource.code, resource.resource]) {
      if (key) keys.add(key);
    }
  }
  return keys;
}

function resourceProvided(resourceKeys, resource) {
  const normalized = normalizeResourceKey(resource);
  for (const key of resourceKeys) {
    if (normalizeResourceKey(key) === normalized) {
      return true;
    }
  }
  return false;
}

function addCapabilityCoverage(target, capabilityID, resources) {
  if (!capabilityID) {
    return;
  }
  if (!target.has(capabilityID)) {
    target.set(capabilityID, []);
  }
  target.get(capabilityID).push(...resources);
}

function validateExpectedCapabilityRouteCoverage(boundaries, capabilityRoutesByID, errors) {
  const coverageByCapability = new Map();
  for (const boundary of values(boundaries)) {
    addCapabilityCoverage(coverageByCapability, boundary.expectedCapability, values(boundary.appRoutes));
  }

  for (const [capabilityID, coveredRoutes] of coverageByCapability.entries()) {
    const capabilityRoutes = capabilityRoutesByID.get(capabilityID);
    if (!capabilityRoutes) {
      continue;
    }
    const covered = new Set(coveredRoutes);
    const expected = new Set(capabilityRoutes);
    for (const route of expected) {
      if (!covered.has(route)) {
        errors.push(`coverage for ${capabilityID} is missing capability app route ${route}`);
      }
    }
    for (const route of covered) {
      if (!expected.has(route)) {
        errors.push(`coverage for ${capabilityID} declares app route ${route} that is not in the capability audit`);
      }
    }
  }
}

function profileByID(profiles) {
  return new Map(values(profiles.profiles).map((profile) => [profile.id, profile]));
}

function extensionBoundaryCapabilitySet(coverage) {
  const capabilities = new Set();
  for (const boundary of values(coverage.extensionBoundary)) {
    for (const capability of values(boundary.referenceCapabilities)) {
      capabilities.add(normalizeResourceKey(capability));
    }
  }
  return capabilities;
}

function validateNonResourceParity(coverage, adminResourceKeys, capabilityIDs, profiles, appRoutes, capabilityRoutesByID, errors) {
  const parity = values(coverage.nonResourceParity);
  const parityCapabilities = new Map();
  const foundationAreas = new Set(values(coverage.foundation).map((area) => area.area));
  const extensionAreas = new Set(values(coverage.extensionBoundary).map((boundary) => boundary.area));
  const extensionCapabilities = extensionBoundaryCapabilitySet(coverage);
  const profilesByID = profileByID(profiles);

  for (const item of parity) {
    const capability = item.referenceCapability ?? "";
    const normalized = normalizeResourceKey(capability);
    const prefix = `reference non-resource capability ${capability || "<missing>"}`;
    if (!capability) {
      errors.push("reference non-resource parity item is missing referenceCapability");
      continue;
    }
    if (parityCapabilities.has(normalized)) {
      errors.push(`${prefix} is duplicated`);
    }
    parityCapabilities.set(normalized, item);

    if (!["foundation", "business", "extension"].includes(item.classification)) {
      errors.push(`${prefix} must classify as foundation, business or extension`);
      continue;
    }
    if (values(item.referenceModules).length === 0) {
      errors.push(`${prefix} must declare referenceModules`);
    }

    if (item.classification === "foundation") {
      if (!foundationAreas.has(item.foundationArea)) {
        errors.push(`${prefix} references missing foundation area ${item.foundationArea}`);
      }
      if (values(item.capabilities).length === 0) {
        errors.push(`${prefix} foundation mapping must declare capabilities`);
      }
      for (const platformResource of values(item.platformResources)) {
        if (!resourceProvided(adminResourceKeys, platformResource)) {
          errors.push(`${prefix} missing platform admin resource ${platformResource}`);
        }
      }
      for (const expectedCapability of values(item.capabilities)) {
        if (!capabilityIDs.has(expectedCapability)) {
          errors.push(`${prefix} missing platform capability ${expectedCapability}`);
        }
      }
    }

    if (item.classification === "extension") {
      if (!extensionAreas.has(item.extensionArea)) {
        errors.push(`${prefix} references missing extension boundary area ${item.extensionArea}`);
      }
      if (!extensionCapabilities.has(normalized)) {
        errors.push(`${prefix} must be listed in extensionBoundary referenceCapabilities`);
      }
      if (item.defaultPlatformPolicy !== "excluded") {
        errors.push(`${prefix} must stay outside the default platform foundation`);
      }
      if (!item.expectedCapability) {
        errors.push(`${prefix} extension mapping must declare expectedCapability`);
      }
      if (
        item.expectedCapability &&
        item.expectedCapability !== "owning-capability" &&
        !capabilityIDs.has(item.expectedCapability) &&
        !values(profilesByID.get(item.expectedProfile)?.capabilities).includes(item.expectedCapability)
      ) {
        errors.push(`${prefix} missing platform capability ${item.expectedCapability}`);
      }
      if (item.expectedProfile) {
        const profile = profilesByID.get(item.expectedProfile);
        if (!profile) {
          errors.push(`${prefix} references missing profile ${item.expectedProfile}`);
        } else if (!values(profile.capabilities).includes(item.expectedCapability)) {
          errors.push(`${prefix} must be enabled through profile ${item.expectedProfile}`);
        }
      }
      const expectedCapabilityRoutes = capabilityRoutesByID.get(item.expectedCapability);
      if (expectedCapabilityRoutes) {
        const routes = new Set(expectedCapabilityRoutes);
        for (const route of values(item.appRoutes)) {
          if (!routes.has(route)) {
            errors.push(`${prefix} declares app route ${route} that is not in capability ${item.expectedCapability}`);
          }
        }
      } else if (capabilityIDs.has(item.expectedCapability)) {
        for (const route of values(item.appRoutes)) {
          if (!appRoutes.has(route)) {
            errors.push(`${prefix} missing app route ${route}`);
          }
        }
      }
    }

    if (capability === "app-phone-binding") {
      if (item.expectedCapability !== "app-phone") {
        errors.push(`${prefix} must be owned by optional capability app-phone`);
      }
      if (item.expectedProfile !== "platform-app-ready") {
        errors.push(`${prefix} must be enabled through profile platform-app-ready`);
      }
      if (item.defaultPlatformPolicy !== "excluded") {
        errors.push(`${prefix} must stay outside the default platform foundation`);
      }
    }

    if (capability === "user-addresses") {
      if (item.classification !== "extension" || item.defaultPlatformPolicy !== "excluded") {
        errors.push(`${prefix} must stay outside the default platform foundation`);
      }
      if (item.expectedCapability !== "owning-capability") {
        errors.push(`${prefix} must stay owned by the consuming business or optional address capability`);
      }
    }

    if (capability === "user-org-memberships") {
      if (item.classification !== "extension" || item.defaultPlatformPolicy !== "excluded") {
        errors.push(`${prefix} must stay outside the default platform foundation`);
      }
      if (item.expectedCapability !== "owning-capability") {
        errors.push(`${prefix} must stay owned by an optional identity, personnel or consuming capability`);
      }
    }
  }

  for (const capability of requiredNonResourceCapabilities) {
    if (!parityCapabilities.has(normalizeResourceKey(capability))) {
      errors.push(`reference non-resource capability ${capability} is missing from nonResourceParity`);
    }
  }
}

function validateExpectedCapabilityCoverage(boundaries, capabilityResourcesByID, errors) {
  const coverageByCapability = new Map();
  for (const boundary of values(boundaries)) {
    addCapabilityCoverage(coverageByCapability, boundary.expectedCapability, values(boundary.referenceResources));
  }

  for (const [capabilityID, coveredResources] of coverageByCapability.entries()) {
    const capabilityResources = capabilityResourcesByID.get(capabilityID);
    if (!capabilityResources) {
      continue;
    }
    const covered = new Map(coveredResources.map((resource) => [normalizeResourceKey(resource), resource]));
    const expected = new Map(capabilityResources.map((resource) => [normalizeResourceKey(resource), resource]));
    for (const [normalized, resource] of expected.entries()) {
      if (!covered.has(normalized)) {
        errors.push(`coverage for ${capabilityID} is missing capability resource ${resource}`);
      }
    }
    for (const [normalized, resource] of covered.entries()) {
      if (!expected.has(normalized)) {
        errors.push(`coverage for ${capabilityID} declares resource ${resource} that is not in the capability audit`);
      }
    }
  }
}

function validateBusinessCapabilityProfileCoverage(coverage, profiles, errors) {
  const declaredBusinessCapabilities = new Set(values(profiles.businessCapabilities));

  const boundaryCapabilities = new Set();
  for (const boundary of values(coverage.businessBoundary)) {
    const expectedCapability = boundary.expectedCapability ?? "";
    if (!expectedCapability) {
      continue;
    }
    boundaryCapabilities.add(expectedCapability);
    const prefix = `business boundary ${boundary.area ?? "<missing>"}`;
    if (!declaredBusinessCapabilities.has(expectedCapability)) {
      errors.push(`${prefix} expectedCapability ${expectedCapability} must be declared in platform capability profile businessCapabilities`);
      continue;
    }
  }

  for (const capability of declaredBusinessCapabilities) {
    if (!boundaryCapabilities.has(capability)) {
      errors.push(`business capability ${capability} from platform capability profiles is missing from reference businessBoundary`);
    }
  }
}

function referenceBoundaryResources(boundaries) {
  const resources = new Set();
  for (const boundary of values(boundaries)) {
    for (const resource of values(boundary.referenceResources)) {
      resources.add(normalizeResourceKey(resource));
    }
  }
  return resources;
}

function boundaryCapabilityByArea(boundaries) {
  const capabilities = new Map();
  for (const boundary of values(boundaries)) {
    if (boundary.area) {
      capabilities.set(boundary.area, boundary.expectedCapability ?? "");
    }
  }
  return capabilities;
}

function validateReferenceResourceParity(coverage, adminResourceKeys, capabilityIDs, capabilityResourceKeys, referenceResources, errors) {
  const parity = values(coverage.referenceResourceParity);
  const parityResources = new Map();
  const foundationAreas = new Set(values(coverage.foundation).map((area) => area.area));
  const businessAreas = new Set(values(coverage.businessBoundary).map((boundary) => boundary.area));
  const extensionAreas = new Set(values(coverage.extensionBoundary).map((boundary) => boundary.area));
  const businessResources = referenceBoundaryResources(coverage.businessBoundary);
  const extensionResources = referenceBoundaryResources(coverage.extensionBoundary);
  const businessCapabilitiesByArea = boundaryCapabilityByArea(coverage.businessBoundary);

  for (const item of parity) {
    const resource = item.referenceResource ?? "";
    const normalized = normalizeResourceKey(resource);
    const prefix = `reference resource parity ${resource || "<missing>"}`;
    if (!resource) {
      errors.push("reference resource parity item is missing referenceResource");
      continue;
    }
    if (parityResources.has(normalized)) {
      errors.push(`${prefix} is duplicated`);
    }
    parityResources.set(normalized, item);

    if (!item.referenceGroup) {
      errors.push(`${prefix} must declare referenceGroup`);
    }
    if (!["foundation", "business", "extension"].includes(item.classification)) {
      errors.push(`${prefix} must classify as foundation, business or extension`);
      continue;
    }
    if (item.classification === "foundation") {
      if (!foundationAreas.has(item.foundationArea)) {
        errors.push(`${prefix} references missing foundation area ${item.foundationArea}`);
      }
      if (values(item.platformResources).length === 0) {
        errors.push(`${prefix} foundation mapping must declare platformResources`);
      }
      for (const platformResource of values(item.platformResources)) {
        if (!resourceProvided(adminResourceKeys, platformResource)) {
          errors.push(`${prefix} missing platform admin resource ${platformResource}`);
        }
      }
      if (values(item.capabilities).length === 0) {
        errors.push(`${prefix} foundation mapping must declare capabilities`);
      }
      for (const capability of values(item.capabilities)) {
        if (!capabilityIDs.has(capability)) {
          errors.push(`${prefix} missing platform capability ${capability}`);
        }
      }
    }
    if (item.classification === "business") {
      if (!businessAreas.has(item.businessArea)) {
        errors.push(`${prefix} references missing business boundary area ${item.businessArea}`);
      }
      if (!item.expectedCapability) {
        errors.push(`${prefix} business mapping must declare expectedCapability`);
      }
      const boundaryCapability = businessCapabilitiesByArea.get(item.businessArea);
      if (item.expectedCapability && boundaryCapability && item.expectedCapability !== boundaryCapability) {
        errors.push(`${prefix} expectedCapability ${item.expectedCapability} must match business boundary ${boundaryCapability}`);
      }
      if (resourceProvided(adminResourceKeys, resource)) {
        errors.push(`${prefix} must stay out of the default platform admin contract`);
      }
      if (!businessResources.has(normalized)) {
        errors.push(`${prefix} must be listed in businessBoundary referenceResources`);
      }
    }
    if (item.classification === "extension") {
      if (!extensionAreas.has(item.extensionArea)) {
        errors.push(`${prefix} references missing extension boundary area ${item.extensionArea}`);
      }
      if (!item.expectedCapability) {
        errors.push(`${prefix} extension mapping must declare expectedCapability`);
      }
      if (resourceProvided(adminResourceKeys, resource)) {
        errors.push(`${prefix} must stay out of the default platform admin contract`);
      }
      if (!extensionResources.has(normalized)) {
        errors.push(`${prefix} must be listed in extensionBoundary referenceResources`);
      }
    }
    if (item.expectedCapability && capabilityIDs.has(item.expectedCapability)) {
      for (const platformResource of values(item.platformResources)) {
        if (!capabilityResourceKeys.has(platformResource)) {
          errors.push(`${prefix} expected capability ${item.expectedCapability} does not provide ${platformResource}`);
        }
      }
    }
  }

  if (referenceResources) {
    for (const resource of referenceResources) {
      const normalized = normalizeResourceKey(resource);
      if (!parityResources.has(normalized)) {
        errors.push(`reference manifest resource ${resource} is missing from referenceResourceParity`);
      }
    }
  }

  const coveredResources = new Set([
    ...referenceBoundaryResources(coverage.foundation),
    ...referenceBoundaryResources(coverage.businessBoundary),
    ...referenceBoundaryResources(coverage.extensionBoundary),
  ]);
  for (const normalized of coveredResources) {
    if (!parityResources.has(normalized)) {
      errors.push(`reference resource parity ${normalized} is missing for a declared reference resource`);
    }
  }
}

function validate() {
  const coverage = readJSON(coveragePath);
  const adminContract = readJSON(adminContractPath);
  const audit = readJSON(auditPath);
  const profiles = readJSON(profilesPath);
  const errors = [];
  const resolvedReferenceManifestPath = resolveReferenceManifestPath(coverage, errors);
  const referenceResources = resolvedReferenceManifestPath
    ? new Set(values(readJSON(resolvedReferenceManifestPath).resources).map((resource) => normalizeResourceKey(resource.code ?? resource.name ?? resource.resource)))
    : null;

  const adminResourceKeys = resourceKeysFromAdminContract(adminContract);

  const capabilityIDs = new Set();
  const capabilityResourceKeys = new Set();
  const capabilityResourcesByID = new Map();
  const capabilityRoutesByID = new Map();
  const appRoutes = new Set();
  const authProviders = new Set();
  for (const route of values(audit.missingAppRouteHandlers)) {
    errors.push(`missing runtime app route handler ${route}`);
  }
  for (const capability of audit.capabilities ?? []) {
    if (capability.id) capabilityIDs.add(capability.id);
    if (capability.id) capabilityResourcesByID.set(capability.id, values(capability.adminResources));
    if (capability.id) capabilityRoutesByID.set(capability.id, values(capability.appRoutes));
    for (const resource of capability.adminResources ?? []) capabilityResourceKeys.add(resource);
    for (const route of capability.appRoutes ?? []) appRoutes.add(route);
    for (const provider of capability.authProviders ?? []) authProviders.add(provider);
  }

  const seenAreas = new Set();
  for (const area of coverage.foundation ?? []) {
    const prefix = `foundation area ${area.area ?? "<missing>"}`;
    if (!area.area) {
      errors.push("foundation area is missing area");
      continue;
    }
    if (seenAreas.has(area.area)) {
      errors.push(`foundation area ${area.area} is duplicated`);
    }
    seenAreas.add(area.area);
    if (values(area.referenceResources).length === 0 && values(area.referenceModules).length === 0) {
      errors.push(`${prefix} must declare referenceResources or referenceModules`);
    }
    if (
      values(area.platformResources).length === 0 &&
      values(area.capabilities).length === 0 &&
      values(area.capabilityResources).length === 0 &&
      values(area.appRoutes).length === 0 &&
      values(area.authProviders).length === 0
    ) {
      errors.push(`${prefix} must declare at least one platform coverage target`);
    }
    for (const resource of values(area.platformResources)) {
      if (!adminResourceKeys.has(resource)) {
        errors.push(`${prefix} missing platform admin resource ${resource}`);
      }
    }
    for (const capability of values(area.capabilities)) {
      if (!capabilityIDs.has(capability)) {
        errors.push(`${prefix} missing platform capability ${capability}`);
      }
    }
    for (const resource of values(area.capabilityResources)) {
      if (!capabilityResourceKeys.has(resource)) {
        errors.push(`${prefix} missing capability admin resource ${resource}`);
      }
    }
    for (const route of values(area.appRoutes)) {
      if (!appRoutes.has(route)) {
        errors.push(`${prefix} missing app route ${route}`);
      }
    }
    for (const provider of values(area.authProviders)) {
      if (!authProviders.has(provider)) {
        errors.push(`${prefix} missing auth provider ${provider}`);
      }
    }
  }
  for (const area of requiredFoundationAreas) {
    if (!seenAreas.has(area)) {
      errors.push(`missing required foundation coverage area ${area}`);
      continue;
    }
    const coverageArea = values(coverage.foundation).find((item) => item.area === area);
    if (values(coverageArea?.capabilities).length === 0) {
      errors.push(`foundation area ${area} must declare at least one platform capability`);
    }
    for (const requiredRoute of values(requiredFoundationAppRoutes.get(area))) {
      if (!values(coverageArea?.appRoutes).includes(requiredRoute)) {
        errors.push(`foundation area ${area} must declare app route ${requiredRoute}`);
      }
    }
  }

  for (const boundary of coverage.businessBoundary ?? []) {
    const prefix = `business boundary ${boundary.area ?? "<missing>"}`;
    if (!boundary.area) {
      errors.push("business boundary is missing area");
      continue;
    }
    const expectedCapability = boundary.expectedCapability ?? "";
    if (boundary.mustStayOutOfDefaultPlatform !== true) {
      errors.push(`${prefix} must declare mustStayOutOfDefaultPlatform=true`);
    }
    if (!expectedCapability) {
      errors.push(`${prefix} must declare expectedCapability`);
    }
    if (expectedCapability && expectedCapability !== referenceBusinessCapabilityOwner) {
      errors.push(`${prefix} must stay owned by ${referenceBusinessCapabilityOwner} outside platform-go`);
    }
    if (expectedCapability && capabilityIDs.has(expectedCapability)) {
      continue;
    }
    for (const resource of values(boundary.referenceResources)) {
      if (adminResourceKeys.has(resource)) {
        errors.push(`${prefix} resource ${resource} must stay out of the default platform contract unless ${expectedCapability} is enabled`);
      }
    }
  }

  for (const boundary of coverage.extensionBoundary ?? []) {
    const prefix = `extension boundary ${boundary.area ?? "<missing>"}`;
    if (!boundary.area) {
      errors.push("extension boundary is missing area");
      continue;
    }
    const expectedCapability = boundary.expectedCapability ?? "";
    if (boundary.mustStayOutOfDefaultPlatform !== true) {
      errors.push(`${prefix} must declare mustStayOutOfDefaultPlatform=true`);
    }
    if (!expectedCapability) {
      errors.push(`${prefix} must declare expectedCapability`);
    }
    if (expectedCapability && capabilityIDs.has(expectedCapability)) {
      continue;
    }
    for (const resource of values(boundary.referenceResources)) {
      if (adminResourceKeys.has(resource)) {
        errors.push(`${prefix} resource ${resource} must stay out of the default platform contract unless ${expectedCapability} is enabled`);
      }
    }
  }
  validateBusinessCapabilityProfileCoverage(coverage, profiles, errors);
  validateExpectedCapabilityCoverage(coverage.businessBoundary, capabilityResourcesByID, errors);
  validateExpectedCapabilityCoverage(coverage.extensionBoundary, capabilityResourcesByID, errors);
  validateExpectedCapabilityRouteCoverage(coverage.businessBoundary, capabilityRoutesByID, errors);
  validateExpectedCapabilityRouteCoverage(coverage.extensionBoundary, capabilityRoutesByID, errors);
  validateReferenceResourceParity(coverage, adminResourceKeys, capabilityIDs, capabilityResourceKeys, referenceResources, errors);
  validateNonResourceParity(coverage, adminResourceKeys, capabilityIDs, profiles, appRoutes, capabilityRoutesByID, errors);

  return { coverage, errors };
}

const { coverage, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${coverage.foundation?.length ?? 0} platform reference coverage areas in ${path.relative(repoRoot, coveragePath)}`);
