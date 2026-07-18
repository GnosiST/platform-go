# platform-go

> A business-neutral operations foundation for Go services. Establish identity, authorization, resource contracts and runtime governance before adding business capabilities.

[![CI](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-2f6f9f.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg)](go.mod)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-0b7285.svg)](https://gnosist.github.io/platform-go/en/)

[English](README.en.md) · [简体中文](README.md) · [Documentation / 文档站](https://gnosist.github.io/platform-go/en/)

## Positioning

`platform-go` is a reusable, auditable and extensible foundation for Go operations services. It is intentionally business-neutral: domain resources, menus and workflows stay outside platform core and attach through capability manifests, public ports and versioned contracts.

`platform-go` is not a business migration target or an application package. It stays business-neutral and does not ship domain-specific resources, routes, stores, state machines, menus or workflows.

It is a good fit for teams that need to:

- build long-running services with Gin and GORM;
- standardize authentication, sessions, tenancy, organizations, RBAC, menus and audit;
- keep frontend/backend work aligned through resource schemas, OpenAPI and code generation;
- preserve migration, cache, file-storage and release-governance boundaries in production.

## Core capabilities

| Area | Included by default |
| --- | --- |
| Platform contracts | capability manifests, resource schemas, route and permission declarations, versioned artifacts |
| Identity and authorization | JWT sessions, server-side revocation, Casbin RBAC, tenant/org scopes and menu visibility |
| Admin resources | Refine + React + Ant Design shell, generic lists/forms, audit and lifecycle operations |
| Engineering governance | OpenAPI, App route contracts, Go/TypeScript codegen previews, validators and release evidence |
| Runtime foundation | GORM persistence, Redis cache port, file storage, branding and production preflight |

Optional profiles keep personnel, notification, jobs and enterprise governance detachable. Multi-datasource routing, sharding, federation, XA, MQ and search projections are future extensions and are not presented as default runtime support.

<details>
<summary>Implementation status and support boundary (maintainer reference)</summary>

The current governance snapshot is **67 total / 58 implemented / 9 controlled unfinished**. `runtime-security-containment`, `sensitive-data-protection-runtime`, `sensitive-data-historical-migration`, `admin-watermark-export-governance` and `organization-user-admin-experience` are `implemented`.

The persistent full-scope unfinished inventory contains only nine `deferred` nodes: `multi-datasource-contract-and-runtime`, `tenant-placement-and-request-routing`, `datasource-read-write-routing`, `sharding-and-tenant-migration`, `federated-read-query`, `xa-optional-adapter`, `database-certification-matrix`, `transactional-outbox-and-one-mq-adapter` and `asynchronous-search-projection`. `open-source-portability`, `public-docs-community`, `public-docs-site` and `github-release-publication` are `implemented`. v0.1.0 is formally published: its [GitHub Release](https://github.com/GnosiST/platform-go/releases/tag/v0.1.0), Tag, CI and Pages evidence are verified. The release supports one datasource and one native transaction boundary. SQLite is development/test-only by support policy; Oracle and KingbaseES are unsupported. `alibaba/page-agent` is only a default-off optional `public-docs-site` sub-capability.

Policy review routes:

```text
POST /api/admin/policy-reviews/:id/request
POST /api/admin/policy-reviews/:id/reject
POST /api/admin/policy-reviews/:id/approve
GET /api/admin/policy-reviews/export
```

</details>

## Technology

- **Backend**: Go, Gin, GORM, Casbin, JWT
- **Admin**: React, Refine, Ant Design, TypeScript
- **Contracts and docs**: OpenAPI, Docusaurus, GitHub Pages
- **Optional runtime dependency**: Redis; database and external adapters follow the support matrix

## Quick start

Requirements: Go, Node.js and npm. Production also requires a durable database and Redis.

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

The API defaults to `http://127.0.0.1:9200`; the Admin development server defaults to `http://127.0.0.1:9202`.

### Development Target Role Menus

When validating role-menu saves locally, bootstrap a fresh SQLite development database into target RBAC and menu-write state explicitly. This path accepts only `development + sqlite + target serving + role-menu write enabled`, and only materializes an empty database; production and staging never run it automatically.

```bash
export PLATFORM_ADMIN_RESOURCE_DSN=<local-sqlite-admin-resource-dsn>

PLATFORM_RUNTIME_ENV=development \
PLATFORM_ADMIN_RESOURCE_DRIVER=sqlite \
PLATFORM_ORGANIZATION_RBAC_MODE=target \
PLATFORM_ADMIN_MENU_SERVING_MODE=target \
PLATFORM_ADMIN_ROLE_MENU_WRITE_ENABLED=true \
go run ./cmd/platform-admin organization-rbac-migrate --mode bootstrap-development
```

Then start the API with the same environment before running the Admin development server:

```bash
export PLATFORM_ADMIN_RESOURCE_DSN=<local-sqlite-admin-resource-dsn>

PLATFORM_RUNTIME_ENV=development \
PLATFORM_ADMIN_RESOURCE_DRIVER=sqlite \
PLATFORM_ORGANIZATION_RBAC_MODE=target \
PLATFORM_ADMIN_MENU_SERVING_MODE=target \
PLATFORM_ADMIN_ROLE_MENU_WRITE_ENABLED=true \
go run ./cmd/platform-api

npm --prefix admin run dev
```

Production role-menu writes still require a reviewed migration packet, database checkpoint, dual-read comparison and audited `promote` operation; do not use the development bootstrap as a production migration entry point.

<details>
<summary>Production baseline and non-mutating preflight</summary>

Production baseline: set `PLATFORM_RUNTIME_ENV=production` and explicitly configure durable stores, Redis, a trusted HTTPS edge, independent secrets and disabled demo authentication. See [Deployment and production baseline](docs/platform-deployment.md) for values, constraints and rotation procedures. This README retains the initialization keys required by the machine contract:

```text
PLATFORM_RUNTIME_ENV
PLATFORM_PUBLIC_BASE_URL
PLATFORM_TRUSTED_PROXIES
PLATFORM_EDGE_TRUSTED_PROXY
PLATFORM_HTTP_MAX_BODY_BYTES
PLATFORM_JWT_SECRET
PLATFORM_DATA_KEY_PROVIDER
PLATFORM_ADMIN_RESOURCE_DRIVER
PLATFORM_ADMIN_RESOURCE_DSN
PLATFORM_SESSION_DRIVER
PLATFORM_SESSION_DSN
PLATFORM_LIFECYCLE_HISTORY_DRIVER
PLATFORM_LIFECYCLE_HISTORY_DSN
PLATFORM_CACHE_DRIVER
PLATFORM_REDIS_ADDR
PLATFORM_RATE_LIMIT_HMAC_KEY
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER
```

The retention runner stays disabled by default. Review `PLATFORM_RETENTION_RUNNER_ENABLED`, `PLATFORM_RETENTION_RUNNER_INTERVAL`, `PLATFORM_RETENTION_RUNNER_BATCH_SIZE` and `PLATFORM_RETENTION_RUNNER_MAX_RETRIES` together before enabling it.

List the non-mutating production checks first, then validate a private environment in strict mode. Run `node scripts/validate-platform-production-env.mjs` directly for the standard template:

```bash
node scripts/validate-platform-foundation-alignment.mjs
node scripts/run-platform-production-preflight.mjs --list
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

`config-backup-export`, `config-import-restore`, `database-migration` and `token-rotation` are production operation policies that require human review, rollback evidence and audit records. Preflight does not deploy, migrate or mutate production state.

Deployment scheme A is selected as the default: run the Gin API as a long-lived service and serve Admin assets from the same origin where practical. See [deployment documentation](docs/platform-deployment.md) and run `node scripts/validate-platform-deployment-topology.mjs` before changing this topology.

</details>

## Support boundary

The default release supports one datasource and one native transaction boundary. Production must explicitly configure a non-default JWT secret, encryption keyrings, durable stores, Redis and a capability profile; demo data and demo login stay disabled.

Platform owns shared mechanisms. Business code must not reach into concrete platform storage, HTTP handlers or Admin internals; use the public capability, service, query/command and storage-port contracts.

Capability operations are governed by [resources/platform-capability-operation-policy.json](resources/platform-capability-operation-policy.json): the platform supports restart-required desired state through profiles, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root. Contract regeneration plus a manual service restart makes the desired set effective. It does not support runtime hot-plugging, one-click remote repository pull or generic destructive uninstall. Disabled Admin resources, App routes, auth providers and demo data sets must disappear from exposed contracts, and foundation capabilities are non-removable.

Plugin management v1 is defined in [resources/platform-plugin-management-v1.json](resources/platform-plugin-management-v1.json). It uses a restart-required desired-state model: install or enable means declaring the desired capability set through a profile, `PLATFORM_CAPABILITIES`, `PLATFORM_CAPABILITY_LOCK_FILE` or a downstream composition root, regenerating contracts and manually restarting the service; disable means removing the capability from the desired set, regenerating contracts and restarting. A mismatch between the running process and desired set is only a `pendingRestart` state, not a hot apply path. v1 does not pull code from remote repositories, does not support runtime hot install/uninstall, does not provide destructive uninstall, and does not integrate WebSocket. Status and update notices use HTTP polling, `version.json` or an API version check.

To start a new business project on the foundation, do not place product-specific code in `platform-go` core. Prefer a downstream business repository or downstream composition root: declare the business capability manifest, resources, permissions, routes and lifecycle first, then keep storage, handlers, Admin panels and product tests downstream. Only behavior reusable across business domains should become a platform profile.

Before onboarding a business capability, copy or adapt the minimum example in
[examples/external-capability](examples/external-capability), then run the
standalone example gate to confirm that external packages rely only on public
contracts and can validate manifests, tests and contract previews:

```bash
rtk node scripts/validate-external-capability-example.mjs
```

Human developers and AI agents should follow the [Human + AI development protocol](docs/platform-human-ai-development-protocol.md) before starting a highly customized business system: declare interface, UI, visual, data and codegen contracts first, then verify the matching gates.

## Documentation

- [Capability development guide](docs/platform-capability-development.md)
- [Human + AI development protocol](docs/platform-human-ai-development-protocol.md)
- [Authentication, sessions and RBAC](docs/platform-auth.md)
- [Admin resources and menus](docs/admin-resource-schema.md)
- [Data lifecycle and retention](docs/platform-data-lifecycle-retention.md)
- [Sensitive data protection and migration](docs/platform-sensitive-data-migration.md)
- [Deployment and production baseline](docs/platform-deployment.md)
- [Public documentation site](https://gnosist.github.io/platform-go/en/)

## Contributing

Read the [contribution guide](CONTRIBUTING.md) before opening an issue or pull request. New capabilities should include manifests, contracts, tests, documentation and required migration/rollback evidence. Internal planning and AI process artifacts do not belong in the public repository.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
