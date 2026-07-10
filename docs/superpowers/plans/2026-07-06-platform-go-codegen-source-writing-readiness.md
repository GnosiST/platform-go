# Codegen Source-Writing Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add a machine-checkable readiness gate for future source-writing code generation while keeping runtime source writing disabled.

**Architecture:** The readiness gate is a policy contract plus validator. It reads the existing scaffold plan and generated artifacts, verifies source writing remains disabled, and records the explicit conditions required before any future generator may write runtime files.

**Tech Stack:** Node.js validation scripts, JSON contracts, existing admin scaffold artifacts, Gin + GORM + Casbin + JWT backend, Refine + React + Ant Design frontend contract.

---

### Task 1: Readiness Contract

**Files:**
- Create: `resources/platform-codegen-source-writing-readiness.json`
- Test: `scripts/platform-codegen-source-writing-readiness.test.mjs`

- [x] **Step 1: Write a failing acceptance test for the missing readiness contract**

Create `scripts/platform-codegen-source-writing-readiness.test.mjs` with:

```javascript
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-codegen-source-writing-readiness.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-codegen-readiness-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

describe("validate-platform-codegen-source-writing-readiness", () => {
  it("accepts the current source-writing readiness contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated codegen source-writing readiness/);
  });
});
```

- [x] **Step 2: Run the test to verify it fails**

Run:

```bash
rtk node --test scripts/platform-codegen-source-writing-readiness.test.mjs
```

Expected: fail because `scripts/validate-platform-codegen-source-writing-readiness.mjs` does not exist.

- [x] **Step 3: Create the readiness contract**

Create `resources/platform-codegen-source-writing-readiness.json`:

```json
{
  "$schema": "https://platform-go.local/schemas/platform-codegen-source-writing-readiness.json",
  "capturedAt": "2026-07-06",
  "purpose": "Machine-checkable readiness policy for future source-writing code generation.",
  "mode": {
    "sourceWriting": "disabled",
    "requiresExplicitSpec": true,
    "requiresHumanReview": true,
    "requiresDiffReview": true,
    "requiresTestMapping": true
  },
  "requiredSourceArtifacts": [
    "resources/generated/admin-resource-contract.json",
    "resources/generated/admin-codegen-preview.json",
    "resources/generated/admin-scaffold-plan.json",
    "resources/generated/admin-scaffold-files.json",
    "resources/generated/admin-scaffold-draft.md"
  ],
  "allowedRuntimeTargets": [
    "internal/platform/model/",
    "internal/platform/repository/",
    "internal/platform/httpapi/",
    "internal/platform/adminresource/",
    "admin/src/platform/resources/",
    "admin/src/platform/refine/"
  ],
  "blockedRuntimeTargets": [
    ".git/",
    ".codegraph/",
    "node_modules/",
    "admin/node_modules/",
    "resources/generated/",
    "docs/",
    "scripts/",
    "go.mod",
    "go.sum",
    "admin/package.json",
    "admin/package-lock.json"
  ],
  "promotionRules": [
    {
      "id": "generated-marker-required",
      "description": "Future source writes must include a generated marker in generated-only files.",
      "required": true
    },
    {
      "id": "no-handwritten-overwrite",
      "description": "Future source writes must treat existing files without the generated marker as conflicts.",
      "required": true
    },
    {
      "id": "review-scaffold-first",
      "description": "Future source writes must be promoted from generated scaffold files, not from raw templates.",
      "required": true
    },
    {
      "id": "test-command-required",
      "description": "Future source writes must declare at least one test or validator command per promoted target family.",
      "required": true
    }
  ],
  "preflightCommands": [
    "rtk node scripts/generate-admin-codegen-preview.mjs",
    "rtk node scripts/generate-admin-scaffold-plan.mjs",
    "rtk node scripts/generate-admin-scaffold-files.mjs",
    "rtk node scripts/generate-admin-scaffold-draft.mjs",
    "rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs",
    "rtk node scripts/validate-admin-resources.mjs"
  ]
}
```

- [x] **Step 4: Run the test again**

Run:

```bash
rtk node --test scripts/platform-codegen-source-writing-readiness.test.mjs
```

Expected: still fail because the validator is not implemented.

### Task 2: Readiness Validator

**Files:**
- Create: `scripts/validate-platform-codegen-source-writing-readiness.mjs`
- Modify: `scripts/platform-codegen-source-writing-readiness.test.mjs`

