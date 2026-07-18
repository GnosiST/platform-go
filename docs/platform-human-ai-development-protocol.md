# Human + AI Development Protocol

`platform-go` is designed for human developers and AI agents working on the same business-neutral foundation. The rule is contract-first customization: describe the business surface through platform contracts, generate reviewable artifacts, then implement only behind approved ports and wrappers.

This document is the public protocol for starting a highly customized business system on top of the platform. It is backed by `resources/platform-human-ai-development-protocol.json` and `scripts/validate-platform-human-ai-development-protocol.mjs`.

## Operating Model

1. Read `AGENTS.md`, this document, `docs/platform-capability-development.md` and `examples/external-capability/README.md` before changing platform or business surfaces.
2. Decide the customization mode:
   - external business package: preferred for product-specific systems;
   - platform extension capability: only for reusable platform behavior;
   - generated scaffold preview: safe default for codegen output and review.
3. Declare the contract before implementation: `capability.Manifest`, Admin resources, App routes, service contracts, ownership fields, permissions and lifecycle steps.
4. Generate or refresh contract artifacts before writing runtime code.
5. Implement through platform ports: storage, HTTP route registration, Admin action handlers, Refine data provider, App request/upload ports and service Query/Command objects.
6. Run the changed-plane validators and keep evidence in docs, generated artifacts or review packages.

## Collaboration Rules

Human and AI contributors follow the same gates.

- Make assumptions explicit when they affect architecture, data, security, UI behavior, generated source, runtime mutation or production operations.
- Do not place business-specific menus, resources, route handlers, state machines or tables into platform core.
- Do not import `internal/platform/**` from a downstream business repository. Extend `pkg/platform/capability` first when the public contract is insufficient.
- Do not let AI-generated code overwrite handwritten runtime files. Source-writing codegen remains disabled unless the reviewed promotion package in `resources/platform-codegen-source-writing-readiness.json` is complete.
- Treat screenshots, local demos and generated previews as evidence, not approval. Production mutation, destructive data work and source-writing promotion require human review plus rollback evidence.

## Interface Contracts

API and service work starts from contracts, not page code.

| Surface | Source of truth | Gate |
| --- | --- | --- |
| Admin resources | `resources/admin-resources.json` plus enabled `capability.Manifest.Admin.Resources` | `rtk node scripts/validate-admin-resources.mjs` |
| Admin API | `resources/generated/openapi.admin.json` | `rtk node scripts/validate-platform-admin-api-boundary.mjs` |
| App API | `capability.Manifest.App.Routes` and `resources/generated/openapi.app.json` | `rtk node scripts/validate-platform-app-client-api-boundary.mjs` |
| Service contracts | `capability.Manifest.Service` | `rtk node scripts/validate-platform-service-contract-standard.mjs` |

Admin collection queries use `POST /api/admin/resources/:resource/query`. App routes stay under `/api/app/**` and use app-domain tokens. Service contracts declare identity, trusted tenant context, operations, events, reliability, PII and compatibility before handlers are attached.

## UI And Visual Customization

Admin customization stays on the shared product UI system:

- Use platform UI wrappers over Ant Design for repeated pages, tables, modals, drawers, buttons, dropdowns, pagination and settings.
- Keep schema-driven CRUD on Refine providers and `PlatformResourceForm` unless the resource declares a controlled action, panel or runtime slot.
- Put business workflow panels in the downstream capability package and attach them through manifest-declared routes/actions/panels.
- Keep visual customization in theme, layout, density, branding and registered components. Avoid page-local forks that bypass accessibility, focus, i18n or reduced-motion behavior.

Public documentation pages may use distinct art direction, but Admin workflows must remain dense, predictable and task-first.

## Code Generation

The platform supports generated contracts, OpenAPI, previews and scaffold packages. Runtime source-writing is not enabled by default.

Safe path:

```bash
rtk go run ./cmd/platform-contracts admin-resources --output resources/generated/admin-capability-resource-contract.json
rtk go run ./cmd/platform-contracts app-routes --output resources/generated/app-route-contract.json
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-app-openapi.mjs
rtk node scripts/generate-app-codegen-preview.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
```

Any future source-writing promotion needs an explicit spec, reviewed diff, target-root owner approval, rollback plan and target-family test output. AI may draft generated artifacts, but it must not approve its own runtime promotion.

## Data, Security And Isolation

Business records declare ownership fields only when the dimension applies:

- `tenantCode` for tenant isolation;
- `orgUnitCode` for organization/team ownership;
- `areaCode` for regional ownership or filtering;
- dedicated business relations for domain-specific coverage or membership.

Authorization is server-side. Row ownership metadata is not an access grant by itself; roles, permissions and data scopes remain authoritative. Sensitive fields, reveal policies, lifecycle retention and production operations must use their dedicated manifest/configuration gates.

## Minimum Acceptance

For a meaningful business customization, start with the narrow checks for the changed plane, then add the common protocol gates:

```bash
rtk node scripts/validate-platform-human-ai-development-protocol.mjs
rtk node scripts/validate-external-capability-example.mjs
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-platform-admin-api-boundary.mjs
rtk node scripts/validate-platform-app-client-api-boundary.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-platform-codegen-source-writing-readiness.mjs
rtk npm --prefix admin run build
```

Add `rtk go test ./...`, website build, browser checks or production preflight when the changed surface touches backend behavior, public docs, visual workflows or deployment operations.
