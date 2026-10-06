package odcs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DataContract is the root object of an ODCS document.
//
// Field order follows the order used in the standard's documentation so that
// [Marshal] produces familiar-looking YAML.
type DataContract struct {
	// Fundamentals.
	APIVersion  string       `json:"apiVersion"`
	Kind        string       `json:"kind"`
	ID          string       `json:"id"`
	Name        string       `json:"name,omitempty"`
	Version     string       `json:"version"`
	Status      string       `json:"status,omitempty"`
	Tenant      string       `json:"tenant,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Domain      string       `json:"domain,omitempty"`
	DataProduct string       `json:"dataProduct,omitempty"` // Deprecated since ODCS v3.1.0.
	Description *Description `json:"description,omitempty"`
	Context     *Context     `json:"context,omitempty"`

	Servers []Server       `json:"servers,omitempty"`
	Schema  []SchemaObject `json:"schema,omitempty"`

	Support []SupportItem `json:"support,omitempty"`
	Price   *Price        `json:"price,omitempty"`
	Team    *Team         `json:"team,omitempty"`
	Roles   []Role        `json:"roles,omitempty"`

	SLADefaultElement string        `json:"slaDefaultElement,omitempty"` // Deprecated by the standard; prefer SLAProperty.Element.
	SLAProperties     []SLAProperty `json:"slaProperties,omitempty"`

	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
	ContractCreatedTs        string                    `json:"contractCreatedTs,omitempty"`
}

// Description is the high-level description of the dataset.
type Description struct {
	Usage                    string                    `json:"usage,omitempty"`
	Purpose                  string                    `json:"purpose,omitempty"`
	Limitations              string                    `json:"limitations,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}

// Server describes where a dataset is physically hosted.
//
// ODCS defines one sub-schema per server type; this struct is the union of
// all their fields. Which fields are required depends on Type and is enforced
// by [Validate].
type Server struct {
	ID          string `json:"id,omitempty"`
	Server      string `json:"server"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Environment string `json:"environment,omitempty"`

	Host        string `json:"host,omitempty"`
	Port        any    `json:"port,omitempty"` // integer, or string such as "${DB_PORT}"
	Database    string `json:"database,omitempty"`
	Schema      string `json:"schema,omitempty"`
	Catalog     string `json:"catalog,omitempty"`
	CatalogURL  string `json:"catalogUrl,omitempty"`
	Account     string `json:"account,omitempty"`
	Warehouse   string `json:"warehouse,omitempty"`
	Project     string `json:"project,omitempty"`
	Dataset     string `json:"dataset,omitempty"`
	Location    string `json:"location,omitempty"`
	Path        string `json:"path,omitempty"`
	Format      string `json:"format,omitempty"`
	Delimiter   string `json:"delimiter,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	EndpointURL string `json:"endpointUrl,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Region      string `json:"region,omitempty"`
	RegionName  string `json:"regionName,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	StagingDir  string `json:"stagingDir,omitempty"`
	Stream      string `json:"stream,omitempty"`
	Workgroup   string `json:"workgroup,omitempty"`

	Roles            []Role           `json:"roles,omitempty"`
	CustomProperties []CustomProperty `json:"customProperties,omitempty"`
}

// SchemaObject is a top-level element of the schema, typically a table,
// view, topic or file.
type SchemaObject struct {
	ID                         string                    `json:"id,omitempty"`
	Name                       string                    `json:"name"`
	PhysicalName               string                    `json:"physicalName,omitempty"`
	PhysicalType               string                    `json:"physicalType,omitempty"`
	LogicalType                string                    `json:"logicalType,omitempty"`
	BusinessName               string                    `json:"businessName,omitempty"`
	Description                string                    `json:"description,omitempty"`
	DataGranularityDescription string                    `json:"dataGranularityDescription,omitempty"`
	Deprecated                 bool                      `json:"deprecated,omitempty"`
	Synonyms                   []Synonym                 `json:"synonyms,omitempty"`
	AuthoritativeDefinitions   []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	Tags                       []string                  `json:"tags,omitempty"`
	Context                    *Context                  `json:"context,omitempty"`
	Relationships              []Relationship            `json:"relationships,omitempty"`
	Properties                 []SchemaProperty          `json:"properties,omitempty"`
	Quality                    []Quality                 `json:"quality,omitempty"`
	CustomProperties           []CustomProperty          `json:"customProperties,omitempty"`
}

// SchemaProperty is a column or field. Properties nest: objects carry
// Properties, arrays carry Items and maps carry Map.
//
// The same type is used for array items and map keys/values, where Name is
// optional.
type SchemaProperty struct {
	ID                       string                    `json:"id,omitempty"`
	Name                     string                    `json:"name,omitempty"`
	PhysicalName             string                    `json:"physicalName,omitempty"`
	PhysicalType             string                    `json:"physicalType,omitempty"`
	LogicalType              string                    `json:"logicalType,omitempty"`
	LogicalTypeOptions       map[string]any            `json:"logicalTypeOptions,omitempty"`
	BusinessName             string                    `json:"businessName,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Deprecated               bool                      `json:"deprecated,omitempty"`
	PrimaryKey               bool                      `json:"primaryKey,omitempty"`
	PrimaryKeyPosition       *int                      `json:"primaryKeyPosition,omitempty"`
	Required                 bool                      `json:"required,omitempty"`
	Unique                   bool                      `json:"unique,omitempty"`
	SemanticType             string                    `json:"semanticType,omitempty"`
	Partitioned              bool                      `json:"partitioned,omitempty"`
	PartitionKeyPosition     *int                      `json:"partitionKeyPosition,omitempty"`
	Classification           string                    `json:"classification,omitempty"`
	EncryptedName            string                    `json:"encryptedName,omitempty"`
	CriticalDataElement      bool                      `json:"criticalDataElement,omitempty"`
	TransformSourceObjects   []string                  `json:"transformSourceObjects,omitempty"`
	TransformLogic           string                    `json:"transformLogic,omitempty"`
	TransformDescription     string                    `json:"transformDescription,omitempty"`
	Examples                 []any                     `json:"examples,omitempty"`
	Enum                     []EnumValue               `json:"enum,omitempty"`
	Synonyms                 []Synonym                 `json:"synonyms,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	Relationships            []Relationship            `json:"relationships,omitempty"`
	Properties               []SchemaProperty          `json:"properties,omitempty"` // logicalType: object
	Items                    *SchemaProperty           `json:"items,omitempty"`      // logicalType: array
	Map                      *MapType                  `json:"map,omitempty"`        // logicalType: map
	Quality                  []Quality                 `json:"quality,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}

// MapType defines the key and value of a property with logicalType map.
type MapType struct {
	Key   *SchemaProperty `json:"key"`
	Value *SchemaProperty `json:"value"`
}

// EnumValue is one allowed value of a property.
type EnumValue struct {
	Value                    any                       `json:"value"`
	Label                    string                    `json:"label,omitempty"`
	ID                       string                    `json:"id,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}

// Synonym is an alternative name for a schema element.
type Synonym struct {
	ID               string           `json:"id,omitempty"`
	Synonym          string           `json:"synonym"`
	Description      string           `json:"description,omitempty"`
	Locale           string           `json:"locale,omitempty"`
	Source           string           `json:"source,omitempty"`
	Status           string           `json:"status,omitempty"`
	CustomProperties []CustomProperty `json:"customProperties,omitempty"`
}

// Relationship is a foreign-key style link between properties. At property
// level From is omitted (it is the property itself).
type Relationship struct {
	ID               string           `json:"id,omitempty"`
	Type             string           `json:"type,omitempty"`
	From             *References      `json:"from,omitempty"`
	To               *References      `json:"to,omitempty"`
	CustomProperties []CustomProperty `json:"customProperties,omitempty"`
}

// References holds one reference or a list of references (composite keys),
// in shorthand (table.column) or fully qualified (schema/id/properties/id)
// notation. IsList records whether the document used the list form so that
// it can be written back unchanged.
type References struct {
	Values []string
	IsList bool
}

// UnmarshalJSON accepts a string or an array of strings.
func (r *References) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		r.IsList = true
		return json.Unmarshal(b, &r.Values)
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("reference must be a string or a list of strings: %w", err)
	}
	r.Values, r.IsList = []string{s}, false
	return nil
}

// MarshalJSON writes a string or an array, mirroring the input form.
func (r References) MarshalJSON() ([]byte, error) {
	if !r.IsList && len(r.Values) == 1 {
		return json.Marshal(r.Values[0])
	}
	if r.Values == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(r.Values)
}

// String joins the references with commas.
func (r *References) String() string {
	if r == nil {
		return ""
	}
	return strings.Join(r.Values, ",")
}

// Quality is a data quality rule. Type selects the flavour: "text",
// "library" (Metric), "sql" (Query) or "custom" (Engine + Implementation).
// Library and SQL rules use one of the Must* comparison operators.
type Quality struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Dimension   string `json:"dimension,omitempty"`
	Method      string `json:"method,omitempty"`

	Metric         string         `json:"metric,omitempty"`
	Rule           string         `json:"rule,omitempty"` // Deprecated: use Metric.
	Arguments      map[string]any `json:"arguments,omitempty"`
	Query          string         `json:"query,omitempty"`
	Engine         string         `json:"engine,omitempty"`
	Implementation any            `json:"implementation,omitempty"`

	MustBe                 any      `json:"mustBe,omitempty"`
	MustNotBe              any      `json:"mustNotBe,omitempty"`
	MustBeGreaterThan      *Number  `json:"mustBeGreaterThan,omitempty"`
	MustBeGreaterOrEqualTo *Number  `json:"mustBeGreaterOrEqualTo,omitempty"`
	MustBeLessThan         *Number  `json:"mustBeLessThan,omitempty"`
	MustBeLessOrEqualTo    *Number  `json:"mustBeLessOrEqualTo,omitempty"`
	MustBeBetween          []Number `json:"mustBeBetween,omitempty"`
	MustNotBeBetween       []Number `json:"mustNotBeBetween,omitempty"`

	Unit                     string                    `json:"unit,omitempty"`
	Severity                 string                    `json:"severity,omitempty"`
	BusinessImpact           string                    `json:"businessImpact,omitempty"`
	Schedule                 string                    `json:"schedule,omitempty"`
	Scheduler                string                    `json:"scheduler,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}

// SupportItem is a support channel.
type SupportItem struct {
	ID               string           `json:"id,omitempty"`
	Channel          string           `json:"channel"`
	URL              string           `json:"url,omitempty"`
	Description      string           `json:"description,omitempty"`
	Tool             string           `json:"tool,omitempty"`
	Scope            string           `json:"scope,omitempty"`
	InvitationURL    string           `json:"invitationUrl,omitempty"`
	CustomProperties []CustomProperty `json:"customProperties,omitempty"`
}

// Price is the pricing of the data.
type Price struct {
	ID            string  `json:"id,omitempty"`
	PriceAmount   *Number `json:"priceAmount,omitempty"`
	PriceCurrency string  `json:"priceCurrency,omitempty"`
	PriceUnit     string  `json:"priceUnit,omitempty"`
}

// Team is the team owning the contract.
//
// Before ODCS v3.1.0 team was a plain list of members. That form is still
// accepted; it is decoded into Members with Legacy set, and written back in
// the same form.
type Team struct {
	ID                       string                    `json:"id,omitempty"`
	Name                     string                    `json:"name,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Members                  []TeamMember              `json:"members,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`

	// Legacy is true when the team was given as a list of members.
	Legacy bool `json:"-"`
}

type teamAlias Team

// UnmarshalJSON accepts the team object or the deprecated member list.
func (t *Team) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		*t = Team{Legacy: true}
		return json.Unmarshal(b, &t.Members)
	}
	var a teamAlias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*t = Team(a)
	return nil
}

// MarshalJSON writes the form the team was read in.
func (t Team) MarshalJSON() ([]byte, error) {
	if t.Legacy {
		if t.Members == nil {
			return []byte("[]"), nil
		}
		return json.Marshal(t.Members)
	}
	return json.Marshal(teamAlias(t))
}

// TeamMember is a member of the owning team.
type TeamMember struct {
	ID                       string                    `json:"id,omitempty"`
	Username                 string                    `json:"username"`
	Name                     string                    `json:"name,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Role                     string                    `json:"role,omitempty"`
	DateIn                   string                    `json:"dateIn,omitempty"`
	DateOut                  string                    `json:"dateOut,omitempty"`
	ReplacedByUsername       string                    `json:"replacedByUsername,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
}

// Role is an IAM role that grants access to the data.
type Role struct {
	ID                   string           `json:"id,omitempty"`
	Role                 string           `json:"role"`
	Description          string           `json:"description,omitempty"`
	Access               string           `json:"access,omitempty"`
	FirstLevelApprovers  string           `json:"firstLevelApprovers,omitempty"`
	SecondLevelApprovers string           `json:"secondLevelApprovers,omitempty"`
	CustomProperties     []CustomProperty `json:"customProperties,omitempty"`
}

// SLAProperty is one service-level agreement entry, such as latency or
// retention.
type SLAProperty struct {
	ID                       string                    `json:"id,omitempty"`
	Property                 string                    `json:"property"`
	Value                    any                       `json:"value"`
	ValueExt                 any                       `json:"valueExt,omitempty"`
	Unit                     string                    `json:"unit,omitempty"`
	Element                  string                    `json:"element,omitempty"`
	Driver                   string                    `json:"driver,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Scheduler                string                    `json:"scheduler,omitempty"`
	Schedule                 string                    `json:"schedule,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
}

// AuthoritativeDefinition links to an external source of truth.
type AuthoritativeDefinition struct {
	ID          string `json:"id,omitempty"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// CustomProperty is the standard's extension point: a free key/value pair.
// Value may be any YAML/JSON value.
type CustomProperty struct {
	ID          string `json:"id,omitempty"`
	Property    string `json:"property"`
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
	Vendor      string `json:"vendor,omitempty"`
}

// Context carries guidance for AI agents and semantic tools (RFC-0038).
// The shorthand string form is decoded into Instructions with Shorthand set.
type Context struct {
	Instructions       string              `json:"instructions,omitempty"`
	VerifiedStatements []VerifiedStatement `json:"verifiedStatements,omitempty"`
	Constraints        []ContextConstraint `json:"constraints,omitempty"`

	// Shorthand is true when the context was a plain string.
	Shorthand bool `json:"-"`
}

type contextAlias Context

// UnmarshalJSON accepts a string or an object.
func (c *Context) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		*c = Context{Shorthand: true}
		return json.Unmarshal(b, &c.Instructions)
	}
	var a contextAlias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*c = Context(a)
	return nil
}

// MarshalJSON writes the form the context was read in.
func (c Context) MarshalJSON() ([]byte, error) {
	if c.Shorthand && len(c.VerifiedStatements) == 0 && len(c.Constraints) == 0 {
		return json.Marshal(c.Instructions)
	}
	return json.Marshal(contextAlias(c))
}

// VerifiedStatement is a canonical question, optionally with its answer.
type VerifiedStatement struct {
	ID                       string                    `json:"id,omitempty"`
	Question                 string                    `json:"question"`
	Answer                   string                    `json:"answer,omitempty"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}

// ContextConstraint is negative guidance: what agents must not do.
type ContextConstraint struct {
	ID                       string                    `json:"id,omitempty"`
	Constraint               string                    `json:"constraint"`
	AuthoritativeDefinitions []AuthoritativeDefinition `json:"authoritativeDefinitions,omitempty"`
	Tags                     []string                  `json:"tags,omitempty"`
	CustomProperties         []CustomProperty          `json:"customProperties,omitempty"`
}
