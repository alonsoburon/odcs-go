package odcs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// upstreamExamples returns the example contracts copied from the ODCS
// repository (see testdata/upstream/README.md).
func upstreamExamples(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("testdata/upstream", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Fatalf("expected the upstream examples, found %d files", len(files))
	}
	return files
}

func TestUpstreamExamples(t *testing.T) {
	for _, path := range upstreamExamples(t) {
		t.Run(strings.TrimPrefix(path, "testdata/upstream/"), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(data); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			c, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if err := c.Validate(); err != nil {
				t.Fatalf("DataContract.Validate: %v", err)
			}

			out, err := Marshal(c)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if err := Validate(out); err != nil {
				t.Fatalf("Validate(Marshal): %v\n%s", err, out)
			}
			c2, err := Parse(out)
			if err != nil {
				t.Fatalf("Parse(Marshal): %v", err)
			}
			if !sameContract(t, c, c2) {
				t.Fatalf("round trip changed the contract:\n%s", out)
			}
			out2, err := Marshal(c2)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != string(out2) {
				t.Fatalf("Marshal is not stable:\n%s\n---\n%s", out, out2)
			}

			j, err := MarshalJSON(c)
			if err != nil {
				t.Fatal(err)
			}
			c3, err := Parse(j)
			if err != nil {
				t.Fatalf("Parse(MarshalJSON): %v", err)
			}
			if !sameContract(t, c, c3) {
				t.Fatalf("JSON round trip changed the contract:\n%s", j)
			}
		})
	}
}

// sameContract compares contracts by their canonical JSON encoding, so that
// nil and empty lists (both omitted) count as equal.
func sameContract(t *testing.T, a, b *DataContract) bool {
	t.Helper()
	ja, err := MarshalJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := MarshalJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(ja) == string(jb)
}

func TestParseFields(t *testing.T) {
	c, err := ParseFile("testdata/upstream/all/full-example.odcs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "53581432-6c55-4ba2-a65f-72344a91553a" || c.APIVersion != "v3.2.0" {
		t.Errorf("fundamentals: id=%q apiVersion=%q", c.ID, c.APIVersion)
	}
	if len(c.Schema) != 2 || c.Schema[0].Name != "tbl" {
		t.Fatalf("schema: %+v", c.Schema)
	}
	rel := c.Schema[0].Relationships[0]
	if !rel.From.IsList || !reflect.DeepEqual(rel.To.Values, []string{"receivers.id", "receivers.country_code"}) {
		t.Errorf("composite relationship: %+v %+v", rel.From, rel.To)
	}
	p := c.Schema[0].Properties[1]
	if p.Name != "rcvr_id" || !p.PrimaryKey || p.PrimaryKeyPosition == nil || *p.PrimaryKeyPosition != 1 {
		t.Errorf("property: %+v", p)
	}
	if p.Relationships[0].To.IsList || p.Relationships[0].To.Values[0] != "receivers.id" {
		t.Errorf("property relationship: %+v", p.Relationships[0].To)
	}
	q := c.Schema[0].Quality[0]
	if q.Metric != "rowCount" || q.MustBeGreaterThan == nil || *q.MustBeGreaterThan != "1000000" {
		t.Errorf("quality: %+v", q)
	}
	m := c.Schema[1].Properties[4].Map
	if m == nil || m.Key.LogicalType != "string" || m.Value.LogicalType != "string" {
		t.Errorf("map: %+v", m)
	}
	if c.Team == nil || c.Team.Legacy || len(c.Team.Members) != 3 || c.Team.Name != "my-team" {
		t.Errorf("team: %+v", c.Team)
	}
	if c.Price == nil || *c.Price.PriceAmount != "9.95" {
		t.Errorf("price: %+v", c.Price)
	}
	if c.Servers[0].Port.(interface{ String() string }).String() != "5432" {
		t.Errorf("port: %#v", c.Servers[0].Port)
	}
	if len(c.SLAProperties) != 8 || c.SLAProperties[0].Unit != "d" {
		t.Errorf("sla: %+v", c.SLAProperties)
	}
}

func TestParseJSON(t *testing.T) {
	c, err := Parse([]byte(`{"apiVersion":"v3.2.0","kind":"DataContract","id":"x","version":"1",
		"schema":[{"name":"t","properties":[{"name":"c","logicalType":"array","items":{"logicalType":"object","properties":[{"name":"n"}]}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Schema[0].Properties[0].Items.Properties[0].Name; got != "n" {
		t.Errorf("nested array item property = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, doc, want string
	}{
		{"empty", "", "empty document"},
		{"not a mapping", "- a\n- b\n", "must be a mapping"},
		{"duplicate key", "id: a\nid: b\n", `duplicate key "id"`},
		{"syntax", "id: [a\n", "odcs: yaml"},
		{"wrong type", "schema: 3\n", "cannot unmarshal"},
		{"bad reference", "schema:\n  - name: t\n    relationships:\n      - from: 1\n        to: x\n", "reference must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
	if _, err := ParseFile("testdata/missing.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ParseFile(missing) = %v", err)
	}
}

// TestRoundTripForms checks that alternative syntaxes are written back the
// way they were read.
func TestRoundTripForms(t *testing.T) {
	doc := `apiVersion: v3.0.2
kind: DataContract
id: x
version: 1.0.0
context: Use for revenue reporting only.
schema:
  - name: t
    relationships:
      - from: t.a
        to: u.a
      - from:
          - t.b
        to:
          - u.b
    quality:
      - metric: rowCount
        mustBeGreaterThan: 1000000
      - metric: nullValues
        mustBeBetween:
          - 0.5
          - 1.25
team:
  - username: alice
    role: owner
`
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Context.Shorthand || c.Context.Instructions == "" {
		t.Errorf("context: %+v", c.Context)
	}
	if !c.Team.Legacy || c.Team.Members[0].Username != "alice" {
		t.Errorf("team: %+v", c.Team)
	}
	out, err := Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != doc {
		t.Errorf("round trip mismatch:\n--- got\n%s--- want\n%s", out, doc)
	}
}

func TestNumber(t *testing.T) {
	n := NewNumber(0.25)
	if f, err := n.Float64(); err != nil || f != 0.25 || n.String() != "0.25" {
		t.Errorf("NewNumber(0.25) = %q", n)
	}
	if _, err := Number("abc").MarshalJSON(); err == nil {
		t.Error("invalid number marshalled")
	}
}
