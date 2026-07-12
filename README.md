# platform-go

Reusable operations platform foundation informed by reusable management patterns observed in `zshenmez`.

## Baseline

Target stack alignment is explicit and guarded:

- backend: Gin + GORM + Casbin + JWT;
- admin frontend: Refine + React + Ant Design;
- platform contract source: capability manifests plus `resources/admin-resources.json`;
- generated contracts: admin resources, OpenAPI, app routes, codegen previews, scaffold dry-run packages and production operations plans.

The default foundation is business-neutral. It owns common platform governance, auth, RBAC, menus, resource schemas, audit/logs, cache, file storage, branding, demo data and operations resources. Business packages attach through capability manifests and process composition roots; they must not be hard-coded into platform core or the admin shell.

Default governance is broader than tenant-only management: `tenants`, `org-units`, `users`, `roles`, `role-groups` and `area-codes` are reusable primitives. `org-units` is the single tenant-owned organization tree for groups, companies, branches, institutions, departments, teams and stores, so the base does not need separate default `organizations` or `departments` resources. `role-groups` classify roles and support governance/review workflows, but they do not grant permissions, own role membership, inherit policies or carry data scopes. `area-codes` are optional regional master data for tenants, org units, users and optional personnel records; they do not imply authorization unless roles explicitly declare area data scopes through `roles.dataScopeAreaCodes`.

Optional profiles keep heavier reusable capabilities detachable: `platform-personnel-ready`, `platform-notification-ready`, `platform-job-ready` and `enterprise-governance` extend the base without changing `platform-default`.

The `zshenmez` project is only external reference evidence for reusable capability coverage. `platform-go` is not a business migration target: it does not ship concrete `zshenmez` business resources, routes, stores, state machines, menus, fixtures or write-cutover plans. External business packages must live outside this foundation and attach through the public capability contracts.

## Active Completion Program

The original 37-node foundation baseline remains implemented and closed. The active completion program adds eight controlled pending nodes, so the current governance state is `45 total / 37 implemented / 8 controlled unfinished` with `completionStatus=not-complete-controlled`. This does not reopen the baseline and does not approve production auth promotion, refresh-token-family default runtime, source writing or external publication.

Approved 2026-07-12 specifications:

- [completion program](docs/superpowers/specs/2026-07-12-platform-go-completion-program-design.md);
- [runtime security hardening](docs/superpowers/specs/2026-07-12-runtime-security-hardening-design.md);
- [admin watermark and export governance](docs/superpowers/specs/2026-07-12-admin-watermark-export-design.md);
- [sensitive data encryption](docs/superpowers/specs/2026-07-12-sensitive-data-encryption-design.md);
- [open-source documentation and site](docs/superpowers/specs/2026-07-12-open-source-docs-site-design.md).

## Run

Start the API health smoke:

```bash
rtk proxy sh -lc 'PLATFORM_HTTP_ADDR=127.0.0.1:19200 go run ./cmd/platform-api & pid=$!; trap "kill $pid 2>/dev/null || true" EXIT; for i in $(seq 1 20); do curl -fsS http://127.0.0.1:19200/api/health && exit 0; sleep 0.5; done; exit 1'
```

Default API base:

```text
http://127.0.0.1:9200/api
```

Start the admin app:

```bash
rtk npm --prefix admin install
rtk npm --prefix admin run dev
```

Default admin URL:

```text
http://127.0.0.1:9202
```

The admin dev server proxies `/api` to `http://127.0.0.1:9200`. Use `VITE_PLATFORM_API_PROXY_TARGET` for another local API target. Use `VITE_PLATFORM_API_BASE` only when the browser should call an absolute API base directly.

## Useful APIs

