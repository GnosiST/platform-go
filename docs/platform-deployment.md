# Platform Deployment

`platform-go` deployment is adapter-based. The reusable foundation must not hard-code one hosting vendor as the default runtime.

## Deployment Decision

Vercel is optional. It is a good fit for the `admin` Vite React build and preview environments, but it is not the default deployment target for the Gin API process.

The default production shape is:

- `admin`: static assets served by a CDN, reverse proxy, object host or optional Vercel project;
- `platform-api`: long-lived Go service running `cmd/platform-api`;
- persistence: MySQL, PostgreSQL or SQLite through GORM-backed admin resources, sessions and lifecycle history;
- cache and invalidation: Redis;
- files: local storage for single-node development, S3-compatible storage for multi-instance deployments;
- business capabilities: attached outside the platform foundation through manifests and process composition.

Selected scheme A is `single-service-production`: build the Gin API as a long-lived service, serve `admin/dist` through the same reverse-proxy origin, and keep browser API calls on `/api`. Vercel remains an optional admin-static adapter, not the selected default deployment topology.

## Supported Topologies

### Local Development

- Run the API with `go run ./cmd/platform-api`.
- Run the admin with `rtk npm --prefix admin run dev`.
- The Vite dev server proxies `/api/` to `VITE_PLATFORM_API_PROXY_TARGET`, defaulting to `http://127.0.0.1:9200`.

### Single Long-Lived Service

Use this for the simplest production-like deployment:

- build `cmd/platform-api` as a long-lived process or container;
- serve `admin/dist` from the same reverse proxy domain;
- proxy `/api/*` to the Go process;
- keep the browser API base as `/api`.

This avoids browser CORS complexity and keeps the admin/API auth boundary same-origin.

The standard adapter is an origin behind a reviewed external TLS edge. It binds the Admin proxy to loopback by default, installs `platform.conf` through the official Nginx `/etc/nginx/templates` envsubst entrypoint, redirects to `PLATFORM_PUBLIC_BASE_URL` rather than the request Host, and emits HSTS only after the reviewed edge supplies one canonical `https` signal. Configure `PLATFORM_TRUSTED_PROXIES` so `PLATFORM_ADMIN_PROXY_IP` is contained by the API policy. Configure `PLATFORM_EDGE_TRUSTED_PROXY` as one canonical direct edge peer IP inside `PLATFORM_INTERNAL_SUBNET`; CIDRs, loopback, unspecified and multicast addresses are rejected. Nginx accepts real IP and forwarded protocol only from that peer, then overwrites `X-Forwarded-For` instead of appending caller-controlled chain state. Reject API trusted-proxy policies that cumulatively cover all IPv4 or IPv6 addresses, and set a positive bounded `PLATFORM_HTTP_MAX_BODY_BYTES`. The API container healthcheck is the sole HTTP exception: direct loopback `GET /api/health`, with no forwarded-header trust and no HSTS. Never expose port 8080 as an unreviewed public HTTP origin.

All browser credential requests, including JWT login/refresh/logout, provider callbacks and API-token use, require this production HTTPS boundary. The application does not add reversible payload encryption around bearer credentials; TLS is the transport confidentiality and integrity layer. Provider and object-storage endpoints must also use HTTPS outside loopback development and test. API responses and runtime error sinks expose stable public/error codes only: raw adapter errors, personal values, credentials, session handles/digests and object paths must not enter responses, audits or production logs.

The repository includes a standard adapter package for this topology:

- `Dockerfile`: multi-stage build with `api` and `admin-static` targets;
- `deploy/compose/docker-compose.prod.yml`: single-node production-like composition with API, admin static proxy, MySQL and Redis;
- `deploy/nginx/platform.conf`: serves `admin/dist` and proxies `/api/` to the Go service; file bytes are never exposed through a static alias;
- `deploy/env/production.example.env`: production environment template with `demo-data` removed, demo auth disabled, bounded upload policy, private S3 encryption policy and the optional `admin-oidc` provider configuration declared.

Use the package as a reviewable starting point:

```bash
rtk node scripts/validate-platform-production-env.mjs
rtk node scripts/validate-platform-objective-conformance.mjs
rtk docker compose -f deploy/compose/docker-compose.prod.yml --env-file deploy/env/production.example.env config
rtk node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env> --run
rtk docker compose -f deploy/compose/docker-compose.prod.yml --env-file <private-production-env> up --build -d
```

Copy `deploy/env/production.example.env` to a private environment file before deployment. Replace every secret, keep `PLATFORM_CAPABILITIES` business-neutral, and do not re-add `demo-data` in production. When `admin-oidc` is enabled, run the stdin-only `platform-admin bind-admin-oidc` procedure in `docs/platform-auth.md` against the same Admin store before starting the demo-disabled API.

