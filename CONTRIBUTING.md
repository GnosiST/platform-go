# Contributing to platform-go

Thank you for helping improve platform-go. The project is a business-neutral
Gin + GORM + Casbin + JWT foundation with a React/Refine/Ant Design admin
client. Contributions should preserve those boundaries and keep business
capabilities in external packages.

## Before You Start

- Search existing issues before opening a new one.
- For security issues, follow [SECURITY.md](SECURITY.md) instead of opening a
  public issue.
- For business customization, follow the
  [Human + AI development protocol](docs/platform-human-ai-development-protocol.md)
  so interface, UI, visual, data and codegen contracts are declared before
  implementation.
- For substantial architecture or API changes, describe the contract and
  migration plan in an issue first.

## Local Development

Install Go, Node.js and npm supported by the repository manifests. Then run:

```bash
go test ./...
npm --prefix admin install
npm --prefix admin run build
```

The commands above are the public contributor workflow; no private agent
tooling or local machine configuration is required.

## Change Requirements

- Keep capability manifests and resource schemas as the contract source.
- Use platform ports instead of reaching into concrete storage or handlers.
- Add Chinese and English i18n keys for shared Admin UI changes.
- Add focused tests and update generated artifacts when their source changes.
- Do not commit secrets, local absolute paths, private screenshots or runtime
  state.
- Use structured, versioned error responses for public API changes.

## Pull Requests

Explain the behavior change, compatibility impact, migration/rollback plan and
verification commands. Keep unrelated formatting or refactoring out of the
same pull request. Maintainers may request a design note or contract test for
changes that affect authorization, persistence, public APIs or generated files.

## License

By contributing, you agree that your contribution is provided under the
Apache License 2.0, as described in [LICENSE](LICENSE).
