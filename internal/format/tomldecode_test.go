package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

// tomlTableTests are shared with FuzzTOMLToJSON as seeds.
var tomlTableTests = []struct {
	name    string
	input   string
	want    string
	wantErr string
}{
	{name: "dotted keys extend a dotted table", input: "a.b = 1\na.c = 2", want: `{"a":{"b":1,"c":2}}`},
	{name: "header adds a sub-table to a dotted table", input: "[fruit]\napple.color = 'red'\n[fruit.apple.texture]\nsmooth = true", want: `{"fruit":{"apple":{"color":"red","texture":{"smooth":true}}}}`},
	{name: "header defines a table an earlier header implied", input: "[a.b]\nc = 1\n[a]\nd = 2", want: `{"a":{"b":{"c":1},"d":2}}`},
	{name: "arrays of tables", input: "[[a]]\nb = 1\n[a.c]\nd = 1\n[[a]]\nb = 2\n[a.c]\nd = 2", want: `{"a":[{"b":1,"c":{"d":1}},{"b":2,"c":{"d":2}}]}`},
	{name: "header under the last element of an array of tables", input: "[[a]]\n[[a.b]]\nc = 1\n[[a.b]]\nc = 2", want: `{"a":[{"b":[{"c":1},{"c":2}]}]}`},
	{name: "dotted keys in an inline table", input: "a = {b.c = 1, b.d = 2}", want: `{"a":{"b":{"c":1,"d":2}}}`},
	{name: "inline tables in an array", input: "a = [{b = 1}, {b = 1}]", want: `{"a":[{"b":1},{"b":1}]}`},
	{name: "quoted keys", input: "\"a.b\" = 1\nc.'d e' = 2", want: `{"a.b":1,"c":{"d e":2}}`},

	{name: "duplicate key", input: "a = 1\na = 2", wantErr: "invalid TOML at line 2, column 1: key a is already defined"},
	{name: "duplicate key in a table", input: "[s]\nport = 1\nport = 2", wantErr: "line 3, column 1: key s.port is already defined"},
	{name: "table defined twice", input: "[a]\n[a]", wantErr: "line 2, column 2: table [a] is defined more than once"},
	{name: "header over a dotted table", input: "[a]\nb.c = 1\n[a.b]", wantErr: "line 3, column 4: table [a.b] is already defined by dotted keys"},
	{name: "header over an array of tables", input: "[[a]]\n[a]", wantErr: "table [a] is already an array of tables"},
	{name: "header over a value", input: "a = 1\n[a]", wantErr: "key a is already a value, not a table"},
	{name: "header through a value", input: "a = 1\n[a.b]", wantErr: "key a is already a value, not a table"},
	{name: "header over an inline table", input: "a = {}\n[a]", wantErr: "key a is already an inline table, not a table"},
	{name: "header through an inline table", input: "a = {b = 1}\n[a.c]", wantErr: "key a is already an inline table, not a table"},
	{name: "array header over a table", input: "[a]\n[[a]]", wantErr: "key a is already a table, not an array of tables"},
	{name: "array header over a static array", input: "a = []\n[[a]]", wantErr: "key a is already a value, not an array of tables"},
	{name: "dotted key into a header table", input: "[a.b]\n[a]\nb.c = 1", wantErr: "line 3, column 1: key a.b is already a table, which dotted keys cannot extend"},
	{name: "dotted key into an inline table", input: "a = {b = 1}\na.c = 2", wantErr: "key a is already an inline table, which dotted keys cannot extend"},
	{name: "dotted key over a value", input: "a = 1\na.b = 2", wantErr: "key a is already a value, which dotted keys cannot extend"},
	{name: "duplicate key in an inline table", input: "a = {b = 1, b = 2}", wantErr: "key a.b is already defined"},
	{name: "duplicate key in an inline table in an array", input: "a = [{}, {b.c = 1, b = 2}]", wantErr: `key a[1].b is already defined`},
	{name: "duplicate key in an array of tables names the element", input: "[[x]]\n[[x]]\ny = 1\ny = 2", wantErr: "line 4, column 1: key x[1].y is already defined"},
	{name: "quoted key in a path", input: "[\"a b\"]\nc = 1\nc = 2", wantErr: `key "a b".c is already defined`},
}

