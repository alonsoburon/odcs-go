package odcs

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// ChangeKind classifies a [Change].
type ChangeKind string

// Change kinds.
const (
	Added   ChangeKind = "added"
	Removed ChangeKind = "removed"
	Changed ChangeKind = "changed"
)

// Change is one difference between two versions of a contract.
type Change struct {
	Kind ChangeKind `json:"kind"`
	// Path locates the element by name, e.g.
	// "schema[orders].properties[amount].logicalType".
	Path string `json:"path"`
	Old  any    `json:"old,omitempty"`
	New  any    `json:"new,omitempty"`
	// Breaking is true when the change can break consumers or make data
	// that satisfied the old contract fail the new one.
	Breaking bool `json:"breaking"`
	// Note explains the change when the values alone do not.
	Note string `json:"note,omitempty"`
}

func (c Change) String() string {
	var b strings.Builder
	if c.Breaking {
		b.WriteString("BREAKING ")
	} else {
		b.WriteString("         ")
	}
	fmt.Fprintf(&b, "%-7s %s", c.Kind, c.Path)
	if c.Kind == Changed {
		fmt.Fprintf(&b, ": %s -> %s", show(c.Old), show(c.New))
	}
	if c.Note != "" {
		fmt.Fprintf(&b, " (%s)", c.Note)
	}
	return b.String()
}

func show(v any) string {
	switch v := v.(type) {
	case nil:
		return "<none>"
	case string:
		return strconv.Quote(v)
	case Number:
		return string(v)
	case *Number:
		if v == nil {
			return "<none>"
		}
		return string(*v)
	}
	return fmt.Sprint(v)
}

// HasBreaking reports whether any change is breaking.
func HasBreaking(changes []Change) bool {
	return slices.ContainsFunc(changes, func(c Change) bool { return c.Breaking })
}

// Diff compares two versions of a contract and lists the changes from old
// to new, in document order.
//
// A change is breaking when consumers relying on the old contract can be
// affected, or when data that satisfied the old contract may not satisfy the
// new one. Breaking:
//
//   - removing a schema object, property, server or role;
//   - adding a required property;
//   - changing logicalType, physicalType or physicalName;
//   - making a property required or unique, or changing its primary key;
//   - tightening logicalTypeOptions (e.g. lower maxLength, new pattern);
//   - removing enum values;
//   - changing a server's type or location fields (host, database, …);
//   - adding a quality rule or tightening its threshold;
//   - changing the contract id.
//
// Additions of optional elements, documentation changes, loosened
// constraints, removed quality rules and SLA changes are reported as
// non-breaking.
func Diff(old, new *DataContract) []Change {
	d := &differ{}
	d.fundamentals(old, new)
	d.servers(old.Servers, new.Servers)
	d.schema(old.Schema, new.Schema)
	d.sla(old.SLAProperties, new.SLAProperties)
	d.roles(old.Roles, new.Roles)
	d.price(old.Price, new.Price)
	return d.changes
}

type differ struct {
	changes []Change
}

func (d *differ) add(c Change) { d.changes = append(d.changes, c) }

// value records a change of a single value, if any.
func (d *differ) value(path string, old, new any, breaking bool) {
	if reflect.DeepEqual(old, new) {
		return
	}
	d.add(Change{Kind: Changed, Path: path, Old: old, New: new, Breaking: breaking})
}

func (d *differ) str(path, old, new string, breaking bool) {
	if old != new {
		d.add(Change{Kind: Changed, Path: path, Old: old, New: new, Breaking: breaking})
	}
}

