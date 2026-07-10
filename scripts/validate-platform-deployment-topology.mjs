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

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/platform-deployment-topology.json"));
const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-production-readiness.json"));
const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-engineering-capabilities.json"));

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

function requireIncludes(items, required, label, errors) {
  const actual = new Set(values(items));
  for (const item of required) {
    if (!actual.has(item)) {
      errors.push(`${label} must include ${item}`);
    }
  }
}

function validateDecision(contract, errors) {
  const decision = contract.decision ?? {};
  if (decision.vercelRequired !== false) {
    errors.push("decision.vercelRequired must stay false");
  }
  if (decision.vercelAdminUsage !== "optional-static-admin") {
    errors.push("decision.vercelAdminUsage must stay optional-static-admin");
  }
  if (decision.selectedTopology !== "single-service-production") {
    errors.push("decision.selectedTopology must stay single-service-production");
  }
  if (decision.defaultApiRuntime !== "long-lived-service") {
    errors.push("decision.defaultApiRuntime must stay long-lived-service");
  }
  if (decision.fullStackVercelGo !== "requires-separate-adapter-spec") {
    errors.push("decision.fullStackVercelGo must stay requires-separate-adapter-spec");
  }
}

function validateTopologies(contract, errors) {
  const topologies = new Map(values(contract.topologies).map((topology) => [topology.id, topology]));
  for (const id of ["local-development", "single-service-production", "split-admin-vercel-api-service", "fullstack-vercel-go-runtime"]) {
    if (!topologies.has(id)) {
      errors.push(`topologies must include ${id}`);
    }
  }
  if (topologies.get("single-service-production")?.status !== "recommended") {
    errors.push("single-service-production status must stay recommended");
  }
  if (topologies.get("single-service-production")?.api?.runtime !== "long-lived-go-service") {
    errors.push("single-service-production api.runtime must stay long-lived-go-service");
  }
  if (topologies.get("split-admin-vercel-api-service")?.status !== "optional") {
    errors.push("split-admin-vercel-api-service status must stay optional");
  }
  if (topologies.get("split-admin-vercel-api-service")?.admin?.host !== "vercel-static") {
    errors.push("split-admin-vercel-api-service admin.host must stay vercel-static");
  }
  if (topologies.get("split-admin-vercel-api-service")?.admin?.apiBaseEnv !== "VITE_PLATFORM_API_BASE") {
    errors.push("split-admin-vercel-api-service admin.apiBaseEnv must stay VITE_PLATFORM_API_BASE");
  }
  if (topologies.get("fullstack-vercel-go-runtime")?.status !== "not-default") {
    errors.push("fullstack-vercel-go-runtime status must stay not-default");
  }
  if (topologies.get("fullstack-vercel-go-runtime")?.api?.requiresSeparateAdapterSpec !== true) {
    errors.push("fullstack-vercel-go-runtime api.requiresSeparateAdapterSpec must stay true");
  }
  if (topologies.get("fullstack-vercel-go-runtime")?.api?.defaultDeployment !== false) {
    errors.push("fullstack-vercel-go-runtime api.defaultDeployment must stay false");
  }
}

