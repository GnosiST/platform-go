import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

function runAdminResourceContract(env = {}) {
  const result = spawnSync(process.execPath, ["scripts/generate-admin-resource-contract.mjs", "--stdout"], {
    cwd: new URL("..", import.meta.url),
    encoding: "utf8",
    env: { ...process.env, ...env },
  });
  assert.equal(result.status, 0, `generate-admin-resource-contract.mjs failed\n${result.stdout}${result.stderr}`);
  return JSON.parse(result.stdout);
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