- [x] **Step 1: Implement the validator**

Create `scripts/validate-platform-codegen-source-writing-readiness.mjs`:

```javascript
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-codegen-source-writing-readiness.json"));
const scaffoldPlanPath = path.resolve(repoRoot, argValue("--scaffold-plan", "resources/generated/admin-scaffold-plan.json"));

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function relativeExistingPath(relativePath) {
  if (!relativePath || path.isAbsolute(relativePath)) return false;
  const absolutePath = path.resolve(repoRoot, relativePath);
  const relative = path.relative(repoRoot, absolutePath);
  return relative !== "" && !relative.startsWith("..") && fs.existsSync(absolutePath);
}

function safeRelativePrefix(value) {
  if (!value || path.isAbsolute(value)) return false;
  const absolute = path.resolve(repoRoot, value);
  const relative = path.relative(repoRoot, absolute);
  return relative !== "" && !relative.startsWith("..");
}

function scriptExistsForCommand(command) {
  const parts = command.trim().split(/\s+/);
  const script = parts.find((part) => part.startsWith("scripts/"));
  return !script || relativeExistingPath(script);
}

function validate() {
  const readiness = readJSON(readinessPath);
  const scaffoldPlan = readJSON(scaffoldPlanPath);
  const errors = [];

  if (readiness.mode?.sourceWriting !== "disabled") {
    errors.push("codegen source-writing readiness must keep mode.sourceWriting disabled");
  }
  for (const flag of ["requiresExplicitSpec", "requiresHumanReview", "requiresDiffReview", "requiresTestMapping"]) {
    if (readiness.mode?.[flag] !== true) {
      errors.push(`codegen source-writing readiness must set mode.${flag}=true`);
    }
  }

  for (const artifact of values(readiness.requiredSourceArtifacts)) {
    if (!relativeExistingPath(artifact)) {
      errors.push(`required source artifact is missing or unsafe: ${artifact}`);
    }
  }

  const allowedTargets = values(readiness.allowedRuntimeTargets);
  if (allowedTargets.length === 0) {
    errors.push("allowedRuntimeTargets must not be empty");
  }
  for (const target of allowedTargets) {
    if (!safeRelativePrefix(target)) {
      errors.push(`allowed runtime target is unsafe: ${target}`);
    }
    if (target.startsWith("resources/generated/") || target === "docs/" || target === "scripts/") {
      errors.push(`allowed runtime target cannot be generated/docs/scripts root: ${target}`);
    }
  }

  const blockedTargets = values(readiness.blockedRuntimeTargets);
  for (const requiredBlocked of [".git/", ".codegraph/", "node_modules/", "resources/generated/", "docs/", "scripts/"]) {
    if (!blockedTargets.includes(requiredBlocked)) {
      errors.push(`blockedRuntimeTargets must include ${requiredBlocked}`);
    }
  }
  for (const target of blockedTargets) {
    if (!safeRelativePrefix(target)) {
      errors.push(`blocked runtime target is unsafe: ${target}`);
    }
  }

  const promotionRuleIDs = new Set(values(readiness.promotionRules).map((rule) => rule.id));
  for (const requiredRule of ["generated-marker-required", "no-handwritten-overwrite", "review-scaffold-first", "test-command-required"]) {
    if (!promotionRuleIDs.has(requiredRule)) {
      errors.push(`promotionRules must include ${requiredRule}`);
    }
  }
  for (const rule of values(readiness.promotionRules)) {
    if (rule.required !== true) {
      errors.push(`promotion rule ${rule.id ?? "<missing>"} must be required`);
    }
    if (!rule.description) {
      errors.push(`promotion rule ${rule.id ?? "<missing>"} must declare description`);
    }
  }

  for (const command of values(readiness.preflightCommands)) {
    if (!command.startsWith("rtk ")) {
      errors.push(`preflight command must start with rtk: ${command}`);
    }
    if (!scriptExistsForCommand(command)) {
      errors.push(`preflight command references a missing script: ${command}`);
    }
  }

  if (scaffoldPlan.mode?.sourceWriting !== "disabled") {
    errors.push("admin scaffold plan must keep sourceWriting disabled before readiness can pass");
  }
  if (scaffoldPlan.mode?.dryRun !== true) {
    errors.push("admin scaffold plan must stay in dry-run mode before readiness can pass");
  }
  if (scaffoldPlan.summary?.conflictCount !== 0) {
    errors.push(`admin scaffold plan conflictCount must be 0, got ${scaffoldPlan.summary?.conflictCount}`);
  }
  if (scaffoldPlan.summary?.unsafePathCount !== 0) {
    errors.push(`admin scaffold plan unsafePathCount must be 0, got ${scaffoldPlan.summary?.unsafePathCount}`);
  }

  return { readiness, errors };
}

const { readiness, errors } = validate();
if (errors.length > 0) {
  console.error("Codegen source-writing readiness validation failed:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exit(1);
}

console.log(`Validated codegen source-writing readiness in ${path.relative(repoRoot, readinessPath)} with ${readiness.preflightCommands.length} preflight commands`);
```

