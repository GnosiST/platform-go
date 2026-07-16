import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-service-contract-standard.mjs", ...args], { cwd: repoRoot, encoding: "utf8" });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-contract-"));
  const file = path.join(dir, name);
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
  return file;
}

function runGenerator(contract) {
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-artifacts-"));
  const result = spawnSync(process.execPath, ["scripts/generate-platform-service-contract-artifacts.mjs", "--contract", tempJSON("contract.json", contract), "--output-dir", outputDir], { cwd: repoRoot, encoding: "utf8" });
  return { outputDir, result };
}

function assertOpenAPIErrorContract(openapi) {
  const registry = readJSON("resources/generated/platform-error-code-contract.json");
  assert.equal(openapi["x-platform-error-registry-source"], "resources/generated/platform-error-code-contract.json");
  assert.equal(openapi["x-platform-error-registry-hash"], registry.contractHash);
  assert.deepEqual(openapi.components.schemas.PlatformErrorCode.enum, registry.definitions.map((definition) => definition.code));
  assert.deepEqual(openapi.components.schemas.ErrorBody.required, ["code", "message", "requestId", "traceId"]);
  assert.equal(openapi.components.schemas.ErrorBody.additionalProperties, false);
  assert.equal(openapi.components.schemas.ErrorBody.properties.code.$ref, "#/components/schemas/PlatformErrorCode");
  assert.equal(openapi.components.schemas.ErrorBody.properties.requestId.pattern, "^req_[0-9a-f]{32}$");
  assert.equal(openapi.components.schemas.ErrorBody.properties.traceId.pattern, "^[0-9a-f]{32}$");
  assert.deepEqual(openapi.components.schemas.ErrorResponse, {
    type: "object",
    required: ["error"],
    properties: { error: { $ref: "#/components/schemas/ErrorBody" } },
    additionalProperties: false,
  });
}

