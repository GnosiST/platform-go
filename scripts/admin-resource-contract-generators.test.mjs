import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import {
  adminServiceObjectDefinitions,
  forbiddenServiceObjectClientInputs,
  isForbiddenServiceObjectClientInput,
} from "./admin-service-object-definitions.mjs";

const requiredNavigationServiceObjects = Object.freeze([
  { kind: "query", id: "platform.navigation.menu-definition.get", clientMethod: "getMenuDefinition" },
  { kind: "query", id: "platform.navigation.role-menus.get", clientMethod: "getRoleMenus" },
  { kind: "query", id: "platform.navigation.role-menu-change.impact", clientMethod: "getRoleMenuChangeImpact" },
  { kind: "query", id: "platform.navigation.role-menu-migration.compare", clientMethod: "compareRoleMenuMigration" },
  { kind: "command", id: "platform.navigation.menu-definition.create", clientMethod: "createMenuDefinition" },
  { kind: "command", id: "platform.navigation.menu-definition.replace", clientMethod: "replaceMenuDefinition" },
  { kind: "command", id: "platform.navigation.role-menu-change.prepare", clientMethod: "prepareRoleMenuChange" },
  { kind: "command", id: "platform.navigation.role-menus.replace", clientMethod: "replaceRoleMenus" },
]);

function runAdminResourceContract(env = {}, args = []) {
  const result = spawnSync(process.execPath, ["scripts/generate-admin-resource-contract.mjs", "--stdout", ...args], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
    env: { ...process.env, ...env },
  });
  assert.equal(result.status, 0, `generate-admin-resource-contract.mjs failed\n${result.stdout}${result.stderr}`);
  return JSON.parse(result.stdout);
}

function runAdminCodegenPreviewForContract(contract) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-contract-"));
  const contractPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(contractPath, JSON.stringify(contract, null, 2));
  const result = spawnSync(process.execPath, ["scripts/generate-admin-codegen-preview.mjs", "--stdout", "--contract", contractPath], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
  });
  assert.equal(result.status, 0, `generate-admin-codegen-preview.mjs failed\n${result.stdout}${result.stderr}`);
  return JSON.parse(result.stdout);
}

function runAdminServiceObjectClientForContract(contract) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-contract-"));
  const contractPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(contractPath, JSON.stringify(contract, null, 2));
  const result = spawnSync(
    process.execPath,
    ["scripts/generate-admin-codegen-preview.mjs", "--typescript-stdout", "--contract", contractPath],
    {
      cwd: new URL("..", import.meta.url),
      encoding: "utf8",
    },
  );
  assert.equal(result.status, 0, `generate-admin-codegen-preview.mjs failed\n${result.stdout}${result.stderr}`);
  return result.stdout;
}

function compileAdminServiceObjectClient(source) {
  const repoRoot = path.resolve(import.meta.dirname, "..");
  const tsc = path.join(repoRoot, "admin", "node_modules", ".bin", "tsc");
  assert.ok(fs.existsSync(tsc), "admin TypeScript compiler must be installed");
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-service-object-client-"));
  const generatedDir = path.join(tempDir, "resources", "generated");
  const clientPath = path.join(generatedDir, "admin-service-object-client.ts");
  const errorContractDir = path.join(generatedDir, "error-sdk", "typescript");
  fs.mkdirSync(errorContractDir, { recursive: true });
  fs.writeFileSync(clientPath, source);
  const generatedErrors = generateErrorArtifacts(path.join(generatedDir, "error-sdk"));
  assert.equal(generatedErrors.status, 0, generatedErrors.stderr);
  fs.writeFileSync(
    path.join(tempDir, "consumer.ts"),
    `import type { AdminServiceObjectClient, AdminServiceObjectMenuDefinition } from "./resources/generated/admin-service-object-client";

declare const client: AdminServiceObjectClient;
declare const menuDefinition: AdminServiceObjectMenuDefinition;

client.listReferenceRecords({
  arguments: { status: "active", codePrefix: "REF-" },
  pagination: { page: 1, pageSize: 25 },
  sort: [{ name: "code", order: "asc" }],
}).then((response) => response.data?.items[0]?.code);

client.renameReferenceRecord({
  arguments: { code: "REF-1", name: "Renamed" },
  idempotencyKey: "rename-ref-1",
}).then((response) => response.data?.values.affected);

client.prepareOrganizationRoleGroupChange({
  arguments: {
    orgUnitCode: "org-1",
    roleGroupCodes: ["group-1", "group-2"],
    remediations: [
      { userCode: "user-1", roleCode: "role-1", action: "remove-role" },
      { userCode: "user-2", roleCode: "role-2", action: "replace-role", replacementRoleCode: "role-3" },
    ],
  },
  idempotencyKey: "prepare-org-role-groups-1",
}).then((response) => response.data?.values.previewId);

client.replaceOrganizationRoleGroups({
  arguments: { previewId: "preview-1", expectedRevision: 7, impactHash: "impact-1" },
  idempotencyKey: "apply-org-role-groups-1",
}).then((response) => response.data?.values.revision);

client.prepareRolePermissionChange({
  arguments: {
    roleCode: "operator",
    allowPermissionCodes: ["admin:user:read", "admin:user:update"],
    denyPermissionCodes: [],
    dataScope: "all",
  },
  idempotencyKey: "prepare-role-permissions-1",
}).then((response) => response.data?.values.previewId);

client.prepareAuthorizationResourceLifecycle({
  arguments: {
    resource: "roles",
    resourceCode: "operator",
    operation: "delete",
    retentionDays: 90,
    policyVersion: 1,
    remediations: [{ userCode: "user-1", roleCode: "operator", action: "remove-role" }],
  },
  idempotencyKey: "prepare-resource-lifecycle-1",
}).then((response) => response.data?.values.previewId);

client.applyAuthorizationResourceLifecycle({
  arguments: { previewId: "preview-lifecycle-1", expectedRevision: 9, impactHash: "impact-lifecycle-1" },
  idempotencyKey: "apply-resource-lifecycle-1",
}).then((response) => response.data?.values.applied);

client.getMenuDefinition({ arguments: { menuCode: "users" } }).then((response) => response.data?.items[0]?.definition);
client.getRoleMenus({ arguments: { roleCode: "operator" } }).then((response) => response.data?.items[0]?.menuCodes);
client.getRoleMenuChangeImpact({ arguments: { previewId: "preview-role-menu-1" } }).then((response) => response.data?.items[0]?.changed);
client.compareRoleMenuMigration({ arguments: { roleCode: "operator" } }).then((response) => response.data?.items[0]?.targetMenuCodes);
client.createMenuDefinition({
  arguments: { definition: menuDefinition, expectedRevision: 0 },
  idempotencyKey: "create-menu-1",
}).then((response) => response.data?.values.revision);
client.replaceMenuDefinition({
  arguments: { definition: menuDefinition, expectedRevision: 1 },
  idempotencyKey: "replace-menu-1",
}).then((response) => response.data?.values.revision);
client.prepareRoleMenuChange({
  arguments: { roleCode: "operator", menuCodes: ["users"] },
  idempotencyKey: "prepare-role-menu-1",
}).then((response) => response.data?.values.previewId);
client.replaceRoleMenus({
  arguments: { previewId: "preview-role-menu-1", expectedRevision: 1, impactHash: "impact-role-menu-1" },
  idempotencyKey: "replace-role-menu-1",
}).then((response) => response.data?.values.revision);

// @ts-expect-error physical fields are not part of persisted query input
client.listReferenceRecords({ field: "status" });
// @ts-expect-error operators are compiled by the server-side definition
client.listReferenceRecords({ arguments: { operator: "equal" } });
// @ts-expect-error arbitrary datasource selection is forbidden
client.listReferenceRecords({ arguments: { datasource: "primary" } });
// @ts-expect-error idempotency keys are required by this command definition
client.renameReferenceRecord({ arguments: { code: "REF-1", name: "Renamed" } });
// @ts-expect-error apply inputs reload server-owned targets from the preview
client.replaceOrganizationRoleGroups({ arguments: { previewId: "preview-1", expectedRevision: 7, impactHash: "impact-1", roleGroupCodes: ["group-1"] }, idempotencyKey: "apply-org-role-groups-2" });
// @ts-expect-error replace-role remediations require an explicit replacement role
client.prepareOrganizationRoleGroupChange({ arguments: { orgUnitCode: "org-1", roleGroupCodes: [], remediations: [{ userCode: "user-1", roleCode: "role-1", action: "replace-role" }] }, idempotencyKey: "prepare-org-role-groups-2" });
`,
  );
  const result = spawnSync(
    tsc,
    ["--noEmit", "--strict", "--target", "ES2022", "--module", "ESNext", "--moduleResolution", "node", clientPath, "consumer.ts"],
    { cwd: tempDir, encoding: "utf8" },
  );
  assert.equal(result.status, 0, `generated Admin service object client did not compile\n${result.stdout}${result.stderr}`);
}

