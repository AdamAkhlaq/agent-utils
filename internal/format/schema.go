package format

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// JSONSchema is a compiled JSON Schema, ready to validate documents.
type JSONSchema struct {
	schema *jsonschema.Schema
	name   string // the schema file as the caller named it
	root   string // file URL of the schema, which locations in errors start with
	dir    string // directory of the schema file
}

// SchemaReport is the outcome of validating one document. Errors is empty,
// never nil, when the document is valid, so its JSON always has the array.
type SchemaReport struct {
	Valid  bool              `json:"valid"`
	Errors []SchemaViolation `json:"errors"`
}

// SchemaViolation is one reason a document fails its schema. InstancePath
// is a JSON Pointer into the document. SchemaPath is a JSON Pointer to the
// failing keyword in the schema file, prefixed with "<document>#" when the
// keyword lives in another document reached through $ref: a local file
// (relative to the schema file's directory when that is shorter) or a
// built-in meta-schema (by its URL).
type SchemaViolation struct {
	InstancePath string `json:"instance_path"`
	SchemaPath   string `json:"schema_path"`
	Message      string `json:"message"`
}

// errNotLocal marks a reference the loader refused because it would need the
// network.
var errNotLocal = errors.New("not a local file")

// CompileJSONSchema reads and compiles the schema file at path. The draft
// comes from its $schema and defaults to 2020-12. Known formats are always
// asserted, whatever the draft. References resolve only to local files
// (relative to the schema file) and the built-in draft meta-schemas; the
// network is never used.
func CompileJSONSchema(path string) (*JSONSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema: %w", err)
	}
	doc, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving schema path: %w", err)
	}
	s := &JSONSchema{name: path, root: fileURL(abs), dir: filepath.Dir(abs)}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(localLoader{})
	if err := c.AddResource(s.root, doc); err != nil {
		return nil, s.compileError(err, doc)
	}
	if s.schema, err = c.Compile(s.root); err != nil {
		return nil, s.compileError(err, doc)
	}
	return s, nil
}

// Validate reads one JSON document from r and checks it against the schema.
// A document that fails the schema is not an error: the report lists every
// leaf violation, sorted by instance path, then schema path, then message.
// Malformed input is an error.
func (s *JSONSchema) Validate(r io.Reader) (SchemaReport, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return SchemaReport{}, fmt.Errorf("reading input: %w", err)
	}
	doc, err := decodeStrict(data)
	if err != nil {
		return SchemaReport{}, err
	}
	err = s.schema.Validate(doc)
	if err == nil {
		return SchemaReport{Valid: true, Errors: []SchemaViolation{}}, nil
	}
	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return SchemaReport{}, fmt.Errorf("validating: %w", err)
	}
	violations, err := s.flatten(verr, doc)
	if err != nil {
		return SchemaReport{}, err
	}
	return SchemaReport{Valid: false, Errors: violations}, nil
}

