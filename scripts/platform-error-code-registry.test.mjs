import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import test from "node:test";

const root = resolve(import.meta.dirname, "..");
const validator = join(root, "scripts", "validate-platform-error-code-registry.mjs");
const baselinePath = join(root, "resources", "fixtures", "platform-error-codes", "compatibility-baseline.json");

function baseline() {
  return JSON.parse(readFileSync(baselinePath, "utf8"));
}

function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value && typeof value === "object") {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonical(value[key])}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

function goJSONString(value) {
  return JSON.stringify(value).replaceAll("&", "\\u0026").replaceAll("<", "\\u003c").replaceAll(">", "\\u003e");
}

function goCanonical(value) {
  if (Array.isArray(value)) return `[${value.map(goCanonical).join(",")}]`;
  if (value && typeof value === "object") {
    return `{${Object.keys(value).sort().map((key) => `${goJSONString(key)}:${goCanonical(value[key])}`).join(",")}}`;
  }
  return typeof value === "string" ? goJSONString(value) : JSON.stringify(value);
}

function refreshHash(contract) {
  const payload = structuredClone(contract);
  delete payload.contractHash;
  contract.contractHash = `sha256:${createHash("sha256").update(canonical(payload)).digest("hex")}`;
}

function compareCodes(left, right) {
  return left.code < right.code ? -1 : left.code > right.code ? 1 : 0;
}

function validate(contract) {
  const directory = mkdtempSync(join(tmpdir(), "platform-error-codes-"));
  const contractPath = join(directory, "contract.json");
  writeFileSync(contractPath, `${JSON.stringify(contract, null, 2)}\n`);
  return spawnSync(process.execPath, [validator, "--contract", contractPath, "--baseline", baselinePath], {
    cwd: root,
    encoding: "utf8",
  });
}

test("checked-in registry contract is valid", () => {
  assert.doesNotThrow(() => execFileSync(process.execPath, [validator], { cwd: root, stdio: "pipe" }));
});

for (const [name, mutate, expected] of [
  ["duplicate code", (contract) => contract.definitions.push(structuredClone(contract.definitions[0])), /duplicate code/],
  ["owner reassignment", (contract) => { contract.definitions[0].owner = "other.owner"; }, /owner changed/],
  ["plane reassignment", (contract) => { contract.definitions[0].planes = ["external"]; }, /planes changed/],
  ["audience reassignment", (contract) => { contract.definitions[0].audiences = ["partner"]; }, /audiences changed/],
  ["category reassignment", (contract) => { contract.definitions[0].category = "internal"; }, /category changed/],
  ["status reassignment", (contract) => { contract.definitions[0].httpStatus = 500; }, /httpStatus changed/],
  ["retry reassignment", (contract) => { contract.definitions[0].retryPolicy = "never"; }, /retryPolicy changed/],
  ["redaction reassignment", (contract) => { contract.definitions[0].redactionClass = "public-safe"; }, /redactionClass changed/],
  ["introduced version reassignment", (contract) => { contract.definitions[0].introducedIn = "0.1.1"; }, /introducedIn changed/],
  ["removal", (contract) => { contract.definitions.shift(); contract.codeCount -= 1; }, /removed without a retained deprecated definition/],
  ["invalid sunset", (contract) => { Object.assign(contract.definitions[0], { deprecated: true, sunsetAt: "invalid", replacedBy: "INTERNAL_ERROR" }); }, /sunsetAt/],
  ["invalid calendar sunset", (contract) => { Object.assign(contract.definitions[0], { deprecated: true, sunsetAt: "2027-02-30", replacedBy: "INTERNAL_ERROR" }); }, /sunsetAt/],
  ["missing replacement", (contract) => { Object.assign(contract.definitions[0], { deprecated: true, sunsetAt: "2027-01-01" }); }, /replacedBy/],
  ["unknown replacement", (contract) => { Object.assign(contract.definitions[0], { deprecated: true, sunsetAt: "2027-01-01", replacedBy: "UNKNOWN_REPLACEMENT" }); }, /unknown replacement/],
]) {
  test(`validator rejects ${name}`, () => {
    const contract = baseline();
    mutate(contract);
    contract.definitions.sort(compareCodes);
    refreshHash(contract);
    const result = validate(contract);
    assert.notEqual(result.status, 0, result.stdout);
    assert.match(`${result.stdout}\n${result.stderr}`, expected);
  });
}

test("validator rejects generated hash drift", () => {
  const contract = baseline();
  contract.contractHash = `sha256:${"0".repeat(64)}`;
  const result = validate(contract);
  assert.notEqual(result.status, 0);
  assert.match(`${result.stdout}\n${result.stderr}`, /contractHash/);
});

test("validator accepts an additive code", () => {
  const contract = baseline();
  const additive = structuredClone(contract.definitions.find((definition) => definition.code === "VALIDATION_ERROR"));
  additive.code = "ADDITIVE_TEST_CODE";
  additive.owner = "platform.test";
  additive.introducedIn = "0.1.1";
  contract.definitions.push(additive);
  contract.definitions.sort(compareCodes);
  contract.codeCount += 1;
  refreshHash(contract);
  const result = validate(contract);
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
});

test("validator rejects a multi-node replacement cycle", () => {
  const contract = baseline();
  const template = structuredClone(contract.definitions.find((definition) => definition.code === "VALIDATION_ERROR"));
  const first = { ...template, code: "CYCLE_CODE_A", deprecated: true, sunsetAt: "2027-01-01", replacedBy: "CYCLE_CODE_B" };
  const second = { ...template, code: "CYCLE_CODE_B", deprecated: true, sunsetAt: "2027-01-01", replacedBy: "CYCLE_CODE_A" };
  contract.definitions.push(first, second);
  contract.definitions.sort(compareCodes);
  contract.codeCount += 2;
  refreshHash(contract);
  const result = validate(contract);
  assert.notEqual(result.status, 0, result.stdout);
  assert.match(`${result.stdout}\n${result.stderr}`, /replacement cycle/);
});

test("validator accepts the Go canonical hash for special characters", () => {
  const contract = baseline();
  const additive = structuredClone(contract.definitions.find((definition) => definition.code === "VALIDATION_ERROR"));
  additive.code = "SPECIAL_CHARACTER_CODE";
  additive.owner = "platform.test<&>";
  additive.publicMessage = "safe <value> & >";
  additive.introducedIn = "0.1.1";
  contract.definitions.push(additive);
  contract.definitions.sort(compareCodes);
  contract.codeCount += 1;
  const payload = structuredClone(contract);
  delete payload.contractHash;
  contract.contractHash = `sha256:${createHash("sha256").update(goCanonical(payload)).digest("hex")}`;
  const result = validate(contract);
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
});
