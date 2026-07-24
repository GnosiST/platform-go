---
sidebar_position: 4
title: API 与合同
---

# API 与合同

平台 API 分为 Admin、App 与公共能力合同。资源查询始终使用结构化 JSON 条件，后端只接受 schema 白名单字段和参数化谓词；不把 UI 中的搜索表达式作为 SQL 传输或执行。

## 服务入口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/health` | 进程 liveness |
| GET | `/api/ready` | 受限运行依赖 readiness |
| GET | `/api/capabilities` | 能力发现 |
| GET | `/api/openapi.json` | 生成的 OpenAPI 合同 |
| POST | `/api/admin/resources/:resource/query` | 结构化资源查询 |
| GET | `/api/admin/resources/:resource/schema` | Admin 资源 schema |
| GET | `/api/admin/session/current` | 当前 Admin session |

## 认证合同

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/auth/providers` | 发现已启用且已配置的登录提供方 |
| GET | `/api/auth/credential-secret-key` | credential secret transport 公钥信息 |
| POST | `/api/auth/challenges` | 创建 mandatory 登录 challenge |
| POST | `/api/auth/challenges/verify` | slider 预验证，成功不消费 challenge |
| POST | `/api/auth/sms-otp/start` | 在有效 challenge 后请求 OTP 发送 |
| POST | `/api/auth/login` | 认证并进入现有 session/JWT 边界 |
| POST | `/api/app/auth/login` | App 登录合同 |

请求和响应 schema 以 `/api/openapi.json` 及生成物为准。`credential-auth` 的 secret、OTP、challenge proof 和 client fingerprint 都是 write-only；不能把它们记录在浏览器日志、错误响应或通用资源字段中。

## 错误、追踪与演进

错误响应应包含稳定错误码、`request_id` 与可安全展示的消息。用 request ID 关联服务端日志，但不要记录 JWT、密钥、完整标识符、密码、OTP 或敏感字段明文。

公开合同变更必须同时更新 OpenAPI/AsyncAPI、consumer contract test、幂等/超时/重试/限流语义和弃用说明。任何破坏性变更先在下游消费者验证，再进入发布流程。
