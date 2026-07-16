---
sidebar_position: 5
title: APIs and contracts
---

# APIs and contracts

Admin, App and public service contracts are separate. Admin queries use structured JSON conditions, schema allowlists and parameterized predicates.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | Health check |
| GET | `/api/capabilities` | Capability discovery |
| GET | `/api/openapi.json` | OpenAPI contract |
| POST | `/api/admin/resources/:resource/query` | Resource query |
| GET | `/api/admin/resources/:resource/schema` | Resource schema |
| GET | `/api/admin/session/current` | Current Admin session |
| POST | `/api/app/auth/login` | App login |

Errors should include a stable error code, `request_id` and a safe message. Public contracts declare stability, version, idempotency, timeout, retry, rate and deprecation policy.
