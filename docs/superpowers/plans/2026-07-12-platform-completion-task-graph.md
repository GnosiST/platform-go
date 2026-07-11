# Platform Completion Task Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register the approved eight-node completion program as controlled pending work without reopening or falsifying the 37 closed foundation nodes.

**Architecture:** The task graph remains the source of task identity, dependencies and locks. Execution, alignment, goal, objective, engineering and closeout resources project the same ordered pending-node list. Validators derive completion from graph state instead of hard-coding the previous Task 8 terminal state.

**Tech Stack:** JSON governance contracts, Node.js validators, `node:test`, Markdown documentation.

## Global Constraints

- Preserve all 37 existing task nodes and closeout entries unchanged.
- Add exactly the eight IDs approved in `docs/superpowers/specs/2026-07-12-platform-go-completion-program-design.md`.
- Initial state is 45 total, 37 implemented and 8 controlled pending.
- Pending tasks require localized `statusReason` and `completionGate` and must not have closeout entries.
- Visual tasks require `superpowers:brainstorming` and `product-design`; pending visual tasks do not claim screenshot evidence.
- Production auth promotion, refresh-token-family default runtime and source writing remain disabled.
- Prefix commands with `rtk`.

---

### Task 1: Add RED Governance Projection Tests

**Files:**
- Modify: `scripts/platform-foundation-task-graph.test.mjs`
- Modify: `scripts/platform-task-execution-audit.test.mjs`
- Modify: `scripts/platform-goal-completion-audit.test.mjs`
- Modify: `scripts/platform-node-closeout-audit.test.mjs`
- Modify: `scripts/platform-objective-conformance.test.mjs`
- Modify: `scripts/platform-foundation-alignment.test.mjs`
- Modify: `scripts/platform-engineering-capabilities.test.mjs`

**Interfaces:**
- Consumes: current seven governance JSON resources and validators.
- Produces: failing tests that require one ordered `completionProgramTaskIDs` projection.

- [ ] **Step 1: Add the shared expected ID list to each focused test file**

```js
const completionProgramTaskIDs = [
  "runtime-security-containment",
  "admin-watermark-export-governance",
  "sensitive-data-protection-runtime",
  "sensitive-data-historical-migration",
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];
```

- [ ] **Step 2: Add graph and projection assertions**

```js
it("tracks the approved completion program as controlled pending work", () => {
  const graph = readJSON("resources/platform-foundation-task-graph.json");
  assert.equal(graph.tasks.length, 45);
  assert.deepEqual(
    graph.tasks.filter((task) => task.status === "pending").map((task) => task.id),
    completionProgramTaskIDs,
  );
});
```

Add equivalent assertions for:

```js
assert.deepEqual(execution.requiredUnfinishedNodes, completionProgramTaskIDs);
assert.deepEqual(goal.completionPolicy.requiredControlledUnfinishedNodes, completionProgramTaskIDs);
assert.deepEqual(closeout.pendingNodeEvidence, completionProgramTaskIDs);
assert.deepEqual(objective.taskControlPolicy.requiredUnfinishedNodes, completionProgramTaskIDs);
assert.deepEqual(alignment.requiredFutureTaskNodes, completionProgramTaskIDs);
assert.equal(goal.completionStatus, "not-complete-controlled");
assert.deepEqual(goal.taskSummary, { expectedTotal: 45, expectedImplemented: 37, expectedControlledUnfinished: 8 });
```

Also replace the production Admin OIDC test's terminal `37/37/0` assertion with a baseline-specific assertion: the first 37 task IDs and their closeouts remain unchanged, while the complete graph is `45/37/8`. Do not weaken the existing OIDC evidence checks.

Replace rather than append to every obsolete terminal-state assertion across the seven focused test files: `37/37/0`, empty unfinished/future/pending lists, `completionStatus: "complete"` and empty controlled blockers. Keep separate baseline assertions proving the original 37 task IDs, their implemented status and their 37 closeouts are unchanged.

- [ ] **Step 3: Add mutation tests for missing IDs, reordered IDs and premature closeout**

