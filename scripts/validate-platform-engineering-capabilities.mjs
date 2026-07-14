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

const matrixPath = path.resolve(repoRoot, argValue("--matrix", "resources/platform-engineering-capabilities.json"));
const adminContractPath = path.resolve(repoRoot, argValue("--admin-contract", "resources/generated/admin-resource-contract.json"));
const adminOpenAPIPath = path.resolve(repoRoot, argValue("--admin-openapi", "resources/generated/openapi.admin.json"));
const appOpenAPIPath = path.resolve(repoRoot, argValue("--app-openapi", "resources/generated/openapi.app.json"));
const scaffoldPlanPath = path.resolve(repoRoot, argValue("--scaffold-plan", "resources/generated/admin-scaffold-plan.json"));
const goModPath = path.resolve(repoRoot, argValue("--go-mod", "go.mod"));
const adminPackagePath = path.resolve(repoRoot, argValue("--admin-package", "admin/package.json"));
const stackSourceRoot = path.resolve(repoRoot, argValue("--stack-source-root", "."));

const allowedStatuses = new Set(["implemented", "preview-scaffold", "partial", "deferred"]);
const requiredImplementedCapabilityIDs = [
  "dynamic-admin-resources",
  "dynamic-menus",
  "permission-codes",
  "resource-schema-form-engine",
  "form-schema-layout-slot-gate",
  "governance-org-area-role-groups",
  "capability-contract-governance",
  "capability-profile-composition-gate",
  "openapi-api-docs",
  "admin-api-boundary-query-security",
  "safe-codegen-scaffold",
  "codegen-source-writing-readiness",
  "system-management",
  "app-route-contracts",
  "app-client-api-boundary",
  "production-persistence-correctness",
  "production-runtime-gate",
  "production-auth-hardening-gate",
  "runtime-cache-invalidation",
  "file-operation-audit-contract",
  "file-storage-admin-experience",
  "task-dependency-governance",
  "foundation-alignment-audit",
  "goal-completion-audit",
  "node-closeout-audit",
  "objective-conformance-gate",
  "reference-discovery-gate",
  "reference-coverage-boundary-gate",
  "personnel-runtime-readiness",
  "production-readiness-preflight",
  "deployment-topology-gate",
  "runtime-security-containment",
  "admin-watermark-export-governance",
  "sensitive-data-protection",
  "mask-strategy-runtime",
  "sensitive-data-reveal-step-up",
  "data-lifecycle-retention",
];
const requiredPartialCapabilityIDs = [
  "platform-service-contract-standard",
  "persisted-query-command-object-runtime",
  "integration-ports-disabled-default",
  "organization-rbac-menu-contract-and-migration-design",
  "organization-role-pool-backend-and-migration",
  "organization-user-admin-experience",
  "role-tree-and-authorization-entry",
  "menu-tree-and-button-permission-configuration",
  "organization-rbac-menu-e2e-qa",
  "multi-datasource-contract-and-runtime",
  "tenant-placement-and-request-routing",
  "datasource-read-write-routing",
  "sharding-and-tenant-migration",
  "federated-read-query",
  "xa-optional-adapter",
  "database-certification-matrix",
  "transactional-outbox-and-one-mq-adapter",
  "asynchronous-search-projection",
  "open-source-portability",
  "public-documentation-and-release",
];
const requiredPartialCapabilityDependencies = {
  "platform-service-contract-standard": ["data-lifecycle-retention"],
  "persisted-query-command-object-runtime": ["platform-service-contract-standard"],
  "integration-ports-disabled-default": ["platform-service-contract-standard"],
  "organization-rbac-menu-contract-and-migration-design": ["persisted-query-command-object-runtime"],
  "organization-role-pool-backend-and-migration": ["organization-rbac-menu-contract-and-migration-design"],
  "organization-user-admin-experience": ["organization-role-pool-backend-and-migration"],
  "role-tree-and-authorization-entry": ["organization-user-admin-experience"],
  "menu-tree-and-button-permission-configuration": ["role-tree-and-authorization-entry"],
  "organization-rbac-menu-e2e-qa": [
    "organization-user-admin-experience",
    "role-tree-and-authorization-entry",
    "menu-tree-and-button-permission-configuration",
  ],
  "multi-datasource-contract-and-runtime": ["platform-service-contract-standard"],
  "tenant-placement-and-request-routing": [
    "multi-datasource-contract-and-runtime",
    "organization-role-pool-backend-and-migration",
  ],
  "datasource-read-write-routing": ["tenant-placement-and-request-routing"],
  "sharding-and-tenant-migration": ["datasource-read-write-routing"],
  "federated-read-query": ["sharding-and-tenant-migration", "persisted-query-command-object-runtime"],
  "xa-optional-adapter": ["federated-read-query"],
  "database-certification-matrix": ["xa-optional-adapter"],
  "transactional-outbox-and-one-mq-adapter": [
    "integration-ports-disabled-default",
    "database-certification-matrix",
  ],
  "asynchronous-search-projection": [
    "transactional-outbox-and-one-mq-adapter",
    "persisted-query-command-object-runtime",
  ],
  "open-source-portability": [
    "runtime-security-containment",
    "admin-watermark-export-governance",
    "sensitive-data-protection",
    "organization-rbac-menu-e2e-qa",
    "asynchronous-search-projection",
  ],
  "public-documentation-and-release": ["open-source-portability"],
};
const requiredCapabilityIDs = [...requiredImplementedCapabilityIDs, ...requiredPartialCapabilityIDs];

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function sameOrderedValues(actual, expected) {
  return actual.length === expected.length && actual.every((value, index) => value === expected[index]);
}

