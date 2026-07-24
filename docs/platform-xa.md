# Optional XA Adapter

The XA coordinator is disabled by default and is not part of a normal business unit of work. It supports an explicitly enabled MySQL resource manager that executes the native XA protocol against a fixed, server-owned DSN. The certification smoke uses two ephemeral MySQL instances and verifies `XA START`, `END`, `PREPARE` and `COMMIT` through the coordinator.

The coordinator records committed, rolled-back and heuristic outcomes in a persistent recovery log. XA remains exceptional: normal multi-source writes use Outbox or Saga patterns.
