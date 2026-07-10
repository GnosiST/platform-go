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

const packagePathArg = argValue("--package", "");
const templatesPath = path.resolve(repoRoot, argValue("--templates", "resources/generated/platform-promotion-evidence-templates.json"));
const productionAuthPath = path.resolve(repoRoot, argValue("--production-auth", "resources/platform-production-auth-hardening.json"));
const codegenReadinessPath = path.resolve(repoRoot, argValue("--codegen-readiness", "resources/platform-codegen-source-writing-readiness.json"));

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

function isPresent(value) {
  if (Array.isArray(value)) return value.length > 0;
  return value !== undefined && value !== null && String(value).trim() !== "";
}

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const value of required) {
    if (!actual.has(value)) {
      errors.push(`${label} must include ${value}`);
    }
  }
}

function unique(items) {
  return Array.from(new Set(values(items)));
}

function duplicateValues(items) {
  const duplicates = new Set();
  const seen = new Set();
  for (const item of values(items)) {
    if (seen.has(item)) {
      duplicates.add(item);
    }
    seen.add(item);
  }
  return Array.from(duplicates);
}

function forbiddenFieldNames(value, forbiddenFields, found = new Set()) {
  if (!value || typeof value !== "object") {
    return found;
  }
  const forbidden = new Set(values(forbiddenFields).map((field) => field.toLowerCase()));
  if (Array.isArray(value)) {
    for (const item of value) {
      forbiddenFieldNames(item, forbiddenFields, found);
    }
    return found;
  }
  for (const [key, nested] of Object.entries(value)) {
    if (forbidden.has(key.toLowerCase())) {
      found.add(key);
    }
    forbiddenFieldNames(nested, forbiddenFields, found);
  }
  return found;
}

function approvalPackageByTemplatePackage(packageId, productionAuth, codegenReadiness) {
  if (packageId === "production-auth-promotion") {
    return productionAuth.productionPromotionApprovalPackage ?? {};
  }
  if (packageId === "source-writing-codegen-promotion") {
    return codegenReadiness.sourceWritingApprovalPackage ?? {};
  }
  return null;
}

function normalizePackages(input) {
  if (Array.isArray(input?.packages)) {
    return input.packages;
  }
  return [input];
}

function validateEvidenceItem({ item, required, schema, prefix, errors }) {
  if (item.id !== required.id) {
    errors.push(`${prefix}.id must match ${required.id}`);
  }
  if (item.owner !== required.owner) {
    errors.push(`${prefix}.owner must match ${required.owner}`);
  }
  if (item.evidenceKind !== required.evidenceKind) {
    errors.push(`${prefix}.evidenceKind must match ${required.evidenceKind}`);
  }
  if (item.status !== "complete") {
    errors.push(`${prefix}.status must be complete`);
  }
  for (const field of values(schema.requiredFields)) {
    if (!isPresent(item[field])) {
      errors.push(`${prefix}.${field} must not be empty`);
    }
  }
  if (item.approvedBy === item.owner) {
    errors.push(`${prefix}.approvedBy must not equal owner`);
  }
  if (item.artifactHash && !validArtifactHash(item.artifactHash, schema.artifactHashPolicy)) {
    errors.push(`${prefix}.artifactHash must be sha256: followed by 64 lowercase hex characters`);
  }
  if (item.artifactURI && !validArtifactURI(item.artifactURI, schema.artifactURIPolicy)) {
    errors.push(`${prefix}.artifactURI must be an external absolute review artifact URI using https, s3, or gs`);
  }
  if (item.approvedAt && Number.isNaN(Date.parse(item.approvedAt))) {
    errors.push(`${prefix}.approvedAt must be an ISO timestamp`);
  }
  if (item.reviewedCommit && !/^[0-9a-f]{40}$/i.test(String(item.reviewedCommit))) {
    errors.push(`${prefix}.reviewedCommit must be a 40 character commit hash`);
  }
  if ("environment" in item && item.environment !== "production") {
    errors.push(`${prefix}.environment must be production`);
  }
  for (const field of ["verificationCommands", "rollbackCommands"]) {
    if (!Array.isArray(item[field]) || item[field].length === 0) {
      errors.push(`${prefix}.${field} must not be empty`);
      continue;
    }
    if (!item[field].every((command) => String(command).startsWith("rtk "))) {
      errors.push(`${prefix}.${field} must use rtk commands`);
    }
  }
  for (const forbiddenField of forbiddenFieldNames(item, schema.forbiddenFields)) {
    errors.push(`${prefix} must not include forbidden field ${forbiddenField}`);
  }
}

