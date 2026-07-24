import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import path from "node:path";
const root = path.resolve(import.meta.dirname, "..");
describe("validate-platform-xa", () => it("accepts the default-off XA contract", () => {
  const result = spawnSync(process.execPath, ["scripts/validate-platform-xa.mjs"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
}));
