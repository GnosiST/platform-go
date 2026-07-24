# Asynchronous Search Projection

`internal/platform/searchprojection` consumes a validated, versioned Outbox envelope through a server-registered `Mapping`. The mapping names one event type, one collection, one document ID field, one tombstone field and an explicit document-field allowlist. There is no HTTP endpoint for an arbitrary collection, field list or tenant identifier.

The trusted tenant code comes only from the validated envelope. A GORM consumer receipt keyed by `consumerID + eventID`, the local projection document and its mapping checkpoint share one transaction. A completed duplicate is skipped; an indexer failure rolls the receipt back so the broker may retry. Adapter `Index` and `Delete` must remain idempotent because a remote call can complete before the local transaction commits.

Deletion records a tombstone and calls the adapter delete operation. `Rebuild` replays the bounded local projection cache, including tombstones, to the registered adapter. The relational business store remains authoritative for access control, lifecycle and audit decisions; search is an optional, default-off derived view.
