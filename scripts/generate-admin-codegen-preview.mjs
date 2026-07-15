import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  adminServiceObjectDefinitions,
  forbiddenServiceObjectClientInputs,
} from "./admin-service-object-definitions.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const contractPath = optionValue("--contract")
  ? path.resolve(repoRoot, optionValue("--contract"))
  : path.join(repoRoot, "resources", "generated", "admin-resource-contract.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "admin-codegen-preview.json");
const generatedTypeScriptPath = path.join(generatedDir, "admin-service-object-client.ts");

const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
const resources = contract.resources ?? [];
const writeMethods = new Set(["POST", "PUT", "PATCH", "DELETE"]);

function optionValue(name) {
  const index = process.argv.indexOf(name);
  if (index >= 0 && process.argv[index + 1]) {
    return process.argv[index + 1];
  }
  const prefix = `${name}=`;
  const value = process.argv.find((arg) => arg.startsWith(prefix));
  return value ? value.slice(prefix.length) : "";
}

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

function serviceObjectTypePrefix(definition) {
  return `${definition.codegenName}V${definition.version.replaceAll(".", "_")}`;
}

function typeScriptValueType(field) {
  if (field.type === "integer") return "number";
  if (field.type === "boolean") return "boolean";
  if (field.type === "string-set") return "ReadonlyArray<string>";
  if (field.type === "menu-definition") return "AdminServiceObjectMenuDefinition";
  if (field.type === "role-remediations") return "ReadonlyArray<AdminServiceObjectRoleRemediation>";
  return "string";
}

function typeScriptObject(fields, { optionalByDefault = false, readonly = false } = {}) {
  const lines = fields.map((field) => {
    const optional = optionalByDefault ? field.required !== true : field.required === false;
    return `  ${readonly ? "readonly " : ""}${JSON.stringify(field.name)}${optional ? "?" : ""}: ${typeScriptValueType(field)};`;
  });
  return ["{", ...lines, "}"].join("\n");
}

function queryTypeScript(definition) {
  const prefix = serviceObjectTypePrefix(definition);
  const sortNames = definition.allowedSort.map((name) => JSON.stringify(name)).join(" | ") || "never";
  const totalField = definition.exposeTotal ? "\n  readonly total: number;" : "";
  const argumentsOptional = definition.arguments.some((argument) => argument.required) ? "" : "?";
  return `export type ${prefix}Arguments = ${typeScriptObject(definition.arguments, { optionalByDefault: true })};

export type ${prefix}Sort = {
  readonly name: ${sortNames};
  readonly order: "asc" | "desc";
};

export type ${prefix}Item = ${typeScriptObject(definition.result, { readonly: true })};

export type ${prefix}QueryInput = {
  readonly arguments${argumentsOptional}: ${prefix}Arguments;
  readonly pagination?: {
    readonly page?: number;
    readonly pageSize?: number;
  };
  readonly sort?: ReadonlyArray<${prefix}Sort>;
};

export type ${prefix}QueryRequest = ${prefix}QueryInput & {
  readonly queryId: ${JSON.stringify(definition.id)};
  readonly version: ${JSON.stringify(definition.version)};
};

export type ${prefix}QueryData = {
  readonly items: ReadonlyArray<${prefix}Item>;
  readonly page: number;
  readonly pageSize: number;${totalField}
};`;
}

function commandTypeScript(definition) {
  const prefix = serviceObjectTypePrefix(definition);
  const argumentsOptional = definition.arguments.some((argument) => argument.required) ? "" : "?";
  const idempotencyField =
    definition.idempotency === "required-key"
      ? "  readonly idempotencyKey: string;"
      : "  readonly idempotencyKey?: string;";
  return `export type ${prefix}Arguments = ${typeScriptObject(definition.arguments, { optionalByDefault: true })};

export type ${prefix}Values = ${typeScriptObject(definition.result, { readonly: true })};

export type ${prefix}CommandInput = {
  readonly arguments${argumentsOptional}: ${prefix}Arguments;
${idempotencyField}
};

export type ${prefix}CommandRequest = ${prefix}CommandInput & {
  readonly commandId: ${JSON.stringify(definition.id)};
  readonly version: ${JSON.stringify(definition.version)};
};

export type ${prefix}CommandData = {
  readonly values: ${prefix}Values;
};`;
}

function clientMethodTypeScript(kind, definition) {
  const prefix = serviceObjectTypePrefix(definition);
  const capitalKind = kind === "query" ? "Query" : "Command";
  const idProperty = kind === "query" ? "queryId" : "commandId";
  const path = `/api/admin/service-objects/${kind}`;
  const hasRequiredInput =
    definition.arguments.some((argument) => argument.required) ||
    (kind === "command" && definition.idempotency === "required-key");
  const defaultInput = hasRequiredInput ? "" : " = {}";
  return `  ${definition.clientMethod}(input: ${prefix}${capitalKind}Input${defaultInput}): Promise<AdminServiceObjectResponse<${prefix}${capitalKind}Data>> {
    const request: ${prefix}${capitalKind}Request = {
      ...input,
      ${idProperty}: ${JSON.stringify(definition.id)},
      version: ${JSON.stringify(definition.version)},
    };
    return this.transport.post<${prefix}${capitalKind}Data, ${prefix}${capitalKind}Request>(${JSON.stringify(path)}, request);
  }`;
}

function generateAdminServiceObjectTypeScript() {
  const definitions = [
    ...adminServiceObjectDefinitions.queries.map(queryTypeScript),
    ...adminServiceObjectDefinitions.commands.map(commandTypeScript),
  ].join("\n\n");
  const methods = [
    ...adminServiceObjectDefinitions.queries.map((definition) => clientMethodTypeScript("query", definition)),
    ...adminServiceObjectDefinitions.commands.map((definition) => clientMethodTypeScript("command", definition)),
  ].join("\n\n");

  return `// Generated by scripts/generate-admin-codegen-preview.mjs. Do not edit manually.
// Codegen definitions: ${adminServiceObjectDefinitions.source}
// Runtime references: ${adminServiceObjectDefinitions.runtimeSources.join(", ")}

export type AdminServiceObjectError = {
  readonly code: string;
  readonly message: string;
};

export type AdminServiceObjectResponse<TData> = {
  readonly data?: TData;
  readonly error?: AdminServiceObjectError;
};

export interface AdminServiceObjectTransport {
  post<TData, TRequest>(path: string, request: TRequest): Promise<AdminServiceObjectResponse<TData>>;
}

export type AdminServiceObjectRoleRemediation =
  | {
      readonly userCode: string;
      readonly roleCode: string;
      readonly action: "remove-role";
    }
  | {
      readonly userCode: string;
      readonly roleCode: string;
      readonly action: "replace-role";
      readonly replacementRoleCode: string;
    };

export type AdminServiceObjectStringSet = ReadonlyArray<string>;

export type AdminServiceObjectMenuParameter =
  | { readonly key: string; readonly type: "string"; readonly value: string }
  | { readonly key: string; readonly type: "number"; readonly value: number }
  | { readonly key: string; readonly type: "boolean"; readonly value: boolean };

export type AdminServiceObjectMenuNode = {
  readonly code: string;
  readonly parentCode: string;
  readonly nodeType: "directory" | "page";
  readonly titleZh: string;
  readonly titleEn: string;
  readonly descriptionZh: string;
  readonly descriptionEn: string;
  readonly status: "enabled" | "disabled";
  readonly icon: string;
  readonly sortOrder: number;
  readonly route: string;
  readonly componentKey: string;
  readonly resourceCode: string;
  readonly external: boolean;
  readonly externalUrl: string;
  readonly openMode: "" | "same-tab" | "new-tab";
  readonly parameters: ReadonlyArray<AdminServiceObjectMenuParameter>;
  readonly cacheEnabled: boolean;
  readonly hidden: boolean;
  readonly activeMenuCode: string;
  readonly breadcrumbVisible: boolean;
};

export type AdminServiceObjectPageButton = {
  readonly menuCode: string;
  readonly buttonKey: string;
  readonly labelZh: string;
  readonly labelEn: string;
  readonly action: string;
  readonly sortOrder: number;
  readonly status: "enabled" | "disabled";
  readonly permissionCode: string;
};

export type AdminServiceObjectMenuDefinition = {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly updatedAt: string;
  readonly node: AdminServiceObjectMenuNode;
  readonly buttons: ReadonlyArray<AdminServiceObjectPageButton>;
};

${definitions}

export class AdminServiceObjectClient {
  constructor(private readonly transport: AdminServiceObjectTransport) {}

${methods}
}
`;
}

function serviceObjectPreview(kind, definition) {
  const prefix = serviceObjectTypePrefix(definition);
  const capitalKind = kind === "query" ? "Query" : "Command";
  return {
    kind,
    id: definition.id,
    version: definition.version,
    clientMethod: definition.clientMethod,
    requestType: `${prefix}${capitalKind}Request`,
    inputType: `${prefix}${capitalKind}Input`,
    responseType: `${prefix}${capitalKind}Data`,
    argumentNames: definition.arguments.map((argument) => argument.name),
    additionalPermissions: definition.additionalPermissions ?? [],
    ...(kind === "query" ? { logicalSortNames: definition.allowedSort } : {}),
    ...(kind === "command" && definition.operationPhase ? { operationPhase: definition.operationPhase } : {}),
    limits: {
      timeoutMs: definition.timeoutMs,
      cost: definition.cost,
      ...(kind === "query"
        ? { maxPageSize: definition.maxPageSize, exposeTotal: definition.exposeTotal }
        : { idempotency: definition.idempotency, maxAffectedRows: definition.maxAffectedRows }),
    },
  };
}

const previewResources = resources
  .map((resource) => {
    const routes = resource.routes.map((route) => routeTarget(resource, route));
    const writeRoutes = routes.filter((route) => route.mutates);
    const protectedFields = resource.schema.fields
      .filter((field) => field.protection)
      .map((field) => ({ key: field.key, ...field.protection }));
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
        ...(resource.schema.protection ? { protection: resource.schema.protection } : {}),
        ...(protectedFields.length > 0 ? { protectedFields } : {}),
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
  serviceObjects: {
    runtimePackage: "internal/platform/serviceobject",
    transportFile: "internal/platform/httpapi/server.go",
    definitionSource: adminServiceObjectDefinitions.source,
    runtimeDefinitionSource: adminServiceObjectDefinitions.runtimeSource,
    runtimeDefinitionSources: adminServiceObjectDefinitions.runtimeSources,
    typescriptClient: "resources/generated/admin-service-object-client.ts",
    transportContract: "AdminServiceObjectTransport",
    runtime: "conditional",
    unavailableError: "SERVICE_OBJECT_UNAVAILABLE",
    operations: [
      ...adminServiceObjectDefinitions.queries.map((definition) => serviceObjectPreview("query", definition)),
      ...adminServiceObjectDefinitions.commands.map((definition) => serviceObjectPreview("command", definition)),
    ],
    forbiddenClientInputs: forbiddenServiceObjectClientInputs,
  },
  resources: previewResources,
};

const output = `${JSON.stringify(preview, null, 2)}\n`;
const typeScriptOutput = generateAdminServiceObjectTypeScript();
if (process.argv.includes("--typescript-stdout")) {
  process.stdout.write(typeScriptOutput);
} else if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  fs.writeFileSync(generatedTypeScriptPath, typeScriptOutput);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
  console.log(`Generated ${path.relative(repoRoot, generatedTypeScriptPath)}`);
}
