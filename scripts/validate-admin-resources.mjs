import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const defaultManifestPath = path.join(repoRoot, "resources", "admin-resources.json");
const manifestPath = optionValue("--manifest") ? path.resolve(repoRoot, optionValue("--manifest")) : defaultManifestPath;
const contractOptionPath = optionValue("--contract");
const contractPath = contractOptionPath ? path.resolve(repoRoot, contractOptionPath) : path.join(repoRoot, "resources", "generated", "admin-resource-contract.json");
const scaffoldPlanPath = path.join(repoRoot, "resources", "generated", "admin-scaffold-plan.json");
const scaffoldFilesPath = path.join(repoRoot, "resources", "generated", "admin-scaffold-files.json");
const allowedFieldTypes = new Set(["text", "textarea", "select", "multiselect", "datetime", "switch", "number", "color"]);
const allowedFormLayouts = new Set(["single-column", "grouped-sections", "two-column-density", "side-detail-preview"]);
const allowedCodegenModes = new Set(["scaffold", "readOnly", "readOnlySeeded", "custom"]);
const allowedRelationOperators = new Set(["contains", "=", "!=", ">", ">=", "<", "<="]);
const allowedRelationDisplays = new Set(["select", "tree"]);
const allowedActionKinds = new Set(["row", "batch", "resource"]);
const allowedActionTones = new Set(["default", "primary", "danger", "warning"]);
const allowedActionMethods = new Set(["GET", "POST", "PUT", "PATCH", "DELETE"]);
const allowedPanelKinds = new Set(["fields", "permissions", "audit", "approval", "files", "custom"]);
const allowedFieldSensitivities = new Set(["public", "internal", "personal", "sensitive", "secret"]);
const allowedFieldStorageModes = new Set(["plain", "masked", "hashed", "encrypted"]);
const allowedFieldProjectionModes = new Set(["full", "masked", "privileged", "omitted"]);
const recordFieldKeys = new Set(["id", "code", "name", "status", "description", "updatedAt"]);
const permissionPattern = /^admin:[a-z0-9-]+:[a-z0-9-]+$/;
const requiredOrgUnitTypeOptions = ["group", "company", "branch", "organization", "department", "team", "store", "custom"];
const defaultStaticExcludedResources = new Map([
  ["app-phone-verifications", "app-phone"],
  ["app-phone-bindings", "app-phone"],
  ["personnel-profiles", "personnel"],
  ["positions", "personnel"],
  ["position-assignments", "personnel"],
  ["policy-reviews", "policy-review"],
  ["notification-templates", "notification"],
  ["notifications", "notification"],
  ["notification-deliveries", "notification"],
  ["job-definitions", "job"],
  ["job-runs", "job"],
  ["job-run-attempts", "job"],
]);

function optionValue(name) {
  const index = process.argv.indexOf(name);
  if (index < 0) {
    return "";
  }
  return process.argv[index + 1] ?? "";
}

function assertUnique(values, label) {
  const seen = new Set();
  const errors = [];
  for (const value of values) {
    if (!value) {
      errors.push(`${label} contains an empty value`);
      continue;
    }
    if (seen.has(value)) {
      errors.push(`${label} contains duplicate value: ${value}`);
    }
    seen.add(value);
  }
  return errors;
}

function permissionValues(resource) {
  return Object.values(resource.permissions ?? {}).filter(Boolean);
}

function hasLocalizedText(value) {
  return typeof value?.zh === "string" && value.zh.trim() !== "" && typeof value?.en === "string" && value.en.trim() !== "";
}

function isMaskedProjection(mode) {
  return mode === "masked" || mode === "omitted";
}

function validSecurityFieldPolicy(sensitivity, storageMode, responseMode, exportMode) {
  if (sensitivity === "personal" && storageMode === "masked") {
    return isMaskedProjection(responseMode) && isMaskedProjection(exportMode);
  }
  if (sensitivity === "public" || !["hashed", "encrypted"].includes(storageMode)) {
    return false;
  }
  return responseMode === "omitted" && exportMode === "omitted";
}

function isSecurityFieldName(key) {
  const normalized = String(key ?? "").trim().toLowerCase().replaceAll(/[_\-.]/g, "");
  let base = normalized;
  for (const suffix of ["hash", "digest"]) {
    if (base.endsWith(suffix)) {
      base = base.slice(0, -suffix.length);
    }
  }
  if (["code", "session"].includes(base) && base !== normalized) {
    return true;
  }
  if (["verificationcode", "debugcode", "providersubject", "phone", "phonenumber", "identitynumber", "idnumber", "email", "address", "detailedaddress", "sessionid", "sessionhandle", "sessiontoken"].includes(base)) {
    return true;
  }
  if (["password", "passwd", "token", "secret", "credential", "credentials", "sessionid", "sessionhandle", "sessiontoken", "session"].some((marker) => protectedNameMatch(base, marker))) {
    return true;
  }
  return ["email", "phone", "phonenumber", "address", "identitynumber", "idnumber", "providersubject"].some((suffix) => base.endsWith(suffix));
}

