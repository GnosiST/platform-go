# platform-go

> A business-neutral operations foundation for Go services. Establish identity, authorization, resource contracts and runtime governance before adding business capabilities.

[![CI](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-2f6f9f.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg)](go.mod)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-0b7285.svg)](https://gnosist.github.io/platform-go/en/)

[Chinese README](README.md) · [English docs](https://gnosist.github.io/platform-go/en/) · [Chinese docs](https://gnosist.github.io/platform-go/)

## Positioning

`platform-go` is a reusable, auditable and extensible foundation for Go operations services. It is intentionally business-neutral: domain resources, menus and workflows stay outside platform core and attach through capability manifests, public ports and versioned contracts.

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

The persistent full-scope unfinished inventory covers `multi-datasource-contract-and-runtime`, `tenant-placement-and-request-routing`, `datasource-read-write-routing`, `sharding-and-tenant-migration`, `federated-read-query`, `xa-optional-adapter`, `database-certification-matrix`, `transactional-outbox-and-one-mq-adapter`, `asynchronous-search-projection`, `open-source-portability`, `public-docs-community`, `public-docs-site` and `github-release-publication`. v0.1.0 supports one datasource and one native transaction boundary. SQLite is development/test-only by support policy; Oracle and KingbaseES are unsupported. `alibaba/page-agent` is only a default-off optional `public-docs-site` sub-capability.

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

## Support boundary

The default release supports one datasource and one native transaction boundary. Production must explicitly configure a non-default JWT secret, encryption keyrings, durable stores, Redis and a capability profile; demo data and demo login stay disabled.

Platform owns shared mechanisms. Business code must not reach into concrete platform storage, HTTP handlers or Admin internals; use the public capability, service, query/command and storage-port contracts.

## Documentation

- [Capability development guide](docs/platform-capability-development.md)
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
