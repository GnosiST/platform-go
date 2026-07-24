---
sidebar_position: 2
title: 五分钟本地运行
---

# 五分钟本地运行

本指南的目标是让你看到一个可用的 API 健康响应和 Admin 页面。它不是生产部署指南，也不会初始化真实短信供应商或外部消息适配器。

## 前置条件

- Go 1.26+；
- Node.js 与 npm；
- 一个可写的本地工作目录。

开发默认值可以使用内存或 SQLite 边界。SQLite 只允许 development/test；生产必须使用持久化 Admin、session 和 lifecycle 存储，并启用 Redis。

## 启动 API

```bash
git clone https://github.com/GnosiST/platform-go.git
cd platform-go

go run ./cmd/platform-api
```

默认监听地址是 `http://127.0.0.1:9200`。在新终端检查存活：

```bash
curl -fsS http://127.0.0.1:9200/api/health
```

成功响应包含 `"ok":true`。`/api/health` 只表示进程存活；运行依赖的就绪状态使用 `/api/ready`，不应把其内部细节公开到无认证页面。

## 启动 Admin

```bash
npm --prefix admin install
npm --prefix admin run dev
```

打开 [http://127.0.0.1:9202](http://127.0.0.1:9202)。Vite 开发服务器默认把 `/api/` 代理到 `http://127.0.0.1:9200`；若你改变 API 地址，使用 `VITE_PLATFORM_API_PROXY_TARGET` 调整本地代理。

开发环境可验证 demo/OIDC 壳层和资源界面。生产环境必须关闭 demo provider，不能把本地账号、调试验证码、开发 JWT 密钥或 SQLite 数据库当作上线配置。

## 通过基础门禁

```bash
go test ./...
node scripts/validate-admin-resources.mjs
npm --prefix admin run build
npm --prefix website run build
```

API、Admin 和文档构建都通过后，再进入[业务能力接入](./business-integration)。需要认证配置时先阅读[认证与授权](./authentication)；需要部署时使用[生产检查清单](./production-checklist)。

## 常见问题

| 现象 | 优先检查 |
| --- | --- |
| `9200` 无法访问 | API 进程是否仍在运行、端口是否被占用、`PLATFORM_HTTP_ADDR` 是否被覆盖 |
| Admin 页面没有 API 数据 | `/api/health` 是否成功、Vite 代理目标是否指向 `9200`、浏览器请求是否命中 `/api/` |
| 生产配置启动失败 | 不要删校验；从 `deploy/env/production.example.env` 复制私有文件并运行生产预检 |
| 登录提供方未出现 | 提供方由 discovery 决定；检查 capability、配置和生产 demo 禁用开关 |