func (d *differ) fundamentals(o, n *DataContract) {
	d.str("id", o.ID, n.ID, true)
	d.str("apiVersion", o.APIVersion, n.APIVersion, false)
	d.str("kind", o.Kind, n.Kind, false)
	d.str("name", o.Name, n.Name, false)
	d.str("version", o.Version, n.Version, false)
	d.str("status", o.Status, n.Status, false)
	d.str("tenant", o.Tenant, n.Tenant, false)
	d.str("domain", o.Domain, n.Domain, false)
	d.str("dataProduct", o.DataProduct, n.DataProduct, false)
	var od, nd Description
	if o.Description != nil {
		od = *o.Description
	}
	if n.Description != nil {
		nd = *n.Description
	}
	d.str("description.usage", od.Usage, nd.Usage, false)
	d.str("description.purpose", od.Purpose, nd.Purpose, false)
	d.str("description.limitations", od.Limitations, nd.Limitations, false)
}

// keyed pairs up two lists by key and calls the callbacks in document
// order: removed and changed elements in old order, then added ones.
func keyed[T any](old, new []T, key func(int, T) string, removed, added func(string, T), both func(string, T, T)) {
	oldKeys := uniqueKeys(old, key)
	newKeys := uniqueKeys(new, key)
	newIdx := make(map[string]int, len(new))
	for i, k := range newKeys {
		newIdx[k] = i
	}
	oldSet := make(map[string]bool, len(old))
	for i, k := range oldKeys {
		oldSet[k] = true
		if j, ok := newIdx[k]; ok {
			both(k, old[i], new[j])
		} else {
			removed(k, old[i])
		}
	}
	for j, k := range newKeys {
		if !oldSet[k] {
			added(k, new[j])
		}
	}
}

// uniqueKeys computes keys and disambiguates repeats with a "#n" suffix.
func uniqueKeys[T any](list []T, key func(int, T) string) []string {
	keys := make([]string, len(list))
	seen := map[string]int{}
	for i, v := range list {
		k := key(i, v)
		if n := seen[k]; n > 0 {
			seen[k]++
			k = k + "#" + strconv.Itoa(n+1)
		} else {
			seen[k] = 1
		}
		keys[i] = k
	}
	return keys
}

func (d *differ) servers(old, new []Server) {
	keyed(old, new, func(_ int, s Server) string { return s.Server },
		func(k string, _ Server) {
			d.add(Change{Kind: Removed, Path: "servers[" + k + "]", Breaking: true})
		},
		func(k string, _ Server) {
			d.add(Change{Kind: Added, Path: "servers[" + k + "]"})
		},
		func(k string, o, n Server) {
			p := "servers[" + k + "]."
			d.str(p+"type", o.Type, n.Type, true)
			for _, f := range []struct {
				name     string
				old, new string
			}{
				{"host", o.Host, n.Host}, {"database", o.Database, n.Database}, {"schema", o.Schema, n.Schema},
				{"catalog", o.Catalog, n.Catalog}, {"catalogUrl", o.CatalogURL, n.CatalogURL},
				{"account", o.Account, n.Account}, {"warehouse", o.Warehouse, n.Warehouse},
				{"project", o.Project, n.Project}, {"dataset", o.Dataset, n.Dataset},
				{"location", o.Location, n.Location}, {"path", o.Path, n.Path},
				{"format", o.Format, n.Format}, {"delimiter", o.Delimiter, n.Delimiter},
				{"encoding", o.Encoding, n.Encoding}, {"endpointUrl", o.EndpointURL, n.EndpointURL},
				{"namespace", o.Namespace, n.Namespace}, {"region", o.Region, n.Region},
				{"regionName", o.RegionName, n.RegionName}, {"serviceName", o.ServiceName, n.ServiceName},
				{"stagingDir", o.StagingDir, n.StagingDir}, {"stream", o.Stream, n.Stream},
				{"workgroup", o.Workgroup, n.Workgroup},
			} {
				d.str(p+f.name, f.old, f.new, true)
			}
			if fmt.Sprint(o.Port) != fmt.Sprint(n.Port) {
				d.add(Change{Kind: Changed, Path: p + "port", Old: o.Port, New: n.Port, Breaking: true})
			}
			d.str(p+"environment", o.Environment, n.Environment, false)
			d.str(p+"description", o.Description, n.Description, false)
		})
}

