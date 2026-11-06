package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestTOMLToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "keys sorted deterministically",
			input: "zebra = 1\nalpha = 2",
			want:  "{\n  \"alpha\": 2,\n  \"zebra\": 1\n}",
		},
		{
			name:  "nested tables and arrays",
			input: "[service]\nname = \"api\"\nports = [80, 443]\ndebug = false",
			want:  "{\n  \"service\": {\n    \"debug\": false,\n    \"name\": \"api\",\n    \"ports\": [\n      80,\n      443\n    ]\n  }\n}",
		},
		{
			name:  "array of tables",
			input: "[[products]]\nname = \"a\"\n\n[[products]]\nname = \"b\"",
			want:  "{\n  \"products\": [\n    {\n      \"name\": \"a\"\n    },\n    {\n      \"name\": \"b\"\n    }\n  ]\n}",
		},
		{
			name:  "integer stays integer, float stays float",
			input: "i = 3\nf = 3.5",
			want:  "{\n  \"f\": 3.5,\n  \"i\": 3\n}",
		},
		{
			name:  "max int64 survives exactly",
			input: "n = 9223372036854775807",
			want:  "{\n  \"n\": 9223372036854775807\n}",
		},
		{
			name:  "heterogeneous array (TOML 1.0)",
			input: "mixed = [1, \"two\", 3.5]",
			want:  "{\n  \"mixed\": [\n    1,\n    \"two\",\n    3.5\n  ]\n}",
		},
		{
			name:  "offset date-time becomes RFC 3339 string",
			input: "odt = 1979-05-27T00:32:00.999-07:00",
			want:  "{\n  \"odt\": \"1979-05-27T00:32:00.999-07:00\"\n}",
		},
		{
			name:  "local date-time, date, and time become literal strings",
			input: "ldt = 1979-05-27T07:32:00\nld = 1979-05-27\nlt = 07:32:00.500",
			want:  "{\n  \"ld\": \"1979-05-27\",\n  \"ldt\": \"1979-05-27T07:32:00\",\n  \"lt\": \"07:32:00.500\"\n}",
		},
		{
			name:  "html characters not escaped",
			input: "s = \"<a> & b\"",
			want:  "{\n  \"s\": \"<a> & b\"\n}",
		},
		{
			name:  "comment-only document is an empty object",
			input: "# only a comment",
			want:  "{}",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "invalid toml with position", input: "a = 1\nb = ?", wantErr: "invalid TOML at line 2, column 5"},
		{name: "duplicate key", input: "a = 1\na = 2", wantErr: "already defined"},
		{name: "integer overflow rejected at parse", input: "n = 92233720368547758080", wantErr: "integer 92233720368547758080 does not fit in a 64-bit signed integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := TOMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("TOMLToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("TOMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("TOMLToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("TOMLToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONToTOML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "keys sorted deterministically",
			input: `{"zebra":1,"alpha":2}`,
			want:  "alpha = 2\nzebra = 1",
		},
		{
			name:  "nested object becomes a table",
			input: `{"service":{"name":"api","ports":[80,443],"debug":false}}`,
			want:  "[service]\ndebug = false\nname = 'api'\nports = [80, 443]",
		},
		{
			name:  "array of objects becomes an array of tables",
			input: `{"products":[{"name":"a"},{"name":"b"}]}`,
			want:  "[[products]]\nname = 'a'\n\n[[products]]\nname = 'b'",
		},
		{
			name:  "integer stays integer, float stays float",
			input: `{"i":3,"f":3.5,"whole":1.0}`,
			want:  "f = 3.5\ni = 3\nwhole = 1.0",
		},
		{
			name:  "max int64 survives exactly",
			input: `{"n":9223372036854775807}`,
			want:  "n = 9223372036854775807",
		},
		{
			name:  "heterogeneous array (TOML 1.0)",
			input: `{"mixed":[1,"two",3.5]}`,
			want:  "mixed = [1, 'two', 3.5]",
		},
		{
			name:  "empty object is an empty document",
			input: `{}`,
			want:  "",
		},
		{
			name:  "pretty-printed input",
			input: "{\n  \"a\": 1\n}",
			want:  "a = 1",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "invalid JSON with position", input: `{"a":}`, wantErr: "line 1, column"},
		{name: "trailing garbage", input: `{} {}`, wantErr: "invalid JSON"},
		{name: "lone surrogate", input: `{"a":` + "\n" + ` "\udc00"}`, wantErr: `string at line 2, column 2 escapes a lone UTF-16 surrogate (\udc00), which has no UTF-8 form`},
		{name: "lone surrogates that would collide as keys", input: `{"\udc00":1,"\udfff":2}`, wantErr: `string at line 1, column 2 escapes a lone UTF-16 surrogate (\udc00)`},
		{name: "null reported before a lone surrogate", input: `{"a":"\udc00","b":null}`, wantErr: `null`},
		{name: "array root", input: `[1,2]`, wantErr: "root must be a JSON object, got an array"},
		{name: "string root", input: `"hi"`, wantErr: "root must be a JSON object, got a string"},
		{name: "null root", input: `null`, wantErr: "root must be a JSON object, got null"},
		{name: "top-level null value", input: `{"a":null}`, wantErr: `null value at a (TOML has no null)`},
		{name: "nested null names the path", input: `{"service":{"tags":[1,null]}}`, wantErr: "null value at service.tags[1]"},
		{name: "null under a quoted key", input: `{"a key":{"b":null}}`, wantErr: `null value at "a key".b`},
		{name: "first sorted null wins deterministically", input: `{"b":null,"a":null}`, wantErr: "null value at a "},
		{name: "integer overflowing int64", input: `{"deep":{"n":12345678901234567890}}`, wantErr: "integer 12345678901234567890 at deep.n overflows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JSONToTOML(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("JSONToTOML() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JSONToTOML() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("JSONToTOML() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JSONToTOML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTOMLToJSONRoundTrip(t *testing.T) {
	input := "alpha = 'yes'\nf = 1.5\nmixed = [1, 'two', 3.5]\nzebra = 1\n\n[service]\ndebug = false\nname = 'api'\nports = [80, 443]"
	var jsonOut, tomlOut bytes.Buffer
	if err := TOMLToJSON(&jsonOut, strings.NewReader(input)); err != nil {
		t.Fatalf("TOMLToJSON() error = %v", err)
	}
	if err := JSONToTOML(&tomlOut, &jsonOut); err != nil {
		t.Fatalf("JSONToTOML() error = %v", err)
	}
	if got := tomlOut.String(); got != input {
		t.Errorf("round trip = %q, want %q", got, input)
	}
}

func TestTOMLToJSONOutputIsValidJSON(t *testing.T) {
	input := "[service]\nname = \"api\"\nports = [80, 443]\nstarted = 2024-01-02T03:04:05Z"
	var out bytes.Buffer
	if err := TOMLToJSON(&out, strings.NewReader(input)); err != nil {
		t.Fatalf("TOMLToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("TOMLToJSON() output is not valid JSON: %v", err)
	}
}

// TestJSONToTOMLOutputLimit checks the limit at its edge on output that grows
// with depth in each way TOML headers do: nested tables and nested arrays of
// tables. At a floor one byte below the output, the headers alone still fit,
// so the size is caught after encoding.
func TestJSONToTOMLOutputLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "nested tables", input: strings.Repeat(`{"a":`, 1000) + "1" + strings.Repeat("}", 1000),
			wantErr: "the document has 999 tables, and each table's header spells out its full dotted key, up to 999 keys and 1999 bytes long"},
		{name: "nested arrays of tables ending in a header", input: strings.Repeat(`{"a":[`, 1000) + "{}" + strings.Repeat("]}", 1000),
			wantErr: "the document has 1000 tables, and each table's header spells out its full dotted key, up to 1000 keys and 2003 bytes long"},
		{name: "quoted keys", input: strings.Repeat(`{"a b":`, 1000) + "1" + strings.Repeat("}", 1000),
			wantErr: "the document has 999 tables, and each table's header spells out its full dotted key, up to 999 keys and 5995 bytes long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want bytes.Buffer
			if err := JSONToTOML(&want, strings.NewReader(tt.input)); err != nil {
				t.Fatal(err)
			}
			if 64*len(tt.input) >= want.Len() {
				t.Fatalf("output is %d bytes, not over 64 times the %d-byte input", want.Len(), len(tt.input))
			}
			var out bytes.Buffer
			if err := jsonToTOML(&out, strings.NewReader(tt.input), want.Len()); err != nil || out.String() != want.String() {
				t.Fatalf("at a floor of exactly the output size: error = %v, output equal %v", err, out.String() == want.String())
			}
			for _, floor := range []int{want.Len() - 1, want.Len() / 2} {
				out.Reset()
				err := jsonToTOML(&out, strings.NewReader(tt.input), floor)
				if !errors.Is(err, ErrOutputLimit) || !strings.Contains(err.Error(), fmt.Sprintf("(the larger of %d bytes and 64 times the input size): %s", floor, tt.wantErr)) {
					t.Fatalf("floor %d: error = %v, want the output limit", floor, err)
				}
				if out.Len() != 0 {
					t.Errorf("floor %d: wrote %d bytes before failing", floor, out.Len())
				}
			}
		})
	}
}

