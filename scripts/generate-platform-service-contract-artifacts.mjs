import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  errorResponse,
  loadPlatformErrorContract,
  platformErrorOpenAPISchemas,
  platformErrorRegistryExtensions,
} from "./platform-error-contract.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

const contractPath = path.resolve(repoRoot, argValue("--contract", "resources/generated/platform-service-contract.json"));
const outputDir = path.resolve(repoRoot, argValue("--output-dir", "resources/generated"));
const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
const errorContract = loadPlatformErrorContract(repoRoot);

function sorted(values) {
  return [...values].sort((left, right) => String(left.id ?? left.name ?? left).localeCompare(String(right.id ?? right.name ?? right)));
}

function schemaName(ref) {
  return String(ref).replace(/^#\/schemas\//, "") || "Payload";
}

function canonicalJSON(value) {
  if (Array.isArray(value)) return value.map(canonicalJSON);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalJSON(value[key])]));
  }
  return value;
}

function setSchema(schemas, sources, name, schema, source) {
  if (!schemas.has(name)) {
    schemas.set(name, schema);
    sources.set(name, source);
    return;
  }
  if (JSON.stringify(canonicalJSON(schemas.get(name))) !== JSON.stringify(canonicalJSON(schema))) {
    throw new Error(`Schema definition collision for ${name}: ${sources.get(name)} conflicts with ${source}`);
  }
}

const forbiddenClientFields = sorted(contract.tenantContext?.forbiddenClientFields ?? []);

function adminResourceRecordSchema() {
  return {
    type: "object",
    properties: {
      id: { type: "string" },
      code: { type: "string" },
      name: { type: "string" },
      status: { type: "string" },
      description: { type: "string" },
      updatedAt: { type: "string" },
      values: { type: "object", additionalProperties: { type: "string" } },
      deletedAt: { type: "string" },
      deletedBy: { type: "string" },
      deleteReason: { type: "string" },
      purgeAfter: { type: "string" },
      deletionPolicyVersion: { type: "integer", minimum: 0 },
    },
    required: ["id", "code", "name", "status", "updatedAt"],
    additionalProperties: false,
  };
}

function appFileUploadResponseSchema(payload) {
  return {
    type: "object",
    properties: {
      data: {
        type: "object",
        properties: {
          resource: { type: "string", const: "files" },
          record: { $ref: "#/components/schemas/AdminResourceRecord" },
        },
        required: ["resource", "record"],
        additionalProperties: false,
      },
    },
    required: ["data"],
    additionalProperties: false,
    "x-platform-pii": payload.pii,
  };
}

function schemaFromPayload(payload, mediaType = "application/json", request = false) {
  if (schemaName(payload.ref) === "AppFileUploadResponse") return appFileUploadResponseSchema(payload);
  const required = payload.requiredFields ?? [];
  const schema = {
    type: "object",
    properties: Object.fromEntries(required.map((field) => [field, mediaType === "multipart/form-data" && field === "file" ? { type: "string", format: "binary" } : {}])),
    required,
    additionalProperties: false,
    "x-platform-pii": payload.pii,
  };
  if (request && forbiddenClientFields.length > 0) {
    schema.not = { anyOf: forbiddenClientFields.map((field) => ({ required: [field] })) };
    schema["x-platform-forbidden-client-fields"] = forbiddenClientFields;
  }
  return schema;
}

function collectSchemas() {
  const schemas = new Map();
  const sources = new Map();
  setSchema(schemas, sources, "AdminResourceRecord", adminResourceRecordSchema(), "generated AdminResourceRecord");
  for (const service of contract.services ?? []) {
    for (const operation of service.operations ?? []) {
      setSchema(schemas, sources, schemaName(operation.requestSchema.ref), schemaFromPayload(operation.requestSchema, operation.requestMediaType, true), `${service.id}.${operation.id} request`);
      setSchema(schemas, sources, schemaName(operation.responseSchema.ref), schemaFromPayload(operation.responseSchema), `${service.id}.${operation.id} response`);
    }
    for (const event of service.events ?? []) {
      setSchema(schemas, sources, schemaName(event.payloadSchema.ref), schemaFromPayload(event.payloadSchema), `${service.id}.${event.id} event payload`);
    }
  }
  return Object.fromEntries([...schemas.entries()].sort(([left], [right]) => left.localeCompare(right)));
}