```text
GET /api/health
GET /api/openapi.json
GET /api/capabilities
GET /api/platform/branding
GET /api/platform/cache/stats
GET /api/auth/providers
POST /api/auth/providers/:provider/start
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET /api/admin/session/current
GET /api/admin/menus
GET /api/admin/demo-data
POST /api/admin/demo-data/:capability/:dataset/apply
GET /api/admin/resources/:resource/schema
POST /api/admin/resources/:resource/query
GET /api/admin/resources/:resource
POST /api/admin/resources/:resource
PUT /api/admin/resources/:resource/:id
DELETE /api/admin/resources/:resource/:id
POST /api/admin/files/upload
GET /api/admin/files/:id/content
POST /api/app/auth/login
POST /api/app/auth/logout
POST /api/app/files
GET /api/app/files/:id/content
GET /api/app/session/current
```

See `docs/platform-auth.md`, `docs/admin-rbac-menu.md`, `docs/admin-resource-schema.md`, `docs/platform-branding.md`, `docs/platform-cache.md` and `docs/platform-capability-development.md` for detailed contracts.

## Contract Gates

The platform-level admin manifest lives at `resources/admin-resources.json`. Enabled `capability.Manifest.Admin.Resources` entries are merged into generated artifacts and validation gates:

```bash
rtk node scripts/generate-admin-resource-contract.mjs
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk go run ./cmd/platform-contracts app-routes --output resources/generated/app-route-contract.json
rtk go run ./cmd/platform-contracts audit --output resources/generated/platform-capability-audit.json
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/generate-admin-scaffold-promotion-review.mjs
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
rtk node scripts/generate-platform-operations-plan.mjs
rtk node scripts/generate-production-auth-promotion-review.mjs
rtk node scripts/generate-platform-promotion-evidence-templates.mjs
rtk node scripts/generate-platform-promotion-evidence-package-draft.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk node scripts/validate-platform-reference-coverage.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-personnel-runtime-readiness.mjs
rtk node scripts/validate-platform-engineering-capabilities.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>
rtk node scripts/validate-platform-file-storage-experience.mjs
rtk node scripts/validate-platform-refresh-token-family-promotion.mjs
rtk node scripts/run-platform-production-preflight.mjs --list
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk npm --prefix admin run build
```

`validate-admin-resources.mjs` checks manifest shape, i18n, relations, governance fields and generated artifact freshness. On the default manifest it also runs capability contracts, capability profiles, reference discovery, reference coverage and engineering capability validators, so resource changes cannot bypass plugin-style capability classification or `zshenmez` reference evidence gates. `validate-platform-admin-api-boundary.mjs` checks that admin UI requests stay behind the shared platform API client and that list queries remain structured, schema-whitelisted and SQL-injection safe. `validate-platform-app-client-api-boundary.mjs` checks that downstream App, H5 and mini-program clients stay behind generated App clients or platform request/upload ports instead of page-level request, upload or Authorization wiring; the default file-storage App routes are `POST /api/app/files` and `GET /api/app/files/:id/content`. `validate-platform-foundation-alignment.mjs` is the separate top-level objective and conflict gate; it cross-checks the approved stack, task graph, engineering matrix, reference discovery and coverage, capability boundaries, visual design gates, source-writing safety and production preflight expectations. The admin-resource gate does not recursively call it.

`validate-platform-governance-topology.mjs` is the architecture boundary for tenant, org-unit, role-group, area-code and optional personnel decisions. It rejects tenant-only regressions, keeps role groups classification-only, keeps address codes as shared regional master data with explicit area scopes, and keeps personnel resources out of the default foundation.

`validate-platform-form-schema-layout-slots.mjs` is the contract boundary for schema-driven form layouts and slots. It keeps single-column, grouped-section, two-column-density and side-detail-preview forms implemented through controlled schema metadata, source-level React slots and allowlisted runtime slot descriptors. Backend manifests may declare slot descriptors, but React renderers stay in frontend registries; source writing, backend React component names, dynamic component paths and raw scripts remain forbidden.

`validate-platform-capability-contracts.mjs` is the plugin-style capability governance boundary. It keeps `resources/platform-capability-contracts.json` aligned with profile declarations and audited Go manifests, classifies default, optional, demo-only and external-business capabilities, and rejects default-profile business leakage or manifest surface drift.

`validate-platform-cache-invalidation.mjs` is the cache boundary for Redis and local invalidation. It keeps cache targets, resource invalidation rules, Redis pub/sub channel, session reload behavior and no-cache policy aligned with runtime code and docs.

