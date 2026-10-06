package odcs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Severity ranks lint issues.
type Severity int

// Severities, from least to most severe.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	}
	return "Severity(" + strconv.Itoa(int(s)) + ")"
}

// MarshalText encodes the severity as its name.
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Issue is a finding reported by [Lint].
type Issue struct {
	Severity Severity `json:"severity"`
	// Path is a JSON Pointer to the element the issue is about.
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	p := i.Path
	if p == "" {
		p = "/"
	}
	return fmt.Sprintf("%s: %s: %s", i.Severity, p, i.Message)
}

// HasErrors reports whether any issue has SeverityError.
func HasErrors(issues []Issue) bool {
	return slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == SeverityError })
}

// Lint runs semantic checks that the JSON Schema cannot express:
//
//   - apiVersion is known, kind is DataContract, id and version are set;
//   - schema objects, properties, servers and stable ids are unique where the
//     standard requires it;
//   - relationships, SLA elements and slaDefaultElement point to schema
//     objects and properties that exist (references to other files are not
//     followed);
//   - composite relationships have as many sources as targets;
//   - SLA and quality units are recognised.
//
// Issues are returned in document order.
func Lint(c *DataContract) []Issue {
	l := &linter{c: c, byName: map[string]*SchemaObject{}, byID: map[string]*SchemaObject{}}
	for i := range c.Schema {
		o := &c.Schema[i]
		if _, ok := l.byName[o.Name]; !ok && o.Name != "" {
			l.byName[o.Name] = o
		}
		if _, ok := l.byID[o.ID]; !ok && o.ID != "" {
			l.byID[o.ID] = o
		}
	}
	l.fundamentals()
	l.servers()
	l.schema()
	l.sla()
	return l.issues
}

type linter struct {
	c      *DataContract
	byName map[string]*SchemaObject
	byID   map[string]*SchemaObject
	issues []Issue
}

func (l *linter) add(sev Severity, path, format string, args ...any) {
	l.issues = append(l.issues, Issue{Severity: sev, Path: path, Message: fmt.Sprintf(format, args...)})
}

func (l *linter) fundamentals() {
	c := l.c
	switch {
	case c.APIVersion == "":
		l.add(SeverityError, "/apiVersion", "apiVersion is missing")
	case !slices.Contains(SupportedAPIVersions, c.APIVersion):
		l.add(SeverityError, "/apiVersion", "unknown apiVersion %q (supported: %s)", c.APIVersion, strings.Join(SupportedAPIVersions, ", "))
	case !strings.HasPrefix(c.APIVersion, "v3."):
		l.add(SeverityWarning, "/apiVersion", "apiVersion %s predates ODCS v3; this library models %s", c.APIVersion, SchemaVersion)
	}
	if c.Kind != "DataContract" {
		l.add(SeverityError, "/kind", "kind must be DataContract, got %q", c.Kind)
	}
	if strings.TrimSpace(c.ID) == "" {
		l.add(SeverityError, "/id", "id is missing")
	}
	if strings.TrimSpace(c.Version) == "" {
		l.add(SeverityError, "/version", "version is missing")
	}
}

func (l *linter) servers() {
	names := map[string]int{}
	ids := map[string]int{}
	for i, s := range l.c.Servers {
		p := "/servers/" + strconv.Itoa(i)
		if j, dup := names[s.Server]; dup {
			l.add(SeverityError, p+"/server", "duplicate server %q (also at /servers/%d)", s.Server, j)
		} else {
			names[s.Server] = i
		}
		l.uniqueID(ids, s.ID, p, "/servers/")
	}
}

func (l *linter) uniqueID(seen map[string]int, id, path, prefix string) {
	if id == "" {
		return
	}
	if j, dup := seen[id]; dup {
		l.add(SeverityError, path+"/id", "duplicate id %q (also at %s%d)", id, prefix, j)
		return
	}
	seen[id] = idx(path)
}

