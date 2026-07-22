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
const exampleRelativeDir = "examples/external-capability";
const rootGoTestCommand = "rtk go -C examples/external-capability test ./...";
const rootGoRunCommand = "rtk go -C examples/external-capability run .";
const publicAppImport = "github.com/GnosiST/platform-go/pkg/platform/app";

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
  let importsPublicApp = false;
  let mountsRunnableAppHandler = false;
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
    if (source.includes(publicAppImport)) {
      importsPublicApp = true;
    }
    if (source.includes("app.NewRouter(") && source.includes("app.Registration{")) {
      mountsRunnableAppHandler = true;
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
  if (!importsPublicApp) {
    errors.push("external capability example must import pkg/platform/app for runnable App handlers");
  }
  if (!mountsRunnableAppHandler) {
    errors.push("external capability example must mount a runnable App handler through app.NewRouter");
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

function requireText(source, expected, label, errors) {
  if (!source.includes(expected)) {
    errors.push(`${label} must mention ${expected}`);
  }
}

function validateTextFile(filePath, requiredTexts, label, errors) {
  if (!fs.existsSync(filePath)) {
    errors.push(`${label} is missing: ${relative(filePath)}`);
    return "";
  }
  const source = fs.readFileSync(filePath, "utf8");
  for (const expected of requiredTexts) {
    requireText(source, expected, label, errors);
  }
  return source;
}

function validateTutorialDocs(errors) {
  if (!fs.existsSync(exampleDir) || !fs.statSync(exampleDir).isDirectory()) return;

  const readme = validateTextFile(
    path.join(exampleDir, "README.md"),
    [
      "No External Configuration Quick Start",
      "without a database, secrets or an external service",
      rootGoTestCommand,
      rootGoRunCommand,
      "business-project-template.json",
      "CatalogManifest()",
      publicCapabilityImport,
      publicAppImport,
      "NewExampleAppRouter()",
      "/settings",
      "demo data",
      "standalone nested Go module",
    ],
    "external capability README",
    errors,
  );
  for (const forbidden of ["exhibition", "exhibitor", "展会", "展商"]) {
    if (readme.includes(forbidden)) {
      errors.push(`external capability README must keep the sample domain business-neutral and avoid ${forbidden}`);
    }
  }

  validateTextFile(
    path.join(repoRoot, "docs/external-business-project-template.md"),
    [
      "本地可运行教程",
      rootGoTestCommand,
      rootGoRunCommand,
      "business-project-template.json",
      "CatalogManifest()",
      publicCapabilityImport,
      publicAppImport,
      "app.NewRouter",
      "demo data",
      "配置 key",
      "下游业务 capability",
    ],
    "external business project template doc",
    errors,
  );

  validateTextFile(
    path.join(repoRoot, "docs/platform-capability-development.md"),
    [
      "External Business Project Template",
      exampleRelativeDir,
      "docs/external-business-project-template.md",
      rootGoTestCommand,
      rootGoRunCommand,
      publicCapabilityImport,
      publicAppImport,
      "app.NewRouter",
      "downstream composition root",
    ],
    "platform capability development doc",
    errors,
  );
}

function templateManifestResource(template, resourceID) {
  const resources = template?.manifestSurface?.adminResources;
  if (!Array.isArray(resources)) return undefined;
  return resources.find((resource) => resource?.resource === resourceID);
}

function templateDemoDataSet(template, dataSetID) {
  const dataSets = template?.demoData?.dataSets;
  if (!Array.isArray(dataSets)) return undefined;
  return dataSets.find((dataSet) => dataSet?.id === dataSetID);
}

function validateTemplateJSON(errors) {
  if (!fs.existsSync(exampleDir) || !fs.statSync(exampleDir).isDirectory()) return;
  const template = readTemplateJSON(errors);
  if (template === null) return;

  if (template?.schema !== "platform-go.external-business-project-template.v1") {
    errors.push("business project template must keep schema platform-go.external-business-project-template.v1");
  }
  if (template?.businessCapability?.publicCapabilityImport !== publicCapabilityImport) {
    errors.push("business project template must use the public pkg/platform/capability import");
  }
  if (
    template?.businessCapability?.id !== "example-catalog" ||
    template?.businessCapability?.manifestFunction !== "CatalogManifest"
  ) {
    errors.push("business project template must identify the example-catalog CatalogManifest entrypoint");
  }
  if (template?.businessCapability?.mustStayOutsidePlatformDefaults !== true) {
    errors.push("business project template must mark the example business capability as outside platform defaults");
  }
  if (!String(template?.businessCapability?.replaceableExampleDomain ?? "").includes("placeholder domain")) {
    errors.push("business project template must state that catalog is a replaceable placeholder domain");
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
  if (
    template?.localTutorial?.requiresExternalConfiguration !== false ||
    template?.localTutorial?.requiresDatabase !== false ||
    template?.localTutorial?.requiresNetworkService !== false
  ) {
    errors.push("business project template local tutorial must run without external config, database or network service");
  }
  if (!String(template?.localTutorial?.moduleBoundary ?? "").includes("standalone nested Go module")) {
    errors.push("business project template must document the standalone nested Go module boundary");
  }
  const tutorialCommands = template?.localTutorial?.commands;
  if (!includesCommand(tutorialCommands, rootGoTestCommand) || !includesCommand(tutorialCommands, rootGoRunCommand)) {
    errors.push("business project template local tutorial must include root go -C test and run commands");
  }
  if (!includesValue(template?.manifestSurface?.appRoutes, "/api/app/catalog/items")) {
    errors.push("business project template manifest surface must declare the app route");
  }
  if (!includesValue(template?.manifestSurface?.authProviders, "catalog-partner")) {
    errors.push("business project template manifest surface must declare the auth provider");
  }
  if (
    !includesValue(template?.manifestSurface?.migrations, "catalog-0001") ||
    !includesValue(template?.manifestSurface?.seeds, "catalog-seed-0001")
  ) {
    errors.push("business project template manifest surface must declare migration and seed IDs");
  }
  if (!includesValue(template?.manifestSurface?.demoDataSets, "catalog-demo-items")) {
    errors.push("business project template manifest surface must declare demo data sets");
  }
  if (
    !includesValue(template?.manifestSurface?.serviceOperations, "catalog-list-items") ||
    !includesValue(template?.manifestSurface?.serviceEvents, "catalog-item-published")
  ) {
    errors.push("business project template manifest surface must declare service operation and event IDs");
  }
  const manifestItemResource = templateManifestResource(template, "catalog-items");
  if (!includesValue(manifestItemResource?.fields, "tenantCode") || !includesValue(manifestItemResource?.actions, "publish")) {
    errors.push("business project template manifest surface must include catalog item fields and publish action");
  }
  const manifestSettingsResource = templateManifestResource(template, "catalog-settings");
  if (manifestSettingsResource?.settingsEntry !== true || !includesValue(manifestSettingsResource?.fields, "partnerProvider")) {
    errors.push("business project template manifest surface must include catalog settings fields and settings entry flag");
  }
  const demoDataSet = templateDemoDataSet(template, "catalog-demo-items");
  if (demoDataSet?.resource !== "catalog-items" || !includesValue(demoDataSet?.records, "catalog-item-demo")) {
    errors.push("business project template demo data must bind catalog-demo-items to catalog-items and include the demo record");
  }
  if (!includesCommand(template?.validationCommands, "rtk node scripts/validate-external-capability-example.mjs")) {
    errors.push("business project template must include the maintained validator command");
  }
  if (
    !includesCommand(template?.validationCommands, rootGoTestCommand) ||
    !includesCommand(template?.validationCommands, rootGoRunCommand)
  ) {
    errors.push("business project template validation commands must include root go -C test and run commands");
  }
  if (!Array.isArray(template?.copyChecklist) || template.copyChecklist.length < 5) {
    errors.push("business project template must include a copy checklist for downstream projects");
  }
  if (
    !includesValue(template?.mainlineIntegrationNotes?.doNotAddBusinessCapabilityTo, "platform-default") ||
    !includesValue(template?.mainlineIntegrationNotes?.doNotAddBusinessCapabilityTo, "internal/platform/core")
  ) {
    errors.push("business project template must document mainline integration boundaries");
  }

  const templateText = JSON.stringify(template);
  for (const forbidden of ["exhibition", "exhibitor", "展会", "展商"]) {
    if (templateText.includes(forbidden)) {
      errors.push(`business project template must keep the sample domain business-neutral and avoid ${forbidden}`);
    }
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
  if (
    !Array.isArray(preview.configKeys) ||
    !preview.configKeys.includes("CATALOG_PARTNER_CLIENT_ID") ||
    !preview.configKeys.includes("CATALOG_PARTNER_CLIENT_SECRET")
  ) {
    errors.push("external capability preview must include auth provider config keys");
  }
  if (!Array.isArray(preview.appRoutes) || !preview.appRoutes.some((route) => route.path === "/api/app/catalog/items")) {
    errors.push("external capability preview must include /api/app/catalog/items app route");
  }
  if (
    !Array.isArray(preview.appRoutes) ||
    !preview.appRoutes.some((route) => {
      return route.path === "/api/app/catalog/items" && route.auth === "session" && route.permission === "app:catalog-item:read";
    })
  ) {
    errors.push("external capability preview must include session auth and app:catalog-item:read on the catalog app route");
  }
  if (!Array.isArray(preview.demoDataSets) || !preview.demoDataSets.includes("catalog-demo-items")) {
    errors.push("external capability preview must include catalog-demo-items demo data");
  }
  if (!Array.isArray(preview.migrations) || !preview.migrations.includes("catalog-0001")) {
    errors.push("external capability preview must include catalog-0001 migration");
  }
  if (!Array.isArray(preview.seeds) || !preview.seeds.includes("catalog-seed-0001")) {
    errors.push("external capability preview must include catalog-seed-0001 seed");
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
validateTutorialDocs(errors);
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

console.log("Validated external capability example: public imports, manifest, admin resources, settings entry, demo data, docs, go test ./..., go run .");
