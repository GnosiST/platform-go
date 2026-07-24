---
sidebar_position: 5
title: 路线图与支持边界
---

# 路线图与支持边界

当前治理快照为 **67 total / 67 implemented / 0 controlled unfinished**。v0.1 已交付 capability manifest、认证会话、RBAC、组织与角色治理、菜单树、资源 schema、审计、文件存储、敏感数据保护、生命周期、OpenAPI 生成和 Pages 文档站。

## 当前默认路径

默认发行版支持单数据源和单原生事务边界。身份、session、RBAC、资源、审计和文件能力可作为常规业务底座使用，但生产仍需要持久化存储、Redis、独立密钥和关闭 demo provider。

## 已实现但默认关闭的受控合同

多数据源注册、租户 placement、读写路由、分片、联邦只读、可选 XA、数据库认证、事务 Outbox/RabbitMQ worker 与异步搜索投影均已实现受控合同。它们不是默认启用能力：每一项都需要下游 composition root、私有环境、适配器/供应商证据、回滚与推广批准。

SMTP 与 WeChat 外发适配器尚未实现；Oracle 和 KingbaseES 不在支持边界内。平台也不支持运行时热安装、远程拉取源码或通用破坏性卸载。

## 后续演进原则

后续优先改进公开服务合同、开发者体验和外部集成推广证据，而不是绕开既有边界。任何新能力先声明合同、测试和文档，再进入受控环境验证。完整运行状态以[支持矩阵](./support-matrix)为准。
