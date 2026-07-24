---
sidebar_position: 2
title: 生产检查清单
---

# 生产检查清单

本清单用于发布前审阅。它不会替代私有环境审计、外部账号证据、数据库恢复演练或变更审批。

## 架构与网络

- API 作为长生命周期 Gin 服务运行；Admin 可以同源静态托管或作为单独静态站点。
- 公网 HTTPS 终止于已审阅的边缘，API 只信任明确的代理 IP；不能把任意 `X-Forwarded-*` 当作可信输入。
- `/api/health` 是 liveness；`/api/ready` 是受限依赖就绪检查。上传文件只能经认证内容端点交付，不能用 Nginx 静态目录公开。

## 配置与密钥

- `PLATFORM_RUNTIME_ENV=production`，`PLATFORM_PUBLIC_BASE_URL` 为规范 HTTPS URL。
- 设置独立 JWT secret、数据加密 keyring、盲索引 keyring、rate-limit HMAC key；所有 key material 彼此不同。
- 使用 GORM-backed Admin、session 和 lifecycle store，选择 Redis cache/invalidation，且设置 `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`。
- 私有环境文件不能提交。生产校验必须替换所有示例 placeholder，日志、trace 和审计中不得出现 token、DSN、完整收件人或 secret。

## 数据、文件与认证

- 默认按单数据源、单原生事务运行。多数据源、读写路由、分片、联邦、XA、消息与搜索只能在受控 contract/profile 下逐项推广。
- 文件对象 key 必须是不透明随机标识；S3 使用 HTTPS 和 `AES256` 或 `aws:kms`，并由运营方独立确认 bucket 无公开访问。
- 若启用 credential auth，配置专用存储、HMAC 和 P-256 transport key；手机号 OTP 还需要非 debug SMS provider 与模板。不要将 demo/OIDC 开发流程转为生产身份来源。

## 发布前验证

```bash
node scripts/validate-platform-production-env.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/run-platform-production-preflight.mjs --list
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

再对私有文件运行 Compose `config`，确认容器环境、代理、挂载和健康检查与审批拓扑一致。预检不会部署、迁移或写入生产。

## 变更与回滚

迁移、密钥轮换、配置导入恢复、数据保留清除、SMS 外发、消息 worker 与搜索推广都需要独立变更包：前置备份、影响分析、执行窗口、指标、停止条件、回滚和红脱敏证据。不能将“合同实现”解释为已有生产外部集成证据。

部署拓扑和全量环境变量以仓库权威[部署文档](https://github.com/GnosiST/platform-go/blob/main/docs/platform-deployment.md)为准；运行时故障处理见[运维手册](./operations)。
