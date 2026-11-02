package format

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// maxTOMLDepth caps the nesting of the JSON a TOML document converts to,
// counting the root table as one level: the most encoding/json accepts
// (from Go 1.27), whatever the toolchain.
const maxTOMLDepth = 10_000

// tomlKind is what a key holds, which decides how later expressions may
// extend it. The rules are TOML's, and they match go-toml's own decoder.
type tomlKind uint8

const (
	// tomlKindValue is a scalar, an array, or an inline table, none of
	// which can be extended.
	tomlKindValue tomlKind = iota
	// tomlKindDotted is a table created by a dotted key. Later dotted keys
	// in the same table extend it, and headers can add sub-tables to it,
	// but a header cannot define it.
	tomlKindDotted
	// tomlKindTable is a table created by a [header], either its own or a
	// longer one that names it as an intermediate part.
	tomlKindTable
	// tomlKindArray is an array of tables, created by [[header]].
	tomlKindArray
)

type tomlEntry struct {
	kind tomlKind
	// explicit marks a tomlKindTable defined by its own header, which no
	// other header may define again.
	explicit bool
	// inline marks a tomlKindValue that is an inline table, for errors.
	inline bool
	// table is the table of a tomlKindDotted or tomlKindTable entry, and
	// the last element of a tomlKindArray, which headers extend.
	table  *tomlTable
	tables []*tomlTable
	value  any
}

type tomlTable struct {
	path    *tomlPath
	depth   int
	entries map[string]*tomlEntry
}

// tomlPath is where a table or value sits, for errors. It links to its
// parent so that each level costs one small node, and the path is rendered
// only when an error names it.
type tomlPath struct {
	parent *tomlPath
	key    string
	// index is the position of an array element, which has no key, or -1.
	index int
}

func (p *tomlPath) child(name string) *tomlPath {
	return &tomlPath{parent: p, key: name, index: -1}
}

func (p *tomlPath) elem(i int) *tomlPath {
	return &tomlPath{parent: p, index: i}
}

// String renders the path for an error, such as servers[1].ports, keeping
// its end when it is too long to echo whole.
func (p *tomlPath) String() string {
	var segs []string
	for ; p != nil; p = p.parent {
		if p.index >= 0 {
			segs = append(segs, fmt.Sprintf("[%d]", p.index))
		} else {
			segs = append(segs, quoteKey(p.key))
		}
	}
	slices.Reverse(segs)
	return elideStart(jsonPath(segs))
}

// tomlDecoder converts go-toml's syntax tree to JSON values, one top-level
// expression at a time. It replaces toml.Unmarshal, whose duplicate-key
// tracker searches every key seen so far for each new one (quadratic in the
// number of keys) and which builds any nesting a document spells out with
// dotted keys before anything can check its depth.
type tomlDecoder struct {
	p        unstable.Parser
	data     []byte
	maxDepth int
}

