# Transactional Outbox

`internal/platform/outbox` is an internal foundation package for business-neutral event delivery. It is intentionally not an HTTP endpoint and is not enabled by capability profiles.

## Transaction Boundary

Create a validated `outbox.Envelope` and call `AppendInTransaction` from the same caller-owned GORM transaction as the business mutation. A returned transaction error rolls both writes back. The envelope is version `1.0` and always carries an event ID, event type, occurrence time, producer, trusted tenant code, W3C `traceparent` and JSON payload.

Payloads are bounded and reject common credential, OTP, recipient and contact keys before persistence. Delivery errors are stored only as the stable `outbox delivery failed` summary.

## Dispatch And Recovery

The GORM store leases eligible events, records retry attempts, moves exhausted events to a dead-letter state and creates an immutable audit row for every append, lease, retry, publication, dead-letter and replay action. Replay creates a new delivery record while retaining the original event ID, so consumers can deduplicate it safely.

`integration.RabbitMQ` publishes the serialized envelope as a durable topic-exchange message. It is default-off and has no effect until a composition root constructs it with a protected AMQP(S) URL. The local smoke test is opt-in and must use an ephemeral broker only:

```zsh
PLATFORM_CERTIFICATION_RABBITMQ_URL='redacted local test URL' \
rtk go test ./internal/platform/integration -run '^TestRabbitMQCertificationSmoke$' -count=1
```

The stock API process does not yet start a worker or register a business event producer. Until a composition root supplies both an enabled adapter and an operational worker, normal business transactions remain single-datasource and no external event is emitted.

## Consumer And Saga Boundaries

`ConsumerLedger.Apply` records `consumerID + eventID` in the same GORM transaction as a consumer's local mutation. Completed duplicate deliveries skip the handler. A handler error rolls the receipt back, so the delivery remains retryable. Replayed dead-letter deliveries retain the original event ID and therefore use the same consumer receipt.

`SagaRunner` is a caller-owned compensation boundary for cross-resource workflows. Every registered step supplies its matching compensation. When a later action fails, completed steps compensate in reverse order and durable run/step status records retain only stable failure summaries. The foundation does not register business Saga steps or attempt automatic recovery of process-local callbacks after a process crash; a business composition root owns those operations.
