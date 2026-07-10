import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const scriptName = "scripts/run-platform-production-preflight.mjs";

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function parseArgs(argv) {
  const options = {
    readiness: "resources/platform-production-readiness.json",
    policies: [],
    commands: [],
    list: false,
    json: false,
    run: false,
    strictEnvFile: "",
    help: false,
  };
  const errors = [];

  const readValue = (index, name) => {
    const value = argv[index + 1];
    if (!value || value.startsWith("--")) {
      errors.push(`${name} requires a value`);
      return ["", index];
    }
    return [value, index + 1];
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--help" || arg === "-h") {
      options.help = true;
    } else if (arg === "--readiness") {
      const [value, nextIndex] = readValue(index, arg);
      options.readiness = value || options.readiness;
      index = nextIndex;
    } else if (arg === "--policy") {
      const [value, nextIndex] = readValue(index, arg);
      options.policies.push(...splitList(value));
      index = nextIndex;
    } else if (arg === "--command") {
      const [value, nextIndex] = readValue(index, arg);
      options.commands.push(...splitList(value));
      index = nextIndex;
    } else if (arg === "--strict-env-file") {
      const [value, nextIndex] = readValue(index, arg);
      options.strictEnvFile = value;
      index = nextIndex;
    } else if (arg === "--list") {
      options.list = true;
    } else if (arg === "--json") {
      options.json = true;
    } else if (arg === "--run") {
      options.run = true;
    } else {
      errors.push(`Unknown option ${arg}`);
    }
  }

  if (options.list && options.run) {
    errors.push("--list cannot be combined with --run");
  }

  return { options, errors };
}

function splitList(value) {
  return String(value ?? "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function relativePath(filePath) {
  const relative = path.relative(repoRoot, filePath).split(path.sep).join("/");
  return relative && !relative.startsWith("..") ? relative : filePath;
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function unique(items) {
  const seen = new Set();
  const result = [];
  for (const item of items) {
    if (seen.has(item)) continue;
    seen.add(item);
    result.push(item);
  }
  return result;
}

function displayCommand(command, options) {
  if (command.id === "production-env-audit" && options.strictEnvFile) {
    return `rtk node scripts/validate-platform-production-env.mjs --env-file ${options.strictEnvFile} --strict-secrets`;
  }
  return command.command;
}

function commandTokens(command, options) {
  if (command.id === "production-env-audit" && options.strictEnvFile) {
    return ["rtk", "node", "scripts/validate-platform-production-env.mjs", "--env-file", options.strictEnvFile, "--strict-secrets"];
  }
  return String(command.command ?? "")
    .split(/\s+/)
    .map((token) => token.trim())
    .filter(Boolean);
}

function selectCommands(readiness, options) {
  const errors = [];
  const preflightCommands = values(readiness.preflightCommands);
  const commandsByID = new Map(preflightCommands.map((command) => [command.id, command]));
  const policiesByID = new Map(values(readiness.operationPolicies).map((policy) => [policy.id, policy]));

  if (options.list) {
    return { commandIDs: preflightCommands.map((command) => command.id), errors };
  }

  const selected = [];
  for (const policyID of options.policies) {
    const policy = policiesByID.get(policyID);
    if (!policy) {
      errors.push(`Unknown preflight policy ${policyID}`);
      continue;
    }
    selected.push(...values(policy.preflightCommands));
  }
  for (const commandID of options.commands) {
    if (!commandsByID.has(commandID)) {
      errors.push(`Unknown preflight command ${commandID}`);
      continue;
    }
    selected.push(commandID);
  }

  const commandIDs = selected.length > 0 ? unique(selected) : preflightCommands.map((command) => command.id);
  for (const commandID of commandIDs) {
    if (!commandsByID.has(commandID)) {
      errors.push(`Selected preflight command ${commandID} is not declared in preflightCommands`);
    }
  }

  return { commandIDs, errors };
}

function commandRecord(command, options) {
  return {
    id: command.id,
    command: displayCommand(command, options),
    purpose: command.purpose,
    status: "pending",
  };
}

function runCommand(record, command, options) {
  const tokens = commandTokens(command, options);
  if (tokens.length === 0) {
    return {
      ...record,
      status: "failed",
      exitCode: 1,
      stderr: `preflight command ${command.id} has no executable tokens`,
      stdout: "",
    };
  }
  const result = spawnSync(tokens[0], tokens.slice(1), {
    cwd: repoRoot,
    encoding: "utf8",
    env: process.env,
  });
  return {
    ...record,
    status: result.status === 0 ? "passed" : "failed",
    exitCode: result.status ?? 1,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? "",
  };
}

function buildOutput(readinessPath, readiness, options, selectedCommands) {
  return {
    generatedBy: scriptName,
    source: relativePath(readinessPath),
    mode: options.list ? "list" : options.run ? "run" : "dry-run",
    dryRun: !options.run,
    selection: {
      policies: options.policies,
      commands: options.commands,
      strictEnvFile: options.strictEnvFile || null,
    },
    commandCount: selectedCommands.length,
    commands: selectedCommands,
  };
}

function printText(output) {
  const title =
    output.mode === "list"
      ? "Production preflight commands"
      : output.mode === "run"
        ? "Production preflight execution"
        : "Production preflight dry run";
  console.log(`${title}: ${output.commandCount}`);
  for (const command of output.commands) {
    const suffix = output.mode === "run" ? ` [${command.status}:${command.exitCode}]` : "";
    console.log(`- ${command.id}${suffix}`);
    console.log(`  ${command.command}`);
    if (command.purpose) {
      console.log(`  ${command.purpose}`);
    }
  }
}

function printHelp() {
  console.log(`Usage: rtk node ${scriptName} [options]

Options:
  --list                         List declared preflight commands without execution.
  --policy <id[,id]>             Select commands attached to one or more operation policies.
  --command <id[,id]>            Select one or more preflight command ids.
  --strict-env-file <path>       Run production-env-audit against a private env file with --strict-secrets.
  --readiness <path>             Read an alternate production readiness contract.
  --run                          Execute selected commands sequentially. Omit for dry-run.
  --json                         Emit machine-readable JSON.
`);
}

function main() {
  const { options, errors } = parseArgs(process.argv.slice(2));
  if (options.help) {
    printHelp();
    return 0;
  }

  const readinessPath = path.resolve(repoRoot, options.readiness);
  if (!fs.existsSync(readinessPath)) {
    errors.push(`Readiness contract does not exist: ${options.readiness}`);
  }
  if (errors.length > 0) {
    console.error(errors.join("\n"));
    return 1;
  }

  const readiness = readJSON(readinessPath);
  const commandsByID = new Map(values(readiness.preflightCommands).map((command) => [command.id, command]));
  const selection = selectCommands(readiness, options);
  if (selection.errors.length > 0) {
    console.error(selection.errors.join("\n"));
    return 1;
  }

  const selectedCommands = [];
  let failed = false;
  for (const commandID of selection.commandIDs) {
    const command = commandsByID.get(commandID);
    const record = commandRecord(command, options);
    if (!options.run) {
      selectedCommands.push(record);
      continue;
    }
    const result = runCommand(record, command, options);
    selectedCommands.push(result);
    if (result.status !== "passed") {
      failed = true;
      break;
    }
  }

  const output = buildOutput(readinessPath, readiness, options, selectedCommands);
  if (options.json) {
    process.stdout.write(`${JSON.stringify(output, null, 2)}\n`);
  } else {
    printText(output);
  }
  return failed ? 1 : 0;
}

process.exitCode = main();