function hasLocalizedText(value) {
  return typeof value?.zh === "string" && value.zh.trim() !== "" && typeof value?.en === "string" && value.en.trim() !== "";
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
  const absolutePath = path.resolve(repoRoot, relativePath);
  return fs.readFileSync(absolutePath, "utf8");
}

function relativeExistingStackSource(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) {
    return false;
  }
  const absolutePath = path.resolve(stackSourceRoot, relativePath);
  const relative = path.relative(stackSourceRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function readStackSource(relativePath) {
  return fs.readFileSync(path.resolve(stackSourceRoot, relativePath), "utf8");
}

function requireIncludes(source, expected, label, errors) {
  if (!source.includes(expected)) {
    errors.push(`${label} must include ${expected}`);
  }
}

function dependencyVersion(dependencies, name) {
  const value = dependencies?.[name];
  return typeof value === "string" && value.trim() !== "" ? value : "";
}

function validateStackEvidence({ matrix, goMod, adminPackage, errors }) {
  if (matrix.stack?.backend?.join(",") !== "Gin,GORM,Casbin,JWT") {
    errors.push("engineering matrix stack.backend must stay Gin + GORM + Casbin + JWT");
  }
  if (matrix.stack?.frontend?.join(",") !== "Refine,React,Ant Design") {
    errors.push("engineering matrix stack.frontend must stay Refine + React + Ant Design");
  }

  const backendDependencies = [
    ["Gin", "github.com/gin-gonic/gin"],
    ["GORM", "gorm.io/gorm"],
    ["Casbin", "github.com/casbin/casbin/v2"],
    ["JWT", "github.com/golang-jwt/jwt/v5"],
  ];
  for (const [label, moduleName] of backendDependencies) {
    if (!goMod.includes(moduleName)) {
      errors.push(`backend stack dependency ${label} is missing from go.mod: ${moduleName}`);
    }
  }

  const frontendDependencies = [
    ["Refine core", "@refinedev/core"],
    ["Refine Ant Design adapter", "@refinedev/antd"],
    ["Refine React Router adapter", "@refinedev/react-router"],
    ["React", "react"],
    ["ReactDOM", "react-dom"],
    ["Ant Design", "antd"],
  ];
  const dependencies = { ...(adminPackage.dependencies ?? {}), ...(adminPackage.devDependencies ?? {}) };
  for (const [label, packageName] of frontendDependencies) {
    if (!dependencyVersion(dependencies, packageName)) {
      errors.push(`frontend stack dependency ${label} is missing from admin/package.json: ${packageName}`);
    }
  }

  const stackSources = [
    ["Gin HTTP runtime", "internal/platform/httpapi/server.go", "\"github.com/gin-gonic/gin\""],
    ["GORM admin resource repository", "internal/platform/adminresource/gorm_store.go", "\"gorm.io/gorm\""],
    ["Casbin authorizer", "internal/platform/authz/casbin.go", "\"github.com/casbin/casbin/v2\""],
    ["Casbin policy bridge", "internal/platform/adminresource/authorization.go", "authz.NewCasbinAuthorizer"],
    ["JWT token service", "internal/platform/httpapi/server.go", "authjwt.NewService"],
    ["Refine runtime", "admin/src/App.tsx", "import { Refine"],
    ["Refine runtime providers", "admin/src/App.tsx", "accessControlProvider={accessControlProvider}"],
    ["Refine data provider", "admin/src/platform/refine/dataProvider.ts", "export const dataProvider"],
    ["Refine access-control provider", "admin/src/platform/refine/accessControlProvider.ts", "export const accessControlProvider"],
    ["Ant Design app shell", "admin/src/App.tsx", "import { App as AntdApp"],
  ];
  for (const [label, relativePath, snippet] of stackSources) {
    if (!relativeExistingStackSource(relativePath)) {
      errors.push(`${label} source path is missing: ${relativePath}`);
      continue;
    }
    requireIncludes(readStackSource(relativePath), snippet, label, errors);
  }
}

function uniqueErrors(valuesToCheck, label) {
  const seen = new Set();
  const errors = [];
  for (const value of valuesToCheck) {
    if (!value) {
      errors.push(`${label} contains an empty value`);
      continue;
    }
    if (seen.has(value)) {
      errors.push(`${label} contains duplicate value ${value}`);
    }
    seen.add(value);
  }
  return errors;
}

function validateScaffoldPlan(expected, actual) {
  const errors = [];
  if (!expected) {
    return errors;
  }
  for (const key of ["sourceWriting", "dryRun"]) {
    if (expected[key] !== undefined && actual.mode?.[key] !== expected[key]) {
      errors.push(`safe-codegen-scaffold scaffold plan mode.${key} must be ${expected[key]}`);
    }
  }
  if (Array.isArray(expected.allowedWriteRoots)) {
    const actualRoots = actual.mode?.allowedWriteRoots ?? [];
    for (const root of expected.allowedWriteRoots) {
      if (!actualRoots.includes(root)) {
        errors.push(`safe-codegen-scaffold scaffold plan missing allowed write root ${root}`);
      }
    }
  }
  for (const key of ["conflictCount", "unsafePathCount"]) {
    if (expected[key] !== undefined && actual.summary?.[key] !== expected[key]) {
      errors.push(`safe-codegen-scaffold scaffold plan summary.${key} must be ${expected[key]}`);
    }
  }
  return errors;
}

function validateSafeCodegenScaffold(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("safe-codegen-scaffold status must be implemented");
  }
  if (capability.evidence?.scaffoldPlan?.sourceWriting !== "disabled") {
    errors.push("safe-codegen-scaffold evidence.scaffoldPlan.sourceWriting must be disabled");
  }
  if (capability.evidence?.scaffoldPlan?.dryRun !== true) {
    errors.push("safe-codegen-scaffold evidence.scaffoldPlan.dryRun must stay true");
  }
  if (!values(capability.evidence?.generatedFiles).includes("resources/generated/admin-scaffold-promotion-review.json")) {
    errors.push("safe-codegen-scaffold must include admin scaffold promotion review generated evidence");
  }
}

