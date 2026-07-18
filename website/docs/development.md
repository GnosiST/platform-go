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

平台能力可以在 `internal/platform` 内实现；具体业务能力优先放在下游仓库或下游 composition root，不直接修改平台核心。

1. 从仓库中的 `examples/external-capability` 复制或改造最小示例。
2. 只导入公开合同 `github.com/GnosiST/platform-go/pkg/platform/capability`。
3. 在 manifest 中声明资源 schema、菜单、权限、App 路由、服务合同、生命周期和演示数据。
4. 在下游 composition root 注入 storage、HTTP handler、Admin action handler 和可选 UI 注册。
5. 同步生成 OpenAPI、资源合同、代码生成预览和文档。
6. 添加单元测试、合同测试和必要的迁移/回滚证据。

示例门禁会检查外部包没有导入 `internal/platform/**`，并在示例目录内执行 `go test ./...` 与 `go run .`：

```bash
node scripts/validate-external-capability-example.mjs
```

## 常用检查

```bash
node scripts/validate-external-capability-example.mjs
node scripts/validate-admin-resources.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-admin-api-boundary.mjs
node scripts/validate-platform-foundation-alignment.mjs
npm --prefix admin run build
npm --prefix website run build
```

共享 Admin 组件应使用平台 UI wrapper，并同时维护中文和英文 i18n key。业务代码不应直接拼接 SQL 或绕过授权中间件。
