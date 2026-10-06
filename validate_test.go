package odcs

import (
	"errors"
	"strings"
	"testing"
)

const minimal = `apiVersion: v3.2.0
kind: DataContract
id: orders
version: 1.0.0
`

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		wantPath string // "" with wantMsg "" means valid
		wantMsg  string
	}{
		{name: "minimal", doc: minimal},
		{name: "json", doc: `{"apiVersion":"v3.2.0","kind":"DataContract","id":"x","version":"1"}`},
		{
			name:    "missing id",
			doc:     "apiVersion: v3.2.0\nkind: DataContract\nversion: 1.0.0\n",
			wantMsg: "missing property 'id'",
		},
		{
			name:     "unknown apiVersion",
			doc:      strings.Replace(minimal, "v3.2.0", "v9.0.0", 1),
			wantPath: "/apiVersion",
			wantMsg:  "must be one of",
		},
		{
			name:     "unknown top-level field",
			doc:      minimal + "owner: me\n",
			wantPath: "",
			wantMsg:  "owner",
		},
		{
			name:     "bad logicalType",
			doc:      minimal + "schema:\n  - name: t\n    properties:\n      - name: c\n        logicalType: text\n",
			wantPath: "/schema/0/properties/0/logicalType",
			wantMsg:  "must be one of",
		},
		{
			name:     "unknown property field",
			doc:      minimal + "schema:\n  - name: t\n    properties:\n      - name: c\n        nullable: true\n",
			wantPath: "/schema/0/properties/0/nullable",
			wantMsg:  `property "nullable" is not allowed`,
		},
		{
			name:     "property without name",
			doc:      minimal + "schema:\n  - name: t\n    properties:\n      - logicalType: string\n",
			wantPath: "/schema/0/properties/0",
			wantMsg:  "missing property 'name'",
		},
		{
			name:     "unknown field next to another error",
			doc:      minimal + "schema:\n  - name: t\n    properties:\n      - name: c\n        logicalType: text\n        nullable: true\n",
			wantPath: "/schema/0/properties/0/nullable",
			wantMsg:  "not allowed",
		},
		{
			name:     "server missing type-specific field",
			doc:      minimal + "servers:\n  - server: k\n    type: kafka\n",
			wantPath: "/servers/0",
			wantMsg:  "missing property 'host'",
		},
		{
			name:     "from at property level",
			doc:      minimal + "schema:\n  - name: t\n    properties:\n      - name: c\n        relationships:\n          - from: t.c\n            to: u.c\n",
			wantPath: "/schema/0/properties/0/relationships/0",
		},
		{
			name:     "string where number expected",
			doc:      minimal + "price:\n  priceAmount: cheap\n",
			wantPath: "/price/priceAmount",
			wantMsg:  "number",
		},
		{
			name:     "invalid stable id",
			doc:      minimal + "schema:\n  - name: t\n    id: has space\n",
			wantPath: "/schema/0/id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate([]byte(tt.doc))
			valid := tt.wantPath == "" && tt.wantMsg == ""
			if valid {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			for _, e := range ve.Errors {
				if e.Path == tt.wantPath && strings.Contains(e.Message, tt.wantMsg) {
					return
				}
			}
			t.Fatalf("no error at %q containing %q; got:\n%v", tt.wantPath, tt.wantMsg, err)
		})
	}
}

func TestValidateDecodeError(t *testing.T) {
	err := Validate([]byte("id: [unterminated\n"))
	var ve *ValidationError
	if err == nil || errors.As(err, &ve) {
		t.Fatalf("want a decode error, got %v", err)
	}
}

func TestValidationErrorString(t *testing.T) {
	err := Validate([]byte("kind: DataContract\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "odcs: document does not conform to ODCS v3.2.0:") || !strings.Contains(msg, "/: missing properties") {
		t.Errorf("unexpected message:\n%s", msg)
	}
}

func TestContractValidate(t *testing.T) {
	c, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Schema = append(c.Schema, SchemaObject{Name: "t", Properties: []SchemaProperty{{Name: "c", LogicalType: "decimal"}}})
	var ve *ValidationError
	if err := c.Validate(); !errors.As(err, &ve) || ve.Errors[0].Path != "/schema/0/properties/0/logicalType" {
		t.Fatalf("got %v", err)
	}
}

func TestSchemaEmbedded(t *testing.T) {
	if !strings.Contains(string(SchemaJSON), `"default": "v3.2.0"`) {
		t.Error("embedded schema is not ODCS v3.2.0")
	}
}
