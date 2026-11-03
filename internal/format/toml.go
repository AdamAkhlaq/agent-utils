package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

// TOMLToJSON converts the TOML document in r to pretty-printed JSON in w.
// Object keys are emitted in sorted order (TOML tables have no order worth
// keeping, so sorting is what makes the output deterministic). TOML
// datetimes become strings: offset date-times in RFC 3339, local
// date-times, dates, and times as their TOML literal text. Nesting deeper
// than maxTOMLDepth levels is an error.
func TOMLToJSON(w io.Writer, r io.Reader) error {
	data, err := ReadText(r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("empty input")
	}
	doc, err := tomlDocument(data, maxTOMLDepth)
	if err != nil {
		return err
	}
	return writeJSON(w, doc, len(data))
}

// JSONToTOML converts the JSON document in r to TOML in w, the inverse of
// TOMLToJSON. Keys are emitted in sorted order. Integers stay integers and
// floats stay floats (1.0 encodes as 1.0, not 1). Values TOML cannot
// represent fail loudly: a non-object root, any null, and integers outside
// the 64-bit signed range (converting those to floats would silently lose
// precision). Output is limited as IndentJSON limits it.
func JSONToTOML(w io.Writer, r io.Reader) error {
	return jsonToTOML(w, r, outputFloor)
}

