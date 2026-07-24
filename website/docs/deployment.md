---
sidebar_position: 2
title: 部署指南
---

# 部署指南

默认拓扑是长期运行的 Gin API 加已审阅的 TLS 边缘。Admin 可以由同源 Nginx 提供静态文件，也可以部署为独立静态站点并通过受审阅的 `/api/*` 代理或 CORS 契约访问 API。Vercel 仅是 Admin 静态托管适配器，不改变 API 长生命周期边界。

## 推荐拓扑

```text
Browser -> reviewed HTTPS edge -> Nginx/Admin static -> Gin API -> durable stores + Redis
```

- 可信边缘终止 HTTPS，规范重定向使用 `PLATFORM_PUBLIC_BASE_URL`，而不是请求的 Host。
- API 只接受来自明确定义代理 IP 的转发头；不要把任意客户端的 `X-Forwarded-For` 或协议头视为可信。
- 健康检查使用 loopback `GET /api/ready`；`GET /api/health` 保持为 liveness。
- 上传文件只经认证内容端点交付，不能配置 Nginx `/uploads`、`alias` 或公开对象存储 URL。

## 最小生产配置

```text
PLATFORM_RUNTIME_ENV=production
PLATFORM_PUBLIC_BASE_URL=https://platform.example.test
PLATFORM_JWT_SECRET=<dedicated-secret>
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_ADMIN_RESOURCE_DRIVER=<certified-gorm-driver>
PLATFORM_ADMIN_RESOURCE_DSN=<private-dsn>
PLATFORM_SESSION_DRIVER=<certified-gorm-driver>
PLATFORM_SESSION_DSN=<private-dsn>
PLATFORM_LIFECYCLE_HISTORY_DRIVER=<certified-gorm-driver>
PLATFORM_LIFECYCLE_HISTORY_DSN=<private-dsn>
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=<redis-address>
PLATFORM_CAPABILITIES=<explicit-profile>
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
```

配置 `PLATFORM_TRUSTED_PROXIES`、`PLATFORM_EDGE_TRUSTED_PROXY`、独立的数据加密/盲索引 keyring 和 `PLATFORM_RATE_LIMIT_HMAC_KEY`。私有 DSN、密钥和 provider credential 不进入 Git、日志或支持工单。完整模板与逐项校验规则以 [`deploy/env/production.example.env`](https://github.com/GnosiST/platform-go/blob/main/deploy/env/production.example.env) 为准。

## 发布流程

1. 对私有环境文件执行严格 preflight，并确认 Compose/Nginx 配置与审阅拓扑一致。
2. 为 schema、数据、权限或 key 变更准备备份、恢复演练、停止条件与 rollback。
3. 部署 API 与 Admin，监控 readiness、错误率、request ID 和红脱敏审计。
4. 只有在受控验证完成后才开启 credential auth、SMS、消息、搜索或多数据源等 default-off 集成。

```bash
node scripts/validate-platform-production-env.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

GitHub Pages 由 `.github/workflows/pages.yml` 在 `main` 推送后构建 `website/` 并部署。文档站默认中文，英文入口为 `/platform-go/en/`。Pages 是静态文档发布，不承担 API 运行时。
