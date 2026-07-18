---
sidebar_position: 1
title: 介绍
---

# platform-go

`platform-go` 是面向 Go 服务的业务中立运营平台底座，提供能力清单、认证、RBAC、菜单、资源合同、审计、文件存储和部署治理。

> English: `platform-go` is a business-neutral Go foundation for reusable operations services. See the language switcher in the top navigation for the complete English version.

## 五分钟开始

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

API 默认监听 `http://127.0.0.1:9200`。生产部署前请阅读认证和部署文档。

## 你将获得什么

- 一套能力 manifest 和资源 schema；
- JWT 会话、RBAC、组织、角色组和菜单治理；
- Admin 通用资源控制台、审计和数据生命周期；
- OpenAPI、代码生成预览、生产 preflight 和发布证据。

## 阅读路线

新用户建议按“[架构总览](./architecture) → [开发指南](./development) → [人机协同开发](./human-ai-development) → [能力与扩展](./capabilities) → [API 与合同](./api)”阅读；部署和安全人员可直接查看“[安全边界](./security) → [部署指南](./deployment)”。

## 继续阅读

- [能力与扩展](./capabilities)
- [人机协同开发](./human-ai-development)
- [运维与安全](./operations)
- [完整开发指南](https://github.com/GnosiST/platform-go/blob/main/docs/platform-capability-development.md)
