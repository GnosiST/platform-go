---
sidebar_position: 2
title: 配置参考
---

# 配置参考

配置按环境和能力边界组织。示例值、测试密钥与 DSN 不能复制到生产；完整私有模板位于 [`deploy/env/production.example.env`](https://github.com/GnosiST/platform-go/blob/main/deploy/env/production.example.env)。

## 基础运行时

| 配置 | 用途 |
| --- | --- |
| `PLATFORM_RUNTIME_ENV` | `development`、`test` 或 `production`；生产开启严格约束 |
| `PLATFORM_PUBLIC_BASE_URL` | 面向用户的规范 HTTPS 地址 |
| `PLATFORM_HTTP_MAX_BODY_BYTES` | 一般请求体上限；上传另受文件策略约束 |
| `PLATFORM_CAPABILITIES` | 启动前期望 capability 集合；不能包含生产 demo-data |
| `PLATFORM_CAPABILITY_LOCK_FILE` | 可审阅的 capability desired state 锁定文件 |

## 存储与缓存

`PLATFORM_ADMIN_RESOURCE_DRIVER/DSN`、`PLATFORM_SESSION_DRIVER/DSN` 和 `PLATFORM_LIFECYCLE_HISTORY_DRIVER/DSN` 负责三类持久化边界。生产使用受认证的 GORM 驱动和 DSN；SQLite 仅限 development/test。

`PLATFORM_CACHE_DRIVER=redis` 与 `PLATFORM_REDIS_ADDR` 是生产缓存和失效基线。Redis 是优化层，不是权限或业务事实来源。

## 身份与数据保护

`PLATFORM_JWT_SECRET`、`PLATFORM_RATE_LIMIT_HMAC_KEY`、数据加密 keyring 和盲索引 keyring 必须独立。keyring 用 version ID 管理历史读取；更新 active ID 前必须先部署新旧材料并验证读取和备份。

生产关闭 demo：`PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`。启用 `admin-oidc` 时还需要完整 issuer、client、redirect 和后台绑定流程。

## 可选能力

Credential auth 使用专用 repository、identifier HMAC key 和 P-256 secret transport key；notification/SMS、消息、搜索和多数据源都要求完整的对应 capability 与适配器配置。不要通过环境变量把一个不存在的 stock adapter 变成“已启用”。

修改生产配置前先执行[生产检查清单](./production-checklist)中的非变更式预检；配置契约、默认值与逐项限制以[部署文档](https://github.com/GnosiST/platform-go/blob/main/docs/platform-deployment.md)为准。