function errorContract() {
  return JSON.parse(
    fs.readFileSync(path.resolve(import.meta.dirname, "..", "resources", "generated", "platform-error-code-contract.json"), "utf8"),
  );
}

function assertOpenAPIErrorContract(openapi) {
  const registry = errorContract();
  assert.equal(openapi["x-platform-error-registry-source"], "resources/generated/platform-error-code-contract.json");
  assert.equal(openapi["x-platform-error-registry-hash"], registry.contractHash);
  assert.deepEqual(openapi.components.schemas.PlatformErrorCode, {
    type: "string",
    enum: registry.definitions.map((definition) => definition.code),
  });
  assert.deepEqual(openapi.components.schemas.ErrorBody, {
    type: "object",
    required: ["code", "message", "requestId", "traceId"],
    properties: {
      code: { $ref: "#/components/schemas/PlatformErrorCode" },
      message: { type: "string" },
      requestId: { type: "string", pattern: "^req_[0-9a-f]{32}$" },
      traceId: { type: "string", pattern: "^[0-9a-f]{32}$" },
    },
    additionalProperties: false,
  });
  assert.deepEqual(openapi.components.schemas.ErrorResponse, {
    type: "object",
    required: ["error"],
    properties: { error: { $ref: "#/components/schemas/ErrorBody" } },
    additionalProperties: false,
  });
}

function generateErrorArtifacts(outputDir) {
  return spawnSync(
    process.execPath,
    ["scripts/generate-platform-error-code-artifacts.mjs", "--output-dir", outputDir],
    { cwd: path.resolve(import.meta.dirname, ".."), encoding: "utf8" },
  );
}

function runAdminAPIBoundaryValidator(openapi) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-openapi-boundary-"));
  const openapiPath = path.join(tempDir, "openapi.admin.json");
  fs.writeFileSync(openapiPath, `${JSON.stringify(openapi, null, 2)}\n`);
  return spawnSync(
    process.execPath,
    ["scripts/validate-platform-admin-api-boundary.mjs", "--admin-openapi", openapiPath],
    { cwd: path.resolve(import.meta.dirname, ".."), encoding: "utf8" },
  );
}

function writeSensitiveManifest() {
  const sourcePath = path.resolve(import.meta.dirname, "..", "resources", "admin-resources.json");
  const manifest = JSON.parse(fs.readFileSync(sourcePath, "utf8"));
  manifest.resources.push({
    name: "custom-sensitive-records",
    code: "custom-sensitive-records",
    label: { zh: "自定义敏感记录", en: "Custom Sensitive Records" },
    group: "foundation",
    menu: { path: "/custom-sensitive-records", icon: "lock", sortOrder: 999 },
    refine: { resource: "custom-sensitive-records", list: "/custom-sensitive-records", component: "ResourceTablePage" },
    apiBase: "/api/admin/custom-sensitive-records",
    permissions: { read: "admin:custom-sensitive-record:read" },
    routes: [],
    schema: {
      protection: { schemaVersion: 7, scope: "tenant-field", tenantField: "tenantCode" },
      fields: [
        { key: "tenantCode", label: { zh: "租户", en: "Tenant" }, type: "text", source: "values", required: true },
        {
          key: "governmentReference",
          label: { zh: "政府引用", en: "Government Reference" },
          type: "text",
          source: "values",
          sensitivity: "sensitive",
          storageMode: "encrypted",
          responseMode: "masked",
          exportMode: "masked",
          filter: true,
          protection: {
            format: "aes-256-gcm-v1",
            normalization: "trim-v1",
            blindIndexNamespace: "custom-government-reference",
          },
          masking: {
            strategy: "partial-v1",
            preservePrefix: 2,
            preserveSuffix: 2,
            maskLength: 6,
          },
          reveal: {
            policyId: "custom-sensitive-step-up-v1",
            permission: "admin:custom-sensitive-record:reveal",
            copyAllowed: true,
          },
        },
      ],
      search: [],
      filter: ["governmentReference"],
      sort: [],
      table: ["tenantCode"],
      form: ["tenantCode", "governmentReference"],
      localizedFields: [],
    },
    codegen: { mode: "custom" },
  });
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-sensitive-manifest-"));
  const manifestPath = path.join(tempDir, "admin-resources.json");
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
  return manifestPath;
}

function runAdminOpenAPIForContract(contract) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-contract-"));
  const contractPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(contractPath, JSON.stringify(contract, null, 2));
  const result = spawnSync(process.execPath, ["scripts/generate-admin-openapi.mjs", "--stdout", "--contract", contractPath], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
  });
  assert.equal(result.status, 0, `generate-admin-openapi.mjs failed\n${result.stdout}${result.stderr}`);
  return JSON.parse(result.stdout);
}

function validateAdminResourceContract(contract) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-admin-contract-"));
  const contractPath = path.join(tempDir, "admin-resource-contract.json");
  fs.writeFileSync(contractPath, JSON.stringify(contract, null, 2));
  return spawnSync(process.execPath, ["scripts/validate-admin-resources.mjs", "--contract", contractPath], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
  });
}