function protectedNameMatch(normalized, marker) {
  if (normalized === marker || normalized.endsWith(marker)) {
    return true;
  }
  if (!normalized.startsWith(marker)) {
    return false;
  }
  return !["prefix", "type", "count", "status", "expiresat", "issuedat", "createdat", "updatedat", "revokedat", "lastusedat"].some((suffix) => normalized.endsWith(suffix));
}

function runCommand(label, command, args = []) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(`${label} failed\n${result.stdout}${result.stderr}`);
  }
  return result.stdout;
}

function runScript(script, args = []) {
  return runCommand(script, process.execPath, [path.join(__dirname, script), ...args]);
}

function assertGeneratedFresh(script, outputPath) {
  const expected = runScript(script, ["--stdout"]);
  return assertOutputFresh(script, outputPath, expected);
}

function assertGeneratedFreshCommand(label, command, args, outputPath) {
  const expected = runCommand(label, command, args);
  return assertOutputFresh(label, outputPath, expected);
}

function assertValidatorPass(script) {
  try {
    runScript(script);
    return [];
  } catch (error) {
    return [error.message];
  }
}

function assertOutputFresh(label, outputPath, expected) {
  const absolutePath = path.join(repoRoot, outputPath);
  if (!fs.existsSync(absolutePath)) {
    return [`${outputPath} is missing; run ${label}`];
  }
  const actual = fs.readFileSync(absolutePath, "utf8");
  return actual === expected ? [] : [`${outputPath} is stale; rerun ${label}`];
}

function loadAdminResourceContractForValidation() {
  if (contractOptionPath) {
    return JSON.parse(fs.readFileSync(contractPath, "utf8"));
  }
  return JSON.parse(runScript("generate-admin-resource-contract.mjs", ["--stdout"]));
}

function validateGeneratedResourceContract(contract) {
  const errors = [];
  const schemas = contract.schemas ?? {};
  for (const resource of contract.resources ?? []) {
    const name = resource.name ?? resource.code ?? resource.refine?.resource;
    const prefix = `generated resource ${name}`;
    const schema = schemas[name];
    if (!name) {
      errors.push("generated resource is missing name");
      continue;
    }
    if (!schema) {
      errors.push(`${prefix} must declare schema`);
      continue;
    }
    for (const listName of ["fields", "table"]) {
      if (!Array.isArray(schema[listName]) || schema[listName].length === 0) {
        errors.push(`${prefix} must declare ${listName === "fields" ? "schema fields" : "table fields"}`);
      }
    }
    if (!Array.isArray(resource.permissionCodes) || resource.permissionCodes.length === 0) {
      errors.push(`${prefix} must declare permission codes`);
    }
    const queryable = (resource.routes ?? []).some((route) => route.method === "POST" && /\/quer(?:y|ies)$/.test(route.path ?? ""));
    if (queryable) {
      for (const listName of ["search", "filter", "sort"]) {
        if (!Array.isArray(schema[listName]) || schema[listName].length === 0) {
          errors.push(`${prefix} must declare ${listName} fields`);
        }
      }
    }
  }
  return errors;
}

