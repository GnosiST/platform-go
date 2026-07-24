---
sidebar_position: 3
title: 接入第一个业务能力
---

# 接入第一个业务能力

业务能力属于下游仓库或下游 composition root。平台只提供稳定的公开合同和通用机制，不能从业务包反向导入或修改 `internal/platform/**`。

## 1. 从外部示例开始

[`examples/external-capability`](https://github.com/GnosiST/platform-go/tree/main/examples/external-capability) 是支持的最小边界。先复制其模块结构，再替换 manifest 中的能力 ID 和资源声明。

```go
import "github.com/GnosiST/platform-go/pkg/platform/capability"

func Manifest() capability.Manifest {
    return capability.Manifest{
        ID: "inventory",
        // 资源 schema、权限、菜单、路由与生命周期在这里声明。
    }
}
```

外部 manifest 只从 `github.com/GnosiST/platform-go/pkg/platform/capability` 获得 capability 合同。App 路由使用公开的 `pkg/platform/app` 表面；不得借用内部 handler、存储或 Admin 壳层实现。

## 2. 先声明，再实现

按以下顺序工作，保证合同先于 UI 和存储细节：

1. 声明资源 schema、权限前缀、菜单可见性和生命周期。
2. 声明 Admin/App 路由与服务合同。
3. 在下游组合根注册 storage、handler、Admin action 和可选 UI。
4. 将领域表、迁移、状态机和测试保留在业务模块。
5. 生成合同，让 OpenAPI、资源合同和代码生成预览反映实际组合。

菜单不是授权。所有写操作和受保护读取都必须在后端经 Casbin/会话范围校验；不能用隐藏菜单代替权限控制。

## 3. 验证公开边界

```bash
node scripts/validate-external-capability-example.mjs
node scripts/validate-platform-capability-contracts.mjs
node scripts/validate-platform-capability-profiles.mjs
node scripts/validate-platform-admin-api-boundary.mjs
```

外部示例门禁会验证外部模块没有导入 `internal/platform/**`，并运行其 Go 测试与 manifest 预览。若你的能力增加 Admin 资源，再运行资源合同、OpenAPI、代码生成和 Admin build 门禁。

## 4. 组合、启用与禁用

能力在启动前由 profile、`PLATFORM_CAPABILITIES`、`PLATFORM_CAPABILITY_LOCK_FILE` 或下游 composition root 声明期望集合。变更后必须重新生成合同并手动重启服务。

运行时不支持热安装、远程拉取源码或通用破坏性卸载。禁用一个可选能力后，它的 Admin resource、App route、auth provider 和 demo surface 都必须从公开合同消失；删除历史数据需要独立的迁移、保留和回滚计划。

## 接入完成的定义

- 业务模块只依赖公开平台包；
- manifest、路由、资源、权限和生命周期可由合同生成物观察到；
- 业务数据与迁移在下游所有权范围内；
- 测试覆盖授权、租户范围、失败与回滚路径；
- 生产启用前已有配置、审阅和回滚证据。

需要查看默认与受控状态的区别时，阅读[支持矩阵](./support-matrix)。
