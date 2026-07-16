---
sidebar_position: 2
title: 架构总览
---

# 架构总览

`platform-go` 将平台能力拆成声明、运行时和适配器三层，业务包只依赖公开合同，不直接依赖 Admin 壳或具体数据库。

## 请求路径

```text
Identity + TenantContext
  -> Capability Manifest
  -> Casbin / data scope
  -> Query / Command contract
  -> GORM repository
  -> durable storage
```

身份来自可信 JWT 会话或服务间身份。租户和数据范围由服务端上下文解析，不能由普通客户端提交物理数据库、DSN 或任意权限码。

## 五个平面

| 平面 | 责任 |
| --- | --- |
| Admin Plane | 管理资源、菜单、表单和审计入口 |
| Service/Data Plane | 业务服务、查询、命令与数据范围 |
| Control Plane | 能力、配置、迁移和发布策略 |
| External/Partner Plane | 外部调用方的稳定服务合同 |
| Event Plane | 版本化事件和未来的 outbox/消息适配 |

## 扩展原则

能力以 `capability.Manifest` 注册。每个能力拥有自己的资源 schema、路由、权限、生命周期和可选 UI 注册；平台核心不反向导入业务应用。
