# External Capability Example

This is a standalone downstream module. It imports only the public capability
contracts from `github.com/GnosiST/platform-go/pkg/platform/capability` and is
not imported by the default API or Admin process.

Run it from this directory:

```bash
go run .
```

Replace the local `replace` directive with the released module version when
consuming the platform from another repository. A real capability should add
its own persistence, routes, permissions, i18n and tests in that repository.
