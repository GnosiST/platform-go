# Organization And User Admin Experience Design

## Goal

Complete the organization and user administration slice without weakening the target organization RBAC contract. Organizations own role-group bindings; tenant-scoped users derive tenant ownership from one primary organization and may hold only roles from that organization's effective role pool.

## Organization Experience

- Organization create keeps role-group binding disabled until the organization exists, because role-group replacement is a versioned domain command rather than generic resource metadata.
- Organization edit exposes the tenant-local role-group set and keeps the selected values separate from projected role-group and effective-role counts.
- A binding change runs prepare, impact, conflict detail and apply in order. Conflict detail pagination must be complete and its row count must match the impact count before remediation can be submitted.
- Existing user-role conflicts require explicit operator confirmation and explicit `remove-role` remediation. Cancellation is a neutral outcome, not a save failure.
- Metadata changes and authorization changes are saved separately. An authorization-only change does not perform a second generic metadata write after the domain command succeeds.
- Organization detail adds a role-pool provenance view with role name, role-group sources and enabled state.

## User Experience

- New tenant-scoped users start with no organization, no derived tenant and no roles. Async option loading must not select the first organization or reset values already entered in the modal.
- Tenant is displayed as a read-only value derived from the selected organization. It is never directly editable in this workflow.
- Role selection stays disabled until an organization is selected and its complete role pool has loaded.
- Role options show their role-group provenance. Existing roles that fall outside the selected organization's role pool remain visible as invalid values until the operator removes them explicitly.
- Organization or role changes use the organization RBAC user-assignment command. Metadata and authorization changes must be submitted separately to avoid ambiguous partial success.
- Platform principals remain the explicit exception: they may have no organization and are constrained to platform roles by the backend.

## Shared Runtime Rules

- Organization and role-group option loaders collect every generic-resource page instead of relying on a page size above the server maximum.
- Role-pool and conflict clients collect every service-object page.
- Context and role-pool failures clear stale options, invalidate stale requests and block unsafe submission.
- Generic delete and direct status toggle actions remain disabled for organizations and users because lifecycle and authorization integrity require governed domain operations.
- Refine third-party telemetry is disabled by default for the reusable Admin foundation.

## Accessibility And Responsive Behavior

- Tenant is semantically read-only, while dependent role controls use real disabled semantics.
- Role-pool status and invalid-role feedback use a polite live region; invalid selection exposes `aria-invalid` and descriptive help.
- Confirmation dialogs use the active Ant Design application context and localized cancel text.
- Role-pool provenance uses semantic list markup.
- Reduced-motion mode removes transient modal animation from acceptance capture.
- The workflow must reflow without page-level horizontal overflow at 375, 390, 768, 1024, 1280 and 1440 CSS pixels.

## Product Design Audit

1. Organization list: healthy. Mobile presents the existing compact list cards and desktop retains the dense table without introducing a second visual language.
2. Organization create: healthy. The role-group field is visibly disabled and explains that binding follows creation; the long form remains operable through an internal modal scroll region.
3. User create entry: healthy. The form preserves platform hierarchy, one primary save action and a clear read-only tenant explanation.
4. User organization context: healthy. Organization begins empty, tenant remains read-only and roles remain disabled until the organization role pool is available.

The audit confirms visible layout, state clarity and responsive reflow from current-run screenshots. Keyboard behavior, ARIA contracts and async state handling are additionally covered by implementation checks and mutation tests. This evidence does not claim full WCAG conformance or assistive-technology certification.

## Acceptance

- Organization role-group changes use prepare, complete impact/conflict retrieval, explicit remediation and apply.
- User tenant and roles are derived and constrained by organization context on both frontend and backend paths.
- New-user modal initialization does not auto-select an organization and does not reset entered values when async options change.
- Six viewport browser acceptance passes with reduced motion, no page-level horizontal overflow, no console errors and no failed first-party requests.
- Product Design, `ui-ux-pro-max`, i18n, Admin UI contracts, generated clients and backend tests remain synchronized.