function validateVercelPolicy(contract, errors) {
  const admin = contract.vercelPolicy?.admin ?? {};
  if (admin.allowed !== true) {
    errors.push("vercelPolicy.admin.allowed must stay true");
  }
  if (admin.required !== false) {
    errors.push("vercelPolicy.admin.required must stay false");
  }
  if (admin.rootDirectory !== "admin") {
    errors.push("vercelPolicy.admin.rootDirectory must stay admin");
  }
  if (admin.buildCommand !== "npm run build") {
    errors.push("vercelPolicy.admin.buildCommand must stay npm run build");
  }
  if (admin.outputDirectory !== "dist") {
    errors.push("vercelPolicy.admin.outputDirectory must stay dist");
  }
  if (admin.apiBaseEnv !== "VITE_PLATFORM_API_BASE") {
    errors.push("vercelPolicy.admin.apiBaseEnv must stay VITE_PLATFORM_API_BASE");
  }
  if (admin.adapterTemplate !== "deploy/vercel/admin.vercel.json") {
    errors.push("vercelPolicy.admin.adapterTemplate must be deploy/vercel/admin.vercel.json");
  }
  if (admin.adapterScope !== "admin-static-only") {
    errors.push("vercelPolicy.admin.adapterScope must stay admin-static-only");
  }
  if (admin.adapterRuntime !== "static-assets") {
    errors.push("vercelPolicy.admin.adapterRuntime must stay static-assets");
  }
  const adapterPackage = admin.adapterPackage ?? {};
  if (adapterPackage.status !== "implemented") {
    errors.push("vercelPolicy.admin.adapterPackage.status must stay implemented");
  }
  if (adapterPackage.template !== admin.adapterTemplate) {
    errors.push("vercelPolicy.admin.adapterPackage.template must match vercelPolicy.admin.adapterTemplate");
  }
  if (adapterPackage.copyTarget !== "admin/vercel.json") {
    errors.push("vercelPolicy.admin.adapterPackage.copyTarget must stay admin/vercel.json");
  }
  if (adapterPackage.installation !== "copy-into-admin-project-only-when-vercel-is-selected") {
    errors.push("vercelPolicy.admin.adapterPackage.installation must stay copy-into-admin-project-only-when-vercel-is-selected");
  }
  if (adapterPackage.defaultIncludedInProduction !== false) {
    errors.push("vercelPolicy.admin.adapterPackage.defaultIncludedInProduction must stay false");
  }
  requireIncludes(
    adapterPackage.apiBindingModes,
    ["absolute-api-base-env", "reviewed-edge-proxy"],
    "vercelPolicy.admin.adapterPackage.apiBindingModes",
    errors,
  );
  requireIncludes(
    adapterPackage.forbiddenRuntimeWiring,
    ["functions", "builds", "routes", "go-build", "vercel-go-runtime", "api-rewrite"],
    "vercelPolicy.admin.adapterPackage.forbiddenRuntimeWiring",
    errors,
  );

  const api = contract.vercelPolicy?.api ?? {};
  if (api.defaultDeployment !== false) {
    errors.push("vercelPolicy.api.defaultDeployment must stay false");
  }
  if (api.requiresSeparateAdapterSpec !== true) {
    errors.push("vercelPolicy.api.requiresSeparateAdapterSpec must stay true");
  }
  if (api.mustRemainLongLivedByDefault !== true) {
    errors.push("vercelPolicy.api.mustRemainLongLivedByDefault must stay true");
  }
  requireIncludes(
    api.requiredEvidenceBeforePromotion,
    [
      "go-runtime-adapter-spec",
      "production-gorm-stores",
      "redis-cache-invalidation",
      "external-file-storage",
      "demo-auth-disabled",
      "production-auth-approval-package",
      "rollback-plan",
    ],
    "vercelPolicy.api.requiredEvidenceBeforePromotion",
    errors,
  );
}

function validateVercelAdminAdapter(contract, errors) {
  const templatePath = contract.vercelPolicy?.admin?.adapterTemplate;
  if (!relativeExistingPath(templatePath)) {
    errors.push(`vercel admin adapter template path is missing or unsafe: ${templatePath ?? "<missing>"}`);
    return;
  }

  let template;
  try {
    template = JSON.parse(readRelativeFile(templatePath));
  } catch (error) {
    errors.push(`vercel admin adapter template must be valid JSON: ${error.message}`);
    return;
  }

  if (template.framework !== "vite") {
    errors.push("vercel admin adapter framework must stay vite");
  }
  if (template.buildCommand !== "npm run build") {
    errors.push("vercel admin adapter buildCommand must stay npm run build");
  }
  if (template.outputDirectory !== "dist") {
    errors.push("vercel admin adapter outputDirectory must stay dist");
  }

  const rewrite = values(template.rewrites).find((item) => item.source === "/(.*)" && item.destination === "/index.html");
  if (!rewrite) {
    errors.push("vercel admin adapter must include SPA fallback rewrite to /index.html");
  }
  for (const rewriteItem of values(template.rewrites)) {
    if (String(rewriteItem.source ?? "").startsWith("/api") || String(rewriteItem.destination ?? "").includes("/api/")) {
      errors.push("vercel admin adapter must not declare API rewrites; use VITE_PLATFORM_API_BASE or a reviewed edge proxy");
    }
  }

  for (const forbiddenField of ["functions", "builds", "routes"]) {
    if (template[forbiddenField] !== undefined) {
      errors.push(`vercel admin adapter must not declare ${forbiddenField}`);
    }
  }
  const serialized = JSON.stringify(template);
  for (const forbiddenSnippet of ["cmd/platform-api", "go build", "@vercel/go", "vercel-go-runtime"]) {
    if (serialized.includes(forbiddenSnippet)) {
      errors.push(`vercel admin adapter must not include API runtime snippet ${forbiddenSnippet}`);
    }
  }
}