function validArtifactHash(value, policy = {}) {
  const expectedPrefix = policy.prefix ?? "sha256:";
  const expectedLength = policy.hexLength ?? 64;
  if (policy.algorithm && policy.algorithm !== "sha256") {
    return false;
  }
  if (policy.format && policy.format !== "prefix-hex") {
    return false;
  }
  if (policy.casing && policy.casing !== "lowercase") {
    return false;
  }
  const pattern = new RegExp(`^${expectedPrefix.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}[0-9a-f]{${expectedLength}}$`);
  return pattern.test(String(value));
}

function validArtifactURI(value, policy = {}) {
  const text = String(value ?? "").trim();
  if (!text) return false;
  if (policy.forbidLocalAbsolutePaths !== false && path.isAbsolute(text)) return false;
  if (policy.forbidRelativePaths !== false && (text.startsWith("./") || text.startsWith("../"))) return false;

  let uri;
  try {
    uri = new URL(text);
  } catch {
    return false;
  }

  const scheme = uri.protocol.replace(":", "").toLowerCase();
  const allowedSchemes = values(policy.allowedSchemes).length > 0 ? values(policy.allowedSchemes).map((item) => String(item).toLowerCase()) : ["https", "s3", "gs"];
  const forbiddenSchemes = values(policy.forbiddenSchemes).map((item) => String(item).toLowerCase());
  if (forbiddenSchemes.includes(scheme) || !allowedSchemes.includes(scheme)) {
    return false;
  }

  const host = uri.hostname.toLowerCase().replace(/^\[|\]$/g, "");
  if (!host) return false;
  const forbiddenHosts = values(policy.forbiddenHosts).map((item) => String(item).toLowerCase());
  if (forbiddenHosts.includes(host)) {
    return false;
  }
  if (policy.forbidLocalhost !== false && (host === "localhost" || host.endsWith(".localhost"))) {
    return false;
  }
  if (policy.forbidPrivateNetworkHosts !== false && isPrivateNetworkHost(host)) {
    return false;
  }

  return true;
}

function isPrivateNetworkHost(host) {
  const ipv4 = parseIPv4(host);
  if (ipv4) {
    const [a, b] = ipv4;
    return (
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168)
    );
  }
  const normalized = host.toLowerCase();
  return normalized === "::1" || normalized.startsWith("fc") || normalized.startsWith("fd") || normalized.endsWith(".local") || normalized.endsWith(".internal");
}

