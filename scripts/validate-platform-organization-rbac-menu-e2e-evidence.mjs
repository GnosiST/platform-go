import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const manifestPath = process.argv[2] ?? "resources/evidence/organization-rbac-menu-e2e-qa-20260716.json";
const absolute = path.resolve(root, manifestPath);
const errors = [];
const requiredViewports = ["375x812", "390x844", "768x1024", "1024x768", "1280x720", "1440x1024"];
const requiredScenarios = [
  "organization-user-conflict-workflows",
  "menu-permission-transfer",
  "tree-transfer-large-dataset",
  "keyboard-accessibility",
  "cutover-rollback"
];

const fail = (message) => errors.push(message);
const isObject = (value) => value && typeof value === "object" && !Array.isArray(value);
const requireEqual = (actual, expected, label) => {
  if (actual !== expected) fail(`${label} must equal ${JSON.stringify(expected)}`);
};
const requireTrue = (actual, label) => {
  if (actual !== true) fail(`${label} must be true`);
};
const requireIncludes = (items, expected, label) => {
  if (!Array.isArray(items) || !expected.every((item) => items.includes(item))) {
    fail(`${label} must include ${expected.join(", ")}`);
  }
};

if (!fs.existsSync(absolute)) {
  console.error(`evidence manifest not found: ${manifestPath}`);
  process.exit(1);
}

let evidence;
try {
  evidence = JSON.parse(fs.readFileSync(absolute, "utf8"));
} catch (error) {
  console.error(`invalid evidence manifest: ${error.message}`);
  process.exit(1);
}

if (!isObject(evidence)) fail("manifest must be an object");
requireEqual(evidence?.taskId, "organization-rbac-menu-e2e-qa", "taskId");
requireEqual(evidence?.tool, "in-app-browser", "tool");
requireIncludes(evidence?.viewports, requiredViewports, "viewports");
requireEqual(evidence?.largeDataset?.nodes, 10000, "largeDataset.nodes");
requireEqual(evidence?.largeDataset?.selected, 2000, "largeDataset.selected");
requireEqual(evidence?.consoleErrors, 0, "consoleErrors");
requireEqual(evidence?.failedFirstPartyRequests, 0, "failedFirstPartyRequests");
requireEqual(evidence?.documentHorizontalOverflow, false, "documentHorizontalOverflow");
requireEqual(evidence?.zoom200PercentOverflow, false, "zoom200PercentOverflow");
requireEqual(evidence?.unapprovedPrincipalDifferences, 0, "unapprovedPrincipalDifferences");
requireTrue(evidence?.rollbackVerified, "rollbackVerified");
requireIncludes(evidence?.scenarios, requiredScenarios, "scenarios");
requireIncludes(evidence?.keyboard?.keys, ["Tab", "Shift+Tab", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End"], "keyboard.keys");
requireEqual(evidence?.keyboard?.mixedState, "aria-checked=mixed", "keyboard.mixedState");
requireTrue(evidence?.keyboard?.countAnnouncement, "keyboard.countAnnouncement");
requireTrue(evidence?.keyboard?.focusRestored, "keyboard.focusRestored");
requireTrue(evidence?.accessibility?.primaryTargetsAtLeast44px, "accessibility.primaryTargetsAtLeast44px");
requireTrue(evidence?.accessibility?.reducedMotionVerified, "accessibility.reducedMotionVerified");
requireTrue(evidence?.accessibility?.ariaTreeSemantics, "accessibility.ariaTreeSemantics");
requireTrue(evidence?.cutover?.checkpointRestored, "cutover.checkpointRestored");
requireEqual(evidence?.cutover?.stateAfterRollback, "legacy-serving-target-writes-false", "cutover.stateAfterRollback");
requireTrue(evidence?.redaction?.scanPassed, "redaction.scanPassed");
requireTrue(evidence?.redaction?.sensitiveDataAbsent, "redaction.sensitiveDataAbsent");

const viewportResults = evidence?.viewportResults;
if (!Array.isArray(viewportResults) || viewportResults.length !== requiredViewports.length) {
  fail("viewportResults must contain exactly six entries");
} else {
  for (const viewport of requiredViewports) {
    const result = viewportResults.find((item) => item.viewport === viewport);
    if (!result) fail(`viewportResults missing ${viewport}`);
    else {
      requireEqual(result.horizontalOverflow, false, `${viewport}.horizontalOverflow`);
      requireEqual(result.zoom200PercentOverflow, false, `${viewport}.zoom200PercentOverflow`);
      requireTrue(result.keyboardReachable, `${viewport}.keyboardReachable`);
      requireTrue(result.touchTargetsAtLeast44px, `${viewport}.touchTargetsAtLeast44px`);
    }
  }
}

if (errors.length) {
  console.error(errors.map((error) => `- ${error}`).join("\n"));
  process.exit(1);
}
console.log(`organization RBAC/menu E2E evidence valid: ${manifestPath}`);
