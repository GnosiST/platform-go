import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback = "") {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function hasFlag(name) {
  return process.argv.includes(name);
}

const root = path.resolve(argValue("--root", repoRoot));
const strict = hasFlag("--strict");
const strictPaths = strict || hasFlag("--strict-paths");
const expectedModule = argValue("--expect-module", "");
const referenceManifestPath = argValue(
  "--reference-manifest",
  "resources/reference-snapshot/manifest.json",
);

const ignoredDirectories = new Set([
  ".git",
  ".codegraph",
  ".codex",
  ".superpowers",
  ".worktrees",
  "node_modules",
  "vendor",
  "tmp",
]);

const requiredReleaseFiles = [
  "LICENSE",
  "NOTICE",
  "CONTRIBUTING.md",
  "SECURITY.md",
  "CODE_OF_CONDUCT.md",
  "SUPPORT.md",
  "GOVERNANCE.md",
  "CHANGELOG.md",
];

const secretPatterns = [
  { name: "PEM private key", pattern: /-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----/ },
  { name: "AWS access key", pattern: /\bAKIA[0-9A-Z]{16}\b/ },
  { name: "GitHub token", pattern: /\bgh[pousr]_[A-Za-z0-9_]{20,}\b/ },
  { name: "Slack token", pattern: /\bxox[baprs]-[0-9A-Za-z-]{20,}\b/ },
  { name: "OpenAI-style secret key", pattern: /\bsk-[A-Za-z0-9_-]{24,}\b/ },
];

const privatePathPatterns = [
  /(?:^|[^A-Za-z0-9_])\/Users\/[^/\s]+(?:\/|$)/,
  /(?:^|[^A-Za-z0-9_])\/home\/[^/\s]+(?:\/|$)/,
  /(?:^|[^A-Za-z0-9_])\/var\/folders\/[^\s]+/,
  /(?:^|[^A-Za-z0-9_])[A-Z]:\\Users\\[^\\\s]+/i,
];

// The validator and its fixture tests contain detection patterns and synthetic
// paths by design; scanning those files would report the tool itself.
const ignoredPathScanFiles = new Set([
  "scripts/validate-open-source-portability.mjs",
  "scripts/validate-open-source-portability.test.mjs",
]);

function relative(file) {
  return path.relative(root, file) || ".";
}

function walk(directory, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (ignoredDirectories.has(entry.name)) continue;
    const file = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(file, output);
    else if (entry.isFile()) output.push(file);
  }
  return output;
}

function readText(file) {
  const buffer = fs.readFileSync(file);
  if (buffer.includes(0)) return null;
  return buffer.toString("utf8");
}

function validateReferenceSnapshot(errors) {
  const manifestFile = path.resolve(root, referenceManifestPath);
  if (!fs.existsSync(manifestFile)) {
    errors.push(`reference snapshot manifest is missing: ${referenceManifestPath}`);
    return;
  }
  let manifest;
  try {
    manifest = JSON.parse(fs.readFileSync(manifestFile, "utf8"));
  } catch {
    errors.push(`reference snapshot manifest is invalid JSON: ${referenceManifestPath}`);
    return;
  }
  if (manifest.root !== "resources/reference-snapshot/zshenmez") {
    errors.push("reference snapshot manifest root must stay inside resources/reference-snapshot");
  }
  if (!Array.isArray(manifest.files) || manifest.files.length === 0) {
    errors.push("reference snapshot manifest must list tracked files");
    return;
  }
  for (const relativePath of manifest.files) {
    if (typeof relativePath !== "string" || path.isAbsolute(relativePath) || relativePath.includes("..")) {
      errors.push(`reference snapshot path must be relative and contained: ${relativePath}`);
      continue;
    }
    const target = path.resolve(root, manifest.root, relativePath);
    const relative = path.relative(root, target);
    if (relative.startsWith("..") || !fs.existsSync(target) || !fs.statSync(target).isFile()) {
      errors.push(`reference snapshot file is missing: ${path.join(manifest.root, relativePath)}`);
    }
  }
}

function validate() {
  const errors = [];
  const warnings = [];
  if (!fs.existsSync(root) || !fs.statSync(root).isDirectory()) {
    return { errors: [`release root is not a directory: ${root}`], warnings };
  }

  if (strict) {
    for (const file of requiredReleaseFiles) {
      if (!fs.existsSync(path.join(root, file))) errors.push(`required release file is missing: ${file}`);
    }
  }

  if (expectedModule) {
    const goModPath = path.join(root, "go.mod");
    if (!fs.existsSync(goModPath)) {
      errors.push("go.mod is missing while --expect-module was provided");
    } else {
      const match = fs.readFileSync(goModPath, "utf8").match(/^module\s+(.+)$/m);
      if (!match || match[1].trim() !== expectedModule) {
        errors.push(`go.mod module must be ${expectedModule}`);
      }
    }
  }

  if (strict) validateReferenceSnapshot(errors);

  for (const file of walk(root)) {
    const text = readText(file);
    if (text === null) continue;
    const fileName = relative(file);
    for (const { name, pattern } of secretPatterns) {
      if (pattern.test(text)) errors.push(`${name} detected in ${fileName}`);
    }
    if (ignoredPathScanFiles.has(fileName)) continue;
    const pathHits = privatePathPatterns.filter((pattern) => pattern.test(text));
    if (pathHits.length > 0) {
      const message = `machine-specific absolute path detected in ${fileName}`;
      if (strictPaths) errors.push(message);
      else warnings.push(message);
    }
  }
  return { errors, warnings };
}

const result = validate();
for (const warning of result.warnings) console.warn(`warning: ${warning}`);
if (result.errors.length > 0) {
  for (const error of result.errors) console.error(error);
  process.exit(1);
}

console.log(
  `Validated open-source portability for ${path.relative(repoRoot, root) || "."}${strict ? " (strict)" : ""}`,
);