// SchemaReportJSON renders a report as pretty-printed JSON.
func SchemaReportJSON(report SchemaReport) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return "", fmt.Errorf("encoding JSON: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// flatten turns the library's error tree into its leaf violations, sorted
// by instance path, then schema path, then message. doc is the validated
// document, or nil when it is not at hand. A $ref loop is not the
// document's fault, so it is an error instead.
func (s *JSONSchema) flatten(e *jsonschema.ValidationError, doc any) ([]SchemaViolation, error) {
	f := flattener{schema: s}
	f.collect(e, nil)
	if f.cycle != nil {
		base, frag, _ := strings.Cut(f.cycle.URL, "#")
		return nil, fmt.Errorf("cannot be validated: schema %s has a $ref loop: %s and %s both resolve to %s without consuming input (reached at instance path %q)",
			s.name, orRoot(f.cycle.KeywordLocation1), orRoot(f.cycle.KeywordLocation2), s.displayURL(base)+"#"+frag, f.cycleAt)
	}
	f.placePropertyNames(doc)
	slices.SortFunc(f.out, func(a, b SchemaViolation) int {
		return cmp.Or(
			comparePointers(a.InstancePath, b.InstancePath),
			strings.Compare(a.SchemaPath, b.SchemaPath),
			strings.Compare(a.Message, b.Message),
		)
	})
	return f.out, nil
}

func orRoot(ptr string) string {
	if ptr == "" {
		return "(root)"
	}
	return ptr
}

type flattener struct {
	schema  *JSONSchema
	out     []SchemaViolation
	names   []propertyNameViolation
	cycle   *kind.RefCycle
	cycleAt string
}

// propertyNameViolation is a propertyNames failure whose instance path is
// still unknown. The library stores that error's location without copying
// it, so validating any later sibling overwrites its tokens (and the order
// of object siblings follows map iteration, so the result is not even
// stable). Only its length and the location of an enclosing error, always
// an ancestor, can be trusted.
type propertyNameViolation struct {
	ancestor []string
	depth    int
	key      string
	SchemaViolation
}

func (f *flattener) collect(e *jsonschema.ValidationError, parent []string) {
	switch k := e.ErrorKind.(type) {
	case *kind.PropertyNames:
		// The causes validate the name as a standalone string, so their
		// locations mean nothing here; their messages join into this one.
		f.names = append(f.names, propertyNameViolation{
			ancestor: parent,
			depth:    len(e.InstanceLocation),
			key:      k.Property,
			SchemaViolation: SchemaViolation{
				SchemaPath: f.schema.schemaPath(e.SchemaURL, nil),
				Message:    fmt.Sprintf("invalid property name %s: %s", quote(k.Property), strings.Join(leafMessages(e.Causes), "; ")),
			},
		})
		return
	case *kind.RefCycle:
		if f.cycle == nil {
			f.cycle, f.cycleAt = k, pointer(e.InstanceLocation)
		}
		return
	case *kind.Contains, *kind.MinContains:
		// Their causes say why each item missed the contains schema, which
		// would read as if every item were wrong; the summary is the finding.
	default:
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				f.collect(c, e.InstanceLocation)
			}
			return
		}
	}
	f.out = append(f.out, SchemaViolation{
		InstancePath: pointer(e.InstanceLocation),
		SchemaPath:   f.schema.schemaPath(e.SchemaURL, keywordPath(e.ErrorKind)),
		Message:      violationMessage(e.ErrorKind),
	})
}

// placePropertyNames finds the objects the propertyNames failures belong
// to: those at the failure's depth below its ancestor that have the failing
// key. Failures with deeper (more specific) ancestors claim their objects
// first. When exactly as many unclaimed objects remain as identical
// failures, the match is exact; otherwise the failures are placed at the
// ancestor, which is coarser but never wrong.
func (f *flattener) placePropertyNames(doc any) {
	type groupKey struct {
		ancestor string
		depth    int
		key      string
		SchemaViolation
	}
	type group struct {
		violation propertyNameViolation
		count     int
	}
	var groups []*group
	byKey := map[groupKey]*group{}
	for _, v := range f.names {
		k := groupKey{pointer(v.ancestor), v.depth, v.key, v.SchemaViolation}
		if g, ok := byKey[k]; ok {
			g.count++
			continue
		}
		g := &group{violation: v, count: 1}
		byKey[k] = g
		groups = append(groups, g)
	}
	slices.SortStableFunc(groups, func(a, b *group) int {
		return cmp.Compare(len(b.violation.ancestor), len(a.violation.ancestor))
	})
	type claim struct{ path, key, schemaPath string }
	claimed := map[claim]bool{}
	for _, g := range groups {
		v := g.violation
		var paths []string
		for _, p := range objectsWithKey(doc, v.ancestor, v.depth, v.key) {
			if !claimed[claim{p, v.key, v.SchemaPath}] {
				paths = append(paths, p)
			}
		}
		if len(paths) != g.count {
			paths = slices.Repeat([]string{pointer(v.ancestor)}, g.count)
		}
		for _, p := range paths {
			claimed[claim{p, v.key, v.SchemaPath}] = true
			violation := v.SchemaViolation
			violation.InstancePath = p
			f.out = append(f.out, violation)
		}
	}
}

