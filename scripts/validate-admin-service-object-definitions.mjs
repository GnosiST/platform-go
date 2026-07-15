import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  adminServiceObjectDefinitions,
  isForbiddenServiceObjectClientInput,
} from "./admin-service-object-definitions.mjs";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceArguments = new Map([
  ["internal/platform/serviceobject/reference.go", "--reference-source"],
  ["internal/platform/organizationrbac/service_objects.go", "--organization-source"],
  ["internal/platform/organizationrbac/lifecycle_service_objects.go", "--lifecycle-source"],
  ["internal/platform/organizationrbac/navigation_service_objects.go", "--navigation-source"],
]);
const valueTypes = new Map([
  ["ValueString", "string"],
  ["ValueInteger", "integer"],
  ["ValueBoolean", "boolean"],
  ["ValueStringSet", "string-set"],
  ["ValueMenuDefinition", "menu-definition"],
  ["ValueRoleRemediations", "role-remediations"],
]);
const tenantModes = new Map([
  ["TenantRequired", "required"],
  ["TenantPlatform", "platform"],
]);

function optionValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function readSource(relativePath) {
  const option = sourceArguments.get(relativePath);
  return fs.readFileSync(path.resolve(repoRoot, optionValue(option, relativePath)), "utf8");
}

function matchingBrace(source, start) {
  let depth = 0;
  let quote = "";
  let escaped = false;
  for (let index = start; index < source.length; index += 1) {
    const character = source[index];
    if (quote) {
      if (escaped) escaped = false;
      else if (character === "\\") escaped = true;
      else if (character === quote) quote = "";
      continue;
    }
    if (character === '"' || character === "'" || character === "`") {
      quote = character;
      continue;
    }
    if (character === "{") depth += 1;
    if (character === "}" && --depth === 0) return index;
  }
  return -1;
}

function compositeContaining(source, marker) {
  const markerIndex = source.indexOf(marker);
  if (markerIndex < 0) return "";
  for (let start = markerIndex; start >= 0; start -= 1) {
    if (source[start] !== "{") continue;
    const end = matchingBrace(source, start);
    if (end > markerIndex) return source.slice(start, end + 1);
  }
  return "";
}

function namedComposite(source, name) {
  const fieldIndex = source.indexOf(`${name}:`);
  if (fieldIndex < 0) return "";
  const start = source.indexOf("{", fieldIndex);
  if (start < 0) return "";
  const end = matchingBrace(source, start);
  return end < 0 ? "" : source.slice(start, end + 1);
}

function variableComposite(source, name) {
  const declaration = new RegExp(`\\b${name}\\s*:=\\s*[^\\n{]*\\{`).exec(source);
  if (!declaration) return "";
  const start = source.indexOf("{", declaration.index);
  const end = matchingBrace(source, start);
  return end < 0 ? "" : source.slice(start, end + 1);
}

function functionBody(source, name) {
  const functionIndex = source.indexOf(`func ${name}(`);
  if (functionIndex < 0) return "";
  const start = source.indexOf("{", functionIndex);
  const end = matchingBrace(source, start);
  return start < 0 || end < 0 ? "" : source.slice(start, end + 1);
}

function functionContaining(source, marker) {
  const markerIndex = source.indexOf(marker);
  if (markerIndex < 0) return "";
  let functionIndex = source.lastIndexOf("\nfunc ", markerIndex);
  if (functionIndex < 0 && source.startsWith("func ")) functionIndex = 0;
  while (functionIndex >= 0) {
    const start = source.indexOf("{", functionIndex);
    const end = matchingBrace(source, start);
    if (start >= 0 && start < markerIndex && end > markerIndex) {
      return source.slice(start, end + 1);
    }
    functionIndex = source.lastIndexOf("\nfunc ", functionIndex - 1);
  }
  return "";
}

function constants(source) {
  return new Map(
    [...source.matchAll(/^\s*([A-Za-z][A-Za-z0-9_]*)\s*=\s*"([^"]*)"/gm)].map((match) => [match[1], match[2]]),
  );
}

