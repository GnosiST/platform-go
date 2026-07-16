# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and releases follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Unified role and role-group administration under one strict two-level role-management workbench while retaining compatibility routes.
- Reworked role details and menu/permission entry points with platform tree and Tree Transfer primitives across desktop, mobile and dark mode.

### Fixed

- Kept role authorization modal actions inside short desktop and mobile viewports, propagated active theme tokens into portaled authorization surfaces, localized cancellation controls, and restored focus to the opening command after close.

## [0.1.0] - 2026-07-16

### Added

- Initial public foundation with capability manifests, resource contracts, JWT sessions, Casbin RBAC, GORM persistence and the Refine Admin shell.
- Bilingual public documentation, community guidance and GitHub Pages release surfaces.
- Organization/RBAC/menu end-to-end governance and a generated unified error-code contract.
- A CI governance job that runs the complete Node contract and drift-test suite.

### Changed

- Reconciled the foundation task graph and kept nine post-release data-plane nodes explicitly deferred.
- Kept local agent and design process files available to maintainers while excluding them from tracked release trees and source archives.
- Updated GitHub Actions to the current Node 24-based major versions.
- Established the single-datasource, single-native-transaction support boundary for the `0.1.x` line.

### Fixed

- Restored bounded pagination for assignment-tree service objects and enforced Go/JavaScript definition parity.
- Hardened release-tree portability checks, external evidence URI validation and migration-test fixture isolation.