func (d *differ) schema(old, new []SchemaObject) {
	keyed(old, new, func(_ int, o SchemaObject) string { return o.Name },
		func(k string, _ SchemaObject) {
			d.add(Change{Kind: Removed, Path: "schema[" + k + "]", Breaking: true})
		},
		func(k string, _ SchemaObject) {
			d.add(Change{Kind: Added, Path: "schema[" + k + "]"})
		},
		func(k string, o, n SchemaObject) {
			p := "schema[" + k + "]"
			d.str(p+".physicalName", o.PhysicalName, n.PhysicalName, true)
			d.str(p+".physicalType", o.PhysicalType, n.PhysicalType, true)
			d.str(p+".logicalType", o.LogicalType, n.LogicalType, true)
			d.str(p+".businessName", o.BusinessName, n.BusinessName, false)
			d.str(p+".description", o.Description, n.Description, false)
			d.str(p+".dataGranularityDescription", o.DataGranularityDescription, n.DataGranularityDescription, false)
			d.value(p+".deprecated", o.Deprecated, n.Deprecated, false)
			d.relationships(p, o.Relationships, n.Relationships)
			d.properties(p, o.Properties, n.Properties)
			d.quality(p, o.Quality, n.Quality)
		})
}

func (d *differ) properties(parent string, old, new []SchemaProperty) {
	keyed(old, new, func(_ int, p SchemaProperty) string { return p.Name },
		func(k string, _ SchemaProperty) {
			d.add(Change{Kind: Removed, Path: parent + ".properties[" + k + "]", Breaking: true})
		},
		func(k string, p SchemaProperty) {
			c := Change{Kind: Added, Path: parent + ".properties[" + k + "]"}
			if p.Required {
				c.Breaking, c.Note = true, "new required property"
			}
			d.add(c)
		},
		func(k string, o, n SchemaProperty) {
			d.property(parent+".properties["+k+"]", &o, &n)
		})
}

func (d *differ) property(p string, o, n *SchemaProperty) {
	d.str(p+".logicalType", o.LogicalType, n.LogicalType, true)
	d.str(p+".physicalType", o.PhysicalType, n.PhysicalType, true)
	d.str(p+".physicalName", o.PhysicalName, n.PhysicalName, true)
	d.value(p+".required", o.Required, n.Required, n.Required)
	d.value(p+".unique", o.Unique, n.Unique, n.Unique)
	d.value(p+".primaryKey", o.PrimaryKey, n.PrimaryKey, true)
	if o.PrimaryKey && n.PrimaryKey {
		d.value(p+".primaryKeyPosition", intOrNil(o.PrimaryKeyPosition), intOrNil(n.PrimaryKeyPosition), true)
	}
	d.options(p+".logicalTypeOptions", o.LogicalTypeOptions, n.LogicalTypeOptions)
	d.enum(p+".enum", o.Enum, n.Enum)
	d.str(p+".businessName", o.BusinessName, n.BusinessName, false)
	d.str(p+".description", o.Description, n.Description, false)
	d.str(p+".classification", o.Classification, n.Classification, false)
	d.value(p+".partitioned", o.Partitioned, n.Partitioned, false)
	d.value(p+".criticalDataElement", o.CriticalDataElement, n.CriticalDataElement, false)
	d.value(p+".deprecated", o.Deprecated, n.Deprecated, false)
	d.relationships(p, o.Relationships, n.Relationships)
	d.properties(p, o.Properties, n.Properties)
	d.nested(p+".items", o.Items, n.Items)
	var ok, ov, nk, nv *SchemaProperty
	if o.Map != nil {
		ok, ov = o.Map.Key, o.Map.Value
	}
	if n.Map != nil {
		nk, nv = n.Map.Key, n.Map.Value
	}
	d.nested(p+".map.key", ok, nk)
	d.nested(p+".map.value", ov, nv)
	d.quality(p, o.Quality, n.Quality)
}

