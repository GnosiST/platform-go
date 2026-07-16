---
sidebar_position: 3
title: 开发指南
---

# 开发指南

## 环境准备

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run dev
```

API 默认监听 `http://127.0.0.1:9200`，Admin 默认监听 `http://127.0.0.1:9202`。

## 添加一个能力

1. 在 `internal/platform` 或业务包中定义 manifest，不把业务菜单硬编码进平台核心。
2. 为资源提供 schema、权限前缀、租户模式和生命周期声明。
3. 通过 composition root 注入 storage、HTTP 和 Admin 注册，而不是跨层引用具体实现。
4. 同步生成 OpenAPI、资源合同、代码生成预览和文档。
5. 添加单元测试、合同测试和必要的迁移/回滚证据。

## 常用检查

```bash
node scripts/validate-admin-resources.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-admin-api-boundary.mjs
node scripts/validate-platform-foundation-alignment.mjs
npm --prefix admin run build
npm --prefix website run build
```

共享 Admin 组件应使用平台 UI wrapper，并同时维护中文和英文 i18n key。业务代码不应直接拼接 SQL 或绕过授权中间件。