// TestTOMLHeaders checks the header count against go-toml's own output: it
// must equal the bytes of the header lines, so a go-toml release that writes
// headers differently fails here rather than loosening the limit.
func TestTOMLHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "no tables", input: `{"a":1,"b":[1,2],"c":[]}`},
		{name: "nested tables", input: `{"a":{"x":1,"b":{"c":1}},"d":{"e":{}}}`},
		{name: "array of tables", input: `{"a":[{"b":{"c":1}},{"b":{"d":2}}]}`},
		{name: "arrays of tables nested", input: `{"a":[{"b":[{"c":{"d":1}}]}]}`},
		{name: "objects in mixed arrays are inline", input: `{"a":[1,{"b":{"c":1}}],"e":[[{"f":{}}]],"g":[{}]}`},
		{name: "literal keys", input: `{"a b":{"é":{"x.y":{"":{"\"":{"\\":{}}}}}}}`},
		{name: "escaped keys", input: `{"a'b":{"\u0001":{"\u007f\b\f\n\r\t":{"'\"\\é":{}}}}}`},
		{name: "raw delete byte", input: "{\"\x7f\x7f\":{\"x\":{}}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tt.input))
			dec.UseNumber()
			var doc any
			if err := dec.Decode(&doc); err != nil {
				t.Fatal(err)
			}
			value, err := tomlValue(doc, nil)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := toml.NewEncoder(&buf).Encode(value); err != nil {
				t.Fatal(err)
			}
			want := 0
			for line := range strings.Lines(buf.String()) {
				if strings.HasPrefix(line, "[") {
					want += len(line)
				}
			}
			h := tomlHeaders{limit: math.MaxInt64 - 1}
			h.table(value.(map[string]any), 0, 0)
			if h.size != int64(want) {
				t.Errorf("counted %d bytes of headers, go-toml wrote %d:\n%s", h.size, want, buf.String())
			}
		})
	}
}