function openAPIRef(payload) {
  return { $ref: `#/components/schemas/${schemaName(payload.ref)}` };
}

function securitySchemes() {
  return {
    adminSession: { type: "http", scheme: "bearer", bearerFormat: "JWT", description: "Management-user Admin session." },
    appSession: { type: "http", scheme: "bearer", bearerFormat: "JWT", description: "Management-user App session." },
    apiToken: { type: "apiKey", in: "header", name: "Authorization", description: "Scoped pgo_ API token." },
    oauth2ClientCredentials: { type: "oauth2", flows: { clientCredentials: { tokenUrl: "/oauth2/token", scopes: {} } }, "x-runtime-status": "declaration-only" },
    mtls: { type: "mutualTLS", "x-runtime-status": "declaration-only" },
    workloadJWT: { type: "http", scheme: "bearer", bearerFormat: "JWT", "x-runtime-status": "declaration-only" },
  };
}

function securityName(authMode) {
  return {
    "admin-session": "adminSession",
    "app-session": "appSession",
    "api-token": "apiToken",
    "oauth2-client-credentials": "oauth2ClientCredentials",
    mtls: "mtls",
    "workload-jwt": "workloadJWT",
  }[authMode];
}

function operationSecurity(operation) {
  if ((operation.authModes ?? []).includes("none")) return [];
  return (operation.authModes ?? []).map((mode) => securityName(mode)).filter(Boolean).map((name) => ({ [name]: [] }));
}

function openAPIOperation(service, operation) {
  const requestBody = operation.method === "GET" || operation.method === "DELETE" ? undefined : {
    required: true,
    content: { [operation.requestMediaType]: { schema: openAPIRef(operation.requestSchema) } },
  };
  const responseSchema = operation.responseMediaType === "application/octet-stream" ? { type: "string", format: "binary" } : openAPIRef(operation.responseSchema);
  return {
    operationId: operation.id,
    tags: [service.id],
    summary: operation.description?.en || operation.description?.zh || operation.id,
    security: operationSecurity(operation),
    parameters: Array.from(operation.path.matchAll(/:([A-Za-z0-9_]+)/g)).map((match) => ({ name: match[1], in: "path", required: true, schema: { type: "string", minLength: 1 } })),
    ...(requestBody ? { requestBody } : {}),
    responses: {
      [String(operation.successStatus)]: { description: operation.successStatus === 201 ? "Created" : "OK", content: { [operation.responseMediaType]: { schema: responseSchema } } },
      "400": errorResponse("Bad request"),
      "401": errorResponse("Authentication required"),
      "403": errorResponse("Permission denied"),
      "429": errorResponse("Rate or cost limit exceeded"),
    },
    "x-platform-capability": service.capabilityId,
    "x-platform-service": service.id,
    "x-platform-plane": operation.plane,
    "x-platform-runtime-status": operation.runtimeStatus,
    "x-platform-identity-mode": operation.identityMode,
    "x-platform-tenant-mode": operation.tenantMode,
    "x-platform-permissions": operation.permissions ?? [],
    "x-platform-data-scopes": operation.dataScopes ?? [],
    "x-platform-reliability": operation.reliability,
    "x-platform-compatibility": operation.compatibility,
  };
}