// idx returns the last index in a JSON pointer.
func idx(path string) int {
	n, _ := strconv.Atoi(path[strings.LastIndexByte(path, '/')+1:])
	return n
}

func (l *linter) schema() {
	names := map[string]int{}
	ids := map[string]int{}
	for i := range l.c.Schema {
		o := &l.c.Schema[i]
		p := "/schema/" + strconv.Itoa(i)
		if strings.TrimSpace(o.Name) == "" {
			l.add(SeverityError, p+"/name", "schema object has no name")
		} else if j, dup := names[o.Name]; dup {
			l.add(SeverityError, p+"/name", "duplicate schema object %q (also at /schema/%d)", o.Name, j)
		} else {
			names[o.Name] = i
		}
		l.uniqueID(ids, o.ID, p, "/schema/")
		for k, r := range o.Relationships {
			rp := p + "/relationships/" + strconv.Itoa(k)
			if r.From == nil || r.To == nil {
				continue // required by the schema; reported by Validate
			}
			if len(r.From.Values) != len(r.To.Values) {
				l.add(SeverityError, rp, "relationship has %d source(s) but %d target(s)", len(r.From.Values), len(r.To.Values))
			}
			l.refs(rp+"/from", r.From.Values)
			l.refs(rp+"/to", r.To.Values)
		}
		l.properties(p, o.Properties)
		l.quality(p, o.Quality)
	}
}

func (l *linter) properties(parent string, props []SchemaProperty) {
	names := map[string]int{}
	ids := map[string]int{}
	for i := range props {
		pr := &props[i]
		p := parent + "/properties/" + strconv.Itoa(i)
		if strings.TrimSpace(pr.Name) == "" {
			l.add(SeverityError, p+"/name", "property has no name")
		} else if j, dup := names[pr.Name]; dup {
			l.add(SeverityError, p+"/name", "duplicate property %q (also at %s/properties/%d)", pr.Name, parent, j)
		} else {
			names[pr.Name] = i
		}
		l.uniqueID(ids, pr.ID, p, parent+"/properties/")
		l.property(p, pr)
	}
}

// property checks what a property contains, apart from its name.
func (l *linter) property(p string, pr *SchemaProperty) {
	for k, r := range pr.Relationships {
		if r.To != nil {
			l.refs(p+"/relationships/"+strconv.Itoa(k)+"/to", r.To.Values)
		}
	}
	l.quality(p, pr.Quality)
	l.properties(p, pr.Properties)
	if pr.Items != nil {
		l.property(p+"/items", pr.Items)
	}
	if pr.Map != nil {
		if pr.Map.Key != nil {
			l.property(p+"/map/key", pr.Map.Key)
		}
		if pr.Map.Value != nil {
			l.property(p+"/map/value", pr.Map.Value)
		}
	}
}

var qualityUnits = []string{"rows", "percent"}

func (l *linter) quality(parent string, rules []Quality) {
	ids := map[string]int{}
	for i, q := range rules {
		p := parent + "/quality/" + strconv.Itoa(i)
		l.uniqueID(ids, q.ID, p, parent+"/quality/")
		if q.Unit != "" && !slices.Contains(qualityUnits, q.Unit) {
			l.add(SeverityInfo, p+"/unit", "unusual quality unit %q (common values: rows, percent)", q.Unit)
		}
	}
}

// timeUnits are the duration units accepted for time-based SLA properties.
var timeUnits = map[string]bool{
	"ms": true, "millisecond": true, "milliseconds": true,
	"s": true, "sec": true, "secs": true, "second": true, "seconds": true,
	"min": true, "mins": true, "minute": true, "minutes": true,
	"h": true, "hr": true, "hrs": true, "hour": true, "hours": true,
	"d": true, "day": true, "days": true,
	"w": true, "wk": true, "week": true, "weeks": true,
	"mo": true, "month": true, "months": true,
	"y": true, "yr": true, "yrs": true, "year": true, "years": true,
}

