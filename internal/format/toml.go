package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

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
// precision).
func JSONToTOML(w io.Writer, r io.Reader) error {
	data, err := readInput(r)
	if err != nil {
		return err
	}
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
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(value); err != nil {
		return fmt.Errorf("converting to TOML: %w", err)
	}
	if _, err := w.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
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

var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func quoteKey(k string) string {
	if bareKey.MatchString(k) {
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