describe("platform service contract standard", () => {
  it("accepts the generated standard and governance closeout", () => {
    const result = runValidator();
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform service contract standard/);
  });

  it("rejects client-selected physical routing", () => {
    const contract = readJSON("resources/generated/platform-service-contract.json");
    contract.tenantContext.clientPhysicalRoutingSelectable = true;
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /must forbid client physical routing selection/);
  });

  it("rejects same-major stable operation removal", () => {
    const contract = readJSON("resources/generated/platform-service-contract.json");
    const service = contract.services.find((item) => item.id === "file-storage");
    service.operations = service.operations.filter((operation) => operation.id !== "upload-file");
    const result = runValidator(["--contract", tempJSON("contract.json", contract)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /stable operation file-storage\.upload-file must not be removed within the same major version/);
  });

  it("rejects stable service version downgrade", () => {
    const baseline = readJSON("resources/fixtures/platform-service-contract/compatibility-baseline.json");
    baseline.services.find((item) => item.id === "file-storage").version = "1.1.0";
    const result = runValidator(["--baseline", tempJSON("baseline.json", baseline)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /stable service file-storage version must not be downgraded/);
  });

  it("rejects same-major stable service classification and identity drift", () => {
    const mutations = [
      ["stability", (service) => { service.stability = "experimental"; }, /stable service file-storage stability must not change/],
      ["audiences", (service) => { service.audiences = ["internal"]; }, /stable service file-storage audiences must not change/],
      ["identity modes", (service) => { service.identityModes = ["workload"]; }, /stable service file-storage identityModes must not change/],
      ["auth modes", (service) => { service.authModes = ["workload-jwt"]; }, /stable service file-storage authModes must not change/],
    ];
    for (const [label, mutate, expected] of mutations) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      mutate(contract.services.find((item) => item.id === "file-storage"));
      const result = runValidator(["--contract", tempJSON(`contract-service-${label.replaceAll(" ", "-")}.json`, contract)]);
      assert.notEqual(result.status, 0, label);
      assert.match(result.stderr, expected, label);
    }
  });

  it("rejects same-major HTTP path, media type, and schema drift", () => {
    const mutations = [
      ["path", (operation) => { operation.path = "/api/app/uploads"; }, /upload-file path must not change/],
      ["request media type", (operation) => { operation.requestMediaType = "application/json"; }, /upload-file requestMediaType must not change/],
      ["response media type", (operation) => { operation.responseMediaType = "application/octet-stream"; }, /upload-file responseMediaType must not change/],
      ["success status", (operation) => { operation.successStatus = 200; }, /upload-file successStatus must not change/],
      ["identity mode", (operation) => { operation.identityMode = "workload"; }, /upload-file identityMode must not change/],
      ["request schema ref", (operation) => { operation.requestSchema.ref = "#\/schemas/OtherUploadRequest"; }, /upload-file requestSchema\.ref must not change/],
      ["response schema PII", (operation) => { operation.responseSchema.pii = "sensitive"; }, /upload-file responseSchema\.pii must not change/],
    ];
    for (const [label, mutate, expected] of mutations) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      const service = contract.services.find((item) => item.id === "file-storage");
      mutate(service.operations.find((item) => item.id === "upload-file"));
      const result = runValidator(["--contract", tempJSON(`contract-${label.replaceAll(" ", "-")}.json`, contract)]);
      assert.notEqual(result.status, 0, label);
      assert.match(result.stderr, expected, label);
    }
  });

  it("rejects required field removal and addition within the same major version", () => {
    for (const [requiredFields, expected] of [
      [[], /upload-file requestSchema must not remove required fields/],
      [["file", "checksum"], /upload-file requestSchema must not add required fields/],
    ]) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      const service = contract.services.find((item) => item.id === "file-storage");
      service.operations.find((item) => item.id === "upload-file").requestSchema.requiredFields = requiredFields;
      const result = runValidator(["--contract", tempJSON("contract-required-fields.json", contract)]);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, expected);
    }
  });

  it("rejects same-major reliability semantic drift", () => {
    const mutations = [
      ["idempotency", "store-file", (reliability) => { reliability.idempotency = "none"; }, /store-file reliability\.idempotency must not change/],
      ["timeout", "upload-file", (reliability) => { reliability.timeoutMilliseconds = 10000; }, /upload-file reliability\.timeoutMilliseconds must not change/],
      ["cost", "upload-file", (reliability) => { reliability.costLimit = 20; }, /upload-file reliability\.costLimit must not change/],
    ];
    for (const [label, operationID, mutate, expected] of mutations) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      const service = contract.services.find((item) => item.id === "file-storage");
      mutate(service.operations.find((item) => item.id === operationID).reliability);
      const result = runValidator(["--contract", tempJSON(`contract-reliability-${label}.json`, contract)]);
      assert.notEqual(result.status, 0, label);
      assert.match(result.stderr, expected, label);
    }
  });

  it("rejects same-major event version and payload schema drift", () => {
    const mutations = [
      ["version", (event) => { event.version = 2; }, /file-stored version must not change/],
      ["payload ref", (event) => { event.payloadSchema.ref = "#\/schemas/OtherStoredEvent"; }, /file-stored payloadSchema\.ref must not change/],
      ["payload PII", (event) => { event.payloadSchema.pii = "sensitive"; }, /file-stored payloadSchema\.pii must not change/],
    ];
    for (const [label, mutate, expected] of mutations) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      const service = contract.services.find((item) => item.id === "file-storage");
      mutate(service.events.find((item) => item.id === "file-stored"));
      const result = runValidator(["--contract", tempJSON(`contract-event-${label.replaceAll(" ", "-")}.json`, contract)]);
      assert.notEqual(result.status, 0, label);
      assert.match(result.stderr, expected, label);
    }
  });

  it("rejects Event Plane delivery overstatement", () => {
    const asyncapi = readJSON("resources/generated/asyncapi.events.json");
    asyncapi.channels["platform.file-storage.file-stored.v1"]["x-platform-runtime-status"] = "bound";
    const result = runValidator(["--asyncapi", tempJSON("asyncapi.json", asyncapi)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /AsyncAPI channels must remain contract-only/);
  });

  it("requires the negative consumer fixture to prove routing rejection", () => {
    const fixture = readJSON("resources/fixtures/platform-service-contract/rejected-physical-routing-consumer.json");
    fixture.physicalRouting = null;
    fixture.tenantContextSource = "authenticated-identity";
    const result = runValidator(["--negative-consumer", tempJSON("consumer.json", fixture)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /negative consumer fixture must prove physical routing rejection/);
  });

  it("generates the bound file API with its real status codes and response shapes", () => {
    const openapi = readJSON("resources/generated/openapi.external.json");
    const upload = openapi.paths["/api/app/files"].post;
    assert.ok(upload.responses["201"]);
    assert.equal(upload.responses["200"], undefined);
    assert.equal(upload.responses["201"].content["application/json"].schema.$ref, "#/components/schemas/AppFileUploadResponse");

    const uploadResponse = openapi.components.schemas.AppFileUploadResponse;
    assert.deepEqual(uploadResponse.required, ["data"]);
    assert.equal(uploadResponse.additionalProperties, false);
    assert.deepEqual(uploadResponse.properties.data.required, ["resource", "record"]);
    assert.equal(uploadResponse.properties.data.additionalProperties, false);
    assert.equal(uploadResponse.properties.data.properties.resource.const, "files");
    assert.equal(uploadResponse.properties.data.properties.record.$ref, "#/components/schemas/AdminResourceRecord");

    const content = openapi.paths["/api/app/files/{id}/content"].get;
    assert.deepEqual(content.responses["200"].content["application/octet-stream"].schema, { type: "string", format: "binary" });
  });

  it("closes request payloads and explicitly rejects physical routing fields", () => {
    const forbidden = ["database", "datasource", "dsn", "schema", "shard"];
    for (const relativePath of ["resources/generated/openapi.service.json", "resources/generated/openapi.control.json", "resources/generated/openapi.external.json"]) {
      const openapi = readJSON(relativePath);
      for (const [name, schema] of Object.entries(openapi.components.schemas)) {
        if (!schema["x-platform-forbidden-client-fields"]) continue;
        assert.equal(schema.additionalProperties, false, `${relativePath} ${name}`);
        assert.deepEqual(schema["x-platform-forbidden-client-fields"], forbidden, `${relativePath} ${name}`);
        assert.deepEqual(schema.not.anyOf, forbidden.map((field) => ({ required: [field] })), `${relativePath} ${name}`);
      }
    }
    const external = readJSON("resources/generated/openapi.external.json");
    assert.equal(external.components.schemas.AppFileUploadRequest.additionalProperties, false);
    assert.deepEqual(external.components.schemas.AppFileUploadRequest["x-platform-forbidden-client-fields"], forbidden);
  });

  it("expresses the versioned event envelope and business payload in AsyncAPI", () => {
    const asyncapi = readJSON("resources/generated/asyncapi.events.json");
    const message = asyncapi.components.messages.file_storage_file_stored_v1;
    assert.equal(message.payload.$ref, "#/components/schemas/FileStorageFileStoredEventEnvelopeV1");
    assert.deepEqual(message.headers.required, ["traceparent"]);
    assert.equal(message.headers.additionalProperties, false);

    const envelope = asyncapi.components.schemas.FileStorageFileStoredEventEnvelopeV1;
    assert.deepEqual(envelope.required, ["eventId", "eventType", "eventVersion", "occurredAt", "producer", "tenantContext", "traceContext", "payload"]);
    assert.equal(envelope.additionalProperties, false);
    assert.equal(envelope.properties.eventType.const, "platform.file-storage.file-stored.v1");
    assert.equal(envelope.properties.eventVersion.const, 1);
    assert.equal(envelope.properties.payload.$ref, "#/components/schemas/FileStoredEvent");
    assert.equal(envelope.properties.tenantContext.additionalProperties, false);
    assert.equal(envelope.properties.traceContext.additionalProperties, false);

    const businessPayload = asyncapi.components.schemas.FileStoredEvent;
    assert.deepEqual(businessPayload.required, ["fileId"]);
    assert.equal(businessPayload.additionalProperties, false);
  });

  it("generates deterministic API artifacts and compilable isolated SDKs", () => {
    const first = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-artifacts-a-"));
    const second = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-artifacts-b-"));
    for (const outputDir of [first, second]) {
      const result = spawnSync(process.execPath, ["scripts/generate-platform-service-contract-artifacts.mjs", "--output-dir", outputDir], { cwd: repoRoot, encoding: "utf8" });
      assert.equal(result.status, 0, result.stderr);
    }
    for (const relativePath of ["openapi.service.json", "openapi.control.json", "openapi.external.json", "asyncapi.events.json", "service-sdk/go/service_contract_sdk.go", "service-sdk/typescript/serviceContractSDK.ts"]) {
      assert.equal(fs.readFileSync(path.join(first, relativePath), "utf8"), fs.readFileSync(path.join(second, relativePath), "utf8"), relativePath);
    }
    for (const relativePath of ["openapi.service.json", "openapi.control.json", "openapi.external.json"]) {
      assertOpenAPIErrorContract(JSON.parse(fs.readFileSync(path.join(first, relativePath), "utf8")));
    }
    assert.doesNotMatch(fs.readFileSync(path.join(first, "asyncapi.events.json"), "utf8"), /PlatformErrorCode|ErrorResponse/);

    const goDir = path.join(first, "service-sdk", "go");
    fs.writeFileSync(path.join(goDir, "go.mod"), "module platform-service-contract-sdk\n\ngo 1.26\n");
    fs.writeFileSync(path.join(goDir, "service_contract_sdk_test.go"), "package servicecontractsdk\n\nimport \"testing\"\n\nfunc TestGeneratedContractConstants(t *testing.T) { if OperationUploadFile == \"\" || EventFileStoredV1 == \"\" { t.Fatal(\"missing generated constants\") }; if definition, ok := LookupError(CodeInternalError); !ok || definition.Code != CodeInternalError { t.Fatal(\"missing generated error definition\") } }\n");
    const goResult = spawnSync("go", ["test", "./..."], { cwd: goDir, encoding: "utf8" });
    assert.equal(goResult.status, 0, goResult.stderr);

    const tsc = path.join(repoRoot, "admin", "node_modules", ".bin", "tsc");
    const tsResult = spawnSync(tsc, ["--noEmit", "--target", "ES2022", "--module", "ESNext", path.join(first, "service-sdk", "typescript", "serviceContractSDK.ts")], { cwd: repoRoot, encoding: "utf8" });
    assert.equal(tsResult.status, 0, tsResult.stderr || tsResult.stdout);
    const typeScriptSDK = fs.readFileSync(path.join(first, "service-sdk", "typescript", "serviceContractSDK.ts"), "utf8");
    assert.match(typeScriptSDK, /export type PlatformErrorBody/);
    assert.match(typeScriptSDK, /export const platformErrorDefinitions/);
  });

  it("rejects unknown explicit OpenAPI error codes", () => {
    const external = readJSON("resources/generated/openapi.external.json");
    external.paths["/api/app/files"].post.responses["400"]["x-platform-error-codes"] = ["UNKNOWN_PLATFORM_ERROR"];
    const result = runValidator(["--external-openapi", tempJSON("openapi.external.json", external)]);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /unknown platform error code UNKNOWN_PLATFORM_ERROR/);
  });

  it("rejects generated identifier and object-key collisions before writing artifacts", () => {
    const cases = [
      ["Go operation constant", (contract) => {
        const service = contract.services.find((item) => item.id === "file-storage");
        const template = service.operations.find((item) => item.id === "upload-file");
        service.operations.push({ ...structuredClone(template), id: "file-1", path: "/api/app/files/a" }, { ...structuredClone(template), id: "file1", path: "/api/app/files/b" });
      }, /Go operation constant collision for "OperationFile1"/],
      ["Go event constant", (contract) => {
        const service = contract.services.find((item) => item.id === "file-storage");
        const template = service.events[0];
        service.events.push({ ...structuredClone(template), id: "file-1", name: "platform.file-storage.a.v1" }, { ...structuredClone(template), id: "file1", name: "platform.file-storage.b.v1" });
      }, /Go event constant collision for "EventFile1V1"/],
      ["TypeScript operation key", (contract) => {
        const source = contract.services.find((item) => item.id === "file-storage");
        const operation = source.operations.find((item) => item.id === "upload-file");
        contract.services.push({ ...structuredClone(source), id: "file-storage-copy", operations: [{ ...structuredClone(operation), path: "/api/app/copied-files" }], events: [] });
      }, /TypeScript operation key collision for "upload-file"/],
      ["TypeScript event key", (contract) => {
        const service = contract.services.find((item) => item.id === "file-storage");
        const template = service.events[0];
        service.events.push({ ...structuredClone(template), id: "stored-copy", name: template.name });
      }, /TypeScript event key collision/],
    ];
    for (const [label, mutate, expected] of cases) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      mutate(contract);
      const { outputDir, result } = runGenerator(contract);
      assert.notEqual(result.status, 0, label);
      assert.match(result.stderr, expected, label);
      assert.deepEqual(fs.readdirSync(outputDir), [], `${label} must fail before writing artifacts`);
    }
  });

  it("reuses identical schema refs and rejects conflicting definitions", () => {
    const compatible = readJSON("resources/generated/platform-service-contract.json");
    const compatibleService = compatible.services.find((item) => item.id === "file-storage");
    const uploadOperation = compatibleService.operations.find((item) => item.id === "upload-file");
    compatibleService.operations.push({ ...structuredClone(uploadOperation), id: "upload-file-copy", path: "/api/app/copied-files" });
    const compatibleGeneration = runGenerator(compatible);
    assert.equal(compatibleGeneration.result.status, 0, compatibleGeneration.result.stderr);

    const conflicting = structuredClone(compatible);
    conflicting.services.find((item) => item.id === "file-storage").operations.find((item) => item.id === "upload-file-copy").requestSchema.requiredFields = ["file", "checksum"];
    const conflictingGeneration = runGenerator(conflicting);
    assert.notEqual(conflictingGeneration.result.status, 0);
    assert.match(conflictingGeneration.result.stderr, /Schema definition collision for AppFileUploadRequest/);
    assert.deepEqual(fs.readdirSync(conflictingGeneration.outputDir), [], "schema collision must fail before writing artifacts");
  });

  it("requires tenant context in event envelopes only for required tenant mode", () => {
    for (const [tenantMode, expectedRequired] of [["required", true], ["optional", false], ["none", false], ["platform", false]]) {
      const contract = readJSON("resources/generated/platform-service-contract.json");
      contract.services.find((item) => item.id === "file-storage").events[0].tenantMode = tenantMode;
      const { outputDir, result } = runGenerator(contract);
      assert.equal(result.status, 0, `${tenantMode}: ${result.stderr}`);
      const asyncapi = JSON.parse(fs.readFileSync(path.join(outputDir, "asyncapi.events.json"), "utf8"));
      const envelope = asyncapi.components.schemas.FileStorageFileStoredEventEnvelopeV1;
      assert.equal(envelope.required.includes("tenantContext"), expectedRequired, tenantMode);
      assert.ok(envelope.properties.tenantContext, `${tenantMode} must retain the tenantContext property`);
      assert.match(fs.readFileSync(path.join(outputDir, "service-sdk/go/service_contract_sdk.go"), "utf8"), /TenantContext \*TenantContext `json:"tenantContext,omitempty"`/);
      assert.match(fs.readFileSync(path.join(outputDir, "service-sdk/typescript/serviceContractSDK.ts"), "utf8"), /tenantContext\?: TenantContext/);
    }
  });
});
