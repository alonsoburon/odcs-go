package odcs_test

import (
	"errors"
	"fmt"

	odcs "github.com/alonsoburon/odcs-go"
)

func Example() {
	doc := []byte(`
apiVersion: v3.2.0
kind: DataContract
id: orders
version: 1.0.0
schema:
  - name: orders
    properties:
      - name: order_id
        logicalType: string
        required: true
      - name: customer_id
        logicalType: string
        relationships:
          - to: customers.id
`)
	if err := odcs.Validate(doc); err != nil {
		fmt.Println(err)
		return
	}
	c, err := odcs.Parse(doc)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c.Schema[0].Properties[0].Name)
	for _, issue := range odcs.Lint(c) {
		fmt.Println(issue)
	}
	// Output:
	// order_id
	// warning: /schema/0/properties/1/relationships/0/to: reference "customers.id" does not resolve: no schema object named "customers"
}

func ExampleValidate() {
	err := odcs.Validate([]byte(`
apiVersion: v3.2.0
kind: DataContract
id: orders
version: 1.0.0
schema:
  - name: orders
    properties:
      - name: amount
        logicalType: decimal
`))
	var ve *odcs.ValidationError
	if errors.As(err, &ve) {
		for _, e := range ve.Errors {
			fmt.Println(e.Path)
		}
	}
	// Output:
	// /schema/0/properties/0/logicalType
}

func ExampleDiff() {
	old, _ := odcs.Parse([]byte(`
apiVersion: v3.2.0
kind: DataContract
id: orders
version: 1.0.0
schema:
  - name: orders
    properties:
      - name: amount
        logicalType: number
      - name: note
        logicalType: string
`))
	new, _ := odcs.Parse([]byte(`
apiVersion: v3.2.0
kind: DataContract
id: orders
version: 1.1.0
schema:
  - name: orders
    properties:
      - name: amount
        logicalType: integer
      - name: channel
        logicalType: string
`))
	changes := odcs.Diff(old, new)
	for _, c := range changes {
		fmt.Println(c)
	}
	fmt.Println("breaking:", odcs.HasBreaking(changes))
	// Output:
	//          changed version: "1.0.0" -> "1.1.0"
	// BREAKING changed schema[orders].properties[amount].logicalType: "number" -> "integer"
	// BREAKING removed schema[orders].properties[note]
	//          added   schema[orders].properties[channel]
	// breaking: true
}