File content is delivered only through the authenticated Admin or App content endpoints. Do not add an Nginx `/uploads` location, point an Nginx `alias` or `root` at upload storage, mount any volume into the Admin proxy, or configure a public file URL. The deployment topology validator parses Compose as YAML and inspects active service mappings; commented examples do not count as runtime configuration. `PLATFORM_HTTP_MAX_BODY_BYTES` is applied to every request body except valid multipart requests on the two declared file-upload paths, which remain governed by `PLATFORM_FILE_MAX_UPLOAD_BYTES` and the MIME allowlist. Declared, detected and allowed MIME values are compared by their canonical base media type after `mime.ParseMediaType`, so parameters such as `charset` are not compared directly. Object keys are cryptographically random opaque identifiers and never include the original filename. S3 deployments must use HTTPS and explicitly select `AES256` or `aws:kms`; `aws:kms` also requires `PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID`. Before promotion, operators must independently verify bucket-level Block Public Access and private bucket policy. The application configures `PutObject` encryption and no public ACL, but it does not claim to inspect external bucket policy.

File deletion is a recoverable tombstone/outbox flow, not a direct metadata delete. The API atomically stores `deletionState=pending`, `deletionRequestedAt` and a redacted `file.delete.request` audit before calling object storage. Tombstoned files disappear immediately from list/query/content access while the internal object reference remains available for idempotent cleanup retry. Missing objects count as cleanup success; other object-delete failures retain the tombstone. Metadata is purged with the completion audit only after object deletion succeeds.

### Split Admin And API

Use this when the admin needs static CDN hosting:

- build `admin` with `rtk npm --prefix admin run build`;
- host `admin/dist` on Vercel or another static host;
- deploy `platform-api` as a long-lived service;
- either proxy `/api/*` from the static host to the API service, or set `VITE_PLATFORM_API_BASE=https://<api-host>/api`.

If `VITE_PLATFORM_API_BASE` points at a different origin, the API deployment must provide a reviewed platform-level CORS configuration. Do not add business-specific CORS code.

## Vercel Policy

Vercel can be added as an admin-only adapter when a project wants hosted previews or global static delivery.

Recommended Vercel settings for the admin project:

```text
Root Directory: admin
Build Command: npm run build
Output Directory: dist
Environment: VITE_PLATFORM_API_BASE=https://<api-host>/api
```

The repository includes `deploy/vercel/admin.vercel.json` as a copyable admin-only template for this topology. Treat it as the optional adapter package recorded in `resources/platform-deployment-topology.json`: copy it to `admin/vercel.json` only in projects that choose Vercel for the admin static build. The template keeps `framework=vite`, `buildCommand=npm run build`, `outputDirectory=dist` and a SPA fallback rewrite to `/index.html`; it intentionally does not declare Vercel functions, Go builds, API routes or platform API runtime settings.

Bind the browser to the API through `VITE_PLATFORM_API_BASE=https://<api-host>/api`, or through a reviewed edge proxy. The adapter package forbids API runtime wiring in the template itself: no `functions`, `builds`, `routes`, Go build commands, Vercel Go runtime snippets or `/api` rewrites should be added to `deploy/vercel/admin.vercel.json`.

If same-origin behavior is required, configure the deployment edge outside this template so browser requests still use `/api/*`. Otherwise, add the platform CORS slice first.

Do not deploy the current API as the default Vercel runtime. A future Vercel Go Runtime adapter must have a separate architecture spec and must prove:

- compatibility with Vercel request handling and port/runtime conventions;
- production GORM stores for admin resources, sessions and lifecycle history;
- Redis cache and invalidation;
- external file storage;
- disabled demo data and demo auth provider;
- production auth and source-writing promotion gates remain unchanged;
- rollback and observability are equivalent to the long-lived service topology.

## Production Environment

Production runtime must set:

```bash
PLATFORM_RUNTIME_ENV=production
PLATFORM_PUBLIC_BASE_URL=https://platform.example.test
PLATFORM_INTERNAL_SUBNET=172.30.0.0/24
PLATFORM_ADMIN_PROXY_IP=172.30.0.10
PLATFORM_TRUSTED_PROXIES=172.30.0.10
PLATFORM_EDGE_TRUSTED_PROXY=172.30.0.1
PLATFORM_HTTP_MAX_BODY_BYTES=1048576
PLATFORM_CAPABILITIES=tenant,identity,session,rbac,menu,api-resource,audit,admin-oidc,dictionary,parameter,file-storage,admin-shell,system-admin
PLATFORM_JWT_SECRET=<at-least-32-characters-and-not-the-dev-default>
PLATFORM_DATA_KEY_PROVIDER=env-aes256
PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID=enc-v1
PLATFORM_DATA_ENCRYPTION_KEYRING_JSON={"enc-v1":"<base64-32-byte-key>"}
PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID=idx-v1
PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON={"idx-v1":"<different-base64-32-byte-key>"}
PLATFORM_ADMIN_RESOURCE_DRIVER=mysql
PLATFORM_ADMIN_RESOURCE_DSN=<dsn>
PLATFORM_SESSION_DRIVER=mysql
PLATFORM_SESSION_DSN=<dsn>
PLATFORM_LIFECYCLE_HISTORY_DRIVER=mysql
PLATFORM_LIFECYCLE_HISTORY_DSN=<dsn>
PLATFORM_CACHE_DRIVER=redis
PLATFORM_REDIS_ADDR=<host:port>
PLATFORM_RATE_LIMIT_HMAC_KEY=<dedicated-at-least-32-byte-secret>
PLATFORM_FILE_MAX_UPLOAD_BYTES=10485760
PLATFORM_FILE_ALLOWED_MIME_TYPES=application/pdf,image/jpeg,image/png,text/plain
PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true
PLATFORM_ADMIN_OIDC_ISSUER_URL=https://identity.example/realms/platform
PLATFORM_ADMIN_OIDC_CLIENT_ID=platform-admin
PLATFORM_ADMIN_OIDC_CLIENT_SECRET=<redacted-secret>
PLATFORM_ADMIN_OIDC_REDIRECT_URL=https://admin.example/login
PLATFORM_ADMIN_OIDC_SCOPES=openid,profile,email
```

