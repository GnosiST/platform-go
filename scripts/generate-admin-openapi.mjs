import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { adminServiceObjectDefinitions } from "./admin-service-object-definitions.mjs";
import {
  errorResponse as platformErrorResponse,
  loadPlatformErrorContract,
  platformErrorOpenAPISchemas,
  platformErrorRegistryExtensions,
} from "./platform-error-contract.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const contractPath = optionValue("--contract")
  ? path.resolve(repoRoot, optionValue("--contract"))
  : path.join(repoRoot, "resources", "generated", "admin-resource-contract.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "openapi.admin.json");

const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
const errorContract = loadPlatformErrorContract(repoRoot);
const resources = contract.resources ?? [];
const usedOperationIds = new Set();

function optionValue(name) {
  const index = process.argv.indexOf(name);
  if (index >= 0 && process.argv[index + 1]) {
    return process.argv[index + 1];
  }
  const prefix = `${name}=`;
  const value = process.argv.find((arg) => arg.startsWith(prefix));
  return value ? value.slice(prefix.length) : "";
}

function localized(value) {
  if (!value) return "";
  return value.en || value.zh || "";
}

function pascalCase(value) {
  return String(value)
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

function camelCase(...parts) {
  const value = pascalCase(parts.join(" "));
  return value.charAt(0).toLowerCase() + value.slice(1);
}

function uniqueOperationId(resource, route) {
  const params = pathParams(route.path);
  let action = "call";
  if (route.path.endsWith("/queries") || route.path.endsWith("/query")) {
    action = "query";
  } else if (route.path.endsWith("/apply")) {
    action = "apply";
  } else if (route.path.endsWith("/request")) {
    action = "request";
  } else if (route.path.endsWith("/approve")) {
    action = "approve";
  } else if (route.path.endsWith("/reject")) {
    action = "reject";
  } else if (route.path.endsWith("/export")) {
    action = "export";
  } else if (route.path.endsWith("/restore")) {
    action = "restore";
  } else if (route.method === "GET") {
    action = params.length > 0 ? "get" : "list";
  } else if (route.method === "POST") {
    action = "create";
  } else if (route.method === "PUT" || route.method === "PATCH") {
    action = "update";
  } else if (route.method === "DELETE") {
    action = "delete";
  }

  const suffix = params.length > 0 ? ["by", ...params] : [];
  const base = camelCase(action, resource.name, ...suffix);
  let candidate = base;
  let index = 2;
  while (usedOperationIds.has(candidate)) {
    candidate = `${base}${index}`;
    index += 1;
  }
  usedOperationIds.add(candidate);
  return candidate;
}

function openapiPath(routePath) {
  return routePath.replace(/:([A-Za-z0-9_]+)/g, "{$1}");
}

function pathParams(routePath) {
  return Array.from(routePath.matchAll(/:([A-Za-z0-9_]+)/g)).map((match) => match[1]);
}

function pathParameter(name) {
  return {
    name,
    in: "path",
    required: true,
    schema: { type: "string", minLength: 1 },
  };
}

function fieldDescription(field) {
  const zh = field.label?.zh ? `zh: ${field.label.zh}` : "";
  const en = field.label?.en ? `en: ${field.label.en}` : "";
  return [en, zh].filter(Boolean).join("; ");
}

function fieldSchema(field) {
  const base = {
    description: fieldDescription(field),
    "x-platform-field-type": field.type,
    "x-platform-field-source": field.source ?? "values",
    "x-platform-label": field.label,
    "x-platform-searchable": field.search === true || field.searchable === true,
    "x-platform-filterable": field.filter === true || field.filterable === true,
    "x-platform-sortable": field.sort === true || field.sortable === true,
    "x-platform-localizable": field.localize === true || field.localizable === true,
    "x-platform-sensitivity": field.sensitivity ?? "public",
    "x-platform-storage-mode": field.storageMode ?? "plain",
    "x-platform-response-mode": field.responseMode ?? "full",
    "x-platform-export-mode": field.exportMode ?? "full",
    ...(field.protection ? { "x-platform-protection": field.protection } : {}),
    ...(field.masking ? { "x-platform-masking": field.masking } : {}),
    ...(field.reveal ? { "x-platform-reveal": field.reveal } : {}),
    ...(field.storageMode === "encrypted"
      ? { "x-platform-query-operators": field.protection?.blindIndexNamespace ? ["="] : [] }
      : {}),
    ...(field.relation ? { "x-platform-relation": field.relation } : {}),
  };

  if (field.type === "number") {
    return { ...base, type: "number" };
  }
  if (field.type === "switch") {
    return { ...base, type: "boolean" };
  }
  if (field.type === "multiselect") {
    return {
      ...base,
      type: "array",
      items: { type: "string" },
      uniqueItems: true,
    };
  }

  const schema = { ...base, type: "string" };
  if (field.type === "datetime") {
    schema.format = "date-time";
  }
  if (field.type === "color") {
    schema.pattern = "^#([0-9A-Fa-f]{6}|[0-9A-Fa-f]{3})$";
  }
  if (Number.isInteger(field.validation?.minLength)) {
    schema.minLength = field.validation.minLength;
  }
  if (Number.isInteger(field.validation?.maxLength)) {
    schema.maxLength = field.validation.maxLength;
  }
  if (typeof field.validation?.pattern === "string" && field.validation.pattern) {
    schema.pattern = field.validation.pattern;
  }
  if ((field.type === "select" || field.type === "text") && Array.isArray(field.options)) {
    schema.enum = field.options.map((option) => (typeof option === "string" ? option : option.value)).filter(Boolean);
  }
  return schema;
}

function resourceFieldMap(resource) {
  const fields = resource.schema?.fields ?? [];
  return new Map(fields.map((field) => [field.key, field]));
}

function resourceRecordSchema(resource) {
  const properties = {
    id: { type: "string" },
    code: { type: "string" },
    name: { type: "string" },
    status: { type: "string" },
    description: { type: "string" },
    updatedAt: { type: "string", format: "date-time" },
    values: {
      type: "object",
      additionalProperties: { type: "string" },
      description: "Extensible values for plugin or business fields that have not been promoted to first-class columns.",
    },
  };

  for (const field of resource.schema?.fields ?? []) {
    properties[field.key] = fieldSchema(field);
  }

  return {
    type: "object",
    required: ["id", "name", "status", "updatedAt"],
    additionalProperties: false,
    properties,
    "x-platform-resource": resource.name,
    "x-platform-codegen-mode": resource.codegen?.mode ?? "unknown",
    ...(resource.schema?.protection ? { "x-platform-protection": resource.schema.protection } : {}),
  };
}

function resourceInputSchema(resource) {
  const fieldMap = resourceFieldMap(resource);
  const formKeys = resource.schema?.form ?? [];
  const properties = {};
  const required = [];

  for (const key of formKeys) {
    const field = fieldMap.get(key);
    if (!field || field.readOnly) continue;
    properties[key] = fieldSchema(field);
    if (field.required) {
      required.push(key);
    }
  }

  return {
    type: "object",
    additionalProperties: false,
    properties,
    ...(required.length > 0 ? { required } : {}),
    "x-platform-resource": resource.name,
  };
}

function resourceListDataSchema(resource) {
  const schemaName = `${pascalCase(resource.name)}Record`;
  return {
    type: "object",
    required: ["resource", "items"],
    properties: {
      resource: { type: "string", const: resource.name },
      items: {
        type: "array",
        items: { $ref: `#/components/schemas/${schemaName}` },
      },
      total: { type: "integer", minimum: 0 },
      page: { type: "integer", minimum: 1 },
      pageSize: { type: "integer", minimum: 1, maximum: 500 },
    },
  };
}

function resourceMutationDataSchema(resource) {
  const schemaName = `${pascalCase(resource.name)}Record`;
  return {
    type: "object",
    required: ["resource", "record"],
    properties: {
      resource: { type: "string", const: resource.name },
      record: { $ref: `#/components/schemas/${schemaName}` },
    },
  };
}

function apiResponse(schema) {
  return {
    type: "object",
    properties: {
      data: schema,
      error: { $ref: "#/components/schemas/ErrorBody" },
    },
  };
}

function successResponse(description, schema) {
  return {
    description,
    content: {
      "application/json": {
        schema,
      },
    },
  };
}

function errorResponses() {
  return {
    "400": { $ref: "#/components/responses/BadRequest" },
    "401": { $ref: "#/components/responses/Unauthorized" },
    "403": { $ref: "#/components/responses/Forbidden" },
    "404": { $ref: "#/components/responses/NotFound" },
    "500": { $ref: "#/components/responses/InternalError" },
  };
}

function publicAuthErrorResponses() {
  return {
    "400": { $ref: "#/components/responses/BadRequest" },
    "401": { $ref: "#/components/responses/Unauthorized" },
    "404": { $ref: "#/components/responses/NotFound" },
    "500": { $ref: "#/components/responses/InternalError" },
    "501": { $ref: "#/components/responses/NotImplemented" },
    "502": { $ref: "#/components/responses/BadGateway" },
  };
}

function sensitiveRevealErrorResponses({ conflict = false, expired = false, rateLimited = false, upstream = false, verificationFailed = false } = {}) {
  return {
    "400": { $ref: "#/components/responses/BadRequest" },
    "401": { $ref: "#/components/responses/Unauthorized" },
    "403": { $ref: "#/components/responses/Forbidden" },
    "404": { $ref: "#/components/responses/NotFound" },
    ...(conflict ? { "409": { $ref: "#/components/responses/Conflict" } } : {}),
    ...(expired ? { "410": { $ref: "#/components/responses/Gone" } } : {}),
    ...(verificationFailed ? { "422": { $ref: "#/components/responses/UnprocessableEntity" } } : {}),
    ...(rateLimited ? { "429": { $ref: "#/components/responses/TooManyRequests" } } : {}),
    "500": { $ref: "#/components/responses/InternalError" },
    ...(upstream ? { "502": { $ref: "#/components/responses/BadGateway" } } : {}),
    "503": { $ref: "#/components/responses/ServiceUnavailable" },
  };
}

function sensitiveRevealRequestBody(schemaName) {
  return {
    required: true,
    content: {
      "application/json": {
        schema: { $ref: `#/components/schemas/${schemaName}` },
      },
    },
  };
}

function sensitiveRevealOperation({ operationId, summary, schemaName, status = "200", requestSchema, challenge = false, errors }) {
  return {
    tags: ["sensitive-reveal"],
    operationId,
    summary,
    description:
      "Reveals one manifest-declared protected field. Authorization, purpose, factor policy and copy behavior come from the field x-platform-reveal contract.",
    security: [{ bearerAuth: [] }],
    parameters: ["resource", "id", "field", ...(challenge ? ["challenge"] : [])].map(pathParameter),
    ...(requestSchema ? { requestBody: sensitiveRevealRequestBody(requestSchema) } : {}),
    responses: {
      [status]: successResponse("Successful sensitive field reveal response", apiResponse({ $ref: `#/components/schemas/${schemaName}` })),
      ...sensitiveRevealErrorResponses(errors),
    },
    "x-platform-permission-source": "field.reveal.permission",
  };
}

function queryRequestSchema(resource) {
  const searchable = resource.schema?.search ?? [];
  const filterable = resource.schema?.filter ?? [];
  const sortable = resource.schema?.sort ?? [];
  return {
    allOf: [{ $ref: "#/components/schemas/AdminQueryRequest" }],
    "x-platform-resource": resource.name,
    "x-platform-allowed-fields": Array.from(new Set([...searchable, ...filterable])).sort(),
    "x-platform-search-fields": searchable,
    "x-platform-filter-fields": filterable,
    "x-platform-sort-fields": sortable,
    "x-platform-localized-fields": resource.schema?.localizedFields ?? [],
  };
}

function operationRequestBody(resource, route) {
  if (route.path.endsWith("/queries") || route.path.endsWith("/query")) {
    return {
      required: true,
      content: {
        "application/json": {
          schema: queryRequestSchema(resource),
        },
      },
    };
  }
  if (route.method === "POST" || route.method === "PUT" || route.method === "PATCH") {
    if (route.path.endsWith("/reject")) {
      return {
        required: true,
        content: {
          "application/json": {
            schema: { $ref: "#/components/schemas/PolicyReviewRejectRequest" },
          },
        },
      };
    }
    if (route.path.endsWith("/apply") || route.path.endsWith("/request") || route.path.endsWith("/approve") || route.path.endsWith("/restore")) {
      return undefined;
    }
    return {
      required: true,
      content: {
        "application/json": {
          schema: { $ref: `#/components/schemas/${pascalCase(resource.name)}Input` },
        },
      },
    };
  }
  return undefined;
}

function operationSuccessSchema(resource, route) {
  if (route.path === "/api/openapi.json") {
    return { type: "object", additionalProperties: true };
  }
  if ((resource.codegen?.mode ?? "unknown") === "custom" && !route.path.endsWith("/queries")) {
    return apiResponse({ $ref: "#/components/schemas/CustomOperationData" });
  }
  if (route.method === "DELETE") {
    return apiResponse({ $ref: "#/components/schemas/DeleteResult" });
  }
  if (route.path.endsWith("/export") && resource.name === "policy-reviews") {
    return apiResponse({ $ref: "#/components/schemas/PolicyReviewExportData" });
  }
  if (route.path.endsWith("/queries") || route.path.endsWith("/query") || route.method === "GET") {
    return apiResponse(resourceListDataSchema(resource));
  }
  return apiResponse(resourceMutationDataSchema(resource));
}

function operation(resource, route) {
  const parameters = pathParams(route.path).map(pathParameter);
  if (route.path.endsWith("/export") && resource.name === "policy-reviews") {
    parameters.push({
      name: "watermark",
      in: "query",
      required: false,
      description: "Apply branding and export provenance watermark metadata to the JSON evidence package.",
      schema: { type: "boolean", default: false },
    });
  }
  const requestBody = operationRequestBody(resource, route);
  const op = {
    tags: [resource.name],
    operationId: uniqueOperationId(resource, route),
    summary: `${route.method} ${localized(resource.label)}`,
    description: [
      `Resource: ${resource.name}`,
      `Permission: ${route.permission}`,
      route.auditAction ? `Audit action: ${route.auditAction}` : "",
    ]
      .filter(Boolean)
      .join("\n"),
    security: [{ bearerAuth: [] }],
    parameters,
    responses: {
      "200": successResponse("Successful response", operationSuccessSchema(resource, route)),
      ...errorResponses(),
      ...(route.path.endsWith("/restore") ? { "409": { $ref: "#/components/responses/Conflict" } } : {}),
    },
    "x-platform-resource": resource.name,
    "x-platform-resource-code": resource.code,
    "x-platform-permission": route.permission,
    "x-platform-codegen-mode": resource.codegen?.mode ?? "unknown",
  };
  if (route.auditAction) {
    op["x-platform-audit-action"] = route.auditAction;
  }
  if (requestBody) {
    op.requestBody = requestBody;
  }
  return op;
}

function addPath(paths, resource, route) {
  const routePath = openapiPath(route.path);
  const method = route.method.toLowerCase();
  paths[routePath] = paths[routePath] ?? {};
  paths[routePath][method] = operation(resource, route);
}

function serviceObjectSchemaPrefix(definition) {
  return `${definition.codegenName}V${definition.version.replaceAll(".", "_")}`;
}

function serviceObjectValueSchema(field) {
  if (field.type === "string-set") {
    if (!field.maxLength) return { $ref: "#/components/schemas/AdminServiceObjectStringSet" };
    return {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: {
        type: "string",
        ...(field.maxLength ? { maxLength: field.maxLength } : {}),
      },
    };
  }
  if (field.type === "role-remediations") {
    const codeProperty = {
      type: "string",
      ...(field.maxLength ? { maxLength: field.maxLength } : {}),
    };
    return {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: {
        oneOf: [
          {
            type: "object",
            required: ["userCode", "roleCode", "action"],
            additionalProperties: false,
            properties: {
              userCode: codeProperty,
              roleCode: codeProperty,
              action: { type: "string", const: "remove-role" },
            },
          },
          {
            type: "object",
            required: ["userCode", "roleCode", "action", "replacementRoleCode"],
            additionalProperties: false,
            properties: {
              userCode: codeProperty,
              roleCode: codeProperty,
              action: { type: "string", const: "replace-role" },
              replacementRoleCode: codeProperty,
            },
          },
        ],
      },
    };
  }
  if (field.type === "menu-definition") {
    return { $ref: "#/components/schemas/AdminServiceObjectMenuDefinition" };
  }
  const schema = { type: field.type };
  if (field.maxLength) schema.maxLength = field.maxLength;
  if (field.minimum !== undefined) schema.minimum = field.minimum;
  if (field.maximum !== undefined) schema.maximum = field.maximum;
  return schema;
}

function serviceObjectObjectSchema(fields) {
  const required = fields.filter((field) => field.required !== false).map((field) => field.name);
  return {
    type: "object",
    additionalProperties: false,
    properties: Object.fromEntries(fields.map((field) => [field.name, serviceObjectValueSchema(field)])),
    ...(required.length > 0 ? { required } : {}),
  };
}

function serviceObjectArgumentSchema(definition) {
  return serviceObjectObjectSchema(definition.arguments.map((argument) => ({ ...argument, required: argument.required === true })));
}

function serviceObjectQuerySchemas(definition) {
  const prefix = serviceObjectSchemaPrefix(definition);
  const argumentsName = `${prefix}Arguments`;
  const sortName = `${prefix}Sort`;
  const itemName = `${prefix}Item`;
  const dataName = `${prefix}QueryData`;
  const requestName = `${prefix}QueryRequest`;
  const required = ["queryId", "version"];
  if (definition.arguments.some((argument) => argument.required)) required.push("arguments");
  const properties = {
    queryId: { type: "string", const: definition.id },
    version: { type: "string", const: definition.version },
    arguments: { $ref: `#/components/schemas/${argumentsName}` },
    pagination: {
      type: "object",
      additionalProperties: false,
      properties: {
        page: { type: "integer", minimum: 1 },
        pageSize: { type: "integer", minimum: 1, maximum: definition.maxPageSize },
      },
    },
  };
  if (definition.allowedSort.length > 0) {
    properties.sort = {
      type: "array",
      maxItems: definition.allowedSort.length,
      items: { $ref: `#/components/schemas/${sortName}` },
    };
  }

  const dataProperties = {
    items: { type: "array", items: { $ref: `#/components/schemas/${itemName}` } },
    page: { type: "integer", minimum: 1 },
    pageSize: { type: "integer", minimum: 1, maximum: definition.maxPageSize },
  };
  if (definition.exposeTotal) {
    dataProperties.total = { type: "integer", minimum: 0 };
  }

  const generated = {
    [argumentsName]: serviceObjectArgumentSchema(definition),
    [itemName]: serviceObjectObjectSchema(definition.result),
    [dataName]: {
      type: "object",
      required: ["items", "page", "pageSize"],
      additionalProperties: false,
      properties: dataProperties,
    },
    [requestName]: {
      type: "object",
      required,
      additionalProperties: false,
      properties,
      "x-platform-definition": {
        resource: definition.resource,
        permission: definition.permission,
        action: definition.action,
        additionalPermissions: definition.additionalPermissions ?? [],
        tenantMode: definition.tenantMode,
        dataScope: definition.dataScope,
        cost: definition.cost,
        timeoutMs: definition.timeoutMs,
      },
    },
  };
  if (definition.allowedSort.length > 0) {
    generated[sortName] = {
      type: "object",
      required: ["name", "order"],
      additionalProperties: false,
      properties: {
        name: { type: "string", enum: definition.allowedSort },
        order: { type: "string", enum: ["asc", "desc"] },
      },
    };
  }
  return generated;
}

function serviceObjectCommandSchemas(definition) {
  const prefix = serviceObjectSchemaPrefix(definition);
  const argumentsName = `${prefix}Arguments`;
  const valuesName = `${prefix}Values`;
  const dataName = `${prefix}CommandData`;
  const requestName = `${prefix}CommandRequest`;
  const required = ["commandId", "version"];
  if (definition.arguments.some((argument) => argument.required)) required.push("arguments");
  if (definition.idempotency === "required-key") required.push("idempotencyKey");

  return {
    [argumentsName]: serviceObjectArgumentSchema(definition),
    [valuesName]: serviceObjectObjectSchema(definition.result),
    [dataName]: {
      type: "object",
      required: ["values"],
      additionalProperties: false,
      properties: { values: { $ref: `#/components/schemas/${valuesName}` } },
    },
    [requestName]: {
      type: "object",
      required,
      additionalProperties: false,
      properties: {
        commandId: { type: "string", const: definition.id },
        version: { type: "string", const: definition.version },
        arguments: { $ref: `#/components/schemas/${argumentsName}` },
        idempotencyKey: { type: "string", minLength: 1, maxLength: 128 },
      },
      "x-platform-definition": {
        resource: definition.resource,
        permission: definition.permission,
        action: definition.action,
        additionalPermissions: definition.additionalPermissions ?? [],
        tenantMode: definition.tenantMode,
        dataScope: definition.dataScope,
        cost: definition.cost,
        timeoutMs: definition.timeoutMs,
        idempotency: definition.idempotency,
        maxAffectedRows: definition.maxAffectedRows,
        ...(definition.operationPhase ? { operationPhase: definition.operationPhase } : {}),
      },
    },
  };
}

function serviceObjectUnionSchema(kind, definitions, suffix) {
  const selector = kind === "query" ? "queryId" : "commandId";
  const schema = {
    oneOf: definitions.map((definition) => ({
      $ref: `#/components/schemas/${serviceObjectSchemaPrefix(definition)}${suffix}`,
    })),
    "x-platform-discriminator": {
      properties: [selector, "version"],
      values: definitions.map((definition) => ({ [selector]: definition.id, version: definition.version })),
    },
  };
  if (new Set(definitions.map((definition) => definition.id)).size === definitions.length) {
    schema.discriminator = {
      propertyName: selector,
      mapping: Object.fromEntries(
        definitions.map((definition) => [
          definition.id,
          `#/components/schemas/${serviceObjectSchemaPrefix(definition)}${suffix}`,
        ]),
      ),
    };
  }
  return schema;
}

function serviceObjectDataUnionSchema(definitions, suffix) {
  return {
    oneOf: definitions.map((definition) => ({
      $ref: `#/components/schemas/${serviceObjectSchemaPrefix(definition)}${suffix}`,
    })),
  };
}

function serviceObjectSchemas() {
  return {
    AdminServiceObjectStringSet: {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: { type: "string" },
    },
    AdminServiceObjectMenuParameter: {
      oneOf: [
        {
          type: "object",
          required: ["key", "type", "value"],
          additionalProperties: false,
          properties: { key: { type: "string" }, type: { type: "string", const: "string" }, value: { type: "string" } },
        },
        {
          type: "object",
          required: ["key", "type", "value"],
          additionalProperties: false,
          properties: { key: { type: "string" }, type: { type: "string", const: "number" }, value: { type: "number" } },
        },
        {
          type: "object",
          required: ["key", "type", "value"],
          additionalProperties: false,
          properties: { key: { type: "string" }, type: { type: "string", const: "boolean" }, value: { type: "boolean" } },
        },
      ],
    },
    AdminServiceObjectMenuNode: {
      type: "object",
      required: [
        "code", "parentCode", "nodeType", "titleZh", "titleEn", "descriptionZh", "descriptionEn", "status", "icon", "sortOrder",
        "route", "componentKey", "resourceCode", "external", "externalUrl", "openMode", "parameters", "cacheEnabled", "hidden",
        "activeMenuCode", "breadcrumbVisible",
      ],
      additionalProperties: false,
      properties: {
        code: { type: "string" },
        parentCode: { type: "string" },
        nodeType: { type: "string", enum: ["directory", "page"] },
        titleZh: { type: "string" },
        titleEn: { type: "string" },
        descriptionZh: { type: "string" },
        descriptionEn: { type: "string" },
        status: { type: "string", enum: ["enabled", "disabled"] },
        icon: { type: "string" },
        sortOrder: { type: "integer" },
        route: { oneOf: [{ const: "" }, { type: "string", pattern: "^/(?!/)[^{}*:]*$" }] },
        componentKey: { type: "string" },
        resourceCode: { type: "string" },
        external: { type: "boolean" },
        externalUrl: { oneOf: [{ const: "" }, { type: "string", format: "uri", pattern: "^https://" }] },
        openMode: { type: "string", enum: ["", "same-tab", "new-tab"] },
        parameters: {
          type: "array",
          maxItems: 32,
          items: { $ref: "#/components/schemas/AdminServiceObjectMenuParameter" },
        },
        cacheEnabled: { type: "boolean" },
        hidden: { type: "boolean" },
        activeMenuCode: { type: "string" },
        breadcrumbVisible: { type: "boolean" },
      },
    },
    AdminServiceObjectPageButton: {
      type: "object",
      required: ["menuCode", "buttonKey", "labelZh", "labelEn", "action", "sortOrder", "status", "permissionCode"],
      additionalProperties: false,
      properties: {
        menuCode: { type: "string" },
        buttonKey: { type: "string" },
        labelZh: { type: "string" },
        labelEn: { type: "string" },
        action: { type: "string" },
        sortOrder: { type: "integer" },
        status: { type: "string", enum: ["enabled", "disabled"] },
        permissionCode: { type: "string" },
      },
    },
    AdminServiceObjectMenuDefinition: {
      type: "object",
      required: ["id", "name", "description", "updatedAt", "node", "buttons"],
      additionalProperties: false,
      properties: {
        id: { type: "string" },
        name: { type: "string" },
        description: { type: "string" },
        updatedAt: { type: "string" },
        node: { $ref: "#/components/schemas/AdminServiceObjectMenuNode" },
        buttons: { type: "array", items: { $ref: "#/components/schemas/AdminServiceObjectPageButton" } },
      },
    },
    ...Object.assign({}, ...adminServiceObjectDefinitions.queries.map(serviceObjectQuerySchemas)),
    ...Object.assign({}, ...adminServiceObjectDefinitions.commands.map(serviceObjectCommandSchemas)),
    AdminServiceObjectQueryRequest: serviceObjectUnionSchema("query", adminServiceObjectDefinitions.queries, "QueryRequest"),
    AdminServiceObjectCommandRequest: serviceObjectUnionSchema("command", adminServiceObjectDefinitions.commands, "CommandRequest"),
    AdminServiceObjectQueryData: serviceObjectDataUnionSchema(adminServiceObjectDefinitions.queries, "QueryData"),
    AdminServiceObjectCommandData: serviceObjectDataUnionSchema(adminServiceObjectDefinitions.commands, "CommandData"),
  };
}

function schemas() {
  const generated = {
    ErrorBody: {
      type: "object",
      required: ["code", "message"],
      properties: {
        code: { type: "string" },
        message: { type: "string" },
      },
    },
    DeleteResult: {
      type: "object",
      required: ["deleted", "resource"],
      properties: {
        deleted: { type: "boolean" },
        resource: { type: "string" },
      },
    },
    CustomOperationData: {
      type: "object",
      additionalProperties: true,
      description:
        "Custom handler response. Use the route metadata, resource schema and handler implementation as the source of truth until a resource-specific response contract is declared.",
    },
    AdminQueryRequest: {
      type: "object",
      additionalProperties: false,
      description:
        "Structured query payload compiled from safe UI filters or SQL-like input. Field names must come from the resource schema whitelist; raw input must never be concatenated into SQL.",
      properties: {
        keywords: {
          type: "array",
          items: { type: "string" },
        },
        conditions: {
          type: "array",
          items: { $ref: "#/components/schemas/AdminQueryCondition" },
        },
        sort: {
          type: "array",
          items: { $ref: "#/components/schemas/AdminSortClause" },
        },
        page: { type: "integer", minimum: 1, default: 1 },
        pageSize: { type: "integer", minimum: 1, maximum: 100, default: 10 },
      },
    },
    AdminQueryCondition: {
      type: "object",
      required: ["field", "operator", "value"],
      additionalProperties: false,
      properties: {
        field: { type: "string" },
        operator: {
          type: "string",
          enum: ["contains", "eq", "ne", "gt", "gte", "lt", "lte", "in", "between"],
        },
        value: {
          oneOf: [
            { type: "string" },
            { type: "number" },
            { type: "boolean" },
            { type: "array", items: { type: ["string", "number", "boolean"] } },
          ],
        },
      },
    },
    AdminSortClause: {
      type: "object",
      required: ["field", "order"],
      additionalProperties: false,
      properties: {
        field: { type: "string" },
        order: { type: "string", enum: ["asc", "desc"] },
      },
    },
    ...serviceObjectSchemas(),
    AdminAuthProviderStartRequest: {
      type: "object",
      required: ["codeChallenge"],
      additionalProperties: false,
      properties: {
        codeChallenge: {
          type: "string",
          minLength: 43,
          maxLength: 43,
          pattern: "^[A-Za-z0-9_-]{43}$",
          description: "PKCE S256 code challenge generated by the Admin client.",
        },
      },
    },
    AdminAuthProviderStartData: {
      type: "object",
      required: ["authorizationUrl", "state", "expiresAt"],
      additionalProperties: false,
      properties: {
        authorizationUrl: { type: "string", format: "uri" },
        state: { type: "string", minLength: 1 },
        expiresAt: { type: "string", format: "date-time" },
      },
    },
    AdminAuthLoginRequest: {
      type: "object",
      required: ["provider"],
      additionalProperties: false,
      properties: {
        provider: { type: "string", minLength: 1 },
        username: { type: "string" },
        code: { type: "string", writeOnly: true },
        state: { type: "string", writeOnly: true },
        codeVerifier: { type: "string", writeOnly: true },
      },
    },
    AdminAuthLoginData: {
      type: "object",
      required: ["token", "expiresAt", "principal"],
      additionalProperties: false,
      properties: {
        token: { type: "string" },
        expiresAt: { type: "string", format: "date-time" },
        principal: { type: "object", additionalProperties: true },
      },
    },
    AdminSensitiveRevealLocalizedText: {
      type: "object",
      required: ["zh", "en"],
      additionalProperties: false,
      properties: {
        zh: { type: "string" },
        en: { type: "string" },
      },
    },
    AdminSensitiveRevealPurpose: {
      type: "object",
      required: ["code", "label"],
      additionalProperties: false,
      properties: {
        code: { type: "string", minLength: 1 },
        label: { $ref: "#/components/schemas/AdminSensitiveRevealLocalizedText" },
      },
    },
    AdminSensitiveRevealProvider: {
      type: "object",
      required: ["id", "title"],
      additionalProperties: false,
      properties: {
        id: { type: "string", minLength: 1 },
        title: { $ref: "#/components/schemas/AdminSensitiveRevealLocalizedText" },
      },
    },
    AdminSensitiveRevealFactor: {
      type: "object",
      required: ["type", "available"],
      additionalProperties: false,
      properties: {
        type: { type: "string", enum: ["oidc-reauth-v1", "admin-sms-otp-v1"] },
        available: { type: "boolean" },
        providers: {
          type: "array",
          items: { $ref: "#/components/schemas/AdminSensitiveRevealProvider" },
        },
        maskedDestination: { type: "string" },
      },
    },
    AdminSensitiveRevealPolicyData: {
      type: "object",
      required: ["policyId", "mode", "purposes", "factors", "challengeTtlSeconds", "grantTtlSeconds", "copyAllowed"],
      additionalProperties: false,
      properties: {
        policyId: { type: "string", minLength: 1 },
        mode: { type: "string", enum: ["anyOf", "allOf"] },
        purposes: {
          type: "array",
          items: { $ref: "#/components/schemas/AdminSensitiveRevealPurpose" },
        },
        factors: {
          type: "array",
          items: { $ref: "#/components/schemas/AdminSensitiveRevealFactor" },
        },
        challengeTtlSeconds: { type: "integer", minimum: 1 },
        grantTtlSeconds: { type: "integer", minimum: 1 },
        copyAllowed: { type: "boolean" },
      },
    },
    AdminSensitiveRevealChallengeRequest: {
      type: "object",
      required: ["purpose"],
      additionalProperties: false,
      properties: {
        purpose: { type: "string", minLength: 1 },
      },
    },
    AdminSensitiveRevealChallengeData: {
      type: "object",
      required: ["challengeId", "challengeToken", "policyId", "mode", "factors", "expiresAt"],
      additionalProperties: false,
      properties: {
        challengeId: { type: "string", minLength: 1 },
        challengeToken: { type: "string", minLength: 1 },
        policyId: { type: "string", minLength: 1 },
        mode: { type: "string", enum: ["anyOf", "allOf"] },
        factors: {
          type: "array",
          items: { type: "string", enum: ["oidc-reauth-v1", "admin-sms-otp-v1"] },
        },
        expiresAt: { type: "string", format: "date-time" },
      },
    },
    AdminSensitiveRevealOIDCStartRequest: {
      type: "object",
      required: ["challengeToken", "purpose", "provider", "codeChallenge"],
      additionalProperties: false,
      properties: {
        challengeToken: { type: "string", minLength: 1, writeOnly: true },
        purpose: { type: "string", minLength: 1 },
        provider: { type: "string", minLength: 1 },
        codeChallenge: {
          type: "string",
          minLength: 43,
          maxLength: 43,
          pattern: "^[A-Za-z0-9_-]{43}$",
        },
      },
    },
    AdminSensitiveRevealOIDCStartData: {
      type: "object",
      required: ["challengeId", "transactionToken", "authorizationUrl", "state", "expiresAt"],
      additionalProperties: false,
      properties: {
        challengeId: { type: "string", minLength: 1 },
        transactionToken: { type: "string", minLength: 1 },
        authorizationUrl: { type: "string", format: "uri" },
        state: { type: "string", minLength: 1 },
        expiresAt: { type: "string", format: "date-time" },
      },
    },
    AdminSensitiveRevealOIDCCompleteRequest: {
      type: "object",
      required: ["challengeToken", "purpose", "transactionToken", "provider", "code", "state", "codeVerifier"],
      additionalProperties: false,
      properties: {
        challengeToken: { type: "string", minLength: 1, writeOnly: true },
        purpose: { type: "string", minLength: 1 },
        transactionToken: { type: "string", minLength: 1, writeOnly: true },
        provider: { type: "string", minLength: 1 },
        code: { type: "string", minLength: 1, writeOnly: true },
        state: { type: "string", minLength: 1, writeOnly: true },
        codeVerifier: { type: "string", minLength: 1, writeOnly: true },
      },
    },
    AdminSensitiveRevealSMSStartRequest: {
      type: "object",
      required: ["challengeToken", "purpose"],
      additionalProperties: false,
      properties: {
        challengeToken: { type: "string", minLength: 1, writeOnly: true },
        purpose: { type: "string", minLength: 1 },
      },
    },
    AdminSensitiveRevealSMSStartData: {
      type: "object",
      required: ["challengeId", "transactionToken", "maskedPhone", "expiresAt"],
      additionalProperties: false,
      properties: {
        challengeId: { type: "string", minLength: 1 },
        transactionToken: { type: "string", minLength: 1 },
        maskedPhone: { type: "string" },
        expiresAt: { type: "string", format: "date-time" },
        debugCode: { type: "string" },
      },
    },
    AdminSensitiveRevealSMSCompleteRequest: {
      type: "object",
      required: ["challengeToken", "purpose", "transactionToken", "code"],
      additionalProperties: false,
      properties: {
        challengeToken: { type: "string", minLength: 1, writeOnly: true },
        purpose: { type: "string", minLength: 1 },
        transactionToken: { type: "string", minLength: 1, writeOnly: true },
        code: { type: "string", minLength: 1, writeOnly: true },
      },
    },
    AdminSensitiveRevealFactorCompleteData: {
      type: "object",
      required: ["challengeId", "policySatisfied"],
      additionalProperties: false,
      properties: {
        challengeId: { type: "string", minLength: 1 },
        policySatisfied: { type: "boolean" },
        grantToken: { type: "string", minLength: 1 },
        grantExpiresAt: { type: "string", format: "date-time" },
      },
    },
    AdminSensitiveRevealRequest: {
      type: "object",
      required: ["purpose", "grantToken"],
      additionalProperties: false,
      properties: {
        purpose: { type: "string", minLength: 1 },
        grantToken: { type: "string", minLength: 1, writeOnly: true },
      },
    },
    AdminSensitiveRevealValueData: {
      type: "object",
      required: ["field", "value", "copyAllowed"],
      additionalProperties: false,
      properties: {
        field: { type: "string", minLength: 1 },
        value: { type: "string" },
        copyAllowed: { type: "boolean" },
      },
    },
  };

  for (const resource of resources) {
    generated[`${pascalCase(resource.name)}Record`] = resourceRecordSchema(resource);
    generated[`${pascalCase(resource.name)}Input`] = resourceInputSchema(resource);
  }

  if (resources.some((resource) => resource.name === "policy-reviews")) {
    generated.PolicyReviewRejectRequest = {
      type: "object",
      required: ["reason"],
      additionalProperties: false,
      properties: {
        reason: {
          type: "string",
          minLength: 1,
          maxLength: 500,
          description: "Localized or operator-entered rejection reason recorded on the policy review ledger.",
        },
      },
    };
    generated.PolicyReviewExportData = {
      type: "object",
      required: ["exportedBy", "exportedAt", "watermark", "reviews", "audits"],
      additionalProperties: false,
      properties: {
        exportedBy: { type: "string" },
        exportedAt: { type: "string", format: "date-time" },
        watermark: { $ref: "#/components/schemas/PolicyReviewExportWatermark" },
        reviews: {
          type: "array",
          items: { $ref: "#/components/schemas/PolicyReviewsRecord" },
        },
        audits: {
          type: "array",
          items: { $ref: "#/components/schemas/AuditLogsRecord" },
        },
      },
    };
    generated.PolicyReviewExportWatermark = {
      type: "object",
      required: ["applied", "product", "exportedBy", "exportedAt"],
      additionalProperties: false,
      properties: {
        applied: { type: "boolean" },
        product: { type: "string" },
        exportedBy: { type: "string" },
        exportedAt: { type: "string", format: "date-time" },
      },
    };
  }

  return generated;
}

const paths = {};
for (const resource of resources) {
  for (const route of resource.routes ?? []) {
    addPath(paths, resource, route);
  }
}

const sensitiveRevealFieldPath = "/api/admin/resources/{resource}/{id}/fields/{field}";
const sensitiveRevealChallengePath = `${sensitiveRevealFieldPath}/reveal/challenges/{challenge}`;

paths[`${sensitiveRevealFieldPath}/reveal-policy`] = {
  get: sensitiveRevealOperation({
    operationId: "getAdminSensitiveRevealPolicy",
    summary: "Get the step-up policy for one sensitive field",
    schemaName: "AdminSensitiveRevealPolicyData",
  }),
};

paths[`${sensitiveRevealFieldPath}/reveal/challenges`] = {
  post: sensitiveRevealOperation({
    operationId: "createAdminSensitiveRevealChallenge",
    summary: "Create a sensitive field reveal challenge",
    schemaName: "AdminSensitiveRevealChallengeData",
    status: "201",
    requestSchema: "AdminSensitiveRevealChallengeRequest",
    errors: { rateLimited: true },
  }),
};

paths[`${sensitiveRevealChallengePath}/factors/oidc/start`] = {
  post: sensitiveRevealOperation({
    operationId: "startAdminSensitiveRevealOIDC",
    summary: "Start OIDC reauthentication for a sensitive field reveal challenge",
    schemaName: "AdminSensitiveRevealOIDCStartData",
    status: "201",
    requestSchema: "AdminSensitiveRevealOIDCStartRequest",
    challenge: true,
    errors: { conflict: true, expired: true, rateLimited: true },
  }),
};

paths[`${sensitiveRevealChallengePath}/factors/oidc/complete`] = {
  post: sensitiveRevealOperation({
    operationId: "completeAdminSensitiveRevealOIDC",
    summary: "Complete OIDC reauthentication for a sensitive field reveal challenge",
    schemaName: "AdminSensitiveRevealFactorCompleteData",
    requestSchema: "AdminSensitiveRevealOIDCCompleteRequest",
    challenge: true,
    errors: { conflict: true, expired: true, rateLimited: true, upstream: true, verificationFailed: true },
  }),
};

paths[`${sensitiveRevealChallengePath}/factors/sms/start`] = {
  post: sensitiveRevealOperation({
    operationId: "startAdminSensitiveRevealSMS",
    summary: "Start SMS verification for a sensitive field reveal challenge",
    schemaName: "AdminSensitiveRevealSMSStartData",
    status: "201",
    requestSchema: "AdminSensitiveRevealSMSStartRequest",
    challenge: true,
    errors: { conflict: true, expired: true, rateLimited: true, upstream: true },
  }),
};

paths[`${sensitiveRevealChallengePath}/factors/sms/complete`] = {
  post: sensitiveRevealOperation({
    operationId: "completeAdminSensitiveRevealSMS",
    summary: "Complete SMS verification for a sensitive field reveal challenge",
    schemaName: "AdminSensitiveRevealFactorCompleteData",
    requestSchema: "AdminSensitiveRevealSMSCompleteRequest",
    challenge: true,
    errors: { conflict: true, expired: true, rateLimited: true, verificationFailed: true },
  }),
};

paths[`${sensitiveRevealFieldPath}/reveal`] = {
  post: sensitiveRevealOperation({
    operationId: "revealAdminSensitiveField",
    summary: "Consume a grant and reveal one sensitive field value",
    schemaName: "AdminSensitiveRevealValueData",
    requestSchema: "AdminSensitiveRevealRequest",
    errors: { conflict: true, expired: true, rateLimited: true },
  }),
};

paths["/api/auth/providers/{provider}/start"] = {
  post: {
    tags: ["auth"],
    operationId: "startAdminAuthProvider",
    summary: "Start an Admin identity provider transaction",
    security: [],
    parameters: [pathParameter("provider")],
    requestBody: {
      required: true,
      content: {
        "application/json": {
          schema: { $ref: "#/components/schemas/AdminAuthProviderStartRequest" },
        },
      },
    },
    responses: {
      "200": successResponse(
        "Admin identity provider transaction",
        apiResponse({ $ref: "#/components/schemas/AdminAuthProviderStartData" }),
      ),
      ...publicAuthErrorResponses(),
    },
  },
};

paths["/api/auth/login"] = {
  post: {
    tags: ["auth"],
    operationId: "adminAuthLogin",
    summary: "Exchange Admin credentials or an identity provider transaction",
    security: [],
    requestBody: {
      required: true,
      content: {
        "application/json": {
          schema: { $ref: "#/components/schemas/AdminAuthLoginRequest" },
        },
      },
    },
    responses: {
      "200": successResponse("Admin login", apiResponse({ $ref: "#/components/schemas/AdminAuthLoginData" })),
      ...publicAuthErrorResponses(),
    },
  },
};

paths["/api/admin/service-objects/query"] = {
  post: {
    tags: ["service-objects"],
    operationId: "executeAdminPersistedQuery",
    summary: "Execute a versioned server-side persisted QueryDefinition",
    security: [{ bearerAuth: [] }],
    requestBody: {
      required: true,
      content: { "application/json": { schema: { $ref: "#/components/schemas/AdminServiceObjectQueryRequest" } } },
    },
    responses: {
      "200": successResponse("Persisted query result", apiResponse({ $ref: "#/components/schemas/AdminServiceObjectQueryData" })),
      ...errorResponses(),
      "404": {
        ...platformErrorResponse("Service-object runtime unavailable or QueryDefinition not found"),
        "x-platform-error-codes": ["SERVICE_OBJECT_UNAVAILABLE"],
      },
      "422": { $ref: "#/components/responses/UnprocessableEntity" },
    },
    "x-platform-query-contract": "server-side-versioned-definition",
    "x-platform-runtime": "conditional",
  },
};

paths["/api/admin/service-objects/command"] = {
  post: {
    tags: ["service-objects"],
    operationId: "executeAdminCommandObject",
    summary: "Execute a versioned server-side CommandDefinition",
    security: [{ bearerAuth: [] }],
    requestBody: {
      required: true,
      content: { "application/json": { schema: { $ref: "#/components/schemas/AdminServiceObjectCommandRequest" } } },
    },
    responses: {
      "200": successResponse("Command result", apiResponse({ $ref: "#/components/schemas/AdminServiceObjectCommandData" })),
      ...errorResponses(),
      "404": {
        ...platformErrorResponse("Service-object runtime unavailable or CommandDefinition not found"),
        "x-platform-error-codes": ["SERVICE_OBJECT_UNAVAILABLE"],
      },
      "409": {
        ...platformErrorResponse("Command conflict, including idempotency-key reuse"),
        "x-platform-error-codes": ["SERVICE_OBJECT_IDEMPOTENCY_CONFLICT", "SERVICE_OBJECT_STATE_CONFLICT"],
      },
    },
    "x-platform-command-contract": "server-side-versioned-definition",
    "x-platform-runtime": "conditional",
  },
};

const openapi = {
  openapi: "3.1.0",
  info: {
    title: "Platform Admin API",
    version: contract.sourceVersion ?? "0.1.0",
    description:
      "Generated from the platform admin resource contract. Do not edit this file manually; update resources/admin-resources.json and regenerate.",
  },
  servers: [{ url: "/" }],
  tags: [
    { name: "auth", description: "Public Admin authentication endpoints." },
    { name: "sensitive-reveal", description: "Step-up verification and one-time sensitive field reveal endpoints." },
    { name: "service-objects", description: "Versioned server-side QueryDefinition and CommandDefinition execution." },
    ...resources.map((resource) => ({
      name: resource.name,
      description: `${resource.label?.en ?? resource.name}${resource.label?.zh ? ` / ${resource.label.zh}` : ""}`,
      "x-platform-group": resource.group,
    })),
  ],
  paths: Object.fromEntries(Object.entries(paths).sort(([left], [right]) => left.localeCompare(right))),
  components: {
    securitySchemes: {
      bearerAuth: {
        type: "http",
        scheme: "bearer",
        bearerFormat: "JWT",
      },
    },
    responses: {
      BadRequest: platformErrorResponse("Bad request"),
      Unauthorized: platformErrorResponse("Authentication required"),
      Forbidden: platformErrorResponse("Permission denied"),
      NotFound: platformErrorResponse("Resource not found"),
      Conflict: platformErrorResponse("Request conflicts with the current reveal state"),
      Gone: platformErrorResponse("Reveal challenge, factor or grant expired"),
      UnprocessableEntity: platformErrorResponse("Sensitive reveal verification failed"),
      TooManyRequests: platformErrorResponse("Sensitive reveal rate limit exceeded"),
      InternalError: platformErrorResponse("Internal server error"),
      NotImplemented: platformErrorResponse("Identity provider resolver not configured"),
      BadGateway: platformErrorResponse("Identity provider unavailable"),
      ServiceUnavailable: platformErrorResponse("Sensitive reveal runtime or protected value unavailable"),
    },
    schemas: { ...schemas(), ...platformErrorOpenAPISchemas(errorContract) },
  },
  "x-generated-by": "scripts/generate-admin-openapi.mjs",
  "x-source": path.relative(repoRoot, contractPath),
  "x-service-object-definition-source": adminServiceObjectDefinitions.source,
  "x-service-object-runtime-reference": adminServiceObjectDefinitions.runtimeSource,
  "x-service-object-runtime-sources": adminServiceObjectDefinitions.runtimeSources,
  "x-source-version": contract.sourceVersion,
  "x-source-updated-at": contract.updatedAt,
  "x-stack": contract.stack,
  "x-platform-plane": "admin",
  ...platformErrorRegistryExtensions(errorContract),
};

const output = `${JSON.stringify(openapi, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
}
