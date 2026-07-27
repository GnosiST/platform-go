import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const severityRank = new Map([
  ["info", 0],
  ["low", 1],
  ["moderate", 2],
  ["high", 3],
  ["critical", 4],
]);

function advisoryID(url = "") {
  return url.match(/GHSA-[a-z0-9-]+/i)?.[0] ?? "unknown-advisory";
}

function exceptionIsActive(exception, prefix, packageName, advisory, now) {
  if (
    exception?.prefix !== prefix ||
    exception?.package !== packageName ||
    exception?.advisory !== advisory ||
    typeof exception?.rationale !== "string" ||
    exception.rationale.trim() === ""
  ) {
    return false;
  }
  const expiresAt = Date.parse(`${exception.expiresOn}T23:59:59.999Z`);
  return Number.isFinite(expiresAt) && now.getTime() <= expiresAt;
}

function advisoryLeaves(report, packageName, visiting = new Set()) {
  if (visiting.has(packageName)) {
    return [{ advisory: "dependency-cycle", package: packageName, severity: "high" }];
  }
  const vulnerability = report.vulnerabilities?.[packageName];
  if (!vulnerability) {
    return [{ advisory: "unresolved-dependency", package: packageName, severity: "high" }];
  }

  const nextVisiting = new Set(visiting).add(packageName);
  const leaves = [];
  for (const via of vulnerability.via ?? []) {
    if (typeof via === "string") {
      leaves.push(...advisoryLeaves(report, via, nextVisiting));
      continue;
    }
    leaves.push({
      advisory: advisoryID(via?.url),
      package: via?.dependency || via?.name || packageName,
      severity: via?.severity || vulnerability.severity || "high",
    });
  }
  return leaves.length > 0
    ? leaves
    : [{ advisory: "unresolved-advisory", package: packageName, severity: vulnerability.severity || "high" }];
}

export function evaluateAuditReport(report, policy, prefix, now = new Date()) {
  const failures = new Map();
  for (const packageName of Object.keys(report.vulnerabilities ?? {})) {
    for (const leaf of advisoryLeaves(report, packageName)) {
      if ((severityRank.get(leaf.severity) ?? 3) < severityRank.get("high")) continue;
      const allowed = (policy.exceptions ?? []).some((exception) =>
        exceptionIsActive(exception, prefix, leaf.package, leaf.advisory, now),
      );
      if (!allowed) failures.set(`${leaf.package}:${leaf.advisory}`, leaf);
    }
  }
  return [...failures.values()].sort((left, right) =>
    `${left.package}:${left.advisory}`.localeCompare(`${right.package}:${right.advisory}`),
  );
}

function argument(name, fallback = "") {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1] ?? "";
}

function main() {
  const prefix = argument("--prefix");
  const policyPath = path.resolve(repoRoot, argument("--policy", "resources/node-production-audit-policy.json"));
  const packagePath = path.resolve(repoRoot, prefix, "package.json");
  if (!prefix || !packagePath.startsWith(`${repoRoot}${path.sep}`) || !fs.existsSync(packagePath)) {
    throw new Error(`invalid npm package prefix: ${prefix || "<missing>"}`);
  }

  const policy = JSON.parse(fs.readFileSync(policyPath, "utf8"));
  if (policy.schemaVersion !== 1 || !Array.isArray(policy.exceptions)) {
    throw new Error(`unsupported production dependency audit policy: ${path.relative(repoRoot, policyPath)}`);
  }
  const audit = spawnSync("npm", ["audit", "--prefix", prefix, "--omit=dev", "--json"], {
    cwd: repoRoot,
    encoding: "utf8",
  });
  if (audit.error) throw audit.error;

  let report;
  try {
    report = JSON.parse(audit.stdout);
  } catch {
    throw new Error(`npm audit did not return JSON for ${prefix}: ${audit.stderr.trim() || "unknown error"}`);
  }
  if (report.auditReportVersion !== 2 || typeof report.vulnerabilities !== "object") {
    throw new Error(`npm audit returned an invalid report for ${prefix}`);
  }

  const failures = evaluateAuditReport(report, policy, prefix);
  if (failures.length > 0) {
    for (const failure of failures) {
      console.error(`${failure.severity}: ${failure.package} ${failure.advisory}`);
    }
    process.exitCode = 1;
    return;
  }

  const active = (policy.exceptions ?? []).filter((exception) =>
    exception.prefix === prefix &&
    exceptionIsActive(exception, prefix, exception.package, exception.advisory, new Date()),
  );
  const suffix = active.length === 0
    ? "no active exceptions"
    : `${active.length} active exception(s), nearest expiry ${active.map((item) => item.expiresOn).sort()[0]}`;
  console.log(`Production dependency audit passed for ${prefix}: ${suffix}`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
