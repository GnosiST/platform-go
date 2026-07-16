import fs from "node:fs";
import path from "node:path";

export const platformErrorContractRelativePath = "resources/generated/platform-error-code-contract.json";

export function registrySourcePath(repoRoot, contractPath) {
  const relative = path.relative(repoRoot, contractPath).split(path.sep).join("/");
  return relative && !relative.startsWith("../") ? relative : path.basename(contractPath);
}

export function loadPlatformErrorContract(repoRoot, contractPath = path.join(repoRoot, platformErrorContractRelativePath)) {
  const resolvedPath = path.resolve(repoRoot, contractPath);
  const contract = JSON.parse(fs.readFileSync(resolvedPath, "utf8"));
  if (!Array.isArray(contract.definitions) || contract.definitions.length === 0) {
    throw new Error("platform error contract must contain definitions");
  }
  if (!/^sha256:[0-9a-f]{64}$/.test(contract.contractHash ?? "")) {
    throw new Error("platform error contract must contain a canonical sha256 hash");
  }
  const codes = new Set();
  for (const definition of contract.definitions) {
    if (!definition.code || codes.has(definition.code)) {
      throw new Error(`platform error contract contains an invalid or duplicate code ${definition.code ?? "<missing>"}`);
    }
    codes.add(definition.code);
  }
  return {
    ...contract,
    contractPath: resolvedPath,
    registrySource: registrySourcePath(repoRoot, resolvedPath),
  };
}

export function platformErrorOpenAPISchemas(contract) {
  return {
    PlatformErrorCode: {
      type: "string",
      enum: contract.definitions.map((definition) => definition.code),
    },
    PlatformErrorDefinition: {
      type: "object",
      required: [
        "code",
        "owner",
        "planes",
        "audiences",
        "category",
        "httpStatus",
        "retryPolicy",
        "redactionClass",
        "publicMessage",
        "introducedIn",
        "deprecated",
      ],
      properties: {
        code: { $ref: "#/components/schemas/PlatformErrorCode" },
        owner: { type: "string" },
        planes: { type: "array", items: { type: "string", enum: contract.catalogs?.planes ?? [] }, uniqueItems: true },
        audiences: { type: "array", items: { type: "string", enum: contract.catalogs?.audiences ?? [] }, uniqueItems: true },
        category: { type: "string", enum: contract.catalogs?.categories ?? [] },
        httpStatus: { type: "integer", minimum: 100, maximum: 599 },
        retryPolicy: { type: "string", enum: contract.catalogs?.retryPolicies ?? [] },
        redactionClass: { type: "string", enum: contract.catalogs?.redactionClasses ?? [] },
        publicMessage: { type: "string" },
        introducedIn: { type: "string" },
        deprecated: { type: "boolean" },
      },
      additionalProperties: false,
    },
    ErrorBody: {
      type: "object",
      required: ["code", "message", "requestId", "traceId"],
      properties: {
        code: { $ref: "#/components/schemas/PlatformErrorCode" },
        message: { type: "string" },
        requestId: { type: "string", pattern: "^req_[0-9a-f]{32}$" },
        traceId: { type: "string", pattern: "^[0-9a-f]{32}$" },
      },
      additionalProperties: false,
    },
    ErrorResponse: {
      type: "object",
      required: ["error"],
      properties: { error: { $ref: "#/components/schemas/ErrorBody" } },
      additionalProperties: false,
    },
  };
}

export function platformErrorRegistryExtensions(contract) {
  return {
    "x-platform-error-registry-source": contract.registrySource,
    "x-platform-error-registry-hash": contract.contractHash,
  };
}

export function errorResponse(description) {
  return {
    description,
    content: {
      "application/json": {
        schema: { $ref: "#/components/schemas/ErrorResponse" },
      },
    },
  };
}

function sameJSON(left, right) {
  return JSON.stringify(left) === JSON.stringify(right);
}

const openAPIHTTPMethods = new Set(["get", "post", "put", "patch", "delete", "head", "options", "trace"]);
const errorResponseSchema = { $ref: "#/components/schemas/ErrorResponse" };
const responseObjectFields = new Set(["description", "headers", "content", "links"]);

function componentResponseName(reference) {
  const prefix = "#/components/responses/";
  if (typeof reference !== "string" || !reference.startsWith(prefix)) return "";
  return reference.slice(prefix.length).replaceAll("~1", "/").replaceAll("~0", "~");
}

function isObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isOpenAPIResponseObject(response) {
  if (!isObject(response) || typeof response.description !== "string" || !response.description.trim()) return false;
  if (Object.keys(response).some((key) => !responseObjectFields.has(key) && !key.startsWith("x-"))) return false;
  return ["headers", "content", "links"].every((key) => !Object.hasOwn(response, key) || isObject(response[key]));
}

