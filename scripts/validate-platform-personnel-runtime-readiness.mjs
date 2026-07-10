import fs from "node:fs";
import os from "node:os";
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

const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const defaultAdminContractPath = path.resolve(repoRoot, argValue("--default-admin-contract", "resources/generated/admin-resource-contract.json"));
const defaultAuditPath = path.resolve(repoRoot, argValue("--default-audit", "resources/generated/platform-capability-audit.json"));
const personnelAdminContractPath = argValue("--personnel-admin-contract", "");
const personnelOpenAPIPath = argValue("--personnel-openapi", "");

const personnelResources = ["personnel-profiles", "positions", "position-assignments"];
const personnelPermissions = [
  "admin:personnel-profile:read",
  "admin:position:read",
  "admin:position-assignment:read",
];
const personnelOpenAPIPaths = [
  "/api/admin/resources/personnel-profiles/query",
  "/api/admin/resources/positions/query",
  "/api/admin/resources/position-assignments/query",
];
const personnelSchemas = ["PersonnelProfilesRecord", "PositionsRecord", "PositionAssignmentsRecord"];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function resourceID(resource) {
  return resource?.resource ?? resource?.code ?? resource?.name ?? "";
}

function runCommand(label, command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
    ...options,
    env: {
      ...process.env,
      ...(options.env ?? {}),
    },
  });
  if (result.status !== 0) {
    throw new Error(`${label} failed\n${result.stdout}${result.stderr}`);
  }
  return result.stdout;
}

function personnelProfileCapabilities(profilesDoc, errors) {
  const profile = values(profilesDoc.profiles).find((item) => item.id === "platform-personnel-ready");
  if (!profile) {
    errors.push("platform-personnel-ready profile is missing");
    return [];
  }
  if (!values(profile.capabilities).includes("personnel")) {
    errors.push("platform-personnel-ready profile must enable personnel");
  }
  if (profile.business === true) {
    errors.push("platform-personnel-ready profile must remain reusable platform extension");
  }
  return values(profile.capabilities);
}

function generatedPersonnelContract(capabilities) {
  if (personnelAdminContractPath) {
    return readJSON(path.resolve(repoRoot, personnelAdminContractPath));
  }
  return JSON.parse(
    runCommand("personnel admin resource contract generation", process.execPath, ["scripts/generate-admin-resource-contract.mjs", "--stdout"], {
      env: { PLATFORM_CAPABILITIES: capabilities.join(",") },
    }),
  );
}

function generatedPersonnelOpenAPI(contract) {
  if (personnelOpenAPIPath) {
    return readJSON(path.resolve(repoRoot, personnelOpenAPIPath));
  }
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-personnel-runtime-readiness-"));
  const contractPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(contractPath, `${JSON.stringify(contract, null, 2)}\n`);
  return JSON.parse(runCommand("personnel OpenAPI generation", process.execPath, ["scripts/generate-admin-openapi.mjs", "--stdout", "--contract", contractPath]));
}

function validateDefaultIsolation(defaultContract, defaultAudit, errors) {
  const defaultResources = new Set(values(defaultContract.resources).map(resourceID));
  const defaultCapabilities = new Set(values(defaultAudit.capabilities).map((capability) => capability.id));
  for (const resource of personnelResources) {
    if (defaultResources.has(resource)) {
      errors.push(`default admin contract must not include optional personnel resource ${resource}`);
    }
  }
  if (defaultCapabilities.has("personnel")) {
    errors.push("default capability audit must not enable personnel");
  }
}

function validatePersonnelContract(contract, errors) {
  const resources = new Set(values(contract.resources).map(resourceID));
  const frontend = new Set(values(contract.frontend).map((resource) => resource.resource));
  const permissions = new Set(values(contract.permissions));
  for (const resource of personnelResources) {
    if (!resources.has(resource)) {
      errors.push(`personnel contract missing resource ${resource}`);
    }
    if (!frontend.has(resource)) {
      errors.push(`personnel contract frontend must include ${resource}`);
    }
  }
  for (const permission of personnelPermissions) {
    if (!permissions.has(permission)) {
      errors.push(`personnel contract missing permission ${permission}`);
    }
  }
}

function validatePersonnelOpenAPI(openapi, errors) {
  const paths = openapi.paths ?? {};
  const schemas = openapi.components?.schemas ?? {};
  for (const openAPIPath of personnelOpenAPIPaths) {
    if (!paths[openAPIPath]) {
      errors.push(`personnel OpenAPI must include path ${openAPIPath}`);
    }
  }
  for (const schema of personnelSchemas) {
    if (!schemas[schema]) {
      errors.push(`personnel OpenAPI must include schema ${schema}`);
    }
  }
}

function validate() {
  const profiles = readJSON(profilesPath);
  const defaultContract = readJSON(defaultAdminContractPath);
  const defaultAudit = readJSON(defaultAuditPath);
  const errors = [];

  validateDefaultIsolation(defaultContract, defaultAudit, errors);
  const capabilities = personnelProfileCapabilities(profiles, errors);
  const personnelContract = generatedPersonnelContract(capabilities);
  const personnelOpenAPI = generatedPersonnelOpenAPI(personnelContract);
  validatePersonnelContract(personnelContract, errors);
  validatePersonnelOpenAPI(personnelOpenAPI, errors);

  return { errors };
}

const { errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated optional personnel runtime readiness");
