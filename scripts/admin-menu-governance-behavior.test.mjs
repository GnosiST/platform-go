import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  isForbiddenMenuParameterKey,
  isForbiddenMenuParameterStringValue,
  isSafeInternalMenuRoute,
} from "../admin/src/platform/resources/menuGovernanceValidation.ts";
import {
  legacyMenuUpdateInput,
  projectMenuGovernanceRecords,
  resolveMenuGovernanceWriteMode,
} from "../admin/src/platform/resources/menuGovernanceRuntime.ts";

describe("menu governance validation behavior", () => {
  it("matches the backend forbidden parameter keys", () => {
    for (const key of [
      "datasource",
      "shard",
      "database",
      "schema",
      "sql",
      "script",
      "expression",
      "route-template",
      "physical-database",
      "physical-schema",
      "physical-routing",
    ]) {
      assert.equal(isForbiddenMenuParameterKey(key), true, key);
      assert.equal(isForbiddenMenuParameterKey(key.toUpperCase()), true, key);
    }

    for (const key of ["mode", "schemaVersion", "routeTemplateVersion"]) {
      assert.equal(isForbiddenMenuParameterKey(key), false, key);
    }
  });

  it("matches the backend forbidden static parameter values", () => {
    for (const value of [
      '<script>alert("x")</script>',
      'javascript:alert("x")',
      "vbscript:msgbox(1)",
      "data:text/html,<h1>x</h1>",
      "eval(userInput)",
      "function(run)",
      "${tenant.id}",
      "#{tenant.id}",
      "@{tenant.id}",
      "{{ currentUser.id }}",
      "SELECT * FROM users",
      "create table users",
      "truncate users",
      "merge users",
      "execute task",
      "/users/:id",
      "/users/{id}",
      "/users/*",
      "datasource=primary",
      "shard:tenant-42",
      "database=platform",
      '{"schema":"public"}',
    ]) {
      assert.equal(isForbiddenMenuParameterStringValue(value), true, value);
    }

    for (const value of ["active", "selection", "schemaVersion", "/users/profile"]) {
      assert.equal(isForbiddenMenuParameterStringValue(value), false, value);
    }
  });

  it("allows SQL-like words in literal routes but rejects route templates", () => {
    for (const route of ["/users/update", "/reports/select", "/schemas/create"]) {
      assert.equal(isSafeInternalMenuRoute(route), true, route);
    }

    for (const route of ["//example.com", "/users/:id", "/users/{id}", "/users/*", "/users?id=1", "/users#details"]) {
      assert.equal(isSafeInternalMenuRoute(route), false, route);
    }
  });

  it("projects missing legacy directories without flattening page records", () => {
    const records = projectMenuGovernanceRecords([
      {
        id: "menu-users",
        code: "users",
        name: "Users",
        status: "enabled",
        updatedAt: "2026-07-17T00:00:00Z",
        values: { parent: "identity/accounts", route: "/users", titleZh: "用户", titleEn: "Users" },
      },
    ], "legacy");

    assert.deepEqual(records.map((record) => [record.code, record.values?.parentCode, record.values?.nodeType]), [
      ["identity", "", "directory"],
      ["identity/accounts", "identity", "directory"],
      ["users", undefined, undefined],
    ]);
  });

  it("detects runtime mode from the authoritative menu schema", () => {
    const schema = (required) => ({ fields: [{ key: "nodeType", required }] });
    assert.equal(resolveMenuGovernanceWriteMode({ fields: [] }), "legacy");
    assert.equal(resolveMenuGovernanceWriteMode(schema(false)), "legacy");
    assert.equal(resolveMenuGovernanceWriteMode(schema(true)), "target");
  });

  it("preserves legacy authorization fields while updating editable menu metadata", () => {
    const record = {
      id: "menu-users",
      code: "users",
      name: "Users",
      status: "enabled",
      updatedAt: "2026-07-17T00:00:00Z",
      values: {
        parent: "identity",
        permission: "admin:user:read",
        group: "foundation",
        resource: "users",
      },
    };
    const definition = {
      id: record.id,
      name: "Users",
      description: "User management",
      updatedAt: record.updatedAt,
      node: {
        code: "users",
        parentCode: "access",
        nodeType: "page",
        titleZh: "用户管理",
        titleEn: "Users",
        descriptionZh: "用户维护",
        descriptionEn: "User management",
        status: "enabled",
        icon: "team",
        sortOrder: 20,
        route: "/users",
        componentKey: "users",
        resourceCode: "users",
        external: false,
        externalUrl: "",
        openMode: "same-tab",
        parameters: [],
        cacheEnabled: true,
        hidden: false,
        activeMenuCode: "",
        breadcrumbVisible: true,
      },
      buttons: [],
    };

    const schema = {
      fields: [
        { key: "code", source: "record", sensitivity: "public" },
        { key: "name", source: "record", sensitivity: "public" },
        { key: "status", source: "record", sensitivity: "public" },
        { key: "description", source: "record", sensitivity: "public" },
        { key: "nodeType", source: "values", sensitivity: "public" },
        { key: "parent", source: "values", sensitivity: "public" },
        { key: "parentCode", source: "values", sensitivity: "public" },
        { key: "permission", source: "values", sensitivity: "public" },
        { key: "group", source: "values", sensitivity: "public" },
        { key: "resource", source: "values", sensitivity: "public" },
        { key: "route", source: "values", sensitivity: "public" },
        { key: "pageButtons", source: "values", sensitivity: "public", readOnly: true },
      ],
    };
    const input = legacyMenuUpdateInput(record, definition, schema);
    assert.equal(input.values.permission, "admin:user:read");
    assert.equal(input.values.group, "foundation");
    assert.equal(input.values.parent, "access");
    assert.equal(input.values.parentCode, "access");
    assert.equal(input.values.nodeType, "page");
    assert.equal(input.values.route, "/users");
    assert.equal(input.values.resource, "users");
    assert.equal(Object.hasOwn(input.values, "pageButtons"), false);
    assert.equal(Object.hasOwn(input.values, "external"), false);
  });
});
