---
sidebar_position: 1
title: Introduction
---

# platform-go

`platform-go` is a business-neutral Go foundation for reusable operations services. It provides capability manifests, authentication, RBAC, menus, resource contracts, audit, file storage and deployment governance.

## Five-minute start

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

The API listens on `http://127.0.0.1:9200` by default. Read the authentication and deployment guides before production use.

## What you get

- capability manifests and resource schemas;
- JWT sessions, RBAC, organization, role-group and menu governance;
- a generic Admin resource console, audit and data lifecycle;
- OpenAPI, codegen previews, production preflight and release evidence.

## Reading path

New contributors should read [Architecture overview](./architecture), [Development guide](./development), [Human + AI development](./human-ai-development), [Capabilities](./capabilities) and [API contracts](./api) in that order. Operators can jump directly to [Security](./security) and [Deployment](./deployment).

## Continue reading

- [Capabilities](./capabilities)
- [Human + AI development](./human-ai-development)
- [Operations and troubleshooting](./operations)
- [Complete capability development guide](https://github.com/GnosiST/platform-go/blob/main/docs/platform-capability-development.md)
