# Tenant Placement

Tenant placement is an internal control-plane store keyed by trusted `tenantCode` and capability ID. It resolves only from `kernel.ExecutionContext`; client requests cannot supply a datasource, database, schema or shard.

Placements are persisted with a monotonic version. Create and transition write an audit entry in the same GORM transaction. A scoped override requires the exact tenant and capability, independent authorization, a non-empty reason and an expiry no later than fifteen minutes; each successful override is audited and never persists as a placement transition.

This control plane records datasource-group placement only. Read/write replica routing and shard routing are separate runtime layers.
