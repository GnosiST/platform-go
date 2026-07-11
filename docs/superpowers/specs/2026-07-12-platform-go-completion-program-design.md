# Platform Go Completion Program Design

## Purpose

This document is the single execution index for the remaining `platform-go` work approved on 2026-07-12. It separates completed foundation work from new security, product, documentation and publication work so completed nodes are not reopened accidentally.

## Confirmed Baseline

The existing foundation task graph is complete at 37 total, 37 implemented, 0 pending and 0 blocked. Production promotion remains `not-approved`; runtime mutation, refresh-token-family default runtime and source-writing generation remain disabled. Those boundaries are not changed by this program.

The following are baseline capabilities, not new tasks:

- Gin, GORM, Casbin and JWT backend alignment;
- Refine, React and Ant Design Admin alignment;
- capability manifests, profiles, generated contracts and governance validators;
- tenants, organization units, users, roles, role groups, permissions, menus and area codes;
- OIDC production-like Admin authentication rehearsal and closeout;
- shared Admin UI primitives, responsive behavior, accessibility contracts and six-viewport acceptance;
- deployment topology, production preflight contracts and disabled promotion gates.

## Approved Program Order

1. Runtime security containment.
2. Unified screen and export watermark controls.
3. Sensitive-data encryption, blind indexes and migration support.
4. Open-source portability, developer documentation and community governance.
5. Public documentation website and GitHub Pages deployment.
6. Final GitHub repository creation, push, release and repository presentation.

Later phases may prepare documentation while earlier code work is active, but public release cannot occur until all security and portability gates pass.

## Workstreams

### New Task Graph Nodes

Before implementation, add new task nodes instead of reopening any of the 37 closed nodes:

| Node | Depends on | Primary lock |
| --- | --- | --- |
| `runtime-security-containment` | completed foundation baseline | resource store, auth, file storage, deployment |
| `admin-watermark-export-governance` | `runtime-security-containment` | Admin UI settings, policy-review export |
| `sensitive-data-protection-runtime` | `runtime-security-containment` | field contracts, persistence, key providers |
| `sensitive-data-historical-migration` | `sensitive-data-protection-runtime` | migration command, stores, backup/restore |
| `open-source-portability` | security and data-protection nodes | module path, validators, public quick start |
| `public-docs-community` | `open-source-portability` | public docs and community files |
| `public-docs-site` | `public-docs-community` | `website/`, metadata and screenshots |
| `github-release-publication` | every prior new node | workflows, release export and remote settings |

Each node requires task-execution evidence, objective-conflict review, resource-lock review and a node-closeout entry. Visual nodes also require the existing brainstorming, Product Design and browser-evidence gates.

### A. Runtime Security Containment

Specification: `docs/superpowers/specs/2026-07-12-runtime-security-hardening-design.md`.

Required outcomes:

- schema-allowlisted resource writes and responses;
- forbidden secret-field enforcement;
- production-safe phone verification and keyed hashes;
- authenticated private file delivery without persisted raw session tokens or public upload bypasses;
- production HTTPS, trusted proxy and security-header requirements;
- log, audit and export redaction rules.

### B. Watermark And Export Governance

Specification: `docs/superpowers/specs/2026-07-12-admin-watermark-export-design.md`.

Required outcomes:

- one master switch;
- independent `screen` and `export` scopes;
- exact `1`, `4`, `9` and `16` visual watermark counts;
- backward-compatible persisted UI settings;
- structured watermark metadata for policy-review JSON exports;
- unchanged canonical OpenAPI bytes and unchanged original file downloads.

### C. Sensitive Data Protection

Specification: `docs/superpowers/specs/2026-07-12-sensitive-data-encryption-design.md`.

Required outcomes:

- field sensitivity, storage, response and export metadata;
- versioned AES-256-GCM envelope encryption for recoverable PII;
- separate keyed blind indexes for exact-match lookup;
- immutable format policies and rotatable key material;
- resumable historical migration, verification and rollback;
- a separate Argon2id credential boundary if local password authentication is ever added.

Runtime crypto and historical migration are independent nodes. Historical mutation cannot begin until the format, key-provider, backup and rollback contracts are implemented and verified.

### D. Open Source, Documentation And Publishing

Specification: `docs/superpowers/specs/2026-07-12-open-source-docs-site-design.md`.

Required outcomes:

- Apache-2.0 licensing;
- portable fresh-clone validation without private absolute paths or mandatory `rtk` wrappers;
- contributor, security, support, governance, changelog and release policies;
- current-state architecture, quick start, business-extension tutorial, API, configuration, test, operations, troubleshooting and upgrade guides;
- Docusaurus-based Chinese-default, English-supported public site;
- GitHub Actions CI, Pages deployment, dependency updates and release automation;
- final publication to `GnosiST/platform-go` only after the release gate passes.

## Cross-Cutting Rules

- Use TDD for every behavior change.
- Keep Chinese and English Admin i18n keys synchronized.
- Use `ui-ux-pro-max` for Admin implementation quality and accessibility.
- Use Product Design and `design-taste-frontend` only for the public site and brand-facing surfaces.
- Preserve canonical and original artifacts; derived watermarked or redacted artifacts must be explicit.
- Never commit credentials, tokens, raw subjects, test passwords, private keys or private production evidence.
- Keep changes reviewable through independent commits per workstream.
- Keep the working tree clean after each accepted workstream.

## Program Completion Gate

The program is complete only when:

- all four workstream specifications have implementation plans and verified commits;
- Go tests, Node drift tests, Admin validators, TypeScript, Vite build and diff checks pass;
- security tests prove prohibited plaintext is absent from persistence, responses, logs, exports and public file routes;
- the eight new task nodes and their dependency, resource-lock and closeout evidence are complete;
- browser acceptance covers watermark settings and the public site on mobile and desktop;
- a fresh clone can follow documented commands without local private dependencies;
- GitHub Actions and Pages pass from the public repository;
- repository metadata, topics, social preview, screenshots, release notes and homepage URL are configured;
- the final worktree is clean and the published commit matches the verified local commit.
