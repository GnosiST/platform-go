import assert from "node:assert/strict";
import test from "node:test";

import { evaluateAuditReport } from "./run-node-production-audit.mjs";

const advisory = {
  source: 1124282,
  name: "react-router",
  dependency: "react-router",
  title: "React Router RSC mode advisory",
  url: "https://github.com/advisories/GHSA-qwww-vcr4-c8h2",
  severity: "high",
};

function reportWith(via = [advisory]) {
  return {
    vulnerabilities: {
      "react-router": {
        name: "react-router",
        severity: "high",
        via,
      },
      "react-router-dom": {
        name: "react-router-dom",
        severity: "high",
        via: ["react-router"],
      },
    },
  };
}

function policy(expiresOn = "2026-08-10") {
  return {
    exceptions: [
      {
        prefix: "admin",
        package: "react-router",
        advisory: "GHSA-qwww-vcr4-c8h2",
        expiresOn,
        rationale: "The Admin is a client-only SPA and does not enable React Server Components.",
      },
    ],
  };
}

test("accepts only a matching, unexpired advisory exception", () => {
  assert.deepEqual(
    evaluateAuditReport(reportWith(), policy(), "admin", new Date("2026-07-27T00:00:00Z")),
    [],
  );
});

test("rejects an unapproved high-severity advisory", () => {
  const report = reportWith([
    { ...advisory, url: "https://github.com/advisories/GHSA-xxxx-yyyy-zzzz" },
  ]);

  assert.deepEqual(
    evaluateAuditReport(report, policy(), "admin", new Date("2026-07-27T00:00:00Z")),
    [
      {
        advisory: "GHSA-xxxx-yyyy-zzzz",
        package: "react-router",
        severity: "high",
      },
    ],
  );
});

test("rejects an expired advisory exception", () => {
  assert.deepEqual(
    evaluateAuditReport(reportWith(), policy("2026-07-26"), "admin", new Date("2026-07-27T00:00:00Z")),
    [
      {
        advisory: "GHSA-qwww-vcr4-c8h2",
        package: "react-router",
        severity: "high",
      },
    ],
  );
});