function numericVariables(source) {
  const values = new Map();
  const numericValue = (expression) => {
    const normalized = expression.trim();
    if (/^\d+$/.test(normalized)) return Number(normalized);
    if (normalized === "^uint32(0)") return 4294967295;
    return undefined;
  };
  for (const match of source.matchAll(/([A-Za-z][A-Za-z0-9_]*),\s*([A-Za-z][A-Za-z0-9_]*)\s*:=\s*int64\((.+)\),\s*int64\((.+)\)/g)) {
    values.set(match[1], numericValue(match[3]));
    values.set(match[2], numericValue(match[4]));
  }
  return values;
}

function parseFields(source, contextSource = source) {
  const numericValues = numericVariables(contextSource);
  return [...source.matchAll(/\{Name:\s*"([^"]+)",\s*Type:\s*(?:serviceobject\.)?(Value[A-Za-z]+)([^}]*)\}/g)].map((match) => ({
    name: match[1],
    type: valueTypes.get(match[2]) ?? match[2],
    ...(match[3].includes("Required: true") ? { required: true } : {}),
    ...(/MaxLength:\s*(\d+)/.exec(match[3]) ? { maxLength: Number(/MaxLength:\s*(\d+)/.exec(match[3])[1]) } : {}),
    ...(/Minimum:\s*&([A-Za-z][A-Za-z0-9_]*)/.exec(match[3])
      ? { minimum: numericValues.get(/Minimum:\s*&([A-Za-z][A-Za-z0-9_]*)/.exec(match[3])[1]) }
      : {}),
    ...(/Maximum:\s*&([A-Za-z][A-Za-z0-9_]*)/.exec(match[3])
      ? { maximum: numericValues.get(/Maximum:\s*&([A-Za-z][A-Za-z0-9_]*)/.exec(match[3])[1]) }
      : {}),
  }));
}

function parseDefinitionFields(block, fieldName, source) {
  const helper = new RegExp(`${fieldName}:\\s*([A-Za-z][A-Za-z0-9_]*)\\(\\)`).exec(block)?.[1];
  return parseFields(helper ? functionBody(source, helper) : namedComposite(block, fieldName), source);
}

function parseResultFields(block, source) {
  return parseDefinitionFields(block, "ResultSchema", source).map(({ name, type }) => ({ name, type }));
}

function parseAdditionalPermissions(block, source) {
  const identifier = identifierField(block, "AdditionalPermissions");
  const permissions = identifier ? variableComposite(source, identifier) : namedComposite(block, "AdditionalPermissions");
  return [...permissions.matchAll(/\{Permission:\s*"([^"]+)",\s*Action:\s*"([^"]+)"\}/g)].map(
    (match) => ({ permission: match[1], action: match[2] }),
  );
}

function stringField(block, name) {
  return new RegExp(`${name}:\\s*"([^"]+)"`).exec(block)?.[1] ?? "";
}

function resolvedStringField(block, name, source, fallback = "") {
  const literal = stringField(block, name);
  if (literal) return literal;
  const identifier = identifierField(block, name);
  return constants(source).get(identifier) ?? fallback;
}

function identifierField(block, name) {
  return new RegExp(`${name}:\\s*(?:serviceobject\\.)?([A-Za-z][A-Za-z0-9_]*)`).exec(block)?.[1] ?? "";
}

function numberField(block, name) {
  const value = new RegExp(`${name}:\\s*(\\d+)`).exec(block)?.[1];
  return value === undefined ? undefined : Number(value);
}

function timeoutMilliseconds(block) {
  const match = /Timeout:\s*(\d+)\s*\*\s*time\.(Millisecond|Second)/.exec(block);
  if (!match) return undefined;
  return Number(match[1]) * (match[2] === "Second" ? 1000 : 1);
}

