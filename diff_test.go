package odcs

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s mismatch (run go test -update to accept):\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func TestDiffGolden(t *testing.T) {
	old, err := ParseFile("testdata/diff/old.odcs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	new, err := ParseFile("testdata/diff/new.odcs.yaml")
	if err != nil {
		t.Fatal(err)
	}
	changes := Diff(old, new)
	if !HasBreaking(changes) {
		t.Error("expected breaking changes")
	}

	var text strings.Builder
	for _, c := range changes {
		text.WriteString(c.String())
		text.WriteByte('\n')
	}
	golden(t, "testdata/diff/diff.golden", []byte(text.String()))

	j, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/diff/diff.golden.json", append(j, '\n'))

	if got := Diff(old, old); len(got) != 0 {
		t.Errorf("Diff(old, old) = %v", got)
	}
}

func TestDiffUpstreamIdentity(t *testing.T) {
	for _, path := range upstreamExamples(t) {
		a, err := ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := ParseFile(path)
		if got := Diff(a, b); len(got) != 0 {
			t.Errorf("%s: Diff with itself = %v", path, got)
		}
	}
}

// TestDiffRules checks one rule at a time. Each case is a pair of schema
// fragments placed under a property named "c" of a table named "t".
func TestDiffRules(t *testing.T) {
	tests := []struct {
		name     string
		old, new string
		path     string
		kind     ChangeKind
		breaking bool
	}{
		{"type changed", "logicalType: string", "logicalType: integer", "schema[t].properties[c].logicalType", Changed, true},
		{"physical type changed", "physicalType: int", "physicalType: bigint", "schema[t].properties[c].physicalType", Changed, true},
		{"made required", "required: false", "required: true", "schema[t].properties[c].required", Changed, true},
		{"made optional", "required: true", "required: false", "schema[t].properties[c].required", Changed, false},
		{"made unique", "unique: false", "unique: true", "schema[t].properties[c].unique", Changed, true},
		{"primary key dropped", "primaryKey: true", "primaryKey: false", "schema[t].properties[c].primaryKey", Changed, true},
		{"description", "description: a", "description: b", "schema[t].properties[c].description", Changed, false},
		{"classification", "classification: public", "classification: restricted", "schema[t].properties[c].classification", Changed, false},
		{"minLength raised", "{logicalType: string, logicalTypeOptions: {minLength: 1}}", "{logicalType: string, logicalTypeOptions: {minLength: 2}}", "schema[t].properties[c].logicalTypeOptions.minLength", Changed, true},
		{"minLength lowered", "{logicalType: string, logicalTypeOptions: {minLength: 2}}", "{logicalType: string, logicalTypeOptions: {minLength: 1}}", "schema[t].properties[c].logicalTypeOptions.minLength", Changed, false},
		{"pattern added", "logicalType: string", "{logicalType: string, logicalTypeOptions: {pattern: '^a'}}", "schema[t].properties[c].logicalTypeOptions.pattern", Added, true},
		{"pattern removed", "{logicalType: string, logicalTypeOptions: {pattern: '^a'}}", "logicalType: string", "schema[t].properties[c].logicalTypeOptions.pattern", Removed, false},
		{"format changed", "{logicalType: integer, logicalTypeOptions: {format: i32}}", "{logicalType: integer, logicalTypeOptions: {format: i64}}", "schema[t].properties[c].logicalTypeOptions.format", Changed, true},
		{"date max earlier", "{logicalType: date, logicalTypeOptions: {maximum: '2030-01-01'}}", "{logicalType: date, logicalTypeOptions: {maximum: '2029-01-01'}}", "schema[t].properties[c].logicalTypeOptions.maximum", Changed, true},
		{"object required grows", "{logicalType: object, logicalTypeOptions: {required: [a]}}", "{logicalType: object, logicalTypeOptions: {required: [a, b]}}", "schema[t].properties[c].logicalTypeOptions.required", Changed, true},
		{"object required shrinks", "{logicalType: object, logicalTypeOptions: {required: [a, b]}}", "{logicalType: object, logicalTypeOptions: {required: [a]}}", "schema[t].properties[c].logicalTypeOptions.required", Changed, false},
		{"enum introduced", "logicalType: string", "enum: [{value: a}]", "schema[t].properties[c].enum", Added, true},
		{"enum dropped", "enum: [{value: a}]", "logicalType: string", "schema[t].properties[c].enum", Removed, false},
		{"map value type", "{logicalType: map, map: {key: {logicalType: string}, value: {logicalType: string}}}", "{logicalType: map, map: {key: {logicalType: string}, value: {logicalType: integer}}}", "schema[t].properties[c].map.value.logicalType", Changed, true},
		{"items removed", "{logicalType: array, items: {logicalType: string}}", "logicalType: array", "schema[t].properties[c].items", Removed, true},
		{"quality loosened", "quality: [{metric: rowCount, mustBeGreaterThan: 10}]", "quality: [{metric: rowCount, mustBeGreaterThan: 5}]", "schema[t].properties[c].quality[rowCount].mustBeGreaterThan", Changed, false},
		{"quality upper tightened", "quality: [{metric: nullValues, mustBeLessThan: 10}]", "quality: [{metric: nullValues, mustBeLessThan: 5}]", "schema[t].properties[c].quality[nullValues].mustBeLessThan", Changed, true},
		{"quality operator swapped", "quality: [{metric: nullValues, mustBeLessThan: 10}]", "quality: [{metric: nullValues, mustBe: 0}]", "schema[t].properties[c].quality[nullValues].mustBe", Added, true},
		{"not-between widened", "quality: [{metric: invalidValues, mustNotBeBetween: [1, 5]}]", "quality: [{metric: invalidValues, mustNotBeBetween: [0, 5]}]", "schema[t].properties[c].quality[invalidValues].mustNotBeBetween", Changed, true},
		{"quality removed", "quality: [{metric: rowCount, mustBeGreaterThan: 10}]", "description: x", "schema[t].properties[c].quality[rowCount]", Removed, false},
		{"text quality added", "description: x", "quality: [{type: text, description: fresh}]", "schema[t].properties[c].quality[text]", Added, false},
		{"sql query changed", "quality: [{id: q, type: sql, query: SELECT 1, mustBe: 1}]", "quality: [{id: q, type: sql, query: SELECT 2, mustBe: 1}]", "schema[t].properties[c].quality[q].query", Changed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := parseFragment(t, tt.old)
			new := parseFragment(t, tt.new)
			for _, c := range Diff(old, new) {
				if c.Path == tt.path {
					if c.Kind != tt.kind || c.Breaking != tt.breaking {
						t.Fatalf("got %s, want kind=%s breaking=%v", c, tt.kind, tt.breaking)
					}
					return
				}
			}
			t.Fatalf("no change at %s; got %v", tt.path, Diff(old, new))
		})
	}
}