function validateProductionRequirements(contract, errors) {
  const requirements = contract.productionApiRequirements ?? {};
  if (requirements.environment !== "PLATFORM_RUNTIME_ENV=production") {
    errors.push("productionApiRequirements.environment must stay PLATFORM_RUNTIME_ENV=production");
  }
  requireIncludes(
    requirements.requiredEnv,
    [
      "PLATFORM_JWT_SECRET",
      "PLATFORM_ADMIN_RESOURCE_DRIVER",
      "PLATFORM_ADMIN_RESOURCE_DSN",
      "PLATFORM_SESSION_DRIVER",
      "PLATFORM_SESSION_DSN",
      "PLATFORM_LIFECYCLE_HISTORY_DRIVER",
      "PLATFORM_LIFECYCLE_HISTORY_DSN",
      "PLATFORM_CACHE_DRIVER",
      "PLATFORM_REDIS_ADDR",
      "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER",
    ],
    "productionApiRequirements.requiredEnv",
    errors,
  );
  requireIncludes(requirements.forbiddenProductionCapabilities, ["demo-data"], "productionApiRequirements.forbiddenProductionCapabilities", errors);
}

function validateDeploymentPackage(contract, errors) {
  const deploymentPackage = contract.deploymentPackage ?? {};
  if (deploymentPackage.status !== "implemented") {
    errors.push("deploymentPackage.status must stay implemented");
  }
  if (deploymentPackage.defaultTopology !== "single-service-production") {
    errors.push("deploymentPackage.defaultTopology must stay single-service-production");
  }
  if (deploymentPackage.selectedTopology !== "single-service-production") {
    errors.push("deploymentPackage.selectedTopology must stay single-service-production");
  }
  const fileFields = [
    ["dockerfile", "Dockerfile"],
    ["composeFile", "deploy/compose/docker-compose.prod.yml"],
    ["adminProxy", "deploy/nginx/platform.conf"],
    ["envTemplate", "deploy/env/production.example.env"],
  ];
  for (const [field, expected] of fileFields) {
    const actual = deploymentPackage[field];
    if (actual !== expected) {
      errors.push(`deploymentPackage.${field} must stay ${expected}`);
    }
    if (!relativeExistingPath(actual)) {
      errors.push(`deploymentPackage.${field} path is missing or unsafe: ${actual}`);
    }
  }
  if (deploymentPackage.dockerTargets?.api !== "api") {
    errors.push("deploymentPackage.dockerTargets.api must stay api");
  }
  if (deploymentPackage.dockerTargets?.admin !== "admin-static") {
    errors.push("deploymentPackage.dockerTargets.admin must stay admin-static");
  }
  if (deploymentPackage.sameOrigin?.apiProxy !== "/api/") {
    errors.push("deploymentPackage.sameOrigin.apiProxy must stay /api/");
  }
  if (deploymentPackage.sameOrigin?.uploadAlias !== "/uploads/") {
    errors.push("deploymentPackage.sameOrigin.uploadAlias must stay /uploads/");
  }
  for (const snippet of values(deploymentPackage.requiredSourceSnippets)) {
    const snippetPath = snippet.path ?? "";
    if (!relativeExistingPath(snippetPath)) {
      errors.push(`deploymentPackage required snippet path is missing or unsafe: ${snippetPath}`);
      continue;
    }
    const contains = snippet.contains ?? "";
    if (!contains) {
      errors.push(`deploymentPackage required snippet for ${snippetPath} is missing contains`);
      continue;
    }
    if (!readRelativeFile(snippetPath).includes(contains)) {
      errors.push(`${snippetPath} must include ${contains}`);
    }
  }
  if (relativeExistingPath(deploymentPackage.envTemplate)) {
    const envTemplate = readRelativeFile(deploymentPackage.envTemplate);
    const capabilitiesLine = envTemplate.split(/\r?\n/).find((line) => line.startsWith("PLATFORM_CAPABILITIES=")) ?? "";
    if (!capabilitiesLine) {
      errors.push("deploymentPackage.envTemplate must declare PLATFORM_CAPABILITIES");
    }
    if (capabilitiesLine.includes("demo-data")) {
      errors.push("deploymentPackage.envTemplate PLATFORM_CAPABILITIES must not include demo-data");
    }
    for (const requiredEnv of [
      "PLATFORM_RUNTIME_ENV=production",
      "PLATFORM_CACHE_DRIVER=redis",
      "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
    ]) {
      if (!envTemplate.includes(requiredEnv)) {
        errors.push(`deploymentPackage.envTemplate must include ${requiredEnv}`);
      }
    }
  }
  if (relativeExistingPath(deploymentPackage.composeFile)) {
    const composeSource = readRelativeFile(deploymentPackage.composeFile);
    if (composeSource.includes("env_file:")) {
      errors.push("deploymentPackage.composeFile must use explicit environment mappings instead of env_file");
    }
  }
}

