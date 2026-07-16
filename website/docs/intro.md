---
sidebar_position: 1
title: 介绍
---

# platform-go

`platform-go` 是面向 Go 服务的业务中立运营平台底座，提供能力清单、认证、RBAC、菜单、资源合同、审计、文件存储和部署治理。

## 五分钟开始

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

API 默认监听 `http://127.0.0.1:9200`。生产部署前请阅读认证和部署文档。

## 继续阅读

- [能力与扩展](./capabilities)
- [运维与安全](./operations)
- [完整开发指南](https://github.com/GnosiST/platform-go/blob/main/docs/platform-capability-development.md)