// TestTOMLTables covers TOML's rules for defining and extending tables, which
// tomlDocument enforces itself instead of leaving them to toml.Unmarshal.
func TestTOMLTables(t *testing.T) {
	for _, tt := range tomlTableTests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tomlDocument([]byte(tt.input), maxTOMLDepth)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("tomlDocument() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("tomlDocument() error = %v", err)
			}
			if got := compactJSON(t, doc); got != tt.want {
				t.Errorf("tomlDocument() = %s, want %s", got, tt.want)
			}
		})
	}
}

// tomlScalarTests are shared with FuzzTOMLToJSON as seeds.
var tomlScalarTests = []struct {
	name    string
	input   string
	want    string
	wantErr string
}{
	{name: "integers in every base", input: "a = [+1_000, -0, 0xDEAD_beef, 0o755, 0b1010, 9223372036854775807, -9223372036854775808]", want: `[1000,0,3735928559,493,10,9223372036854775807,-9223372036854775808]`},
	{name: "floats", input: "a = [1.5, -0.0, 6.626e-34, 1_000.000_1, 5e+22, 1e-400]", want: `[1.5,-0,6.626e-34,1000.0001,5e+22,0]`},
	{name: "booleans and strings", input: "a = [true, false, \"\\u00e9\\t\", 'C:\\x', \"\"\"\nline\"\"\"]", want: `[true,false,"é\t","C:\\x","line"]`},
	{name: "offset date-times", input: "a = [1979-05-27T07:32:00Z, 1979-05-27 07:32:00.999999999999-07:00, 1979-05-27T00:32:00+00:00, 1979-05-27T00:32:00-00:00, 1979-05-27t07:32:00+05:30]", want: `["1979-05-27T07:32:00Z","1979-05-27T07:32:00.999999999-07:00","1979-05-27T00:32:00Z","1979-05-27T00:32:00Z","1979-05-27T07:32:00+05:30"]`},
	{name: "local date-times, dates, and times", input: "a = [1979-05-27T07:32:00, 1979-05-27 07:32:00.5, 1979-05-27, 07:32:00.1230, 00:00:00]", want: `["1979-05-27T07:32:00","1979-05-27T07:32:00.5","1979-05-27","07:32:00.1230","00:00:00"]`},

	{name: "decimal overflow", input: "a = 9223372036854775808", wantErr: "line 1, column 5: integer 9223372036854775808 does not fit in a 64-bit signed integer"},
	{name: "negative overflow", input: "a = -9223372036854775809", wantErr: "integer -9223372036854775809 does not fit"},
	{name: "hexadecimal overflow", input: "a = 0x8000000000000000", wantErr: "integer 0x8000000000000000 does not fit"},
	{name: "float out of range", input: "a = 1e400", wantErr: "line 1, column 5: float 1e400 is outside the 64-bit range"},
	{name: "infinity", input: "a = -inf", wantErr: "value at line 1, column 5 is -inf, which JSON cannot represent"},
	{name: "not a number", input: "a = 1\nb = [nan]", wantErr: "value at line 2, column 6 is nan, which JSON cannot represent"},
	{name: "impossible date", input: "a = 2023-02-29", wantErr: "line 1, column 13: impossible date"},
	{name: "impossible offset", input: "a = 1979-05-27T07:32:00+24:00", wantErr: "invalid time zone offset +24:00"},
	{name: "offset without minutes", input: "a = 1997-09-09T09:09:09.09+09", wantErr: "line 1, column 27: date-time 1997-09-09T09:09:09.09+09 has no valid time zone offset"},
	{name: "offset with a sign in its minutes", input: "a = 0000-01-01 00:00+00:+0", wantErr: "line 1, column 21: invalid time zone offset +00:+0"},
	{name: "syntax error", input: "a = 1\nb = ?", wantErr: "invalid TOML at line 2, column 5: "},
	{name: "leading zero", input: "a = 012", wantErr: "line 1, column 5: "},
}