- [x] **Step 2: Add negative tests**

Append these tests inside the existing `describe` block in `scripts/platform-codegen-source-writing-readiness.test.mjs`:

```javascript
  it("rejects enabled source writing", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.mode.sourceWriting = "enabled";
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /mode\.sourceWriting disabled/);
  });

  it("rejects missing human review and diff review gates", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.mode.requiresHumanReview = false;
    readiness.mode.requiresDiffReview = false;
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiresHumanReview=true/);
    assert.match(result.stderr, /requiresDiffReview=true/);
  });

  it("rejects unsafe allowed runtime targets", () => {
    const readiness = readJSON("resources/platform-codegen-source-writing-readiness.json");
    readiness.allowedRuntimeTargets.push("../outside");
    readiness.allowedRuntimeTargets.push("resources/generated/");
    const readinessPath = tempJSON("readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /allowed runtime target is unsafe/);
    assert.match(result.stderr, /allowed runtime target cannot be generated/);
  });

  it("rejects scaffold plans that enable source writing", () => {
    const scaffoldPlan = readJSON("resources/generated/admin-scaffold-plan.json");
    scaffoldPlan.mode.sourceWriting = "enabled";
    const scaffoldPlanPath = tempJSON("admin-scaffold-plan.json", scaffoldPlan);

    const result = runValidator(["--scaffold-plan", scaffoldPlanPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin scaffold plan must keep sourceWriting disabled/);
  });
```

- [x] **Step 3: Run validator tests**

Run:

```bash
rtk node --test scripts/platform-codegen-source-writing-readiness.test.mjs
```

Expected: all tests pass.

### Task 3: Engineering Matrix And Task Graph

**Files:**
- Modify: `resources/platform-engineering-capabilities.json`
- Modify: `scripts/validate-platform-engineering-capabilities.mjs`
- Modify: `scripts/platform-engineering-capabilities.test.mjs`
- Modify: `resources/platform-foundation-task-graph.json`

- [x] **Step 1: Extend the required engineering capability list**

In `scripts/validate-platform-engineering-capabilities.mjs`, add this ID to `requiredCapabilityIDs`:

```javascript
  "codegen-source-writing-readiness",
```

- [x] **Step 2: Add a missing-capability regression test**

In `scripts/platform-engineering-capabilities.test.mjs`, add:

```javascript
  it("rejects missing codegen source-writing readiness gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((capability) => capability.id !== "codegen-source-writing-readiness");
    const matrixPath = tempJSON("engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /engineering capability matrix is missing required capability codegen-source-writing-readiness/);
  });
```

- [x] **Step 3: Add the engineering capability entry**

In `resources/platform-engineering-capabilities.json`, add:

```json
{
  "id": "codegen-source-writing-readiness",
  "label": { "zh": "源码生成准入门禁", "en": "Source-Writing Codegen Readiness" },
  "status": "implemented",
  "purpose": "Keep future source-writing generation behind explicit spec, review, diff, test mapping and scaffold safety gates while the current generator remains dry-run only.",
  "dependsOn": ["safe-codegen-scaffold", "task-dependency-governance"],
  "evidence": {
    "sourcePaths": ["resources/platform-codegen-source-writing-readiness.json", "scripts/validate-platform-codegen-source-writing-readiness.mjs"],
    "validators": ["scripts/validate-platform-codegen-source-writing-readiness.mjs", "scripts/platform-codegen-source-writing-readiness.test.mjs"],
    "requiredSourceSnippets": [
      { "path": "resources/platform-codegen-source-writing-readiness.json", "contains": "\"sourceWriting\": \"disabled\"" },
      { "path": "scripts/validate-platform-codegen-source-writing-readiness.mjs", "contains": "mode.sourceWriting disabled" }
    ]
  }
}
```

