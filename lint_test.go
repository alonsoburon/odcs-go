package odcs

import (
	"fmt"
	"strings"
	"testing"
)

func TestLint(t *testing.T) {
	type want struct {
		sev  Severity
		path string
		msg  string
	}
	tests := []struct {
		name string
		doc  string
		want []want
	}{
		{name: "clean", doc: minimal + `schema:
  - id: orders_tbl
    name: orders
    properties:
      - id: id_prop
        name: id
      - name: customer_id
        relationships:
          - to: customers.id
          - to: schema/customers_tbl/properties/cust_id
      - name: address
        logicalType: object
        properties:
          - name: city
      - name: lines
        logicalType: array
        items:
          logicalType: object
          properties:
            - name: sku
  - id: customers_tbl
    name: customers
    properties:
      - id: cust_id
        name: id
    relationships:
      - from: [orders.address.city, orders.lines.sku]
        to: [customers.id, external.yaml#/schema/x/properties/y]
slaDefaultElement: orders.id
slaProperties:
  - property: latency
    value: 4
    unit: h
    element: orders.id, orders.customer_id
  - property: availability
    value: 99.9
    unit: "%"
`},
		{
			name: "fundamentals",
			doc:  "apiVersion: v9\nkind: Contract\n",
			want: []want{
				{SeverityError, "/apiVersion", `unknown apiVersion "v9"`},
				{SeverityError, "/kind", "kind must be DataContract"},
				{SeverityError, "/id", "id is missing"},
				{SeverityError, "/version", "version is missing"},
			},
		},
		{
			name: "old apiVersion",
			doc:  strings.Replace(minimal, "v3.2.0", "v2.2.2", 1),
			want: []want{{SeverityWarning, "/apiVersion", "predates ODCS v3"}},
		},
		{
			name: "duplicates",
			doc: minimal + `servers:
  - {server: prod, type: local, path: a, format: csv}
  - {server: prod, type: local, path: b, format: csv}
schema:
  - id: t1
    name: orders
    properties:
      - {name: id}
      - {name: id}
      - name: nested
        properties:
          - {id: p, name: a}
          - {id: p, name: b}
  - id: t1
    name: orders
`,
			want: []want{
				{SeverityError, "/servers/1/server", `duplicate server "prod" (also at /servers/0)`},
				{SeverityError, "/schema/0/properties/1/name", `duplicate property "id" (also at /schema/0/properties/0)`},
				{SeverityError, "/schema/0/properties/2/properties/1/id", `duplicate id "p"`},
				{SeverityError, "/schema/1/name", `duplicate schema object "orders" (also at /schema/0)`},
				{SeverityError, "/schema/1/id", `duplicate id "t1" (also at /schema/0)`},
			},
		},
		{
			name: "dangling references",
			doc: minimal + `schema:
  - id: orders_tbl
    name: orders
    properties:
      - name: customer_id
        relationships:
          - to: customers.id
          - to: orders.missing
          - to: schema/orders_tbl/properties/nope
    relationships:
      - from: [orders.customer_id, orders.customer_id]
        to: orders.customer_id
slaDefaultElement: nowhere.col
slaProperties:
  - property: latency
    value: 1
    unit: d
    element: orders.customer_id,orders.ghost
`,
			want: []want{
				{SeverityError, "/schema/0/relationships/0", "2 source(s) but 1 target(s)"},
				{SeverityWarning, "/schema/0/properties/0/relationships/0/to", `"customers.id" does not resolve: no schema object named "customers"`},
				{SeverityWarning, "/schema/0/properties/0/relationships/1/to", `"orders" has no property "missing"`},
				{SeverityWarning, "/schema/0/properties/0/relationships/2/to", `no property with id "nope"`},
				{SeverityWarning, "/slaDefaultElement", `"nowhere.col" does not resolve`},
				{SeverityWarning, "/slaProperties/0/element", `"orders.ghost" does not resolve`},
			},
		},
		{
			name: "units",
			doc: minimal + `schema:
  - name: t
    quality:
      - metric: rowCount
        mustBeGreaterThan: 1
        unit: bananas
slaProperties:
  - property: latency
    value: 4
  - property: retention
    value: 3
    unit: fortnights
  - property: frequency
    value: 1
    unit: Days
  - property: throughput
    value: 10
    unit: widgets
`,
			want: []want{
				{SeverityInfo, "/schema/0/quality/0/unit", `unusual quality unit "bananas"`},
				{SeverityWarning, "/slaProperties/0/unit", `"latency" needs a unit`},
				{SeverityWarning, "/slaProperties/1/unit", `invalid time unit "fortnights"`},
				{SeverityInfo, "/slaProperties/3/unit", `unrecognised unit "widgets"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Parse([]byte(tt.doc))
			if err != nil {
				t.Fatal(err)
			}
			got := Lint(c)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d issues, want %d:\n%s", len(got), len(tt.want), joinIssues(got))
			}
			for i, w := range tt.want {
				g := got[i]
				if g.Severity != w.sev || g.Path != w.path || !strings.Contains(g.Message, w.msg) {
					t.Errorf("issue %d = %s\nwant %s: %s: ...%s...", i, g, w.sev, w.path, w.msg)
				}
			}
			if HasErrors(got) != (len(tt.want) > 0 && tt.want[0].sev == SeverityError) {
				t.Errorf("HasErrors = %v", HasErrors(got))
			}
		})
	}
}

func joinIssues(issues []Issue) string {
	var b strings.Builder
	for _, i := range issues {
		fmt.Fprintln(&b, i)
	}
	return b.String()
}

func TestLintUpstreamExamples(t *testing.T) {
	// The official examples are valid; Lint must not report errors on them.
	for _, path := range upstreamExamples(t) {
		c, err := ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if issues := Lint(c); HasErrors(issues) {
			t.Errorf("%s:\n%s", path, joinIssues(issues))
		}
	}
}

func TestSeverity(t *testing.T) {
	for s, want := range map[Severity]string{SeverityInfo: "info", SeverityWarning: "warning", SeverityError: "error", 7: "Severity(7)"} {
		if s.String() != want {
			t.Errorf("%d.String() = %q", s, s.String())
		}
	}
	if got := (Issue{SeverityError, "", "x"}).String(); got != "error: /: x" {
		t.Errorf("Issue.String() = %q", got)
	}
}
