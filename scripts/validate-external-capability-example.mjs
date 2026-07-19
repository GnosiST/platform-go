import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const exampleDir = path.resolve(repoRoot, argValue("--example-dir", "examples/external-capability"));
const publicCapabilityImport = "github.com/GnosiST/platform-go/pkg/platform/capability";

function relative(filePath) {
  return path.relative(repoRoot, filePath) || ".";
}

function run(command, args, cwd) {
  return spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    env: process.env,
  });
}

function walk(directory, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === ".git" || entry.name === "vendor") continue;
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(target, output);
    else if (entry.isFile()) output.push(target);
  }
  return output;
}

function validateExampleFiles(errors) {
  if (!fs.existsSync(exampleDir) || !fs.statSync(exampleDir).isDirectory()) {
    errors.push(`external capability example directory is missing: ${relative(exampleDir)}`);
    return;
  }

  const goModPath = path.join(exampleDir, "go.mod");
  if (!fs.existsSync(goModPath)) {
    errors.push(`external capability go.mod is missing: ${relative(goModPath)}`);
  } else {
    const goMod = fs.readFileSync(goModPath, "utf8");
    if (!goMod.includes("require github.com/GnosiST/platform-go")) {
      errors.push("external capability go.mod must require github.com/GnosiST/platform-go");
    }
    if (!goMod.includes("replace github.com/GnosiST/platform-go => ../..")) {
      errors.push("external capability go.mod must keep the local replace directive for repository validation");
    }
  }

  const goFiles = walk(exampleDir).filter((file) => file.endsWith(".go"));
  if (goFiles.length === 0) {
    errors.push("external capability example must contain Go source files");
  }

  let importsPublicCapability = false;
  let declaresManifest = false;
  let declaresAdminResource = false;
  let declaresPermissionPrefix = false;
  let declaresSettingsEntry = false;
  for (const file of goFiles) {
    const source = fs.readFileSync(file, "utf8");
    if (source.includes("github.com/GnosiST/platform-go/internal/") || source.includes("/internal/platform/")) {
      errors.push(`${relative(file)} must not import platform internal packages`);
    }
    if (source.includes(publicCapabilityImport)) {
      importsPublicCapability = true;
    }
    if (source.includes("capability.Manifest{")) {
      declaresManifest = true;
    }
    if (source.includes("capability.AdminResource{")) {
      declaresAdminResource = true;
    }
    if (source.includes("PermissionPrefix:")) {
      declaresPermissionPrefix = true;
    }
    if (/Resource:\s*"catalog-settings"/.test(source) && /Parent:\s*"configuration"/.test(source)) {
      declaresSettingsEntry = true;
    }
  }
  if (!importsPublicCapability) {
    errors.push("external capability example must import pkg/platform/capability");
  }
  if (!declaresManifest) {
    errors.push("external capability example must declare a capability.Manifest");
  }
  if (!declaresAdminResource) {
    errors.push("external capability example must declare capability.AdminResource entries");
  }
  if (!declaresPermissionPrefix) {
    errors.push("external capability example must declare admin PermissionPrefix values");
  }
  if (!declaresSettingsEntry) {
    errors.push("external capability example must declare a settings/config admin resource under the configuration menu parent");
  }
}

function validateGoCommand(command, args, cwd, errors) {
  const result = run(command, args, cwd);
  if (result.status !== 0) {
    errors.push(`${command} ${args.join(" ")} failed in ${relative(cwd)}:\n${result.stderr || result.stdout}`);
  }
  return result;
}

function readTemplateJSON(errors) {
  const templatePath = path.join(exampleDir, "business-project-template.json");
  if (!fs.existsSync(templatePath)) {
    errors.push(`external capability business project template is missing: ${relative(templatePath)}`);
    return null;
  }
  try {
    return JSON.parse(fs.readFileSync(templatePath, "utf8"));
  } catch (error) {
    errors.push(`external capability business project template must be JSON: ${error.message}`);
    return null;
  }
}

function templateResource(template, resourceID) {
  const resources = template?.adminSurface?.resources;
  if (!Array.isArray(resources)) return undefined;
  return resources.find((resource) => resource?.resource === resourceID);
}

function includesValue(values, expected) {
  return Array.isArray(values) && values.includes(expected);
}

function includesCommand(commands, expected) {
  if (!Array.isArray(commands)) return false;
  return commands.some((entry) => {
    if (typeof entry === "string") return entry === expected;
    return entry?.command === expected;
  });
}

