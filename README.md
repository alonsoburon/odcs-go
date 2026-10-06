# odcs-go

[![CI](https://github.com/alonsoburon/odcs-go/actions/workflows/ci.yml/badge.svg)](https://github.com/alonsoburon/odcs-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alonsoburon/odcs-go.svg)](https://pkg.go.dev/github.com/alonsoburon/odcs-go)

A Go implementation of the [Open Data Contract Standard](https://github.com/bitol-io/open-data-contract-standard)
(ODCS) **v3.2**, the YAML format maintained by [Bitol](https://bitol.io) (LF AI & Data)
for describing datasets: schema, quality rules, SLAs, servers, ownership and support.

It provides:

- **Types** for the whole contract (`DataContract`, `Server`, `SchemaObject`,
  `SchemaProperty`, `Quality`, `SLAProperty`, `Team`, `Role`, …), including nested
  objects, arrays and maps.
- **Parse / Marshal** for YAML and JSON.
- **Validate** against the official ODCS JSON Schema (embedded), with errors that
  carry JSON Pointer paths.
- **Lint**: semantic checks the schema cannot express (duplicates, dangling
  references, units, …).
- **Diff**: compare two versions of a contract and flag breaking changes.
- A small **CLI**, `odcs`, for CI pipelines.

Two dependencies: [`santhosh-tekuri/jsonschema/v6`](https://github.com/santhosh-tekuri/jsonschema)
and [`go.yaml.in/yaml/v3`](https://github.com/yaml/go-yaml).

## Install

```sh
go get github.com/alonsoburon/odcs-go
```

CLI:

```sh
go install github.com/alonsoburon/odcs-go/cmd/odcs@latest
```

Requires Go 1.24 or later.

## Library usage

```go
import odcs "github.com/alonsoburon/odcs-go"

data, err := os.ReadFile("orders.odcs.yaml")
if err != nil {
	log.Fatal(err)
}

// 1. Schema validation of the raw document.
if err := odcs.Validate(data); err != nil {
	var ve *odcs.ValidationError
	if errors.As(err, &ve) {
		for _, e := range ve.Errors {
			fmt.Println(e.Path, e.Message) // /schema/0/properties/2/logicalType value must be one of ...
		}
	}
	log.Fatal(err)
}

// 2. Typed access.
c, err := odcs.Parse(data) // or odcs.ParseFile(path)
if err != nil {
	log.Fatal(err)
}
for _, obj := range c.Schema {
	for _, p := range obj.Properties {
		fmt.Println(obj.Name, p.Name, p.LogicalType, p.Required)
	}
}

// 3. Semantic checks.
for _, issue := range odcs.Lint(c) {
	fmt.Println(issue) // warning: /slaProperties/0/element: reference "tab1.col" does not resolve: ...
}

// 4. Write it back.
out, err := odcs.Marshal(c) // YAML; odcs.MarshalJSON for JSON
```

Breaking-change detection:

```go
old, _ := odcs.ParseFile("v1.odcs.yaml")
new, _ := odcs.ParseFile("v2.odcs.yaml")

changes := odcs.Diff(old, new)
for _, ch := range changes {
	fmt.Println(ch)
}
if odcs.HasBreaking(changes) {
	os.Exit(1)
}
```

```text
BREAKING removed servers[lake]
BREAKING changed schema[orders].properties[amount].logicalType: "number" -> "integer"
BREAKING changed schema[orders].properties[amount].logicalTypeOptions.maximum: 100000 -> 50000 (tightened)
         changed schema[orders].properties[note].logicalTypeOptions.maxLength: 500 -> 1000 (loosened)
         added   schema[orders].properties[channel]
BREAKING added   schema[orders].properties[currency] (new required property)
BREAKING changed schema[orders].quality[row_count].mustBeGreaterThan: 1000 -> 5000 (tightened)
```

Each `Change` has `Kind` (`added`, `removed`, `changed`), a name-based `Path`,
`Old`/`New` values, `Breaking` and an optional `Note`. It marshals to JSON.

## CLI

```text
odcs validate FILE...               check files against the ODCS JSON Schema
odcs lint [-strict] [-json] FILE... schema validation plus semantic checks
odcs diff [-json] OLD NEW           list changes; exit 1 if any is breaking
odcs version                        print the supported ODCS version
```

Exit status: `0` ok, `1` check failed (invalid document, lint errors — or warnings
with `-strict` — or breaking changes), `2` usage or I/O error.

A typical CI gate:

```sh
odcs lint contracts/*.odcs.yaml
git show origin/main:contracts/orders.odcs.yaml > /tmp/orders.old.yaml
odcs diff /tmp/orders.old.yaml contracts/orders.odcs.yaml
```

## What counts as breaking

A change is **breaking** when consumers relying on the old contract can be
affected, or when data that satisfied the old contract may fail the new one.

| Breaking | Not breaking |
|---|---|
| Removed schema object, property, server or role | Added schema object, optional property, server or role |
| Added **required** property | Description, business name, classification, tags |
| Changed `logicalType`, `physicalType`, `physicalName` | `required: true → false`, `unique: true → false` |
| `required: false → true`, `unique: false → true`, primary key changes | Loosened `logicalTypeOptions` (e.g. higher `maxLength`) |
| Tightened `logicalTypeOptions` (lower `maxLength`, higher `minimum`, new `pattern`, changed `format`) | Removed `logicalTypeOptions` |
| Removed enum values, or an enum introduced | Added enum values |
| Server `type`, `host`, `port`, `database`, `schema`, `location`, `format`, … changed | Server `description`, `environment` |
| Added quality rule (non-`text`), tightened threshold, changed metric/query/unit | Removed quality rule, loosened threshold |
| Contract `id` changed | `version`, `status`, SLA values, price, relationships |

Matching is by name (`schema[orders]`, `properties[amount]`, `servers[prod]`),
quality rules by `id`, `name` or `metric`, SLA entries by `property` and `element`.
A rename therefore shows as a removal plus an addition.

## Lint checks

| Severity | Check |
|---|---|
| error | `apiVersion` missing or unknown; `kind` is not `DataContract`; `id` or `version` missing |
| error | Duplicate schema object names, property names (per parent), server names, or stable `id`s within the same list |
| error | Composite relationship with different numbers of `from` and `to` columns |
| warning | Relationship, SLA `element` or `slaDefaultElement` pointing to a schema object or property that does not exist (shorthand `table.column[.nested]` and fully qualified `schema/<id>/properties/<id>` notations; references to other files are not followed) |
| warning | Time-based SLA (`latency`, `retention`, `frequency`, `timeToDetect`, …) without a unit or with an unrecognised time unit |
| warning | `apiVersion` older than v3 |
| info | Uncommon quality unit (not `rows`/`percent`) or unrecognised SLA unit |

The official examples are valid, but `Lint` does find a few dangling references in
them (e.g. `tab1.txn_ref_dt` in the SLA examples); these are reported as warnings.

## Supported ODCS version

- Types and lint follow **ODCS v3.2.0**.
- The embedded schema is the official
  [`odcs-json-schema-v3.2.0.json`](schema/README.md) (tag `v3.2.0`), JSON Schema
  draft 2019-09. Like the upstream schema, it accepts documents with `apiVersion`
  from v2.2.0 to v3.2.0.
- The deprecated pre-v3.1 `team` list form and the shorthand string form of
  `context` are supported and written back as read.

## Design notes

- **Hand-written types.** The ODCS schema relies heavily on `allOf`, `oneOf` and
  `if/then` (server types, quality rule types, `logicalTypeOptions` per logical
  type), which code generators turn into awkward Go. The structs here are written by
  hand to match the schema; `Server` is the union of all server-type fields and
  `logicalTypeOptions` is a `map[string]any`. Validation against the official schema
  covers what the types do not enforce.
- **No unknown fields.** ODCS closes every object (`additionalProperties` /
  `unevaluatedProperties: false`); the extension point is `customProperties`, which
  is preserved with arbitrary values.
- **Numbers keep their literal form** (`odcs.Number`), so `1000000` is not written
  back as `1e+06`.
- **Round trips are semantic.** `Marshal(Parse(x))` validates and parses to the same
  contract, with keys in the standard's order. Comments, key order of the input and
  explicit default values such as `required: false` are not preserved.
- Decoding goes through JSON, so the structs only have `json` tags. Use `Parse` and
  `Marshal` rather than a YAML library directly.

## Relation to the standard

This is an **independent community implementation**. It is not affiliated with or
endorsed by Bitol or LF AI & Data. The standard, its JSON Schema and the example
contracts used in the tests are © The Bitol Contributors under Apache-2.0 (see
[NOTICE](NOTICE)).

Other implementations: [`odcs`](https://crates.io/crates/odcs) (Rust) and
[`pyodcs`](https://pypi.org/project/pyodcs/) (Python). If the Bitol project would
like to host or adopt this library, I would be glad to transfer it.

## Contributing

Issues and pull requests are welcome. Please:

1. Keep the dependency footprint small and the API close to the standard's names.
2. Add table-driven tests; for diff output, update the golden files with
   `go test -run DiffGolden -update` and review the result.
3. Run `gofmt`, `go vet ./...` and `go test ./...` before submitting.

When a new ODCS version is released, update the vendored schema (see
[`schema/README.md`](schema/README.md)), the examples in `testdata/upstream/` and
the types.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
