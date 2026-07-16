import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import test from "node:test";

const root = process.cwd();
const validator = path.join(root, "scripts/validate-platform-organization-rbac-menu-e2e-evidence.mjs");
const source = path.join(root, "resources/evidence/organization-rbac-menu-e2e-qa-20260716.json");

function run(manifest) {
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "organization-e2e-")), "evidence.json");
  fs.writeFileSync(file, JSON.stringify(manifest));
  try {
    return execFileSync(process.execPath, [validator, file], { cwd: root, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  } catch (error) {
    return `${error.stdout ?? ""}\n${error.stderr ?? ""}`;
  }
}

test("accepts the collected organization E2E evidence", () => {
  const manifest = JSON.parse(fs.readFileSync(source, "utf8"));
  assert.match(run(manifest), /evidence valid/);
});

test("rejects missing viewport coverage", () => {
  const manifest = JSON.parse(fs.readFileSync(source, "utf8"));
  manifest.viewports = manifest.viewports.slice(0, -1);
  assert.match(run(manifest), /viewports must include/);
});

test("rejects unsafe accessibility evidence", () => {
  const manifest = JSON.parse(fs.readFileSync(source, "utf8"));
  manifest.accessibility.primaryTargetsAtLeast44px = false;
  assert.match(run(manifest), /primaryTargetsAtLeast44px must be true/);
});

test("rejects unverified rollback", () => {
  const manifest = JSON.parse(fs.readFileSync(source, "utf8"));
  manifest.rollbackVerified = false;
  assert.match(run(manifest), /rollbackVerified must be true/);
});
