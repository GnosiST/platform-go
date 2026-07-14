export const forbiddenServiceObjectClientInputs = Object.freeze([
  "field",
  "operator",
  "sql",
  "join",
  "dsn",
  "datasource",
  "database",
  "schema",
  "shard",
]);

const forbiddenPhysicalInputPrefixes = Object.freeze([
  "dsn",
  "datasource",
  "database",
  "schema",
  "shard",
  "field",
  "operator",
  "sql",
  "join",
]);

export function isForbiddenServiceObjectClientInput(name) {
  const normalized = String(name ?? "").trim().toLowerCase();
  return (
    forbiddenServiceObjectClientInputs.includes(normalized) ||
    forbiddenPhysicalInputPrefixes.some((prefix) => normalized.startsWith(prefix))
  );
}

// Executable codegen metadata mirrors the reference definitions registered by
// internal/platform/serviceobject/reference.go. OpenAPI and Admin client
// previews must derive their public contracts from this single declaration.
export const adminServiceObjectDefinitions = Object.freeze({
  source: "scripts/admin-service-object-definitions.mjs",
  runtimeSource: "internal/platform/serviceobject/reference.go",
  queries: Object.freeze([
    Object.freeze({
      codegenName: "ReferenceRecordsList",
      clientMethod: "listReferenceRecords",
      id: "platform.reference-records.list",
      version: "1.0.0",
      resource: "reference-records",
      permission: "admin:reference-records:read",
      action: "read",
      tenantMode: "required",
      dataScope: "tenant",
      arguments: Object.freeze([
        Object.freeze({ name: "status", type: "string", maxLength: 32 }),
        Object.freeze({ name: "codePrefix", type: "string", maxLength: 64 }),
      ]),
      allowedSort: Object.freeze(["code", "name"]),
      cost: Object.freeze({
        baseCost: 2,
        perRowCost: 1,
        perOffsetCost: 1,
        predicateCost: 1,
        sortCost: 1,
        totalCost: 20,
        maxOffset: 1000,
        limit: 128,
      }),
      timeoutMs: 2000,
      maxPageSize: 100,
      exposeTotal: false,
      result: Object.freeze([
        Object.freeze({ name: "id", type: "integer" }),
        Object.freeze({ name: "code", type: "string" }),
        Object.freeze({ name: "name", type: "string" }),
        Object.freeze({ name: "status", type: "string" }),
      ]),
    }),
  ]),
  commands: Object.freeze([
    Object.freeze({
      codegenName: "ReferenceRecordsRename",
      clientMethod: "renameReferenceRecord",
      id: "platform.reference-records.rename",
      version: "1.0.0",
      resource: "reference-records",
      permission: "admin:reference-records:update",
      action: "update",
      tenantMode: "required",
      dataScope: "tenant",
      arguments: Object.freeze([
        Object.freeze({ name: "code", type: "string", required: true, maxLength: 64 }),
        Object.freeze({ name: "name", type: "string", required: true, maxLength: 128 }),
      ]),
      cost: Object.freeze({
        baseCost: 5,
        perRowCost: 1,
        perOffsetCost: 0,
        predicateCost: 1,
        sortCost: 0,
        totalCost: 0,
        maxOffset: 0,
        limit: 7,
      }),
      timeoutMs: 2000,
      idempotency: "required-key",
      maxAffectedRows: 1,
      result: Object.freeze([Object.freeze({ name: "affected", type: "integer" })]),
    }),
  ]),
});

const supportedValueTypes = new Set(["string", "integer", "boolean"]);
const versionPattern = /^[0-9]+\.[0-9]+\.[0-9]+$/;

function assertDefinition(kind, definition, keys, codegenKeys, clientMethods) {
  if (!definition.id || !versionPattern.test(definition.version) || !definition.codegenName || !definition.clientMethod) {
    throw new Error(`invalid Admin service object definition: ${definition.id || "<missing>"}`);
  }
  const key = `${definition.id}@${definition.version}`;
  if (keys.has(key)) {
    throw new Error(`duplicate Admin service object definition: ${key}`);
  }
  keys.add(key);
  const codegenKey = `${definition.codegenName}@${definition.version}`;
  if (codegenKeys.has(codegenKey)) {
    throw new Error(`duplicate Admin service object codegen name: ${codegenKey}`);
  }
  codegenKeys.add(codegenKey);
  if (!/^[A-Za-z_$][A-Za-z0-9_$]*$/.test(definition.clientMethod) || clientMethods.has(definition.clientMethod)) {
    throw new Error(`invalid or duplicate Admin service object client method: ${definition.clientMethod}`);
  }
  clientMethods.add(definition.clientMethod);

  for (const entry of [...definition.arguments, ...definition.result]) {
    if (!entry.name || !supportedValueTypes.has(entry.type)) {
      throw new Error(`invalid ${kind} field in ${key}: ${entry.name || "<missing>"}`);
    }
  }
  for (const argument of definition.arguments) {
    if (isForbiddenServiceObjectClientInput(argument.name)) {
      throw new Error(`forbidden physical query input in ${key}: ${argument.name}`);
    }
  }
  for (const sortName of definition.allowedSort ?? []) {
    if (!sortName || isForbiddenServiceObjectClientInput(sortName)) {
      throw new Error(`forbidden physical sort input in ${key}: ${sortName || "<missing>"}`);
    }
  }
}

const definitionKeys = new Set();
const codegenKeys = new Set();
const clientMethods = new Set();
for (const definition of adminServiceObjectDefinitions.queries) {
  assertDefinition("query", definition, definitionKeys, codegenKeys, clientMethods);
}
for (const definition of adminServiceObjectDefinitions.commands) {
  assertDefinition("command", definition, definitionKeys, codegenKeys, clientMethods);
}