Data-protection settings are initialization-time compatibility contracts. `PLATFORM_DATA_KEY_PROVIDER` must be `env-aes256` in production. Both keyrings are JSON objects keyed by canonical version IDs, and every value is a standard-base64-encoded 32-byte key. Encryption and blind-index key material must be distinct. The active IDs select new writes only; startup still requires every historical key referenced by stored envelopes. To rotate, add the new material under a new ID, deploy with the old and new entries present, change the active ID, verify reads and backups, and retire an old entry only after a separately approved migration proves no envelope references it. Replacing material under an existing ID fails startup.

The standard env template contains recognizable placeholder material and passes only the non-strict shape check. Private production files must pass `--strict-secrets`. Do not commit real keyrings or print them in logs, traces, errors or audit records. `local-test` is limited to development/test. KMS/HSM providers and an authorized reveal HTTP flow are not implemented by this runtime.

Historical plaintext migration is an offline maintenance workflow, not an API deployment step. MySQL and PostgreSQL remain production targets only after real driver/version integration rehearsal and certification evidence exists; SQLite is accepted only in development/test for local rehearsal and fails closed in staging/production. Oracle, Kingbase, file mutation and legacy SQL mutation are outside the certified boundary. Before migration, operators must create an external backup and retain isolated restore evidence; encrypted escrow is not a replacement. Follow [Sensitive Data Historical Migration Runbook](platform-sensitive-data-migration.md) for inventory, dry-run, prepare, apply, verify, restore rehearsal, rollback, resume and incident-stop procedures.

Production `PLATFORM_CAPABILITIES` must not be empty and must not include `demo-data`. Capability IDs are trimmed, must use lowercase letters, numbers and hyphens, and must not contain empty or duplicate comma-separated entries. Use `minimal-admin` for the smallest supported admin foundation, or include `admin-oidc` with complete OIDC configuration when OIDC is the Admin provider. The OIDC subject must enter only through `platform-admin bind-admin-oidc --subject-stdin`; API startup does not provision accounts or authorization relationships.

When the optional `app-phone` capability is enabled, also set `PLATFORM_PHONE_HMAC_KEY=<dedicated-at-least-32-byte-secret>`, `PLATFORM_PHONE_CODE_HMAC_KEY=<different-dedicated-at-least-32-byte-secret>` and `PLATFORM_PHONE_VERIFICATION_PROVIDER=<configured-provider-id>`. The rate-limit, phone and verification-code HMAC keys must be mutually distinct, and the production verification provider must not be `debug`.

The local Keycloak rehearsal documented in `design-qa.md` proves the protocol, binding, session and browser paths against production-like components. It does not approve an external production promotion or satisfy provider-secret ownership, rotation, rollback and release-approval requirements.

`rtk node scripts/validate-platform-production-env.mjs` validates the standard template shape. Use `rtk node scripts/run-platform-production-preflight.mjs --command production-env-audit --strict-env-file <private-production-env>` for a dry-run view of the strict env check, then add `--run` for real deployment files so copied placeholders, weak compose database passwords, `demo-data`, demo auth, non-Redis cache and non-GORM stores fail before startup.

## Verification

Run deployment topology validation before release planning:

```bash
rtk node scripts/run-platform-production-preflight.mjs --list
rtk node scripts/run-platform-production-preflight.mjs --policy database-migration
rtk node scripts/validate-platform-deployment-topology.mjs
rtk node scripts/validate-platform-production-readiness.mjs
rtk node scripts/validate-platform-sensitive-data-migration.mjs
rtk go test ./...
rtk npm --prefix admin run build
```

The topology contract is `resources/platform-deployment-topology.json`. It records Vercel as optional admin static hosting, keeps the API on a long-lived-service default, checks the standard Docker/Nginx/compose deployment package, and rejects treating Vercel as the mandatory full-stack foundation runtime.