func parseFragment(t *testing.T, prop string) *DataContract {
	t.Helper()
	doc := minimal + "schema:\n  - name: t\n    properties:\n      - name: c\n"
	if strings.HasPrefix(prop, "{") {
		doc = minimal + "schema:\n  - name: t\n    properties:\n      - " + "{name: c, " + prop[1:] + "\n"
	} else {
		doc += "        " + prop + "\n"
	}
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("%v\n%s", err, doc)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("fixture is invalid: %v\n%s", err, doc)
	}
	return c
}

func TestDiffTopLevel(t *testing.T) {
	old := parseFragment(t, "logicalType: string")
	new := parseFragment(t, "logicalType: string")
	new.ID = "other"
	new.Version = "2.0.0"
	new.Schema = append(new.Schema, SchemaObject{Name: "u"})
	new.Price = &Price{PriceAmount: ptr(NewNumber(1.5)), PriceCurrency: "USD"}
	got := Diff(old, new)
	want := []string{
		`BREAKING changed id: "orders" -> "other"`,
		`         changed version: "1.0.0" -> "2.0.0"`,
		`         added   schema[u]`,
		`         changed price.priceAmount: <none> -> 1.5`,
		`         changed price.priceCurrency: "" -> "USD"`,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("change %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func ptr[T any](v T) *T { return &v }
