import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
describe("validate-platform-search-projection", () => it("accepts the registered replayable projection contract", () => {
  const result = spawnSync(process.execPath, ["scripts/validate-platform-search-projection.mjs"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
}));
