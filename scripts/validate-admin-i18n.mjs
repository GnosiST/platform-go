import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const adminSrcDir = path.join(repoRoot, "admin", "src");
const adminPlatformDir = path.join(repoRoot, "admin", "src", "platform");

const allowedLocalizedDataFiles = new Set([
  path.join(adminPlatformDir, "i18n.ts"),
  path.join(adminPlatformDir, "resources", "registry.ts"),
  path.join(adminPlatformDir, "capabilities", "metadata.ts"),
  path.join(adminPlatformDir, "dashboard", "dashboardData.ts"),
]);

const cjkPattern = /[\u3400-\u9fff]/;
const checkedExtensions = new Set([".ts", ".tsx"]);

const failures = [];
const dictionarySource = fs.readFileSync(path.join(adminPlatformDir, "i18n.ts"), "utf8");

const zhKeys = extractDictionaryKeys(dictionarySource, "zh");
const enKeys = extractDictionaryKeys(dictionarySource, "en");
for (const key of zhKeys) {
  if (!enKeys.has(key)) {
    failures.push(`admin/src/platform/i18n.ts: missing en key ${key}`);
  }
}
for (const key of enKeys) {
  if (!zhKeys.has(key)) {
    failures.push(`admin/src/platform/i18n.ts: missing zh key ${key}`);
  }
}

for (const filePath of walk(adminSrcDir)) {
  if (!checkedExtensions.has(path.extname(filePath))) {
    continue;
  }
  if (allowedLocalizedDataFiles.has(filePath)) {
    continue;
  }
  const lines = fs.readFileSync(filePath, "utf8").split(/\r?\n/);
  lines.forEach((line, index) => {
    if (cjkPattern.test(line)) {
      failures.push(`${path.relative(repoRoot, filePath)}:${index + 1}: ${line.trim()}`);
    }
  });
}

if (failures.length > 0) {
  console.error("Admin i18n validation failed. Move visible text into admin/src/platform/i18n.ts or localized data files.");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log("Admin i18n validation passed.");

function extractDictionaryKeys(source, language) {
  const start = source.indexOf(`  ${language}: {`);
  if (start === -1) {
    failures.push(`admin/src/platform/i18n.ts: missing ${language} dictionary`);
    return new Set();
  }
  const bodyStart = source.indexOf("{", start);
  let depth = 0;
  let end = bodyStart;
  for (; end < source.length; end += 1) {
    const char = source[end];
    if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        break;
      }
    }
  }
  const body = source.slice(bodyStart + 1, end);
  const keys = new Set();
  for (const line of body.split(/\r?\n/)) {
    const match = line.match(/^\s{4}([a-zA-Z0-9_]+):/);
    if (match) {
      keys.add(match[1]);
    }
  }
  return keys;
}

function* walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      yield* walk(fullPath);
      continue;
    }
    yield fullPath;
  }
}
