import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const exampleDir = path.resolve(repoRoot, argValue("--example-dir", "examples/external-capability"));

function relative(filePath) {
  return path.relative(repoRoot, filePath) || ".";
}

function run(command, args, cwd) {
  return spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    env: process.env,
  });
}

function walk(directory, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === ".git" || entry.name === "vendor") continue;
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(target, output);
    else if (entry.isFile()) output.push(target);
  }
  return output;
}

function validateExampleFiles(errors) {
  if (!fs.existsSync(exampleDir) || !fs.statSync(exampleDir).isDirectory()) {
    errors.push(`external capability example directory is missing: ${relative(exampleDir)}`);
    return;
  }

  const goModPath = path.join(exampleDir, "go.mod");
  if (!fs.existsSync(goModPath)) {
    errors.push(`external capability go.mod is missing: ${relative(goModPath)}`);
  } else {
    const goMod = fs.readFileSync(goModPath, "utf8");
    if (!goMod.includes("require github.com/GnosiST/platform-go")) {
      errors.push("external capability go.mod must require github.com/GnosiST/platform-go");
    }
    if (!goMod.includes("replace github.com/GnosiST/platform-go => ../..")) {
      errors.push("external capability go.mod must keep the local replace directive for repository validation");
    }
  }

  const goFiles = walk(exampleDir).filter((file) => file.endsWith(".go"));
  if (goFiles.length === 0) {
    errors.push("external capability example must contain Go source files");
  }

  let importsPublicCapability = false;
  for (const file of goFiles) {
    const source = fs.readFileSync(file, "utf8");
    if (source.includes("github.com/GnosiST/platform-go/internal/")) {
      errors.push(`${relative(file)} must not import platform internal packages`);
    }
    if (source.includes("github.com/GnosiST/platform-go/pkg/platform/capability")) {
      importsPublicCapability = true;
    }
  }
  if (!importsPublicCapability) {
    errors.push("external capability example must import pkg/platform/capability");
  }
}

function validateGoCommand(command, args, cwd, errors) {
  const result = run(command, args, cwd);
  if (result.status !== 0) {
    errors.push(`${command} ${args.join(" ")} failed in ${relative(cwd)}:\n${result.stderr || result.stdout}`);
  }
  return result;
}

function validateRunOutput(stdout, errors) {
  let preview;
  try {
    preview = JSON.parse(stdout);
  } catch (error) {
    errors.push(`external capability go run output must be JSON: ${error.message}`);
    return;
  }
  if (preview.capabilityId !== "example-catalog") {
    errors.push(`external capability preview capabilityId = ${preview.capabilityId}, want example-catalog`);
  }
  if (!Array.isArray(preview.adminResources) || !preview.adminResources.includes("catalog-items")) {
    errors.push("external capability preview must include catalog-items admin resource");
  }
  if (!Array.isArray(preview.appRoutes) || !preview.appRoutes.some((route) => route.path === "/api/app/catalog/items")) {
    errors.push("external capability preview must include /api/app/catalog/items app route");
  }
  if (typeof preview.serviceContractHash !== "string" || !preview.serviceContractHash.startsWith("sha256:")) {
    errors.push("external capability preview must include a service contract sha256 hash");
  }
  if (preview.serviceCount !== 1) {
    errors.push(`external capability preview serviceCount = ${preview.serviceCount}, want 1`);
  }
}

const errors = [];
validateExampleFiles(errors);
if (errors.length === 0) {
  validateGoCommand("go", ["test", "./..."], exampleDir, errors);
}
if (errors.length === 0) {
  const result = validateGoCommand("go", ["run", "."], exampleDir, errors);
  if (result.status === 0) validateRunOutput(result.stdout, errors);
}

if (errors.length > 0) {
  for (const error of errors) console.error(error);
  process.exit(1);
}

console.log("Validated external capability example: public imports, go test ./..., go run .");