function validateRuntimeCacheInvalidation(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("runtime-cache-invalidation status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-cache-invalidation.json")) {
    errors.push("runtime-cache-invalidation must cite resources/platform-cache-invalidation.json");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-cache-invalidation.mjs")) {
    errors.push("runtime-cache-invalidation must cite validate-platform-cache-invalidation.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-cache-invalidation.test.mjs")) {
    errors.push("runtime-cache-invalidation must cite platform-cache-invalidation.test.mjs");
  }
}

function validateProductionAuthHardening(capability, errors) {
  if (!capability) {
    return;
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-foundation-task-graph.json")) {
    errors.push("production-auth-hardening-gate must cite resources/platform-foundation-task-graph.json");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/evidence/production-admin-oidc-auth-20260711.json")) {
    errors.push("production-auth-hardening-gate must cite the tracked production Admin OIDC evidence manifest");
  }
  if (!values(capability.evidence?.tests).includes("internal/platform/authprovider/oidc/resolver_test.go")) {
    errors.push("production-auth-hardening-gate must cite internal/platform/authprovider/oidc/resolver_test.go");
  }
  for (const requirement of ["production-like-oidc-rehearsal", "six-viewport-browser-acceptance", "neat-freak-cleanup-closeout"]) {
    if (!values(capability.completedEvidence).includes(requirement)) {
      errors.push(`production-auth-hardening-gate completedEvidence must include ${requirement}`);
    }
  }
}

function validateAdminAPIBoundaryQuerySecurity(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("admin-api-boundary-query-security status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-admin-api-boundary.json")) {
    errors.push("admin-api-boundary-query-security must cite resources/platform-admin-api-boundary.json");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-admin-api-boundary.mjs")) {
    errors.push("admin-api-boundary-query-security must cite validate-platform-admin-api-boundary.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-admin-api-boundary.test.mjs")) {
    errors.push("admin-api-boundary-query-security must cite platform-admin-api-boundary.test.mjs");
  }
}

