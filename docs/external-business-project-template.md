# External Business Project Template

Date: 2026-07-19

## Conclusion

用 `platform-go` 启动真实业务项目时，默认路径是新起业务包、业务服务或业务部署仓库，不是直接改平台核心源码。

业务代码通过 `github.com/GnosiST/platform-go/pkg/platform/capability` 暴露 `capability.Manifest`。菜单、权限、资源、配置入口、App 路由、服务合同、迁移和种子数据都由 manifest 声明后投影到平台合同。`internal/platform/**`、默认 capability、`resources/admin-resources.json` 和默认 profile 继续保持 business-neutral。

## Repository Shape

推荐下游项目结构：

```text
my-business/
  go.mod
  cmd/business-api/
  internal/catalog/
    manifest.go
    handlers.go
    repository.go
  contracts/
  deploy/
```

`examples/external-capability` 是本仓库维护的可执行模板切片。它不是默认平台能力，也不会被 `cmd/platform-api` 自动导入。

## Start From Base

1. 在业务仓库 pin 一个明确的 `platform-go` 版本。
2. 选择平台 foundation 能力列表，例如 `platform-default` 对应的能力集合。
3. 把业务能力 ID 加入下游 desired capability list。
4. 生成目标能力集合的资源、OpenAPI、App route 和审计合同。
5. 打包业务自己的 API/service artifact，并重启。

当前 v1 是 restart-required desired-state 模型。stock `cmd/platform-api` 不能通过配置热加载一个没有编译进进程的外部业务包；真实部署需要业务自己的 composition root 或服务镜像把外部 manifest 编译进去。不要用修改 `internal/platform/**` 的方式把一个产品的业务模型塞进底座。

## Capability Manifest

业务包只依赖公共合同：

```go
package catalog

import "github.com/GnosiST/platform-go/pkg/platform/capability"

func Manifest() capability.Manifest {
    return capability.Manifest{
        ID:      "example-catalog",
        Name:    "Example Catalog",
        Version: "0.1.0",
        Admin: capability.AdminSurface{
            Resources: []capability.AdminResource{
                catalogItemsResource(),
                catalogSettingsResource(),
            },
        },
        App: capability.AppSurface{
            Routes: []capability.AppRoute{{
                Method:      "GET",
                Path:        "/api/app/catalog/items",
                Auth:        capability.AppRouteAuthSession,
                Permission:  "app:catalog-item:read",
                Description: capability.Text("读取目录项列表。", "Read catalog item list."),
            }},
        },
    }
}
```

Manifest 至少要声明：

- capability ID 和 semver 版本；
- `Admin.Resources`，包括资源 key、标题、说明、`PermissionPrefix`、字段、菜单、删除策略；
- App 路由和 `app:` 权限；
- 生命周期迁移和 seed；
- 认证提供方及 `ConfigKeys`，如业务侧登录或外部集成需要；
- 服务合同和事件合同，如果业务服务要被其他服务调用或发布事件。

## Admin Resource, Menu And Permission

业务资源放在能力 manifest 内：

```go
capability.AdminResource{
    Resource:         "catalog-items",
    Title:            capability.Text("目录项", "Catalog Items"),
    Description:      capability.Text("业务目录项。", "Business catalog items."),
    PermissionPrefix: "admin:catalog-item",
    Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
    Menu: capability.AdminMenu{
        Route: "/catalog-items",
        Group: "business",
        Icon:  "appstore",
        Order: 100,
        Cache: true,
    },
}
```

平台会由 `PermissionPrefix` 生成标准读写权限，并把菜单和资源 schema 投影到 Admin 合同。后端权限检查仍是权威，前端隐藏菜单只是体验层。

## Settings Entry

系统级业务配置也声明成普通 Admin Resource，不改 shell：

```go
capability.AdminResource{
    Resource:         "catalog-settings",
    Title:            capability.Text("目录配置", "Catalog Settings"),
    Description:      capability.Text("目录能力配置入口。", "Catalog capability settings."),
    PermissionPrefix: "admin:catalog-setting",
    Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionDisabled, PolicyVersion: 1},
    Menu: capability.AdminMenu{
        Route:  "/catalog-settings",
        Parent: "configuration",
        Group:  "foundation",
        Icon:   "settings",
        Order:  320,
        Cache:  true,
    },
}
```

`Menu.Parent == "configuration"` 代表它是能力配置资源。`/settings` 通过 `GET /api/capabilities` 的 `configResources` 聚合这些入口，不需要硬编码业务页面。

## Register Manifest

注册发生在业务项目自己的 composition root：

```go
businessManifests := []capability.Manifest{
    catalog.Manifest(),
}
```

composition root 必须在解析 enabled capabilities 前把这些 manifest 传给平台启动流程，然后选择包含业务 ID 的目标能力集合。业务 handlers、stores、自定义 Admin renderer 和状态机也在这个业务项目里注册。平台核心包不能 import 业务包。

如果某段逻辑已经被证明是跨业务复用的基础能力，才单独提出平台能力设计，把公共合同、默认/可选 profile、迁移、验证和文档一起补齐；不能因为一个业务项目需要就把它加入默认底座。

## Validate

从本仓库根目录验证模板：

```bash
rtk node scripts/validate-external-capability-example.mjs
rtk node --test scripts/validate-external-capability-example.test.mjs
```

示例包内也可以直接跑：

```bash
rtk go test ./...
rtk go run .
```

validator 会检查：

- Go 代码导入公共 `pkg/platform/capability`；
- Go 代码不导入 `github.com/GnosiST/platform-go/internal/platform/**`；
- manifest 可通过公共 registry 解析；
- 输出包含业务 admin resource、settings resource、permission prefix、App route、service contract hash；
- `business-project-template.json` 声明 public import、manifest、admin resource、permission prefix、settings/config 入口和禁止修改平台核心源码的边界。

真实业务项目上线前还要按目标能力集合重跑 admin resource、App route、OpenAPI、profile、operation policy 和生产环境 validator。

## Package And Deploy

打包物应该是业务项目自己的 API/service artifact：

- 平台模块版本固定；
- 业务 manifest 和 handlers 已编译进业务 artifact；
- desired capability list 同时包含平台能力和业务能力；
- 生成合同来自同一套 manifest；
- 部署后手动重启 API 进程；
- 验证 `/api/capabilities`、Admin resource contract、App route contract 和 OpenAPI 中出现业务资源，禁用后消失。

不要把业务菜单写进 `resources/admin-resources.json`，不要把业务 manifest 加进平台默认能力，不要让业务包 import `internal/platform/**`。

## Upgrade Strategy

平台升级和业务升级分开：

- 平台升级：更新 `platform-go` 版本，重跑合同和生产 validator，修复公共合同变更后再重建业务 artifact。
- 业务升级：递增业务 capability semver，保持已发布的 resource key、route、permission prefix 稳定；数据结构变化走幂等 migration。
- 禁用或回滚：从 desired capability list 移除业务 ID，重新生成合同并重启；业务持久化数据由业务仓库的迁移/归档计划处理，平台 v1 不做 destructive uninstall 或数据 purge。
