import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  isForbiddenMenuParameterKey,
  isForbiddenMenuParameterStringValue,
  isSafeInternalMenuRoute,
} from "../admin/src/platform/resources/menuGovernanceValidation.ts";

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
});
