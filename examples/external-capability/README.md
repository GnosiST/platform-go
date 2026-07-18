# External Capability Example

This is a standalone downstream-style module. It imports only the public
capability contracts from `github.com/GnosiST/platform-go/pkg/platform/capability`
and is not imported by the default API or Admin process.

Run it from this directory:

```bash
rtk go test ./...
rtk go run .
```

`go run .` validates the manifest through the public registry and prints a
JSON-safe contract preview. It intentionally does not marshal
`capability.Manifest` directly because manifests may contain lifecycle hooks.

From the repository root, the maintained regression gate is:

```bash
rtk node scripts/validate-external-capability-example.mjs
```

The example covers the minimum external-business onboarding shape:

- `capability.Manifest` with a semver capability ID and version;
- admin resource schema, menu, permission prefix, deletion policy, form groups,
  fields, action, panel and runtime slot declarations;
- App route declaration under `/api/app/**`;
- service contract metadata with trusted tenant context and no client-selected
  physical routing;
- auth provider, migration, seed and demo-data declarations;
- idempotent lifecycle execution through the public recorded executor and
  lifecycle history contracts;
- public-contract validation without importing `internal/platform/**`.

Replace the local `replace` directive with the released module version when
consuming the platform from another repository. A real capability should add
its own persistence, routes, permissions, i18n and tests in that repository.
When the downstream composition root also enables platform tenant resources,
replace free-form ownership fields such as `tenantCode` with schema-declared
relations to the enabled platform resources.

External capabilities participate in the same restart-required desired-state
model as platform capabilities. Installation means the downstream composition
root or selected profile declares the capability before startup; disabling means
removing that declaration, regenerating contracts and manually restarting the
service. v1 does not hot-load external packages, pull source from remote
repositories, remove source code, purge persisted data or expose WebSocket
progress. After a capability is disabled and the process restarts, its Admin
resources, App routes, auth providers and demo data sets must disappear from
the generated/exposed contracts while downstream data remains owner-managed.
