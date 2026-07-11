# Open Source Documentation And Site Design

## Goal

Turn `platform-go` into a portable, contributor-friendly Apache-2.0 open-source project with a public developer website hosted on GitHub Pages and a repeatable GitHub release process.

## Publication Boundary

GitHub Pages hosts only the static landing and documentation site. The Gin API remains a long-running service deployed through its own production topology. Public release occurs only after runtime security, watermark, encryption, portability and documentation acceptance gates pass.

The target repository is `GnosiST/platform-go`. It does not exist at design time and must not be created until the final publication phase.

## License And Community Policy

Use Apache License 2.0 because the project is a reusable platform foundation and benefits from explicit patent licensing. Add:

- `LICENSE` and `NOTICE`;
- `CONTRIBUTING.md`;
- `SECURITY.md` with private disclosure instructions and supported-version policy;
- `CODE_OF_CONDUCT.md` based on Contributor Covenant;
- `SUPPORT.md` and `GOVERNANCE.md`;
- `CHANGELOG.md` following Keep a Changelog and SemVer;
- `.github/CODEOWNERS`, Issue forms, PR template and Dependabot configuration.

The initial public copyright holder is `GnosiST`. Changing to a personal or organizational legal entity requires explicit review before the first public release.

## Portability Corrections

Public documentation uses native `go`, `npm`, `node` and `docker compose` commands. The repository's agent-only `rtk` command requirement stays in `AGENTS.md` and internal automation instructions, not in end-user quick starts.

Fresh-clone validators must not require `/Users/irainbow/...`, private Codex assets or a sibling `zshenmez` checkout. Replace live default reference discovery with a tracked, sanitized reference snapshot. Maintainers may opt into live reference validation through an explicit flag or environment variable. CI uses only tracked repository content.

Before publication, migrate the Go module from `platform-go` to `github.com/GnosiST/platform-go` and update every internal import, generated artifact, validator and public example in the same reviewed change. The Admin package remains a private workspace package and is not published to npm.

Historical Superpowers plans and specifications are reviewed before publication. Durable decisions move to public architecture decision records; local paths, private screenshots and agent-specific execution logs are removed or replaced with portable references. Destructive history cleanup is not required; the release tree must be portable and free of sensitive material.

Scan the complete local Git history for credentials, private evidence, personal data and private absolute paths before any remote push. Preserve the existing local development history. Build the public release from a separate sanitized orphan/export branch so the public repository does not inherit private development history. If the scan finds a secret that was used outside the local machine, rotate it even when the public export excludes it.

## Documentation Information Architecture

### Project Entry

- concise README with real screenshots, capability summary, prerequisites, five-minute quick start and documentation links;
- `README.en.md` for the English project entry;
- Chinese is the default public-site locale and English is supported for project entry and core guides;
- badges reflect real CI, release, license and Pages status only after those services exist.

### Learn

- introduction and architecture overview;
- local development and first login;
- capability and profile concepts;
- build a minimal external business capability tutorial;
- Admin resource, route, permission and UI extension walkthroughs.

Provide a separately built `examples/external-capability` module that consumes only public platform contracts and is never imported by the default runtime. The tutorial starts from that module, adds one neutral resource, permissions, Admin registration, App/Admin routes, persistence, i18n and tests, then explains how a real downstream business repository replaces the example namespace.

### Use

- Admin login, navigation, work tabs and system settings;
- resource search, filters, sorting, pagination, create, edit and delete;
- users, organizations, roles, permissions and data scopes;
- files, audit records, API docs and optional policy review;
- common authorization, validation, session and export errors;
- operator-facing initialization checklist for capabilities, identity binding, storage and production-safe settings.

### Reference

- complete environment variable catalog with defaults, allowed values and secret classification;
- CLI command reference for `platform-api`, `platform-admin` and `platform-contracts`;
- generated Admin and App OpenAPI with authentication, errors, pagination and curl examples;
- capability manifest and resource schema reference.

### Operate

- deployment guide and post-deployment smoke tests;
- security hardening checklist;
- backup, restore, migration, key rotation and rollback runbooks;
- monitoring, logging, incident response and troubleshooting;
- version support and upgrade guides.

### Contribute

- development environment, code style, TDD and validation matrix;
- generated artifact and i18n rules;
- commit, review, security and release workflows;
- governance and support boundaries.

## Website Architecture

Use Docusaurus under `website/` because it provides React-based landing customization, Markdown/MDX docs, Chinese/English i18n, versioned documentation, sitemap and GitHub Pages support in one package. Do not build a custom documentation engine.

Product Design owns the brief and three visual directions before implementation. `design-taste-frontend` sharpens the selected public-facing direction. The design read is a technical open-source foundation for backend and full-stack developers: trustworthy, precise and product-aware, with restrained motion and real Admin screenshots rather than decorative abstract graphics.

The landing page includes the product name, literal category, primary screenshot, capability and architecture summary, quick-start command, documentation and GitHub actions, and a visible preview of the next section in the first viewport. It is not a card-heavy SaaS marketing template.

## GitHub Automation

### Pull Request CI

Run Go tests, Node drift tests, generated-contract freshness, Admin i18n/UI/Refine validators, TypeScript, Vite build, website build, markdown link checks, secret scanning and `git diff --check`. Cache Go and npm dependencies without caching generated credentials or runtime state.

### Pages

Build `website/` and deploy the static artifact through GitHub's Pages actions. Configure the repository subpath for `platform-go`; reserve an optional custom domain without requiring one for initial release.

### Releases

The first public version is `v0.1.0`. Add a single repository version source consumed by Go version injection, generated contracts, container labels, the website and release notes. Tags follow SemVer. A release workflow builds checksummed binaries and container metadata, generates an SBOM, signs release artifacts where supported and creates GitHub Release notes from the changelog. No production deployment occurs from the release workflow.

## Repository Presentation

After final verification:

- create a private staging `GnosiST/platform-go` repository only after the local release candidate is complete;
- push the sanitized public-release branch, validate GitHub CI and establish the protected default branch;
- confirm the remote tree and release commit contain no private history or material, then change repository visibility to public;
- configure description, homepage, topics and social preview;
- publish representative desktop and mobile screenshots;
- enable Issues, Discussions when maintainership is ready, security advisories, Dependabot and Pages;
- publish the first tagged release with release notes and checksums.

Only project-owned screenshots and generated graphics may be published. Third-party fonts, icons, code samples and images require license attribution or replacement. Generate a third-party notice inventory and include relevant notices in `NOTICE` and the site.

## Acceptance

- A clean clone on a machine without `rtk`, private paths or sibling repositories can follow the quick start and full validation instructions.
- The external capability example builds and tests as an independent module without entering `platform-default`.
- `go.mod`, internal imports, generated artifacts and public examples use `github.com/GnosiST/platform-go` consistently.
- All public links and local Markdown links pass automated checking.
- Chinese and English navigation and core entry pages build successfully.
- The site has valid metadata, canonical URLs, sitemap, robots policy, favicon and social preview.
- Mobile and desktop browser acceptance shows no overflow, overlap, accessibility regression or console error.
- GitHub CI, Pages and release workflows pass from the public repository.
- Full-history and release-tree secret/privacy scans pass; the public default branch begins from the sanitized release history.
- One version source resolves to `0.1.0` across binaries, generated artifacts, container labels, site and release metadata.
- Repository settings and published commit hashes match the reviewed release record.