function parseIPv4(host) {
  if (!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(host)) return null;
  const parts = host.split(".").map((part) => Number(part));
  if (parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return null;
  return parts;
}

function validateSourceWritingCoverage({ packageId, evidence, codegenReadiness, errors }) {
  if (packageId !== "source-writing-codegen-promotion") {
    return;
  }
  const families = values(codegenReadiness.targetFamilies);
  const requiredFamilyIds = families.map((family) => family.id).filter(Boolean);
  const requiredRuntimeTargets = unique(families.flatMap((family) => values(family.runtimeTargets)));
  const requiredTestCommands = unique(families.flatMap((family) => values(family.testCommands)));
  const knownFamilyIds = new Set(requiredFamilyIds);
  const knownRuntimeTargets = new Set(requiredRuntimeTargets);
  const evidenceByID = new Map(evidence.map((item) => [item.id, item]));

  const submittedFamilyIds = unique(evidence.flatMap((item) => values(item.targetFamilies)));
  const submittedRuntimeTargets = unique(evidence.flatMap((item) => values(item.runtimeTargets)));
  requireIncludes(submittedFamilyIds, requiredFamilyIds, `${packageId} targetFamilies`, errors);
  requireIncludes(submittedRuntimeTargets, requiredRuntimeTargets, `${packageId} runtimeTargets`, errors);

  for (const item of evidence) {
    for (const familyId of values(item.targetFamilies)) {
      if (!knownFamilyIds.has(familyId)) {
        errors.push(`${packageId}.${item.id}.targetFamilies references unknown target family ${familyId}`);
      }
    }
    for (const runtimeTarget of values(item.runtimeTargets)) {
      if (!knownRuntimeTargets.has(runtimeTarget)) {
        errors.push(`${packageId}.${item.id}.runtimeTargets references unknown runtime target ${runtimeTarget}`);
      }
    }
  }

  const diffReview = evidenceByID.get("diff-review");
  if (diffReview) {
    requireIncludes(diffReview.runtimeTargets, requiredRuntimeTargets, `${packageId}.diff-review.runtimeTargets`, errors);
  }
  const testRun = evidenceByID.get("target-family-test-run");
  if (testRun) {
    requireIncludes(testRun.targetFamilies, requiredFamilyIds, `${packageId}.target-family-test-run.targetFamilies`, errors);
    requireIncludes(testRun.verificationCommands, requiredTestCommands, "target-family-test-run.verificationCommands", errors);
  }
  const runtimeTargetOwnerApproval = evidenceByID.get("runtime-target-owner-approval");
  if (runtimeTargetOwnerApproval) {
    requireIncludes(
      runtimeTargetOwnerApproval.runtimeTargets,
      requiredRuntimeTargets,
      `${packageId}.runtime-target-owner-approval.runtimeTargets`,
      errors,
    );
  }
}

function validateProductionAuthCoverage({ packageId, evidence, productionAuth, errors }) {
  if (packageId !== "production-auth-promotion") {
    return;
  }
  const providers = values(productionAuth.providerPromotionMatrix?.providers);
  const requiredProviderIds = providers.map((provider) => provider.id).filter(Boolean);
  const rotationProviderIds = providers.filter((provider) => provider.rotationRunbookRequired === true).map((provider) => provider.id).filter(Boolean);
  const requiredProviderControls = unique(providers.flatMap((provider) => values(provider.requiredControls)));
  const requiredRuntimeTests = values(productionAuth.providerRuntimePolicy?.requiredTests);
  const knownProviderIds = new Set(requiredProviderIds);
  const knownProviderControls = new Set(requiredProviderControls);
  const knownRuntimeTests = new Set(requiredRuntimeTests);
  const evidenceByID = new Map(evidence.map((item) => [item.id, item]));

  const submittedProviderIds = unique(evidence.flatMap((item) => values(item.providerIds)));
  const submittedProviderControls = unique(evidence.flatMap((item) => values(item.providerControls)));
  const submittedRuntimeTestRefs = unique(evidence.flatMap((item) => values(item.runtimeTestRefs)));
  requireIncludes(submittedProviderIds, requiredProviderIds, `${packageId} providerIds`, errors);
  requireIncludes(submittedProviderControls, requiredProviderControls, `${packageId} providerControls`, errors);
  requireIncludes(submittedRuntimeTestRefs, requiredRuntimeTests, `${packageId} runtimeTestRefs`, errors);

  for (const item of evidence) {
    for (const providerId of values(item.providerIds)) {
      if (!knownProviderIds.has(providerId)) {
        errors.push(`${packageId}.${item.id}.providerIds references unknown provider ${providerId}`);
      }
    }
    for (const providerControl of values(item.providerControls)) {
      if (!knownProviderControls.has(providerControl)) {
        errors.push(`${packageId}.${item.id}.providerControls references unknown provider control ${providerControl}`);
      }
    }
    for (const runtimeTestRef of values(item.runtimeTestRefs)) {
      if (!knownRuntimeTests.has(runtimeTestRef)) {
        errors.push(`${packageId}.${item.id}.runtimeTestRefs references unknown runtime test ${runtimeTestRef}`);
      }
    }
  }

  const runtimeTestOutput = evidenceByID.get("runtime-test-output");
  if (runtimeTestOutput) {
    requireIncludes(runtimeTestOutput.runtimeTestRefs, requiredRuntimeTests, "runtime-test-output.runtimeTestRefs", errors);
  }
  const providerRunbook = evidenceByID.get("provider-secret-rotation-runbook");
  if (providerRunbook) {
    requireIncludes(providerRunbook.providerIds, rotationProviderIds, "provider-secret-rotation-runbook.providerIds", errors);
  }
}

function validatePackage({ pkg, templatePackage, approvalPackage, productionAuth, codegenReadiness, errors }) {
  const packageId = pkg?.packageId ?? pkg?.id ?? "";
  const prefix = `promotion evidence package ${packageId || "<missing>"}`;
  if (!packageId) {
    errors.push("promotion evidence package is missing packageId");
    return;
  }
  if (!templatePackage) {
    errors.push(`${prefix} is not declared in platform promotion evidence templates`);
    return;
  }
  if (!approvalPackage) {
    errors.push(`${prefix} has no matching source approval package`);
    return;
  }
  if (pkg.taskId !== templatePackage.taskId) {
    errors.push(`${prefix}.taskId must be ${templatePackage.taskId}`);
  }
  if (pkg.source !== templatePackage.source) {
    errors.push(`${prefix}.source must be ${templatePackage.source}`);
  }
  if (pkg.approvalState !== "submitted") {
    errors.push(`${prefix}.approvalState must be submitted`);
  }
  if (pkg.defaultRuntimeMutation !== approvalPackage.defaultRuntimeMutation) {
    errors.push(`${prefix}.defaultRuntimeMutation must match source approval package`);
  }
  if (!pkg.reviewedCommit || !/^[0-9a-f]{40}$/i.test(String(pkg.reviewedCommit))) {
    errors.push(`${prefix}.reviewedCommit must be a 40 character commit hash`);
  }

  const requiredEvidence = values(approvalPackage.requiredEvidence);
  const evidence = values(pkg.evidence);
  for (const duplicateID of duplicateValues(evidence.map((item) => item.id))) {
    errors.push(`${packageId}.evidence contains duplicate id ${duplicateID}`);
  }
  const evidenceByID = new Map(evidence.map((item) => [item.id, item]));
  requireIncludes(
    evidence.map((item) => item.id),
    requiredEvidence.map((item) => item.id),
    `${packageId}.evidence`,
    errors,
  );
  requireIncludes(
    requiredEvidence.map((item) => item.id),
    evidence.map((item) => item.id),
    `${packageId}.requiredEvidence`,
    errors,
  );

  const owners = new Set(evidence.map((item) => item.owner));
  for (const owner of values(approvalPackage.requiredApprovals)) {
    if (!owners.has(owner)) {
      errors.push(`${packageId}.evidence must include owner ${owner}`);
    }
  }

  const schema = approvalPackage.completedEvidenceSchema ?? {};
  for (const required of requiredEvidence) {
    const item = evidenceByID.get(required.id);
    if (!item) continue;
    if (pkg.reviewedCommit && item.reviewedCommit && item.reviewedCommit !== pkg.reviewedCommit) {
      errors.push(`${packageId}.${required.id}.reviewedCommit must match package reviewedCommit`);
    }
    validateEvidenceItem({
      item,
      required,
      schema,
      prefix: `${packageId}.${required.id}`,
      errors,
    });
  }
  validateSourceWritingCoverage({ packageId, evidence, codegenReadiness, errors });
  validateProductionAuthCoverage({ packageId, evidence, productionAuth, errors });
}

function validate() {
  const errors = [];
  if (!packagePathArg) {
    errors.push("--package is required");
    return { packageCount: 0, errors };
  }
  const packagePath = path.resolve(repoRoot, packagePathArg);
  if (!fs.existsSync(packagePath)) {
    errors.push(`promotion evidence package file is missing: ${packagePathArg}`);
    return { packageCount: 0, errors };
  }

  const submitted = readJSON(packagePath);
  const templates = readJSON(templatesPath);
  const productionAuth = readJSON(productionAuthPath);
  const codegenReadiness = readJSON(codegenReadinessPath);
  const templatePackages = new Map(values(templates.packages).map((pkg) => [pkg.id, pkg]));

  if (templates.sources?.productionAuthHardening !== relativeToRepo(productionAuthPath)) {
    errors.push("promotion evidence templates source productionAuthHardening is stale");
  }
  if (templates.sources?.codegenSourceWritingReadiness !== relativeToRepo(codegenReadinessPath)) {
    errors.push("promotion evidence templates source codegenSourceWritingReadiness is stale");
  }

  const packages = normalizePackages(submitted);
  if (packages.length === 0) {
    errors.push("promotion evidence package bundle must include at least one package");
  }
  for (const duplicateID of duplicateValues(packages.map((pkg) => pkg?.packageId ?? pkg?.id ?? ""))) {
    errors.push(`promotion evidence packages contains duplicate packageId ${duplicateID}`);
  }
  for (const pkg of packages) {
    const packageId = pkg?.packageId ?? pkg?.id ?? "";
    validatePackage({
      pkg,
      templatePackage: templatePackages.get(packageId),
      approvalPackage: approvalPackageByTemplatePackage(packageId, productionAuth, codegenReadiness),
      productionAuth,
      codegenReadiness,
      errors,
    });
  }
  return { packageCount: packages.length, errors };
}

const { packageCount, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${packageCount} platform promotion evidence packages`);
