---
sidebar_position: 4
title: 架构总览
---

# 架构总览

`platform-go` 将平台能力拆成声明、运行时和适配器三层。业务包依赖公开合同，不依赖 Admin 壳、具体数据库或 HTTP handler 内部实现。

## 请求与合同路径

```text
Trusted identity + TenantContext
  -> capability manifest / route declaration
  -> session + Casbin + data scope
  -> Query / Command contract
  -> service and storage port
  -> GORM, cache, file or external adapter
```

身份来自可信 JWT session、OIDC 或声明的 credential provider。租户和数据范围由服务端上下文解析；普通客户端不能提交 DSN、物理数据库、schema、shard 或任意权限码。

## 五个平面

| 平面 | 责任 | 关键边界 |
| --- | --- | --- |
| Admin Plane | 管理资源、菜单、表单、审计入口 | 菜单仅可见性，后端授权独立执行 |
| Service/Data Plane | 业务服务、查询、命令与数据范围 | 默认一个数据源与一个原生事务 |
| Control Plane | capability、配置、迁移、发布策略 | desired state 需要合同重生成和重启 |
| External/Partner Plane | 面向外部调用方的稳定服务合同 | OpenAPI、版本、错误与幂等语义可审阅 |
| Event Plane | 版本化事件与受控异步边界 | Outbox/MQ/search 为 default-off，不默认启用 |

## 扩展方式

每个 `capability.Manifest` 拥有自己的资源 schema、路由、权限、生命周期和可选 UI 注册。外部业务 package 只能使用 `pkg/platform/capability` 等公开表面；如果合同不足，应先扩展公开合同，再触及内部平台实现。

平台核心和可选平台 capability 不反向导入业务应用。具体业务的表、迁移、状态机、handler 与 UI 保持在下游 composition root，这保证底座可复用且业务可以独立演进。

查看[业务能力接入](./business-integration)获得最小实践；查看[支持矩阵](./support-matrix)区分默认路径与受控集成。
