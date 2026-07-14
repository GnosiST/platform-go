import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-service-object-runtime.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempFile(name, value) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "platform-service-object-runtime-"));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, typeof value === "string" ? value : `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-service-object-runtime", () => {
  it("accepts the executable service-object runtime contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform service-object runtime governance/);
  });

  it("rejects client-selectable physical routing", () => {
    const contract = readJSON("resources/platform-service-object-runtime.json");
    contract.clientContract.forbiddenInputs = contract.clientContract.forbiddenInputs.filter((item) => item !== "datasource");
    contract.clientContract.tenantContextClientSelectable = true;

    const result = runValidator(["--contract", tempFile("physical-routing.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /forbiddenInputs must include datasource/);
    assert.match(result.stderr, /tenant and data scopes must remain server selected/);
  });

  it("rejects expanding deferred federation and XA into this node", () => {
    const contract = readJSON("resources/platform-service-object-runtime.json");
    contract.runtimeBoundary.federatedQuery = "implemented";
    contract.runtimeBoundary.xa = "enabled";

    const result = runValidator(["--contract", tempFile("expanded-runtime-boundary.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtime boundary must remain explicit and non-expansive/);
  });

  it("rejects a runtime that drops cost enforcement", () => {
    const runtime = fs.readFileSync(path.join(repoRoot, "internal/platform/serviceobject/runtime.go"), "utf8");

    const result = runValidator(["--runtime", tempFile("runtime.go", runtime.replaceAll("queryCostExceeded", "removedCostCheck"))]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /service-object runtime must include queryCostExceeded/);
  });

  it("rejects weakening the hard offset and positive cost contract", () => {
    const contract = readJSON("resources/platform-service-object-runtime.json");
    contract.definitionContract.maximumQueryOffset = 100000;
    contract.definitionContract.positiveQueryCostDimensions = ["row"];

    const result = runValidator(["--contract", tempFile("weak-cost-contract.json", contract)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /maximum query offset must stay/);
    assert.match(result.stderr, /cost dimensions must remain positively priced/);
  });

  it("rejects governance regression after closeout", () => {
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    graph.tasks.find((item) => item.id === "persisted-query-command-object-runtime").status = "pending";

    const result = runValidator(["--graph", tempFile("pending-task.json", graph)]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /persisted-query-command-object-runtime task must be implemented/);
  });
});