// objectsWithKey returns the JSON Pointers of the objects depth tokens deep
// in doc, below the ancestor path, that have key.
func objectsWithKey(doc any, ancestor []string, depth int, key string) []string {
	v, ok := lookup(doc, ancestor)
	if !ok || depth < len(ancestor) {
		return nil
	}
	var found []string
	var walk func(v any, path []string)
	walk = func(v any, path []string) {
		if len(path) == depth {
			if obj, ok := v.(map[string]any); ok {
				if _, has := obj[key]; has {
					found = append(found, pointer(path))
				}
			}
			return
		}
		switch v := v.(type) {
		case map[string]any:
			for k, child := range v {
				walk(child, append(slices.Clip(path), k))
			}
		case []any:
			for i, child := range v {
				walk(child, append(slices.Clip(path), strconv.Itoa(i)))
			}
		}
	}
	walk(v, slices.Clone(ancestor))
	return found
}

func lookup(doc any, path []string) (any, bool) {
	if doc == nil {
		return nil, false
	}
	v := doc
	for _, tok := range path {
		switch c := v.(type) {
		case map[string]any:
			child, ok := c[tok]
			if !ok {
				return nil, false
			}
			v = child
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(c) {
				return nil, false
			}
			v = c[i]
		default:
			return nil, false
		}
	}
	return v, true
}

// keywordPath is the failing keyword's path below its schema object. The
// library's own path is wrong for two kinds: it has none for "not", and
// names draft-07's "dependencies" as "dependency".
func keywordPath(k jsonschema.ErrorKind) []string {
	switch k := k.(type) {
	case *kind.Not:
		return []string{"not"}
	case *kind.Dependency:
		return []string{"dependencies", k.Prop}
	}
	return k.KeywordPath()
}

func leafMessages(errs []*jsonschema.ValidationError) []string {
	var msgs []string
	for _, e := range errs {
		if len(e.Causes) > 0 {
			msgs = append(msgs, leafMessages(e.Causes)...)
		} else {
			msgs = append(msgs, violationMessage(e.ErrorKind))
		}
	}
	return msgs
}

// schemaPath turns the library's absolute schema location plus a keyword
// path into the SchemaViolation form.
func (s *JSONSchema) schemaPath(schemaURL string, keyword []string) string {
	base, frag, _ := strings.Cut(schemaURL, "#")
	if unescaped, err := url.PathUnescape(frag); err == nil {
		frag = unescaped
	}
	ptr := frag + pointer(keyword)
	if base == s.root {
		return ptr
	}
	return s.displayURL(base) + "#" + ptr
}

// displayURL shows a schema document's URL the way a user would name it:
// the root schema by the path it was given as, another local file relative
// to the root schema's directory, anything else as-is.
func (s *JSONSchema) displayURL(u string) string {
	if u == s.root {
		return s.name
	}
	path, err := filePath(u)
	if err != nil {
		return u
	}
	if rel, err := filepath.Rel(s.dir, path); err == nil && len(rel) < len(path) {
		return filepath.ToSlash(rel)
	}
	return path
}

// compileError rewrites the library's compile errors, which name documents
// by absolute file URL and nest meta-schema failures over several lines,
// into one specific line.
func (s *JSONSchema) compileError(err error, doc any) error {
	var invalid *jsonschema.SchemaValidationError
	var load *jsonschema.LoadURLError
	switch {
	case errors.As(err, &invalid):
		base, frag, _ := strings.Cut(invalid.URL, "#")
		if base != s.root || frag != "" {
			doc = nil // the failing paths are relative to another document
		}
		var details []string
		var verr *jsonschema.ValidationError
		if errors.As(invalid.Err, &verr) {
			leaves, err := s.flatten(verr, doc)
			if err != nil {
				return err
			}
			for _, l := range leaves {
				details = append(details, orRoot(l.InstancePath)+": "+l.Message)
			}
		} else {
			details = append(details, invalid.Err.Error())
		}
		name := s.displayURL(base)
		if frag != "" {
			name += "#" + frag
		}
		return fmt.Errorf("invalid schema %s: %s", name, strings.Join(details, "; "))
	case errors.As(err, &load) && errors.Is(load.Err, errNotLocal) && isMetaSchemaURL(load.URL):
		return fmt.Errorf("unsupported meta-schema %q: the supported drafts are 4, 6, 7, 2019-09, and 2020-12", load.URL)
	case errors.As(err, &load) && errors.Is(load.Err, errNotLocal):
		return fmt.Errorf("cannot resolve %q: only local files and the built-in draft meta-schemas can be referenced (json-schema never accesses the network)", load.URL)
	case errors.As(err, &load):
		return fmt.Errorf("cannot load %s: %w", s.displayURL(load.URL), load.Err)
	default:
		return errors.New(strings.ReplaceAll(err.Error(), s.root, s.name))
	}
}

