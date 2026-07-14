import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const contractPath = optionValue("--contract")
  ? path.resolve(repoRoot, optionValue("--contract"))
  : path.join(repoRoot, "resources", "generated", "admin-resource-contract.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "openapi.admin.json");

const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
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
    if (route.path.endsWith("/apply") || route.path.endsWith("/request") || route.path.endsWith("/approve")) {
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
      BadRequest: successResponse("Bad request", apiResponse({ nullable: true })),
      Unauthorized: successResponse("Authentication required", apiResponse({ nullable: true })),
      Forbidden: successResponse("Permission denied", apiResponse({ nullable: true })),
      NotFound: successResponse("Resource not found", apiResponse({ nullable: true })),
      Conflict: successResponse("Request conflicts with the current reveal state", apiResponse({ nullable: true })),
      Gone: successResponse("Reveal challenge, factor or grant expired", apiResponse({ nullable: true })),
      UnprocessableEntity: successResponse("Sensitive reveal verification failed", apiResponse({ nullable: true })),
      TooManyRequests: successResponse("Sensitive reveal rate limit exceeded", apiResponse({ nullable: true })),
      InternalError: successResponse("Internal server error", apiResponse({ nullable: true })),
      NotImplemented: successResponse("Identity provider resolver not configured", apiResponse({ nullable: true })),
      BadGateway: successResponse("Identity provider unavailable", apiResponse({ nullable: true })),
      ServiceUnavailable: successResponse("Sensitive reveal runtime or protected value unavailable", apiResponse({ nullable: true })),
    },
    schemas: schemas(),
  },
  "x-generated-by": "scripts/generate-admin-openapi.mjs",
  "x-source": path.relative(repoRoot, contractPath),
  "x-source-version": contract.sourceVersion,
  "x-source-updated-at": contract.updatedAt,
  "x-stack": contract.stack,
};

const output = `${JSON.stringify(openapi, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
}
