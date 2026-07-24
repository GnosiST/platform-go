---
sidebar_position: 2
title: Support matrix
---

# Support matrix

States describe runtime defaults and promotion requirements. They prevent an implemented controlled contract from being misread as default certified production support.

| Scope | State | Runtime and promotion requirement |
| --- | --- | --- |
| Identity, sessions, RBAC, organization, menus, resources, audit, files | Default | Use public contracts; production still needs durable stores, Redis, keys, and demo disabled |
| Personnel, notification, jobs, enterprise governance, Admin OIDC, credential auth | Explicit profile | Compose through manifest/profile; regenerate contracts and restart |
| Datasource registry, tenant placement, read/write routing, sharding, read-only federation, XA | Default-off controlled contract | Needs a concrete downstream composition, database certification, controlled environment, rollback, and promotion evidence |
| Transactional Outbox/RabbitMQ worker, search projection | Default-off controlled contract | Stock API stays disabled; needs named adapters, delivery/replay/recovery evidence, and operations approval |
| Aliyun/Tencent SMS | Controlled external adapter | Configuration/rehearsal is possible; real production sends need separate account, template, compliance, and incident evidence |
| SMTP, WeChat delivery | Not implemented | Must not be described as available suppliers |
| Oracle, KingbaseES | Unsupported | Outside the certified database boundary |
| Runtime hot plugins, remote source pull, generic destructive uninstall | Unsupported | Only restart-required desired state is supported; no source or data destruction action runs |

## How state changes

Capabilities are first declared in manifests and contracts, then elevated through tests, configuration validation, and deployment evidence. External services additionally need controlled-environment account ownership, replay/retry, observability, rollback, and release approval. A local mock or structural contract does not establish supplier production availability.

The current governance snapshot is **67 total / 67 implemented / 0 controlled unfinished**. `open-source-portability`, `public-docs-community`, `public-docs-site`, and `github-release-publication` are `implemented`. That snapshot closes the task graph; it does not change this page's default-enable and external-evidence boundaries.

## Selection principle

Ordinary business services should begin with one datasource and one native transaction. When data or asynchronous boundaries grow, read the relevant contract, rehearse in isolation, then promote through a separate change packet. Changing a profile from desired state to effective runtime state requires contract regeneration and a manual restart; the running process can only report `pendingRestart`.