function generateOpenAPI(plane) {
  const operations = (contract.services ?? []).flatMap((service) => (service.operations ?? []).map((operation) => ({ service, operation }))).filter(({ operation }) => operation.plane === plane);
  const paths = {};
  for (const { service, operation } of operations.filter(({ operation }) => operation.runtimeStatus === "bound")) {
    const openAPIPath = operation.path.replace(/:([A-Za-z0-9_]+)/g, "{$1}");
    paths[openAPIPath] = { ...(paths[openAPIPath] ?? {}), [operation.method.toLowerCase()]: openAPIOperation(service, operation) };
  }
  const contractOnly = operations.filter(({ operation }) => operation.runtimeStatus === "contract-only").map(({ service, operation }) => ({
    capabilityId: service.capabilityId,
    serviceId: service.id,
    ...operation,
  }));
  return {
    openapi: "3.1.0",
    info: {
      title: `Platform ${plane === "service" ? "Service/Data" : plane[0].toUpperCase() + plane.slice(1)} Plane API`,
      version: contract.contractVersion,
      description: "Generated from capability.Manifest.Service. Contract-only operations are declarations and do not claim runtime handlers.",
    },
    paths: Object.fromEntries(Object.entries(paths).sort(([left], [right]) => left.localeCompare(right))),
    components: {
      securitySchemes: securitySchemes(),
      schemas: { ...collectSchemas(), ...platformErrorOpenAPISchemas(errorContract) },
    },
    "x-generated-by": "scripts/generate-platform-service-contract-artifacts.mjs",
    "x-source": path.relative(repoRoot, contractPath),
    "x-source-hash": contract.contractHash,
    "x-platform-plane": plane,
    ...(plane === "service" ? { "x-platform-planes": ["service", "data"] } : {}),
    "x-platform-tenant-context": contract.tenantContext,
    "x-platform-trace-context": contract.traceContext,
    "x-platform-contract-only-operations": sorted(contractOnly),
    ...platformErrorRegistryExtensions(errorContract),
  };
}

function tenantContextSchema() {
  const fields = contract.tenantContext?.fields ?? [];
  return {
    type: "object",
    properties: Object.fromEntries(fields.map((field) => [field, { type: "string" }])),
    required: fields.filter((field) => field !== "organizationId"),
    additionalProperties: false,
  };
}

function traceContextSchema() {
  const fields = contract.traceContext?.fields ?? [];
  return {
    type: "object",
    properties: Object.fromEntries(fields.map((field) => [field, { type: "string" }])),
    required: fields.includes("traceparent") ? ["traceparent"] : [],
    additionalProperties: false,
  };
}

function eventEnvelopeSchema(event) {
  const required = (contract.eventEnvelope?.requiredFields ?? []).filter((field) => field !== "tenantContext" || event.tenantMode === "required");
  return {
    type: "object",
    properties: {
      eventId: { type: "string", minLength: 1 },
      eventType: { type: "string", const: event.name },
      eventVersion: { type: "integer", const: event.version },
      occurredAt: { type: "string", format: "date-time" },
      producer: { type: "string", minLength: 1 },
      tenantContext: tenantContextSchema(),
      traceContext: traceContextSchema(),
      payload: openAPIRef(event.payloadSchema),
    },
    required,
    additionalProperties: false,
    "x-platform-envelope-version": event.envelopeVersion,
  };
}

function eventEnvelopeSchemaName(service, event) {
  return `${goIdentifier(service.id)}${goIdentifier(event.id)}EventEnvelopeV${event.version}`;
}

function asyncAPIMessage(service, event) {
  return {
    name: event.name,
    title: event.description?.en || event.description?.zh || event.name,
    headers: traceContextSchema(),
    payload: { $ref: `#/components/schemas/${eventEnvelopeSchemaName(service, event)}` },
    "x-platform-capability": service.capabilityId,
    "x-platform-service": service.id,
    "x-platform-event-id": event.id,
    "x-platform-event-version": event.version,
    "x-platform-envelope-version": event.envelopeVersion,
    "x-platform-runtime-status": event.runtimeStatus,
    "x-platform-tenant-mode": event.tenantMode,
    "x-platform-data-scopes": event.dataScopes ?? [],
    "x-platform-compatibility": event.compatibility,
  };
}

