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

人员、通知、任务和企业治理通过显式 profile 启用。多数据源、租户切库、读写分离、分片、联邦查询、XA、MQ 和搜索投影按独立合同和认证门逐步加入，默认不启用。

## 注册检查

```bash
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-capability-profiles.mjs
```
