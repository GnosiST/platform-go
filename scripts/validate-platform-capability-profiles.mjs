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

const profilesPath = path.resolve(repoRoot, argValue("--profiles", "resources/platform-capability-profiles.json"));
const configPath = path.resolve(repoRoot, "internal/platform/config/config.go");
const appManifestsPath = path.resolve(repoRoot, "internal/apps/manifests.go");

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

function relativeExistingDirectory(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath) && fs.statSync(absolutePath).isDirectory();
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function normalizedRelativePath(relativePath) {
  return relativePath.split(path.sep).join("/");
}

function manifestGoSources(relativePath) {
  const absolutePath = path.resolve(repoRoot, relativePath);
  return fs
    .readdirSync(absolutePath, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".go") && !entry.name.endsWith("_test.go"))
    .map((entry) => fs.readFileSync(path.join(absolutePath, entry.name), "utf8"));
}

function containsGoManifestFile(relativePath) {
  for (const source of manifestGoSources(relativePath)) {
    if (/func\s+Manifests\s*\(\)\s*\[\]capability\.Manifest/.test(source)) {
      return true;
    }
  }
  return false;
}

function manifestPackageName(relativePath) {
  for (const source of manifestGoSources(relativePath)) {
    if (!/func\s+Manifests\s*\(\)\s*\[\]capability\.Manifest/.test(source)) {
      continue;
    }
    const match = source.match(/^package\s+([A-Za-z_][A-Za-z0-9_]*)/m);
    if (match) {
      return match[1];
    }
  }
  return "";
}

function registeredInAppCompositionRoot(relativePath) {
  if (!fs.existsSync(appManifestsPath)) {
    return false;
  }
  const source = fs.readFileSync(appManifestsPath, "utf8");
  const importPath = `platform-go/${normalizedRelativePath(relativePath)}`;
  const importMatch = source.match(new RegExp(`(?:^|\\n)\\s*(?:(\\w+)\\s+)?"${escapeRegExp(importPath)}"`));
  if (!importMatch) {
    return false;
  }
  const packageName = importMatch[1] || manifestPackageName(relativePath);
  if (!packageName) {
    return false;
  }
  return new RegExp(`\\b${escapeRegExp(packageName)}\\.Manifests\\s*\\(`).test(source);
}

function readDefaultCapabilities() {
  const source = fs.readFileSync(configPath, "utf8");
  const match = source.match(/var\s+defaultCapabilities\s*=\s*\[\]string\s*{([\s\S]*?)\n}/);
  if (!match) {
    throw new Error("cannot find defaultCapabilities in internal/platform/config/config.go");
  }
  return [...match[1].matchAll(/"([^"]+)"/g)].map((item) => item[1]);
}

function sameList(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

function runAudit(profile) {
  const result = spawnSync("go", ["run", "./cmd/platform-contracts", "audit", "--stdout"], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      PLATFORM_CAPABILITIES: profile.capabilities.join(","),
    },
  });
  if (result.status !== 0) {
    return {
      error: `profile ${profile.id} failed capability audit\n${result.stdout}${result.stderr}`,
    };
  }
  try {
    return { audit: JSON.parse(result.stdout) };
  } catch (error) {
    return { error: `profile ${profile.id} audit output is not valid JSON: ${error.message}` };
  }
}

function requireDeclaredResources(profile, resources, label, errors) {
  const declared = new Set(values(profile.mustIncludeResources));
  for (const resource of resources) {
    if (!declared.has(resource)) {
      errors.push(`profile ${profile.id} must declare ${label} resource ${resource} in mustIncludeResources`);
    }
  }
}

function validateGovernanceProfileDeclarations(profile, errors) {
  if (profile.id === "minimal-admin" || profile.id === "platform-default") {
    requireDeclaredResources(profile, ["tenants", "org-units", "users", "roles", "role-groups", "area-codes"], "governance", errors);
  }
  if (profile.id === "platform-personnel-ready") {
    requireDeclaredResources(
      profile,
      ["personnel-profiles", "positions", "position-assignments", "tenants", "org-units", "area-codes", "users"],
      "personnel governance",
      errors,
    );
  }
}

