---
sidebar_position: 2
title: 支持矩阵
---

# 支持矩阵

状态描述运行时默认值与推广要求，避免将“已有受控合同”误读为“默认生产已认证”。

| 范围 | 状态 | 运行和推广要求 |
| --- | --- | --- |
| 身份、session、RBAC、组织、菜单、资源、审计、文件 | 默认支持 | 使用公开合同；生产仍需持久化存储、Redis、密钥和 demo 禁用 |
| 人员、通知、任务、企业治理、Admin OIDC、credential-auth | 显式 profile | 由 manifest/profile 组合，合同重生成并重启后生效 |
| datasource registry、租户 placement、读写路由、分片、联邦只读、XA | default-off 受控合同 | 需要具体下游组合、数据库认证、受控环境、回滚和推广证据 |
| transactional Outbox/RabbitMQ worker、搜索投影 | default-off 受控合同 | stock API 保持禁用；需要命名适配器、投递/重放/恢复证据和运营批准 |
| Aliyun/Tencent SMS | 受控外部适配器 | 可进行配置/演练；真实生产投递需独立账号、模板、合规和事故证据 |
| SMTP、WeChat 外发 | 尚未实现 | 不应声明为已可用供应商 |
| Oracle、KingbaseES | 不支持 | 不在认证数据库边界内 |
| 运行时热插件、远程拉取源码、通用破坏性卸载 | 不支持 | 仅支持 restart-required desired state，不执行源码或数据破坏动作 |

## 状态如何变化

能力先在 manifest 与合同中声明，再经过测试、配置验证和部署证据提升。对于外部服务，另需受控环境的账号归属、重试/回放、观测、回滚和发布批准。平台不因为本地 mock 或结构化合同通过就声称供应商生产可用。

当前治理快照为 **67 total / 67 implemented / 0 controlled unfinished**。其中 `open-source-portability`、`public-docs-community`、`public-docs-site` 和 `github-release-publication` 是 `implemented`。该快照表示任务图闭合，不改变本页的默认启用和外部证据边界。

## 选择原则

常规业务优先使用单数据源与一个原生事务。需要扩大数据或异步边界时，先阅读对应合同、在隔离环境演练，再以单独变更包推广。将 profile 从 desired state 变为有效运行状态需要重新生成合同和手动重启，运行时只会报告 `pendingRestart`。
