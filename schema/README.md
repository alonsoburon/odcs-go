# Vendored ODCS JSON Schema

`odcs-json-schema-v3.2.0.json` is an unmodified copy of the official Open Data
Contract Standard JSON Schema:

- Repository: https://github.com/bitol-io/open-data-contract-standard
- Tag: `v3.2.0` (commit `f0bdad95346905d500be5ef4b2c2d9b1d95223b7`)
- File: `schema/odcs-json-schema-v3.2.0.json` (identical to `schema/odcs-json-schema-latest.json` at that tag)
- URL: https://raw.githubusercontent.com/bitol-io/open-data-contract-standard/v3.2.0/schema/odcs-json-schema-v3.2.0.json

Copyright The Bitol Contributors. Licensed under the Apache License 2.0; see
[`LICENSE-ODCS`](LICENSE-ODCS). It is embedded in the Go package with
`go:embed` and exposed as `odcs.SchemaJSON`.

To update, replace the file with the new upstream version, update the
`go:embed` directive and `SchemaVersion` in the package, and run the tests.
