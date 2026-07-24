---
sidebar_position: 5
title: Roadmap and support boundary
---

# Roadmap and support boundary

The current governance snapshot is **67 total / 67 implemented / 0 controlled unfinished**. v0.1 delivers capability manifests, authentication sessions, RBAC, organization and role governance, menu trees, resource schemas, audit, file storage, sensitive-data protection, lifecycle, OpenAPI generation, and the Pages documentation site.

## Current default path

The default release supports one datasource and one native transaction boundary. Identity, sessions, RBAC, resources, audit, and file capabilities form the ordinary business baseline, while production still requires durable stores, Redis, independent keys, and demo-provider disablement.

## Implemented, default-off controlled contracts

Datasource registry, tenant placement, read/write routing, sharding, read-only federation, optional XA, database certification, transactional Outbox/RabbitMQ worker, and asynchronous search projection have implemented controlled contracts. They are not default-on: each needs a downstream composition root, private environment, adapter/supplier evidence, rollback, and promotion approval.

SMTP and WeChat delivery adapters are not implemented. Oracle and KingbaseES are outside the support boundary. The platform also does not support runtime hot installation, remote source pulls, or generic destructive uninstall.

## Evolution principle

Next work improves public service contracts, developer experience, and external-integration promotion evidence rather than bypassing established boundaries. New capabilities declare contracts, tests, and documentation before controlled-environment validation. The [support matrix](./support-matrix) is the complete runtime-state reference.
