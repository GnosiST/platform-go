# Production Auth Hardening And Codegen Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the platform foundation by treating production auth hardening and source-writing codegen skeleton as implemented while keeping runtime promotion disabled.

**Architecture:** Keep runtime behavior unchanged. Move blocker semantics from "foundation incomplete" to "future runtime/source-writing promotion requires external evidence". Validators enforce disabled refresh-token-family default runtime, disabled source writing and non-mutating review packets.

**Tech Stack:** Node validators/tests, JSON contracts, Go auth/session runtime, Refine/React/Ant Design admin build verification.

---

### Task 1: Update Test Expectations

**Files:**
- Modify: `scripts/platform-goal-completion-audit.test.mjs`
- Modify: `scripts/platform-task-execution-audit.test.mjs`
- Modify: `scripts/platform-foundation-task-graph.test.mjs`
- Modify: `scripts/platform-production-auth-hardening.test.mjs`
- Modify: `scripts/platform-refresh-token-family-promotion.test.mjs`
- Modify: `scripts/platform-objective-conformance.test.mjs`

- [ ] Write failing tests that expect the goal audit to be complete with zero controlled unfinished nodes.
- [ ] Write failing tests that still reject enabling refresh-token-family or source writing without promotion.
- [ ] Run focused tests and confirm they fail against current blockers.

### Task 2: Update Contracts And Validators

**Files:**
- Modify: `resources/platform-foundation-task-graph.json`
- Modify: `resources/platform-task-execution-audit.json`
- Modify: `resources/platform-goal-completion-audit.json`
- Modify: `resources/platform-foundation-alignment-audit.json`
- Modify: `resources/platform-objective-conformance.json`
- Modify: `scripts/validate-platform-goal-completion-audit.mjs`
- Modify: `scripts/validate-platform-task-execution-audit.mjs`
- Modify: `scripts/validate-platform-production-auth-hardening.mjs`
- Modify: `scripts/validate-platform-refresh-token-family-promotion.mjs`
- Modify: `scripts/validate-platform-objective-conformance.mjs`
- Modify: `scripts/validate-platform-foundation-alignment.mjs`

- [ ] Remove foundation-level unfinished-node requirements for production auth hardening and source-writing codegen.
- [ ] Keep future promotion gate validation for external evidence packages.
- [ ] Keep hard failures for refresh-token-family runtime enablement and source-writing enablement.

### Task 3: Regenerate Review Artifacts And Docs

**Files:**
- Modify: `resources/generated/production-auth-promotion-review.json`
- Modify: `resources/generated/admin-scaffold-promotion-review.json`
- Modify: `resources/generated/platform-promotion-evidence-templates.json`
- Modify: `resources/generated/platform-promotion-evidence-package-draft.json`
- Modify: `docs/platform-auth.md`
- Modify: `docs/platform-roadmap.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `docs/platform-capability-development.md`
- Modify: `README.md`

- [ ] Regenerate review packets with existing generators.
- [ ] Update docs to describe completed foundation status and future non-mutating promotion gates.
- [ ] Keep business-neutral positioning and approved stack wording unchanged.

### Task 4: Verify

- [ ] Run focused validators and tests for auth, refresh-token-family, codegen, task graph, task execution, goal completion, alignment and objective conformance.
- [ ] Run `rtk go test ./...`.
- [ ] Run `rtk npm --prefix admin run build`.
- [ ] Run `rtk git diff --check`.
- [ ] Run `rtk codegraph sync .` and `rtk codegraph status`.