function generateAsyncAPI() {
  const channels = {};
  const messages = {};
  const schemas = collectSchemas();
  for (const service of contract.services ?? []) {
    for (const event of service.events ?? []) {
      const messageKey = `${service.id}_${event.id}_v${event.version}`.replaceAll("-", "_");
      messages[messageKey] = asyncAPIMessage(service, event);
      schemas[eventEnvelopeSchemaName(service, event)] = eventEnvelopeSchema(event);
      channels[event.name] = {
        address: event.name,
        messages: { [messageKey]: { $ref: `#/components/messages/${messageKey}` } },
        "x-platform-direction": event.direction,
        "x-platform-runtime-status": event.runtimeStatus,
      };
    }
  }
  return {
    asyncapi: "3.0.0",
    info: { title: "Platform Event Plane", version: contract.contractVersion, description: "Schema-level Event Plane contract. Reliable delivery is not implemented by this node." },
    channels: Object.fromEntries(Object.entries(channels).sort(([left], [right]) => left.localeCompare(right))),
    components: {
      messages: Object.fromEntries(Object.entries(messages).sort(([left], [right]) => left.localeCompare(right))),
      schemas: Object.fromEntries(Object.entries(schemas).sort(([left], [right]) => left.localeCompare(right))),
    },
    "x-generated-by": "scripts/generate-platform-service-contract-artifacts.mjs",
    "x-source": path.relative(repoRoot, contractPath),
    "x-source-hash": contract.contractHash,
    "x-platform-event-envelope": contract.eventEnvelope,
    "x-platform-trace-context": contract.traceContext,
  };
}

function goIdentifier(value) {
  return String(value).split(/[^A-Za-z0-9]+/).filter(Boolean).map((part) => part[0].toUpperCase() + part.slice(1)).join("");
}

function errorGoIdentifier(value) {
  return String(value).toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).map((part) => part[0].toUpperCase() + part.slice(1)).join("");
}

function assertUniqueGeneratedKeys(entries, label) {
  const sources = new Map();
  for (const { key, source } of entries) {
    if (sources.has(key)) throw new Error(`${label} collision for ${JSON.stringify(key)}: ${sources.get(key)} conflicts with ${source}`);
    sources.set(key, source);
  }
}

function validateGeneratedKeys() {
  const operations = (contract.services ?? []).flatMap((service) => (service.operations ?? []).map((operation) => ({ service, operation })));
  const events = (contract.services ?? []).flatMap((service) => (service.events ?? []).map((event) => ({ service, event })));
  assertUniqueGeneratedKeys(operations.map(({ service, operation }) => ({ key: operation.id, source: `${service.id}.${operation.id}` })), "TypeScript operation key");
  assertUniqueGeneratedKeys(events.map(({ service, event }) => ({ key: event.name, source: `${service.id}.${event.id}@${event.version}` })), "TypeScript event key");
  assertUniqueGeneratedKeys(operations.map(({ service, operation }) => ({ key: `Operation${goIdentifier(operation.id)}`, source: `${service.id}.${operation.id}` })), "Go operation constant");
  assertUniqueGeneratedKeys(events.map(({ service, event }) => ({ key: `Event${goIdentifier(event.id)}V${event.version}`, source: `${service.id}.${event.id}@${event.version}` })), "Go event constant");
  assertUniqueGeneratedKeys(events.map(({ service, event }) => ({ key: `${service.id}_${event.id}_v${event.version}`.replaceAll("-", "_"), source: `${service.id}.${event.id}@${event.version}` })), "AsyncAPI message key");
  assertUniqueGeneratedKeys(events.map(({ service, event }) => ({ key: eventEnvelopeSchemaName(service, event), source: `${service.id}.${event.id}@${event.version}` })), "AsyncAPI envelope schema key");
}

