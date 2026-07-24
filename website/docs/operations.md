---
sidebar_position: 4
title: 运维与故障排查
---

# 运维与故障排查

先使用生成物和只读预检定位问题，再进入受审阅的变更流程。生产操作不能通过临时修改配置、删除数据或跳过验证来“恢复”。

## 日常检查

```bash
node scripts/validate-platform-foundation-alignment.mjs
node scripts/validate-platform-production-readiness.mjs
node scripts/validate-platform-deployment-topology.mjs
node scripts/run-platform-production-preflight.mjs --list
```

服务的 `request_id` 用于关联 API、应用日志和审计事件。状态输出只能显示可安全公开的 `disabled`、`ready` 或 `unavailable` 等汇总，不返回适配器端点、凭据或上游错误详情。

## 诊断路径

| 症状 | 先检查 | 安全恢复路径 |
| --- | --- | --- |
| API 不健康 | 进程、监听地址、`/api/health` | 恢复配置或进程；不要暴露内部 readiness 明细 |
| API 未就绪 | `GET /api/ready`、存储/Redis 配置、启动日志 | 修复被验证的依赖配置，再重启并复测 |
| Admin 无数据 | API proxy、session、`request_id` | 先确认认证与授权，不用前端隐藏绕过权限 |
| 登录失败或 OTP 未送达 | provider discovery、challenge、通知配置、红脱敏审计 | 重新创建 challenge/OTP；不要展示或回填 debug code |
| 消息/搜索不可用 | capability/profile、命名适配器、健康状态 | 保持 default-off，按独立变更包恢复或回滚 |
| 迁移/保留任务异常 | 变更包、备份、checkpoint、审计记录 | 停止后按已批准 rollback/restore 演练执行 |

## 变更纪律

数据库迁移、密钥轮换、配置导入恢复、数据保留清除、消息重放和外部短信投递都需要变更负责人、影响分析、停止条件、回滚、观测和红脱敏证据。预检不写入生产，不能替代这些操作证据。

生产配置请从[生产检查清单](./production-checklist)开始；能力是否默认可用见[支持矩阵](./support-matrix)。