// isMetaSchemaURL reports whether u names a json-schema.org draft
// meta-schema (draft-03/schema, draft/2021-01/schema, ...). The supported
// drafts' meta-schemas are built in and never reach a loader, so one that
// does belongs to a draft that is not supported.
func isMetaSchemaURL(u string) bool {
	rest, ok := strings.CutPrefix(u, "http://json-schema.org/")
	if !ok {
		rest, ok = strings.CutPrefix(u, "https://json-schema.org/")
	}
	return ok && (strings.HasPrefix(rest, "draft-") || strings.HasPrefix(rest, "draft/"))
}

// localLoader resolves every reference that is not a built-in meta-schema
// (those never reach a loader). Only file URLs load; anything else would
// need the network.
type localLoader struct{}

func (localLoader) Load(u string) (any, error) {
	path, err := filePath(u)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// The caller names the file already; drop the path os repeats.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, pathErr.Err
		}
		return nil, err
	}
	return decodeStrict(data)
}

func fileURL(abs string) string {
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // a Windows drive path: C:/x becomes /C:/x
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func filePath(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "file" || (parsed.Host != "" && parsed.Host != "localhost") {
		return "", errNotLocal
	}
	if parsed.Opaque != "" {
		return "", errors.New("not a usable file URL (it has no absolute path); refer to the file by a relative path instead")
	}
	p := parsed.Path
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // /C:/x is the drive path C:/x; //server/share stays a UNC path
	}
	return filepath.FromSlash(p), nil
}

// decodeStrict parses data, which must hold exactly one JSON document, into
// the generic form the validator takes, with every number kept exact as a
// json.Number. It is stricter than encoding/json where leniency would mean
// validating something other than the document as written: invalid UTF-8
// (silently replaced by U+FFFD), duplicate keys (the last one silently
// wins), and numbers whose decimal exponent is beyond ±1000000 (math/big
// refuses them, and the library then panics or skips the keyword).
func decodeStrict(data []byte) (any, error) {
	data = trimBOM(data)
	if len(bytes.Trim(data, " \t\r\n")) == 0 {
		return nil, errors.New("empty input")
	}
	if err := checkUTF8(data); err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		// checkSyntax is slower, but finds the position.
		if err := checkSyntax(data); err != nil {
			return nil, err
		}
		return nil, ErrInvalidJSON
	}
	d := strictDecoder{data: data, dec: json.NewDecoder(bytes.NewReader(data))}
	d.dec.UseNumber()
	return d.value()
}

type strictDecoder struct {
	data []byte
	dec  *json.Decoder
}

func (d *strictDecoder) value() (any, error) {
	start := nextTokenOffset(d.data, d.dec)
	tok, err := d.dec.Token()
	if err != nil {
		return nil, syntaxError(d.data, err)
	}
	switch tok := tok.(type) {
	case json.Delim:
		if tok == '{' {
			return d.object()
		}
		return d.array()
	case json.Number:
		if _, ok := new(big.Rat).SetString(tok.String()); !ok {
			line, col := lineCol(d.data, start+1)
			return nil, fmt.Errorf("number %s at line %d, column %d is out of range: decimal exponents beyond ±1000000 are not supported", tok, line, col)
		}
		return tok, nil
	default:
		return tok, nil
	}
}

func (d *strictDecoder) object() (any, error) {
	obj := map[string]any{}
	for d.dec.More() {
		start := nextTokenOffset(d.data, d.dec)
		tok, err := d.dec.Token()
		if err != nil {
			return nil, syntaxError(d.data, err)
		}
		key, _ := tok.(string)
		if _, dup := obj[key]; dup {
			line, col := lineCol(d.data, start+1)
			return nil, fmt.Errorf("duplicate key %q at line %d, column %d", key, line, col)
		}
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		obj[key] = v
	}
	if _, err := d.dec.Token(); err != nil {
		return nil, syntaxError(d.data, err)
	}
	return obj, nil
}

