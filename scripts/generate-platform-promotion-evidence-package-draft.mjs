import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "platform-promotion-evidence-package-draft.json");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const templatesPath = path.resolve(repoRoot, argValue("--templates", "resources/generated/platform-promotion-evidence-templates.json"));
const packageID = argValue("--package-id", "");
const stdout = process.argv.includes("--stdout");

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function relativeToRepo(filePath) {
  return toPosix(path.relative(repoRoot, filePath));
}

function draftEvidence(template) {
  return {
    ...template,
    status: "draft",
  };
}

function draftPackage(templatePackage) {
  return {
    packageId: templatePackage.id,
    taskId: templatePackage.taskId,
    source: templatePackage.source,
    approvalState: "draft-submission",
    defaultRuntimeMutation: templatePackage.defaultRuntimeMutation,
    requiredApprovals: values(templatePackage.requiredApprovals),
    requiredEvidenceCount: templatePackage.requiredEvidenceCount,
    completionBlockers: values(templatePackage.completionBlockers),
    requiredBeforeCompletion: values(templatePackage.requiredBeforeCompletion),
    completedEvidenceSchema: templatePackage.completedEvidenceSchema ?? {},
    reviewContext: templatePackage.reviewContext ?? {},
    evidence: values(templatePackage.evidenceTemplates).map((template) => draftEvidence(template)),
  };
}

const templates = readJSON(templatesPath);
const allPackages = values(templates.packages);
const selectedPackages = packageID ? allPackages.filter((pkg) => pkg.id === packageID) : allPackages;

if (packageID && selectedPackages.length === 0) {
  console.error(`Unknown promotion evidence package ${packageID}`);
  process.exit(1);
}

const output = {
  generatedBy: "scripts/generate-platform-promotion-evidence-package-draft.mjs",
  source: relativeToRepo(templatesPath),
  purpose: "Non-submitted draft package for external promotion evidence collection. Fill every evidence item, change approvalState to submitted, mark evidence status complete, then validate with scripts/validate-platform-promotion-evidence-package.mjs.",
  mode: {
    approvalState: "draft-submission",
    runtimeMutation: "disabled",
    sourceWriting: "disabled",
  },
  validator: templates.submittedEvidenceValidator ?? {
    script: "scripts/validate-platform-promotion-evidence-package.mjs",
    command: "rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>",
  },
  packages: selectedPackages.map((pkg) => draftPackage(pkg)),
};

const serialized = `${JSON.stringify(output, null, 2)}\n`;
if (stdout) {
  process.stdout.write(serialized);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  const temporaryPath = `${generatedPath}.tmp-${process.pid}`;
  fs.writeFileSync(temporaryPath, serialized);
  fs.renameSync(temporaryPath, generatedPath);
  console.log(`Generated ${relativeToRepo(generatedPath)}`);
}
