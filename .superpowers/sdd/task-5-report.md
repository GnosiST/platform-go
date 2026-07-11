# Task 5 Report: Explicit OIDC Binding Provisioning Command

## Status

Implemented the explicit `platform-admin bind-admin-oidc` operator command, production initialization documentation and production OIDC environment wiring. No API startup automation, deployment mutation or production promotion was added or executed.

## Delivered Scope

- Added `cmd/platform-admin/main.go` with the required injectable `run` entrypoint.
- Added `cmd/platform-admin/main_test.go` covering stdin-only subject handling, provider/config validation, Admin principal validation, idempotency, conflict rejection and raw identity redaction.
- Updated `docs/platform-auth.md` with the exact safe command, first-start order, idempotency, conflict and redaction boundaries.
- Added `admin-oidc` and placeholder OIDC variables to `deploy/env/production.example.env`.
- Passed OIDC variables into the production API service in `deploy/compose/docker-compose.prod.yml` without adding automatic provisioning.

## TDD Evidence

- RED: `rtk go test ./cmd/platform-admin -count=1` failed with `cmd/platform-admin/main_test.go:172:9: undefined: run` before production code existed.
- GREEN: `rtk go test ./cmd/platform-admin -count=1` passed 13 tests after the minimal implementation.

## Verification

- `rtk go test ./cmd/platform-admin ./internal/platform/httpapi -run 'Provision|BindAdminOIDC|AdminIdentity' -count=1`: 28 tests passed in 2 packages.
- `rtk go build ./cmd/platform-admin`: passed; the generated local binary was removed from the working tree.
- `rtk node scripts/validate-platform-deployment-topology.mjs`: passed.
- `rtk git diff --check`: passed.

## Self-Review

- Raw subject input is accepted only after `--subject-stdin` and is read exclusively from stdin.
- `--subject` and positional arguments are rejected without echoing their values.
- Provider, issuer and username are validated before provisioning; the issuer must match the configured OIDC issuer.
- The command uses `bootstrap.CapabilitiesFromConfig`, `bootstrap.AdminResourcesFromConfig`, the resource-backed `AdminIdentityBindingStore` and its atomic `ProvisionAdminIdentityBinding` method.
- Missing, disabled and permissionless users return the same normalized rejection.
- Success output contains only provider ID and platform username.
- Persistent bindings and provisioning audits contain no raw issuer or subject.
- The audit write follows the atomic binding write. If audit persistence fails, the command returns an error; an explicit retry is safe because binding provisioning is idempotent.

## Attention

The production Compose image was not changed to bundle or automatically run `platform-admin`; that is intentional within Task 5's file boundary. Operators must make the compiled command available in a trusted environment with access to the same persistent Admin store and configuration before starting a demo-disabled API for the first time.
