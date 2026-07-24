# Datasource Registry

The optional datasource registry is a versioned, process-owned JSON configuration. It binds enabled capability IDs to named datasource groups. A group has one primary datasource and may declare replicas for the later read/write-routing node.

The registry never contains a DSN, password, database name or schema. Each datasource has only a canonical `configRef`; the process resolves it at startup from protected environment configuration, opens the connection and performs a ping. Runtime health exposes only `datasourceId` and `ready`.

Set `PLATFORM_DATASOURCE_REGISTRY_FILE` to an immutable, read-only registry file. Leave it empty for the supported single-datasource deployment. The API validates every capability binding against the enabled manifest set and closes all opened connections if startup fails.

`Runtime.Transaction` and `Runtime.TransactionForGroup` pin a callback to exactly one primary datasource. The registry does not permit client-controlled datasource, database, schema or shard selection. It does not enable tenant placement, read/write routing, sharding, federation or XA; those are separate controlled runtime nodes.
