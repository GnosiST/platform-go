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

const topologyPath = path.resolve(repoRoot, argValue("--topology", "resources/platform-governance-topology.json"));
const manifestPath = path.resolve(repoRoot, argValue("--manifest", "resources/admin-resources.json"));
const generatedContractPath = path.resolve(repoRoot, argValue("--admin-contract", "resources/generated/admin-resource-contract.json"));
const capabilityAdminContractPath = path.resolve(repoRoot, argValue("--capability-admin-contract", "resources/generated/admin-capability-resource-contract.json"));
const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-engineering-capabilities.json"));
const personnelAdminContractPath = argValue("--personnel-admin-contract", "");
const requiredOrgUnitTypeOptions = ["group", "company", "branch", "organization", "department", "team", "store", "custom"];
const requiredAreaCodeLevelOptions = ["country", "province", "city", "district", "street", "custom"];
const requiredOrganizationMigrationTaskIDs = [
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function resourceByCode(resources) {
  return new Map(values(resources).map((resource) => [resourceCode(resource), resource]));
}

function fieldByKey(resource, key) {
  return values(resource?.schema?.fields ?? resource?.fields).find((field) => field.key === key);
}

function resourceCode(resource) {
  return resource?.code ?? resource?.resource ?? "<missing-resource>";
}

function optionValues(field) {
  return new Set(values(field?.options).map((option) => (typeof option === "string" ? option : option.value)).filter(Boolean));
}

function includesAll(actualValues, requiredValues) {
  const actual = new Set(actualValues);
  return requiredValues.every((value) => actual.has(value));
}

function sameList(actualValues, requiredValues) {
  return actualValues.length === requiredValues.length && actualValues.every((value, index) => value === requiredValues[index]);
}

function existingRelativePath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function validateFieldContract(resource, contract, errors) {
  const field = fieldByKey(resource, contract.key);
  const prefix = `${resourceCode(resource)}.${contract.key}`;
  if (!field) {
    errors.push(`${prefix} is missing`);
    return;
  }
  if (contract.type && field.type !== contract.type) {
    errors.push(`${prefix} type must be ${contract.type}`);
  }
  if (contract.required === true && field.required !== true) {
    errors.push(`${prefix} must be required`);
  }
  if (contract.requiredPolicy === "optional" && field.required === true) {
    errors.push(`${prefix} must stay optional by default`);
  }
  if (values(contract.options).length > 0 && !includesAll([...optionValues(field)], contract.options)) {
    errors.push(`${prefix} must include options ${contract.options.join(", ")}`);
  }
  if (contract.relationResource) {
    const relation = field.relation;
    if (!relation || relation.resource !== contract.relationResource) {
      errors.push(`${prefix} must relate to ${contract.relationResource}`);
      return;
    }
    if (contract.display && relation.display !== contract.display) {
      errors.push(`${prefix} relation.display must be ${contract.display}`);
    }
    if (contract.multiple != null && relation.multiple !== contract.multiple) {
      errors.push(`${prefix} relation.multiple must be ${contract.multiple}`);
    }
    if (contract.parentField && relation.parentField !== contract.parentField) {
      errors.push(`${prefix} relation.parentField must be ${contract.parentField}`);
    }
    if (contract.pathField && relation.pathField !== contract.pathField) {
      errors.push(`${prefix} relation.pathField must be ${contract.pathField}`);
    }
  }
}

function validateResourceContracts(topology, manifestResources, errors, sourceLabel = "admin manifest") {
  const resources = resourceByCode(manifestResources);
  const manifestCodes = new Set(resources.keys());
  for (const resource of values(topology.defaultFoundation?.mustIncludeResources)) {
    if (!manifestCodes.has(resource)) {
      errors.push(`${sourceLabel} default foundation must include resource ${resource}`);
    }
  }
  for (const resource of values(topology.defaultFoundation?.mustExcludeResources)) {
    if (manifestCodes.has(resource)) {
      errors.push(`${sourceLabel} default foundation must exclude resource ${resource}`);
    }
  }
  for (const contract of values(topology.resourceContracts)) {
    const resource = resources.get(contract.resource);
    if (!resource) {
      errors.push(`governance contract resource ${contract.resource} is missing from ${sourceLabel}`);
      continue;
    }
    for (const field of values(contract.fields)) {
      validateFieldContract(resource, field, errors);
    }
    if (contract.classificationOnly === true) {
      validateRoleGroupClassification(resource, topology, errors);
    }
  }
}

function validateRoleGroupClassification(roleGroups, topology, errors) {
  const forbiddenTerms = values(topology.roleGroupPolicy?.forbiddenFieldTerms).map((term) => term.toLowerCase());
  const forbiddenFields = values(roleGroups.schema?.fields).filter((field) => {
    const key = String(field.key ?? "").toLowerCase();
    return forbiddenTerms.some((term) => key.includes(term));
  });
  if (forbiddenFields.length > 0) {
    errors.push(`role-groups must remain classification-only; forbidden fields: ${forbiddenFields.map((field) => field.key).join(", ")}`);
  }
}

function validateOrgUnitPolicy(topology, errors) {
  const policy = topology.orgUnitPolicy ?? {};
  if (policy.mode !== "single-tree") {
    errors.push("orgUnitPolicy.mode must stay single-tree");
  }
  if (policy.resource !== "org-units") {
    errors.push("orgUnitPolicy.resource must stay org-units");
  }
  if (policy.requiredDefault !== true) {
    errors.push("orgUnitPolicy.requiredDefault must stay true");
  }
  if (!includesAll(values(policy.levels), requiredOrgUnitTypeOptions)) {
    errors.push(`orgUnitPolicy.levels must include ${requiredOrgUnitTypeOptions.join(", ")}`);
  }
  if (policy.tenantBoundaryField !== "tenantCode") {
    errors.push("orgUnitPolicy.tenantBoundaryField must stay tenantCode");
  }
  if (policy.parentField !== "parentCode") {
    errors.push("orgUnitPolicy.parentField must stay parentCode");
  }
  if (policy.areaField !== "areaCode") {
    errors.push("orgUnitPolicy.areaField must stay areaCode");
  }
  if (policy.personnelOwnerCapability !== "personnel") {
    errors.push("orgUnitPolicy.personnelOwnerCapability must stay personnel");
  }
  for (const resource of ["organizations", "departments", "regions", "employees"]) {
    if (!values(policy.forbiddenDefaultResources).includes(resource)) {
      errors.push(`orgUnitPolicy.forbiddenDefaultResources must include ${resource}`);
    }
    if (!values(topology.defaultFoundation?.mustExcludeResources).includes(resource)) {
      errors.push(`default foundation must exclude parallel governance resource ${resource}`);
    }
  }
}

function validateDefaultFoundationPolicy(topology, errors) {
  const requiredGovernanceResources = ["tenants", "org-units", "users", "roles", "role-groups", "area-codes"];
  const optionalPersonnelResources = ["personnel-profiles", "positions", "position-assignments"];
  const defaultIncludes = values(topology.defaultFoundation?.mustIncludeResources);
  const defaultExcludes = values(topology.defaultFoundation?.mustExcludeResources);
  const defaultExcludedCapabilities = values(topology.defaultFoundation?.mustExcludeCapabilities);
  const profileDefaultIncludes = values(topology.profileRequirements?.defaultMustIncludeResources);
  const profileDefaultExcludedCapabilities = values(topology.profileRequirements?.defaultMustExcludeCapabilities);

  for (const resource of requiredGovernanceResources) {
    if (!defaultIncludes.includes(resource)) {
      errors.push(`defaultFoundation.mustIncludeResources must include ${resource}`);
    }
    if (!profileDefaultIncludes.includes(resource)) {
      errors.push(`profileRequirements.defaultMustIncludeResources must include ${resource}`);
    }
  }
  for (const resource of optionalPersonnelResources) {
    if (defaultIncludes.includes(resource)) {
      errors.push(`defaultFoundation.mustIncludeResources must not include optional personnel resource ${resource}`);
    }
    if (!defaultExcludes.includes(resource)) {
      errors.push(`defaultFoundation.mustExcludeResources must include optional personnel resource ${resource}`);
    }
  }
  if (!defaultExcludedCapabilities.includes("personnel")) {
    errors.push("defaultFoundation.mustExcludeCapabilities must include personnel");
  }
  if (!profileDefaultExcludedCapabilities.includes("personnel")) {
    errors.push("profileRequirements.defaultMustExcludeCapabilities must include personnel");
  }
}

function validateGovernanceEvaluation(topology, errors) {
  const evaluation = topology.governanceEvaluation ?? {};
  const tenantOnly = evaluation.tenantOnlySupport ?? {};
  if (tenantOnly.decision !== "rejected") {
    errors.push("governanceEvaluation.tenantOnlySupport.decision must stay rejected");
  }
  if (tenantOnly.requiredDefaultResource !== "org-units") {
    errors.push("governanceEvaluation.tenantOnlySupport.requiredDefaultResource must stay org-units");
  }
  if (tenantOnly.requiredTenantBoundary !== "org-units.tenantCode") {
    errors.push("governanceEvaluation.tenantOnlySupport.requiredTenantBoundary must stay org-units.tenantCode");
  }
  if (!includesAll(values(tenantOnly.supportedOrgUnitTypes), requiredOrgUnitTypeOptions)) {
    errors.push(`governanceEvaluation.tenantOnlySupport.supportedOrgUnitTypes must include ${requiredOrgUnitTypeOptions.join(", ")}`);
  }
  if (tenantOnly.parallelDefaultResources !== "forbidden") {
    errors.push("governanceEvaluation.tenantOnlySupport.parallelDefaultResources must stay forbidden");
  }

  const roleGroups = evaluation.roleGroups ?? {};
  if (roleGroups.decision !== "supported-as-classification") {
    errors.push("governanceEvaluation.roleGroups.decision must stay supported-as-classification");
  }
  if (roleGroups.resource !== "role-groups") {
    errors.push("governanceEvaluation.roleGroups.resource must stay role-groups");
  }
  if (roleGroups.roleField !== "roles.groupCode") {
    errors.push("governanceEvaluation.roleGroups.roleField must stay roles.groupCode");
  }
  if (roleGroups.treeField !== "role-groups.parentCode") {
    errors.push("governanceEvaluation.roleGroups.treeField must stay role-groups.parentCode");
  }
  for (const semantic of ["permission-grant", "role-membership", "permission-inheritance", "data-scope-ownership"]) {
    if (!values(roleGroups.forbiddenSemantics).includes(semantic)) {
      errors.push(`governanceEvaluation.roleGroups.forbiddenSemantics must include ${semantic}`);
    }
  }
  if (roleGroups.policyOwners?.permissionOwner !== topology.roleGroupPolicy?.permissionOwner) {
    errors.push("governanceEvaluation.roleGroups.policyOwners.permissionOwner must match roleGroupPolicy.permissionOwner");
  }
  if (roleGroups.policyOwners?.membershipOwner !== topology.roleGroupPolicy?.membershipOwner) {
    errors.push("governanceEvaluation.roleGroups.policyOwners.membershipOwner must match roleGroupPolicy.membershipOwner");
  }
  if (roleGroups.policyOwners?.dataScopeOwner !== topology.roleGroupPolicy?.dataScopeOwner) {
    errors.push("governanceEvaluation.roleGroups.policyOwners.dataScopeOwner must match roleGroupPolicy.dataScopeOwner");
  }

  const areaCodes = evaluation.areaCodes ?? {};
  if (areaCodes.decision !== "supported-as-shared-master-data") {
    errors.push("governanceEvaluation.areaCodes.decision must stay supported-as-shared-master-data");
  }
  if (areaCodes.resource !== "area-codes") {
    errors.push("governanceEvaluation.areaCodes.resource must stay area-codes");
  }
  if (areaCodes.defaultResourceRequired !== topology.areaCodePolicy?.defaultResourceRequired) {
    errors.push("governanceEvaluation.areaCodes.defaultResourceRequired must match areaCodePolicy.defaultResourceRequired");
  }
  if (areaCodes.attachmentRequiredByDefault !== topology.areaCodePolicy?.attachmentRequiredByDefault) {
    errors.push("governanceEvaluation.areaCodes.attachmentRequiredByDefault must match areaCodePolicy.attachmentRequiredByDefault");
  }
  if (areaCodes.authorizationConsumer !== topology.areaCodePolicy?.dataScopeOwner) {
    errors.push("governanceEvaluation.areaCodes.authorizationConsumer must match areaCodePolicy.dataScopeOwner");
  }
  if (areaCodes.implicitAuthorization !== topology.areaCodePolicy?.implicitAuthorization) {
    errors.push("governanceEvaluation.areaCodes.implicitAuthorization must match areaCodePolicy.implicitAuthorization");
  }
  for (const field of ["tenants.areaCode", "org-units.areaCode", "users.areaCode"]) {
    if (!values(areaCodes.defaultAttachments).includes(field)) {
      errors.push(`governanceEvaluation.areaCodes.defaultAttachments must include ${field}`);
    }
    if (!values(topology.areaCodePolicy?.attachmentFields).includes(field)) {
      errors.push(`areaCodePolicy.attachmentFields must include default governance evaluation field ${field}`);
    }
  }
  if (!values(areaCodes.optionalAttachments).includes("personnel-profiles.areaCode")) {
    errors.push("governanceEvaluation.areaCodes.optionalAttachments must include personnel-profiles.areaCode");
  }
  if (areaCodes.detailedAddressesDefault !== topology.areaCodePolicy?.detailedAddressPolicy?.defaultFoundation) {
    errors.push("governanceEvaluation.areaCodes.detailedAddressesDefault must match areaCodePolicy.detailedAddressPolicy.defaultFoundation");
  }

  const personnel = evaluation.personnel ?? {};
  if (personnel.decision !== "optional-capability") {
    errors.push("governanceEvaluation.personnel.decision must stay optional-capability");
  }
  if (personnel.capability !== topology.personnelBoundary?.capability) {
    errors.push("governanceEvaluation.personnel.capability must match personnelBoundary.capability");
  }
  if (personnel.profile !== topology.personnelBoundary?.profile) {
    errors.push("governanceEvaluation.personnel.profile must match personnelBoundary.profile");
  }
}

function validateOrganizationRbacMenuMigration(topology, errors) {
  const migration = topology.organizationRbacMenuMigration ?? {};
  if (migration.status !== "backend-and-migration-implemented-ui-menu-e2e-pending") {
    errors.push("organizationRbacMenuMigration.status must keep backend and migration implemented while UI, menu and E2E remain pending");
  }
  if (migration.designStatus !== "frozen") {
    errors.push("organizationRbacMenuMigration.designStatus must stay frozen after the contract node closes");
  }
  if (migration.designContract !== "resources/platform-organization-rbac-menu-contract.json" || !existingRelativePath(migration.designContract)) {
    errors.push("organizationRbacMenuMigration.designContract must reference the frozen organization RBAC menu contract");
  }
  if (!sameList(values(migration.taskIds), requiredOrganizationMigrationTaskIDs)) {
    errors.push("organizationRbacMenuMigration.taskIds must match the approved six-node migration lane");
  }
  const requiredSource = {
    userTenant: "direct-required",
    primaryOrganization: "optional",
    roleGroupHierarchy: "nested-parentCode",
    roleMembership: "users.roles",
    menuVisibility: "permission-derived",
    roleMenuBinding: "absent",
  };
  for (const [key, value] of Object.entries(requiredSource)) {
    if (migration.sourceModel?.[key] !== value) {
      errors.push(`organizationRbacMenuMigration.sourceModel.${key} must stay ${value}`);
    }
  }
  const requiredTarget = {
    roleGroupHierarchy: "flat-two-level-role-group-to-role",
    roleGroupScope: "platform-or-tenant",
    roleOwnership: "exactly-one-role-group",
    organizationRoleGroupBinding: "org_unit_role_groups",
    organizationBindingInheritance: "none-direct-bindings-only",
    organizationRolePool: "enabled-union-of-enabled-bound-tenant-groups",
    userTenant: "derived-from-primary-organization",
    userRoleConstraint: "subset-of-organization-role-pool",
    platformPrincipalException: "no-organization-platform-roles-only",
    roleMenuBinding: "role_menu",
    roleMenuStoredNodes: "page-only",
    writeEnforcement: "backend-all-write-import-bulk-paths",
    menuMigration: "backfill-dual-read-compare-switch-deprecate-menu-permission",
    authorizationDatasourceBoundary: "single-tenant-single-datasource",
    federationAndXa: "forbidden-for-authorization",
  };
  for (const [key, value] of Object.entries(requiredTarget)) {
    if (migration.targetModel?.[key] !== value) {
      errors.push(`organizationRbacMenuMigration.targetModel.${key} must stay ${value}`);
    }
  }
  if (!sameList(values(migration.targetModel?.menuNodeTypes), ["directory", "page"])) {
    errors.push("organizationRbacMenuMigration.targetModel.menuNodeTypes must be directory then page");
  }
  if (!sameList(values(migration.targetModel?.permissionResourceTypes), ["api", "page-button"])) {
    errors.push("organizationRbacMenuMigration.targetModel.permissionResourceTypes must be api then page-button");
  }
}

function validateGeneratedDefaultBoundary(topology, generatedContract, errors) {
  const generatedCodes = new Set(values(generatedContract.resources).map((resource) => resource.code));
  for (const resource of values(topology.defaultFoundation?.mustIncludeResources)) {
    if (!generatedCodes.has(resource)) {
      errors.push(`generated default admin contract must include ${resource}`);
    }
  }
  for (const resource of values(topology.defaultFoundation?.mustExcludeResources)) {
    if (generatedCodes.has(resource)) {
      errors.push(`generated default admin contract must not include optional/business resource ${resource}`);
    }
  }
}

function validateProfiles(topology, profilesDoc, errors) {
  const profiles = new Map(values(profilesDoc.profiles).map((profile) => [profile.id, profile]));
  for (const profileID of values(topology.profileRequirements?.defaultProfiles)) {
    const profile = profiles.get(profileID);
    if (!profile) {
      errors.push(`governance topology references missing default profile ${profileID}`);
      continue;
    }
    const resources = new Set(values(profile.mustIncludeResources));
    for (const resource of values(topology.profileRequirements?.defaultMustIncludeResources)) {
      if (!resources.has(resource)) {
        errors.push(`profile ${profileID} must include governance resource ${resource}`);
      }
    }
    const capabilities = new Set(values(profile.capabilities));
    const excludedCapabilities = new Set(values(profile.mustExcludeCapabilities));
    for (const capability of values(topology.profileRequirements?.defaultMustExcludeCapabilities)) {
      if (capabilities.has(capability) || !excludedCapabilities.has(capability)) {
        errors.push(`profile ${profileID} must exclude optional capability ${capability}`);
      }
    }
  }

  const personnelProfileID = topology.profileRequirements?.personnelProfile;
  const personnelProfile = profiles.get(personnelProfileID);
  if (!personnelProfile) {
    errors.push(`governance topology references missing personnel profile ${personnelProfileID}`);
    return;
  }
  const personnelResources = new Set(values(personnelProfile.mustIncludeResources));
  for (const resource of values(topology.profileRequirements?.personnelMustIncludeResources)) {
    if (!personnelResources.has(resource)) {
      errors.push(`profile ${personnelProfileID} must include personnel governance resource ${resource}`);
    }
  }
  if (!values(personnelProfile.capabilities).includes(topology.personnelBoundary?.capability)) {
    errors.push(`profile ${personnelProfileID} must enable personnel capability ${topology.personnelBoundary?.capability}`);
  }
  if (personnelProfile.business === true) {
    errors.push(`profile ${personnelProfileID} must remain a reusable platform extension, not a business profile`);
  }
}

function runAdminResourceContractForProfile(profile, errors) {
  if (personnelAdminContractPath) {
    const contractPath = path.resolve(repoRoot, personnelAdminContractPath);
    return readJSON(contractPath);
  }
  const result = spawnSync("go", ["run", "./cmd/platform-contracts", "admin-resources", "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      PLATFORM_CAPABILITIES: values(profile.capabilities).join(","),
    },
  });
  if (result.status !== 0) {
    errors.push(`profile ${profile.id} failed admin resource contract generation\n${result.stdout}${result.stderr}`);
    return null;
  }
  try {
    return JSON.parse(result.stdout);
  } catch (error) {
    errors.push(`profile ${profile.id} admin resource contract is not valid JSON: ${error.message}`);
    return null;
  }
}