```js
const graph = readJSON("resources/platform-foundation-task-graph.json");
graph.tasks = graph.tasks.filter((task) => task.id !== "runtime-security-containment");
// Validator must fail with the missing task ID.

const closeout = readJSON("resources/platform-node-closeout-audit.json");
closeout.nodeCloseouts.push({
  taskId: "runtime-security-containment",
  status: "closed",
  neatFreak: true,
  cleanupEvidence: ["docs/platform-auth.md"],
  dimensions: ["docs", "tests-or-validators", "resource-lock-review", "objective-conflict-review"],
});
// Validator must fail because pending nodes cannot have closeout entries.
```

- [ ] **Step 4: Run the focused tests and verify RED**

Run:

```bash
rtk node --test scripts/platform-foundation-task-graph.test.mjs scripts/platform-task-execution-audit.test.mjs scripts/platform-goal-completion-audit.test.mjs scripts/platform-node-closeout-audit.test.mjs scripts/platform-objective-conformance.test.mjs scripts/platform-foundation-alignment.test.mjs scripts/platform-engineering-capabilities.test.mjs
```

Expected: FAIL because the graph still has 37 tasks and the projection lists are empty.

### Task 2: Register The Eight Pending Nodes

**Files:**
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-node-closeout-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-engineering-capabilities.json`

**Interfaces:**
- Consumes: approved completion-program spec and existing task graph schema.
- Produces: eight pending nodes and exact ordered projections.

- [ ] **Step 1: Add new resource locks and policies**

Add these locks to `resourceLocks` with exclusive localized policies:

```json
["data-protection", "migration-runtime", "open-source-release", "public-docs-site", "github-publication"]
```

Add conflict groups so `data-protection` conflicts with `storage-runtime` and `admin-resource-contract`, while publication locks conflict with `docs` and each other.

- [ ] **Step 2: Append the exact task declarations in approved order**

Each task uses `phase: "production-governance"`, `status: "pending"` and localized rationale/gates. Use these exact scopes and resource locks:

| Task ID | Scope | Resource locks |
| --- | --- | --- |
| `runtime-security-containment` | `foundation` | `admin-resource-contract`, `jwt-session`, `storage-runtime`, `cache-runtime`, `production-config`, `audit-policy` |
| `admin-watermark-export-governance` | `admin-ui` | `admin-ui`, `i18n`, `browser-qa`, `audit-policy`, `docs` |
| `sensitive-data-protection-runtime` | `foundation` | `data-protection`, `admin-resource-contract`, `storage-runtime`, `production-config` |
| `sensitive-data-historical-migration` | `foundation` | `data-protection`, `migration-runtime`, `storage-runtime`, `docs` |
| `open-source-portability` | `governance` | `open-source-release`, `codegen`, `docs` |
| `public-docs-community` | `governance` | `open-source-release`, `docs` |
| `public-docs-site` | `governance` | `public-docs-site`, `i18n`, `browser-qa`, `docs` |
| `github-release-publication` | `governance` | `github-publication`, `open-source-release`, `public-docs-site` |

Required dependency chain:

```json
{
  "runtime-security-containment": ["production-admin-oidc-auth", "production-persistence-correctness"],
  "admin-watermark-export-governance": ["runtime-security-containment", "admin-ui-system-quality-hardening"],
  "sensitive-data-protection-runtime": ["runtime-security-containment", "resource-schema-contract", "gorm-storage-runtime"],
  "sensitive-data-historical-migration": ["sensitive-data-protection-runtime", "production-persistence-correctness"],
  "open-source-portability": ["admin-watermark-export-governance", "sensitive-data-historical-migration"],
  "public-docs-community": ["open-source-portability"],
  "public-docs-site": ["public-docs-community"],
  "github-release-publication": ["public-docs-site", "open-source-portability"]
}
```

`admin-watermark-export-governance` and `public-docs-site` are visual and declare:

```json
"visual": true,
"designGate": ["superpowers:brainstorming", "product-design"]
```

Evidence docs initially reference the approved spec and plan only. Do not add screenshot paths before files exist.

- [ ] **Step 3: Update every pending-node projection**

Use the exact ID list from Task 1 in all resources. Set:

```json
"completionStatus": "not-complete-controlled"
```

and:

```json
"taskSummary": { "expectedTotal": 45, "expectedImplemented": 37, "expectedControlledUnfinished": 8 }
```

Keep existing 37 closeout entries unchanged. Add only the eight strings to `pendingNodeEvidence`.

In `platform-objective-conformance.json`, set `completionPolicy.controlledBlockers` to the same ordered list. In `platform-foundation-alignment-audit.json`, set `requiredFutureTaskNodes` to the list, append all eight IDs to `nonDroppableGoalNodes`, and validate required task coverage as `requiredTaskNodes + requiredFutureTaskNodes` without moving the preserved 37 baseline nodes out of `requiredTaskNodes`.

Also set `platform-objective-conformance.json` `completionPolicy.goalCompletionStatus` to `"not-complete-controlled"`; the goal audit's `completionStatus`, objective status and controlled blockers must be derived from the same unfinished graph list.

- [ ] **Step 4: Add partial engineering capabilities**

Add these partial capability records in the same order and add their IDs to `requiredEngineeringCapabilities` and `nonDroppableEngineeringCapabilities`:

```text
runtime-security-containment
admin-watermark-export-governance
sensitive-data-protection
open-source-portability
public-documentation-and-release
```

Their evidence points only to the approved specs, plans and pending task IDs; no implementation source path, generated artifact, screenshot or completed evidence is claimed yet. Generalize `requiredCapabilityIDs` in the engineering validator so these five approved partial entries are required without weakening validation of the existing 31 implemented capabilities.

### Task 3: Generalize Governance Validators

**Files:**
- Modify: `scripts/validate-platform-foundation-task-graph.mjs`
- Modify: `scripts/validate-platform-task-execution-audit.mjs`
- Modify: `scripts/validate-platform-goal-completion-audit.mjs`
- Modify: `scripts/validate-platform-node-closeout-audit.mjs`
- Modify: `scripts/validate-platform-objective-conformance.mjs`
- Modify: `scripts/validate-platform-foundation-alignment.mjs`
- Modify: `scripts/validate-platform-engineering-capabilities.mjs`

**Interfaces:**
- Consumes: graph-derived implemented and unfinished task lists.
- Produces: exact projection validation without Task 8 terminal-state constants.

- [ ] **Step 1: Derive graph state once in each validator**

```js
const implementedTaskIDs = graph.tasks.filter((task) => task.status === "implemented").map((task) => task.id);
const unfinishedTaskIDs = graph.tasks.filter((task) => task.status !== "implemented").map((task) => task.id);
const expectedCompletionStatus = unfinishedTaskIDs.length === 0 ? "complete" : "not-complete-controlled";
```

- [ ] **Step 2: Replace hard-coded empty-list rules with exact projection rules**

```js
if (!sameList(audit.requiredUnfinishedNodes, unfinishedTaskIDs)) {
  errors.push("requiredUnfinishedNodes must exactly match unfinished task graph nodes in graph order");
}
```

Apply the same rule to goal, closeout, objective and alignment resources. Reject a closeout whose task ID is unfinished. Require every implemented task to have exactly one closeout.

Keep the production Admin OIDC evidence validator scoped to that completed node, but remove its global `37/37/0` terminal-count assertion. Instead, assert that the preserved 37-node baseline prefix and its 37 closeouts remain unchanged, then derive current totals from the full graph.

- [ ] **Step 3: Preserve visual and promotion boundaries**

Keep existing visual-gate, screenshot, promotion blocker and disabled-runtime checks. Pending visual tasks need design gates but do not need screenshot evidence until implemented.

- [ ] **Step 4: Run focused tests and validators and verify GREEN**

Run the Task 1 command, then:

```bash
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
```

Expected: all focused tests and validators PASS with 45/37/8.

### Task 4: Synchronize Public Task Documentation And Commit

**Files:**
- Modify: `README.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-ui-optimization-assessment.md`

**Interfaces:**
- Consumes: verified 45/37/8 governance state.
- Produces: honest current-state documentation without changing promotion status.

- [ ] **Step 1: Document the new active program**

State that the original 37-node foundation baseline remains closed while the active completion program adds eight controlled pending nodes. Link the five approved 2026-07-12 specifications.

- [ ] **Step 2: Run final governance verification**

```bash
rtk node --test scripts/platform-foundation-task-graph.test.mjs scripts/platform-task-execution-audit.test.mjs scripts/platform-goal-completion-audit.test.mjs scripts/platform-node-closeout-audit.test.mjs scripts/platform-objective-conformance.test.mjs scripts/platform-foundation-alignment.test.mjs scripts/platform-engineering-capabilities.test.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

- [ ] **Step 3: Commit**

```bash
rtk git add README.md docs resources scripts
rtk git commit -m "docs: register platform completion program"
```