function validateTemplateJSON(errors) {
  if (!fs.existsSync(exampleDir) || !fs.statSync(exampleDir).isDirectory()) return;
  const template = readTemplateJSON(errors);
  if (template === null) return;

  if (template?.businessCapability?.publicCapabilityImport !== publicCapabilityImport) {
    errors.push("business project template must use the public pkg/platform/capability import");
  }
  if (template?.businessCapability?.id !== "example-catalog" || template?.businessCapability?.manifestFunction !== "CatalogManifest") {
    errors.push("business project template must identify the example-catalog CatalogManifest entrypoint");
  }
  if (!String(template?.businessCapability?.registration ?? "").includes("downstream composition root")) {
    errors.push("business project template must register the manifest through a downstream composition root");
  }
  if (!includesValue(template?.platformBase?.mustNotModify, "internal/platform/**")) {
    errors.push("business project template must prohibit modifying internal/platform/** for business capabilities");
  }
  if (!includesValue(template?.platformBase?.mustNotModify, "internal/apps/**")) {
    errors.push("business project template must prohibit modifying internal/apps/** for business capabilities");
  }

  const itemResource = templateResource(template, "catalog-items");
  if (itemResource?.permissionPrefix !== "admin:catalog-item") {
    errors.push("business project template must declare catalog-items with admin:catalog-item permission prefix");
  }
  const settingsResource = templateResource(template, "catalog-settings");
  if (settingsResource?.permissionPrefix !== "admin:catalog-setting") {
    errors.push("business project template must declare catalog-settings with admin:catalog-setting permission prefix");
  }
  if (settingsResource?.menuParent !== "configuration" || settingsResource?.settingsEntry !== true) {
    errors.push("business project template must mark catalog-settings as a configuration/settings entry");
  }
  if (template?.configuration?.settingsCenter !== "/settings" || template?.configuration?.settingsResource !== "catalog-settings") {
    errors.push("business project template must route capability configuration through /settings and catalog-settings");
  }
  if (!includesValue(template?.configuration?.authProviderConfigKeys, "CATALOG_PARTNER_CLIENT_ID")) {
    errors.push("business project template must declare auth provider configuration keys");
  }
  if (!includesCommand(template?.validationCommands, "rtk node scripts/validate-external-capability-example.mjs")) {
    errors.push("business project template must include the maintained validator command");
  }
}

function validateRunOutput(stdout, errors) {
  let preview;
  try {
    preview = JSON.parse(stdout);
  } catch (error) {
    errors.push(`external capability go run output must be JSON: ${error.message}`);
    return;
  }
  if (preview.capabilityId !== "example-catalog") {
    errors.push(`external capability preview capabilityId = ${preview.capabilityId}, want example-catalog`);
  }
  if (!Array.isArray(preview.adminResources) || !preview.adminResources.includes("catalog-items")) {
    errors.push("external capability preview must include catalog-items admin resource");
  }
  if (!Array.isArray(preview.adminResources) || !preview.adminResources.includes("catalog-settings")) {
    errors.push("external capability preview must include catalog-settings admin resource");
  }
  if (!Array.isArray(preview.permissionPrefixes) || !preview.permissionPrefixes.includes("admin:catalog-item")) {
    errors.push("external capability preview must include admin:catalog-item permission prefix");
  }
  if (!Array.isArray(preview.permissionPrefixes) || !preview.permissionPrefixes.includes("admin:catalog-setting")) {
    errors.push("external capability preview must include admin:catalog-setting permission prefix");
  }
  if (!Array.isArray(preview.configResources) || !preview.configResources.includes("catalog-settings")) {
    errors.push("external capability preview must expose catalog-settings as a config resource");
  }
  const settingsContract = Array.isArray(preview.adminResourceContracts)
    ? preview.adminResourceContracts.find((resource) => resource.resource === "catalog-settings")
    : undefined;
  if (settingsContract?.menuParent !== "configuration") {
    errors.push("external capability preview must keep catalog-settings under the configuration menu parent");
  }
  if (!Array.isArray(preview.configKeys) || !preview.configKeys.includes("CATALOG_PARTNER_CLIENT_ID") || !preview.configKeys.includes("CATALOG_PARTNER_CLIENT_SECRET")) {
    errors.push("external capability preview must include auth provider config keys");
  }
  if (!Array.isArray(preview.appRoutes) || !preview.appRoutes.some((route) => route.path === "/api/app/catalog/items")) {
    errors.push("external capability preview must include /api/app/catalog/items app route");
  }
  if (typeof preview.serviceContractHash !== "string" || !preview.serviceContractHash.startsWith("sha256:")) {
    errors.push("external capability preview must include a service contract sha256 hash");
  }
  if (preview.serviceCount !== 1) {
    errors.push(`external capability preview serviceCount = ${preview.serviceCount}, want 1`);
  }
}

const errors = [];
validateExampleFiles(errors);
validateTemplateJSON(errors);
if (errors.length === 0) {
  validateGoCommand("go", ["test", "./..."], exampleDir, errors);
}
if (errors.length === 0) {
  const result = validateGoCommand("go", ["run", "."], exampleDir, errors);
  if (result.status === 0) validateRunOutput(result.stdout, errors);
}

if (errors.length > 0) {
  for (const error of errors) console.error(error);
  process.exit(1);
}

console.log("Validated external capability example: public imports, manifest, admin resources, settings entry, go test ./..., go run .");
