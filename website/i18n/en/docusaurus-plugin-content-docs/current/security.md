---
sidebar_position: 6
title: Security boundaries
---

# Security boundaries

- JWT carries identity; server-side sessions control revocation.
- Casbin backend authorization is authoritative; menus are visibility hints.
- Sensitive fields use manifest-driven encryption, blind indexes, masking and step-up reveal.
- Ordinary clients cannot submit DSNs, physical datasources, schemas, shards or bypass parameters.
- Redis is an optimization, never the source of truth.

Production requires a non-default `PLATFORM_JWT_SECRET`, durable storage, encryption keyrings, Redis, an explicit capability profile and demo auth disabled. Deletion is logical by default; restore and purge are separate audited workflows.