function validateFileStorageAdminExperience(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("file-storage-admin-experience status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-file-storage-experience.json")) {
    errors.push("file-storage-admin-experience must cite resources/platform-file-storage-experience.json");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-file-storage-experience.mjs")) {
    errors.push("file-storage-admin-experience must cite validate-platform-file-storage-experience.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-file-storage-experience.test.mjs")) {
    errors.push("file-storage-admin-experience must cite platform-file-storage-experience.test.mjs");
  }
  const screenshots = values(capability.evidence?.screenshots);
  if (screenshots.length < 4) {
    errors.push("file-storage-admin-experience must cite desktop and mobile browser screenshot evidence");
  }
  for (const screenshot of screenshots) {
    if (!relativeExistingPath(screenshot)) {
      errors.push(`file-storage-admin-experience screenshot path is missing or unsafe: ${screenshot}`);
    }
  }
}

function validateDeploymentTopologyGate(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("deployment-topology-gate status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-deployment-topology.json")) {
    errors.push("deployment-topology-gate must cite resources/platform-deployment-topology.json");
  }
  if (!values(capability.evidence?.sourcePaths).includes("docs/platform-deployment.md")) {
    errors.push("deployment-topology-gate must cite docs/platform-deployment.md");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-deployment-topology.mjs")) {
    errors.push("deployment-topology-gate must cite validate-platform-deployment-topology.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-deployment-topology.test.mjs")) {
    errors.push("deployment-topology-gate must cite platform-deployment-topology.test.mjs");
  }
}

function validateCapabilityContractGovernance(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("capability-contract-governance status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-capability-contracts.json")) {
    errors.push("capability-contract-governance must cite resources/platform-capability-contracts.json");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-capability-contracts.mjs")) {
    errors.push("capability-contract-governance must cite validate-platform-capability-contracts.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-capability-contracts.test.mjs")) {
    errors.push("capability-contract-governance must cite platform-capability-contracts.test.mjs");
  }
}

function validateAppClientAPIBoundary(capability, errors) {
  if (!capability) {
    return;
  }
  if (capability.status !== "implemented") {
    errors.push("app-client-api-boundary status must be implemented");
  }
  if (!values(capability.evidence?.sourcePaths).includes("resources/platform-app-client-api-boundary.json")) {
    errors.push("app-client-api-boundary must cite resources/platform-app-client-api-boundary.json");
  }
  if (!values(capability.evidence?.validators).includes("scripts/validate-platform-app-client-api-boundary.mjs")) {
    errors.push("app-client-api-boundary must cite validate-platform-app-client-api-boundary.mjs");
  }
  if (!values(capability.evidence?.tests).includes("scripts/platform-app-client-api-boundary.test.mjs")) {
    errors.push("app-client-api-boundary must cite platform-app-client-api-boundary.test.mjs");
  }
}

