# AGENTS.md

This repository is the reusable `platform-go` foundation informed by reusable management patterns observed in `zshenmez`.

## Prime Direction

- Build a business-neutral platform foundation first, then attach business capabilities.
- Keep the target stack aligned: backend `Gin + GORM + Casbin + JWT`; admin frontend `Refine + React + Ant Design`.
- Treat capability manifests and resource schemas as contracts. Do not hard-code business menus, permissions or admin resources into the shell.
- Use platform ports and APIs for common capabilities. Business packages must not reach into concrete storage, HTTP handlers or admin shell internals.
- Reference coverage is live by default: `scripts/validate-platform-reference-coverage.mjs` reads the current `zshenmez/resources/admin-resources.json` through `resources/platform-reference-discovery.json`, so new reference resources must be classified before coverage can pass.

## Local Rules

- Prefix shell commands with `rtk`.
- Use CodeGraph before changing shared platform contracts, UI primitives, authorization, persistence, route registration or capability manifests:

```bash
rtk codegraph sync .
rtk codegraph status
```

- Keep `.codegraph/` local and ignored.
- Protect the working tree. Do not reset, clean, revert or overwrite unrelated changes.
- Prefer small, contract-preserving changes over broad rewrites unless architecture drift requires a deliberate correction.

## Architecture Boundaries

- Capability registration starts from `capability.Manifest`.
- Concrete business capabilities live under `internal/apps/<app>` and are injected by process composition roots. `internal/platform/core` and optional platform capabilities must not import application packages.
- Admin resources, menus, permission prefixes, demo data, lifecycle steps and auth providers belong in capability declarations.
- Menus are visibility hints; backend permission checks are authoritative.
- Dynamic RBAC uses `user -> roles -> permissions -> menus/resources/actions`, with Casbin enforcing protected HTTP actions.
- Admin HTTP auth uses JWT bearer tokens backed by server-side sessions for TTL and revocation.
- Do not enable `httpapi.ServerOptions.AllowInsecureHeaderAuth` in the default API process; it exists only for tests and tightly controlled local harnesses.
- Production runtime must not enable the `demo-data` capability or leave the `demo` auth provider enabled; set `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`.
- Vercel is optional for `admin` static hosting only. Keep `cmd/platform-api` on a long-lived-service default unless a separate Vercel Go runtime adapter spec and evidence package is approved.
- Redis/cache is an optimization behind the platform cache port, not a source of truth.
- Common admin UI must use platform wrappers over Ant Design for repeated surfaces: page, table, modal, drawer, button, alert, dropdown, pagination and settings.
- i18n is a hard gate for shared admin components. Add Chinese and English dictionary keys in the same change.
- Implemented task nodes must have a `resources/platform-node-closeout-audit.json` entry with cleanup evidence. Do not invoke `neat-freak` for every small node or routine sub-agent task; reserve it for phase closeout, major cross-module work, or release preparation. Visual nodes must keep both `superpowers:brainstorming` and product-design evidence.

## Resource And Codegen Rules

When changing admin resources:

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk go run ./cmd/platform-contracts app-routes --output resources/generated/app-route-contract.json
rtk go run ./cmd/platform-contracts audit --output resources/generated/platform-capability-audit.json
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/generate-admin-scaffold-promotion-review.mjs
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk node scripts/validate-admin-resources.mjs
```

Generic list queries use:

```text
POST /api/admin/resources/:resource/query
```

The UI may expose SQL-like search text, but transport must remain structured JSON conditions and backend/database implementations must use whitelisted fields and parameterized predicates.
Run `rtk node scripts/validate-platform-admin-api-boundary.mjs` when changing admin API clients, Refine data providers, generic resource query behavior, generated OpenAPI query schemas or backend query validation.
Run `rtk node scripts/validate-platform-app-client-api-boundary.mjs` when changing App route contracts, App OpenAPI output, App codegen previews or downstream App/H5/mini-program request/upload client conventions.
Run `rtk node scripts/validate-platform-production-env.mjs` when changing production environment templates, production security settings, Compose environment wiring or production configuration validation.

## Verification

Run the narrowest relevant set, then broaden when shared contracts changed:

```bash
rtk go test ./...
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk node scripts/validate-platform-reference-coverage.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/generate-platform-operations-plan.mjs
rtk node scripts/generate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-production-env.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-platform-file-storage-experience.mjs
rtk node --test scripts/platform-foundation-docs-drift.test.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

Use the local browser for visual checks when admin UI behavior changes.
