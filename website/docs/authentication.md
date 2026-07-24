---
sidebar_position: 3
title: 认证与授权
---

# 认证与授权

平台将身份、会话、授权和本地凭据认证分开建模。JWT 承载身份，服务端 session 负责 TTL 与撤销，Casbin 在后端执行最终授权；菜单只表示可见性。

## Provider discovery

登录页先读取 `GET /api/auth/providers`，只展示同时满足 `enabled` 和 `configured` 的提供方。不要把用户名密码、手机密码、邮箱密码和短信 OTP 写死成固定菜单，也不要在产品 UI 显示 debug code。

| 路径 | 用途 |
| --- | --- |
| `GET /api/auth/providers` | 发现可用登录提供方 |
| `GET /api/auth/credential-secret-key` | 获取本地凭据的 ECDH 公钥信息 |
| `POST /api/auth/challenges` | 创建 mandatory CAPTCHA/slider challenge |
| `POST /api/auth/challenges/verify` | 仅对 slider 做服务端预验证 |
| `POST /api/auth/sms-otp/start` | 在有效 challenge 后请求发送 OTP |
| `POST /api/auth/login` | 完成 credential/OIDC 等登录 |
| `GET /api/admin/session/current` | 读取当前 Admin session |

## Credential auth v1

`credential-auth` 支持 `username-password`、`phone-password`、`email-password` 与 `phone-sms-otp` provider mode。它使用专用 GORM 存储保存规范化标识 hash、Argon2id verifier、challenge/OTP digest、过期与消费状态；密码、原始 OTP、挑战答案和原始手机号/邮箱不进入 generic `Record.Values`。

浏览器先取得公钥，使用 ECDH P-256 + HKDF-SHA256 派生密钥，并以 AES-256-GCM 提交 `secret.encrypted`。HTTPS 仍是生产基线，但不是该应用层 secret transport 的替代。启用 `RequireEncryptedSecrets` 后，服务端会拒绝明文 secret 字段。

## Challenge 与 SMS OTP

每次 credential password 登录、OTP 发送与 OTP 登录都要求单次 challenge。slider 的正确预验证不会消费 challenge；最终登录才会原子消费它。错误 proof 会累积尝试数，过期、已消费或达到最大尝试数的 transaction 不能重新激活。

SMS 发送归属 `notification` SMS channel。OTP transaction 在发送前持久化，发送失败会禁用 transaction；限流使用 provider/IP 和规范化 phone hash，不记录明文目标。Aliyun/Tencent 的外部投递仍需要受控环境、账号所有权、回滚与红脱敏证据；`mock-local` 仅可用于 development/test，SMTP/WeChat 适配器尚未实现。

## 生产要求

生产启用 credential auth 时，除 `credential-auth` capability 外，必须提供独立 credential repository driver/DSN、32 字节以上 identifier HMAC key 和合法 P-256 secret-transport private key/key ID。手机号 OTP 还需要 `notification`、非 debug SMS provider 和登录模板 ID。

生产必须设置 `PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true`，并使用独立 JWT、数据加密和盲索引 keyring。轮换按“新增 key ID -> 双 key 部署 -> 切换 active ID -> 验证读取与备份 -> 受审阅地退役旧 key”执行，不能原地替换旧 key material。

更完整的参数、session/revocation、step-up、密码变更和回滚规则见仓库权威文档 [`docs/platform-auth.md`](https://github.com/GnosiST/platform-go/blob/main/docs/platform-auth.md)。
