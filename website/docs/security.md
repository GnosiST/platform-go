---
sidebar_position: 6
title: 安全边界
---

# 安全边界

## 默认原则

- JWT 只用于身份承载，会话撤销由服务端 session 控制。
- Casbin 后端授权是最终安全边界，菜单只是可见性提示。
- 敏感字段按 manifest 配置加密、盲索引、脱敏和 reveal step-up，不使用固定字段名猜测。
- 普通客户端不能提交 DSN、物理数据源、数据库 schema、shard 或权限绕过参数。
- Redis 是缓存优化，不是权限或业务数据真相源。

## 生产基线

生产环境必须设置非默认 `PLATFORM_JWT_SECRET`、持久化数据库、数据加密 keyring、Redis 和显式能力 profile，并关闭 demo auth/provider。

## 数据生命周期

删除默认采用逻辑删除，restore 与 purge 是分开的受审计流程。定时清除必须经过 retention policy、租户范围和回滚窗口校验。