func (d *strictDecoder) array() (any, error) {
	arr := []any{}
	for d.dec.More() {
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	if _, err := d.dec.Token(); err != nil {
		return nil, syntaxError(d.data, err)
	}
	return arr, nil
}

var printer = message.NewPrinter(language.English)

// violationMessage describes one leaf failure. The library's own messages
// render numbers through float64 and with digit grouping (9007199254740993
// becomes "9.007199254740992 × 10¹⁵"), list additional properties in map
// order, and print composite enum values as just "value"; those kinds are
// rendered here instead.
func violationMessage(k jsonschema.ErrorKind) string {
	switch k := k.(type) {
	case *kind.Minimum:
		return fmt.Sprintf("must be >= %s, got %s", formatNumber(k.Want), formatNumber(k.Got))
	case *kind.Maximum:
		return fmt.Sprintf("must be <= %s, got %s", formatNumber(k.Want), formatNumber(k.Got))
	case *kind.ExclusiveMinimum:
		return fmt.Sprintf("must be > %s, got %s", formatNumber(k.Want), formatNumber(k.Got))
	case *kind.ExclusiveMaximum:
		return fmt.Sprintf("must be < %s, got %s", formatNumber(k.Want), formatNumber(k.Got))
	case *kind.MultipleOf:
		return fmt.Sprintf("must be a multiple of %s, got %s", formatNumber(k.Want), formatNumber(k.Got))
	case *kind.MinLength:
		return fmt.Sprintf("must be at least %s long, got %d", count(k.Want, "character"), k.Got)
	case *kind.MaxLength:
		return fmt.Sprintf("must be at most %s long, got %d", count(k.Want, "character"), k.Got)
	case *kind.MinItems:
		return fmt.Sprintf("must have at least %s, got %d", count(k.Want, "item"), k.Got)
	case *kind.MaxItems:
		return fmt.Sprintf("must have at most %s, got %d", count(k.Want, "item"), k.Got)
	case *kind.MinProperties:
		return fmt.Sprintf("must have at least %s, got %d", count(k.Want, "property"), k.Got)
	case *kind.MaxProperties:
		return fmt.Sprintf("must have at most %s, got %d", count(k.Want, "property"), k.Got)
	case *kind.MinContains:
		if len(k.Got) == 0 {
			return fmt.Sprintf("must have at least %s matching the contains schema, got none", count(k.Want, "item"))
		}
		return fmt.Sprintf("must have at least %s matching the contains schema, got %d (at %s)", count(k.Want, "item"), len(k.Got), joinInts(k.Got))
	case *kind.MaxContains:
		return fmt.Sprintf("must have at most %s matching the contains schema, got %d (at %s)", count(k.Want, "item"), len(k.Got), joinInts(k.Got))
	case *kind.UniqueItems:
		return fmt.Sprintf("items at %d and %d are equal", k.Duplicates[0], k.Duplicates[1])
	case *kind.AdditionalItems:
		return fmt.Sprintf("%s too many: additionalItems is false", count(k.Count, "item"))
	case *kind.OneOf:
		if len(k.Subschemas) == 2 {
			return fmt.Sprintf("'oneOf' failed, subschemas %d and %d both matched", k.Subschemas[0], k.Subschemas[1])
		}
	case *kind.AdditionalProperties:
		props := slices.Sorted(slices.Values(k.Properties))
		quoted := make([]string, len(props))
		for i, p := range props {
			quoted[i] = quote(p)
		}
		if len(quoted) == 1 {
			return fmt.Sprintf("additional property %s not allowed", quoted[0])
		}
		return fmt.Sprintf("additional properties %s not allowed", strings.Join(quoted, ", "))
	case *kind.Enum:
		if len(k.Want) == 1 {
			return "value must be " + displayValue(k.Want[0])
		}
		want := make([]string, len(k.Want))
		for i, v := range k.Want {
			want[i] = displayValue(v)
		}
		return "value must be one of " + strings.Join(want, ", ")
	case *kind.Const:
		return "value must be " + displayValue(k.Want)
	case *kind.Not:
		return "must not match the 'not' schema"
	case *kind.FalseSchema:
		return "not allowed: the schema here is false"
	}
	return k.LocalizedString(printer)
}

// quote matches the library's quoting, so every message quotes strings
// alike: single quotes read cleanly inside a JSON string.
func quote(s string) string {
	s = strconv.Quote(s)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s[1:len(s)-1] + "'"
}

func displayValue(v any) string {
	switch v := v.(type) {
	case string:
		return quote(v)
	case json.Number:
		return v.String()
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return fmt.Sprint(v)
		}
		return strings.TrimSuffix(buf.String(), "\n")
	}
}

