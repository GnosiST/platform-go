# platform-go

> 面向 Go 服务的业务中立运营平台底座。先把身份、授权、资源合同和运行治理做好，再让业务能力按边界接入。

[![CI](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GnosiST/platform-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-2f6f9f.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg)](go.mod)
[![在线文档](https://img.shields.io/badge/文档-GitHub%20Pages-0b7285.svg)](https://gnosist.github.io/platform-go/)

**中文首选** · [English README](README.en.md) · [在线文档（中文）](https://gnosist.github.io/platform-go/) · [在线文档（English）](https://gnosist.github.io/platform-go/en/)

## 项目定位

`platform-go` 是可复用、可审计、可扩展的 Go 平台基础层。它不绑定具体业务领域，也不把业务菜单、业务数据或业务工作流写进平台核心；业务包通过 capability manifest、公开端口和稳定合同接入。

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

当前治理快照为 **67 total / 58 implemented / 9 controlled unfinished**。`runtime-security-containment` 为 `implemented`；`sensitive-data-protection-runtime` 为 `implemented`；`sensitive-data-historical-migration` 为 `implemented`；`admin-watermark-export-governance` 为 `implemented`；`organization-user-admin-experience` 为 `implemented`。

当前 persistent full-scope unfinished inventory 包括 `multi-datasource-contract-and-runtime`、`tenant-placement-and-request-routing`、`datasource-read-write-routing`、`sharding-and-tenant-migration`、`federated-read-query`、`xa-optional-adapter`、`database-certification-matrix`、`transactional-outbox-and-one-mq-adapter`、`asynchronous-search-projection`、`open-source-portability`、`public-docs-community`、`public-docs-site` 和 `github-release-publication`。v0.1.0 只承诺 one datasource and one native transaction boundary；SQLite is development/test-only by support policy，Oracle and KingbaseES are unsupported。`alibaba/page-agent` is only a default-off optional `public-docs-site` sub-capability。

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