// timeSLA are SLA properties whose value is a duration and needs a unit.
var timeSLA = map[string]bool{
	"latency": true, "retention": true, "frequency": true,
	"timetodetect": true, "timetonotify": true, "timetorepair": true,
}

func (l *linter) sla() {
	if e := l.c.SLADefaultElement; e != "" {
		l.refs("/slaDefaultElement", splitElements(e))
	}
	ids := map[string]int{}
	for i, s := range l.c.SLAProperties {
		p := "/slaProperties/" + strconv.Itoa(i)
		l.uniqueID(ids, s.ID, p, "/slaProperties/")
		unit := strings.ToLower(strings.TrimSpace(s.Unit))
		switch {
		case timeSLA[strings.ToLower(s.Property)] && unit == "":
			l.add(SeverityWarning, p+"/unit", "SLA property %q needs a unit (for example d, h, ms)", s.Property)
		case timeSLA[strings.ToLower(s.Property)] && !timeUnits[unit]:
			l.add(SeverityWarning, p+"/unit", "invalid time unit %q for SLA property %q", s.Unit, s.Property)
		case unit != "" && !timeUnits[unit] && unit != "%" && unit != "percent":
			l.add(SeverityInfo, p+"/unit", "unrecognised unit %q", s.Unit)
		}
		if s.Element != "" {
			l.refs(p+"/element", splitElements(s.Element))
		}
	}
}

func splitElements(s string) []string {
	var out []string
	for _, e := range strings.Split(s, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func (l *linter) refs(path string, refs []string) {
	for _, r := range refs {
		if ok, why := l.resolve(r); !ok {
			l.add(SeverityWarning, path, "reference %q does not resolve: %s", r, why)
		}
	}
}

// resolve reports whether ref points to an existing schema object or
// property. References into other files or other sections are accepted
// without checking.
func (l *linter) resolve(ref string) (bool, string) {
	if strings.Contains(ref, "#") || strings.Contains(ref, "/") {
		return l.resolveQualified(ref)
	}
	parts := strings.Split(ref, ".")
	o, ok := l.byName[parts[0]]
	if !ok {
		return false, fmt.Sprintf("no schema object named %q", parts[0])
	}
	props := o.Properties
	for i, name := range parts[1:] {
		pr := findProperty(props, func(p *SchemaProperty) bool { return p.Name == name })
		if pr == nil {
			return false, fmt.Sprintf("%q has no property %q", strings.Join(parts[:i+1], "."), name)
		}
		props = children(pr)
	}
	return true, ""
}

func (l *linter) resolveQualified(ref string) (bool, string) {
	if strings.Contains(ref, "#") {
		return true, "" // another document
	}
	segs := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if segs[0] != "schema" {
		return true, "" // only schema references are checked
	}
	if len(segs) < 2 {
		return false, "missing schema object id"
	}
	o, ok := l.byID[segs[1]]
	if !ok {
		return false, fmt.Sprintf("no schema object with id %q", segs[1])
	}
	props := o.Properties
	for i := 2; i < len(segs); i += 2 {
		if segs[i] != "properties" || i+1 >= len(segs) {
			return false, fmt.Sprintf("unexpected segment %q", segs[i])
		}
		id := segs[i+1]
		pr := findProperty(props, func(p *SchemaProperty) bool { return p.ID == id })
		if pr == nil {
			return false, fmt.Sprintf("no property with id %q", id)
		}
		props = children(pr)
	}
	return true, ""
}

func findProperty(props []SchemaProperty, match func(*SchemaProperty) bool) *SchemaProperty {
	for i := range props {
		if match(&props[i]) {
			return &props[i]
		}
	}
	return nil
}

// children returns the nested properties of an object, or of the items of
// an array of objects.
func children(p *SchemaProperty) []SchemaProperty {
	if len(p.Properties) > 0 {
		return p.Properties
	}
	if p.Items != nil {
		return p.Items.Properties
	}
	return nil
}