function validateManifest() {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const resources = manifest.resources ?? [];
  const resourcesByCode = new Map(resources.map((resource) => [resource.code, resource]));
  const errors = [];

  if (manifest.stack?.backend?.join(",") !== "Gin,GORM,Casbin,JWT") {
    errors.push("stack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (manifest.stack?.frontend?.join(",") !== "Refine,React,Ant Design") {
    errors.push("stack.frontend must stay Refine + React + Ant Design");
  }
  if (manifest.database?.runtimeDefault !== "mysql") {
    errors.push("database.runtimeDefault must stay mysql unless the runtime plan changes explicitly");
  }
  if ((manifest.database?.supportedDrivers ?? []).includes("sqlite")) {
    errors.push("database.supportedDrivers must not list sqlite as a runtime driver; use database.testDriver instead");
  }
  if (manifest.database?.testDriver !== "sqlite") {
    errors.push("database.testDriver should be sqlite for tests");
  }

  errors.push(...assertUnique(resources.map((resource) => resource.name), "resource.name"));
  errors.push(...assertUnique(resources.map((resource) => resource.code), "resource.code"));

  for (const resource of resources) {
    const prefix = `resource ${resource.name}`;
    const optionalCapability = defaultStaticExcludedResources.get(resource.code) ?? defaultStaticExcludedResources.get(resource.name);
    if (optionalCapability) {
      errors.push(`${prefix} belongs to optional capability ${optionalCapability} and must be contributed by capability.Manifest, not resources/admin-resources.json`);
    }
    for (const field of ["name", "code", "label", "group", "menu", "refine", "apiBase", "permissions", "routes", "schema", "codegen"]) {
      if (!resource[field]) {
        errors.push(`${prefix} missing ${field}`);
      }
    }
    if (!hasLocalizedText(resource.label)) {
      errors.push(`${prefix} label must declare zh/en text`);
    }

    const permissions = permissionValues(resource);
    if (permissions.length === 0) {
      errors.push(`${prefix} must declare at least one permission`);
    }
    for (const permission of permissions) {
      if (!permissionPattern.test(permission)) {
        errors.push(`${prefix} has invalid permission code: ${permission}`);
      }
    }

    const menuPath = resource.menu?.path ?? "";
    if (!menuPath) {
      errors.push(`${prefix} menu.path is required`);
    } else if (resource.menu?.external) {
      if (!/^https?:\/\//.test(menuPath)) {
        errors.push(`${prefix} external menu.path must start with http:// or https://`);
      }
    } else if (!menuPath.startsWith("/")) {
      errors.push(`${prefix} internal menu.path must start with /`);
    }
    for (const booleanField of ["visible", "hidden", "external", "keepAlive"]) {
      if (typeof resource.menu?.[booleanField] !== "boolean") {
        errors.push(`${prefix} menu.${booleanField} must be boolean`);
      }
    }
    if (!resource.refine?.resource || !resource.refine?.list || !resource.refine?.component) {
      errors.push(`${prefix} refine must declare resource, list and component`);
    }

    const permissionSet = new Set(permissions);
    for (const route of resource.routes ?? []) {
      if (!route.method || !route.path || !route.permission) {
        errors.push(`${prefix} has incomplete route`);
        continue;
      }
      if (!route.path.startsWith("/api/")) {
        errors.push(`${prefix} route ${route.method} ${route.path} must start with /api/`);
      }
      if (!permissionSet.has(route.permission)) {
        errors.push(`${prefix} route ${route.method} ${route.path} uses undeclared permission ${route.permission}`);
      }
    }
    errors.push(...validateResourceActions(resource, permissionSet, prefix));
    errors.push(...validateResourcePanels(resource, permissionSet, prefix));

    const schema = resource.schema ?? {};
    if (schema.formLayout && !allowedFormLayouts.has(schema.formLayout)) {
      errors.push(`${prefix} schema.formLayout has unsupported layout ${schema.formLayout}`);
    }
    const formGroups = schema.formGroups ?? [];
    errors.push(...assertUnique(formGroups.map((group) => group.key), `${prefix} schema.formGroups.key`));
    const formGroupKeys = new Set();
    for (const group of formGroups) {
      if (!group.key) {
        errors.push(`${prefix} form group is missing key`);
      } else {
        formGroupKeys.add(group.key);
      }
      if (!hasLocalizedText(group.label)) {
        errors.push(`${prefix} form group ${group.key} must declare zh/en labels`);
      }
      if (group.description && !hasLocalizedText(group.description)) {
        errors.push(`${prefix} form group ${group.key} description must declare zh/en text`);
      }
    }
    const fields = schema.fields ?? [];
    errors.push(...assertUnique(fields.map((field) => field.key), `${prefix} schema.fields.key`));
    for (const field of fields) {
      if (!field.label?.zh || !field.label?.en) {
        errors.push(`${prefix} field ${field.key} must declare zh/en labels`);
      }
      if (field.help && !hasLocalizedText(field.help)) {
        errors.push(`${prefix} field ${field.key} help must declare zh/en text`);
      }
      if (field.group && formGroupKeys.size > 0 && !formGroupKeys.has(field.group)) {
        errors.push(`${prefix} field ${field.key} references unknown form group ${field.group}`);
      }
      if (!allowedFieldTypes.has(field.type)) {
        errors.push(`${prefix} field ${field.key} has unsupported type ${field.type}`);
      }
      const sensitivity = field.sensitivity ?? "public";
      const storageMode = field.storageMode ?? "plain";
      const responseMode = field.responseMode ?? "full";
      const exportMode = field.exportMode ?? "full";
      const source = field.source ?? (recordFieldKeys.has(field.key) ? "record" : "values");
      if (!allowedFieldSensitivities.has(sensitivity)) {
        errors.push(`${prefix} field ${field.key} has unsupported sensitivity ${sensitivity}`);
      }
      if (!allowedFieldStorageModes.has(storageMode)) {
        errors.push(`${prefix} field ${field.key} has unsupported storageMode ${storageMode}`);
      }
      if (!allowedFieldProjectionModes.has(responseMode)) {
        errors.push(`${prefix} field ${field.key} has unsupported responseMode ${responseMode}`);
      }
      if (!allowedFieldProjectionModes.has(exportMode)) {
        errors.push(`${prefix} field ${field.key} has unsupported exportMode ${exportMode}`);
      }
      if (["sensitive", "secret"].includes(sensitivity) && source === "record") {
        errors.push(`${prefix} field ${field.key} sensitive or secret values cannot use record storage`);
      }
      if (sensitivity === "personal" && storageMode === "plain") {
        errors.push(`${prefix} field ${field.key} personal values require masked or protected storage`);
      }
      if (["sensitive", "secret"].includes(sensitivity) && storageMode === "plain") {
        errors.push(`${prefix} field ${field.key} sensitive or secret values require protected storage`);
      }
      if (storageMode === "masked" && sensitivity !== "personal") {
        errors.push(`${prefix} field ${field.key} masked storage requires personal sensitivity`);
      }
      if (storageMode === "masked" && (!isMaskedProjection(responseMode) || !isMaskedProjection(exportMode))) {
        errors.push(`${prefix} field ${field.key} masked storage must use masked or omitted response and export`);
      }
      if (["hashed", "encrypted"].includes(storageMode) && (responseMode !== "omitted" || exportMode !== "omitted")) {
        errors.push(`${prefix} field ${field.key} protected storage must be omitted from response and export`);
      }
      if (isSecurityFieldName(field.key) && !validSecurityFieldPolicy(sensitivity, storageMode, responseMode, exportMode)) {
        errors.push(`${prefix} field ${field.key} security field names require masked personal or protected non-public storage`);
      }
      if (field.relation) {
        const relation = field.relation;
        const target = resourcesByCode.get(relation.resource);
        if (!target) {
          errors.push(`${prefix} field ${field.key} relation.resource references unknown resource ${relation.resource}`);
        }
        if (relation.multiple === true && field.type !== "multiselect") {
          errors.push(`${prefix} field ${field.key} relation.multiple requires field type multiselect`);
        }
        if (relation.multiple !== true && field.type !== "select") {
          errors.push(`${prefix} field ${field.key} relation requires field type select or multiselect`);
        }
        if (target) {
          const targetFields = new Set(["id", "code", "name", "status", "description", "updatedAt", ...(target.schema?.fields ?? []).map((targetField) => targetField.key)]);
          for (const relationField of ["valueField", "labelField", "sortField", "parentField", "pathField"]) {
            const value = relation[relationField];
            if (value && !targetFields.has(value)) {
              errors.push(`${prefix} field ${field.key} relation.${relationField} references unknown ${relation.resource}.${value}`);
            }
          }
          for (const filter of relation.filters ?? []) {
            if (!targetFields.has(filter.field)) {
              errors.push(`${prefix} field ${field.key} relation filter references unknown ${relation.resource}.${filter.field}`);
            }
            if (!allowedRelationOperators.has(filter.operator)) {
              errors.push(`${prefix} field ${field.key} relation filter has unsupported operator ${filter.operator}`);
            }
            if (typeof filter.value !== "string") {
              errors.push(`${prefix} field ${field.key} relation filter value must be a string`);
            }
          }
        }
        if (!relation.valueField || !relation.labelField) {
          errors.push(`${prefix} field ${field.key} relation must declare valueField and labelField`);
        }
        if (relation.sortOrder && relation.sortOrder !== "asc" && relation.sortOrder !== "desc") {
          errors.push(`${prefix} field ${field.key} relation.sortOrder must be asc or desc`);
        }
        if (relation.display && !allowedRelationDisplays.has(relation.display)) {
          errors.push(`${prefix} field ${field.key} relation.display must be select or tree`);
        }
        if (relation.display === "tree" && !relation.parentField) {
          errors.push(`${prefix} field ${field.key} tree relation must declare parentField`);
        }
        if (relation.rootValue != null && typeof relation.rootValue !== "string") {
          errors.push(`${prefix} field ${field.key} relation.rootValue must be a string`);
        }
      }
      const validation = field.validation;
      if (validation) {
        for (const numericKey of ["minLength", "maxLength", "min", "max"]) {
          if (validation[numericKey] != null && typeof validation[numericKey] !== "number") {
            errors.push(`${prefix} field ${field.key} validation.${numericKey} must be numeric`);
          }
        }
        if (validation.pattern != null && typeof validation.pattern !== "string") {
          errors.push(`${prefix} field ${field.key} validation.pattern must be a string`);
        }
      }
    }
    const fieldKeys = new Set(fields.map((field) => field.key));
    for (const listName of ["search", "filter", "sort", "table", "form", "localizedFields"]) {
      for (const key of schema[listName] ?? []) {
        if (!fieldKeys.has(key)) {
          errors.push(`${prefix} schema.${listName} references unknown field ${key}`);
        }
      }
    }
    if (!allowedCodegenModes.has(resource.codegen?.mode)) {
      errors.push(`${prefix} has unsupported codegen mode ${resource.codegen?.mode}`);
    }
  }
  errors.push(...validatePlatformGovernanceContract(resourcesByCode));

  return { resources, errors };
}

function fieldByKey(resource, key) {
  return (resource?.schema?.fields ?? []).find((field) => field.key === key);
}

function optionValues(field) {
  return new Set(
    (field?.options ?? [])
      .map((option) => (typeof option === "string" ? option : option?.value))
      .filter(Boolean),
  );
}

function hasOptions(field, values) {
  const options = optionValues(field);
  return values.every((value) => options.has(value));
}

function validateRelation(resource, fieldKey, targetResource, options = {}) {
  const field = fieldByKey(resource, fieldKey);
  if (!field || !field.relation || field.relation.resource !== targetResource) {
    return false;
  }
  if (options.type && field.type !== options.type) {
    return false;
  }
  if (options.multiple != null && field.relation.multiple !== options.multiple) {
    return false;
  }
  if (options.display && field.relation.display !== options.display) {
    return false;
  }
  if (options.parentField && field.relation.parentField !== options.parentField) {
    return false;
  }
  if (options.pathField && field.relation.pathField !== options.pathField) {
    return false;
  }
  return true;
}

function validatePlatformGovernanceContract(resourcesByCode) {
  const errors = [];
  const tenants = resourcesByCode.get("tenants");
  if (tenants && !validateRelation(tenants, "areaCode", "area-codes", { type: "select", display: "tree", parentField: "parentCode", pathField: "path" })) {
    errors.push("tenants must declare areaCode relation to area-codes");
  }

  const users = resourcesByCode.get("users");
  if (users) {
    if (fieldByKey(users, "tenantCode")?.required !== true) {
      errors.push("users tenantCode must be required");
    }
    if (fieldByKey(users, "orgUnitCode")?.required === true) {
      errors.push("users orgUnitCode must stay optional by default");
    }
    if (fieldByKey(users, "areaCode")?.required === true) {
      errors.push("users areaCode must stay optional by default");
    }
    if (!validateRelation(users, "tenantCode", "tenants", { type: "select" })) {
      errors.push("users must declare tenantCode relation to tenants");
    }
    if (!validateRelation(users, "orgUnitCode", "org-units", { type: "select", display: "tree", parentField: "parentCode" })) {
      errors.push("users must declare orgUnitCode relation to org-units");
    }
    if (!validateRelation(users, "areaCode", "area-codes", { type: "select", display: "tree", parentField: "parentCode", pathField: "path" })) {
      errors.push("users must declare areaCode relation to area-codes");
    }
    if (!validateRelation(users, "roles", "roles", { type: "multiselect", multiple: true })) {
      errors.push("users must declare roles relation to roles");
    }
  }

  const orgUnits = resourcesByCode.get("org-units");
  if (orgUnits) {
    const typeField = fieldByKey(orgUnits, "type");
    if (!typeField || typeField.type !== "select" || typeField.required !== true || !hasOptions(typeField, requiredOrgUnitTypeOptions)) {
      errors.push(`org-units must declare required type select options: ${requiredOrgUnitTypeOptions.join(", ")}`);
    }
    if (fieldByKey(orgUnits, "tenantCode")?.required !== true) {
      errors.push("org-units tenantCode must be required");
    }
    if (!validateRelation(orgUnits, "tenantCode", "tenants", { type: "select" })) {
      errors.push("org-units must declare tenantCode relation to tenants");
    }
    if (!validateRelation(orgUnits, "parentCode", "org-units", { type: "select", display: "tree", parentField: "parentCode" })) {
      errors.push("org-units must declare parentCode tree relation to org-units");
    }
    if (!validateRelation(orgUnits, "areaCode", "area-codes", { type: "select", display: "tree", parentField: "parentCode", pathField: "path" })) {
      errors.push("org-units must declare areaCode relation to area-codes");
    }
  }

  const areaCodes = resourcesByCode.get("area-codes");
  if (areaCodes) {
    if (!validateRelation(areaCodes, "parentCode", "area-codes", { type: "select", display: "tree", parentField: "parentCode", pathField: "path" })) {
      errors.push("area-codes must declare parentCode tree relation to area-codes");
    }
    const levelField = fieldByKey(areaCodes, "level");
    if (!levelField || levelField.type !== "select" || !hasOptions(levelField, ["country", "province", "city", "district", "street", "custom"])) {
      errors.push("area-codes must declare level select options");
    }
    const pathField = fieldByKey(areaCodes, "path");
    if (!pathField || pathField.type !== "text") {
      errors.push("area-codes must declare path hierarchy field");
    }
  }

  const roles = resourcesByCode.get("roles");
  if (roles) {
    if (!validateRelation(roles, "groupCode", "role-groups", { type: "select", display: "tree", parentField: "parentCode" })) {
      errors.push("roles must declare groupCode tree relation to role-groups");
    }
    const dataScopeField = fieldByKey(roles, "dataScope");
    if (
      !dataScopeField ||
      dataScopeField.type !== "select" ||
      dataScopeField.required !== true ||
      !hasOptions(dataScopeField, ["all", "current_org", "current_and_children", "custom_orgs", "current_area", "current_and_children_areas", "custom_areas", "self"])
    ) {
      errors.push("roles must declare required dataScope select options");
    }
    if (!validateRelation(roles, "dataScopeOrgCodes", "org-units", { type: "multiselect", multiple: true, display: "tree", parentField: "parentCode" })) {
      errors.push("roles must declare dataScopeOrgCodes relation to org-units");
    }
    if (!validateRelation(roles, "dataScopeAreaCodes", "area-codes", { type: "multiselect", multiple: true, display: "tree", parentField: "parentCode", pathField: "path" })) {
      errors.push("roles must declare dataScopeAreaCodes relation to area-codes");
    }
    if (!validateRelation(roles, "permissions", "permissions", { type: "multiselect", multiple: true })) {
      errors.push("roles must declare permissions relation to permissions");
    }
  }

  const roleGroups = resourcesByCode.get("role-groups");
  if (roleGroups) {
    if (!validateRelation(roleGroups, "parentCode", "role-groups", { type: "select", display: "tree", parentField: "parentCode" })) {
      errors.push("role-groups must declare parentCode tree relation to role-groups");
    }
    const forbiddenTerms = ["permission", "datascope", "scope", "inherit", "includedrole", "rolecodes", "membership", "membercodes", "usercodes"];
    const forbiddenRoleGroupFields = (roleGroups.schema?.fields ?? []).filter((field) => {
      const key = String(field.key ?? "").toLowerCase();
      return forbiddenTerms.some((term) => key.includes(term)) || key === "parentrolecode" || key === "parentrolegroupcode";
    });
    if (forbiddenRoleGroupFields.length > 0) {
      errors.push(`role-groups must stay classification-only and must not declare permission, inheritance or data-scope fields: ${forbiddenRoleGroupFields.map((field) => field.key).join(", ")}`);
    }
  }

  return errors;
}

function validateResourceActions(resource, permissionSet, prefix) {
  const errors = [];
  const actions = resource.actions ?? [];
  errors.push(...assertUnique(actions.map((action) => action.key), `${prefix} actions.key`));
  for (const action of actions) {
    const label = `${prefix} action ${action.key ?? "<missing>"}`;
    if (!action.key) {
      errors.push(`${label} key is required`);
    }
    if (!hasLocalizedText(action.label)) {
      errors.push(`${label} label must declare zh/en text`);
    }
    const kind = action.kind ?? "row";
    if (!allowedActionKinds.has(kind)) {
      errors.push(`${label} has unsupported kind ${kind}`);
    }
    const tone = action.tone ?? "default";
    if (!allowedActionTones.has(tone)) {
      errors.push(`${label} has unsupported tone ${tone}`);
    }
    if (!action.permission) {
      errors.push(`${label} permission is required`);
    } else if (!permissionSet.has(action.permission)) {
      errors.push(`${label} uses undeclared permission ${action.permission}`);
    }
    if (action.route) {
      if (!String(action.route).startsWith("/api/admin/")) {
        errors.push(`${label} route must start with /api/admin/`);
      }
      const method = String(action.method ?? "").toUpperCase();
      if (!allowedActionMethods.has(method)) {
        errors.push(`${label} method must be GET, POST, PUT, PATCH or DELETE`);
      }
    }
    if (tone === "danger" && !action.confirm) {
      errors.push(`${label} danger action requires confirmation`);
    }
    if (action.confirm && !hasLocalizedText(action.confirm.title)) {
      errors.push(`${label} confirm.title must declare zh/en text`);
    }
  }
  return errors;
}

function validateResourcePanels(resource, permissionSet, prefix) {
  const errors = [];
  const panels = resource.panels ?? [];
  errors.push(...assertUnique(panels.map((panel) => panel.key), `${prefix} panels.key`));
  for (const panel of panels) {
    const label = `${prefix} panel ${panel.key ?? "<missing>"}`;
    if (!panel.key) {
      errors.push(`${label} key is required`);
    }
    if (!hasLocalizedText(panel.label)) {
      errors.push(`${label} label must declare zh/en text`);
    }
    const kind = panel.kind ?? "custom";
    if (!allowedPanelKinds.has(kind)) {
      errors.push(`${label} has unsupported kind ${kind}`);
    }
    if (panel.permission && !permissionSet.has(panel.permission)) {
      errors.push(`${label} uses undeclared permission ${panel.permission}`);
    }
    const component = String(panel.component ?? "");
    if (component.includes("/") || component.includes("\\") || component.includes(".")) {
      errors.push(`${label} component must be a semantic key`);
    }
    if (panel.empty && !hasLocalizedText(panel.empty)) {
      errors.push(`${label} empty must declare zh/en text`);
    }
  }
  return errors;
}

function pathInsideRoot(relativePath, allowedRoot) {
  const root = path.resolve(repoRoot, allowedRoot);
  const target = path.resolve(repoRoot, relativePath);
  const relative = path.relative(root, target);
  return relative !== "" && !relative.startsWith("..") && !path.isAbsolute(relative);
}

function validateScaffoldPlan() {
  const errors = [];
  if (!fs.existsSync(scaffoldPlanPath)) {
    return ["resources/generated/admin-scaffold-plan.json is missing; run generate-admin-scaffold-plan.mjs"];
  }
  const plan = JSON.parse(fs.readFileSync(scaffoldPlanPath, "utf8"));
  if (plan.mode?.dryRun !== true) {
    errors.push("admin scaffold plan must run in dry-run mode");
  }
  if (plan.mode?.sourceWriting !== "disabled") {
    errors.push("admin scaffold plan must keep sourceWriting disabled");
  }
  if (plan.mode?.conflictDetection !== "enabled") {
    errors.push("admin scaffold plan must enable conflict detection");
  }
  const allowedRoots = plan.mode?.allowedWriteRoots ?? [];
  if (!allowedRoots.includes("resources/generated/scaffold/")) {
    errors.push("admin scaffold plan must restrict writes to resources/generated/scaffold/");
  }
  for (const resource of plan.resources ?? []) {
    for (const file of resource.candidateFiles ?? []) {
      if (!file.path || !pathInsideRoot(file.path, "resources/generated/scaffold")) {
        errors.push(`admin scaffold plan candidate path escapes generated scaffold root: ${file.path ?? "<missing>"}`);
      }
      if (file.conflict) {
        errors.push(`admin scaffold plan candidate has unresolved conflict: ${file.path}`);
      }
      if (!file.eventualRuntimeTarget) {
        errors.push(`admin scaffold plan candidate missing eventualRuntimeTarget: ${file.path}`);
      }
    }
  }
  if ((plan.summary?.conflictCount ?? 0) !== 0) {
    errors.push(`admin scaffold plan conflictCount must be 0, got ${plan.summary.conflictCount}`);
  }
  if ((plan.summary?.unsafePathCount ?? 0) !== 0) {
    errors.push(`admin scaffold plan unsafePathCount must be 0, got ${plan.summary.unsafePathCount}`);
  }
  return errors;
}

function validateScaffoldFiles() {
  const errors = assertGeneratedFresh("generate-admin-scaffold-files.mjs", "resources/generated/admin-scaffold-files.json");
  if (errors.length > 0) {
    return errors;
  }
  const manifest = JSON.parse(fs.readFileSync(scaffoldFilesPath, "utf8"));
  if (manifest.mode?.sourceWriting !== "disabled") {
    errors.push("admin scaffold files must keep sourceWriting disabled");
  }
  const allowedRoots = manifest.mode?.allowedWriteRoots ?? [];
  if (!allowedRoots.includes("resources/generated/scaffold/")) {
    errors.push("admin scaffold files must restrict writes to resources/generated/scaffold/");
  }
  for (const file of manifest.files ?? []) {
    if (!file.path || !pathInsideRoot(file.path, "resources/generated/scaffold")) {
      errors.push(`admin scaffold file path escapes generated scaffold root: ${file.path ?? "<missing>"}`);
      continue;
    }
    const absoluteFile = path.join(repoRoot, file.path);
    if (!fs.existsSync(absoluteFile)) {
      errors.push(`admin scaffold file is missing: ${file.path}`);
      continue;
    }
    const content = fs.readFileSync(absoluteFile, "utf8");
    if (!content.includes(manifest.mode?.generatedMarker ?? "")) {
      errors.push(`admin scaffold file is missing generated marker: ${file.path}`);
    }
    const digest = cryptoHash(content);
    if (digest !== file.contentSha256) {
      errors.push(`admin scaffold file hash mismatch: ${file.path}`);
    }
  }
  if ((manifest.summary?.fileCount ?? 0) !== (manifest.files ?? []).length) {
    errors.push("admin scaffold files summary.fileCount must match files length");
  }
  return errors;
}

function cryptoHash(content) {
  return crypto.createHash("sha256").update(content).digest("hex");
}

function run() {
  const { resources, errors } = validateManifest();
  if (manifestPath === defaultManifestPath || contractOptionPath) {
    errors.push(...validateGeneratedResourceContract(loadAdminResourceContractForValidation()));
  }
  if (errors.length === 0 && manifestPath === defaultManifestPath) {
    errors.push(...assertGeneratedFresh("generate-admin-resource-contract.mjs", "resources/generated/admin-resource-contract.json"));
    errors.push(...assertGeneratedFresh("generate-admin-openapi.mjs", "resources/generated/openapi.admin.json"));
    errors.push(...assertGeneratedFresh("generate-admin-codegen-preview.mjs", "resources/generated/admin-codegen-preview.json"));
    errors.push(...assertGeneratedFresh("generate-admin-scaffold-plan.mjs", "resources/generated/admin-scaffold-plan.json"));
    errors.push(...assertGeneratedFresh("generate-admin-scaffold-files.mjs", "resources/generated/admin-scaffold-files.json"));
    errors.push(...assertGeneratedFresh("generate-admin-scaffold-draft.mjs", "resources/generated/admin-scaffold-draft.md"));
    errors.push(...assertGeneratedFresh("generate-admin-scaffold-promotion-review.mjs", "resources/generated/admin-scaffold-promotion-review.json"));
    errors.push(...assertGeneratedFresh("generate-app-openapi.mjs", "resources/generated/openapi.app.json"));
    errors.push(...assertGeneratedFresh("generate-app-codegen-preview.mjs", "resources/generated/app-codegen-preview.json"));
    errors.push(
      ...assertGeneratedFreshCommand(
        "go run ./cmd/platform-contracts app-routes --stdout",
        "go",
        ["run", "./cmd/platform-contracts", "app-routes", "--stdout"],
        "resources/generated/app-route-contract.json",
      ),
    );
    errors.push(
      ...assertGeneratedFreshCommand(
        "go run ./cmd/platform-contracts admin-resources --stdout",
        "go",
        ["run", "./cmd/platform-contracts", "admin-resources", "--stdout"],
        "resources/generated/admin-capability-resource-contract.json",
      ),
    );
    errors.push(
      ...assertGeneratedFreshCommand(
        "go run ./cmd/platform-contracts audit --stdout",
        "go",
        ["run", "./cmd/platform-contracts", "audit", "--stdout"],
        "resources/generated/platform-capability-audit.json",
      ),
    );
    errors.push(...validateScaffoldPlan());
    errors.push(...validateScaffoldFiles());
    errors.push(...assertValidatorPass("validate-platform-governance-topology.mjs"));
    errors.push(...assertValidatorPass("validate-platform-capability-contracts.mjs"));
    errors.push(...assertValidatorPass("validate-platform-capability-profiles.mjs"));
    errors.push(...assertValidatorPass("validate-platform-reference-discovery.mjs"));
    errors.push(...assertValidatorPass("validate-platform-reference-coverage.mjs"));
    errors.push(...assertValidatorPass("validate-platform-engineering-capabilities.mjs"));
  }

  if (errors.length > 0) {
    console.error(errors.map((error) => `- ${error}`).join("\n"));
    process.exit(1);
  }
  console.log(`Validated ${resources.length} admin resources in ${path.relative(repoRoot, manifestPath)}`);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  run();
}
