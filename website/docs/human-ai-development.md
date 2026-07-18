---
sidebar_position: 4
title: 人机协同开发
---

# 人机协同开发

`platform-go` 面向人类开发者和 AI agent 共同接手。核心规则是：合同先行，边界内高定制，机器门禁兜底。

## 启动业务系统

1. 先读 `README.md`、`CONTRIBUTING.md`、能力开发指南和 `examples/external-capability`。
2. 业务能力优先放在下游仓库或下游 composition root。
3. 只导入公开合同 `github.com/GnosiST/platform-go/pkg/platform/capability`。
4. 先声明 manifest、资源 schema、菜单、权限、App 路由、服务合同和生命周期。
5. 再接入 storage、HTTP handler、Admin action handler、可选 UI panel 和测试。

## 必守边界

| 方向 | 规则 |
| --- | --- |
| 接口 | Admin 资源、App 路由和服务合同先声明再实现 |
| UI | Admin 使用平台 UI wrapper、Refine provider 和 schema-driven 表单 |
| 视觉 | 通过主题、布局、密度、品牌和注册组件定制，不绕过可访问性和 i18n |
| 代码生成 | 默认只生成合同、预览和 scaffold；运行时代码写入需要人工评审 |
| 能力生命周期 | 安装、禁用、卸载按 operation policy 执行；基础能力不可卸载 |
| 数据安全 | 租户、组织、区域和敏感字段都由服务端合同与策略控制 |

完整协议见仓库中的 [Human + AI Development Protocol](https://github.com/GnosiST/platform-go/blob/main/docs/platform-human-ai-development-protocol.md)。

## 扩展生命周期

当前平台支持启动前组合能力，不支持运行时热插拔。启用能力要先注册 manifest，再通过 profile、`PLATFORM_CAPABILITIES`、`PLATFORM_CAPABILITY_LOCK_FILE` 或下游 composition root 选择；禁用后重新生成合同并重启，已禁用资源不能继续从 Admin/API 暴露。历史数据清理或源码移除不是通用卸载按钮，需要迁移、回滚和负责人证据。

插件管理 v1 的合同是 `resources/platform-plugin-management-v1.json`。它把通过 profile、`PLATFORM_CAPABILITIES`、`PLATFORM_CAPABILITY_LOCK_FILE` 或下游 composition root 声明期望能力后“启用/禁用手动重启生效”固定为 restart-required desired-state model，并明确 v1 不集成 WebSocket、不从远端仓库拉插件、不支持运行时热安装/热卸载或破坏性卸载。新业务项目优先放在下游仓库或 composition root，平台只提供通用能力边界。

## 验证入口

```bash
rtk node scripts/validate-platform-human-ai-development-protocol.mjs
rtk node scripts/validate-platform-plugin-management-v1.mjs
rtk node scripts/validate-platform-capability-operation-policy.mjs
rtk node scripts/validate-external-capability-example.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
```
