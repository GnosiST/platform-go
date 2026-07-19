---
sidebar_position: 2
title: 能力与扩展
---

能力通过 manifest 注册，公共平台包不直接依赖业务应用。外部业务能力应使用公开端口、资源 schema、路由和权限合同接入，并保持租户范围与审计边界。

> English: Capabilities are declared through versioned manifests and attach through public ports and contracts. Use the language switcher for the complete English guide.

参考仓库中的 [能力开发指南](https://github.com/GnosiST/platform-go/blob/main/docs/platform-capability-development.md)。

## 默认能力

身份、租户、组织、角色、角色组、菜单、资源、审计、会话、文件存储和品牌配置组成默认底座。角色组只负责角色分类和治理边界，不直接授予权限。

## 可选 profile

人员、通知、任务、企业治理和生产后台 OIDC 通过显式 profile 启用。多数据源、租户切库、读写分离、分片、联邦查询、XA、MQ 和搜索投影按独立合同和认证门逐步加入，默认不启用。

## 装停卸边界

平台不是运行时热插拔市场。安装能力等于在启动前通过 profile、`PLATFORM_CAPABILITIES`、`PLATFORM_CAPABILITY_LOCK_FILE` 或下游 composition root 声明 desired state；禁用能力等于移出期望集合、重新生成合同并手动重启。运行进程与期望集合不一致时只能标记 `pendingRestart`，不能热应用。重启后 Admin resource、App route、auth provider 和 demo data set 不能继续暴露。`dictionary`、`tenant`、`identity`、`session`、`rbac`、`menu` 和 `admin-shell` 属于不可卸载基础能力。破坏性卸载或历史数据清理需要单独评审、迁移和回滚证据。

## 插件管理 v1

插件管理 v1 采用 restart-required desired-state model。先通过 profile、`PLATFORM_CAPABILITIES`、`PLATFORM_CAPABILITY_LOCK_FILE` 或下游 composition root 声明期望能力集合，再跑合同校验、重新生成合同并手动重启；前台只通过 HTTP polling、`version.json` 或 API version check 提示新版本，不通过 WebSocket 做热更新。v1 明确不支持 runtime hot install/uninstall、远端仓库拉取、源码删除、数据清理或一键破坏性卸载。

## Credential Auth v1

`credential-auth` 是本地凭据认证能力，用于用户名/密码、手机号/密码、邮箱/密码和手机号/短信验证码登录。当前完成了合同、文档、验证门禁、provider 声明和第一批开发态 HTTP/UI 切片：`GET /api/auth/providers` 可发现 credential provider，`POST /api/auth/sms-otp/start` 可启动短信验证码事务，`POST /api/auth/login` 可处理结构化 credential-password / credential-sms-otp 登录并继续兼容 demo/OIDC。它仍不启用旧式 `password` provider kind，也不是生产完整能力。密码、OTP、验证码答案和挑战证明不能存入 generic `Record.Values`；已落地的内部包覆盖 identifier hash、Argon2id PHC 校验、memory repository、开发态 bootstrap Admin 凭据和 SMS OTP 一次性消费语义。持久化仓储、CAPTCHA/滑块 challenge、真实短信供应商、OpenAPI/审计/限流治理和生产启用门禁仍待后续补齐。短信发送作为 `notification` 的 SMS channel 扩展，当前已有 SMS sender port、`mock-local` dev/test sender 和生产禁用 mock 的配置门禁。

新业务项目应把具体业务能力放在下游仓库或下游 composition root，只把跨业务复用能力沉淀为平台 profile。

## 注册检查

```bash
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-credential-auth-v1.mjs
rtk node scripts/validate-platform-capability-contracts.mjs
rtk node scripts/validate-platform-capability-profiles.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
```