function validatePersonnelContract(topology, profilesDoc, errors) {
  const profileID = topology.personnelBoundary?.profile;
  const profile = values(profilesDoc.profiles).find((item) => item.id === profileID);
  if (!profile) {
    errors.push(`personnel boundary references missing profile ${profileID}`);
    return;
  }
  const contract = runAdminResourceContractForProfile(profile, errors);
  if (!contract) {
    return;
  }
  const resources = new Map(values(contract.resources).map((resource) => [resource.resource ?? resource.code, resource]));
  for (const resource of values(topology.personnelBoundary?.resources)) {
    if (!resources.has(resource)) {
      errors.push(`personnel profile contract must include ${resource}`);
    }
  }
  for (const contractResource of values(topology.personnelBoundary?.resourceContracts)) {
    const resource = resources.get(contractResource.resource);
    if (!resource) {
      errors.push(`personnel resource contract ${contractResource.resource} is missing from generated profile contract`);
      continue;
    }
    for (const field of values(contractResource.fields)) {
      validateFieldContract(resource, field, errors);
    }
  }
}

function validateMatrix(topology, matrix, errors) {
  const capability = values(matrix.capabilities).find((item) => item.id === "governance-org-area-role-groups");
  if (!capability) {
    errors.push("engineering matrix must include governance-org-area-role-groups");
    return;
  }
  const sourcePaths = values(capability.evidence?.sourcePaths);
  const validators = values(capability.evidence?.validators);
  if (!sourcePaths.includes("resources/platform-governance-topology.json")) {
    errors.push("governance engineering capability must cite resources/platform-governance-topology.json");
  }
  if (!validators.includes("scripts/validate-platform-governance-topology.mjs")) {
    errors.push("governance engineering capability must cite validate-platform-governance-topology.mjs");
  }
  for (const documentPath of values(topology.documents)) {
    if (!existingRelativePath(documentPath)) {
      errors.push(`governance document path is missing: ${documentPath}`);
    }
  }
}

