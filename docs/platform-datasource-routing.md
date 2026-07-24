# Datasource Read/Write Routing

The datasource router accepts only trusted tenant and datasource-group identifiers. Writes and all transactions use the group primary. Read requests may use a healthy replica only while its measured lag is within the configured limit; otherwise they fall back to primary.

After a successful primary commit, the caller invokes `RecordWrite` to pin read-after-write requests for the bounded sticky window. Failed or rolled-back writes must not call it. The router contains no client-selected physical datasource input and does not implement tenant placement or shard routing.