// tomlDocument converts the TOML document in data to the JSON value
// TOMLToJSON writes.
func tomlDocument(data []byte, maxDepth int) (map[string]any, error) {
	d := &tomlDecoder{data: data, maxDepth: maxDepth}
	d.p.Reset(data)
	root := &tomlTable{depth: 1, entries: make(map[string]*tomlEntry)}
	current := root
	for d.p.NextExpression() {
		expr := d.p.Expression()
		var err error
		switch expr.Kind {
		case unstable.KeyValue:
			err = d.keyValue(current, expr)
		case unstable.Table:
			current, err = d.header(root, expr)
		case unstable.ArrayTable:
			current, err = d.arrayHeader(root, expr)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := d.p.Error(); err != nil {
		return nil, d.parserError(err)
	}
	return root.json(), nil
}

// header handles a [table] header and returns the table it defines.
func (d *tomlDecoder) header(root *tomlTable, expr *unstable.Node) (*tomlTable, error) {
	t, part, err := d.headerParent(root, expr.Key())
	if err != nil {
		return nil, err
	}
	name := string(part.Data)
	e := t.entries[name]
	if e == nil {
		child, err := d.newTable(d.raw(part), t.path.child(name), t.depth+1)
		if err != nil {
			return nil, err
		}
		t.entries[name] = &tomlEntry{kind: tomlKindTable, explicit: true, table: child}
		return child, nil
	}
	switch {
	case e.kind == tomlKindTable && !e.explicit:
		e.explicit = true
		return e.table, nil
	case e.kind == tomlKindTable:
		return nil, d.errorf(d.raw(part), "table [%s] is defined more than once", t.path.child(name))
	case e.kind == tomlKindDotted:
		return nil, d.errorf(d.raw(part), "table [%s] is already defined by dotted keys", t.path.child(name))
	case e.kind == tomlKindArray:
		return nil, d.errorf(d.raw(part), "table [%s] is already an array of tables", t.path.child(name))
	default:
		return nil, d.errorf(d.raw(part), "key %s is already %s, not a table", t.path.child(name), e.describe())
	}
}

// arrayHeader handles an [[array of tables]] header and returns the new
// element it adds.
func (d *tomlDecoder) arrayHeader(root *tomlTable, expr *unstable.Node) (*tomlTable, error) {
	t, part, err := d.headerParent(root, expr.Key())
	if err != nil {
		return nil, err
	}
	name := string(part.Data)
	e := t.entries[name]
	if e != nil && e.kind != tomlKindArray {
		return nil, d.errorf(d.raw(part), "key %s is already %s, not an array of tables", t.path.child(name), e.describe())
	}
	index := 0
	if e != nil {
		index = len(e.tables)
	}
	elem, err := d.newTable(d.raw(part), t.path.child(name).elem(index), t.depth+2)
	if err != nil {
		return nil, err
	}
	if e == nil {
		e = &tomlEntry{kind: tomlKindArray}
		t.entries[name] = e
	}
	e.tables = append(e.tables, elem)
	e.table = elem
	return elem, nil
}

// headerParent walks the intermediate parts of a header's key from the root,
// creating the tables it names, and returns the table that holds the last
// part, and that part.
func (d *tomlDecoder) headerParent(root *tomlTable, key unstable.Iterator) (*tomlTable, *unstable.Node, error) {
	t := root
	for key.Next() {
		part := key.Node()
		if key.IsLast() {
			return t, part, nil
		}
		name := string(part.Data)
		e := t.entries[name]
		if e == nil {
			child, err := d.newTable(d.raw(part), t.path.child(name), t.depth+1)
			if err != nil {
				return nil, nil, err
			}
			e = &tomlEntry{kind: tomlKindTable, table: child}
			t.entries[name] = e
		} else if e.kind == tomlKindValue {
			return nil, nil, d.errorf(d.raw(part), "key %s is already %s, not a table", t.path.child(name), e.describe())
		}
		t = e.table
	}
	return nil, nil, errors.New("invalid TOML: header without a key")
}

// keyValue adds a key-value expression, whose key may be dotted, to t.
func (d *tomlDecoder) keyValue(t *tomlTable, kv *unstable.Node) error {
	key := kv.Key()
	for key.Next() {
		part := key.Node()
		name := string(part.Data)
		e := t.entries[name]
		if key.IsLast() {
			if e != nil {
				return d.errorf(d.raw(part), "key %s is already defined", t.path.child(name))
			}
			value := kv.Value()
			v, err := d.value(t.path, name, -1, value, t.depth+1, d.raw(part))
			if err != nil {
				return err
			}
			t.entries[name] = &tomlEntry{kind: tomlKindValue, inline: value.Kind == unstable.InlineTable, value: v}
			return nil
		}
		if e == nil {
			child, err := d.newTable(d.raw(part), t.path.child(name), t.depth+1)
			if err != nil {
				return err
			}
			e = &tomlEntry{kind: tomlKindDotted, table: child}
			t.entries[name] = e
		} else if e.kind != tomlKindDotted {
			return d.errorf(d.raw(part), "key %s is already %s, which dotted keys cannot extend", t.path.child(name), e.describe())
		}
		t = e.table
	}
	return errors.New("invalid TOML: key-value without a key")
}

// value converts a value node: the value of key name under parent, or of
// element index when index is not -1. depth is the nesting level the value
// occupies if it is an array or an inline table; at is where errors about
// its nesting point, since arrays carry no position of their own. The
// value's own path is only built for those two, which are all that need it.
func (d *tomlDecoder) value(parent *tomlPath, name string, index int, n *unstable.Node, depth int, at []byte) (any, error) {
	switch n.Kind {
	case unstable.InlineTable:
		at = d.raw(n)
		t, err := d.newTable(at, valuePath(parent, name, index), depth)
		if err != nil {
			return nil, err
		}
		children := n.Children()
		for children.Next() {
			if err := d.keyValue(t, children.Node()); err != nil {
				return nil, err
			}
		}
		return t.json(), nil
	case unstable.Array:
		if depth > d.maxDepth {
			return nil, d.depthError(at)
		}
		path := valuePath(parent, name, index)
		out := []any{}
		children := n.Children()
		for i := 0; children.Next(); i++ {
			v, err := d.value(path, "", i, children.Node(), depth+1, at)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	default:
		return d.scalar(n)
	}
}

func valuePath(parent *tomlPath, name string, index int) *tomlPath {
	if index >= 0 {
		return parent.elem(index)
	}
	return parent.child(name)
}

func (d *tomlDecoder) scalar(n *unstable.Node) (any, error) {
	switch n.Kind {
	case unstable.String:
		return string(n.Data), nil
	case unstable.Bool:
		return n.Data[0] == 't', nil
	case unstable.Integer:
		return d.integer(n.Data)
	case unstable.Float:
		return d.float(n.Data)
	case unstable.DateTime:
		return d.dateTime(n.Data)
	case unstable.LocalDateTime:
		return d.localText(&toml.LocalDateTime{}, n.Data)
	case unstable.LocalDate:
		return d.localText(&toml.LocalDate{}, n.Data)
	case unstable.LocalTime:
		return d.localText(&toml.LocalTime{}, n.Data)
	default:
		return nil, d.errorf(n.Data, "unexpected %s value", n.Kind)
	}
}

// localText decodes a local date, time, or date-time into its TOML text,
// which go-toml normalizes (a space separator becomes T, and fractional
// seconds past nanoseconds are dropped).
func (d *tomlDecoder) localText(v interface {
	UnmarshalText([]byte) error
	fmt.Stringer
}, raw []byte) (string, error) {
	if err := v.UnmarshalText(raw); err != nil {
		return "", d.textError(err, raw)
	}
	return v.String(), nil
}

// integer decodes an integer whose syntax (sign, prefix, digits, and
// underscores) the parser has already checked.
func (d *tomlDecoder) integer(raw []byte) (int64, error) {
	digits, base := raw, 10
	if len(raw) > 2 && raw[0] == '0' {
		switch raw[1] {
		case 'x':
			base = 16
		case 'o':
			base = 8
		case 'b':
			base = 2
		}
		if base != 10 {
			digits = raw[2:]
		}
	}
	n, err := strconv.ParseInt(string(bytes.ReplaceAll(digits, []byte("_"), nil)), base, 64)
	if err != nil {
		return 0, d.errorf(raw, "integer %s does not fit in a 64-bit signed integer", elideEnd(string(raw)))
	}
	return n, nil
}

// float decodes a float whose syntax the parser has already checked.
func (d *tomlDecoder) float(raw []byte) (float64, error) {
	if word := bytes.TrimLeft(raw, "+-"); string(word) == "inf" || string(word) == "nan" {
		line, column := d.position(raw)
		return 0, fmt.Errorf("value at line %d, column %d is %s, which JSON cannot represent", line, column, raw)
	}
	f, err := strconv.ParseFloat(string(bytes.ReplaceAll(raw, []byte("_"), nil)), 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) && math.IsInf(f, 0) {
			return 0, d.errorf(raw, "float %s is outside the 64-bit range", elideEnd(string(raw)))
		}
		return 0, d.errorf(raw, "invalid float %s", elideEnd(string(raw)))
	}
	return f, nil
}

// dateTime decodes an offset date-time into RFC 3339, with a zero offset
// written as Z.
func (d *tomlDecoder) dateTime(raw []byte) (string, error) {
	local, offset := raw, 0
	switch last := raw[len(raw)-1]; {
	case last == 'Z' || last == 'z':
		local = raw[:len(raw)-1]
	case len(raw) > 6 && (raw[len(raw)-6] == '+' || raw[len(raw)-6] == '-') && raw[len(raw)-3] == ':':
		zone := raw[len(raw)-6:]
		hours, hok := twoDigits(zone[1:3])
		minutes, mok := twoDigits(zone[4:6])
		if !hok || !mok || hours > 23 || minutes > 59 {
			return "", d.errorf(zone, "invalid time zone offset %s", zone)
		}
		offset = hours*3600 + minutes*60
		if zone[0] == '-' {
			offset = -offset
		}
		local = raw[:len(raw)-6]
	default:
		// Point at the offset: past the date, whose hyphens are not one.
		at := raw[len(raw):]
		if i := bytes.LastIndexAny(raw, "+-"); i > len("2006-01-02") {
			at = raw[i:]
		}
		return "", d.errorf(at, "date-time %s has no valid time zone offset", elideEnd(string(raw)))
	}
	var v toml.LocalDateTime
	if err := v.UnmarshalText(local); err != nil {
		return "", d.textError(err, raw)
	}
	zone := time.UTC
	if offset != 0 {
		zone = time.FixedZone("", offset)
	}
	return v.AsTime(zone).Format(time.RFC3339Nano), nil
}

// twoDigits parses exactly two decimal digits, which strconv.Atoi does not
// guarantee: it accepts a sign.
func twoDigits(b []byte) (int, bool) {
	if b[0] < '0' || b[0] > '9' || b[1] < '0' || b[1] > '9' {
		return 0, false
	}
	return int(b[0]-'0')*10 + int(b[1]-'0'), true
}

// newTable creates the table at path, failing with an error at the start
// of at if it nests past the depth limit.
func (d *tomlDecoder) newTable(at []byte, path *tomlPath, depth int) (*tomlTable, error) {
	if depth > d.maxDepth {
		return nil, d.depthError(at)
	}
	return &tomlTable{path: path, depth: depth, entries: make(map[string]*tomlEntry)}, nil
}

func (d *tomlDecoder) depthError(at []byte) error {
	return d.errorf(at, "nesting exceeds the depth limit of %d levels", d.maxDepth)
}

// raw returns the input bytes a node spans, for error positions.
func (d *tomlDecoder) raw(n *unstable.Node) []byte {
	return d.p.Raw(n.Raw)
}

// textError positions an error from a go-toml UnmarshalText method, whose
// highlight is a subslice of raw.
func (d *tomlDecoder) textError(err error, raw []byte) error {
	if err == nil {
		return nil
	}
	var perr *unstable.ParserError
	if errors.As(err, &perr) {
		return d.errorf(perr.Highlight, "%s", perr.Message)
	}
	return d.errorf(raw, "%v", err)
}

func (d *tomlDecoder) parserError(err error) error {
	var perr *unstable.ParserError
	if errors.As(err, &perr) {
		return d.errorf(perr.Highlight, "%s", perr.Message)
	}
	return fmt.Errorf("invalid TOML: %w", err)
}

// errorf reports invalid TOML at the start of at, a subslice of the input.
func (d *tomlDecoder) errorf(at []byte, format string, args ...any) error {
	line, column := d.position(at)
	return fmt.Errorf("invalid TOML at line %d, column %d: %s", line, column, fmt.Sprintf(format, args...))
}

// position returns the line and byte column where at, a subslice of the
// input, starts. A slice's capacity runs to the end of its backing array, so
// the difference in capacities is its offset.
func (d *tomlDecoder) position(at []byte) (line, column int) {
	offset := cap(d.data) - cap(at)
	if offset < 0 || offset > len(d.data) || (offset < len(d.data) && len(at) > 0 && &d.data[offset] != &at[0]) {
		offset = len(d.data)
	}
	before := d.data[:offset]
	return bytes.Count(before, []byte("\n")) + 1, offset - (bytes.LastIndexByte(before, '\n') + 1) + 1
}

// maxEchoed caps how much of a key path or literal an error repeats: errors
// land in an agent's context, and either can be megabytes long.
const maxEchoed = 100

// elideEnd shortens s to its first maxEchoed bytes, at a character boundary.
func elideEnd(s string) string {
	if len(s) <= maxEchoed {
		return s
	}
	cut := maxEchoed
	for !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

// elideStart shortens s to its last maxEchoed bytes, at a character boundary.
func elideStart(s string) string {
	if len(s) <= maxEchoed {
		return s
	}
	cut := len(s) - maxEchoed
	for !utf8.RuneStart(s[cut]) {
		cut++
	}
	return "..." + s[cut:]
}

func (e *tomlEntry) describe() string {
	switch e.kind {
	case tomlKindDotted:
		return "a table defined by dotted keys"
	case tomlKindTable:
		return "a table"
	case tomlKindArray:
		return "an array of tables"
	}
	if e.inline {
		return "an inline table"
	}
	return "a value"
}

func (t *tomlTable) json() map[string]any {
	out := make(map[string]any, len(t.entries))
	for name, e := range t.entries {
		switch e.kind {
		case tomlKindValue:
			out[name] = e.value
		case tomlKindArray:
			elems := make([]any, len(e.tables))
			for i, elem := range e.tables {
				elems[i] = elem.json()
			}
			out[name] = elems
		default:
			out[name] = e.table.json()
		}
	}
	return out
}
