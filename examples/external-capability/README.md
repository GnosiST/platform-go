# External Business Capability Template

This is a standalone downstream-style business module. It imports only the public
capability contracts from `github.com/GnosiST/platform-go/pkg/platform/capability`
and is not imported by the default API or Admin process.

Use this template when a product asks whether to start from the generic base by
creating a business service/package or by editing platform source:

- create the business capability outside platform core;
- expose its admin resources, menus, permissions, App routes, auth providers,
  settings entry and lifecycle through `capability.Manifest`;
- register that manifest in the downstream composition root or service build;
- do not add business menus, resources or permission prefixes to platform
  defaults.

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
rtk node --test scripts/validate-external-capability-example.test.mjs
```

The example covers the minimum external-business onboarding shape:

- `capability.Manifest` with a semver capability ID and version;
- admin resource schema, menu, permission prefix, deletion policy, form groups,
  fields, action, panel and runtime slot declarations;
- a capability-owned settings resource, `catalog-settings`, under the
  `configuration` menu parent so `/settings` can aggregate it dynamically;
- App route declaration under `/api/app/**`;
- service contract metadata with trusted tenant context and no client-selected
  physical routing;
- auth provider, migration, seed and demo-data declarations;
- idempotent lifecycle execution through the public recorded executor and
  lifecycle history contracts;
- public-contract validation without importing `internal/platform/**`.

`business-project-template.json` is the static machine-checkable project slice.
The validator checks that it points to the public package import, declares the
business manifest, includes both the operational resource and settings resource,
keeps the settings resource under `configuration`, declares permission prefixes,
and rejects platform-internal imports.

Replace the local `replace` directive with the released module version when
consuming the platform from another repository. A real capability should add its
own persistence, handlers, custom Admin renderers, i18n and tests in that
repository. When the downstream composition root also enables platform tenant
resources, replace free-form ownership fields such as `tenantCode` with
schema-declared relations to the enabled platform resources.

External capabilities participate in the same restart-required desired-state
model as platform capabilities. Installation means the downstream composition
root declares the capability before startup; disabling means removing that
declaration, regenerating contracts and manually restarting the service. The
stock `cmd/platform-api` binary does not hot-load uncompiled external packages
from configuration. Build a downstream API/service artifact that includes the
business manifest, or keep the business service separate and consume generated
platform contracts. Do not patch `internal/platform/**` or default capabilities
to ship one product's business model.

## Startup Path

1. Start from the platform base: pin `github.com/GnosiST/platform-go` in the
   business module, and use the released version instead of a local replace in
   production.
2. Create a business capability package that exports `CatalogManifest()` or an
   equivalent manifest function.
3. Declare business resources in `Manifest.Admin.Resources`. Menus,
   permissions, schemas and configuration entries come from those declarations.
4. Put system-level business configuration in a normal Admin resource whose menu
   parent is `configuration`; `/settings` discovers it through
   `GET /api/capabilities`.
5. Register the manifest in the downstream composition root before resolving the
   enabled capability list. Keep business handlers, stores, state machines and
   custom renderers downstream.
6. Run the validators, regenerate contracts for the target capability set, build
   the downstream API/service artifact and restart the process.
7. Upgrade by bumping platform and capability versions separately. Keep resource
   keys and permission prefixes stable, add idempotent migrations for data
   changes, and roll back by restoring the previous downstream artifact and
   desired capability list.
