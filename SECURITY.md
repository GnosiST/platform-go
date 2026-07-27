# Security Policy

## Supported Versions

Security fixes are applied to the latest release line. Development snapshots
may change without notice and are not supported for production security
claims.

## Reporting a Vulnerability

Please do not open a public issue for a suspected vulnerability. Send a private
report to the repository maintainers through the security contact configured on
the GitHub repository. Include the affected version or commit, impact,
reproduction steps and any safe mitigation. Do not include real personal data,
credentials or production tokens.

We will acknowledge a report within seven days, coordinate a fix and disclosure
date with the reporter, and credit the reporter when they request it. If the
repository has GitHub Private Vulnerability Reporting enabled, use that channel
in preference to email.

## Deployment Notes

Use a non-default JWT secret, persistent stores, Redis where required by the
production baseline, disabled demo authentication and the documented migration
and rollback runbooks. Review [docs/platform-auth.md](docs/platform-auth.md)
and [docs/platform-deployment.md](docs/platform-deployment.md) before exposing
the API.

Node production dependency audits run through
`scripts/run-node-production-audit.mjs`. Any temporary exception must name one
package and one GHSA in `resources/node-production-audit-policy.json`, include
a concrete rationale and expiry date, and remain limited to a runtime mode the
repository does not use. Expired, unknown, high or critical advisories still
fail CI; do not lower the audit threshold to bypass them.