`validate-platform-deployment-topology.mjs` is the deployment boundary. It keeps Vercel optional for `admin` static hosting, keeps the Gin API on a long-lived-service default, and requires a separate adapter spec before any full-stack Vercel runtime promotion.

`validate-platform-task-execution-audit.mjs` is the execution-state boundary for the roadmap. It keeps task evidence, resource-lock/dependency conflicts and future promotion blockers visible, requires the submitted promotion evidence package validator, and preserves external artifact URI requirements before runtime mutation.

`validate-platform-goal-completion-audit.mjs` is the completion-claim boundary for the active foundation goal. It keeps `resources/platform-goal-completion-audit.json` aligned with the task graph, business-neutral reference policy, approved stack, deployment topology, disabled refresh-token-family default runtime, disabled source-writing mode and future promotion evidence gates, so foundation completion cannot be mistaken for production runtime mutation approval.

`validate-platform-node-closeout-audit.mjs` is the node closeout boundary. It keeps `resources/platform-node-closeout-audit.json` aligned with the task graph so every implemented node has neat-freak cleanup evidence, docs/tests/validator evidence, resource-lock review, objective-conflict review and visual design-gate evidence where required.

`validate-platform-objective-conformance.mjs` is the persistent objective boundary. It keeps `resources/platform-objective-conformance.json` aligned with the approved stack, `zshenmez` reference-only policy, capability-contract governance, future promotion gates, visual gate order, Vercel deployment boundary, production preflight catalog, task-graph evidence preflight, actual-stack evidence preflight and node closeout knowledge cleanup.

`validate-platform-reference-coverage.mjs` is the reference drift boundary. It reads the current `zshenmez/resources/admin-resources.json` through `resources/platform-reference-discovery.json` by default, so newly added reference admin resources must be classified as foundation, optional extension or business-only before platform coverage can pass. Reference business boundaries must stay owned by the abstract `external-business-capability`, and business resource parity owners must match their boundary owner.

`generate-platform-promotion-evidence-templates.mjs` and `validate-platform-promotion-evidence-templates.mjs` provide draft-only evidence package templates for the two controlled promotion gates: production auth hardening and source-writing codegen. The generated templates stay `draft-template` / `not-submitted`; they contain required fields from the approval schemas but no completed approval values, and the validator rejects runtime mutation, submitted approval state or sensitive fields. The templates and draft package also carry read-only `reviewContext` for provider controls, source-writing target families, review artifacts and preflight commands, so external reviewers can fill evidence without guessing the platform contract. `generate-platform-promotion-evidence-package-draft.mjs` turns those templates into `resources/generated/platform-promotion-evidence-package-draft.json`, a non-submitted package external reviewers can fill. When reviewers submit a completed package, validate it with `rtk node scripts/validate-platform-promotion-evidence-package.mjs --package <promotion-evidence-package>`; that check proves package shape, anti-self-approval, production-auth provider/control/runtime-test coverage, source-writing target-family/runtime-target/test-command coverage, external absolute `https`/`s3`/`gs` artifact URIs, `sha256:` plus 64 lowercase hex artifact hashes, `rtk` verification commands, rollback commands and sensitive-field redaction without writing approval state back into the platform contracts.

`validate-platform-file-storage-experience.mjs` gates the implemented file-storage admin experience. It keeps the UI aligned with the generic resource console, locks preview/download/audit requirements, and preserves the Product Design, i18n, build and browser evidence needed before future visual changes.

`validate-platform-refresh-token-family-promotion.mjs` gates the independent refresh-token-family runtime slice. The slice is implemented but disabled by default: the current `/api/auth/refresh` runtime stays sliding renewal backed by server-side sessions, raw refresh-token persistence is rejected, database-authoritative token-family storage is required, and production enablement is tied to production auth and production readiness contracts.

Source-writing code generation remains disabled. Scaffold artifacts under `resources/generated/` are review packets, not runtime source changes; promotion requires platform/codegen/runtime/operations approvals, reviewed diffs, rollback evidence, target-family tests and artifact-backed completed evidence.

