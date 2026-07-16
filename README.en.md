# platform-go

`platform-go` is a business-neutral operations platform foundation built for
Gin, GORM, Casbin and JWT services with a Refine, React and Ant Design admin
client.

It provides reusable capability manifests, authentication, RBAC and menus,
resource contracts, audit records, file storage, caching and deployment
guardrails. Business applications attach through public capability contracts;
the default foundation does not contain a concrete business domain.

## Quick Start

Requirements: Go, Node.js, npm and (for production) a supported database and
Redis instance.

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

The API listens on `http://127.0.0.1:9200` by default. The Admin development
server listens on `http://127.0.0.1:9202` and proxies `/api` to the API.

## Documentation

- [Platform capability development](docs/platform-capability-development.md)
- [Admin resource schema](docs/admin-resource-schema.md)
- [RBAC and menus](docs/admin-rbac-menu.md)
- [Authentication](docs/platform-auth.md)
- [Deployment](docs/platform-deployment.md)
- [Contribution guide](CONTRIBUTING.md)

## License

platform-go is licensed under the Apache License 2.0. See [LICENSE](LICENSE)
and [NOTICE](NOTICE).
