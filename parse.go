package odcs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// Parse decodes an ODCS document in YAML or JSON (JSON is valid YAML).
//
// Parse only checks that the document is well formed and that values have
// the expected Go types; use [Validate] to check it against the ODCS JSON
// Schema and [Lint] for semantic checks.
func Parse(data []byte) (*DataContract, error) {
	doc, err := decodeGeneric(data)
	if err != nil {
		return nil, err
	}
	return fromGeneric(doc)
}

// ParseFile reads and parses the file at path.
func ParseFile(path string) (*DataContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Marshal encodes the contract as YAML, with keys in the standard's order
// and two-space indentation.
func Marshal(c *DataContract) ([]byte, error) {
	j, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal(j, &n); err != nil {
		return nil, err
	}
	resetStyle(&n)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalJSON encodes the contract as indented JSON.
func MarshalJSON(c *DataContract) ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func fromGeneric(doc any) (*DataContract, error) {
	if _, ok := doc.(map[string]any); !ok {
		return nil, errors.New("odcs: document must be a mapping")
	}
	j, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(j))
	dec.UseNumber()
	var c DataContract
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("odcs: %w", err)
	}
	return &c, nil
}

// decodeGeneric decodes YAML or JSON into maps, slices and scalars, keeping
// numbers as json.Number so that their literal form is preserved.
func decodeGeneric(data []byte) (any, error) {
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("odcs: %w", err)
	}
	if n.Kind == 0 {
		return nil, errors.New("odcs: empty document")
	}
	return nodeToGeneric(&n)
}

func nodeToGeneric(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, errors.New("odcs: empty document")
		}
		return nodeToGeneric(n.Content[0])
	case yaml.AliasNode:
		return nodeToGeneric(n.Alias)
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("odcs: line %d: mapping keys must be scalars", k.Line)
			}
			if _, dup := m[k.Value]; dup {
				return nil, fmt.Errorf("odcs: line %d: duplicate key %q", k.Line, k.Value)
			}
			val, err := nodeToGeneric(v)
			if err != nil {
				return nil, err
			}
			m[k.Value] = val
		}
		return m, nil
	case yaml.SequenceNode:
		s := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			val, err := nodeToGeneric(c)
			if err != nil {
				return nil, err
			}
			s = append(s, val)
		}
		return s, nil
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!null":
			return nil, nil
		case "!!bool":
			var b bool
			if err := n.Decode(&b); err != nil {
				return nil, err
			}
			return b, nil
		case "!!int":
			var i int64
			if err := n.Decode(&i); err == nil {
				return json.Number(strconv.FormatInt(i, 10)), nil
			}
			var u uint64
			if err := n.Decode(&u); err != nil {
				return nil, fmt.Errorf("odcs: line %d: %w", n.Line, err)
			}
			return json.Number(strconv.FormatUint(u, 10)), nil
		case "!!float":
			var f float64
			if err := n.Decode(&f); err != nil {
				return nil, err
			}
			if _, err := strconv.ParseFloat(n.Value, 64); err == nil {
				return json.Number(n.Value), nil
			}
			if math.IsInf(f, 0) || math.IsNaN(f) {
				return nil, fmt.Errorf("odcs: line %d: %q is not representable in JSON", n.Line, n.Value)
			}
			return json.Number(strconv.FormatFloat(f, 'g', -1, 64)), nil
		default: // !!str, !!timestamp, !!binary and custom tags stay strings.
			return n.Value, nil
		}
	}
	return nil, fmt.Errorf("odcs: line %d: unsupported YAML node", n.Line)
}

// resetStyle turns the flow-style JSON node tree into block-style YAML.
// The encoder still quotes strings whose plain form would change type.
func resetStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		resetStyle(c)
	}
}

// Number is a JSON/YAML number kept in its literal form, so that values such
// as 1000000 or 0.1 are written back exactly as they were read.
type Number string

// NewNumber returns the Number for f.
func NewNumber(f float64) Number {
	return Number(strconv.FormatFloat(f, 'f', -1, 64))
}

// Float64 returns the numeric value of n.
func (n Number) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// String returns the literal form of n.
func (n Number) String() string { return string(n) }

// MarshalJSON writes n as a bare JSON number.
func (n Number) MarshalJSON() ([]byte, error) {
	if _, err := n.Float64(); err != nil {
		return nil, fmt.Errorf("odcs: invalid number %q", string(n))
	}
	return []byte(n), nil
}

// UnmarshalJSON reads a JSON number.
func (n *Number) UnmarshalJSON(b []byte) error {
	var v json.Number
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("odcs: expected a number, got %s", b)
	}
	*n = Number(v)
	return nil
}
