package odcs

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// SchemaJSON is the official ODCS JSON Schema this package validates against.
//
// Source: https://github.com/bitol-io/open-data-contract-standard,
// tag v3.2.0, file schema/odcs-json-schema-v3.2.0.json (Apache-2.0).
//
//go:embed schema/odcs-json-schema-v3.2.0.json
var SchemaJSON []byte

const schemaURL = "https://github.com/bitol-io/open-data-contract-standard/schema/odcs-json-schema-v3.2.0.json"

var compiled = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2019)
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, err
	}
	return c.Compile(schemaURL)
})

// SchemaError is one violation of the JSON Schema.
type SchemaError struct {
	// Path is a JSON Pointer (RFC 6901) to the offending value, e.g.
	// "/schema/0/properties/2/logicalType". The root is "".
	Path string
	// Keyword is the location of the failing keyword in the schema.
	Keyword string
	// Message describes the violation.
	Message string
}

func (e SchemaError) String() string {
	p := e.Path
	if p == "" {
		p = "/"
	}
	return p + ": " + e.Message
}

// ValidationError is returned by [Validate] when the document does not
// conform to the schema. Use errors.As to retrieve it.
type ValidationError struct {
	Errors []SchemaError
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "odcs: document does not conform to ODCS %s:", SchemaVersion)
	for _, se := range e.Errors {
		b.WriteString("\n  ")
		b.WriteString(se.String())
	}
	return b.String()
}

// Validate checks a raw YAML or JSON document against the embedded ODCS
// JSON Schema (draft 2019-09). It returns nil if the document is valid,
// a *ValidationError listing every violation, or another error if the
// document cannot be decoded.
func Validate(data []byte) error {
	doc, err := decodeGeneric(data)
	if err != nil {
		return err
	}
	return validateGeneric(doc)
}

// Validate checks the contract against the ODCS JSON Schema. It is
// equivalent to marshalling the contract and calling the package-level
// [Validate].
func (c *DataContract) Validate() error {
	j, err := json.Marshal(c)
	if err != nil {
		return err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		return err
	}
	return validateGeneric(doc)
}

func validateGeneric(doc any) error {
	sch, err := compiled()
	if err != nil {
		return fmt.Errorf("odcs: compiling embedded schema: %w", err)
	}
	err = sch.Validate(doc)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	return &ValidationError{Errors: flatten(ve)}
}

var printer = message.NewPrinter(language.English)

// flatten keeps the leaf errors, which carry the actual causes, sorted by
// path and without duplicates.
//
// ODCS closes its objects with unevaluatedProperties: false. When a
// sub-schema fails, the properties it covered count as unevaluated and are
// reported again as "not allowed". Those echoes are dropped when the
// property is defined somewhere in ODCS and the same object has a real error;
// properties unknown to ODCS are always reported.
func flatten(root *jsonschema.ValidationError) []SchemaError {
	var leaves, unevaluated []SchemaError
	seen := map[SchemaError]bool{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		se := SchemaError{
			Path:    jsonPointer(e.InstanceLocation),
			Keyword: jsonPointer(e.ErrorKind.KeywordPath()),
			Message: e.ErrorKind.LocalizedString(printer),
		}
		if _, ok := e.ErrorKind.(*kind.FalseSchema); ok && len(e.InstanceLocation) > 0 {
			se.Keyword = "/unevaluatedProperties"
			se.Message = fmt.Sprintf("property %q is not allowed here", e.InstanceLocation[len(e.InstanceLocation)-1])
			unevaluated = append(unevaluated, se)
			return
		}
		if !seen[se] {
			seen[se] = true
			leaves = append(leaves, se)
		}
	}
	walk(root)
	out := leaves
	for _, u := range unevaluated {
		parent := u.Path[:strings.LastIndexByte(u.Path, '/')]
		echo := slices.ContainsFunc(leaves, func(e SchemaError) bool {
			return e.Path == parent || strings.HasPrefix(e.Path, parent+"/")
		})
		echo = echo && knownProperties()[u.Path[len(parent)+1:]]
		if !echo && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// knownProperties is the set of every property name defined by the schema.
var knownProperties = sync.OnceValue(func() map[string]bool {
	var doc any
	_ = json.Unmarshal(SchemaJSON, &doc)
	names := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if props, ok := v["properties"].(map[string]any); ok {
				for k := range props {
					names[k] = true
				}
			}
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(doc)
	return names
})

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func jsonPointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(pointerEscaper.Replace(t))
	}
	return b.String()
}
