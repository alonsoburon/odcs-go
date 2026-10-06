# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-10-06

First release, targeting ODCS v3.2.0.

### Added

- Go types for the full ODCS v3.2.0 contract, including nested properties,
  arrays, maps, relationships, quality rules, SLAs, team, roles, support,
  pricing, custom properties and the `context` block.
- `Parse`, `ParseFile`, `Marshal` and `MarshalJSON` for YAML and JSON documents.
- `Validate` against the embedded official ODCS v3.2.0 JSON Schema, with JSON
  Pointer paths in errors.
- `Lint` for semantic checks: duplicates, dangling references, units,
  `apiVersion`, `id`.
- `Diff` with breaking-change classification.
- `odcs` CLI with `validate`, `lint` and `diff` commands.

[0.1.0]: https://github.com/alonsoburon/odcs-go/releases/tag/v0.1.0
