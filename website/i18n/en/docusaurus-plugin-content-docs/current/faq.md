---
sidebar_position: 10
title: FAQ
---

# FAQ

## Is this a business application?

No. It is a business-neutral foundation. Domain resources and workflows attach through capability contracts.

## Why is SQL not exposed to clients?

High-risk queries use server-owned persisted Query Objects. Clients submit a query ID, version and typed arguments instead of physical schema, joins or operators.

## Can I connect MQ or Elasticsearch?

The platform provides disabled-by-default ports and contracts. An adapter needs health, retry, rate, audit and recovery evidence before enablement.