## Production Baseline

`PLATFORM_RUNTIME_ENV` defaults to `development`. Production mode fails fast unless persistent stores, a non-default JWT secret and Redis cache/invalidation are configured.

Production baseline: set `PLATFORM_RUNTIME_ENV=production`, a non-default `PLATFORM_JWT_SECRET`, GORM driver/DSN pairs for admin resources, sessions and lifecycle history, plus `PLATFORM_CACHE_DRIVER=redis` and `PLATFORM_REDIS_ADDR`.
Production `PLATFORM_CAPABILITIES` must also be non-empty, remove `demo-data`, contain only lowercase letters, numbers and hyphenated capability IDs, and avoid empty or duplicate comma-separated entries. Use the `minimal-admin` profile for the smallest supported foundation. `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true` must be set so demo login is filtered from provider discovery and login.

```bash
PLATFORM_RUNTIME_ENV=production
PLATFORM_CAPABILITIES=tenant,identity,session,rbac,menu,api-resource,audit,admin-oidc,dictionary,parameter,file-storage,admin-shell,system-admin
PLATFORM_JWT_SECRET=<at-least-32-characters-and-not-the-dev-default>
PLATFORM_ADMIN_RESOURCE_DRIVER=mysql
PLATFORM_ADMIN_RESOURCE_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_SESSION_DRIVER=mysql
PLATFORM_SESSION_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_LIFECYCLE_HISTORY_DRIVER=mysql
PLATFORM_LIFECYCLE_HISTORY_DSN=user:pass@tcp(localhost:3306)/platform
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=127.0.0.1:6379
PLATFORM_RATE_LIMIT_HMAC_KEY=replace-with-dedicated-rate-limit-key
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
PLATFORM_ADMIN_OIDC_ISSUER_URL=https://identity.example/realms/platform
PLATFORM_ADMIN_OIDC_CLIENT_ID=platform-admin
PLATFORM_ADMIN_OIDC_CLIENT_SECRET=<redacted-secret>
PLATFORM_ADMIN_OIDC_REDIRECT_URL=https://admin.example/login
PLATFORM_ADMIN_OIDC_SCOPES=openid,profile,email
```

When `admin-oidc` is the production Admin provider, provision an existing enabled Admin user through `platform-admin bind-admin-oidc --subject-stdin` before API startup. OIDC authentication never creates platform users, roles, permissions, tenants, organizations or areas automatically. See `docs/platform-auth.md` for the stdin-only binding procedure and readiness gate.

Run `rtk node scripts/generate-platform-operations-plan.mjs`, `rtk node scripts/generate-production-auth-promotion-review.mjs` and `rtk node scripts/validate-platform-production-readiness.mjs` before release or production configuration work. Use `rtk node scripts/run-platform-production-preflight.mjs --list` to inspect the declared preflight catalog, `rtk node scripts/run-platform-production-preflight.mjs --policy <policy-id>` to dry-run a policy-specific command set, and add `--run` only when the operator is ready to execute the selected checks. Production operation policies and their policy-level preflight requirements are contract-first and non-mutating by default:

- `config-backup-export`
- `config-import-restore`
- `database-migration`
- `token-rotation`

Use `rtk node scripts/validate-platform-production-env.mjs` to verify the standard production env template. For a private deployment env, run `rtk node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>` for dry-run review, then add `--run` to execute the strict check before any release operation; strict mode rejects copied placeholders, `demo-data`, demo auth, non-Redis cache and non-GORM production stores.

