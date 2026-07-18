# Global Exhibition Business Assessment

## Decision

The platform can support a worldwide exhibition intelligence business, but the exhibition domain should be delivered as a downstream business capability rather than promoted into the default platform foundation.

The reusable platform foundation should provide identity, RBAC, tenant/org/area data scopes, generic Admin resources, App route contracts, audit, file storage and generated API contracts. Exhibition collection, exhibitors, booth builders, venues, source crawling, enrichment, deduplication and review workflows are domain capabilities and must stay outside `internal/platform/**`.

## Address-Code Scope

Use `area-codes` as global regional master data, not as a China-only administrative code table. The contract now supports these tree levels:

- `continent`
- `country`
- `subdivision`
- `state`
- `province`
- `city`
- `district`
- `street`
- `custom`

Recommended code conventions:

- Use ISO 3166-1 alpha-2 values for country roots where possible, such as `CN`, `US`, `DE` and `AE`.
- Use ISO 3166-2 style values for first-level subdivisions when available, such as `US-CA`, `DE-BE` or `CN-110000`.
- Use stable business-owned codes for venues, halls or local operating regions only when public standards do not fit.
- Keep detailed postal/street/contact addresses in the owning exhibition capability. Do not add `addresses` or `user-addresses` to the platform default foundation.

The default seed data should remain small. A production exhibition product should import global countries and target-market subdivisions through a reviewed seed/import job owned by the business deployment, not by hard-coding a full world geographic dataset into `platform-go`.

## Business Capability Shape

A first production-oriented capability can be named `exhibition-intelligence` or live in a downstream business repository. It should import only the public manifest contract from `github.com/GnosiST/platform-go/pkg/platform/capability`.

Recommended Admin resources:

- `exhibition-events`: exhibition name, industry/category, event dates, organizer, venue reference, `areaCode`, country, city, source status and collection status.
- `exhibitors`: company profile, official website, brands, industries, country/area ownership, contacts classification and verification status.
- `exhibition-exhibitors`: event-to-exhibitor relation, booth number, hall, source confidence and verification state.
- `booth-builders`: stand contractors or booth builders, service regions, supported industries, cases, verification status and contact policy.
- `venues`: venue name, area code, address text, halls, organizer links and source provenance.
- `source-records`: original URL/source/provider, captured snapshot hash, normalized target, confidence and review state.
- `collection-jobs`: crawl/import task status, rate-limit profile, source category and failure reason.

Use relation fields to the foundation where applicable:

- `tenantCode -> tenants`
- `orgUnitCode -> org-units`
- `areaCode -> area-codes`

This lets the existing data-scope layer filter regional or tenant-owned records after RBAC allows the read action.

## Feasibility

Feasible now for an MVP:

- Admin CRUD, filtering, structured query and generated OpenAPI can be exposed through capability-declared resources.
- RBAC, role menus, audit logs and area data scopes already support regional back-office operations.
- GORM-backed generic resources can store early exhibition records before a dedicated domain repository is justified.
- File storage can hold brochures, floor plans, source snapshots and evidence attachments.

Needs additional domain work before production scale:

- Source ingestion adapters with rate limits, robots/legal review and retry evidence.
- Deduplication across event names, venues, exhibitors, domains and source records.
- Search projection for multilingual names, industries, dates, countries, cities and venue names.
- Review workflow for confidence, conflicts, stale data and manual approval.
- Contact-data classification, masking and reveal policy for emails, phones and personal contacts.
- Import/export governance for bulk global address and exhibition datasets.

## Implementation Path

1. Create a downstream `exhibition-intelligence` capability manifest with Admin resource schemas and permissions.
2. Seed only a small global area-code baseline in development, then add deployment-owned import jobs for the larger geographic dataset.
3. Start with generic GORM-backed Admin resources for collection and review screens.
4. Add App routes only after public or customer-facing search/query flows are defined.
5. Promote hot paths to dedicated tables/search projections once deduplication and query load are proven.
6. Keep source snapshots, crawl logs and contact details governed by audit, file storage and sensitive-field policies.

## Boundary

Do not add exhibition resources to `platform-default`. The platform should make this business easy to build, but it should not become an exhibition-specific product by default.