function validate() {
  const matrix = readJSON(matrixPath);
  const adminContract = readJSON(adminContractPath);
  const adminOpenAPI = readJSON(adminOpenAPIPath);
  const appOpenAPI = readJSON(appOpenAPIPath);
  const scaffoldPlan = readJSON(scaffoldPlanPath);
  const goMod = fs.readFileSync(goModPath, "utf8");
  const adminPackage = readJSON(adminPackagePath);
  const errors = [];

  validateStackEvidence({ matrix, goMod, adminPackage, errors });

  const capabilities = values(matrix.capabilities);
  errors.push(...uniqueErrors(capabilities.map((capability) => capability.id), "capabilities.id"));
  const capabilityIDs = new Set(capabilities.map((capability) => capability.id));
  const capabilityByID = new Map(capabilities.map((capability) => [capability.id, capability]));
  for (const capabilityID of requiredCapabilityIDs) {
    if (!capabilityIDs.has(capabilityID)) {
      errors.push(`engineering capability matrix is missing required capability ${capabilityID}`);
    }
  }
  for (const capabilityID of requiredImplementedCapabilityIDs) {
    const capability = capabilityByID.get(capabilityID);
    if (capability && capability.status !== "implemented") {
      errors.push(`required implemented capability ${capabilityID} must stay implemented`);
    }
  }
  for (const capabilityID of requiredPartialCapabilityIDs) {
    const capability = capabilityByID.get(capabilityID);
    if (capability && capability.status !== "partial") {
      errors.push(`approved completion program capability ${capabilityID} must stay partial until implementation closeout`);
    }
    if (capability) {
      const expectedDependencies = requiredPartialCapabilityDependencies[capabilityID];
      const actualDependencies = values(capability.dependsOn);
      if (!sameOrderedValues(actualDependencies, expectedDependencies)) {
        errors.push(
          `approved completion program capability ${capabilityID} dependsOn must equal ${JSON.stringify(expectedDependencies)}`,
        );
      }
    }
  }
  validateSafeCodegenScaffold(capabilityByID.get("safe-codegen-scaffold"), errors);
  validateProductionAuthHardening(capabilityByID.get("production-auth-hardening-gate"), errors);
  validateRuntimeCacheInvalidation(capabilityByID.get("runtime-cache-invalidation"), errors);
  validateAdminAPIBoundaryQuerySecurity(capabilityByID.get("admin-api-boundary-query-security"), errors);
  validateFileStorageAdminExperience(capabilityByID.get("file-storage-admin-experience"), errors);
  validateDeploymentTopologyGate(capabilityByID.get("deployment-topology-gate"), errors);
  validateCapabilityContractGovernance(capabilityByID.get("capability-contract-governance"), errors);
  validateAppClientAPIBoundary(capabilityByID.get("app-client-api-boundary"), errors);
  const adminResourceCodes = new Set((adminContract.resources ?? []).map((resource) => resource.code).filter(Boolean));
  const adminPaths = new Set(Object.keys(adminOpenAPI.paths ?? {}));
  const appPaths = new Set(Object.keys(appOpenAPI.paths ?? {}));

  for (const capability of capabilities) {
    const prefix = `engineering capability ${capability.id ?? "<missing>"}`;
    if (!capability.id) {
      errors.push("engineering capability is missing id");
      continue;
    }
    if (!hasLocalizedText(capability.label)) {
      errors.push(`${prefix} must declare zh/en label`);
    }
    if (!allowedStatuses.has(capability.status)) {
      errors.push(`${prefix} has unsupported status ${capability.status}`);
    }
    if (!capability.purpose) {
      errors.push(`${prefix} must declare purpose`);
    }
    for (const dependency of values(capability.dependsOn)) {
      if (!capabilityIDs.has(dependency)) {
        errors.push(`${prefix} depends on unknown capability ${dependency}`);
      }
    }

    const evidence = capability.evidence ?? {};
    const sourcePaths = values(evidence.sourcePaths);
    const generatedFiles = values(evidence.generatedFiles);
    const validators = values(evidence.validators);
    const tests = values(evidence.tests);
    if (capability.status === "implemented" && sourcePaths.length === 0 && generatedFiles.length === 0) {
      errors.push(`${prefix} must declare sourcePaths or generatedFiles evidence`);
    }
    for (const relativePath of [...sourcePaths, ...generatedFiles, ...validators, ...tests]) {
      if (!relativeExistingPath(relativePath)) {
        errors.push(`${prefix} evidence path is missing or unsafe: ${relativePath}`);
      }
    }
    for (const resource of values(evidence.adminResources)) {
      if (!adminResourceCodes.has(resource)) {
        errors.push(`${prefix} missing admin resource ${resource}`);
      }
    }
    for (const openAPIPath of values(evidence.adminOpenAPIPaths)) {
      if (!adminPaths.has(openAPIPath)) {
        errors.push(`${prefix} missing admin OpenAPI path ${openAPIPath}`);
      }
    }
    for (const openAPIPath of values(evidence.appOpenAPIPaths)) {
      if (!appPaths.has(openAPIPath)) {
        errors.push(`${prefix} missing app OpenAPI path ${openAPIPath}`);
      }
    }
    for (const snippet of values(evidence.requiredSourceSnippets)) {
      if (!relativeExistingPath(snippet.path)) {
        errors.push(`${prefix} snippet path is missing or unsafe: ${snippet.path}`);
        continue;
      }
      if (!snippet.contains) {
        errors.push(`${prefix} snippet for ${snippet.path} must declare contains`);
        continue;
      }
      const source = readRelativeFile(snippet.path);
      if (!source.includes(snippet.contains)) {
        errors.push(`${prefix} source ${snippet.path} is missing snippet ${snippet.contains}`);
      }
    }
    errors.push(...validateScaffoldPlan(evidence.scaffoldPlan, scaffoldPlan));
  }

  return { matrix, errors };
}

const { matrix, errors } = validate();
if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log(`Validated ${matrix.capabilities?.length ?? 0} platform engineering capabilities in ${path.relative(repoRoot, matrixPath)}`);