function costPolicy(source, block) {
  let costBlock = "";
  const literal = /Cost:\s*(?:serviceobject\.)?CostPolicy\s*\{/.exec(block);
  if (literal) {
    const start = block.indexOf("{", literal.index);
    const end = matchingBrace(block, start);
    costBlock = block.slice(start, end + 1);
  } else if (/Cost:\s*(?:baseCost|cost)\b/.test(block)) {
    const shared = /baseCost\s*:=\s*serviceobject\.CostPolicy\s*\{/.exec(source);
    if (shared) {
      const start = source.indexOf("{", shared.index);
      const end = matchingBrace(source, start);
      costBlock = source.slice(start, end + 1);
    }
  }
  const number = (name) => Number(new RegExp(`${name}:\\s*(\\d+)`).exec(costBlock)?.[1] ?? 0);
  return {
    baseCost: number("BaseCost"),
    perRowCost: number("PerRowCost"),
    perOffsetCost: number("PerOffsetCost"),
    predicateCost: number("PredicateCost"),
    sortCost: number("SortCost"),
    totalCost: number("TotalCost"),
    maxOffset: number("MaxOffset"),
    limit: number("Limit"),
  };
}

function parseGoDefinition(definition, source, costSource = source, resultSource = source) {
  const definitionSource = definition.goFactory ? functionBody(source, definition.goFactory) : source;
  const idMarker = definition.goIDMarker ?? (definition.goFactory ? "ID: id" : `ID: ${definition.goIDSymbol}`);
  const definitionScope = functionContaining(definitionSource, idMarker) || definitionSource;
  const block = compositeContaining(definitionScope, idMarker);
  if (!block) return null;
  const allowedSortBlock = namedComposite(block, "AllowedSort");
  return {
    resource: resolvedStringField(block, "Resource", source, identifierField(block, "Resource") === "resource" ? definition.resource : ""),
    permission: resolvedStringField(
      block,
      "Permission",
      source,
      identifierField(block, "Permission") === "permission" ? definition.permission : "",
    ),
    action: stringField(block, "Action"),
    additionalPermissions: parseAdditionalPermissions(block, definitionScope),
    tenantMode: tenantModes.get(identifierField(block, "TenantMode")) ?? identifierField(block, "TenantMode"),
    dataScope: stringField(block, "DataScope"),
    arguments: parseDefinitionFields(block, "Arguments", definitionSource),
    allowedSort: [...allowedSortBlock.matchAll(/\{Name:\s*"([^"]+)"/g)].map((match) => match[1]),
    cost: costPolicy(costSource, block),
    timeoutMs: timeoutMilliseconds(block),
    maxPageSize: numberField(block, "MaxPageSize"),
    exposeTotal: /ExposeTotal:\s*true/.test(block),
    idempotency:
      identifierField(block, "Idempotency") === "IdempotencyRequiredKey" ? "required-key" : "none",
    maxAffectedRows: numberField(block, "MaxAffectedRows"),
    result: parseResultFields(block, resultSource),
  };
}

function comparableJSDefinition(kind, definition) {
  const shared = {
    resource: definition.resource,
    permission: definition.permission,
    action: definition.action,
    additionalPermissions: (definition.additionalPermissions ?? []).map(({ permission, action }) => ({ permission, action })),
    tenantMode: definition.tenantMode,
    dataScope: definition.dataScope,
    arguments: definition.arguments.map(({ name, type, required, maxLength, minimum, maximum }) => ({
      name,
      type,
      ...(required === true ? { required: true } : {}),
      ...(maxLength ? { maxLength } : {}),
      ...(minimum !== undefined ? { minimum } : {}),
      ...(maximum !== undefined ? { maximum } : {}),
    })),
    allowedSort: kind === "query" ? [...definition.allowedSort] : [],
    cost: definition.cost,
    timeoutMs: definition.timeoutMs,
    maxPageSize: kind === "query" ? definition.maxPageSize : undefined,
    exposeTotal: kind === "query" ? definition.exposeTotal : false,
    idempotency: kind === "command" ? definition.idempotency : "none",
    maxAffectedRows: kind === "command" ? definition.maxAffectedRows : undefined,
    result: definition.result.map(({ name, type }) => ({ name, type })),
  };
  return shared;
}

const errors = [];
const sources = new Map(adminServiceObjectDefinitions.runtimeSources.map((sourcePath) => [sourcePath, readSource(sourcePath)]));
const declaredSources = [...new Set([
  ...adminServiceObjectDefinitions.queries,
  ...adminServiceObjectDefinitions.commands,
].map((definition) => definition.goSource))].sort();
if (JSON.stringify(declaredSources) !== JSON.stringify([...adminServiceObjectDefinitions.runtimeSources].sort())) {
  errors.push("service-object runtimeSources must exactly cover definition goSource values");
}

for (const [kind, definitions] of [
  ["query", adminServiceObjectDefinitions.queries],
  ["command", adminServiceObjectDefinitions.commands],
]) {
  for (const definition of definitions) {
    const source = sources.get(definition.goSource);
    if (!source) {
      errors.push(`${definition.id}@${definition.version} references an undeclared Go source`);
      continue;
    }
    const idSource = sources.get(definition.goIDSource ?? definition.goSource) ?? source;
    const versionSource = sources.get(definition.goVersionSource ?? definition.goSource) ?? source;
    const definitionSource = sources.get(definition.goDefinitionSource ?? definition.goSource) ?? source;
    const costSource = sources.get(definition.goCostSource ?? definition.goDefinitionSource ?? definition.goSource) ?? definitionSource;
    const resultSource = sources.get(definition.goResultSource ?? definition.goDefinitionSource ?? definition.goSource) ?? definitionSource;
    const sourceConstants = constants(idSource);
    if (sourceConstants.get(definition.goIDSymbol) !== definition.id) {
      errors.push(`${definition.id}@${definition.version} Go ID constant does not match the JS definition`);
    }
    if (constants(versionSource).get(definition.goVersionSymbol) !== definition.version) {
      errors.push(`${definition.id}@${definition.version} Go version constant does not match the JS definition`);
    }
    for (const requirement of definition.goRequiredSnippets ?? []) {
      const requiredSource = sources.get(requirement.source);
      if (!requiredSource?.includes(requirement.value)) {
        errors.push(`${definition.id}@${definition.version} Go registration must include ${requirement.value}`);
      }
    }
    if (definition.goFactory && !definition.goRequiredSnippets) {
      const invocation = new RegExp(
        `${definition.goFactory}\\(\\s*${definition.goIDSymbol},\\s*"([^"]+)",\\s*"([^"]+)"(?:,\\s*([A-Za-z][A-Za-z0-9_]*))?`,
      ).exec(source);
      if (
        !invocation ||
        invocation[1] !== definition.resource ||
        invocation[2] !== definition.permission ||
        (definition.goFactory === "applyDomainDefinition" && invocation[3] !== "baseCost")
      ) {
        errors.push(`${definition.id}@${definition.version} Go factory invocation does not match the JS resource, permission and cost`);
      }
    }
    const goDefinition = parseGoDefinition(definition, definitionSource, costSource, resultSource);
    if (!goDefinition) {
      errors.push(`${definition.id}@${definition.version} Go definition block is missing`);
      continue;
    }
    const jsDefinition = comparableJSDefinition(kind, definition);
    if (JSON.stringify(goDefinition) !== JSON.stringify(jsDefinition)) {
      errors.push(`${definition.id}@${definition.version} Go and JS definition fields differ`);
    }
    if (
      kind === "command" &&
      definition.operationPhase === "apply" &&
      definition.arguments.some(
        (argument) =>
          !["previewId", "expectedRevision", "impactHash"].includes(argument.name) ||
          isForbiddenServiceObjectClientInput(argument.name) ||
          argument.type === "string-set" ||
          argument.type === "role-remediations",
      )
    ) {
      errors.push(`${definition.id}@${definition.version} apply input exposes target, tenant, datasource or physical fields`);
    }
  }
}

for (const [sourcePath, source] of sources) {
  const registeredSymbols = new Set(
    [...adminServiceObjectDefinitions.queries, ...adminServiceObjectDefinitions.commands]
      .filter((definition) => definition.goSource === sourcePath)
      .map((definition) => definition.goIDSymbol),
  );
  const runtimeSymbols = new Set(
    [
      ...source.matchAll(/\bID:\s*([A-Za-z][A-Za-z0-9_]*(?:QueryID|CommandID))\b/g),
      ...source.matchAll(/\b(?:impactQueryDefinition|conflictQueryDefinition|applyDomainDefinition)\(\s*([A-Za-z][A-Za-z0-9_]*(?:QueryID|CommandID))\b/g),
    ].map((match) => match[1]),
  );
  if (JSON.stringify([...runtimeSymbols].sort()) !== JSON.stringify([...registeredSymbols].sort())) {
    errors.push(`${sourcePath} service-object IDs are not fully mirrored by the JS definition registry`);
  }
}

if (errors.length > 0) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}

console.log("Validated Admin service-object Go/JS definition consistency");