// FuzzJSONToTOML checks that a failed conversion writes nothing, and that the
// header count equals the bytes of the headers go-toml writes.
func FuzzJSONToTOML(f *testing.F) {
	for _, seed := range []string{
		`{"a":{"b":{"c":1}},"d":[{"e":{}},{"f":[{"g":1}]}]}`,
		`{"a b":{"é":{"x.y":{"":{}}}},"a'b":{"\u0001":{"\"":{}}}}`,
		"{\"\x7f\":{\"\\\\\":{}}}",
		`{"a":[1,{"b":{"c":1}}],"e":[[{"f":{}}]],"g":[{}],"h":[]}`,
		`{"a":null}`,
		`[1]`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		var out bytes.Buffer
		if err := JSONToTOML(&out, strings.NewReader(input)); err != nil {
			if out.Len() != 0 {
				t.Fatalf("JSONToTOML() wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		dec := json.NewDecoder(strings.NewReader(input))
		dec.UseNumber()
		var doc any
		if err := dec.Decode(&doc); err != nil {
			t.Fatal(err)
		}
		value, err := tomlValue(doc, nil)
		if err != nil {
			t.Fatal(err)
		}
		written := 0
		for line := range strings.Lines(out.String() + "\n") {
			if strings.HasPrefix(line, "[") {
				written += len(line)
			}
		}
		h := tomlHeaders{limit: math.MaxInt64 - 1}
		h.table(value.(map[string]any), 0, 0)
		if h.size != int64(written) {
			t.Fatalf("counted %d bytes of headers, go-toml wrote %d:\n%s", h.size, written, out.String())
		}
	})
}
