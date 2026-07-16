# platform-go

> 面向 Go 服务的业务中立运营平台底座。先把身份、授权、资源合同和运行治理做好，再让业务能力按边界接入。

[![CI](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-2f6f9f.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg)](go.mod)
[![在线文档](https://img.shields.io/badge/文档-GitHub%20Pages-0b7285.svg)](https://gnosist.github.io/platform-go/)

[简体中文](README.md) · [English](README.en.md) · [文档站 / Documentation](https://gnosist.github.io/platform-go/)

## 项目定位

`platform-go` 是可复用、可审计、可扩展的 Go 平台基础层。它不绑定具体业务领域，也不把业务菜单、业务数据或业务工作流写进平台核心；业务包通过 capability manifest、公开端口和稳定合同接入。

`platform-go` is not a business migration target or an application package. 平台保持业务中立，不交付具体业务资源、路由、存储、状态机、菜单或工作流。

适合需要以下基础能力的团队：

- 用 Gin + GORM 构建长期运行的 Go 服务；
- 统一处理认证、会话、租户、组织、RBAC、菜单和审计；
- 用资源 schema、OpenAPI 和代码生成约束前后端协作；
- 在生产环境中保留迁移、缓存、文件存储和发布治理边界。

## 核心能力

| 领域 | 默认提供 |
| --- | --- |
| 平台合同 | capability manifest、资源 schema、路由与权限声明、版本化生成物 |
| 身份与授权 | JWT 会话、服务端会话撤销、Casbin RBAC、租户与组织范围、菜单可见性 |
| 管理资源 | Refine + React + Ant Design 管理壳、通用列表/表单、审计与生命周期 |
| 工程治理 | OpenAPI、App 路由合同、Go/TypeScript codegen 预览、验证器与发布证据 |
| 运行基础 | GORM 持久化、Redis 缓存端口、文件存储、品牌配置和生产预检 |

可选能力以 profile 方式启用，默认不改变基础平台：人员、通知、任务和企业治理等扩展保持可拆卸。多数据源、读写路由、分片、联邦查询、XA、MQ 和搜索投影属于后续扩展，不会伪装成当前默认能力。

<details>
<summary>当前实现状态与支持边界（维护者参考）</summary>

当前治理快照为 **67 total / 57 implemented / 10 controlled unfinished**。`runtime-security-containment` 为 `implemented`；`sensitive-data-protection-runtime` 为 `implemented`；`sensitive-data-historical-migration` 为 `implemented`；`admin-watermark-export-governance` 为 `implemented`；`organization-user-admin-experience` 为 `implemented`。

当前 persistent full-scope unfinished inventory 包括 9 个 `deferred` 节点：`multi-datasource-contract-and-runtime`、`tenant-placement-and-request-routing`、`datasource-read-write-routing`、`sharding-and-tenant-migration`、`federated-read-query`、`xa-optional-adapter`、`database-certification-matrix`、`transactional-outbox-and-one-mq-adapter` 和 `asynchronous-search-projection`，以及唯一的 v0.1.0 release blocker：`github-release-publication`（`pending`）。`open-source-portability`、`public-docs-community` 和 `public-docs-site` 均为 `implemented`。目标版本保持 v0.1.0，当前尚未正式发布；其支持边界为 one datasource and one native transaction boundary。SQLite is development/test-only by support policy，Oracle and KingbaseES are unsupported。`alibaba/page-agent` is only a default-off optional `public-docs-site` sub-capability。

策略评审接口：

```text
POST /api/admin/policy-reviews/:id/request
POST /api/admin/policy-reviews/:id/reject
POST /api/admin/policy-reviews/:id/approve
GET /api/admin/policy-reviews/export
```

</details>

## 技术栈

- **后端**：Go、Gin、GORM、Casbin、JWT
- **管理端**：React、Refine、Ant Design、TypeScript
- **合同与文档**：OpenAPI、Docusaurus、GitHub Pages
- **可选运行依赖**：Redis；数据库和外部适配器按支持矩阵接入

## 快速开始

环境要求：Go、Node.js、npm。生产环境另需持久化数据库和 Redis。

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
go run ./cmd/platform-api
```

API 默认地址为 `http://127.0.0.1:9200`，管理端开发服务器为 `http://127.0.0.1:9202`。

<details>
<summary>生产基线与非变更式预检</summary>

Production baseline: set `PLATFORM_RUNTIME_ENV=production`，并显式配置持久化存储、Redis、可信 HTTPS 边缘、独立密钥和关闭 demo 认证。完整取值、约束和轮换说明以 [部署与生产基线](docs/platform-deployment.md) 为准；README 只保留机器合同要求的初始化清单：

```text
PLATFORM_RUNTIME_ENV
PLATFORM_PUBLIC_BASE_URL
PLATFORM_TRUSTED_PROXIES
PLATFORM_EDGE_TRUSTED_PROXY
PLATFORM_HTTP_MAX_BODY_BYTES
PLATFORM_JWT_SECRET
PLATFORM_DATA_KEY_PROVIDER
PLATFORM_ADMIN_RESOURCE_DRIVER
PLATFORM_ADMIN_RESOURCE_DSN
PLATFORM_SESSION_DRIVER
PLATFORM_SESSION_DSN
PLATFORM_LIFECYCLE_HISTORY_DRIVER
PLATFORM_LIFECYCLE_HISTORY_DSN
PLATFORM_CACHE_DRIVER
PLATFORM_REDIS_ADDR
PLATFORM_RATE_LIMIT_HMAC_KEY
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER
```

保留策略执行器默认关闭；启用前必须一起评审 `PLATFORM_RETENTION_RUNNER_ENABLED`、`PLATFORM_RETENTION_RUNNER_INTERVAL`、`PLATFORM_RETENTION_RUNNER_BATCH_SIZE` 和 `PLATFORM_RETENTION_RUNNER_MAX_RETRIES`。

先列出非变更式生产预检，再针对私有环境执行严格配置审计。标准模板可直接运行 `node scripts/validate-platform-production-env.mjs` 检查：

```bash
node scripts/validate-platform-foundation-alignment.mjs
node scripts/run-platform-production-preflight.mjs --list
node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>
```

`config-backup-export`、`config-import-restore`、`database-migration` 和 `token-rotation` 是需要人工评审、回滚证据与审计记录的生产操作策略；预检本身不会部署、迁移或写入生产状态。

Deployment scheme A is selected as the default：以长生命周期 Gin 服务承载 API，并优先同源提供 Admin 静态资源；边界详见 [部署文档](docs/platform-deployment.md)，变更拓扑前运行 `node scripts/validate-platform-deployment-topology.mjs`。

</details>

## 能力边界

默认发行版以单数据源、单事务边界为支持基线。生产环境不会启用 demo 数据或 demo 登录；JWT 密钥、加密密钥环、持久化存储、Redis 和能力 profile 都必须显式配置。

平台只负责共享机制。业务代码不应直接依赖平台具体存储、HTTP handler 或 Admin 壳层内部实现；应通过公开 capability、service、query/command 和 storage port 接入。

## 文档导航

- [快速开始与架构介绍](docs/platform-capability-development.md)
- [认证、会话与 RBAC](docs/platform-auth.md)
- [Admin 资源与菜单](docs/admin-resource-schema.md)
- [数据生命周期与保留策略](docs/platform-data-lifecycle-retention.md)
- [敏感数据保护与迁移](docs/platform-sensitive-data-migration.md)
- [部署与生产基线](docs/platform-deployment.md)
- [在线文档站](https://gnosist.github.io/platform-go/)

## 参与贡献

请先阅读 [贡献指南](CONTRIBUTING.md)，再提交 issue 或 pull request。新能力应提供 manifest、合同、测试、文档和必要的迁移/回滚证据；内部计划和 AI 工作过程不属于公开仓库内容。

## License

Apache License 2.0，详见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。

---

English version: [README.en.md](README.en.md)
