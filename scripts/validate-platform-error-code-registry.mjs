import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const defaults = {
  contract: resolve(root, "resources/generated/platform-error-code-contract.json"),
  baseline: resolve(root, "resources/fixtures/platform-error-codes/compatibility-baseline.json"),
  standard: resolve(root, "resources/platform-error-code-standard.json"),
};

function parseArgs(argv) {
  const options = { ...defaults, explicitContract: false };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (!["--contract", "--baseline", "--standard"].includes(argument) || !argv[index + 1]) {
      throw new Error(`unexpected argument ${argument}`);
    }
    const key = argument.slice(2);
    options[key] = resolve(argv[index + 1]);
    if (key === "contract") options.explicitContract = true;
    index += 1;
  }
  return options;
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value && typeof value === "object") {
    return `{${Object.keys(value).sort().map((key) => `${canonicalString(key)}:${canonical(value[key])}`).join(",")}}`;
  }
  return typeof value === "string" ? canonicalString(value) : JSON.stringify(value);
}

function canonicalString(value) {
  return JSON.stringify(value).replaceAll("&", "\\u0026").replaceAll("<", "\\u003c").replaceAll(">", "\\u003e");
}

function expectedHash(contract) {
  const payload = structuredClone(contract);
  delete payload.contractHash;
  return `sha256:${createHash("sha256").update(canonical(payload)).digest("hex")}`;
}

function same(left, right) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function validISODate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value ?? "")) return false;
  const date = new Date(`${value}T00:00:00Z`);
  return !Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === value;
}

function validateDocument(contract, standard, label) {
  const errors = [];
  if (contract.contractVersion !== standard.contractVersion) errors.push(`${label}.contractVersion must equal ${standard.contractVersion}`);
  if (!/^sha256:[0-9a-f]{64}$/.test(contract.contractHash ?? "")) errors.push(`${label}.contractHash must be sha256 lowercase hex`);
  else if (contract.contractHash !== expectedHash(contract)) errors.push(`${label}.contractHash does not match canonical content`);
  if (!same(contract.catalogs, standard.catalogs)) errors.push(`${label}.catalogs must match the ordered standard catalogs`);
  if (!Array.isArray(contract.definitions)) return [...errors, `${label}.definitions must be an array`];
  if (contract.codeCount !== contract.definitions.length) errors.push(`${label}.codeCount must equal definitions length`);

  const codes = new Set();
  for (let index = 0; index < contract.definitions.length; index += 1) {
    const definition = contract.definitions[index];
    const prefix = `${label}.definitions[${index}]`;
    if (index > 0 && contract.definitions[index - 1].code >= definition.code) errors.push(`${label}.definitions must be sorted by code`);
    if (!/^[A-Z][A-Z0-9_]{2,127}$/.test(definition.code ?? "")) errors.push(`${prefix}.code is invalid`);
    if (codes.has(definition.code)) errors.push(`${label} duplicate code ${definition.code}`);
    codes.add(definition.code);
    if (typeof definition.owner !== "string" || definition.owner.length === 0) errors.push(`${prefix}.owner is required`);
    for (const [field, catalog] of [["planes", "planes"], ["audiences", "audiences"]]) {
      if (!Array.isArray(definition[field]) || definition[field].length === 0) errors.push(`${prefix}.${field} is required`);
      else if (definition[field].some((value) => !standard.catalogs[catalog].includes(value))) errors.push(`${prefix}.${field} contains an unknown value`);
    }
    if (!standard.catalogs.categories.includes(definition.category)) errors.push(`${prefix}.category is invalid`);
    if (!Number.isInteger(definition.httpStatus) || definition.httpStatus < 400 || definition.httpStatus > 599) errors.push(`${prefix}.httpStatus must be 400..599`);
    if (!standard.catalogs.retryPolicies.includes(definition.retryPolicy)) errors.push(`${prefix}.retryPolicy is invalid`);
    if (!standard.catalogs.redactionClasses.includes(definition.redactionClass)) errors.push(`${prefix}.redactionClass is invalid`);
    if (typeof definition.publicMessage !== "string" || definition.publicMessage.length === 0) errors.push(`${prefix}.publicMessage is required`);
    if (typeof definition.introducedIn !== "string" || definition.introducedIn.length === 0) errors.push(`${prefix}.introducedIn is required`);
    if (definition.deprecated === true) {
      if (!validISODate(definition.sunsetAt)) errors.push(`${prefix}.sunsetAt must be a valid YYYY-MM-DD date`);
      if (typeof definition.replacedBy !== "string" || definition.replacedBy.length === 0) errors.push(`${prefix}.replacedBy is required`);
      if (definition.replacedBy === definition.code) errors.push(`${prefix}.replacedBy cannot reference itself`);
    } else if (definition.sunsetAt !== undefined || definition.replacedBy !== undefined) {
      errors.push(`${prefix} active definition cannot include deprecation metadata`);
    }
  }
  for (let index = 0; index < contract.definitions.length; index += 1) {
    const definition = contract.definitions[index];
    if (definition.deprecated === true && definition.replacedBy && !codes.has(definition.replacedBy)) {
      errors.push(`${label}.definitions[${index}] has unknown replacement ${definition.replacedBy}`);
    }
  }
  const definitionsByCode = new Map(contract.definitions.map((definition) => [definition.code, definition]));
  for (const definition of contract.definitions) {
    const path = new Set();
    let cursor = definition;
    while (cursor?.deprecated === true && cursor.replacedBy) {
      if (path.has(cursor.code)) {
        errors.push(`${label} replacement cycle includes ${cursor.code}`);
        break;
      }
      path.add(cursor.code);
      cursor = definitionsByCode.get(cursor.replacedBy);
    }
  }
  return errors;
}

function validateCompatibility(contract, baseline, standard) {
  const errors = [];
  const current = new Map(contract.definitions.map((definition) => [definition.code, definition]));
  for (const original of baseline.definitions) {
    const candidate = current.get(original.code);
    if (!candidate) {
      errors.push(`${original.code} was removed without a retained deprecated definition`);
      continue;
    }
    for (const field of standard.compatibility.immutableWithinMajor) {
      if (!same(candidate[field], original[field])) errors.push(`${original.code}.${field} changed within the major version`);
    }
  }
  return errors;
}

function main() {
  const options = parseArgs(process.argv.slice(2));
  const standard = readJSON(options.standard);
  const contract = readJSON(options.contract);
  const baseline = readJSON(options.baseline);
  const errors = [
    ...validateDocument(baseline, standard, "baseline"),
    ...validateDocument(contract, standard, "contract"),
    ...validateCompatibility(contract, baseline, standard),
  ];

  if (!options.explicitContract) {
    const generated = JSON.parse(execFileSync("go", ["run", "./cmd/platform-contracts", "error-codes", "--stdout"], { cwd: root, encoding: "utf8" }));
    if (!same(contract, generated)) errors.push("checked-in generated contract differs from the Go registry export");
  }

  if (errors.length > 0) {
    for (const error of errors) console.error(`- ${error}`);
    process.exitCode = 1;
    return;
  }
  console.log(`platform error-code registry valid: ${contract.definitions.length} definitions`);
}

main();