// jsonToTOML is JSONToTOML with the output limit's floor as a parameter, so
// tests can reach its edge without writing 64 MiB.
func jsonToTOML(w io.Writer, r io.Reader, floor int) error {
	text, err := ReadText(r)
	if err != nil {
		return err
	}
	data := trimTrailingSpace(text)
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}
	// Check the syntax first: the decoder below would stop after the first
	// value and silently ignore trailing garbage.
	if err := checkSyntax(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return syntaxError(data, err)
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return fmt.Errorf("cannot convert to TOML: the root must be a JSON object, got %s", jsonTypeName(doc))
	}
	value, err := tomlValue(root, nil)
	if err != nil {
		return err
	}
	limit := outputLimit(floor, len(text))
	headers := tomlHeaders{limit: limit}
	headers.table(value.(map[string]any), 0, 0)
	if headers.over() {
		return headers.limitError(floor)
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(value); err != nil {
		return fmt.Errorf("converting to TOML: %w", err)
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if int64(len(out)) > limit {
		return headers.limitError(floor)
	}
	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// tomlHeaders counts the bytes of the table headers go-toml's encoder writes
// for a document, line breaks included, without encoding it. Each header
// spells out its table's full dotted key, so headers are the only output
// that grows faster than the input: 60 KB of objects nested 10,000 deep need
// 100 MB of them. The rest of the output is in proportion to the input, so
// once the headers fit, encoding costs memory in proportion to the limit, and
// the encoded size is checked exactly. go-toml writes every object reached
// through object keys as a table, and every element of an array holding only
// objects as an [[array table]]; objects anywhere else are inline and have no
// header.
type tomlHeaders struct {
	limit   int64
	size    int64 // the headers' bytes, counted until they pass limit+1
	count   int   // the number of headers
	longest int64 // the bytes in the longest header, without its line break
	deepest int   // the most keys in one header
}

// table counts the headers below the table m, whose own dotted key is
// keyLen bytes long and depth keys deep.
func (h *tomlHeaders) table(m map[string]any, keyLen int64, depth int) {
	for k, v := range m {
		n := keyLen + tomlKeyLen(k)
		if depth > 0 {
			n++ // the dot before k
		}
		switch v := v.(type) {
		case map[string]any:
			h.add(n+int64(len("[]")), depth+1)
			h.table(v, n, depth+1)
		case []any:
			if !arrayOfTables(v) {
				continue
			}
			for _, e := range v {
				h.add(n+int64(len("[[]]")), depth+1)
				h.table(e.(map[string]any), n, depth+1)
			}
		}
	}
}

// add counts a header of n bytes and keys keys, and its line break. The size
// stops growing once it passes limit+1, so it cannot overflow.
func (h *tomlHeaders) add(n int64, keys int) {
	h.count++
	h.longest = max(h.longest, n)
	h.deepest = max(h.deepest, keys)
	if h.size <= h.limit+1 {
		h.size += n + 1
	}
}

// over reports whether the headers alone make the output larger than the
// limit. The output's final line break, which may end a header, is left to
// the CLI, so it does not count.
func (h *tomlHeaders) over() bool {
	return h.size-1 > h.limit
}

func (h *tomlHeaders) limitError(floor int) error {
	return limitError("TOML output", floor, h.limit, fmt.Sprintf("the document has %s, and each table's header spells out its full dotted key, up to %s and %d bytes long",
		plural(h.count, "table"), plural(h.deepest, "key"), h.longest))
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// tomlKeyLen returns the length of k as go-toml writes it in a header, by
// go-toml's own rules: bare if it can be, otherwise as a literal string if
// it can be, and otherwise as a basic string with escapes.
func tomlKeyLen(k string) int64 {
	if isBareKey(k) {
		return int64(len(k))
	}
	literal := utf8.ValidString(k)
	n := int64(2)
	for i := 0; i < len(k); {
		c := k[i]
		switch {
		case c == '\'':
			literal = false
			n++
			i++
		case c == '"' || c == '\\':
			n += 2
			i++
		case c == '\b' || c == '\f' || c == '\n' || c == '\r' || c == '\t':
			literal = false
			n += 2
			i++
		case c < 0x20 || c == 0x7f:
			literal = false
			n += int64(len(`\u0000`))
			i++
		default:
			r, size := utf8.DecodeRuneInString(k[i:])
			if r == utf8.RuneError && size == 1 {
				n += int64(len(`\uFFFD`))
			} else {
				n += int64(size)
			}
			i += size
		}
	}
	if literal {
		return int64(len(k)) + 2
	}
	return n
}

// isBareKey reports whether k can be a TOML key without quotes.
func isBareKey(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func arrayOfTables(a []any) bool {
	if len(a) == 0 {
		return false
	}
	for _, e := range a {
		if _, ok := e.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// tomlValue vets a decoded JSON tree for TOML and rewrites json.Number into
// int64 or float64. The checks exist because go-toml's encoder silently drops
// nil values instead of erroring (verified empirically), and json.Number
// would otherwise reach it as a plain string.
func tomlValue(v any, path []string) (any, error) {
	switch v := v.(type) {
	case nil:
		return nil, fmt.Errorf("cannot convert to TOML: null value at %s (TOML has no null)", jsonPath(path))
	case map[string]any:
		out := make(map[string]any, len(v))
		// Sorted keys so that when several values are unrepresentable, the
		// reported path is always the same one.
		for _, k := range slices.Sorted(maps.Keys(v)) {
			converted, err := tomlValue(v[k], append(path, quoteKey(k)))
			if err != nil {
				return nil, err
			}
			out[k] = converted
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			converted, err := tomlValue(e, append(path, fmt.Sprintf("[%d]", i)))
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	case json.Number:
		s := v.String()
		if !strings.ContainsAny(s, ".eE") {
			n, err := v.Int64()
			if err != nil {
				return nil, fmt.Errorf("cannot convert to TOML: integer %s at %s overflows TOML's 64-bit signed range", s, jsonPath(path))
			}
			return n, nil
		}
		f, err := v.Float64()
		if err != nil {
			return nil, fmt.Errorf("cannot convert to TOML: number %s at %s does not fit a 64-bit float", s, jsonPath(path))
		}
		return f, nil
	default:
		return v, nil
	}
}

func jsonTypeName(v any) string {
	switch v.(type) {
	case []any:
		return "an array"
	case string:
		return "a string"
	case json.Number:
		return "a number"
	case bool:
		return "a boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func quoteKey(k string) string {
	if isBareKey(k) {
		return k
	}
	return fmt.Sprintf("%q", k)
}

// jsonPath renders a path like item.tags[2] for error messages. Segments are
// pre-rendered: quoted keys and [i] indexes.
func jsonPath(path []string) string {
	if len(path) == 0 {
		return "the document root"
	}
	var b strings.Builder
	for i, seg := range path {
		if i > 0 && !strings.HasPrefix(seg, "[") {
			b.WriteByte('.')
		}
		b.WriteString(seg)
	}
	return b.String()
}
