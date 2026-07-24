# Sharding And Tenant Migration

Shard routes are resolved only from trusted tenant context and a registered capability. Each route carries a datasource group, shard identifier and monotonic placement version; clients cannot select physical routing.

Tenant migration creates a cryptographic global identifier and persists prepared/running/compensated state. Checkpoints are monotonic and stored with an append-only audit entry in the same GORM transaction. Compensation is explicit and auditable; it does not claim a distributed transaction or automatically move business records.