- [x] **Step 4: Add the task graph node**

In `resources/platform-foundation-task-graph.json`, add a new task after `codegen-preview-scaffold`:

```json
{
  "id": "codegen-source-writing-readiness",
  "title": { "zh": "源码生成准入门禁", "en": "Source-Writing Codegen Readiness" },
  "phase": "stack-and-contracts",
  "scope": "foundation",
  "status": "implemented",
  "resourceLocks": ["codegen", "docs"],
  "dependsOn": ["codegen-preview-scaffold"],
  "evidence": {
    "docs": ["docs/superpowers/specs/2026-07-06-platform-go-codegen-source-writing-readiness-design.md"],
    "validators": ["scripts/validate-platform-codegen-source-writing-readiness.mjs"],
    "tests": ["scripts/platform-codegen-source-writing-readiness.test.mjs"]
  }
}
```

- [x] **Step 5: Update dependent tasks**

In `resources/platform-foundation-task-graph.json`, change `external-business-boundary.dependsOn` from:

```json
["governance-org-area-role-groups", "openapi-app-contracts", "codegen-preview-scaffold"]
```

to:

```json
["governance-org-area-role-groups", "openapi-app-contracts", "codegen-source-writing-readiness"]
```

- [x] **Step 6: Run graph and matrix tests**

Run:

```bash
rtk node --test scripts/platform-engineering-capabilities.test.mjs scripts/platform-foundation-task-graph.test.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
```

Expected: all commands pass.

### Task 4: Documentation Alignment

**Files:**
- Modify: `README.md`
- Modify: `docs/admin-resource-schema.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`

- [x] **Step 1: Add the validator command to README**

Add `rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs` near the existing codegen/scaffold validation commands in `README.md`.

- [x] **Step 2: Document the gate in admin resource schema docs**

In `docs/admin-resource-schema.md`, update the scaffold safety bullets to include:

```markdown
- future source-writing generators must pass `resources/platform-codegen-source-writing-readiness.json` and `scripts/validate-platform-codegen-source-writing-readiness.mjs`; source writing remains disabled until a separate source-writing spec is approved;
```

- [x] **Step 3: Document the capability development rule**

In `docs/platform-capability-development.md`, add:

```markdown
Source-writing code generation is gated by `resources/platform-codegen-source-writing-readiness.json`. The gate does not enable source writes; it keeps explicit-spec, human-review, diff-review, test-mapping and scaffold-safety requirements machine-checkable before any future source-writing generator is designed.
```

- [x] **Step 4: Update the task map**

In `docs/platform-foundation-task-map.md`, update the Code Generator row to mention the source-writing readiness gate:

```markdown
| Code Generator | contract, platform audit, reference coverage gate, preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft and source-writing readiness gate | preview + guarded dry-run | source-writing implementation only after a separate source-writing spec and readiness gate review |
```

- [x] **Step 5: Update the roadmap**

In `docs/platform-roadmap.md`, update the code generation notes so they say preview/scaffold/readiness gate is present, while source writing remains deferred.

- [x] **Step 6: Run doc-related validators**

Run:

```bash
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk git diff --check
```

Expected: all commands pass.

### Task 5: Full Verification And Neat-Freak Closure

**Files:**
- No source edits unless verification exposes a real inconsistency.

- [x] **Step 1: Run full Node tests**

Run:

```bash
rtk node --test scripts/*.test.mjs
```

Expected: all script tests pass.

- [x] **Step 2: Run full Go tests**

Run:

```bash
rtk go test ./...
```

Expected: all Go tests pass.

- [x] **Step 3: Run placeholder and relative-time scan**

Run a placeholder and relative-time phrase scan over the readiness contract, validator, tests, spec, plan, README and related platform docs.

Expected: no matches except the scan description itself.

- [x] **Step 4: Check docs and memory size**

Run:

```bash
rtk wc -l AGENTS.md README.md docs/*.md docs/superpowers/plans/*.md docs/superpowers/specs/*.md
rtk wc -c /Users/irainbow/.codex/memories/MEMORY.md
rtk du -sh docs /Users/irainbow/.codex/memories 2>/dev/null
```

Expected: record any pre-existing size risk in the final handoff.

- [x] **Step 5: Refresh CodeGraph**

Run:

```bash
rtk codegraph sync .
rtk codegraph status
```

Expected: index is up to date.
