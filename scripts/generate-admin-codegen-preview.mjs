import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const contractPath = path.join(repoRoot, "resources", "generated", "admin-resource-contract.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "admin-codegen-preview.json");

const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
const resources = contract.resources ?? [];
const writeMethods = new Set(["POST", "PUT", "PATCH", "DELETE"]);

function uniqueSorted(values) {
  return Array.from(new Set(values.filter(Boolean))).sort();
}

function isWriteRoute(route) {
  if (route.path.endsWith("/queries") || route.path.endsWith("/query")) {
    return false;
  }
  return writeMethods.has(route.method);
}

function routeOperation(resource, route) {
  if (route.path.endsWith("/queries") || route.path.endsWith("/query")) return `${resource.name}.query`;
  if (!isWriteRoute(route)) return `${resource.name}.read`;
  if (route.auditAction) return `${resource.name}.${route.auditAction.split(".").pop()}`;
  if (route.method === "POST") return `${resource.name}.create`;
  if (route.method === "DELETE") return `${resource.name}.delete`;
  return `${resource.name}.update`;
}

function routeTarget(resource, route) {
  return {
    method: route.method,
    path: route.path,
    permission: route.permission,
    operationId: routeOperation(resource, route),
    mutates: isWriteRoute(route),
    auditActionCandidate: route.auditAction ?? null,
  };
}

function generationLevel(resource) {
  const mode = resource.codegen?.mode ?? "unknown";
  if (mode === "scaffold" || mode === "readOnly" || mode === "readOnlySeeded") {
    return "templateCandidate";
  }
  return "manualReview";
}

const previewResources = resources
  .map((resource) => {
    const routes = resource.routes.map((route) => routeTarget(resource, route));
    const writeRoutes = routes.filter((route) => route.mutates);
    return {
      resource: resource.name,
      code: resource.code,
      label: resource.label,
      group: resource.group,
      codegenMode: resource.codegen?.mode ?? "unknown",
      generationLevel: generationLevel(resource),
      backend: {
        model: resource.codegen?.model ?? null,
        table: resource.codegen?.table ?? null,
        apiBase: resource.apiBase,
        routeFile: "internal/platform/httpapi/server.go",
        modelPackage: "internal/platform/model",
        repositoryPackage: "internal/platform/repository",
        routes,
      },
      frontend: {
        resourceName: resource.refine.resource,
        adminPath: resource.refine.list,
        component: resource.refine.component,
        dataProviderFile: "admin/src/platform/refine/dataProvider.ts",
        pageComponent: "admin/src/platform/resources/GenericResourceConsole.tsx",
      },
      schema: {
        fieldCount: resource.schema.fields.length,
        search: resource.schema.search,
        filter: resource.schema.filter,
        sort: resource.schema.sort,
        table: resource.schema.table,
        form: resource.schema.form,
        localizedFields: resource.schema.localizedFields,
      },
      docs: {
        openapi: "resources/generated/openapi.admin.json",
        scaffoldDraft: "resources/generated/admin-scaffold-draft.md",
      },
      audit: {
        required: writeRoutes.length > 0,
        actionCandidates: uniqueSorted(writeRoutes.map((route) => route.auditActionCandidate)),
      },
    };
  })
  .sort((a, b) => a.resource.localeCompare(b.resource));

const preview = {
  generatedBy: "scripts/generate-admin-codegen-preview.mjs",
  source: "resources/generated/admin-resource-contract.json",
  sourceVersion: contract.sourceVersion,
  resourceCount: previewResources.length,
  summary: {
    modes: contract.codegenBuckets ?? {},
    templateCandidates: previewResources.filter((resource) => resource.generationLevel === "templateCandidate").map((resource) => resource.resource),
    manualReview: previewResources.filter((resource) => resource.generationLevel === "manualReview").map((resource) => resource.resource),
    writeResources: previewResources.filter((resource) => resource.audit.required).map((resource) => resource.resource),
    permissions: contract.permissions ?? [],
    routeCount: contract.routes?.length ?? 0,
    schemaCount: Object.keys(contract.schemas ?? {}).length,
  },
  guardrails: [
    "This preview is read-only and must not overwrite Go, React or Markdown files.",
    "Source-writing code generation is deferred until GORM models and repository contracts are stable.",
    "Every write route must keep permission code, tenant scope and audit action together.",
    "Runtime form slot descriptors are controlled by schema and frontend registries; source-writing form generators remain disabled.",
    "Resource-localized data uses schema.localizedFields plus <field>Zh/<field>En values; business resources can opt in per field.",
  ],
  resources: previewResources,
};

const output = `${JSON.stringify(preview, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
}
