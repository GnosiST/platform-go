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
