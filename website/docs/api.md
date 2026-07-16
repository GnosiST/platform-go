---
sidebar_position: 5
title: API 与合同
---

# API 与合同

平台 API 分为 Admin、App 和公共服务合同。Admin 资源查询使用结构化 JSON 条件，后端只接受 schema 白名单字段和参数化谓词。

## 常用入口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查 |
| GET | `/api/capabilities` | 能力发现 |
| GET | `/api/openapi.json` | OpenAPI 合同 |
| POST | `/api/admin/resources/:resource/query` | 资源查询 |
| GET | `/api/admin/resources/:resource/schema` | 资源 schema |
| GET | `/api/admin/session/current` | 当前 Admin 会话 |
| POST | `/api/app/auth/login` | App 登录 |

## 错误与追踪

错误响应应携带统一错误码、`request_id` 和可安全展示的消息。服务端日志可以通过 request ID 关联，但不得把 JWT、密钥、完整身份证号或明文敏感字段写入日志。

## 版本策略

公开服务合同必须声明稳定性、版本、幂等、超时、重试、限流和弃用策略。破坏性变更要先更新 OpenAPI/AsyncAPI 与 consumer contract tests。
