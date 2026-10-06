// Package odcs implements the Open Data Contract Standard (ODCS) v3.2 in Go.
//
// ODCS is a YAML format, maintained by the Bitol project (LF AI & Data), for
// describing a dataset: its schema, quality rules, service levels, servers,
// ownership and support channels. See
// https://github.com/bitol-io/open-data-contract-standard.
//
// The package provides:
//
//   - Go types for the whole contract ([DataContract] and friends);
//   - [Parse], [ParseFile] and [Marshal] for YAML and JSON;
//   - [Validate], which checks a document against the official JSON Schema;
//   - [Lint], for semantic checks the schema cannot express;
//   - [Diff], which compares two versions and flags breaking changes.
//
// A typical pipeline:
//
//	data, _ := os.ReadFile("orders.odcs.yaml")
//	if err := odcs.Validate(data); err != nil {
//		log.Fatal(err)
//	}
//	c, _ := odcs.Parse(data)
//	for _, issue := range odcs.Lint(c) {
//		fmt.Println(issue)
//	}
package odcs

// SchemaVersion is the ODCS version of the embedded JSON Schema.
const SchemaVersion = "v3.2.0"

// SupportedAPIVersions lists the apiVersion values accepted by the embedded
// schema, newest first.
var SupportedAPIVersions = []string{"v3.2.0", "v3.1.0", "v3.0.2", "v3.0.1", "v3.0.0", "v2.2.2", "v2.2.1", "v2.2.0"}