function validateProfile(profile, doc, defaultCapabilities) {
  const errors = [];
  const prefix = `profile ${profile.id ?? "<missing>"}`;
  if (!profile.id) {
    return ["profile is missing id"];
  }
  if (!profile.label?.zh || !profile.label?.en) {
    errors.push(`${prefix} must declare zh/en label`);
  }
  if (!profile.purpose) {
    errors.push(`${prefix} must declare purpose`);
  }
  const capabilities = values(profile.capabilities);
  if (capabilities.length === 0) {
    errors.push(`${prefix} must declare capabilities`);
  }
  errors.push(...uniqueErrors(capabilities, `${prefix}.capabilities`));

  const businessCapabilities = new Set(values(doc.businessCapabilities));
  const hasBusinessCapability = capabilities.some((capability) => businessCapabilities.has(capability));
  if (profile.default === true && hasBusinessCapability) {
    errors.push(`${prefix} is default and must not include business capabilities`);
  }
  if (profile.business !== true && hasBusinessCapability) {
    errors.push(`${prefix} includes business capabilities but does not declare business=true`);
  }
  const additionalManifestPaths = values(profile.requiresAdditionalManifests);
  errors.push(...uniqueErrors(additionalManifestPaths, `${prefix}.requiresAdditionalManifests`));
  if (hasBusinessCapability && additionalManifestPaths.length === 0) {
    errors.push(`${prefix} includes business capabilities and must declare requiresAdditionalManifests`);
  }
  for (const manifestPath of additionalManifestPaths) {
    if (!relativeExistingDirectory(manifestPath)) {
      errors.push(`${prefix} requiresAdditionalManifests path is missing or unsafe: ${manifestPath}`);
      continue;
    }
    if (!containsGoManifestFile(manifestPath)) {
      errors.push(`${prefix} requiresAdditionalManifests ${manifestPath} must contain at least one Go manifest file`);
      continue;
    }
    if (!registeredInAppCompositionRoot(manifestPath)) {
      errors.push(`${prefix} requiresAdditionalManifests ${manifestPath} is not registered in internal/apps/manifests.go`);
    }
  }
  if (profile.id === doc.runtimeDefault && !sameList(capabilities, defaultCapabilities)) {
    errors.push(`${prefix} capabilities must match config defaultCapabilities`);
  }
  validateGovernanceProfileDeclarations(profile, errors);

  if (errors.length > 0) {
    return errors;
  }

  const { audit, error } = runAudit({ ...profile, capabilities });
  if (error) {
    return [error];
  }

  const capabilityIDs = new Set();
  const resourceIDs = new Set();
  for (const capability of audit.capabilities ?? []) {
    capabilityIDs.add(capability.id);
    for (const resource of capability.adminResources ?? []) {
      resourceIDs.add(resource);
    }
  }
  for (const capability of capabilities) {
    if (!capabilityIDs.has(capability)) {
      errors.push(`${prefix} audit output missing capability ${capability}`);
    }
  }
  if (capabilityIDs.size !== capabilities.length) {
    errors.push(`${prefix} audit capability count ${capabilityIDs.size} does not match declared ${capabilities.length}`);
  }
  for (const capability of values(profile.mustIncludeCapabilities)) {
    if (!capabilityIDs.has(capability)) {
      errors.push(`${prefix} missing required capability ${capability}`);
    }
  }
  for (const capability of values(profile.mustExcludeCapabilities)) {
    if (capabilityIDs.has(capability)) {
      errors.push(`${prefix} must exclude capability ${capability}`);
    }
  }
  for (const resource of values(profile.mustIncludeResources)) {
    if (!resourceIDs.has(resource)) {
      errors.push(`${prefix} missing required resource ${resource}`);
    }
  }
  for (const resource of values(profile.mustExcludeResources)) {
    if (resourceIDs.has(resource)) {
      errors.push(`${prefix} must exclude resource ${resource}`);
    }
  }
  return errors;
}

function validate() {
  const doc = readJSON(profilesPath);
  const defaultCapabilities = readDefaultCapabilities();
  const errors = [];
  const profiles = values(doc.profiles);
  errors.push(...uniqueErrors(profiles.map((profile) => profile.id), "profiles.id"));
  if (!doc.runtimeDefault) {
    errors.push("runtimeDefault is required");
  }
  const defaultProfiles = profiles.filter((profile) => profile.default === true);
  if (defaultProfiles.length !== 1) {
    errors.push(`exactly one profile must declare default=true, got ${defaultProfiles.length}`);
  }
  const runtimeDefault = profiles.find((profile) => profile.id === doc.runtimeDefault);
  if (!runtimeDefault) {
    errors.push(`runtimeDefault profile ${doc.runtimeDefault} is missing`);
  } else if (runtimeDefault.default !== true) {
    errors.push(`runtimeDefault profile ${doc.runtimeDefault} must declare default=true`);
  }

  for (const profile of profiles) {
    errors.push(...validateProfile(profile, doc, defaultCapabilities));
  }
  return { doc, errors };
}

const { doc, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${doc.profiles?.length ?? 0} platform capability profiles in ${path.relative(repoRoot, profilesPath)}`);
