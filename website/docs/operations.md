---
sidebar_position: 3
title: 运维与安全
---

生产环境必须使用非默认 JWT 密钥、持久化存储、Redis（按生产基线）和禁用 demo 认证。迁移、密钥轮换、备份恢复和回滚应先完成演练，再执行变更。

> English: Production requires a non-default JWT secret, durable storage, Redis and demo authentication disabled. Use the language switcher for the complete English guide.

详见 [认证指南](https://github.com/GnosiST/platform-go/blob/main/docs/platform-auth.md) 与 [部署指南](https://github.com/GnosiST/platform-go/blob/main/docs/platform-deployment.md)。

## 发布前检查

```bash
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/validate-platform-goal-completion-audit.mjs
node scripts/validate-platform-node-closeout-audit.mjs
```

## 故障处理

先用 `request_id` 关联 API 日志和审计，再确认能力 profile、数据源、缓存和会话状态。恢复操作必须记录影响范围、操作者、前置备份和回滚结果；不要直接修改生产数据库绕过平台合同。
