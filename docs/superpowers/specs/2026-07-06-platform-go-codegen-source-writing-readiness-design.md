# Platform Go Codegen Source-Writing Readiness Design

## Goal

Add a machine-checkable readiness gate for future source-writing code generation while keeping the current generator in preview and dry-run mode.

## Context

The platform foundation already generates admin resource contracts, OpenAPI documents, codegen previews, scaffold dry-run safety plans, scaffold file manifests and scaffold draft files. These artifacts are useful because they standardize repeatable work without touching runtime source.

Direct source-writing generation is intentionally not enabled. Generated runtime code would cross backend GORM models, repositories, Gin routes, Casbin/RBAC checks, Refine metadata, Ant Design pages and tests. That blast radius is too large to unlock by changing one script flag.

## Design Decision

The next node is a source-writing readiness gate, not a source-writing generator.

The gate records the conditions required before any future source-writing generator can exist:

- source writing remains disabled by default;
- generated output must first be represented as reviewable scaffold files;
- runtime target paths must be whitelisted;
- existing hand-written files must be treated as conflicts;
- generated files must carry a stable marker;
- every future source write must have a matching test command and expected check;
- any enablement must require an explicit, separate source-writing spec and human review step.

## Contract

Create `resources/platform-codegen-source-writing-readiness.json` with:

- `mode.sourceWriting`: must stay `disabled`;
- `mode.requiresExplicitSpec`: must be `true`;
- `mode.requiresHumanReview`: must be `true`;
- `mode.requiresDiffReview`: must be `true`;
- `mode.requiresTestMapping`: must be `true`;
- `allowedRuntimeTargets`: whitelisted runtime target roots for future generated code;
- `runtimeTargetPolicy`: explicit root registry for allowed runtime targets, including whether a root is existing or only proposed, who owns it, whether it requires an existing directory, and whether a separate architecture/source-writing spec is required;
- `blockedRuntimeTargets`: roots that generated code must not write;
- `requiredSourceArtifacts`: existing preview/scaffold artifacts that must exist and stay fresh;
- `promotionRules`: the exact conditions for promoting a generated draft into runtime source later;
- `targetFamilies`: source-writing candidate families that bind scaffold roles to allowed runtime target roots and required test commands;
- `sourceWritingApprovalPackage`: external approval evidence that must stay blocked until platform, codegen, runtime and operations owners approve artifact-backed evidence;
- `sourceWritingApprovalPackage.completedEvidenceSchema`: required artifact URI, artifact hash, reviewed commit, target families, runtime targets, `rtk` verification commands, rollback commands and anti-self-approval rules for future completed evidence;
- `preflightCommands`: commands that prove the preview, scaffold and readiness gate are coherent.

## Validator

Create `scripts/validate-platform-codegen-source-writing-readiness.mjs`.

The validator should:

- reject enabled source writing;
- reject missing explicit-spec, human-review, diff-review or test-mapping flags;
- reject missing preview/scaffold artifacts;
- reject allowed targets outside safe repo-relative runtime roots;
- reject allowed targets or target-family runtime targets that are not declared in `runtimeTargetPolicy.roots`;
- reject proposed runtime roots that try to behave like existing directories before a separate source-writing promotion spec exists;
- reject blocked targets that are not declared;
- reject promotion rules that allow overwriting hand-written files;
- reject missing target families;
- reject target families that reference scaffold roles absent from the current scaffold plan;
- reject target families whose runtime targets are outside the allowlist or inside blocked roots;
- reject target families without `rtk` test commands;
- reject missing operations-owner approval in the source-writing approval package;
- reject source-writing approval packages without a completed-evidence artifact schema;
- reject preflight commands that do not use `rtk`;
- verify referenced scripts and artifact files exist;
- verify the current scaffold plan still has `sourceWriting=disabled`, `dryRun=true`, `conflictCount=0` and `unsafePathCount=0`;
- verify the promotion review packet contains runtime target policy evidence so reviewers can distinguish existing platform roots from proposed future package roots.

## Task Graph And Capability Matrix

Add a new task graph node `codegen-source-writing-readiness` after `codegen-preview-scaffold`.

Keep `codegen-preview-scaffold` as `preview`. Do not change it to implemented until the project has a real source-writing generator or explicitly decides that preview-only is the final supported capability.

Add an engineering capability matrix entry for the readiness gate so stack and scaffold drift checks include this policy.

## Non-Goals

- Do not write generated runtime source.
- Do not add model/repository/router generators.
- Do not change generated scaffold file content beyond what the readiness validator requires.
- Do not introduce form slots.
- Do not change admin UI behavior.

## Verification

Minimum verification for this node:

- `rtk node --test scripts/platform-codegen-source-writing-readiness.test.mjs`
- `rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs`
- `rtk node scripts/validate-platform-foundation-task-graph.mjs`
- `rtk node scripts/validate-platform-engineering-capabilities.mjs`
- `rtk node scripts/validate-admin-resources.mjs`
- `rtk node --test scripts/*.test.mjs`
- `rtk go test ./...`
- `rtk git diff --check`
- `rtk codegraph sync .`
- `rtk codegraph status`
