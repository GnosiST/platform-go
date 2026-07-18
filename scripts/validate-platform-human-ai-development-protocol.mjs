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

const protocolPath = path.resolve(repoRoot, argValue("--protocol", "resources/platform-human-ai-development-protocol.json"));

const requiredPrinciples = [
  "contract-first",
  "boundary-first-customization",
  "ai-readable-context",
  "machine-verified",
  "human-reviewed-risk",
];
const requiredDomains = [
  "api-interface-contracts",
  "admin-ui-contracts",
  "visual-system",
  "code-generation",
  "capability-lifecycle-operations",
  "data-security-governance",
  "docs-handoff",
];
const requiredCustomizationModes = [
  "external-business-package",
  "platform-extension-capability",
  "generated-scaffold-preview",
];
const requiredAcceptanceCommands = [
  "rtk node scripts/validate-platform-human-ai-development-protocol.mjs",
  "rtk node scripts/validate-platform-capability-operation-policy.mjs",
  "rtk node scripts/validate-external-capability-example.mjs",
  "rtk node scripts/validate-admin-resources.mjs",
  "rtk node scripts/validate-platform-admin-api-boundary.mjs",
  "rtk node scripts/validate-platform-app-client-api-boundary.mjs",
  "rtk node scripts/validate-admin-ui-contracts.mjs",
  "rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs",
  "rtk npm --prefix admin run build",
];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
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

function requireIDs(items, requiredIDs, label, errors) {
  const actual = new Set(values(items).map((item) => item.id));
  for (const id of requiredIDs) {
    if (!actual.has(id)) {
      errors.push(`${label} must include ${id}`);
    }
  }
}

function requireIncludes(items, requiredItems, label, errors) {
  const actual = new Set(values(items));
  for (const item of requiredItems) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function validateLocalizedLabel(item, label, errors) {
  if (typeof item.label?.zh !== "string" || item.label.zh.trim() === "") {
    errors.push(`${label} must declare label.zh`);
  }
  if (typeof item.label?.en !== "string" || item.label.en.trim() === "") {
    errors.push(`${label} must declare label.en`);
  }
}

function validateRelativePaths(protocol, errors) {
  const pathEntries = [];
  for (const [key, relativePath] of Object.entries(protocol.entrypoints ?? {})) {
    pathEntries.push([`entrypoints.${key}`, relativePath]);
  }
  for (const domain of values(protocol.domains)) {
    const prefix = `domain ${domain.id ?? "<missing>"}`;
    for (const relativePath of [
      ...values(domain.requiredDocs),
      ...values(domain.requiredGeneratedEvidence),
      ...values(domain.requiredValidators),
      ...values(domain.requiredTests),
    ]) {
      pathEntries.push([prefix, relativePath]);
    }
  }
  for (const relativePath of [...values(protocol.requiredValidators), ...values(protocol.requiredTests)]) {
    pathEntries.push(["protocol required path", relativePath]);
  }

  for (const [label, relativePath] of pathEntries) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`${label} path is missing or unsafe: ${relativePath || "<missing>"}`);
    }
  }
}

function validateSourceSnippets(protocol, errors) {
  for (const snippet of values(protocol.requiredSourceSnippets)) {
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

function validateDomain(domain, errors) {
  const prefix = `domain ${domain.id ?? "<missing>"}`;
  if (!domain.id) {
    errors.push("protocol domain is missing id");
  }
  if (values(domain.sourceOfTruth).length === 0) {
    errors.push(`${prefix} must declare sourceOfTruth`);
  }
  if (values(domain.requiredDocs).length === 0) {
    errors.push(`${prefix} must declare requiredDocs`);
  }
  if (values(domain.requiredValidators).length === 0) {
    errors.push(`${prefix} must declare requiredValidators`);
  }
  if (values(domain.requiredPractices).length === 0) {
    errors.push(`${prefix} must declare requiredPractices`);
  }
}

function validateProtocol(protocol) {
  const errors = [];

  if (!protocol.purpose || !protocol.purpose.includes("human and AI")) {
    errors.push("protocol purpose must describe human and AI contributors");
  }
  requireIncludes(protocol.audiences, ["human-developers", "ai-agents", "downstream-business-teams"], "audiences", errors);
  requireIDs(protocol.principles, requiredPrinciples, "principles", errors);
  requireIDs(protocol.customizationModes, requiredCustomizationModes, "customizationModes", errors);
  requireIDs(protocol.domains, requiredDomains, "domains", errors);
  requireIncludes(protocol.minimumAcceptanceCommands, requiredAcceptanceCommands, "minimumAcceptanceCommands", errors);
  requireIncludes(protocol.requiredValidators, [
    "scripts/validate-platform-human-ai-development-protocol.mjs",
    "scripts/validate-platform-capability-operation-policy.mjs",
    "scripts/validate-external-capability-example.mjs",
    "scripts/validate-admin-ui-contracts.mjs",
    "scripts/validate-platform-codegen-source-writing-readiness.mjs",
  ], "requiredValidators", errors);
  requireIncludes(protocol.requiredTests, [
    "scripts/platform-human-ai-development-protocol.test.mjs",
    "scripts/platform-capability-operation-policy.test.mjs",
  ], "requiredTests", errors);

  for (const principle of values(protocol.principles)) {
    validateLocalizedLabel(principle, `principle ${principle.id ?? "<missing>"}`, errors);
    if (values(principle.requires).length === 0) {
      errors.push(`principle ${principle.id ?? "<missing>"} must declare requires`);
    }
    if (values(principle.forbids).length === 0) {
      errors.push(`principle ${principle.id ?? "<missing>"} must declare forbids`);
    }
  }
  for (const mode of values(protocol.customizationModes)) {
    if (!mode.id || !mode.status || !mode.description) {
      errors.push(`customization mode ${mode.id ?? "<missing>"} must declare id, status and description`);
    }
  }
  for (const domain of values(protocol.domains)) {
    validateDomain(domain, errors);
  }

  validateRelativePaths(protocol, errors);
  validateSourceSnippets(protocol, errors);

  return errors;
}

const protocol = readJSON(protocolPath);
const errors = validateProtocol(protocol);
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

const relativeProtocolPath = path.relative(repoRoot, protocolPath).split(path.sep).join("/");
console.log(`Validated human + AI development protocol in ${relativeProtocolPath} (${values(protocol.domains).length} domains)`);
