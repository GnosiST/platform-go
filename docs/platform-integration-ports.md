# Optional Integration Ports

`platform-go` exposes business-neutral ports for a message bus, search indexing and search reads. The stock process keeps every port disabled and does not bundle a broker, Elasticsearch, OpenSearch, Outbox, DLQ, replay worker or search projection.

## Runtime Contract

The ports live in `internal/platform/integration`:

- `MessageBus` publishes a platform message.
- `SearchIndexer` indexes or deletes a document.
- `SearchReader` executes a server-selected query ID with typed arguments.

Disabled implementations return `ErrMessageBusDisabled` or `ErrSearchDisabled`; they never report success. Unified status is one of `disabled`, `ready` or `unavailable`. Adapter health errors are not copied into status output, so endpoints and credentials cannot leak through this contract.

## Configuration

Production must explicitly declare both switches. The checked-in deployment template keeps them false:

```text
PLATFORM_MESSAGE_BUS_ENABLED=false
PLATFORM_MESSAGE_BUS_ADAPTER=
PLATFORM_SEARCH_ENABLED=false
PLATFORM_SEARCH_ADAPTER=
```

Enabling a switch requires the process composition root to register an adapter whose canonical kind exactly matches the configured value. Search requires both an indexer and reader of that kind. The stock `cmd/platform-api` registers no external adapters, so it fails closed if either integration is enabled.

At startup the stock process logs each port's `enabled`, `state` and canonical adapter kind. This provides an operator-visible status surface without exposing adapter health errors or claiming an HTTP consumer that does not yet exist.

Capability profiles such as `notification` and `job` do not implicitly enable an external integration. A downstream product must opt in through configuration and composition together.

## Deferred Work

This node intentionally does not implement a vendor adapter, transactional Outbox, delivery retry, DLQ, replay, search projection or persisted search engine query. Those remain separate completion nodes so enabling a port cannot be confused with delivering a production integration.

Focused verification:

```bash
rtk go test ./internal/platform/integration ./internal/platform/bootstrap ./internal/platform/config ./cmd/platform-api
rtk node --test scripts/platform-integration-ports.test.mjs
rtk node scripts/validate-platform-integration-ports.mjs
```