function validateJSONErrorResponse(response, location, errors) {
  if (!response?.content || !Object.hasOwn(response.content, "application/json")) return;
  const schema = response.content["application/json"]?.schema;
  if (!sameJSON(schema, errorResponseSchema)) {
    errors.push(`${location} application/json schema must reference ErrorResponse`);
  }
}

function validateConcreteErrorResponse(response, location, errors) {
  if (!isOpenAPIResponseObject(response)) {
    errors.push(`${location} must resolve to a valid OpenAPI Response Object`);
    return;
  }
  validateJSONErrorResponse(response, location, errors);
}

function validateComponentErrorResponse(openapi, name, label, errors, visiting, validated) {
  if (validated.has(name)) return;
  const location = `${label} components.responses.${name}`;
  if (visiting.has(name)) {
    errors.push(`${location} contains a circular component response reference`);
    return;
  }
  const response = openapi.components?.responses?.[name];
  if (!response) {
    errors.push(`${location} is missing`);
    return;
  }
  visiting.add(name);
  if (response.$ref) {
    const referencedName = componentResponseName(response.$ref);
    if (!referencedName) {
      errors.push(`${location} must reference a component response`);
    } else {
      validateComponentErrorResponse(openapi, referencedName, label, errors, visiting, validated);
    }
  } else {
    validateConcreteErrorResponse(response, location, errors);
  }
  visiting.delete(name);
  validated.add(name);
}

function validateOpenAPIErrorResponses(openapi, label, errors) {
  const referencedComponents = new Set();
  for (const [apiPath, pathItem] of Object.entries(openapi.paths ?? {})) {
    for (const [method, operation] of Object.entries(pathItem ?? {})) {
      if (!openAPIHTTPMethods.has(method)) continue;
      for (const [status, response] of Object.entries(operation?.responses ?? {})) {
        if (!/^[45](?:[0-9]{2}|XX)$/i.test(status)) continue;
        const location = `${label} ${method.toUpperCase()} ${apiPath} response ${status}`;
        if (response?.$ref) {
          const name = componentResponseName(response.$ref);
          if (!name) {
            errors.push(`${location} must reference a component response`);
            continue;
          }
          if (!openapi.components?.responses?.[name]) {
            errors.push(`${location} references missing component response ${name}`);
            continue;
          }
          referencedComponents.add(name);
          continue;
        }
        validateConcreteErrorResponse(response, location, errors);
      }
    }
  }
  const validatedComponents = new Set();
  for (const name of referencedComponents) {
    validateComponentErrorResponse(openapi, name, label, errors, new Set(), validatedComponents);
  }
}

export function validateOpenAPIErrorContract(openapi, contract, { label, planes }) {
  const errors = [];
  const expectedSchemas = platformErrorOpenAPISchemas(contract);
  if (openapi["x-platform-error-registry-source"] !== contract.registrySource) {
    errors.push(`${label} error registry source must match ${contract.registrySource}`);
  }
  if (openapi["x-platform-error-registry-hash"] !== contract.contractHash) {
    errors.push(`${label} error registry hash must match the generated registry`);
  }
  for (const [name, expected] of Object.entries(expectedSchemas)) {
    if (!sameJSON(openapi.components?.schemas?.[name], expected)) {
      errors.push(`${label} ${name} schema must match the generated error registry contract`);
    }
  }

  const definitions = new Map(contract.definitions.map((definition) => [definition.code, definition]));
  function visit(value, inheritedPlanes, location) {
    if (Array.isArray(value)) {
      value.forEach((item, index) => visit(item, inheritedPlanes, `${location}[${index}]`));
      return;
    }
    if (!value || typeof value !== "object") return;
    const currentPlanes = Array.isArray(value["x-platform-planes"])
      ? value["x-platform-planes"]
      : typeof value["x-platform-plane"] === "string"
        ? [value["x-platform-plane"]]
        : inheritedPlanes;
    if (Object.hasOwn(value, "x-platform-error-codes")) {
      const codes = value["x-platform-error-codes"];
      if (!Array.isArray(codes) || codes.length === 0) {
        errors.push(`${label} ${location} x-platform-error-codes must be a non-empty array`);
      } else {
        for (const code of codes) {
          const definition = definitions.get(code);
          if (!definition) {
            errors.push(`${label} ${location} references unknown platform error code ${code}`);
            continue;
          }
          if (currentPlanes.length > 0 && !definition.planes.some((plane) => currentPlanes.includes(plane))) {
            errors.push(`${label} ${location} platform error code ${code} does not belong to plane ${currentPlanes.join("/")}`);
          }
        }
      }
    }
    for (const [key, child] of Object.entries(value)) {
      if (key !== "x-platform-error-codes") visit(child, currentPlanes, `${location}.${key}`);
    }
  }
  visit(openapi, planes, "document");
  validateOpenAPIErrorResponses(openapi, label, errors);
  return errors;
}