function validateDocs(contract, errors) {
  for (const docPath of values(contract.docs)) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`deployment topology doc path is missing or unsafe: ${docPath}`);
      continue;
    }
  }
  const docs = [
    [
      "docs/platform-deployment.md",
      [
        "Vercel is optional",
        "Selected scheme A is `single-service-production`",
        "not the default deployment target for the Gin API process",
        "deploy/vercel/admin.vercel.json",
        "admin/vercel.json",
        "VITE_PLATFORM_API_BASE",
        "PLATFORM_RUNTIME_ENV=production",
      ],
    ],
    ["README.md", ["Deployment scheme A is selected as the default", "docs/platform-deployment.md", "validate-platform-deployment-topology.mjs"]],
  ];
  for (const [docPath, snippets] of docs) {
    if (!relativeExistingPath(docPath)) {
      errors.push(`deployment topology required doc is missing: ${docPath}`);
      continue;
    }
    const source = readRelativeFile(docPath);
    for (const snippet of snippets) {
      if (!source.includes(snippet)) {
        errors.push(`${docPath} must include ${snippet}`);
      }
    }
  }
}

function validateEvidencePaths(contract, errors) {
  for (const relativePath of [...values(contract.validators), ...values(contract.tests)]) {
    if (!relativeExistingPath(relativePath)) {
      errors.push(`deployment topology evidence path is missing or unsafe: ${relativePath}`);
    }
  }
}

function validateProductionReadiness(readiness, errors) {
  const command = values(readiness.preflightCommands).find((item) => item.id === "deployment-topology");
  if (!command) {
    errors.push("production readiness preflight must include deployment-topology");
    return;
  }
  if (command.command !== "rtk node scripts/validate-platform-deployment-topology.mjs") {
    errors.push("deployment-topology preflight command must run scripts/validate-platform-deployment-topology.mjs");
  }
}

function validateEngineeringMatrix(matrix, errors) {
  const capability = values(matrix.capabilities).find((item) => item.id === "deployment-topology-gate");
  if (!capability) {
    errors.push("engineering capabilities must include deployment-topology-gate");
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("deployment-topology-gate status must be implemented");
  }
  requireIncludes(
    capability.evidence?.sourcePaths,
    ["resources/platform-deployment-topology.json", "docs/platform-deployment.md"],
    "deployment-topology-gate evidence.sourcePaths",
    errors,
  );
  requireIncludes(
    capability.evidence?.validators,
    ["scripts/validate-platform-deployment-topology.mjs"],
    "deployment-topology-gate evidence.validators",
    errors,
  );
  requireIncludes(
    capability.evidence?.tests,
    ["scripts/platform-deployment-topology.test.mjs"],
    "deployment-topology-gate evidence.tests",
    errors,
  );
}

const contract = readJSON(contractPath);
const readiness = readJSON(readinessPath);
const matrix = readJSON(matrixPath);
const errors = [];

validateDecision(contract, errors);
validateTopologies(contract, errors);
validateVercelPolicy(contract, errors);
validateVercelAdminAdapter(contract, errors);
validateProductionRequirements(contract, errors);
validateDeploymentPackage(contract, errors);
validateDocs(contract, errors);
validateEvidencePaths(contract, errors);
validateProductionReadiness(readiness, errors);
validateEngineeringMatrix(matrix, errors);

if (errors.length > 0) {
  for (const error of errors) {
    console.error(error);
  }
  process.exit(1);
}

console.log(`Validated platform deployment topology in ${path.relative(repoRoot, contractPath)}`);
