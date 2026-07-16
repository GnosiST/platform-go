---
sidebar_position: 8
title: 部署指南
---

# 部署指南

## API 服务

API 默认按长期运行的 Gin 服务部署，使用反向代理或容器编排提供 TLS、健康检查和滚动发布。Vercel 仅作为可选的 Admin 静态托管，不改变 API 的长期运行边界。

## 必要配置

```text
PLATFORM_RUNTIME_ENV=production
PLATFORM_JWT_SECRET=<non-default-secret>
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=<redis-address>
PLATFORM_CAPABILITIES=<explicit-profile>
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
```

发布前执行生产 preflight、迁移演练、备份恢复演练和回滚演练。不要在没有数据迁移证据的情况下切换存储驱动或权限模型。

## GitHub Pages

Pages 由 `.github/workflows/pages.yml` 在 `main` 推送后构建 `website/` 并部署到 GitHub Pages。文档站默认中文，英文入口为 `/platform-go/en/`。