func (d *differ) nested(p string, o, n *SchemaProperty) {
	switch {
	case o == nil && n == nil:
	case o == nil:
		d.add(Change{Kind: Added, Path: p, Breaking: true})
	case n == nil:
		d.add(Change{Kind: Removed, Path: p, Breaking: true})
	default:
		d.property(p, o, n)
	}
}

func intOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// Lower and upper bounds in logicalTypeOptions.
var (
	lowerBounds = []string{"minLength", "minimum", "exclusiveMinimum", "minItems", "minProperties"}
	upperBounds = []string{"maxLength", "maximum", "exclusiveMaximum", "maxItems", "maxProperties"}
	// Options that only document the data and never break anything.
	informativeOptions = []string{"defaultTimezone"}
)

func (d *differ) options(p string, old, new map[string]any) {
	keys := make([]string, 0, len(old)+len(new))
	for k := range old {
		keys = append(keys, k)
	}
	for k := range new {
		if _, ok := old[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		ov, inOld := old[k]
		nv, inNew := new[k]
		path := p + "." + k
		informative := slices.Contains(informativeOptions, k)
		switch {
		case !inOld:
			breaking := !informative && !(k == "uniqueItems" && nv == false)
			d.add(Change{Kind: Added, Path: path, New: nv, Breaking: breaking, Note: noteIf(breaking, "new constraint")})
		case !inNew:
			d.add(Change{Kind: Removed, Path: path, Old: ov})
		case fmt.Sprint(ov) != fmt.Sprint(nv):
			breaking := !informative
			switch {
			case slices.Contains(lowerBounds, k):
				breaking = compare(nv, ov) > 0
			case slices.Contains(upperBounds, k):
				breaking = compare(nv, ov) < 0
			case k == "uniqueItems":
				breaking = nv == true
			case k == "required":
				breaking = !subset(nv, ov)
			}
			d.add(Change{Kind: Changed, Path: path, Old: ov, New: nv, Breaking: breaking, Note: tightNote(breaking, k)})
		}
	}
}

func noteIf(b bool, s string) string {
	if b {
		return s
	}
	return ""
}

func tightNote(breaking bool, key string) string {
	if !slices.Contains(lowerBounds, key) && !slices.Contains(upperBounds, key) {
		return ""
	}
	if breaking {
		return "tightened"
	}
	return "loosened"
}

// compare orders two option values numerically when possible, otherwise as
// strings (which works for ISO 8601 dates).
func compare(a, b any) int {
	fa, aok := toFloat(a)
	fb, bok := toFloat(b)
	if aok && bok {
		switch {
		case fa < fb:
			return -1
		case fa > fb:
			return 1
		}
		return 0
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

func toFloat(v any) (float64, bool) {
	switch v := v.(type) {
	case interface{ Float64() (float64, error) }: // json.Number, Number
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// subset reports whether every element of list a is in list b.
func subset(a, b any) bool {
	as, _ := a.([]any)
	bs, _ := b.([]any)
	for _, x := range as {
		if !slices.ContainsFunc(bs, func(y any) bool { return fmt.Sprint(x) == fmt.Sprint(y) }) {
			return false
		}
	}
	return true
}

func (d *differ) enum(p string, old, new []EnumValue) {
	if len(old) == 0 && len(new) > 0 {
		d.add(Change{Kind: Added, Path: p, Breaking: true, Note: "values restricted to an enumeration"})
		return
	}
	if len(old) > 0 && len(new) == 0 {
		d.add(Change{Kind: Removed, Path: p, Note: "values no longer restricted"})
		return
	}
	key := func(_ int, e EnumValue) string { return fmt.Sprint(e.Value) }
	keyed(old, new, key,
		func(k string, _ EnumValue) {
			d.add(Change{Kind: Removed, Path: p + "[" + k + "]", Breaking: true})
		},
		func(k string, _ EnumValue) {
			d.add(Change{Kind: Added, Path: p + "[" + k + "]"})
		},
		func(k string, o, n EnumValue) {
			d.str(p+"["+k+"].label", o.Label, n.Label, false)
			d.str(p+"["+k+"].description", o.Description, n.Description, false)
		})
}

func (d *differ) relationships(p string, old, new []Relationship) {
	key := func(_ int, r Relationship) string {
		if r.From == nil {
			return r.To.String()
		}
		return r.From.String() + "->" + r.To.String()
	}
	keyed(old, new, key,
		func(k string, _ Relationship) {
			d.add(Change{Kind: Removed, Path: p + ".relationships[" + k + "]"})
		},
		func(k string, _ Relationship) {
			d.add(Change{Kind: Added, Path: p + ".relationships[" + k + "]"})
		},
		func(string, Relationship, Relationship) {})
}

func qualityKey(i int, q Quality) string {
	switch {
	case q.ID != "":
		return q.ID
	case q.Name != "":
		return q.Name
	case q.Metric != "":
		return q.Metric
	case q.Rule != "":
		return q.Rule
	case q.Query != "":
		return "sql"
	case q.Type != "":
		return q.Type
	}
	return "#" + strconv.Itoa(i)
}

func (d *differ) quality(parent string, old, new []Quality) {
	keyed(old, new, qualityKey,
		func(k string, _ Quality) {
			d.add(Change{Kind: Removed, Path: parent + ".quality[" + k + "]"})
		},
		func(k string, q Quality) {
			c := Change{Kind: Added, Path: parent + ".quality[" + k + "]"}
			if q.Type != "text" {
				c.Breaking, c.Note = true, "new quality rule"
			}
			d.add(c)
		},
		func(k string, o, n Quality) {
			p := parent + ".quality[" + k + "]"
			d.str(p+".type", o.Type, n.Type, true)
			d.str(p+".metric", o.Metric, n.Metric, true)
			d.str(p+".query", o.Query, n.Query, true)
			d.str(p+".engine", o.Engine, n.Engine, true)
			d.value(p+".implementation", o.Implementation, n.Implementation, true)
			d.value(p+".arguments", o.Arguments, n.Arguments, true)
			d.unit(p+".unit", o.Unit, n.Unit)
			d.thresholds(p, &o, &n)
			d.str(p+".severity", o.Severity, n.Severity, false)
			d.str(p+".dimension", o.Dimension, n.Dimension, false)
			d.str(p+".description", o.Description, n.Description, false)
		})
}

func (d *differ) unit(p, o, n string) {
	if o != n {
		d.add(Change{Kind: Changed, Path: p, Old: o, New: n, Breaking: true, Note: "thresholds change meaning"})
	}
}

// thresholds compares quality operators. A rule that admits fewer values is
// tighter, hence breaking.
func (d *differ) thresholds(p string, o, n *Quality) {
	bound := func(name string, ov, nv *Number, tighter func(o, n float64) bool) {
		switch {
		case ov == nil && nv == nil:
		case ov == nil:
			d.add(Change{Kind: Added, Path: p + "." + name, New: *nv, Breaking: true})
		case nv == nil:
			d.add(Change{Kind: Removed, Path: p + "." + name, Old: *ov})
		case *ov != *nv:
			of, _ := ov.Float64()
			nf, _ := nv.Float64()
			if of == nf {
				return
			}
			t := tighter(of, nf)
			d.add(Change{Kind: Changed, Path: p + "." + name, Old: *ov, New: *nv, Breaking: t, Note: tightNoteBool(t)})
		}
	}
	up := func(o, n float64) bool { return n > o }
	down := func(o, n float64) bool { return n < o }
	bound("mustBeGreaterThan", o.MustBeGreaterThan, n.MustBeGreaterThan, up)
	bound("mustBeGreaterOrEqualTo", o.MustBeGreaterOrEqualTo, n.MustBeGreaterOrEqualTo, up)
	bound("mustBeLessThan", o.MustBeLessThan, n.MustBeLessThan, down)
	bound("mustBeLessOrEqualTo", o.MustBeLessOrEqualTo, n.MustBeLessOrEqualTo, down)

	rng := func(name string, ov, nv []Number, inside bool) {
		switch {
		case len(ov) != 2 && len(nv) != 2:
		case len(ov) != 2:
			d.add(Change{Kind: Added, Path: p + "." + name, New: nv, Breaking: true})
		case len(nv) != 2:
			d.add(Change{Kind: Removed, Path: p + "." + name, Old: ov})
		default:
			olo, _ := ov[0].Float64()
			ohi, _ := ov[1].Float64()
			nlo, _ := nv[0].Float64()
			nhi, _ := nv[1].Float64()
			if olo == nlo && ohi == nhi {
				return
			}
			// mustBeBetween: narrower is tighter; mustNotBeBetween: wider is.
			t := nlo > olo || nhi < ohi
			if !inside {
				t = nlo < olo || nhi > ohi
			}
			d.add(Change{Kind: Changed, Path: p + "." + name, Old: ov, New: nv, Breaking: t, Note: tightNoteBool(t)})
		}
	}
	rng("mustBeBetween", o.MustBeBetween, n.MustBeBetween, true)
	rng("mustNotBeBetween", o.MustNotBeBetween, n.MustNotBeBetween, false)

	exact := func(name string, ov, nv any) {
		switch {
		case ov == nil && nv == nil:
		case ov == nil:
			d.add(Change{Kind: Added, Path: p + "." + name, New: nv, Breaking: true})
		case nv == nil:
			d.add(Change{Kind: Removed, Path: p + "." + name, Old: ov})
		default:
			d.value(p+"."+name, ov, nv, true)
		}
	}
	exact("mustBe", o.MustBe, n.MustBe)
	exact("mustNotBe", o.MustNotBe, n.MustNotBe)
}

func tightNoteBool(t bool) string {
	if t {
		return "tightened"
	}
	return "loosened"
}

func (d *differ) sla(old, new []SLAProperty) {
	key := func(_ int, s SLAProperty) string {
		if s.Element != "" {
			return s.Property + "@" + s.Element
		}
		return s.Property
	}
	keyed(old, new, key,
		func(k string, _ SLAProperty) {
			d.add(Change{Kind: Removed, Path: "slaProperties[" + k + "]"})
		},
		func(k string, _ SLAProperty) {
			d.add(Change{Kind: Added, Path: "slaProperties[" + k + "]"})
		},
		func(k string, o, n SLAProperty) {
			p := "slaProperties[" + k + "]"
			if fmt.Sprint(o.Value) != fmt.Sprint(n.Value) {
				d.add(Change{Kind: Changed, Path: p + ".value", Old: o.Value, New: n.Value})
			}
			d.str(p+".unit", o.Unit, n.Unit, false)
			d.str(p+".driver", o.Driver, n.Driver, false)
		})
}

func (d *differ) roles(old, new []Role) {
	keyed(old, new, func(_ int, r Role) string { return r.Role },
		func(k string, _ Role) {
			d.add(Change{Kind: Removed, Path: "roles[" + k + "]", Breaking: true})
		},
		func(k string, _ Role) {
			d.add(Change{Kind: Added, Path: "roles[" + k + "]"})
		},
		func(k string, o, n Role) {
			d.str("roles["+k+"].access", o.Access, n.Access, false)
		})
}

func (d *differ) price(o, n *Price) {
	var op, np Price
	if o != nil {
		op = *o
	}
	if n != nil {
		np = *n
	}
	d.value("price.priceAmount", numOrNil(op.PriceAmount), numOrNil(np.PriceAmount), false)
	d.str("price.priceCurrency", op.PriceCurrency, np.PriceCurrency, false)
	d.str("price.priceUnit", op.PriceUnit, np.PriceUnit, false)
}

func numOrNil(n *Number) any {
	if n == nil {
		return nil
	}
	return *n
}
