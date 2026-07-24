# Federated Read Reports

Federated reports are registered, versioned definitions. Definitions are persisted through GORM, then revalidated against server-owned source bindings when the runtime starts. Requests contain only report ID and version; clients cannot provide SQL, datasource groups, tables, fields or tenant identifiers.

Each GORM source binding declares one datasource group, table and field allowlist. Reads require trusted tenant context, add a `tenant_code` predicate, select only declared fields and apply row, cost and timeout limits. Results remain projections; the relational stores remain authoritative.