describe("admin resource contract generators", () => {
  it("generates the unified Admin OpenAPI error contract", () => {
    const contract = JSON.parse(fs.readFileSync(path.resolve(import.meta.dirname, "..", "resources", "generated", "admin-resource-contract.json"), "utf8"));
    const openapi = runAdminOpenAPIForContract(contract);
    assertOpenAPIErrorContract(openapi);
    assert.deepEqual(
      openapi.paths["/api/admin/service-objects/query"].post.responses["404"]["x-platform-error-codes"],
      ["SERVICE_OBJECT_UNAVAILABLE"],
    );
    assert.equal(
      openapi.components.responses.BadRequest.content["application/json"].schema.$ref,
      "#/components/schemas/ErrorResponse",
    );
  });

  it("rejects unknown and cross-plane explicit Admin OpenAPI error codes", () => {
    const contract = JSON.parse(fs.readFileSync(path.resolve(import.meta.dirname, "..", "resources", "generated", "admin-resource-contract.json"), "utf8"));
    const openapi = runAdminOpenAPIForContract(contract);
    openapi.paths["/api/admin/service-objects/query"].post.responses["404"]["x-platform-error-codes"] = [
      "UNKNOWN_PLATFORM_ERROR",
      "APP_AUTH_INVALID_REQUEST",
    ];
    const result = runAdminAPIBoundaryValidator(openapi);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /unknown platform error code UNKNOWN_PLATFORM_ERROR/);
    assert.match(result.stderr, /platform error code APP_AUTH_INVALID_REQUEST does not belong to plane admin/);
  });

  it("rejects permissive Admin component error responses without constraining success responses", () => {
    const contract = JSON.parse(fs.readFileSync(path.resolve(import.meta.dirname, "..", "resources", "generated", "admin-resource-contract.json"), "utf8"));
    const openapi = runAdminOpenAPIForContract(contract);
    openapi.components.responses.BadRequest.content["application/json"].schema = {
      type: "object",
      properties: { data: {}, error: {} },
    };
    const invalid = runAdminAPIBoundaryValidator(openapi);
    assert.notEqual(invalid.status, 0);
    assert.match(invalid.stderr, /components\.responses\.BadRequest application\/json schema must reference ErrorResponse/);

    const success = runAdminOpenAPIForContract(contract);
    success.paths["/api/admin/service-objects/query"].post.responses["200"].content["application/json"].schema = {
      type: "object",
      properties: { data: {}, error: {} },
    };
    const accepted = runAdminAPIBoundaryValidator(success);
    assert.equal(accepted.status, 0, accepted.stderr);
  });

  it("generates deterministic standalone Go and TypeScript error SDKs", () => {
    const repoRoot = path.resolve(import.meta.dirname, "..");
    const first = fs.mkdtempSync(path.join(os.tmpdir(), "platform-error-sdk-a-"));
    const second = fs.mkdtempSync(path.join(os.tmpdir(), "platform-error-sdk-b-"));
    for (const outputDir of [first, second]) {
      const result = generateErrorArtifacts(outputDir);
      assert.equal(result.status, 0, result.stderr);
    }
    for (const relativePath of ["go/error_contract.go", "typescript/errorContract.ts"]) {
      assert.equal(fs.readFileSync(path.join(first, relativePath), "utf8"), fs.readFileSync(path.join(second, relativePath), "utf8"));
    }

    const goDir = path.join(first, "go");
    fs.writeFileSync(path.join(goDir, "go.mod"), "module platform-error-contract-sdk\n\ngo 1.26\n");
    fs.writeFileSync(
      path.join(goDir, "error_contract_test.go"),
      'package errorcontract\n\nimport "testing"\n\nfunc TestGeneratedRegistry(t *testing.T) { if definition, ok := Lookup(CodeInternalError); !ok || definition.Code != CodeInternalError || len(Definitions()) == 0 { t.Fatal("missing generated error registry") } }\n',
    );
    const goResult = spawnSync("go", ["test", "./..."], { cwd: goDir, encoding: "utf8" });
    assert.equal(goResult.status, 0, goResult.stderr);

    const tsc = path.join(repoRoot, "admin", "node_modules", ".bin", "tsc");
    const consumerPath = path.join(first, "typescript", "consumer.ts");
    fs.writeFileSync(
      consumerPath,
      'import { isPlatformErrorCode, platformErrorDefinitions, type PlatformErrorBody } from "./errorContract";\nconst body: PlatformErrorBody = { code: "INTERNAL_ERROR", message: "internal server error", requestId: "req_00000000000000000000000000000000", traceId: "00000000000000000000000000000000" };\nif (!isPlatformErrorCode(body.code) || platformErrorDefinitions[body.code].code !== body.code) throw new Error("missing generated definition");\n',
    );
    const tsResult = spawnSync(
      tsc,
      ["--noEmit", "--strict", "--target", "ES2022", "--module", "ESNext", "--moduleResolution", "node", path.join(first, "typescript", "errorContract.ts"), consumerPath],
      { cwd: repoRoot, encoding: "utf8" },
    );
    assert.equal(tsResult.status, 0, tsResult.stderr || tsResult.stdout);
  });

  it("rejects physical routing input name variants", () => {
    for (const name of [
      "datasource",
      "datasourceId",
      "shardKey",
      "schemaVersion",
      "fieldName",
      "operatorType",
      "sqlExpression",
      "joinTarget",
    ]) {
      assert.equal(isForbiddenServiceObjectClientInput(name), true, `${name} must remain server-controlled`);
    }
    assert.equal(isForbiddenServiceObjectClientInput("status"), false);
  });

  it("publishes separate persisted query and command object transports", () => {
    const contract = runAdminResourceContract();
    const openapi = runAdminOpenAPIForContract(contract);
    const query = openapi.paths["/api/admin/service-objects/query"].post;
    const command = openapi.paths["/api/admin/service-objects/command"].post;

    assert.equal(query.operationId, "executeAdminPersistedQuery");
    assert.equal(command.operationId, "executeAdminCommandObject");
    assert.deepEqual(query.security, [{ bearerAuth: [] }]);
    assert.deepEqual(command.security, [{ bearerAuth: [] }]);
    assert.equal(query["x-platform-runtime"], "conditional");
    assert.equal(command["x-platform-runtime"], "conditional");
    assert.ok(query.responses["404"]["x-platform-error-codes"].includes("SERVICE_OBJECT_UNAVAILABLE"));
    assert.ok(command.responses["404"]["x-platform-error-codes"].includes("SERVICE_OBJECT_UNAVAILABLE"));

    const queryDefinition = adminServiceObjectDefinitions.queries[0];
    const commandDefinition = adminServiceObjectDefinitions.commands[0];
    const queryUnion = openapi.components.schemas.AdminServiceObjectQueryRequest;
    const commandUnion = openapi.components.schemas.AdminServiceObjectCommandRequest;
    assert.equal(queryUnion.oneOf.length, adminServiceObjectDefinitions.queries.length);
    assert.equal(commandUnion.oneOf.length, adminServiceObjectDefinitions.commands.length);
    assert.equal(queryUnion.discriminator.propertyName, "queryId");
    assert.equal(commandUnion.discriminator.propertyName, "commandId");
    assert.deepEqual(queryUnion["x-platform-discriminator"].properties, ["queryId", "version"]);
    assert.deepEqual(commandUnion["x-platform-discriminator"].properties, ["commandId", "version"]);

    const queryRequestName = queryUnion.oneOf[0].$ref.split("/").pop();
    const commandRequestName = commandUnion.oneOf[0].$ref.split("/").pop();
    const queryRequest = openapi.components.schemas[queryRequestName];
    const commandRequest = openapi.components.schemas[commandRequestName];
    assert.equal(queryRequest.additionalProperties, false);
    assert.equal(commandRequest.additionalProperties, false);
    assert.equal(queryRequest.properties.queryId.const, queryDefinition.id);
    assert.equal(queryRequest.properties.version.const, queryDefinition.version);
    assert.equal(commandRequest.properties.commandId.const, commandDefinition.id);
    assert.equal(commandRequest.properties.version.const, commandDefinition.version);
    assert.ok(commandRequest.required.includes("idempotencyKey"));
    assert.deepEqual(queryRequest["x-platform-definition"].cost, queryDefinition.cost);
    assert.equal(commandRequest["x-platform-definition"].maxAffectedRows, commandDefinition.maxAffectedRows);
    assert.deepEqual(queryRequest["x-platform-definition"].additionalPermissions, []);
    assert.deepEqual(commandRequest["x-platform-definition"].additionalPermissions, []);

    const stringSet = openapi.components.schemas.AdminServiceObjectStringSet;
    assert.equal(stringSet.type, "array");
    assert.equal(stringSet.uniqueItems, true);
    assert.equal(stringSet.maxItems, 2000);
    assert.equal(stringSet.items.type, "string");

    const menuDefinition = openapi.components.schemas.AdminServiceObjectMenuDefinition;
    assert.equal(menuDefinition.additionalProperties, false);
    assert.ok(menuDefinition.required.includes("node"));
    assert.ok(menuDefinition.required.includes("buttons"));
    assert.equal(menuDefinition.properties.node.$ref, "#/components/schemas/AdminServiceObjectMenuNode");
    assert.equal(menuDefinition.properties.buttons.items.$ref, "#/components/schemas/AdminServiceObjectPageButton");
    const menuNode = openapi.components.schemas.AdminServiceObjectMenuNode;
    assert.equal(menuNode.properties.parameters.maxItems, 32);
    assert.equal(menuNode.properties.parameters.items.$ref, "#/components/schemas/AdminServiceObjectMenuParameter");
    assert.equal(menuNode.properties.externalUrl.oneOf[1].pattern, "^https://");
    assert.match(menuNode.properties.route.oneOf[1].pattern, /\[\^\{\}\*:\]/);
    assert.equal(openapi.components.schemas.AdminServiceObjectMenuParameter.oneOf.length, 3);

    const queryArguments = openapi.components.schemas[queryRequest.properties.arguments.$ref.split("/").pop()];
    const commandArguments = openapi.components.schemas[commandRequest.properties.arguments.$ref.split("/").pop()];
    assert.equal(queryArguments.additionalProperties, false);
    assert.equal(commandArguments.additionalProperties, false);
    assert.deepEqual(Object.keys(queryArguments.properties), queryDefinition.arguments.map((argument) => argument.name));
    assert.deepEqual(Object.keys(commandArguments.properties), commandDefinition.arguments.map((argument) => argument.name));
    for (const forbidden of forbiddenServiceObjectClientInputs) {
      assert.equal(queryArguments.properties[forbidden], undefined);
      assert.equal(commandArguments.properties[forbidden], undefined);
    }

    const queryDataRef = openapi.components.schemas.AdminServiceObjectQueryData.oneOf[0].$ref;
    const queryData = openapi.components.schemas[queryDataRef.split("/").pop()];
    const item = openapi.components.schemas[queryData.properties.items.items.$ref.split("/").pop()];
    assert.equal(item.additionalProperties, false);
    assert.deepEqual(Object.keys(item.properties), queryDefinition.result.map((field) => field.name));
    assert.equal(queryData.properties.total, undefined, "total must follow the definition exposeTotal policy");

    const prepareDefinition = adminServiceObjectDefinitions.commands.find(
      (definition) => definition.id === "platform.identity.organization-role-group-change.prepare",
    );
    const prepareRequestName = commandUnion.oneOf
      .map((entry) => entry.$ref.split("/").pop())
      .find((name) => openapi.components.schemas[name].properties.commandId.const === prepareDefinition.id);
    const prepareRequest = openapi.components.schemas[prepareRequestName];
    const prepareArguments = openapi.components.schemas[prepareRequest.properties.arguments.$ref.split("/").pop()];
    assert.deepEqual(Object.keys(prepareArguments.properties), ["orgUnitCode", "roleGroupCodes", "remediations"]);
    assert.deepEqual(prepareArguments.properties.roleGroupCodes, {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: { type: "string", maxLength: 191 },
    });
    assert.equal(prepareArguments.properties.remediations.type, "array");
    assert.equal(prepareArguments.properties.remediations.uniqueItems, true);
    assert.equal(prepareArguments.properties.remediations.maxItems, 2000);
    assert.equal(prepareArguments.properties.remediations.items.oneOf.length, 2);
    assert.ok(
      prepareArguments.properties.remediations.items.oneOf.every((schema) => schema.additionalProperties === false),
      "remediation variants must remain closed objects",
    );

    const rolePermissionDefinition = adminServiceObjectDefinitions.commands.find(
      (definition) => definition.id === "platform.authorization.role-permission-change.prepare",
    );
    const rolePermissionRequestName = commandUnion.oneOf
      .map((entry) => entry.$ref.split("/").pop())
      .find((name) => openapi.components.schemas[name].properties.commandId.const === rolePermissionDefinition.id);
    const rolePermissionRequest = openapi.components.schemas[rolePermissionRequestName];
    const rolePermissionArguments =
      openapi.components.schemas[rolePermissionRequest.properties.arguments.$ref.split("/").pop()];
    assert.deepEqual(rolePermissionArguments.properties.allowPermissionCodes, {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: { type: "string", maxLength: 191 },
    });
    assert.deepEqual(rolePermissionArguments.properties.denyPermissionCodes, {
      type: "array",
      uniqueItems: true,
      maxItems: 2000,
      items: { type: "string", maxLength: 191 },
    });
    assert.equal(rolePermissionArguments.properties.dataScope.type, "string");
    assert.equal(rolePermissionArguments.properties.dataScopeOrgCodes.type, "array");
    assert.equal(rolePermissionArguments.properties.dataScopeAreaCodes.type, "array");

    const lifecyclePrepareDefinition = adminServiceObjectDefinitions.commands.find(
      (definition) => definition.id === "platform.authorization.resource-lifecycle.prepare",
    );
    const lifecyclePrepareRequestName = commandUnion.oneOf
      .map((entry) => entry.$ref.split("/").pop())
      .find((name) => openapi.components.schemas[name].properties.commandId.const === lifecyclePrepareDefinition.id);
    const lifecyclePrepareRequest = openapi.components.schemas[lifecyclePrepareRequestName];
    const lifecyclePrepareArguments =
      openapi.components.schemas[lifecyclePrepareRequest.properties.arguments.$ref.split("/").pop()];
    assert.deepEqual(Object.keys(lifecyclePrepareArguments.properties), [
      "resource",
      "resourceCode",
      "operation",
      "retentionDays",
      "policyVersion",
      "remediations",
    ]);
    assert.deepEqual(lifecyclePrepareArguments.properties.retentionDays, {
      type: "integer",
      minimum: 1,
      maximum: 36500,
    });
    assert.deepEqual(lifecyclePrepareArguments.properties.policyVersion, {
      type: "integer",
      minimum: 1,
      maximum: 4294967295,
    });

    const lifecycleImpactDefinition = adminServiceObjectDefinitions.queries.find(
      (definition) => definition.id === "platform.authorization.resource-lifecycle.impact",
    );
    const lifecycleImpactRequestName = queryUnion.oneOf
      .map((entry) => entry.$ref.split("/").pop())
      .find((name) => openapi.components.schemas[name].properties.queryId.const === lifecycleImpactDefinition.id);
    const lifecycleImpactData = openapi.components.schemas[lifecycleImpactRequestName.replace("QueryRequest", "QueryData")];
    const lifecycleImpactItem =
      openapi.components.schemas[lifecycleImpactData.properties.items.items.$ref.split("/").pop()];
    assert.deepEqual(Object.keys(lifecycleImpactItem.properties), lifecycleImpactDefinition.result.map((field) => field.name));

    for (const applyDefinition of adminServiceObjectDefinitions.commands.filter(
      (definition) => definition.operationPhase === "apply",
    )) {
      const applyRequestName = commandUnion.oneOf
        .map((entry) => entry.$ref.split("/").pop())
        .find((name) => openapi.components.schemas[name].properties.commandId.const === applyDefinition.id);
      const applyRequest = openapi.components.schemas[applyRequestName];
      const applyArguments = openapi.components.schemas[applyRequest.properties.arguments.$ref.split("/").pop()];
      assert.deepEqual(Object.keys(applyArguments.properties), ["previewId", "expectedRevision", "impactHash"]);
      for (const forbidden of [...forbiddenServiceObjectClientInputs, "tenantCode", "roleGroupCodes", "roleCodes", "remediations"]) {
        assert.equal(applyArguments.properties[forbidden], undefined, `${applyDefinition.id} must not expose ${forbidden}`);
      }
    }
  });

  it("generates a consumable strongly typed Admin service object client from the same definitions", () => {
    const contract = runAdminResourceContract();
    const preview = runAdminCodegenPreviewForContract(contract);
    const source = runAdminServiceObjectClientForContract(contract);

    assert.equal(preview.serviceObjects.definitionSource, adminServiceObjectDefinitions.source);
    assert.equal(preview.serviceObjects.runtimeDefinitionSource, adminServiceObjectDefinitions.runtimeSource);
    assert.equal(preview.serviceObjects.runtime, "conditional");
    assert.equal(preview.serviceObjects.unavailableError, "SERVICE_OBJECT_UNAVAILABLE");
    assert.equal(preview.serviceObjects.typescriptClient, "resources/generated/admin-service-object-client.ts");
    assert.deepEqual(
      preview.serviceObjects.operations.map(({ kind, id, version, clientMethod }) => ({ kind, id, version, clientMethod })),
      [
        ...adminServiceObjectDefinitions.queries.map(({ id, version, clientMethod }) => ({ kind: "query", id, version, clientMethod })),
        ...adminServiceObjectDefinitions.commands.map(({ id, version, clientMethod }) => ({ kind: "command", id, version, clientMethod })),
      ],
    );
    assert.match(source, /class AdminServiceObjectClient/);
    assert.match(source, /import type \{ PlatformErrorBody \} from "\.\/error-sdk\/typescript\/errorContract"/);
    assert.match(source, /readonly error\?: PlatformErrorBody/);
    assert.match(source, /queryId: "platform\.reference-records\.list"/);
    assert.match(source, /commandId: "platform\.reference-records\.rename"/);
    assert.match(source, /"roleGroupCodes": ReadonlyArray<string>/);
    assert.match(source, /"allowPermissionCodes": ReadonlyArray<string>/);
    assert.match(source, /"denyPermissionCodes": ReadonlyArray<string>/);
    assert.match(source, /"dataScope": string/);
    assert.match(source, /ReadonlyArray<AdminServiceObjectRoleRemediation>/);
    assert.match(source, /export type AdminServiceObjectStringSet = ReadonlyArray<string>/);
    assert.match(source, /export type AdminServiceObjectMenuDefinition =/);
    assert.match(source, /readonly node: AdminServiceObjectMenuNode/);
    assert.match(source, /readonly parameters: ReadonlyArray<AdminServiceObjectMenuParameter>/);
    assert.match(source, /replaceOrganizationRoleGroups/);
    assert.match(source, /prepareAuthorizationResourceLifecycle/);
    assert.match(source, /applyAuthorizationResourceLifecycle/);
    assert.doesNotMatch(source, /\[key: string\]/);
    compileAdminServiceObjectClient(source);
  });

  it("generates the complete navigation query and command client surface", () => {
    const contract = runAdminResourceContract();
    const preview = runAdminCodegenPreviewForContract(contract);
    const source = runAdminServiceObjectClientForContract(contract);
    const operations = preview.serviceObjects.operations.map(({ kind, id, clientMethod }) => ({ kind, id, clientMethod }));

    for (const expected of requiredNavigationServiceObjects) {
      assert.ok(
        operations.some(
          (operation) =>
            operation.kind === expected.kind &&
            operation.id === expected.id &&
            operation.clientMethod === expected.clientMethod,
        ),
        `generated preview is missing ${expected.kind} ${expected.id}`,
      );
      assert.match(source, new RegExp(`\\b${expected.clientMethod}\\(`));
      assert.ok(source.includes(`${expected.kind}Id: "${expected.id}"`));
    }
  });

  it("projects lifecycle routes without exposing maintenance purge over HTTP", () => {
    const contract = runAdminResourceContract();
    const openapi = runAdminOpenAPIForContract(contract);
    const byCode = (code) => contract.resources.find((resource) => resource.code === code);

    assert.deepEqual(byCode("app-identities").deletion, { mode: "disabled", policyVersion: 1 });
    assert.equal(byCode("app-identities").routes.some((route) => route.method === "DELETE"), false);

    assert.deepEqual(byCode("api-tokens").deletion, {
      mode: "revoke",
      policyVersion: 1,
      retentionDays: 90,
      autoPurge: true,
    });
    assert.equal(byCode("api-tokens").routes.some((route) => route.method === "DELETE"), true);
    assert.equal(byCode("api-tokens").routes.some((route) => route.path.endsWith("/restore")), false);

    assert.deepEqual(byCode("files").deletion, {
      mode: "tombstone",
      policyVersion: 1,
      retentionDays: 30,
      autoPurge: true,
    });
    assert.equal(byCode("files").routes.some((route) => route.path === "/api/admin/resources/files/:id/restore"), true);
    assert.equal(contract.routes.some((route) => route.path.includes("/purge")), false);

    assert.equal(openapi.paths["/api/admin/resources/app-identities/{id}"].delete, undefined);
    assert.equal(openapi.paths["/api/admin/resources/api-tokens/{id}"].delete["x-platform-permission"], "admin:api-token:delete");
    assert.equal(openapi.paths["/api/admin/resources/files/{id}/restore"].post.operationId, "restoreFilesById");
    assert.equal(openapi.paths["/api/admin/resources/files/{id}/restore"].post["x-platform-permission"], "admin:file:restore");
    assert.equal(Object.keys(openapi.paths).some((route) => route.includes("/purge")), false);
  });

  it("keeps optional policy-review routes out of the default generated contract", () => {
    const contract = runAdminResourceContract();
    const openapi = runAdminOpenAPIForContract(contract);

    assert.ok(!contract.resources.some((resource) => resource.name === "policy-reviews"));
    assert.ok(!contract.routes.some((route) => route.path === "/api/admin/policy-reviews/:id/approve"));
    assert.equal(openapi.components.schemas.PolicyReviewRejectRequest, undefined);
    assert.equal(openapi.components.schemas.PolicyReviewExportData, undefined);
  });

  it("keeps optional personnel resources out of the default generated contract", () => {
    const contract = runAdminResourceContract();

    assert.ok(!contract.resources.some((resource) => resource.name === "personnel-profiles"));
    assert.ok(!contract.resources.some((resource) => resource.name === "positions"));
    assert.ok(!contract.resources.some((resource) => resource.name === "position-assignments"));
  });

  it("keeps optional app phone resources out of the default generated contract", () => {
    const contract = runAdminResourceContract();

    assert.ok(!contract.resources.some((resource) => resource.name === "app-phone-verifications"));
    assert.ok(!contract.resources.some((resource) => resource.name === "app-phone-bindings"));
    assert.ok(!contract.permissions.includes("admin:app-phone-verification:read"));
    assert.ok(!contract.permissions.includes("admin:app-phone-binding:read"));
  });

  it("adds app phone resources when app-ready profile capabilities are enabled", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,app-phone,dictionary,parameter,file-storage,admin-shell,demo-data,system-admin",
    });

    const verifications = contract.resources.find((resource) => resource.name === "app-phone-verifications");
    const bindings = contract.resources.find((resource) => resource.name === "app-phone-bindings");

    assert.ok(verifications, "expected app-phone-verifications resource");
    assert.ok(bindings, "expected app-phone-bindings resource");
    assert.equal(verifications.permissions.read, "admin:app-phone-verification:read");
    assert.equal(bindings.permissions.read, "admin:app-phone-binding:read");
  });

  it("preserves field security policy through contract and OpenAPI generation", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,app-phone,dictionary,parameter,file-storage,admin-shell,demo-data,system-admin",
    });
    const verification = contract.schemas["app-phone-verifications"].fields.find((field) => field.key === "codeHash");
    assert.deepEqual(
      {
        sensitivity: verification.sensitivity,
        storageMode: verification.storageMode,
        responseMode: verification.responseMode,
        exportMode: verification.exportMode,
      },
      { sensitivity: "secret", storageMode: "hashed", responseMode: "omitted", exportMode: "omitted" },
    );

    const openapi = runAdminOpenAPIForContract(contract);
    const property = openapi.components.schemas.AppPhoneVerificationsRecord.properties.codeHash;
    assert.equal(property["x-platform-sensitivity"], "secret");
    assert.equal(property["x-platform-storage-mode"], "hashed");
    assert.equal(property["x-platform-response-mode"], "omitted");
    assert.equal(property["x-platform-export-mode"], "omitted");

    const staticIdentityHash = contract.schemas.appIdentities.fields.find((field) => field.key === "providerSubjectHash");
    assert.deepEqual(
      {
        sensitivity: staticIdentityHash.sensitivity,
        storageMode: staticIdentityHash.storageMode,
        responseMode: staticIdentityHash.responseMode,
        exportMode: staticIdentityHash.exportMode,
      },
      { sensitivity: "secret", storageMode: "hashed", responseMode: "omitted", exportMode: "omitted" },
    );
    const staticIdentityProperty = openapi.components.schemas.AppIdentitiesRecord.properties.providerSubjectHash;
    assert.equal(staticIdentityProperty["x-platform-sensitivity"], "secret");
    assert.equal(staticIdentityProperty["x-platform-storage-mode"], "hashed");
    assert.equal(staticIdentityProperty["x-platform-response-mode"], "omitted");
    assert.equal(staticIdentityProperty["x-platform-export-mode"], "omitted");
  });

  it("preserves configurable resource and field protection through Admin, OpenAPI, codegen and TypeScript contracts", () => {
    const manifestPath = writeSensitiveManifest();
    const contract = runAdminResourceContract({}, ["--manifest", manifestPath]);
    const resource = contract.resources.find((candidate) => candidate.name === "custom-sensitive-records");
    assert.ok(resource, "expected custom sensitive resource");
    assert.deepEqual(resource.schema.protection, { schemaVersion: 7, scope: "tenant-field", tenantField: "tenantCode" });
    const field = resource.schema.fields.find((candidate) => candidate.key === "governmentReference");
    assert.deepEqual(field.protection, {
      format: "aes-256-gcm-v1",
      normalization: "trim-v1",
      blindIndexNamespace: "custom-government-reference",
    });
    assert.deepEqual(field.masking, {
      strategy: "partial-v1",
      preservePrefix: 2,
      preserveSuffix: 2,
      maskLength: 6,
    });

    const openapi = runAdminOpenAPIForContract(contract);
    const recordSchema = openapi.components.schemas.CustomSensitiveRecordsRecord;
    assert.deepEqual(recordSchema["x-platform-protection"], resource.schema.protection);
    assert.deepEqual(recordSchema.properties.governmentReference["x-platform-protection"], field.protection);
    assert.deepEqual(recordSchema.properties.governmentReference["x-platform-masking"], field.masking);
    assert.deepEqual(recordSchema.properties.governmentReference["x-platform-reveal"], field.reveal);
    assert.deepEqual(recordSchema.properties.governmentReference["x-platform-query-operators"], ["="]);

    const preview = runAdminCodegenPreviewForContract(contract);
    const previewResource = preview.resources.find((candidate) => candidate.resource === "custom-sensitive-records");
    assert.deepEqual(previewResource.schema.protection, resource.schema.protection);
    assert.deepEqual(previewResource.schema.protectedFields, [{ key: "governmentReference", ...field.protection }]);

    const clientSource = fs.readFileSync(path.resolve(import.meta.dirname, "..", "admin", "src", "platform", "api", "client.ts"), "utf8");
    assert.match(clientSource, /export type AdminResourceFieldProtection/);
    assert.match(clientSource, /export type AdminResourceFieldMasking/);
    assert.match(clientSource, /export type AdminResourceProtection/);
    assert.match(clientSource, /protection\?: AdminResourceFieldProtection/);
    assert.match(clientSource, /masking\?: AdminResourceFieldMasking/);
    assert.match(clientSource, /protection\?: AdminResourceProtection/);
  });

  it("always documents strict generic sensitive field reveal routes", () => {
    const openapi = runAdminOpenAPIForContract(runAdminResourceContract());
    const routeCases = [
      ["/api/admin/resources/{resource}/{id}/fields/{field}/reveal-policy", "get", "getAdminSensitiveRevealPolicy", undefined],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal/challenges",
        "post",
        "createAdminSensitiveRevealChallenge",
        "AdminSensitiveRevealChallengeRequest",
      ],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal/challenges/{challenge}/factors/oidc/start",
        "post",
        "startAdminSensitiveRevealOIDC",
        "AdminSensitiveRevealOIDCStartRequest",
      ],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal/challenges/{challenge}/factors/oidc/complete",
        "post",
        "completeAdminSensitiveRevealOIDC",
        "AdminSensitiveRevealOIDCCompleteRequest",
      ],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal/challenges/{challenge}/factors/sms/start",
        "post",
        "startAdminSensitiveRevealSMS",
        "AdminSensitiveRevealSMSStartRequest",
      ],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal/challenges/{challenge}/factors/sms/complete",
        "post",
        "completeAdminSensitiveRevealSMS",
        "AdminSensitiveRevealSMSCompleteRequest",
      ],
      [
        "/api/admin/resources/{resource}/{id}/fields/{field}/reveal",
        "post",
        "revealAdminSensitiveField",
        "AdminSensitiveRevealRequest",
      ],
    ];

    for (const [routePath, method, operationId, requestSchema] of routeCases) {
      const operation = openapi.paths[routePath]?.[method];
      assert.ok(operation, `expected ${method.toUpperCase()} ${routePath}`);
      assert.equal(operation.operationId, operationId);
      if (requestSchema) {
        assert.equal(operation.requestBody.content["application/json"].schema.$ref, `#/components/schemas/${requestSchema}`);
        assert.equal(openapi.components.schemas[requestSchema].additionalProperties, false);
      } else {
        assert.equal(operation.requestBody, undefined);
      }
      assert.ok(operation.responses["503"], `${operationId} must document unavailable runtime responses`);
    }

    for (const operationId of [
      "startAdminSensitiveRevealOIDC",
      "completeAdminSensitiveRevealOIDC",
      "startAdminSensitiveRevealSMS",
      "completeAdminSensitiveRevealSMS",
    ]) {
      const operation = Object.values(openapi.paths)
        .flatMap((pathItem) => Object.values(pathItem))
        .find((candidate) => candidate.operationId === operationId);
      for (const status of ["409", "410", "429", "503"]) {
        assert.ok(operation.responses[status], `${operationId} must document ${status}`);
      }
    }
    for (const operationId of ["completeAdminSensitiveRevealOIDC", "startAdminSensitiveRevealSMS"]) {
      const operation = Object.values(openapi.paths)
        .flatMap((pathItem) => Object.values(pathItem))
        .find((candidate) => candidate.operationId === operationId);
      assert.ok(operation.responses["502"], `${operationId} must document upstream failures`);
    }
    for (const operationId of ["completeAdminSensitiveRevealOIDC", "completeAdminSensitiveRevealSMS"]) {
      const operation = Object.values(openapi.paths)
        .flatMap((pathItem) => Object.values(pathItem))
        .find((candidate) => candidate.operationId === operationId);
      assert.equal(operation.responses["422"].$ref, "#/components/responses/UnprocessableEntity");
    }
    for (const operationId of ["startAdminSensitiveRevealOIDC", "startAdminSensitiveRevealSMS"]) {
      const operation = Object.values(openapi.paths)
        .flatMap((pathItem) => Object.values(pathItem))
        .find((candidate) => candidate.operationId === operationId);
      assert.equal(operation.responses["422"], undefined, `${operationId} must not document verification failure before completion`);
    }
    assert.ok(openapi.components.responses.UnprocessableEntity);

    const schemas = openapi.components.schemas;
    assert.deepEqual(schemas.AdminSensitiveRevealPolicyData.properties.purposes.items, {
      $ref: "#/components/schemas/AdminSensitiveRevealPurpose",
    });
    assert.deepEqual(schemas.AdminSensitiveRevealPolicyData.properties.factors.items, {
      $ref: "#/components/schemas/AdminSensitiveRevealFactor",
    });
    assert.deepEqual(schemas.AdminSensitiveRevealFactor.properties.providers.items, {
      $ref: "#/components/schemas/AdminSensitiveRevealProvider",
    });
    assert.equal(schemas.AdminSensitiveRevealPolicyData.properties.grantToken, undefined);
    assert.equal(schemas.AdminSensitiveRevealOIDCStartData.properties.grantToken, undefined);
    assert.equal(schemas.AdminSensitiveRevealSMSStartData.properties.grantToken, undefined);
    assert.ok(schemas.AdminSensitiveRevealFactorCompleteData.properties.grantToken);

    const revealResponseSchemas = [
      "AdminSensitiveRevealPolicyData",
      "AdminSensitiveRevealChallengeData",
      "AdminSensitiveRevealOIDCStartData",
      "AdminSensitiveRevealSMSStartData",
      "AdminSensitiveRevealFactorCompleteData",
      "AdminSensitiveRevealValueData",
    ];
    assert.deepEqual(
      revealResponseSchemas.filter((schemaName) => Object.hasOwn(schemas[schemaName].properties, "value")),
      ["AdminSensitiveRevealValueData"],
    );
  });

  it("keeps optional notification resources out of the default generated contract", () => {
    const contract = runAdminResourceContract();

    assert.ok(!contract.resources.some((resource) => resource.name === "notification-templates"));
    assert.ok(!contract.resources.some((resource) => resource.name === "notifications"));
    assert.ok(!contract.resources.some((resource) => resource.name === "notification-deliveries"));
  });

  it("keeps optional job resources out of the default generated contract", () => {
    const contract = runAdminResourceContract();

    assert.ok(!contract.resources.some((resource) => resource.name === "job-definitions"));
    assert.ok(!contract.resources.some((resource) => resource.name === "job-runs"));
    assert.ok(!contract.resources.some((resource) => resource.name === "job-run-attempts"));
  });

  it("keeps default capability resources backed by usable schema fields", () => {
    const contract = runAdminResourceContract();

    for (const resourceName of ["dictionary-parameters", "monitoring"]) {
      const schema = contract.schemas[resourceName];
      assert.ok(schema, `expected ${resourceName} schema`);
      assert.ok(schema.fields.length > 0, `expected ${resourceName} fields`);
      assert.ok(schema.table.length > 0, `expected ${resourceName} table fields`);
      assert.ok(schema.search.length > 0, `expected ${resourceName} search fields`);
      assert.ok(schema.filter.length > 0, `expected ${resourceName} filter fields`);
      assert.ok(schema.sort.length > 0, `expected ${resourceName} sort fields`);
    }
  });

  it("uses capability-owned schema fields for overlapping read-only seeded resources", () => {
    const contract = runAdminResourceContract();
    const permissionResource = contract.resources.find((resource) => resource.name === "permissions");
    const expectedFields = [
      "action",
      "buttonKey",
      "capability",
      "code",
      "description",
      "menuCode",
      "name",
      "prefix",
      "resource",
      "resourceType",
      "status",
    ];

    assert.ok(permissionResource, "expected permissions resource");
    assert.deepEqual(permissionResource.schema.fields.map((field) => field.key), expectedFields);
    assert.deepEqual(contract.schemas.permissions.fields.map((field) => field.key), expectedFields);
    assert.equal(permissionResource.schema.fields.some((field) => field.key === "module"), false);

    const openapi = runAdminOpenAPIForContract(contract);
    assert.deepEqual(
      Object.keys(openapi.components.schemas.PermissionsRecord.properties).filter((key) => expectedFields.includes(key)).sort(),
      expectedFields,
    );
    assert.equal(openapi.components.schemas.PermissionsRecord.properties.module, undefined);

    const preview = runAdminCodegenPreviewForContract(contract);
    const permissionPreview = preview.resources.find((resource) => resource.resource === "permissions");
    assert.equal(permissionPreview.schema.fieldCount, expectedFields.length);
    assert.deepEqual(permissionPreview.schema.table, ["code", "name", "resourceType", "capability", "resource", "action", "prefix", "status"]);
  });

  it("adds policy-review approve route when enterprise governance is enabled", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,policy-review,wechat-login,dictionary,parameter,file-storage,admin-shell,demo-data,system-admin",
    });

    const policyReviews = contract.resources.find((resource) => resource.name === "policy-reviews");
    assert.ok(policyReviews, "expected enterprise governance to include policy-reviews");
    assert.ok(policyReviews.permissionCodes.includes("admin:policy-review:export"));
    assert.ok(contract.permissions.includes("admin:policy-review:export"));
    for (const action of ["request", "approve", "reject"]) {
      assert.ok(
        policyReviews.routes.some(
          (route) =>
            route.method === "POST" &&
            route.path === `/api/admin/policy-reviews/:id/${action}` &&
            route.permission === "admin:policy-review:update" &&
            route.auditAction === `policy-review.${action}`,
        ),
        `expected policy-reviews ${action} route`,
      );
    }
    assert.ok(
      policyReviews.routes.some(
        (route) =>
          route.method === "GET" &&
          route.path === "/api/admin/policy-reviews/export" &&
          route.permission === "admin:policy-review:export" &&
          route.auditAction === "policy-review.export",
      ),
      "expected policy-reviews export route",
    );
    assert.ok(contract.routes.some((route) => route.path === "/api/admin/policy-reviews/:id/approve"));
    assert.ok(contract.routes.some((route) => route.path === "/api/admin/policy-reviews/:id/request"));
    assert.ok(contract.routes.some((route) => route.path === "/api/admin/policy-reviews/:id/reject"));
    assert.ok(contract.routes.some((route) => route.path === "/api/admin/policy-reviews/export"));
    const validation = validateAdminResourceContract(contract);
    assert.equal(validation.status, 0, `enterprise admin resource contract validation failed\n${validation.stdout}${validation.stderr}`);
  });

  it("rejects generated contracts whose route permissions are not declared at resource and contract level", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,policy-review,wechat-login,dictionary,parameter,file-storage,admin-shell,demo-data,system-admin",
    });
    const policyReviews = contract.resources.find((resource) => resource.name === "policy-reviews");
    policyReviews.permissionCodes = policyReviews.permissionCodes.filter((permission) => permission !== "admin:policy-review:export");

    let validation = validateAdminResourceContract(contract);
    assert.notEqual(validation.status, 0);
    assert.match(`${validation.stdout}${validation.stderr}`, /uses undeclared resource permission admin:policy-review:export/);

    policyReviews.permissionCodes.push("admin:policy-review:export");
    contract.permissions = contract.permissions.filter((permission) => permission !== "admin:policy-review:export");
    validation = validateAdminResourceContract(contract);
    assert.notEqual(validation.status, 0);
    assert.match(`${validation.stdout}${validation.stderr}`, /permission admin:policy-review:export is missing from contract.permissions/);

    contract.permissions.push("admin:policy-review:export");
    const exportRoute = policyReviews.routes.find((route) => route.path === "/api/admin/policy-reviews/export");
    delete exportRoute.permission;
    validation = validateAdminResourceContract(contract);
    assert.notEqual(validation.status, 0);
    assert.match(`${validation.stdout}${validation.stderr}`, /route GET \/api\/admin\/policy-reviews\/export must declare permission/);

    exportRoute.permission = "admin:policy-review:export";
    const resourceWithAction = contract.resources.find((resource) => resource.actions.length > 0);
    const action = resourceWithAction.actions[0];
    delete action.permission;
    validation = validateAdminResourceContract(contract);
    assert.notEqual(validation.status, 0);
    assert.match(`${validation.stdout}${validation.stderr}`, new RegExp(`action ${action.key} must declare permission`));
  });

  it("documents policy-review custom actions in enterprise OpenAPI", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,policy-review,wechat-login,dictionary,parameter,file-storage,admin-shell,demo-data,system-admin",
    });
    const openapi = runAdminOpenAPIForContract(contract);

    assert.ok(!openapi.paths["/api/admin/resources/policy-reviews/{id}/approve"]);
    const approve = openapi.paths["/api/admin/policy-reviews/{id}/approve"]?.post;
    assert.ok(approve, "expected policy review approve OpenAPI path");
    assert.equal(approve.operationId, "approvePolicyReviewsById");
    assert.equal(approve["x-platform-permission"], "admin:policy-review:update");
    assert.equal(approve["x-platform-audit-action"], "policy-review.approve");
    assert.equal(approve.requestBody, undefined);
    const request = openapi.paths["/api/admin/policy-reviews/{id}/request"]?.post;
    assert.ok(request, "expected policy review request OpenAPI path");
    assert.equal(request.operationId, "requestPolicyReviewsById");
    assert.equal(request["x-platform-permission"], "admin:policy-review:update");
    assert.equal(request["x-platform-audit-action"], "policy-review.request");
    assert.equal(request.requestBody, undefined);
    const reject = openapi.paths["/api/admin/policy-reviews/{id}/reject"]?.post;
    assert.ok(reject, "expected policy review reject OpenAPI path");
    assert.equal(reject.operationId, "rejectPolicyReviewsById");
    assert.equal(reject["x-platform-permission"], "admin:policy-review:update");
    assert.equal(reject["x-platform-audit-action"], "policy-review.reject");
    assert.ok(reject.requestBody, "expected reject reason request body");
    const exportOperation = openapi.paths["/api/admin/policy-reviews/export"]?.get;
    assert.ok(exportOperation, "expected policy review export OpenAPI path");
    assert.equal(exportOperation.operationId, "exportPolicyReviews");
    assert.equal(exportOperation["x-platform-permission"], "admin:policy-review:export");
    assert.equal(exportOperation["x-platform-audit-action"], "policy-review.export");
    assert.deepEqual(exportOperation.parameters, [
      {
        name: "watermark",
        in: "query",
        required: false,
        description: "Apply branding and export provenance watermark metadata to the JSON evidence package.",
        schema: { type: "boolean", default: false },
      },
    ]);
    const exportSchema = openapi.components.schemas.PolicyReviewExportData;
    assert.equal(exportSchema.additionalProperties, false);
    assert.deepEqual(exportSchema.required, ["exportedBy", "exportedAt", "watermark", "reviews", "audits"]);
    assert.deepEqual(exportSchema.properties.watermark, {
      $ref: "#/components/schemas/PolicyReviewExportWatermark",
    });
    const watermarkSchema = openapi.components.schemas.PolicyReviewExportWatermark;
    assert.equal(watermarkSchema.additionalProperties, false);
    assert.deepEqual(watermarkSchema.required, ["applied", "product", "exportedBy", "exportedAt"]);
    assert.deepEqual(watermarkSchema.properties, {
      applied: { type: "boolean" },
      product: { type: "string" },
      exportedBy: { type: "string" },
      exportedAt: { type: "string", format: "date-time" },
    });
  });

  it("adds personnel resources with shared ownership relations when personnel is enabled", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,dictionary,parameter,file-storage,personnel,admin-shell,demo-data,system-admin",
    });

    const personnel = contract.resources.find((resource) => resource.name === "personnel-profiles");
    const positions = contract.resources.find((resource) => resource.name === "positions");
    const assignments = contract.resources.find((resource) => resource.name === "position-assignments");

    assert.ok(personnel, "expected personnel-profiles resource");
    assert.ok(positions, "expected positions resource");
    assert.ok(assignments, "expected position-assignments resource");
    assert.equal(personnel.permissions.read, "admin:personnel-profile:read");
    assert.ok(contract.schemas["personnel-profiles"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas["personnel-profiles"].fields.some((field) => field.key === "orgUnitCode" && field.relation?.resource === "org-units"));
    assert.ok(contract.schemas["personnel-profiles"].fields.some((field) => field.key === "areaCode" && field.relation?.resource === "area-codes"));
    assert.ok(contract.schemas["personnel-profiles"].fields.some((field) => field.key === "userCode" && field.relation?.resource === "users"));
    assert.ok(contract.schemas.positions.fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas.positions.fields.some((field) => field.key === "orgUnitCode" && field.relation?.resource === "org-units"));
    assert.ok(contract.schemas["position-assignments"].fields.some((field) => field.key === "personnelCode" && field.relation?.resource === "personnel-profiles"));
    assert.ok(contract.schemas["position-assignments"].fields.some((field) => field.key === "positionCode" && field.relation?.resource === "positions"));
  });

  it("adds notification resources with shared tenant and user relations when notification is enabled", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,dictionary,parameter,file-storage,notification,admin-shell,demo-data,system-admin",
    });

    const templates = contract.resources.find((resource) => resource.name === "notification-templates");
    const notifications = contract.resources.find((resource) => resource.name === "notifications");
    const deliveries = contract.resources.find((resource) => resource.name === "notification-deliveries");

    assert.ok(templates, "expected notification-templates resource");
    assert.ok(notifications, "expected notifications resource");
    assert.ok(deliveries, "expected notification-deliveries resource");
    assert.equal(notifications.permissions.read, "admin:notification:read");
    assert.ok(contract.schemas["notification-templates"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas.notifications.fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas.notifications.fields.some((field) => field.key === "templateCode" && field.relation?.resource === "notification-templates"));
    assert.ok(contract.schemas.notifications.fields.some((field) => field.key === "recipientUserCode" && field.relation?.resource === "users"));
    assert.ok(contract.schemas["notification-deliveries"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas["notification-deliveries"].fields.some((field) => field.key === "notificationCode" && field.relation?.resource === "notifications"));
    assert.ok(contract.schemas["notification-deliveries"].fields.some((field) => field.key === "recipientUserCode" && field.relation?.resource === "users"));
  });

  it("adds job resources with shared tenant and user relations when job is enabled", () => {
    const contract = runAdminResourceContract({
      PLATFORM_CAPABILITIES:
        "tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,dictionary,parameter,file-storage,job,admin-shell,demo-data,system-admin",
    });

    const definitions = contract.resources.find((resource) => resource.name === "job-definitions");
    const runs = contract.resources.find((resource) => resource.name === "job-runs");
    const attempts = contract.resources.find((resource) => resource.name === "job-run-attempts");

    assert.ok(definitions, "expected job-definitions resource");
    assert.ok(runs, "expected job-runs resource");
    assert.ok(attempts, "expected job-run-attempts resource");
    assert.equal(definitions.permissions.read, "admin:job-definition:read");
    assert.ok(contract.schemas["job-definitions"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas["job-definitions"].fields.some((field) => field.key === "ownerUserCode" && field.relation?.resource === "users"));
    assert.ok(contract.schemas["job-runs"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas["job-runs"].fields.some((field) => field.key === "jobCode" && field.relation?.resource === "job-definitions"));
    assert.ok(contract.schemas["job-runs"].fields.some((field) => field.key === "triggeredBy" && field.relation?.resource === "users"));
    assert.ok(contract.schemas["job-run-attempts"].fields.some((field) => field.key === "tenantCode" && field.relation?.resource === "tenants"));
    assert.ok(contract.schemas["job-run-attempts"].fields.some((field) => field.key === "runCode" && field.relation?.resource === "job-runs"));
  });
});
