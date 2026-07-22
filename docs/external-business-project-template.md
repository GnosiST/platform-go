# External Business Project Template

Date: 2026-07-20

## Conclusion

用 `platform-go` 启动真实业务项目时，默认路径是新起业务包、业务服务或业务部署仓库，不是直接改平台核心源码。

业务代码通过 `github.com/GnosiST/platform-go/pkg/platform/capability` 暴露 `capability.Manifest`，并通过 `github.com/GnosiST/platform-go/pkg/platform/app` 挂载声明过的 App handler。菜单、权限、资源、配置入口、App 路由、服务合同、迁移和种子数据都由 manifest 声明后投影到平台合同。`internal/platform/**`、默认 capability、`resources/admin-resources.json` 和默认 profile 继续保持 business-neutral。

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

## 本地可运行教程

这个模板不依赖数据库、密钥或外部服务。示例 migration/seed 是通过公共 lifecycle executor 记录的 no-op 步骤；auth provider 只声明禁用状态和配置 key；service contract 是 contract-only，真实 handlers、persistence 和事件投递由下游业务项目实现。

从仓库根目录运行：

```bash
rtk node scripts/validate-external-capability-example.mjs
rtk node --test scripts/validate-external-capability-example.test.mjs
rtk go -C examples/external-capability test ./...
rtk go -C examples/external-capability run .
```

也可以进入示例目录后运行：

```bash
rtk go test ./...
rtk go run .
```

`examples/external-capability` 是独立嵌套 Go module，所以根目录不要用 `go test ./examples/external-capability/...`。用 `go -C examples/external-capability` 或进入目录运行，才能保持“下游业务仓库独立依赖平台模块”的真实形态。

## Start From Base

1. 在业务仓库 pin 一个明确、已审阅的 `platform-go` Git commit SHA。`v0.1.0` tag 是已发布的不可变版本；保持包版本 `0.1.0` 的后续 `main` 封版提交必须按 SHA 选择，不能移动 tag 或依赖可变分支。
2. 选择平台 foundation 能力列表，例如 `platform-default` 对应的能力集合。
3. 把业务能力 ID 加入下游 desired capability list。
4. 生成目标能力集合的资源、OpenAPI、App route 和审计合同。
5. 打包业务自己的 API/service artifact，并重启。

当前 v1 是 restart-required desired-state 模型。stock `cmd/platform-api` 不能通过配置热加载一个没有编译进进程的外部业务包；真实部署需要业务自己的 composition root 或服务镜像把外部 manifest 编译进去。不要用修改 `internal/platform/**` 的方式把一个产品的业务模型塞进底座。

## Copy Checklist

把模板变成真实业务项目时，最小复制清单是：

| 要替换 | 示例值 | 真实项目要求 |
| --- | --- | --- |
| capability ID | `example-catalog` | 使用业务能力自己的 kebab-case ID，发布后保持稳定 |
| module/package | `github.com/GnosiST/platform-go/examples/external-capability` | 换成下游业务仓库 module 和业务 package |
| manifest function | `CatalogManifest()` | 暴露业务 package 的 manifest entrypoint |
| admin resources | `catalog-items`, `catalog-settings` | 换成业务资源和能力配置资源 |
| permission prefix | `admin:catalog-item`, `admin:catalog-setting` | 按资源单数命名，避免和平台资源冲突 |
| App routes | `/api/app/catalog/items` | 保持 `/api/app/**`，显式声明 auth 和 `app:` 权限 |
| demo data | `catalog-demo-items` | 只保留可安全重放的业务演示数据 |
| config keys | `CATALOG_PARTNER_CLIENT_ID` | 只声明 key 名，不把 secret 写进 manifest |

`catalog` 只是可替换示例域，不是内置平台能力。展会、展商等真实业务对象如果被某个产品需要，也应当作为下游业务 capability 资源声明；只有被证明跨业务复用的能力，才进入新的平台 capability 设计和 profile 审核。

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
- demo data，如果本地验证或演示需要安全、可重放的初始记录；
- 服务合同和事件合同，如果业务服务要被其他服务调用或发布事件。

## Public App Handler Composition

下游 composition root 用标准库 HTTP 挂载 manifest 已声明的路由。`app.NewRouter` 会拒绝未声明、重复或缺失 handler 的注册，并在调用 handler 前执行 manifest 的 session 与 `app:` permission 策略：

```go
router, err := app.NewRouter(manifests, []app.Registration{{
    Method: "GET",
    Path:   "/api/app/catalog/items",
    Handler: func(ctx context.Context, identity app.Identity, writer http.ResponseWriter, request *http.Request) {
        // identity.SubjectID and identity.TenantID are trusted, typed context.
    },
}})
if err != nil {
    return err
}
```

认证中间件必须在验证 bearer token、session 或工作负载凭证后，用 `app.WithIdentity` 写入 `app.Identity`。不要从客户端 body、query 或 header 直接构造身份，也不要把 Gin、GORM、Admin resource store 或平台内部 HTTP handler 暴露给业务 handler。

对应到可执行模板：

| 面向 | 模板位置 | 说明 |
| --- | --- | --- |
| Manifest | `CatalogManifest()` | 注册 `example-catalog`，版本 `0.1.0` |
| Admin resource | `catalogItemResource()` | 业务资源、字段、删除策略、菜单、权限、action、panel、runtime slot |
| Settings resource | `catalogSettingsResource()` | `Parent: "configuration"`，由 `/settings` 动态聚合 |
| App route | `Manifest.App.Routes` | `/api/app/catalog/items`，session auth，`app:catalog-item:read` |
| Auth provider config | `Manifest.AuthProviders` | disabled provider + config key 声明，无本地 secret 依赖 |
| Lifecycle | `Manifest.Migrations` / `Manifest.Seeds` | 通过公共 lifecycle runtime 幂等记录 |
| Demo data | `Manifest.DemoData` | 绑定 `catalog-items` 的演示记录 |
| Service contract | `catalogServiceSurface()` | trusted tenant context、operation、event、reliability 和 runtime boundary |

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
rtk go -C examples/external-capability test ./...
rtk go -C examples/external-capability run .
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
- 输出包含业务 admin resource、settings resource、permission prefix、App route、demo data、migration、seed、service contract hash；
- README 包含无外部配置的本地运行教程；
- `business-project-template.json` 声明 public import、manifest、admin resource、permission prefix、settings/config 入口、demo data、lifecycle、service contract、本地验证命令和禁止修改平台核心源码的边界。

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
