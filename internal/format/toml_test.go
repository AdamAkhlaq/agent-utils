package format

import (
	"bytes"
	"strings"
	"testing"
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
		{name: "integer overflow rejected at parse", input: "n = 92233720368547758080", wantErr: "too large"},
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
