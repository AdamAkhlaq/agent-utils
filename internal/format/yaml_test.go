package format

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
)

func TestYAMLToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "key order preserved",
			input: "zebra: 1\nalpha: 2",
			want:  "{\n  \"zebra\": 1,\n  \"alpha\": 2\n}",
		},
		{
			name:  "!!int of a 64-bit unsigned integer",
			input: "[!!int 12345678901234567890, !!int 0x8000000000000000, !!int \"18446744073709551615\"]",
			want:  "[\n  12345678901234567890,\n  9223372036854775808,\n  18446744073709551615\n]",
		},
		{
			name:  "tag on an empty value at the end of the input",
			input: "a: !!str",
			want:  "{\n  \"a\": \"\"\n}",
		},
		{
			name:  "tags at the end of the input in a sequence",
			input: "- x\n- !!str",
			want:  "[\n  \"x\",\n  \"\"\n]",
		},
		{
			name:  "block scalar at the end of the input keeps its chomping",
			input: "a: !!str |+\n  x",
			want:  "{\n  \"a\": \"x\"\n}",
		},
		{
			name:  "block scalar at the end of the input with a tag-like line",
			input: "a: |+\n  x\n  !!str y",
			want:  "{\n  \"a\": \"x\\n!!str y\"\n}",
		},
		{
			name:  "block scalar at the end of the input with a tag-like word",
			input: "a: |+\n  x !y",
			want:  "{\n  \"a\": \"x !y\"\n}",
		},
		{
			name:  "YAML 1.2 directive",
			input: "%YAML 1.2\n---\na: 1\nb: [yes, 0x1F]",
			want:  "{\n  \"a\": 1,\n  \"b\": [\n    \"yes\",\n    31\n  ]\n}",
		},
		{
			name:  "YAML 1.1 directive reads scalars the same way",
			input: "%YAML 1.1\n---\na: yes",
			want:  "{\n  \"a\": \"yes\"\n}",
		},
		{
			name:  "document end and comments after a document",
			input: "---\na: 1\n...\n# done\n",
			want:  "{\n  \"a\": 1\n}",
		},
		{
			name:  "document marker inside a block scalar",
			input: "a: |\n  ---\n  x\n",
			want:  "{\n  \"a\": \"---\\nx\\n\"\n}",
		},
		{
			name:  "YAML directive before an empty document",
			input: "%YAML 1.2\n---\n",
			want:  "null",
		},
		{
			name:  "nested structures",
			input: "b:\n  y: 1\n  x: [true, null]\na: str",
			want:  "{\n  \"b\": {\n    \"y\": 1,\n    \"x\": [\n      true,\n      null\n    ]\n  },\n  \"a\": \"str\"\n}",
		},
		{
			name:  "empty collections stay compact",
			input: "a: {}\nb: []\nc: [{}, []]",
			want:  "{\n  \"a\": {},\n  \"b\": [],\n  \"c\": [\n    {},\n    []\n  ]\n}",
		},
		{
			name:  "flow pairs in a sequence are mappings",
			input: "[a: 1, b: 2]",
			want:  "[\n  {\n    \"a\": 1\n  },\n  {\n    \"b\": 2\n  }\n]",
		},
		{
			name:  "big integer survives exactly",
			input: "n: 12345678901234567890",
			want:  "{\n  \"n\": 12345678901234567890\n}",
		},
		{
			name:  "yaml 1.2 semantics keep yes as a string",
			input: "on: yes",
			want:  "{\n  \"on\": \"yes\"\n}",
		},
		{
			name:  "non-string key becomes a JSON string",
			input: "1: one",
			want:  "{\n  \"1\": \"one\"\n}",
		},
		{
			name:  "anchors and aliases resolved",
			input: "base: &b\n  k: v\ncopy: *b",
			want:  "{\n  \"base\": {\n    \"k\": \"v\"\n  },\n  \"copy\": {\n    \"k\": \"v\"\n  }\n}",
		},
		{
			name:  "alias to a scalar",
			input: "a: &x 5\nb: [*x, *x]",
			want:  "{\n  \"a\": 5,\n  \"b\": [\n    5,\n    5\n  ]\n}",
		},
		{
			name:  "alias as a mapping key",
			input: "k: &k name\n*k : v",
			want:  "{\n  \"k\": \"name\",\n  \"name\": \"v\"\n}",
		},
		{
			name:  "alias to an anchor nested in its parent's anchor",
			input: "a: &A [&B [1, 2], *B]",
			want:  "{\n  \"a\": [\n    [\n      1,\n      2\n    ],\n    [\n      1,\n      2\n    ]\n  ]\n}",
		},
		{
			name:  "a redefined anchor applies from its new definition on",
			input: "a: &x 1\nb: *x\nc: &x 2\nd: *x",
			want:  "{\n  \"a\": 1,\n  \"b\": 1,\n  \"c\": 2,\n  \"d\": 2\n}",
		},
		{
			name:  "an anchor redefined inside its own value",
			input: "a: &x [&x 1, *x]",
			want:  "{\n  \"a\": [\n    1,\n    1\n  ]\n}",
		},
		{
			name:  "quoted anchor names match when written the same way",
			input: "a: &\"x y\" 1\nb: *\"x y\"\nc: &'q' 2\nd: *'q'",
			want:  "{\n  \"a\": 1,\n  \"b\": 1,\n  \"c\": 2,\n  \"d\": 2\n}",
		},
		{
			name:  "alias to a tagged anchor",
			input: "a: &x !!str 5\nb: *x",
			want:  "{\n  \"a\": \"5\",\n  \"b\": \"5\"\n}",
		},
		{
			name:  "!!binary alias",
			input: "b: &b !!binary AAE=\nc: *b",
			want:  "{\n  \"b\": [\n    0,\n    1\n  ],\n  \"c\": [\n    0,\n    1\n  ]\n}",
		},
		{
			name:  "scalar document",
			input: "just a string",
			want:  "\"just a string\"",
		},
		{
			name:  "comment-only document is null",
			input: "# only a comment",
			want:  "null",
		},
		{
			name:  "document marker only is null",
			input: "---\n",
			want:  "null",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "multiple documents", input: "a: 1\n---\nb: 2", wantErr: "2 YAML documents"},
		{name: "YAML 1.0 directive", input: "%YAML 1.0\n---\na: 1", wantErr: "unsupported directive %YAML 1.0 at line 1, column 1: yaml2json reads YAML 1.1 and 1.2"},
		{name: "TAG directive", input: "%TAG !e! tag:example.com,2000:\n---\na: !e!x 1", wantErr: "unsupported directive %TAG at line 1, column 1: yaml2json cannot resolve tag handles that %TAG defines"},
		{name: "unknown directive", input: "%FOO bar\n---\na: 1", wantErr: "unsupported directive %FOO at line 1, column 1: yaml2json reads only %YAML directives"},
		{name: "empty document before another", input: "---\n---\na: 1", wantErr: "input has 2 YAML documents; expected one"},
		{name: "document after a document end", input: "a: 1\n...\nb: 2", wantErr: "input has 2 YAML documents; expected one"},
		{name: "directive before a second document", input: "a: 1\n...\n%YAML 1.2\n---\nb: 2", wantErr: "input has 2 YAML documents; expected one"},
		{name: "two tags on one node", input: "a: !!str !!str v", wantErr: "invalid YAML: node at line 1, column 9 has more than one tag (!!str and !!str)"},
		{name: "two tags around an anchor", input: "a: !!str &x !!int 5", wantErr: "has more than one tag (!!str and !!int)"},
		{name: "two anchors on one node", input: "a: &x !!str &y 5", wantErr: "invalid YAML: node at line 1, column 12 has more than one anchor"},
		{name: "invalid yaml", input: "a: [unclosed", wantErr: "invalid YAML"},
		{name: "unknown alias", input: "a: *nope", wantErr: "invalid YAML: alias *nope at line 1, column 4 does not refer to an earlier anchor"},
		{name: "alias before its anchor", input: "a: *x\nb: &x 1", wantErr: "does not refer to an earlier anchor"},
		{name: "unknown alias in a merge key", input: "m: {<<: *nope}", wantErr: "does not refer to an earlier anchor"},
		{name: "unknown alias in a mapping key", input: "*nope : v", wantErr: "does not refer to an earlier anchor"},
		{name: "quoted and plain anchor names differ", input: "a: &\"x\" 1\nb: *x", wantErr: "alias *x at line 2, column 4 does not refer to an earlier anchor"},
		{name: "alias inside its own anchor", input: "a: &a [1, *a]", wantErr: "invalid YAML: alias *a at line 1, column 11 refers to an anchor that contains it"},
		{name: "merge of the enclosing anchor", input: "a: &a {<<: *a}", wantErr: "alias *a at line 1, column 12 refers to an anchor that contains it"},
		{name: "merge of a scalar", input: "m: {<<: 1}", wantErr: "invalid YAML: merge key value at line 1, column 9 is not a mapping or a list of mappings"},
		{name: "merge of an alias to a scalar", input: "s: &s 1\nm: {<<: *s}", wantErr: "is not a mapping or a list of mappings"},
		{name: "merge of a nested list", input: "m: {<<: [[{a: 1}]]}", wantErr: "invalid YAML: merge list entry at line 1, column 10 is not a mapping"},
		{name: "merge of a list with a scalar", input: "a: &a {x: 1}\nm: {<<: [*a, 2]}", wantErr: "invalid YAML: merge list entry at line 2, column 14 is not a mapping"},
		{name: "merge of null", input: "m:\n  <<: ~\n  a: 1", wantErr: "is not a mapping or a list of mappings"},
		{name: "merge key without a value", input: "m:\n  <<:\n  a: 1", wantErr: "is not a mapping or a list of mappings"},
		{name: "alias key to a sequence", input: "a: &k [1]\n*k : v", wantErr: "invalid YAML: mapping key at line 2, column 1 is a sequence; JSON object keys must be scalars"},
		{name: "explicit alias key to a mapping", input: "a: &k {b: 1}\n? *k\n: v", wantErr: "mapping key at line 2, column 1 is a mapping"},
		{name: "tag on an alias", input: "a: &x 5\nb: !!str *x", wantErr: "invalid YAML: alias *x at line 2, column 9 has a tag (!!str), which YAML does not allow on an alias"},
		{name: "custom tag on an alias", input: "a: &x [1]\nb: !custom *x", wantErr: "has a tag (!custom)"},
		{name: "scalar tag on an anchored sequence", input: "a: !!str &x [1]", wantErr: "invalid YAML: !!str at line 1, column 4 applies to a sequence, not a scalar"},
		{name: "duplicate keys after formatting", input: "1: a\n1.0: b", wantErr: "invalid YAML: duplicate mapping key \"1\" at line 2, column 1"},
		{name: "duplicate keys through an alias", input: "&k key: v\n*k : w", wantErr: "duplicate mapping key \"key\""},
		{name: "duplicate null keys", input: "~: a\nnull: b", wantErr: "duplicate mapping key \"null\""},
		{name: "infinity", input: "a: .inf", wantErr: "value at line 1, column 4 is +Inf, which JSON cannot represent"},
		{name: "negative infinity", input: "a: [-.inf]", wantErr: "is -Inf, which JSON cannot represent"},
		{name: "NaN", input: "a: .nan", wantErr: "is NaN, which JSON cannot represent"},
		{name: "infinity from !!float", input: "a: !!float 1e400", wantErr: "cannot convert \"1e400\" to !!float"},
		{name: "!!int overflow", input: "a: !!int 123456789012345678901", wantErr: "cannot convert \"123456789012345678901\" to !!int at line 1, column 4"},
		{name: "!!int of a negative overflow", input: "a: !!int \"-9223372036854775809\"", wantErr: "cannot convert \"-9223372036854775809\" to !!int"},
		{name: "!!null of text", input: "a: !!null x", wantErr: "invalid YAML: cannot convert \"x\" to !!null at line 1, column 4"},
		{name: "!!null of a number", input: "a: !!null 0", wantErr: "cannot convert \"0\" to !!null"},
		{name: "!!int of a tagged empty value at the end of the input", input: "a: !!int", wantErr: "cannot convert \"\" to !!int at line 1, column 4"},
		{name: "tag after a quoted scalar at the end of the input", input: "''''!0", wantErr: "value is not allowed in this context"},
		{name: "empty tagged value before another key", input: "a: !!str &x\nb: 1", wantErr: "unsupported YAML: go-yaml's parser misreads the empty value at line 1, column 4, which has a tag and is followed by another key"},
		{name: "empty anchored value before an alias to it", input: "a: !c &x\nb: *x", wantErr: "misreads the empty value at line 1, column 4"},
		{name: "!!int of a float", input: "a: !!int 1e300", wantErr: "cannot convert \"1e300\" to !!int"},
		{name: "!!int of null", input: "a: !!int ~", wantErr: "cannot convert \"~\" to !!int"},
		{name: "!!float of text", input: "a: !!float abc", wantErr: "cannot convert \"abc\" to !!float"},
		{name: "!!float of a boolean", input: "a: !!float true", wantErr: "cannot convert \"true\" to !!float"},
		{name: "!!binary of invalid base64", input: "a: !!binary \"AA*EC\"", wantErr: "cannot convert \"AA*EC\" to !!binary"},
		{name: "!!binary of a number", input: "a: !!binary 5", wantErr: "cannot convert \"5\" to !!binary"},
		{name: "!!bool of text", input: "a: !!bool maybe", wantErr: "cannot convert \"maybe\" to !!bool"},
		{name: "!!timestamp of text", input: "a: !!timestamp soon", wantErr: "cannot convert \"soon\" to !!timestamp"},
		{name: "!!timestamp of a number", input: "a: !!timestamp 2001", wantErr: "cannot convert \"2001\" to !!timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := YAMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("YAMLToJSON() expected an error, got nil and output %q", out.String())
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("YAMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				if out.Len() != 0 {
					t.Errorf("YAMLToJSON() wrote %q before failing", out.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("YAMLToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("YAMLToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

// goYAMLToJSON is YAMLToJSON as it was before it wrote JSON itself: go-yaml's
// own conversion, pretty-printed by json.Indent. YAMLToJSON matches it byte
// for byte on documents without anchors, aliases, or merge keys, except for
// the deliberate changes in TestYAMLToJSONBehaviorChanges.
func goYAMLToJSON(input string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("go-yaml panicked: %v", r)
		}
	}()
	data := []byte(input)
	if len(bytes.TrimSpace(data)) == 0 {
		return "", fmt.Errorf("empty input")
	}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return "", err
	}
	if len(file.Docs) > 1 {
		return "", fmt.Errorf("%d documents", len(file.Docs))
	}
	compact, err := yaml.YAMLToJSON(data)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimRight(compact, "\n"), "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TestYAMLToJSONScalars pins how every kind of scalar is written, and that
// it is written exactly as go-yaml's own conversion writes it.
func TestYAMLToJSONScalars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain string", input: "x", want: `"x"`},
		{name: "integer", input: "42", want: `42`},
		{name: "negative integer", input: "-7", want: `-7`},
		{name: "signed integer", input: "+1", want: `1`},
		{name: "negative zero integer", input: "-0", want: `0`},
		{name: "hex integer", input: "0x1F", want: `31`},
		{name: "octal integer", input: "0o17", want: `15`},
		{name: "legacy octal integer", input: "017", want: `15`},
		{name: "binary integer", input: "0b11", want: `3`},
		{name: "integer with underscores", input: "1_000", want: `1000`},
		{name: "largest uint64", input: "18446744073709551615", want: `18446744073709551615`},
		{name: "integer beyond uint64 is a string", input: "99999999999999999999999", want: `"99999999999999999999999"`},
		{name: "negative integer beyond int64 is a string", input: "-12345678901234567890", want: `"-12345678901234567890"`},
		{name: "float", input: "1.5", want: `1.5`},
		{name: "float keeps a decimal point", input: "1.0", want: `1.0`},
		{name: "float with exponent", input: "1.5e3", want: `1500.0`},
		{name: "float without leading digit", input: ".5", want: `0.5`},
		{name: "float without trailing digit", input: "5.", want: `5.0`},
		{name: "negative zero float", input: "-0.0", want: `-0.0`},
		{name: "large float", input: "1.0e300", want: `1e+300`},
		{name: "float rounded to float64", input: "123456789.123456789", want: `1.2345678912345679e+08`},
		{name: "exponent without a point is a string", input: "1e3", want: `"1e3"`},
		{name: "upper-case exponent is a string", input: "1E3", want: `"1E3"`},
		{name: "float overflow is a string", input: "1.7976931348623157e309", want: `"1.7976931348623157e309"`},
		{name: "true", input: "true", want: `true`},
		{name: "capitalized true", input: "True", want: `true`},
		{name: "upper-case false", input: "FALSE", want: `false`},
		{name: "yes is a string", input: "yes", want: `"yes"`},
		{name: "on is a string", input: "on", want: `"on"`},
		{name: "tilde", input: "~", want: `null`},
		{name: "null", input: "null", want: `null`},
		{name: "capitalized null", input: "Null", want: `null`},
		{name: "empty value", input: "a:", want: "{\n  \"a\": null\n}"},
		{name: "date is a string", input: "2001-12-14", want: `"2001-12-14"`},
		{name: "plus infinity is a string", input: "+.inf", want: `"+.inf"`},
		{name: "double-quoted escapes", input: `"q\"\\ \t \n \r \b \f \/ \N \_ \L \P"`, want: `"q\"\\ \t \n \r \b \f / \u0085 \u00a0 \u2028 \u2029"`},
		{name: "non-printable runes escaped", input: `"\u00ad \u200b \ufeff"`, want: `"\u00ad \u200b \ufeff"`},
		{name: "printable unicode kept", input: "é 😀 <>&", want: `"é 😀 <>&"`},
		{name: "single-quoted", input: `'it''s'`, want: `"it's"`},
		{name: "block literal", input: "|\n  line one\n  line two\n", want: `"line one\nline two\n"`},
		{name: "stripped block literal", input: "|-\n  x\n", want: `"x"`},
		{name: "folded block", input: ">\n  x\n  y\n", want: `"x y\n"`},
		{name: "plain << value is a string", input: "a: <<", want: "{\n  \"a\": \"<<\"\n}"},
		{name: "!!str of an integer", input: "!!str 5", want: `"5"`},
		{name: "!!str block literal", input: "!!str |\n  x\n", want: `"x\n"`},
		{name: "!!int of a string", input: `!!int "7"`, want: `7`},
		{name: "!!int of hex", input: "!!int 0x1F", want: `31`},
		{name: "!!int of a signed string", input: `!!int "+5"`, want: `5`},
		{name: "!!float of an integer", input: "!!float 1", want: `1.0`},
		{name: "!!float of a string", input: `!!float "1e3"`, want: `1000.0`},
		{name: "!!float of hex", input: "!!float 0x10", want: `16.0`},
		{name: "!!float of a big integer", input: "!!float 12345678901234567890", want: `1.2345678901234567e+19`},
		{name: "!!bool of yes", input: "!!bool yes", want: `true`},
		{name: "!!bool of NO", input: "!!bool NO", want: `false`},
		{name: "!!bool of 1", input: "!!bool 1", want: `true`},
		{name: "!!null of null forms", input: "[!!null ~, !!null null, !!null Null, !!null NULL, !!null \"\"]", want: "[\n  null,\n  null,\n  null,\n  null,\n  null\n]"},
		{name: "tag on an empty value before a newline", input: "a: !!str\n", want: "{\n  \"a\": \"\"\n}"},
		{name: "tag on a mapping on the next line", input: "a: !!map\n  b: 1", want: "{\n  \"a\": {\n    \"b\": 1\n  }\n}"},
		{name: "anchor on an empty value before another key", input: "a: &x\nb: 1", want: "{\n  \"a\": null,\n  \"b\": 1\n}"},
		{name: "!!binary", input: "!!binary AAEC/w==", want: "[\n  0,\n  1,\n  2,\n  255\n]"},
		{name: "!!binary of an empty string", input: `!!binary ""`, want: `[]`},
		{name: "!!binary with line breaks", input: "!!binary \"AAEC\\nAAEC\"", want: "[\n  0,\n  1,\n  2,\n  0,\n  1,\n  2\n]"},
		{name: "!!binary block literal", input: "!!binary |\n  AAEC\n  AAEC\n", want: "[\n  0,\n  1,\n  2,\n  0,\n  1,\n  2\n]"},
		{name: "!!timestamp date", input: "!!timestamp 2001-12-14", want: `"2001-12-14T00:00:00Z"`},
		{name: "!!timestamp with zone", input: "!!timestamp 2001-12-14t21:59:43.10-05:00", want: `"2001-12-14T21:59:43.1-05:00"`},
		{name: "!!timestamp space separated", input: "!!timestamp 2001-12-14 21:59:43.10", want: `"2001-12-14T21:59:43.1Z"`},
		{name: "custom tag on a scalar", input: "!custom 5", want: `"5"`},
		{name: "custom tag on a sequence", input: "!custom [1]", want: "[\n  1\n]"},
		{name: "verbatim tag", input: "!<tag:yaml.org,2002:str> 5", want: `"5"`},
		{name: "!!map", input: "!!map {b: 1}", want: "{\n  \"b\": 1\n}"},
		{name: "!!seq", input: "!!seq [1]", want: "[\n  1\n]"},
		{name: "!!set", input: "!!set {b: 1}", want: "{\n  \"b\": 1\n}"},
		{name: "!!omap", input: "!!omap [b: 1]", want: "[\n  {\n    \"b\": 1\n  }\n]"},
		{name: "tag with an empty value in a flow sequence", input: "[!!int , !!str , 1]", want: "[\n  0,\n  \"\",\n  1\n]"},
		{name: "integer key", input: "1: a", want: "{\n  \"1\": \"a\"\n}"},
		{name: "float key", input: "1.5: a", want: "{\n  \"1.5\": \"a\"\n}"},
		{name: "float key without a fraction", input: "1.0: a", want: "{\n  \"1\": \"a\"\n}"},
		{name: "exponent key", input: "1.5e3: a", want: "{\n  \"1500\": \"a\"\n}"},
		{name: "hex key", input: "0x1F: a", want: "{\n  \"31\": \"a\"\n}"},
		{name: "big integer key", input: "12345678901234567890: a", want: "{\n  \"12345678901234567890\": \"a\"\n}"},
		{name: "boolean key", input: "true: a", want: "{\n  \"true\": \"a\"\n}"},
		{name: "null key", input: "~: a", want: "{\n  \"null\": \"a\"\n}"},
		{name: "infinity key", input: ".inf: a", want: "{\n  \"+Inf\": \"a\"\n}"},
		{name: "NaN key", input: ".nan: a", want: "{\n  \"NaN\": \"a\"\n}"},
		{name: "quoted << key", input: "'<<': 1", want: "{\n  \"<<\": 1\n}"},
		{name: "explicit key", input: "? a\n: 1", want: "{\n  \"a\": 1\n}"},
		{name: "explicit key without a value", input: "{? a}", want: "{\n  \"a\": null\n}"},
		{name: "empty !!str key", input: "{!!str : 1}", want: "{\n  \"\": 1\n}"},
		{name: "!!str key", input: "!!str 5: a", want: "{\n  \"5\": \"a\"\n}"},
		{name: "!!int key", input: `!!int "7": a`, want: "{\n  \"7\": \"a\"\n}"},
		{name: "!!bool key", input: "!!bool yes: a", want: "{\n  \"true\": \"a\"\n}"},
		{name: "!!null key", input: "!!null ~: a", want: "{\n  \"null\": \"a\"\n}"},
		{name: "custom tag key", input: "!custom x: a", want: "{\n  \"x\": \"a\"\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := YAMLToJSON(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("YAMLToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("YAMLToJSON() = %q, want %q", got, tt.want)
			}
			if !json.Valid(out.Bytes()) {
				t.Errorf("YAMLToJSON() output is not valid JSON: %q", out.String())
			}
			if before, err := goYAMLToJSON(tt.input); err != nil || before != tt.want {
				t.Errorf("go-yaml's conversion = %q, %v; want the same output", before, err)
			}
		})
	}
}