func TestTOMLScalars(t *testing.T) {
	for _, tt := range tomlScalarTests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tomlDocument([]byte(tt.input), maxTOMLDepth)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("tomlDocument() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("tomlDocument() error = %v", err)
			}
			if got := compactJSON(t, doc["a"]); got != tt.want {
				t.Errorf("tomlDocument() a = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTOMLDepthLimit(t *testing.T) {
	const limit = 4
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "dotted key at the limit", input: "a.b.c = {}"},
		{name: "dotted key past the limit", input: "a.b.c.d = {}", wantErr: "line 1, column 11: nesting exceeds the depth limit of 4 levels"},
		{name: "dotted key with a scalar at the limit", input: "a.b.c.d = 1"},
		{name: "dotted key past the limit with a scalar", input: "a.b.c.d.e = 1", wantErr: "line 1, column 7: nesting exceeds the depth limit"},
		{name: "header at the limit", input: "[a.b.c]"},
		{name: "header past the limit", input: "x = 1\n[a.b.c.d]", wantErr: "line 2, column 8: nesting exceeds the depth limit"},
		{name: "array header at the limit", input: "[a]\n[[a.b]]"},
		{name: "array header past the limit", input: "[a.b]\n[[a.b.c]]", wantErr: "line 2, column 7: nesting exceeds the depth limit"},
		{name: "header then dotted key at the limit", input: "[a.b]\nc.d = 1"},
		{name: "header then dotted key past the limit", input: "[a.b]\nc.d.e = 1", wantErr: "line 2, column 3: nesting exceeds the depth limit"},
		{name: "array value at the limit", input: "a = [[[1]]]"},
		{name: "array value past the limit", input: "a = [[[[1]]]]", wantErr: "line 1, column 1: nesting exceeds the depth limit"},
		{name: "inline tables past the limit", input: "a = {b = {c = {d = {}}}}", wantErr: "line 1, column 20: nesting exceeds the depth limit"},
		{name: "array of tables under arrays past the limit", input: "[[a]]\n[[a.b]]", wantErr: "line 2, column 5: nesting exceeds the depth limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tomlDocument([]byte(tt.input), limit)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("tomlDocument() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("tomlDocument() error = %v", err)
			}
			out := compactJSON(t, doc)
			if _, nesting := indentedSize([]byte(out), 0); nesting > limit {
				t.Errorf("tomlDocument() = %s nests %d levels, past the limit of %d", out, nesting, limit)
			}
		})
	}
}

func TestTOMLToJSONDepthLimit(t *testing.T) {
	segments := func(n int) string { return strings.TrimSuffix(strings.Repeat("a.", n), ".") }
	atLimit := segments(maxTOMLDepth-1) + " = {}"
	doc, err := tomlDocument([]byte(atLimit), maxTOMLDepth)
	if err != nil {
		t.Fatalf("tomlDocument() at the depth limit error = %v", err)
	}
	if _, err := json.Marshal(doc); err != nil {
		t.Fatalf("json.Marshal() of a document at the depth limit error = %v", err)
	}
	tests := map[string]string{
		"dotted key one level past the limit": segments(maxTOMLDepth) + " = {}",
		"header one level past the limit":     "[" + segments(maxTOMLDepth) + "]",
		"dotted key of five million segments": segments(5_000_000) + " = 1",
		"header of five million segments":     "[" + segments(5_000_000) + "]",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := TOMLToJSON(&out, strings.NewReader(input))
			want := fmt.Sprintf("nesting exceeds the depth limit of %d levels", maxTOMLDepth)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("TOMLToJSON() error = %v, want it to contain %q", err, want)
			}
			if out.Len() != 0 {
				t.Errorf("TOMLToJSON() wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// TestTOMLErrorsEchoLittle checks that errors repeat at most the start of a
// long literal and the end of a long key path, since errors land in an
// agent's context.
func TestTOMLErrorsEchoLittle(t *testing.T) {
	long := strings.Repeat("9", 1_000_000)
	deep := strings.Repeat("k.", 5000) + "k"
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "long integer", input: "a = " + long, wantErr: "integer " + strings.Repeat("9", maxEchoed) + "... does not fit"},
		{name: "long float", input: "a = 1e" + long, wantErr: "float 1e" + strings.Repeat("9", maxEchoed-2) + "... is outside the 64-bit range"},
		{name: "deep key path", input: deep + " = 1\n" + deep + " = 2", wantErr: "key ..." + strings.Repeat(".k", maxEchoed/2) + " is already defined"},
		{name: "long multibyte key cut at a character", input: "\"" + strings.Repeat("é", 100) + "\" = 1\n\"" + strings.Repeat("é", 100) + "\" = 2", wantErr: "key ..." + strings.Repeat("é", 49) + "\" is already defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tomlDocument([]byte(tt.input), maxTOMLDepth)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("tomlDocument() error = %.300v, want it to contain %q", err, tt.wantErr)
			}
			if len(err.Error()) > 3*maxEchoed {
				t.Errorf("tomlDocument() error is %d bytes long", len(err.Error()))
			}
		})
	}
}