function goSDK() {
  const operations = sorted((contract.services ?? []).flatMap((service) => (service.operations ?? []).map((operation) => ({ serviceId: service.id, ...operation }))));
  const events = sorted((contract.services ?? []).flatMap((service) => (service.events ?? []).map((event) => ({ serviceId: service.id, ...event }))));
  const operationConstants = operations.map((operation) => `\tOperation${goIdentifier(operation.id)} = ${JSON.stringify(operation.id)}`).join("\n");
  const eventConstants = events.map((event) => `\tEvent${goIdentifier(event.id)}V${event.version} = ${JSON.stringify(event.name)}`).join("\n");
  return `// Code generated by scripts/generate-platform-service-contract-artifacts.mjs; DO NOT EDIT.\npackage servicecontractsdk\n\nimport \"encoding/json\"\n\ntype TenantContext struct { TenantID string \`json:\"tenantId\"\`; TenantCode string \`json:\"tenantCode\"\`; OrganizationID string \`json:\"organizationId,omitempty\"\`; ConfigurationVersion string \`json:\"configurationVersion\"\` }\ntype TraceContext struct { Traceparent string \`json:\"traceparent\"\`; Tracestate string \`json:\"tracestate,omitempty\"\` }\ntype EventEnvelope struct { EventID string \`json:\"eventId\"\`; EventType string \`json:\"eventType\"\`; EventVersion uint32 \`json:\"eventVersion\"\`; OccurredAt string \`json:\"occurredAt\"\`; Producer string \`json:\"producer\"\`; TenantContext *TenantContext \`json:\"tenantContext,omitempty\"\`; TraceContext TraceContext \`json:\"traceContext\"\`; Payload json.RawMessage \`json:\"payload\"\` }\n\nconst (\n${operationConstants}\n${eventConstants}\n)\n`;
}

function typeScriptSDK() {
  const operations = sorted((contract.services ?? []).flatMap((service) => (service.operations ?? []).map((operation) => ({ serviceId: service.id, ...operation }))));
  const events = sorted((contract.services ?? []).flatMap((service) => (service.events ?? []).map((event) => ({ serviceId: service.id, ...event }))));
  return `// Code generated by scripts/generate-platform-service-contract-artifacts.mjs; DO NOT EDIT.\nexport type TenantContext = { tenantId: string; tenantCode: string; organizationId?: string; configurationVersion: string };\nexport type TraceContext = { traceparent: string; tracestate?: string };\nexport type EventEnvelope<T = unknown> = { eventId: string; eventType: string; eventVersion: number; occurredAt: string; producer: string; tenantContext?: TenantContext; traceContext: TraceContext; payload: T };\nexport const operations = ${JSON.stringify(Object.fromEntries(operations.map((operation) => [operation.id, operation])), null, 2)} as const;\nexport const events = ${JSON.stringify(Object.fromEntries(events.map((event) => [event.name, event])), null, 2)} as const;\n`;
}

function goSDKWithErrors() {
  const constants = errorContract.definitions
    .map((definition) => `\tCode${errorGoIdentifier(definition.code)} PlatformErrorCode = ${JSON.stringify(definition.code)}`)
    .join("\n");
  const definitions = errorContract.definitions
    .map((definition) => `\t{Code: Code${errorGoIdentifier(definition.code)}, Owner: ${JSON.stringify(definition.owner)}, Planes: []string{${definition.planes.map((plane) => JSON.stringify(plane)).join(", ")}}, Audiences: []string{${definition.audiences.map((audience) => JSON.stringify(audience)).join(", ")}}, Category: ${JSON.stringify(definition.category)}, HTTPStatus: ${definition.httpStatus}, RetryPolicy: ${JSON.stringify(definition.retryPolicy)}, RedactionClass: ${JSON.stringify(definition.redactionClass)}, PublicMessage: ${JSON.stringify(definition.publicMessage)}, IntroducedIn: ${JSON.stringify(definition.introducedIn)}, Deprecated: ${definition.deprecated}},`)
    .join("\n");
  const header = `const PlatformErrorContractSource = ${JSON.stringify(errorContract.registrySource)}\nconst PlatformErrorContractHash = ${JSON.stringify(errorContract.contractHash)}\n\ntype PlatformErrorCode string\ntype PlatformErrorDefinition struct { Code PlatformErrorCode \`json:"code"\`; Owner string \`json:"owner"\`; Planes []string \`json:"planes"\`; Audiences []string \`json:"audiences"\`; Category string \`json:"category"\`; HTTPStatus int \`json:"httpStatus"\`; RetryPolicy string \`json:"retryPolicy"\`; RedactionClass string \`json:"redactionClass"\`; PublicMessage string \`json:"publicMessage"\`; IntroducedIn string \`json:"introducedIn"\`; Deprecated bool \`json:"deprecated"\` }\ntype PlatformErrorBody struct { Code PlatformErrorCode \`json:"code"\`; Message string \`json:"message"\`; RequestID string \`json:"requestId"\`; TraceID string \`json:"traceId"\` }\n\n`;
  const registry = `var platformErrorDefinitions = []PlatformErrorDefinition{\n${definitions}\n}\n\nfunc cloneErrorDefinition(definition PlatformErrorDefinition) PlatformErrorDefinition { definition.Planes = append([]string(nil), definition.Planes...); definition.Audiences = append([]string(nil), definition.Audiences...); return definition }\nfunc ErrorDefinitions() []PlatformErrorDefinition { result := make([]PlatformErrorDefinition, len(platformErrorDefinitions)); for index, definition := range platformErrorDefinitions { result[index] = cloneErrorDefinition(definition) }; return result }\nfunc LookupError(code PlatformErrorCode) (PlatformErrorDefinition, bool) { for _, definition := range platformErrorDefinitions { if definition.Code == code { return cloneErrorDefinition(definition), true } }; return PlatformErrorDefinition{}, false }\n`;
  return goSDK()
    .replace('import "encoding/json"\n\n', `import "encoding/json"\n\n${header}`)
    .replace("\n)\n", `\n${constants}\n)\n\n${registry}`);
}

