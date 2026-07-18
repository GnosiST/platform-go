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

平台不是运行时热插拔市场。安装能力等于在启动前通过 profile、`PLATFORM_CAPABILITIES` 或下游 composition root 启用已注册 manifest；禁用能力等于移出启用集合、重新生成合同并重启，资源、菜单、provider、路由和演示数据不能继续暴露。`dictionary`、`tenant`、`identity`、`session`、`rbac`、`menu` 和 `admin-shell` 属于不可卸载基础能力。破坏性卸载或历史数据清理需要单独评审、迁移和回滚证据。

## 注册检查

```bash
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-capability-profiles.mjs
node scripts/validate-platform-capability-operation-policy.mjs
```
