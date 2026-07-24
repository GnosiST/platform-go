import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) return false;
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-database-certification-matrix.json"));
const taskGraphPath = path.resolve(repoRoot, argValue("--task-graph", "resources/platform-foundation-task-graph.json"));
const matrix = readJSON(matrixPath);
const taskGraph = readJSON(taskGraphPath);
const errors = [];

if (matrix.version !== 1) errors.push("database certification matrix version must be 1");
if (matrix.certificationScope !== "full-platform-repository-migration-query-transaction-pagination-locking") {
  errors.push("database certification matrix must retain the full-platform certification scope");
}

const statuses = new Set(["certified", "experimental", "unsupported"]);
const entries = values(matrix.entries);
const seen = new Set();
for (const entry of entries) {
  const prefix = `database certification entry ${entry.driver ?? "<missing>"}/${entry.version ?? "<missing>"}/${entry.feature ?? "<missing>"}`;
  const key = [entry.driver, entry.version, entry.feature].join("/");
  if (!entry.driver || !entry.version || !entry.feature) errors.push(`${prefix} must declare driver, version and feature`);
  if (seen.has(key)) errors.push(`${prefix} is duplicated`);
  seen.add(key);
  if (!statuses.has(entry.status)) errors.push(`${prefix} has unsupported status ${entry.status ?? "<missing>"}`);
  if (!relativeExistingPath(entry.evidenceRef)) errors.push(`${prefix} evidenceRef is missing or unsafe: ${entry.evidenceRef ?? "<missing>"}`);
  if (!entry.note) errors.push(`${prefix} must include a bounded support note`);
  if (entry.status === "unsupported" && !entry.note) errors.push(`${prefix} must state why it is unsupported`);
}

function requireEntry(driver, version, feature, status) {
  if (!entries.some((entry) => entry.driver === driver && entry.version === version && entry.feature === feature && entry.status === status)) {
    errors.push(`database certification matrix must include ${driver}/${version}/${feature} as ${status}`);
  }
}

requireEntry("mysql", "8.4", "gorm-storage-smoke", "experimental");
requireEntry("postgres", "16", "gorm-storage-smoke", "experimental");
requireEntry("mysql", "8.4", "platform-gorm-contracts", "certified");
requireEntry("postgres", "16", "platform-gorm-contracts", "certified");
requireEntry("sqlite", "current", "development-and-test-storage", "experimental");
requireEntry("kingbasees", "unverified", "platform-runtime", "unsupported");
requireEntry("oracle", "unverified", "platform-runtime", "unsupported");

const task = values(taskGraph.tasks).find((item) => item.id === "database-certification-matrix");
if (!task) {
  errors.push("task graph must include database-certification-matrix");
} else if (task.status === "implemented" && entries.every((entry) => entry.status !== "certified")) {
  errors.push("database-certification-matrix cannot be implemented without a certified full-platform entry");
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated database certification matrix (${entries.length} entries)`);