Production transport is initialized with `PLATFORM_PUBLIC_BASE_URL`, `PLATFORM_TRUSTED_PROXIES`, `PLATFORM_EDGE_TRUSTED_PROXY` and `PLATFORM_HTTP_MAX_BODY_BYTES`. The public base must be an HTTPS origin, trusted proxies must be exact IPs or narrow CIDRs that do not cumulatively cover an address family, and the standard Admin container binds `127.0.0.1:8080` behind a reviewed external TLS edge. `PLATFORM_EDGE_TRUSTED_PROXY` is one canonical direct peer IP for that edge; CIDRs, loopback, unspecified and multicast addresses are rejected, and the IP must stay inside `PLATFORM_INTERNAL_SUBNET` in the standard Compose topology. Nginx accepts real client IP and forwarded protocol only from that peer, then overwrites `X-Forwarded-For` with one canonical client IP before proxying to the API. The API ignores non-canonical, duplicate and untrusted forwarded HTTPS headers. Only direct loopback `GET /api/health` may use HTTP without HSTS; all other production HTTP requests use the canonical redirect. The global body limit covers every non-upload request body, while the two multipart file routes retain their Task 4 upload-specific boundary.

`resources/generated/platform-operations-plan.json` is a review artifact. It keeps `dryRun=true`, `runtimeMutation=disabled` and `sourceWriting=disabled`, exposes each policy's required and missing preflight gates for review, and carries the Provider Promotion Matrix plus the production approval completed-evidence schema from `resources/platform-production-auth-hardening.json` so production credential work cannot ignore provider-specific controls, rollback commands, audit samples or refresh-token-family test evidence. `resources/generated/production-auth-promotion-review.json` is the narrower production-auth review packet; it stays `not-approved` while the implemented refresh-token-family slice remains disabled by default and external approval evidence is missing.

Deployment scheme A is selected as the default: build and run `cmd/platform-api` as a long-lived service, serve `admin/dist` from the same origin where practical, and keep browser API calls on `/api`. Vercel is optional for hosted admin previews or static delivery only; cross-origin `VITE_PLATFORM_API_BASE` requires a reviewed platform-level CORS slice. The standard deployment adapter package is `Dockerfile`, `deploy/compose/docker-compose.prod.yml`, `deploy/nginx/platform.conf` and `deploy/env/production.example.env`; the optional admin-only Vercel template is `deploy/vercel/admin.vercel.json`. See `docs/platform-deployment.md`.

## Documentation Map

- `AGENTS.md`: repository rules for AI agents and required command prefixes.
- `docs/platform-stack-alignment-audit.md`: stack correction and parity audit.
- `docs/platform-foundation-task-map.md`: roadmap, dependency graph and current P0/P1/P2 work.
- `docs/platform-capability-development.md`: capability, manifest, app route, persistence, file storage and extension rules.
- `docs/admin-resource-schema.md`: resource schema, query, relation and codegen contracts.
- `docs/admin-rbac-menu.md`: RBAC, dynamic menu and data-scope behavior.
- `docs/platform-auth.md`: admin/app JWT, provider, API-token and WeChat boundaries.
- `docs/admin-ui-foundation.md`: admin shell, shared UI components, themes and visual QA.
- `docs/platform-cache.md`: noop/memory/Redis cache and invalidation behavior.
- `docs/platform-deployment.md`: deployment topology, Vercel boundary and production API runtime requirements.
- `docs/platform-demo-data.md`: demo dataset contract.
- `docs/platform-branding.md`: branding API and settings resource.
- `docs/platform-roadmap.md`: broader extraction roadmap and remaining work.

## CodeGraph

Keep `.codegraph/` local and ignored. Use it before editing shared platform contracts, UI primitives, repositories, authorization or route registration:

```bash
rtk codegraph sync .
rtk codegraph status
```

## Verification

Run the narrowest relevant checks first, then broaden when shared contracts changed:

```bash
rtk go test ./...
rtk node --test scripts/*.test.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node --test scripts/admin-ui-contracts.test.mjs
rtk node scripts/validate-platform-governance-topology.mjs
rtk node scripts/validate-platform-form-schema-layout-slots.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-cache-invalidation.mjs
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk node scripts/validate-platform-task-execution-audit.mjs
rtk node scripts/validate-platform-goal-completion-audit.mjs
rtk node scripts/validate-platform-node-closeout-audit.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk node scripts/validate-platform-promotion-evidence-templates.mjs
rtk node scripts/validate-platform-file-storage-experience.mjs
rtk node scripts/validate-platform-refresh-token-family-promotion.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-platform-foundation-alignment.mjs
rtk node scripts/validate-platform-reference-discovery.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```