// TestYAMLToJSONBehaviorChanges pins where YAMLToJSON deliberately differs
// from go-yaml's own conversion, and checks that go-yaml still behaves as
// described, so an upgrade that changes it is noticed.
func TestYAMLToJSONBehaviorChanges(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		before     string // go-yaml's output, or "error: " and part of its error
		want       string
		wantErr    string
		skipBefore bool // go-yaml expands without limit
	}{
		{
			name:   "explicit key wins over a merged one without a duplicate",
			input:  "b: &b {k: 1, j: 2}\nm: {k: 3, <<: *b}",
			before: "{\n  \"b\": {\n    \"k\": 1,\n    \"j\": 2\n  },\n  \"m\": {\n    \"k\": 3,\n    \"k\": 1,\n    \"j\": 2\n  }\n}",
			want:   "{\n  \"b\": {\n    \"k\": 1,\n    \"j\": 2\n  },\n  \"m\": {\n    \"k\": 3,\n    \"j\": 2\n  }\n}",
		},
		{
			name:   "alias to an anchor nested in its parent's anchor",
			input:  "a: &A [&B [1, 2], *B]",
			before: "{\n  \"a\": [\n    [\n      1,\n      2\n    ],\n    null\n  ]\n}",
			want:   "{\n  \"a\": [\n    [\n      1,\n      2\n    ],\n    [\n      1,\n      2\n    ]\n  ]\n}",
		},
		{
			name:   "anchor in a merged mapping is not rebound",
			input:  "x: &X 1\nt: &T {a: *X}\nx2: &X 2\nm: {<<: *T}",
			before: "{\n  \"x\": 1,\n  \"t\": {\n    \"a\": 1\n  },\n  \"x2\": 2,\n  \"m\": {\n    \"a\": 2\n  }\n}",
			want:   "{\n  \"x\": 1,\n  \"t\": {\n    \"a\": 1\n  },\n  \"x2\": 2,\n  \"m\": {\n    \"a\": 1\n  }\n}",
		},
		{
			name:    "self-referencing alias",
			input:   "a: &a [1, *a]",
			before:  "{\n  \"a\": [\n    1,\n    null\n  ]\n}",
			wantErr: "refers to an anchor that contains it",
		},
		{
			name:   "single-quoted anchor name",
			input:  "a: &'x' 1\nb: *'x'",
			before: "error: could not find alias",
			want:   "{\n  \"a\": 1,\n  \"b\": 1\n}",
		},
		{
			name:    "quoted anchor name and plain alias name",
			input:   "a: &\"x\" 1\nb: *x",
			before:  "{\n  \"a\": 1,\n  \"b\": 1\n}",
			wantErr: "does not refer to an earlier anchor",
		},
		{
			name:   "explicit << key is a merge key",
			input:  "? <<\n: {a: 1}",
			before: "{\n  \"<<\": {\n    \"a\": 1\n  }\n}",
			want:   "{\n  \"a\": 1\n}",
		},
		{
			name:  "control characters escaped as valid JSON",
			input: `a: "\0 \x01 \e \x7f \a \v \U000E0001"`,
			// encoding/json words this error differently across Go versions.
			before: "error: escape",
			want:   `{` + "\n" + `  "a": "\u0000 \u0001 \u001b \u007f \u0007 \u000b \udb40\udc01"` + "\n" + `}`,
		},
		{
			name:    "infinity fails with a clear error",
			input:   "a: .inf",
			before:  "error: invalid character '.'",
			wantErr: "is +Inf, which JSON cannot represent",
		},
		{
			name:   "!!int keeps the unsigned 64-bit range",
			input:  "a: !!int 12345678901234567890",
			before: "{\n  \"a\": 9223372036854775807\n}",
			want:   "{\n  \"a\": 12345678901234567890\n}",
		},
		{
			name:    "!!null of text",
			input:   "a: !!null x",
			before:  "{\n  \"a\": null\n}",
			wantErr: "cannot convert \"x\" to !!null",
		},
		{
			name:   "timestamp and binary keys",
			input:  "!!timestamp 2001-12-14: a\n!!binary AAE=: b",
			before: "{\n  \"2001-12-14 00:00:00 +0000 UTC\": \"a\",\n  \"[0 1]\": \"b\"\n}",
			want:   "{\n  \"2001-12-14T00:00:00Z\": \"a\",\n  \"AAE=\": \"b\"\n}",
		},
		{
			name:    "tag after a quoted scalar at the end of the input",
			input:   "''''!0",
			before:  "\"'\"",
			wantErr: "value is not allowed in this context",
		},
		{
			name:   "tag on an empty value at the end of the input",
			input:  "a: !!str",
			before: "{\n  \"a\": null\n}",
			want:   "{\n  \"a\": \"\"\n}",
		},
		{
			name:    "empty tagged value before another key",
			input:   "a: !!str &x\nb: 1",
			before:  "{\n  \"a\": \"[{b 1}]\"\n}",
			wantErr: "misreads the empty value",
		},
		{
			name:    "!!int of a float",
			input:   "a: !!int 1e300",
			before:  "{\n  \"a\": 0\n}",
			wantErr: "cannot convert",
		},
		{
			name:    "!!float of text",
			input:   "a: !!float abc",
			before:  "{\n  \"a\": 0.0\n}",
			wantErr: "cannot convert",
		},
		{
			name:    "!!binary of invalid base64",
			input:   `a: !!binary "AA*EC"`,
			before:  "{\n  \"a\": []\n}",
			wantErr: "cannot convert",
		},
		{
			name:   "!!binary with spaces",
			input:  `a: !!binary "aGVs bG8="`,
			before: "{\n  \"a\": [\n    104,\n    101,\n    108\n  ]\n}",
			want:   "{\n  \"a\": [\n    104,\n    101,\n    108,\n    108,\n    111\n  ]\n}",
		},
		{
			name:    "empty document before another",
			input:   "---\n# nothing here\n---\na: 1",
			before:  "null",
			wantErr: "input has 2 YAML documents",
		},
		{
			name:    "three tags on one node",
			input:   "x: !!str !!str !!str v\nb: 1",
			before:  "{\n  \"x\": \"v\"\n}",
			wantErr: "has more than one tag",
		},
		{
			name:   "!!binary reads the scalar as written",
			input:  "a: [!!binary 1234, !!binary true, !!binary null]",
			before: "error: cannot convert",
			want:   "{\n  \"a\": [\n    [\n      215,\n      109,\n      248\n    ],\n    [\n      182,\n      187,\n      158\n    ],\n    [\n      158,\n      233,\n      101\n    ]\n  ]\n}",
		},
		{
			name:   "YAML directive",
			input:  "%YAML 1.2\n---\na: 1",
			before: "error: 2 documents",
			want:   "{\n  \"a\": 1\n}",
		},
		{
			name:   "!!str keeps a number as written",
			input:  "a: [!!str 012345, !!str 0x1F, !!str 1.0, !!str 1_000]",
			before: "{\n  \"a\": [\n    \"5349\",\n    \"31\",\n    \"1\",\n    \"1000\"\n  ]\n}",
			want:   "{\n  \"a\": [\n    \"012345\",\n    \"0x1F\",\n    \"1.0\",\n    \"1_000\"\n  ]\n}",
		},
		{
			name:   "!!str keeps null and infinity as written",
			input:  "a: [!!str ~, !!str null, !!str .inf]",
			before: "{\n  \"a\": [\n    \"\",\n    \"\",\n    \"+Inf\"\n  ]\n}",
			want:   "{\n  \"a\": [\n    \"~\",\n    \"null\",\n    \".inf\"\n  ]\n}",
		},
		{
			name:   "!!str key keeps a number as written",
			input:  "!!str 1.0: a",
			before: "{\n  \"1\": \"a\"\n}",
			want:   "{\n  \"1.0\": \"a\"\n}",
		},
		{
			name:   "tag written before an anchor carries to its aliases",
			input:  "a: !!str &x 5\nb: *x\nc: !!int &z \"7\"\nd: *z",
			before: "{\n  \"a\": \"5\",\n  \"b\": 5,\n  \"c\": 7,\n  \"d\": \"7\"\n}",
			want:   "{\n  \"a\": \"5\",\n  \"b\": \"5\",\n  \"c\": 7,\n  \"d\": 7\n}",
		},
		{
			name:    "tag on an alias",
			input:   "a: &x 5\nb: !!str *x",
			before:  "{\n  \"a\": 5,\n  \"b\": \"5\"\n}",
			wantErr: "which YAML does not allow on an alias",
		},
		{
			name:    "!!timestamp of text",
			input:   "a: !!timestamp soon",
			before:  "{\n  \"a\": \"0001-01-01T00:00:00Z\"\n}",
			wantErr: "cannot convert",
		},
		{
			name:    "keys that format the same",
			input:   "1: a\n1.0: b",
			before:  "{\n  \"1\": \"a\",\n  \"1\": \"b\"\n}",
			wantErr: "duplicate mapping key",
		},
		{
			name:    "alias key to a sequence",
			input:   "a: &k [1]\n*k : v",
			before:  "{\n  \"a\": [\n    1\n  ],\n  \"[1]\": \"v\"\n}",
			wantErr: "JSON object keys must be scalars",
		},
		{
			name:    "!!str of a mapping",
			input:   "a: !!str &m {b: 1}",
			before:  "{\n  \"a\": \"[{b 1}]\"\n}",
			wantErr: "applies to a mapping",
		},
		{
			name:    "merge key without a value",
			input:   "<<:",
			before:  "{\n  \"<<\": null\n}",
			wantErr: "is not a mapping or a list of mappings",
		},
		{
			name:   "mappings nested deeper than go-yaml decodes",
			input:  strings.Repeat("{a: ", 5000) + "1" + strings.Repeat("}", 5000),
			before: "error: exceeded max depth",
		},
		{
			name:       "anchors rebound under merge keys",
			input:      rebindBombYAML(60),
			skipBefore: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.skipBefore {
				before, err := goYAMLToJSON(tt.input)
				if prefix, ok := strings.CutPrefix(tt.before, "error: "); ok {
					if err == nil || !strings.Contains(err.Error(), prefix) {
						t.Errorf("go-yaml's conversion = %q, %v; want an error containing %q", before, err, prefix)
					}
				} else if err != nil || before != tt.before {
					t.Errorf("go-yaml's conversion = %q, %v; want %q", before, err, tt.before)
				}
			}
			var out bytes.Buffer
			err := YAMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("YAMLToJSON() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("YAMLToJSON() error = %v", err)
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("YAMLToJSON() output is not valid JSON")
			}
			if tt.want != "" && out.String() != tt.want {
				t.Errorf("YAMLToJSON() = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

// TestYAMLToJSONMergeKeys pins the YAML merge-key spec: explicit keys win
// over merged ones wherever the merge key appears, earlier mappings in a
// merge list win over later ones, and no key is written twice. Explicit
// keys stay where they are written; merged keys go where the merge key is.
func TestYAMLToJSONMergeKeys(t *testing.T) {
	const anchors = "a: &a {x: a, y: a}\nb: &b {y: b, z: b}\n"
	tests := []struct {
		name  string
		input string
		want  string // the "m" member, compacted
	}{
		{name: "merge only", input: "m: {<<: *a}", want: `{"x":"a","y":"a"}`},
		{name: "merge then explicit", input: "m: {<<: *a, k: 1}", want: `{"x":"a","y":"a","k":1}`},
		{name: "explicit then merge", input: "m: {k: 1, <<: *a}", want: `{"k":1,"x":"a","y":"a"}`},
		{name: "explicit after the merge wins and stays in place", input: "m: {<<: *a, x: 1}", want: `{"y":"a","x":1}`},
		{name: "explicit before the merge wins", input: "m: {x: 1, <<: *a}", want: `{"x":1,"y":"a"}`},
		{name: "earlier mapping in a list wins", input: "m: {<<: [*a, *b]}", want: `{"x":"a","y":"a","z":"b"}`},
		{name: "list order decides", input: "m: {<<: [*b, *a]}", want: `{"y":"b","z":"b","x":"a"}`},
		{name: "explicit wins over a list", input: "m: {<<: [*a, *b], y: 1}", want: `{"x":"a","z":"b","y":1}`},
		{name: "inline mapping", input: "m: {<<: {x: 1}, y: 2}", want: `{"x":1,"y":2}`},
		{name: "inline mappings in a list", input: "m: {<<: [{x: 1}, {x: 2, y: 2}]}", want: `{"x":1,"y":2}`},
		{name: "anchored inline mapping", input: "m: {<<: &c {x: 1}}\nn: *c", want: `{"x":1}`},
		{name: "nested merge", input: "c: &c {<<: *a, x: c}\nm: {<<: *c}", want: `{"y":"a","x":"c"}`},
		{name: "nested merge overridden from outside", input: "c: &c {<<: *a, w: c}\nm: {<<: *c, y: 1}", want: `{"x":"a","w":"c","y":1}`},
		{name: "nested merge loses to an earlier list entry", input: "c: &c {<<: *b}\nm: {<<: [*a, *c]}", want: `{"x":"a","y":"a","z":"b"}`},
		{name: "alias to a list of mappings", input: "l: &l [*a, *b]\nm: {<<: *l}", want: `{"x":"a","y":"a","z":"b"}`},
		{name: "merge into an empty mapping", input: "e: &e {}\nm: {<<: *e}", want: `{}`},
		{name: "merge key tagged !!merge", input: "m: {!!merge <<: *a}", want: `{"x":"a","y":"a"}`},
		{name: "explicit merge key", input: "m:\n  ? <<\n  : *a", want: `{"x":"a","y":"a"}`},
		{name: "block style", input: "m:\n  <<: *a\n  x: 1", want: `{"y":"a","x":1}`},
		{name: "explicit key formatted like a merged key", input: "n: &n {1: a}\nm: {<<: *n, 1.0: b}", want: `{"1":"b"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := YAMLToJSON(&out, strings.NewReader(anchors+tt.input)); err != nil {
				t.Fatalf("YAMLToJSON() error = %v", err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatalf("output is not a JSON object: %v", err)
			}
			var got bytes.Buffer
			if err := json.Compact(&got, doc["m"]); err != nil {
				t.Fatal(err)
			}
			if got.String() != tt.want {
				t.Errorf("m = %s, want %s", got.String(), tt.want)
			}
		})
	}
}

// laughsYAML is the classic alias bomb: each level is a list of width aliases
// to the level before it.
func laughsYAML(levels, width int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "a0: &a0 [%s]\n", strings.Repeat("lol, ", width-1)+"lol")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, "a%d: &a%d [%s]\n", i, i, strings.Repeat(fmt.Sprintf("*a%d, ", i-1), width-1)+fmt.Sprintf("*a%d", i-1))
	}
	return b.String()
}

// quotedLaughsYAML is laughsYAML with double-quoted anchor and alias names.
func quotedLaughsYAML(levels, width int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "a0: &\"L 0\" [%s]\n", strings.Repeat("lol, ", width-1)+"lol")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, "a%d: &\"L %d\" [%s]\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*\"L %d\", ", i-1), width), ", "))
	}
	return b.String()
}

// binaryYAML anchors a !!binary value of size zero bytes and aliases it n
// times at the given depth. Each decoded byte is written on a line of its
// own, so the output is far larger than the base64 text.
func binaryYAML(size, depth, n int) string {
	return fmt.Sprintf("b: &b !!binary %s\nc: %s%s%s\n",
		base64.StdEncoding.EncodeToString(make([]byte, size)),
		strings.Repeat("[", depth), strings.TrimSuffix(strings.Repeat("*b, ", n), ", "), strings.Repeat("]", depth))
}

// mergeBombYAML nests merge keys instead: each level is a mapping of width
// keys, each merging the level before it.
func mergeBombYAML(levels, width int) string {
	var b strings.Builder
	b.WriteString("m0: &m0 {")
	for k := range width {
		fmt.Fprintf(&b, "k%d: lol, ", k)
	}
	b.WriteString("end: lol}\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, "m%d: &m%d {", i, i)
		for k := range width {
			fmt.Fprintf(&b, "k%d: {<<: *m%d}, ", k, i-1)
		}
		b.WriteString("end: lol}\n")
	}
	return b.String()
}

// rebindBombYAML redefines X and Y on every line as a merge of a mapping
// holding two aliases to the other. go-yaml resolves those aliases against
// the anchors in effect where the mapping is merged, so the document
// doubles with every line; read as written, every anchor stays tiny.
func rebindBombYAML(lines int) string {
	var b strings.Builder
	b.WriteString("x: &X {v: 1}\ny: &Y {v: 1}\ntx: &TX {a: *X, b: *X}\nty: &TY {a: *Y, b: *Y}\n")
	for i := range lines / 2 {
		fmt.Fprintf(&b, "y%d: &Y {<<: *TX}\nx%d: &X {<<: *TY}\n", i, i)
	}
	return b.String()
}

// inlineMergeBombYAML defines its anchors inside merge keys; every level
// holds width aliases to the one before.
func inlineMergeBombYAML(levels, width int) string {
	var b strings.Builder
	b.WriteString("l0: {<<: &x0 {v: lol}}\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, "l%d: {<<: &x%d {", i, i)
		for k := range width {
			fmt.Fprintf(&b, "k%d: *x%d, ", k, i-1)
		}
		b.WriteString("end: lol}}\n")
	}
	return b.String()
}

// emptyMergeYAML merges width aliases to the previous level into each
// level's mapping. Every mapping is empty, so the output is tiny, but the
// merges repeat.
func emptyMergeYAML(levels, width int) string {
	var b strings.Builder
	b.WriteString("e0: &e0 {}\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&b, "e%d: &e%d {<<: [%s]}\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*e%d, ", i-1), width), ", "))
	}
	return b.String()
}

// aliasListYAML anchors a list of size scalars and aliases it n times, so
// the aliases add exactly n*(size+1) nodes: the list and its items.
func aliasListYAML(size, n int) string {
	return fmt.Sprintf("base: &b [%s]\ncopies: [%s]\n",
		strings.TrimSuffix(strings.Repeat("1, ", size), ", "),
		strings.TrimSuffix(strings.Repeat("*b, ", n), ", "))
}

// aliasTextYAML anchors one scalar of size bytes and aliases it n times, so
// the aliases write exactly n*(size+2) bytes: the scalar and its quotes.
func aliasTextYAML(size, n int) string {
	return fmt.Sprintf("s: &s %s\ncopies: [%s]\n", strings.Repeat("x", size), strings.TrimSuffix(strings.Repeat("*s, ", n), ", "))
}

// hiddenMergeChainYAML builds a chain of n mappings, each merging the one
// before, but defines each link as the value of a merged key that an
// explicit key overrides, so no link is written until the last alias.
func hiddenMergeChainYAML(n int) string {
	var b strings.Builder
	b.WriteString("h0: {k: 0, <<: {k: &m0 {v: 1}}}\n")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, "h%d: {k: 0, <<: {k: &m%d {<<: *m%d}}}\n", i, i, i-1)
	}
	fmt.Fprintf(&b, "last: *m%d\n", n-1)
	return b.String()
}

func TestYAMLToJSONAliasBombs(t *testing.T) {
	const nodes, output = "more than 250000 nodes", "more than 16 MiB of JSON output"
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "billion laughs", input: laughsYAML(9, 9), wantErr: nodes},
		{name: "laughs one level past the limit", input: laughsYAML(6, 9), wantErr: nodes},
		{name: "wide shallow laughs", input: laughsYAML(3, 600), wantErr: nodes},
		{name: "quoted alias names", input: quotedLaughsYAML(9, 9), wantErr: nodes},
		{name: "nested merge keys", input: mergeBombYAML(9, 9), wantErr: nodes},
		{name: "anchors defined inside merge keys", input: inlineMergeBombYAML(9, 9), wantErr: nodes},
		{name: "merges of empty mappings", input: emptyMergeYAML(8, 10), wantErr: nodes},
		{name: "inline merges repeated by an alias", input: "m: {<<: &x {<<: [" + strings.TrimSuffix(strings.Repeat("{}, ", 600), ", ") + "]}}\nr: [" + strings.TrimSuffix(strings.Repeat("*x, ", 600), ", ") + "]\n", wantErr: nodes},
		{name: "binary aliased", input: binaryYAML(4096, 1, 100), wantErr: nodes},
		{name: "binary aliased deep", input: binaryYAML(1024, 200, 60), wantErr: output},
		{name: "binary of an aliased string", input: "s: &s \"" + base64.StdEncoding.EncodeToString(make([]byte, 6144)) + "\"\nb: [" + strings.TrimSuffix(strings.Repeat("!!binary *s, ", 50), ", ") + "]\n", wantErr: "which YAML does not allow on an alias"},
		{name: "long scalar aliased", input: aliasTextYAML(1<<20, 17), wantErr: output},
		{name: "escaped scalar aliased", input: "s: &s \"" + strings.Repeat(`\x01`, 1<<18) + "\"\ncopies: [" + strings.TrimSuffix(strings.Repeat("*s, ", 12), ", ") + "]\n", wantErr: output},
		{name: "deep list aliased", input: "a: &a " + strings.Repeat("[", 1000) + "x" + strings.Repeat("]", 1000) + "\nb: [" + strings.TrimSuffix(strings.Repeat("*a, ", 60), ", ") + "]\n", wantErr: output},
		{name: "laughs under a mapping key", input: laughsYAML(9, 9) + "k:\n  - ? *a8\n    : v\n", wantErr: nodes},
		{name: "tagged laughs", input: laughsYAML(9, 9) + "t: [!!str *a8]\n", wantErr: "which YAML does not allow on an alias"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := YAMLToJSON(&out, strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("YAMLToJSON() expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("YAMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("YAMLToJSON() wrote %d bytes before failing", out.Len())
			}
			// Aliases stop within one line or one scalar of the limit.
			c, _ := convertYAML([]byte(tt.input), realYAMLLimits)
			if max := maxAliasBytes + 8*len(tt.input); c.aliasBytes > max {
				t.Errorf("aliases wrote %d bytes before failing, want at most %d", c.aliasBytes, max)
			}
		})
	}
}

// realConfigYAML exercises anchors, aliases, and merge keys the way CI
// pipelines and compose files use them.
const realConfigYAML = `x-logging: &logging
  driver: json-file
  options: {max-size: 10m, max-file: "3"}
x-service: &service
  restart: unless-stopped
  logging: *logging
  environment: &env
    TZ: UTC
    LOG_LEVEL: info
.rules: &rules
  - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
    changes: &code [app/**/*, lib/**/*, spec/**/*, Gemfile.lock]
  - if: '$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH'
services:
  web:
    <<: *service
    image: example/web:1.4
    environment:
      <<: *env
      PORT: "8080"
  worker:
    <<: [*service]
    image: example/worker:1.4
    command: [bundle, exec, sidekiq]
  cron:
    <<: *service
    image: example/cron:1.4
    logging: {driver: none}
jobs:
  rspec:
    rules: *rules
    script: [bin/rspec]
  danger:
    rules:
      - if: '$CI_MERGE_REQUEST_IID'
        changes: *code
`

func TestYAMLToJSONAnchorHeavyConfigs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "compose and CI config with merge keys",
			input: realConfigYAML,
			want: []string{
				`"web": {` + "\n" + `      "restart": "unless-stopped",` + "\n" + `      "logging": {`,
				`"environment": {` + "\n" + `        "TZ": "UTC",` + "\n" + `        "LOG_LEVEL": "info",` + "\n" + `        "PORT": "8080"` + "\n" + `      }`,
				`"image": "example/cron:1.4",` + "\n" + `      "logging": {` + "\n" + `        "driver": "none"` + "\n" + `      }`,
				`"Gemfile.lock"`,
			},
		},
		{name: "laughs within the limit", input: laughsYAML(5, 9), want: []string{`"a4": [`}},
		{name: "merge keys within the limit", input: mergeBombYAML(4, 9), want: []string{`"m3": {`}},
		{name: "anchors rebound under merge keys", input: rebindBombYAML(60), want: []string{`"x29": {` + "\n" + `    "a": {` + "\n" + `      "v": 1` + "\n" + `    },`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := YAMLToJSON(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("YAMLToJSON() error = %v", err)
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("YAMLToJSON() output is not valid JSON:\n%s", out.String())
			}
			for _, w := range tt.want {
				if !strings.Contains(out.String(), w) {
					t.Errorf("YAMLToJSON() output lacks %q:\n%s", w, out.String())
				}
			}
		})
	}
	// Each service is written once per key: the merged "logging" of cron is
	// replaced by its explicit one, not repeated.
	var out bytes.Buffer
	if err := YAMLToJSON(&out, strings.NewReader(realConfigYAML)); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), `"logging"`); n != 4 {
		t.Errorf("output has %d logging keys, want 4 (x-service, web, worker, cron)", n)
	}
}

var realYAMLLimits = yamlLimits{aliasNodes: maxAliasNodes, aliasBytes: maxAliasBytes, depth: maxYAMLDepth}

// maxYAMLWork bounds yamlConverter.work for an input: each scalar's text is
// decoded at most twice (once where it is written, once for every alias that
// brings it in), and a key string is at most three times its source text.
func maxYAMLWork(input string) int {
	return 8*len(input) + 64
}

// TestYAMLToJSONAliasWork pins the cost of an alias use to the nodes it
// visits and the bytes it writes. A scalar that is slow to decode, such as a
// long !!int, !!float, or !!binary text, or a !!binary key, is decoded once
// however many aliases bring it in. Without that, the first case decodes its
// 100 KB text 100,000 times.
func TestYAMLToJSONAliasWork(t *testing.T) {
	zeros := strings.Repeat("0", 100_000)
	uses := func(alias string, n int) string {
		return strings.TrimSuffix(strings.Repeat(alias+", ", n), ", ")
	}
	key := func(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "long !!int aliased", input: "x: &x !!int \"" + zeros + "1\"\nb: &b [" + uses("*x", 100) + "]\nc: [" + uses("*b", 1000) + "]\n"},
		{name: "long !!float aliased", input: "x: &x !!float \"" + zeros + "1.5\"\nb: &b [" + uses("*x", 100) + "]\nc: [" + uses("*b", 1000) + "]\n"},
		{name: "!!binary padded with spaces", input: "x: &x !!binary \"AAAA" + strings.Repeat(" ", 100_000) + "\"\nb: &b [" + uses("*x", 100) + "]\nc: [" + uses("*b", 400) + "]\n"},
		{name: "tagged scalar inside an aliased list", input: "b: &b [!!int \"" + zeros + "1\"]\nc: [" + uses("*b", 50_000) + "]\n"},
		{name: "losing !!binary keys", input: "k: &k !!binary " + key(30_000) + "\nm: &m {*k : 1}\no: {*k : 2, <<: [" + uses("*m", 1000) + "]}\n"},
		{name: "!!binary alias keys outside any alias", input: "k: &k !!binary " + key(2000) + "\nl: [" + uses("{*k : 1}", 1000) + "]\n"},
		{name: "long !!int aliased past the node limit", input: "x: &x !!int \"" + zeros + "1\"\nb: &b [" + uses("*x", 100) + "]\nc: [" + uses("*b", 2000) + "]\n", wantErr: "more than 250000 nodes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := convertYAML([]byte(tt.input), realYAMLLimits)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("convertYAML() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("convertYAML() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if max := maxYAMLWork(tt.input); c.work > max {
				t.Errorf("work = %d for a %d-byte input, want at most %d", c.work, len(tt.input), max)
			}
		})
	}
}

// TestYAMLToJSONAliasOutputCharged checks that a !!binary value, which
// writes one line per byte indented for its depth, is charged in full before
// it is written: 20 KB aliased 1,000 levels deep would write 40 MB.
func TestYAMLToJSONAliasOutputCharged(t *testing.T) {
	input := "x: &x !!binary " + base64.StdEncoding.EncodeToString(make([]byte, 20_000)) + "\ny: " + strings.Repeat("[", 1000) + "*x" + strings.Repeat("]", 1000) + "\n"
	c, err := convertYAML([]byte(input), realYAMLLimits)
	if err == nil || !strings.Contains(err.Error(), "more than 16 MiB of JSON output") {
		t.Fatalf("convertYAML() error = %v, want the output limit", err)
	}
	if c.aliasBytes > maxAliasBytes {
		t.Errorf("aliases wrote %d bytes before failing, want at most %d", c.aliasBytes, maxAliasBytes)
	}
	small := yamlLimits{aliasNodes: 1000, aliasBytes: 64 << 10, depth: 50}
	for _, tt := range []struct {
		name    string
		over    int
		wantErr bool
	}{
		{name: "exactly at the limit"},
		{name: "one byte over the limit", over: 1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// *p writes the padding and its quotes; *x then writes three
			// bytes at level 3, 41 bytes, unless they would pass the limit.
			pad := 64<<10 - 2 - 41 + tt.over
			input := "p: &p " + strings.Repeat("x", pad) + "\nx: &x !!binary AAAA\ny: [*p, [*x]]\n"
			c, err := convertYAML([]byte(input), small)
			if (err != nil) != tt.wantErr {
				t.Fatalf("convertYAML() error = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr && c.aliasBytes != pad+2 {
				t.Errorf("aliases wrote %d bytes, want %d: the array was written before the check", c.aliasBytes, pad+2)
			}
		})
	}
}

// TestYAMLToJSONAliasLimit pins the limits at their exact edges. Most rows
// use small limits, which exercise the same code as the real ones without
// writing megabytes; the rest check the real limits themselves.
func TestYAMLToJSONAliasLimit(t *testing.T) {
	small := yamlLimits{aliasNodes: 1000, aliasBytes: 64 << 10, depth: 50}
	tests := []struct {
		name    string
		limits  yamlLimits // the real limits when zero
		input   string
		wantErr string
	}{
		{name: "nodes exactly at the limit", limits: small, input: aliasListYAML(9, 100)},
		{name: "one node over the limit", limits: small, input: aliasListYAML(9, 100) + "one: &one 1\nmore: *one\n", wantErr: "alias expansion exceeds the size limit: aliases and merge keys add more than 1000 nodes (keys, values, and merged mappings); expansion is capped at 1000 nodes and 65536 bytes of JSON output"},
		{name: "output exactly at the limit", limits: small, input: aliasTextYAML(16_382, 4)},
		{name: "one byte over the output limit", limits: small, input: aliasTextYAML(16_383, 4), wantErr: "alias expansion exceeds the size limit: aliases and merge keys add more than 65536 bytes of JSON output; expansion is capped at 1000 nodes and 65536 bytes of JSON output"},
		{name: "flow sequences at the depth limit", limits: small, input: strings.Repeat("[", 50) + strings.Repeat("]", 50)},
		{name: "flow sequences past the depth limit", limits: small, input: strings.Repeat("[", 51) + strings.Repeat("]", 51), wantErr: "invalid YAML: nesting exceeds the depth limit of 50 levels"},
		{name: "unclosed flow sequences past the depth limit", limits: small, input: strings.Repeat("[", 51), wantErr: "depth limit of 50"},
		{name: "flow mappings at the depth limit", limits: small, input: strings.Repeat("{a: ", 50) + "1" + strings.Repeat("}", 50)},
		{name: "flow mappings past the depth limit", limits: small, input: strings.Repeat("{a: ", 51) + "1" + strings.Repeat("}", 51), wantErr: "depth limit of 50"},
		{name: "compact block sequences at the depth limit", limits: small, input: strings.Repeat("- ", 50) + "x"},
		{name: "compact block sequences past the depth limit", limits: small, input: strings.Repeat("- ", 51) + "x", wantErr: "depth limit of 50"},
		{name: "compact entries then flow past the depth limit", limits: small, input: strings.Repeat("- ", 25) + strings.Repeat("[", 26) + strings.Repeat("]", 26), wantErr: "depth limit of 50"},
		{name: "compact entries on earlier lines do not add up", limits: small, input: "a:\n  " + strings.Repeat("- ", 30) + "x\nb: " + strings.Repeat("[", 30) + strings.Repeat("]", 30)},
		{name: "explicit keys past the depth limit", limits: small, input: strings.Repeat("? ", 51) + "x", wantErr: "depth limit of 50"},
		{name: "indented mappings at the depth limit", limits: small, input: indentedYAML(50)},
		{name: "indented mappings past the depth limit", limits: small, input: indentedYAML(51), wantErr: "depth limit of 50"},
		{name: "alias that nests to the depth limit", limits: small, input: "a: &a [[x]]\nb: " + strings.Repeat("[", 47) + "*a" + strings.Repeat("]", 47) + "\n"},
		{name: "alias that nests past the depth limit", limits: small, input: "a: &a [[x]]\nb: " + strings.Repeat("[", 48) + "*a" + strings.Repeat("]", 48) + "\n", wantErr: "depth limit of 50"},
		{name: "merge chain within the depth limit", limits: small, input: hiddenMergeChainYAML(45)},
		{name: "merge chain hidden from the output past the depth limit", limits: small, input: hiddenMergeChainYAML(60), wantErr: "depth limit of 50"},
		{name: "real limit: nodes exactly at the limit", input: aliasListYAML(999, 250)},
		{name: "real limit: one node over the limit", input: aliasListYAML(999, 250) + "one: &one 1\nmore: *one\n", wantErr: "more than 250000 nodes (keys, values, and merged mappings); expansion is capped at 250000 nodes and 16 MiB of JSON output"},
		{name: "real limit: output over the limit", input: aliasTextYAML(1<<20-2, 16) + "c: &c x\nmore: *c\n", wantErr: "more than 16 MiB of JSON output; expansion is capped at 250000 nodes and 16 MiB of JSON output"},
		{name: "real limit: sequences past the depth limit", input: strings.Repeat("[", 10_001) + strings.Repeat("]", 10_001), wantErr: "invalid YAML: nesting exceeds the depth limit of 10000 levels"},
		{name: "real limit: compact sequences past the depth limit", input: strings.Repeat("- ", 10_001) + "x", wantErr: "depth limit of 10000"},
		{name: "real limit: merge chain past the depth limit", input: hiddenMergeChainYAML(12_000), wantErr: "depth limit of 10000"},
		{name: "real limit: brackets far past the depth limit fail before parsing", input: strings.Repeat("[", 200_000), wantErr: "depth limit of 10000"},
		{name: "large document without aliases", limits: small, input: "big: [" + strings.TrimSuffix(strings.Repeat("{a: 1}, ", 5_000), ", ") + "]"},
		{name: "deeply nested tags", input: "a: " + strings.Repeat("!!str ", 20_000) + "x", wantErr: "has more than one tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := tt.limits
			if limits == (yamlLimits{}) {
				limits = realYAMLLimits
			}
			var out bytes.Buffer
			err := yamlToJSON(&out, strings.NewReader(tt.input), limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("yamlToJSON() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("yamlToJSON() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("yamlToJSON() wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// indentedYAML nests n block mappings by indentation alone, which the token
// check before parsing does not count.
func indentedYAML(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(strings.Repeat(" ", i) + "a:")
		if i < n-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString(" 1\n")
	return b.String()
}

func TestYAMLToJSONOutputIsValidJSON(t *testing.T) {
	input := "service:\n  name: api\n  ports:\n    - 80\n    - 443\n  debug: false"
	var out bytes.Buffer
	if err := YAMLToJSON(&out, strings.NewReader(input)); err != nil {
		t.Fatalf("YAMLToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("YAMLToJSON() output is not valid JSON: %v", err)
	}
}

// hasReferences reports whether input uses anchors, aliases, or merge keys,
// where YAMLToJSON deliberately differs from go-yaml.
func hasReferences(input string) bool {
	if strings.Contains(input, "<<") {
		return true
	}
	file, err := parser.ParseBytes([]byte(input), 0)
	if err != nil {
		return true
	}
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		for _, typ := range []ast.NodeType{ast.AnchorType, ast.AliasType, ast.MergeKeyType} {
			if len(ast.Filter(typ, doc.Body)) > 0 {
				return true
			}
		}
	}
	return false
}

// hasDuplicateKeys reports whether some JSON object in doc repeats a key.
func hasDuplicateKeys(doc string) bool {
	dec := json.NewDecoder(strings.NewReader(doc))
	type frame struct {
		object bool
		keys   map[string]bool
		key    bool // the next string token is a key
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, &frame{object: true, keys: map[string]bool{}, key: true})
				continue
			case '[':
				stack = append(stack, &frame{})
				continue
			default:
				stack = stack[:len(stack)-1]
			}
		case string:
			if top != nil && top.object && top.key {
				if top.keys[v] {
					return true
				}
				top.keys[v] = true
				top.key = false
				continue
			}
		}
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].key = true
		}
	}
}

// FuzzYAMLToJSON checks that YAMLToJSON never panics, writes valid JSON or
// nothing at all, and, on documents without anchors, aliases, or merge keys,
// writes exactly what go-yaml's own conversion writes wherever that
// succeeds, apart from the documented behavior changes: tags go-yaml turns
// into zero values, keys that format the same, nodes with two tags, and
// empty tagged values that go-yaml nests the next key under are errors,
// and !!str, !!binary, !!int, and !!timestamp values and keys can differ. On
// every input it also checks the work bound of maxYAMLWork.
func FuzzYAMLToJSON(f *testing.F) {
	seeds := []string{
		"a: 1\nb: [x, {c: d}]\n",
		"a: &a {k: [1, 2]}\nb: *a\nc: [*a, *a]\n",
		"base: &b {k: 1, j: 2}\nm: {<<: *b, k: 3}\n",
		"a: &a {x: 1}\nb: &b {y: 2}\nm: {<<: [*a, *b], z: 3}\n",
		"a: &A [&B [1, 2], *B]\n",
		"? &k key\n: !!str 5\nt: !custom {a: *k}\n",
		"b: &b !!binary AAEC/w==\nc: [*b, {k: *b}]\n",
		"s: \"q\\\"\\\\\\N\\u2028é\\t\\x7f\"\nn: [~, null, 0x1F, 0o17, 12345678901234567890, true, 1.0, -0.0, 1e3]\n",
		"t: !!timestamp 2001-12-14\nu: [!!int \"7\", !!float 1, !!bool yes, !!null x, !!str 1.0]\n",
		"1: a\n1.5: b\ntrue: c\n~: d\n!!binary AAE=: e\n",
		"a: |\n  line one\n  line two\nb: >-\n  folded\n  text\n",
		"- a: 1\n  b: 2\n- [c, d]\n- {e: f}\n",
		"m: {<<: &x {<<: [{}, {}, {a: 1}]}}\nr: [*x, *x, {<<: *x}]\n",
		"%YAML 1.2\n---\na: 1\n",
		"- - - [x, {y: [z]}]\n- ? - k\n  : v\n",
		"a: !!str &x 5\nb: *x\nc: !!binary \"aGVs\\n bG8=\"\n",
		"? a\n: [!!int , !!str , {? b}]\n'c': \"d\"\n",
		"a: 'it''s'\nb: \"\\0 \\a \\v \\e \\U0001F600\"\nc: 1.5e3\nd: .inf\n",
		"!!binary \xfa\xdf",
		"!!binary ! 0",
		laughsYAML(3, 3),
		mergeBombYAML(3, 2),
		rebindBombYAML(4),
		inlineMergeBombYAML(3, 2),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			return
		}
		if c, _ := convertYAML([]byte(input), realYAMLLimits); c.work > maxYAMLWork(input) {
			t.Fatalf("convertYAML(%q) did %d work, want at most %d", input, c.work, maxYAMLWork(input))
		}
		var out bytes.Buffer
		err := YAMLToJSON(&out, strings.NewReader(input))
		if err != nil && out.Len() != 0 {
			t.Fatalf("YAMLToJSON() wrote %d bytes before failing: %v", out.Len(), err)
		}
		if err == nil && !json.Valid(out.Bytes()) {
			t.Fatalf("YAMLToJSON() output is not valid JSON: %q", out.String())
		}
		if hasReferences(input) {
			return
		}
		oracleInput := input
		if countTags(tokenize([]byte(input))) != countTags(lexer.Tokenize(input)) {
			oracleInput += "\n"
		}
		want, goErr := goYAMLToJSON(oracleInput)
		if goErr != nil {
			return
		}
		if err != nil {
			msg := err.Error()
			switch {
			case strings.Contains(msg, "cannot convert") && strings.Contains(input, "!!"):
			case strings.Contains(msg, "duplicate mapping key") && hasDuplicateKeys(want):
			case strings.Contains(msg, "more than one tag"), strings.Contains(msg, "more than one anchor"):
			case strings.Contains(msg, "YAML documents; expected one"):
			case strings.Contains(msg, "misreads the empty value"):
			default:
				t.Fatalf("go-yaml converts %q to %q, but YAMLToJSON() fails: %v", input, want, err)
			}
			return
		}
		if got := out.String(); got != want && !strings.Contains(input, "!!str") && !strings.Contains(input, "!!binary") && !strings.Contains(input, "!!timestamp") && !strings.Contains(input, "!!int") {
			t.Fatalf("YAMLToJSON(%q) = %q, go-yaml writes %q", input, got, want)
		}
	})
}
