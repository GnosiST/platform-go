---
sidebar_position: 4
title: 人机协同开发
---

# 人机协同开发

`platform-go` 面向人类开发者和 AI agent 共同接手。核心规则是：合同先行，边界内高定制，机器门禁兜底。

## 启动业务系统

1. 先读 `AGENTS.md`、能力开发指南和 `examples/external-capability`。
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
| 数据安全 | 租户、组织、区域和敏感字段都由服务端合同与策略控制 |

完整协议见仓库中的 [Human + AI Development Protocol](https://github.com/GnosiST/platform-go/blob/main/docs/platform-human-ai-development-protocol.md)。

## 验证入口

```bash
node scripts/validate-platform-human-ai-development-protocol.mjs
node scripts/validate-external-capability-example.mjs
node scripts/validate-admin-resources.mjs
node scripts/validate-admin-ui-contracts.mjs
node scripts/validate-platform-codegen-source-writing-readiness.mjs
```