// TestTOMLDeepNestingCost bounds what deep arrays and inline tables cost.
// Each level must cost a constant amount: building an error path for every
// element once made 20 KB of nested arrays allocate about a gigabyte.
func TestTOMLDeepNestingCost(t *testing.T) {
	tests := map[string]string{
		"arrays":                  "a = " + strings.Repeat("[", 9998) + strings.Repeat("]", 9998),
		"inline tables":           "a = " + strings.Repeat("{b = ", 9998) + "1" + strings.Repeat("}", 9998),
		"arrays of many elements": "a = " + strings.Repeat("[", 5000) + strings.Repeat("1,", 50_000) + strings.Repeat("]", 5000),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			if _, err := tomlDocument([]byte(input), maxTOMLDepth); err != nil {
				t.Fatalf("tomlDocument() error = %v", err)
			}
			runtime.ReadMemStats(&after)
			// go-toml's parser alone allocates about 100 bytes per input
			// byte; the conversion may add as much again, not more.
			if allocated, limit := after.TotalAlloc-before.TotalAlloc, uint64(200*len(input)); allocated > limit {
				t.Errorf("tomlDocument() allocated %d bytes for %d bytes of input, want at most %d", allocated, len(input), limit)
			}
		})
	}
}

// goTOMLToJSON is the conversion tomlDocument replaced: toml.Unmarshal, with
// its datetime types rendered as strings. It is the reference for
// FuzzTOMLToJSON.
func goTOMLToJSON(input string) (string, error) {
	var doc map[string]any
	if err := toml.Unmarshal([]byte(input), &doc); err != nil {
		return "", err
	}
	out, err := json.Marshal(goTOMLValue(doc))
	return string(out), err
}

func goTOMLValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = goTOMLValue(e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = goTOMLValue(e)
		}
		return out
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case toml.LocalDateTime, toml.LocalDate, toml.LocalTime:
		return fmt.Sprint(v)
	default:
		return v
	}
}

func compactJSON(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(out)
}

// FuzzTOMLToJSON checks that tomlDocument accepts exactly the documents
// toml.Unmarshal accepts and converts them to the same JSON. The one
// difference is deliberate: inf and nan, which JSON cannot represent, fail
// in both, but toml.Unmarshal only fails when the result is encoded.
func FuzzTOMLToJSON(f *testing.F) {
	seeds := []string{
		"title = \"TOML\"\n[owner]\nname = 'Tom'\ndob = 1979-05-27T07:32:00-08:00\n[database]\nports = [ 8000, 8001, 8002 ]\ntemp = { cpu = 79.5, case = 72.0 }\n",
		"[fruit]\napple.color = 'red'\napple.taste.sweet = true\n[fruit.apple.texture]\nsmooth = true\n",
		"[[products]]\nname = 'Hammer'\n[[products]]\n[[products]]\nname = 'Nail'\n[products.dims]\nw = 1\n",
		"[a.b.c]\n[a]\nx = 1\n[a.b]\ny = 2\n",
		"a = [1, 'x', [2.5, {b.c = true}], {}]\n'q k'.\"r\" = 0x_1\n",
		"a = {b = 1}\na.c = 2\n",
		"[a]\nb.c = 1\n[a.b]\n",
		"a.b = 1\n[a]\n",
		"t = [07:32, 07:32:00.123456789123, 1979-05-27, 1979-05-27 07:32Z, 1979-05-27t07:32:00.5+01:00]\n",
		"n = [inf, -nan, +0.0, 1e400, -9_223_372_036_854_775_808, 0o777, 0b1]\n",
		"s = \"\\e\\x41\\U0001F600\"\nm = \"\"\"\\\n  x\"\"\"\nl = '''\nraw'''\n",
		"x = {\n  a = 1,\n  b = 2,\n}\n",
	}
	for _, tt := range tomlTableTests {
		seeds = append(seeds, tt.input)
	}
	for _, tt := range tomlScalarTests {
		seeds = append(seeds, tt.input)
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 || !utf8.ValidString(input) {
			return
		}
		doc, err := tomlDocument([]byte(input), maxTOMLDepth)
		want, wantErr := goTOMLToJSON(input)
		switch {
		case err != nil && wantErr != nil:
		case err != nil:
			t.Fatalf("tomlDocument(%q) error = %v, but toml.Unmarshal converts it to %s", input, err, want)
		case wantErr != nil:
			t.Fatalf("tomlDocument(%q) = %s, but toml.Unmarshal fails: %v", input, compactJSON(t, doc), wantErr)
		default:
			if got := compactJSON(t, doc); got != want {
				t.Fatalf("tomlDocument(%q) = %s, want %s", input, got, want)
			}
		}
	})
}