function typeScriptSDKWithErrors() {
  const union = errorContract.definitions.map((definition) => `  | ${JSON.stringify(definition.code)}`).join("\n");
  const definitions = Object.fromEntries(errorContract.definitions.map((definition) => [definition.code, definition]));
  const errorSDK = `export const platformErrorContractSource = ${JSON.stringify(errorContract.registrySource)} as const;\nexport const platformErrorContractHash = ${JSON.stringify(errorContract.contractHash)} as const;\nexport type PlatformErrorCode =\n${union};\nexport type PlatformErrorDefinition = { readonly code: PlatformErrorCode; readonly owner: string; readonly planes: ReadonlyArray<string>; readonly audiences: ReadonlyArray<string>; readonly category: string; readonly httpStatus: number; readonly retryPolicy: string; readonly redactionClass: string; readonly publicMessage: string; readonly introducedIn: string; readonly deprecated: boolean };\nexport type PlatformErrorBody = { readonly code: PlatformErrorCode; readonly message: string; readonly requestId: string; readonly traceId: string };\nexport const platformErrorDefinitions = Object.freeze(${JSON.stringify(definitions, null, 2)}) as Readonly<Record<PlatformErrorCode, PlatformErrorDefinition>>;\nexport function isPlatformErrorCode(value: unknown): value is PlatformErrorCode { return typeof value === "string" && Object.prototype.hasOwnProperty.call(platformErrorDefinitions, value); }\n`;
  return typeScriptSDK().replace("export type TenantContext", `${errorSDK}export type TenantContext`);
}

function writeJSON(relativePath, value) {
  const target = path.join(outputDir, relativePath);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, `${JSON.stringify(value, null, 2)}\n`);
  return path.relative(repoRoot, target);
}

function writeText(relativePath, value) {
  const target = path.join(outputDir, relativePath);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, value);
  return path.relative(repoRoot, target);
}

validateGeneratedKeys();
collectSchemas();

const outputs = [
  writeJSON("openapi.service.json", generateOpenAPI("service")),
  writeJSON("openapi.control.json", generateOpenAPI("control")),
  writeJSON("openapi.external.json", generateOpenAPI("external")),
  writeJSON("asyncapi.events.json", generateAsyncAPI()),
  writeText("service-sdk/go/service_contract_sdk.go", goSDKWithErrors()),
  writeText("service-sdk/typescript/serviceContractSDK.ts", typeScriptSDKWithErrors()),
];

console.log(`Generated ${outputs.join(", ")}`);