function validatePolicyFlags(topology, errors) {
  validateGovernanceEvaluation(topology, errors);
  validateOrganizationRbacMenuMigration(topology, errors);
  validateDefaultFoundationPolicy(topology, errors);
  validateOrgUnitPolicy(topology, errors);
  if (topology.roleGroupPolicy?.mode !== "classification-only") {
    errors.push("roleGroupPolicy.mode must stay classification-only");
  }
  if (topology.roleGroupPolicy?.permissionOwner !== "roles") {
    errors.push("roleGroupPolicy.permissionOwner must stay roles");
  }
  if (topology.roleGroupPolicy?.membershipOwner !== "users.roles") {
    errors.push("roleGroupPolicy.membershipOwner must stay users.roles");
  }
  if (topology.roleGroupPolicy?.dataScopeOwner !== "roles.dataScope") {
    errors.push("roleGroupPolicy.dataScopeOwner must stay roles.dataScope");
  }
  if (topology.roleGroupPolicy?.inheritance !== "forbidden") {
    errors.push("roleGroupPolicy.inheritance must stay forbidden");
  }
  if (topology.areaCodePolicy?.mode !== "shared-master-data") {
    errors.push("areaCodePolicy.mode must stay shared-master-data");
  }
  if (topology.areaCodePolicy?.defaultResourceRequired !== true) {
    errors.push("areaCodePolicy.defaultResourceRequired must stay true");
  }
  if (topology.areaCodePolicy?.attachmentRequiredByDefault !== false) {
    errors.push("areaCodePolicy.attachmentRequiredByDefault must stay false");
  }
  if (topology.areaCodePolicy?.implicitAuthorization !== false) {
    errors.push("areaCodePolicy.implicitAuthorization must stay false");
  }
  for (const resource of ["tenants", "org-units", "users"]) {
    if (!values(topology.areaCodePolicy?.defaultConsumers).includes(resource)) {
      errors.push(`areaCodePolicy.defaultConsumers must include ${resource}`);
    }
  }
  if (!values(topology.areaCodePolicy?.optionalConsumers).includes("personnel-profiles")) {
    errors.push("areaCodePolicy.optionalConsumers must include personnel-profiles");
  }
  if (topology.areaCodePolicy?.dataScopeOwner !== "roles.dataScopeAreaCodes") {
    errors.push("areaCodePolicy.dataScopeOwner must stay roles.dataScopeAreaCodes");
  }
  if (!includesAll(values(topology.areaCodePolicy?.levels), requiredAreaCodeLevelOptions)) {
    errors.push(`areaCodePolicy.levels must include ${requiredAreaCodeLevelOptions.join(", ")}`);
  }
  for (const field of ["tenants.areaCode", "org-units.areaCode", "users.areaCode", "personnel-profiles.areaCode"]) {
    if (!values(topology.areaCodePolicy?.attachmentFields).includes(field)) {
      errors.push(`areaCodePolicy.attachmentFields must include ${field}`);
    }
  }
  if (!values(topology.areaCodePolicy?.authorizationConsumers).includes("roles.dataScopeAreaCodes")) {
    errors.push("areaCodePolicy.authorizationConsumers must include roles.dataScopeAreaCodes");
  }
  if (topology.areaCodePolicy?.detailedAddressPolicy?.defaultFoundation !== "excluded") {
    errors.push("areaCodePolicy.detailedAddressPolicy.defaultFoundation must stay excluded");
  }
  if (topology.areaCodePolicy?.detailedAddressPolicy?.owner !== "owning-capability") {
    errors.push("areaCodePolicy.detailedAddressPolicy.owner must stay owning-capability");
  }
  if (topology.areaCodePolicy?.detailedAddressPolicy?.promotionRule !== "promote-only-after-two-reusable-capabilities") {
    errors.push("areaCodePolicy.detailedAddressPolicy.promotionRule must stay promote-only-after-two-reusable-capabilities");
  }
  for (const resource of ["user-addresses", "addresses"]) {
    if (!values(topology.defaultFoundation?.mustExcludeResources).includes(resource)) {
      errors.push(`defaultFoundation.mustExcludeResources must include detailed address resource ${resource}`);
    }
  }
  if (topology.personnelBoundary?.defaultCapability !== "excluded") {
    errors.push("personnelBoundary.defaultCapability must stay excluded");
  }
}

function validate() {
  const topology = readJSON(topologyPath);
  const manifest = readJSON(manifestPath);
  const generatedContract = readJSON(generatedContractPath);
  const capabilityAdminContract = readJSON(capabilityAdminContractPath);
  const profiles = readJSON(profilesPath);
  const matrix = readJSON(matrixPath);
  const errors = [];

  if (!topology.purpose) {
    errors.push("governance topology purpose is required");
  }
  validatePolicyFlags(topology, errors);
  validateResourceContracts(topology, manifest.resources, errors, "static admin manifest");
  validateResourceContracts(topology, capabilityAdminContract.resources, errors, "generated capability admin contract");
  validateGeneratedDefaultBoundary(topology, generatedContract, errors);
  validateProfiles(topology, profiles, errors);
  validatePersonnelContract(topology, profiles, errors);
  validateMatrix(topology, matrix, errors);

  return { topology, errors };
}

const { topology, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated platform governance topology in ${path.relative(repoRoot, topologyPath)} (${values(topology.resourceContracts).length} resource contracts)`);