// count renders n with the noun, pluralized: "1 item", "2 items".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	if strings.HasSuffix(noun, "y") {
		noun = strings.TrimSuffix(noun, "y") + "ie"
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

// formatNumber renders r exactly, in JavaScript's Number notation (plain
// digits, switching to an exponent beyond 21 integer digits or 6 leading
// fractional zeros). Every value it gets came from a JSON number, so its
// denominator is a product of 2s and 5s and the decimal expansion ends; the
// fraction form is only a fallback.
func formatNumber(r *big.Rat) string {
	if r == nil {
		return "?"
	}
	den := new(big.Int).Set(r.Denom())
	twos := int(den.TrailingZeroBits())
	den.Rsh(den, uint(twos))
	// What is left must be a power of 5. Dividing by 5 one step at a time
	// is quadratic, and a 1e-999999 has a million of them, so derive the
	// exponent from the bit length and check it instead.
	five := big.NewInt(5)
	fives := -1
	guess := int(float64(den.BitLen()-1) / math.Log2(5))
	for k := max(guess-1, 0); k <= guess+1; k++ {
		if new(big.Int).Exp(five, big.NewInt(int64(k)), nil).Cmp(den) == 0 {
			fives = k
			break
		}
	}
	if fives < 0 {
		return r.RatString()
	}
	// r = num / (2^twos * 5^fives) = num * 2^(scale-twos) * 5^(scale-fives) / 10^scale
	scale := max(twos, fives)
	num := new(big.Int).Abs(r.Num())
	num.Lsh(num, uint(scale-twos))
	num.Mul(num, new(big.Int).Exp(five, big.NewInt(int64(scale-fives)), nil))
	all := num.String()
	digits := strings.TrimRight(all, "0")
	if digits == "" {
		return "0"
	}
	exp := len(all) - len(digits) - scale // value = digits * 10^exp
	point := len(digits) + exp            // digits before the decimal point
	var out string
	switch {
	case exp >= 0 && point <= 21:
		out = digits + strings.Repeat("0", exp)
	case point > 0 && point <= 21:
		out = digits[:point] + "." + digits[point:]
	case point > -6 && point <= 0:
		out = "0." + strings.Repeat("0", -point) + digits
	default:
		out = digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		e := point - 1
		if e >= 0 {
			out += "e+" + strconv.Itoa(e)
		} else {
			out += "e-" + strconv.Itoa(-e)
		}
	}
	if r.Sign() < 0 {
		out = "-" + out
	}
	return out
}

func pointer(tokens []string) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteByte('/')
		tok = strings.ReplaceAll(tok, "~", "~0")
		b.WriteString(strings.ReplaceAll(tok, "/", "~1"))
	}
	return b.String()
}

// comparePointers orders JSON Pointers token by token, with array indexes
// (and any other all-digit tokens) in numeric order, so /items/2 sorts
// before /items/10.
func comparePointers(a, b string) int {
	at, bt := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(at) && i < len(bt); i++ {
		if c := compareTokens(at[i], bt[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(at), len(bt))
}

func compareTokens(a, b string) int {
	if isDigits(a) && isDigits(b) {
		ta, tb := strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		if c := cmp.Or(cmp.Compare(len(ta), len(tb)), strings.Compare(ta, tb)); c != 0 {
			return c
		}
	}
	return strings.Compare(a, b)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
