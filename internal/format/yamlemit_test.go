package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
)

func TestJSONToYAML(t *testing.T) {
	longKey := strings.Repeat("k", maxImplicitKey+1)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "key order preserved", input: `{"zebra":1,"alpha":2}`, want: "zebra: 1\nalpha: 2"},
		{
			name:  "nested structures",
			input: `{"b":{"y":1,"x":[true,null]},"a":"str"}`,
			want:  "b:\n  \"y\": 1\n  x:\n  - true\n  - null\na: str",
		},
		{name: "array document", input: `[1,"two",3.5]`, want: "- 1\n- two\n- 3.5"},
		{name: "scalar document", input: `42`, want: "42"},
		{name: "string document", input: `"x"`, want: "x"},
		{name: "null document", input: `null`, want: "null"},
		{name: "empty object document", input: `{}`, want: "{}"},
		{name: "empty array document", input: `[]`, want: "[]"},
		{name: "pretty-printed input", input: "{\n  \"a\": 1\n}", want: "a: 1"},
		{
			name:  "sequences of sequences",
			input: `[1,[2,[3,4]],[]]`,
			want:  "- 1\n- - 2\n  - - 3\n    - 4\n- []",
		},
		{
			name:  "mappings in sequences",
			input: `[{"a":1,"b":[1,2]},{"c":{"d":{}}},{}]`,
			want:  "- a: 1\n  b:\n  - 1\n  - 2\n- c:\n    d: {}\n- {}",
		},
		{
			name:  "sequences under keys",
			input: `{"a":[{"b":1,"c":2}],"d":[[],{}]}`,
			want:  "a:\n- b: 1\n  c: 2\nd:\n- []\n- {}",
		},

		{name: "integer", input: `{"x":-7}`, want: "x: -7"},
		{name: "integer past 64 bits", input: `{"x":100000000000000000000}`, want: "x: 100000000000000000000"},
		{name: "negative zero", input: `{"x":-0}`, want: "x: -0"},
		{name: "decimal digits kept", input: `{"x":0.933333333333333348}`, want: "x: 0.933333333333333348"},
		{name: "decimal past float64 precision", input: `{"x":12345678901234567.5}`, want: "x: 12345678901234567.5"},
		{name: "exponent gains a point and a sign", input: `{"x":1e3}`, want: "x: 1.0e+3"},
		{name: "capital exponent", input: `{"x":1E3}`, want: "x: 1.0E+3"},
		{name: "signed exponent", input: `{"x":-1e-3}`, want: "x: -1.0e-3"},
		{name: "exponent with a point", input: `{"x":1.5e7}`, want: "x: 1.5e+7"},
		{name: "exponent already YAML 1.1", input: `{"x":1.5e-7}`, want: "x: 1.5e-7"},
		{name: "exponent past float64", input: `{"x":1e400}`, want: "x: 1.0e+400"},

		{name: "plain string", input: `{"a":"hello world"}`, want: "a: hello world"},
		{name: "URL", input: `{"a":"https://example.com/a?b=c#d"}`, want: "a: https://example.com/a?b=c#d"},
		{name: "punctuation after the first character", input: `{"a":"npm run x -- --watch, \"y\" [z] {w} & *v !u"}`, want: `a: npm run x -- --watch, "y" [z] {w} & *v !u`},
		{name: "non-ASCII letters", input: `{"a":"café"}`, want: "a: café"},
		{name: "colon inside", input: `{"a":"app:1"}`, want: "a: app:1"},
		{name: "hash inside", input: `{"a":"books#volume"}`, want: "a: books#volume"},
		{name: "allowed first characters", input: `["_x","/usr","$schema","^1.2","(a)","./bin","../lib",".github"]`, want: "- _x\n- /usr\n- $schema\n- ^1.2\n- (a)\n- ./bin\n- ../lib\n- .github"},
		{name: "boolean keywords", input: `["yes","Yes","NO","on","Off","y","N","true","False"]`, want: "- \"yes\"\n- \"Yes\"\n- \"NO\"\n- \"on\"\n- \"Off\"\n- \"y\"\n- \"N\"\n- \"true\"\n- \"False\""},
		{name: "null keywords", input: `["null","Null","~",""]`, want: "- \"null\"\n- \"Null\"\n- \"~\"\n- \"\""},
		{name: "special floats", input: `[".inf",".NaN","-.inf"]`, want: "- \".inf\"\n- \".NaN\"\n- \"-.inf\""},
		{name: "number-like strings", input: `["123","1e3","0x1F","017","1_000","1:20","0.1.0","2001-12-14",".5","+1"]`, want: "- \"123\"\n- \"1e3\"\n- \"0x1F\"\n- \"017\"\n- \"1_000\"\n- \"1:20\"\n- \"0.1.0\"\n- \"2001-12-14\"\n- \".5\"\n- \"+1\""},
		{name: "indicators first", input: `["-x","?x",":x","#x","&a","*a","!t","|x",">x","'x","%x","@x","` + "`x" + `","[x","{x",",x","=","..","...","..x"]`, want: "- \"-x\"\n- \"?x\"\n- \":x\"\n- \"#x\"\n- \"&a\"\n- \"*a\"\n- \"!t\"\n- \"|x\"\n- \">x\"\n- \"'x\"\n- \"%x\"\n- \"@x\"\n- \"`x\"\n- \"[x\"\n- \"{x\"\n- \",x\"\n- \"=\"\n- \"..\"\n- \"...\"\n- \"..x\""},
		{name: "ends a plain scalar", input: `["x: y","x:","x #y","x\ty"]`, want: "- \"x: y\"\n- \"x:\"\n- \"x #y\"\n- \"x\\ty\""},
		{name: "edge spaces", input: `[" x","x "]`, want: "- \" x\"\n- \"x \""},
		{name: "escapes", input: `{"a":"q\"b\\s\b\f\r\u0000\u007f\u0085\u2028\u2029\ufeff\uffff"}`, want: `a: "q\"b\\s\b\f\r\u0000\u007f\u0085\u2028\u2029\ufeff\uffff"`},
		{name: "emoji quoted raw", input: `{"a":"\ud83d\ude00"}`, want: "a: \"\U0001F600\""},

		{name: "literal block without final newline", input: `{"a":"line1\nline2"}`, want: "a: |-\n  line1\n  line2"},
		{name: "literal block with final newline", input: `{"a":"line1\nline2\n"}`, want: "a: |\n  line1\n  line2"},
		{name: "literal block keeping newlines", input: `{"a":"l1\n\n  l3\n\n","b":1}`, want: "a: |+\n  l1\n\n    l3\n\nb: 1"},
		{name: "literal block starting empty", input: `{"a":"\nx"}`, want: "a: |-\n\n  x"},
		{name: "literal block with a tab inside", input: `{"a":"x\n\ty"}`, want: "a: |-\n  x\n  \ty"},
		{name: "literal block in a sequence", input: `[["m\nn"]]`, want: "- - |-\n    m\n    n"},
		{name: "literal block document", input: `"m\nn"`, want: "|-\n  m\n  n"},
		{name: "single line with final newline", input: `{"a":"x\n"}`, want: `a: "x\n"`},
		{name: "first line indented", input: `{"a":" lead\nx"}`, want: `a: " lead\nx"`},
		{name: "first non-empty line indented", input: `{"a":"\n\n\tlead\nx"}`, want: `a: "\n\n\tlead\nx"`},
		{name: "trailing space on a line", input: `{"a":"x \ny"}`, want: `a: "x \ny"`},
		{name: "whitespace-only line", input: `{"a":"x\n  \ny"}`, want: `a: "x\n  \ny"`},
		{name: "carriage return", input: `{"a":"x\r\ny"}`, want: `a: "x\r\ny"`},
		{name: "YAML 1.1 line break", input: `{"a":"x\n\u0085y"}`, want: `a: "x\n\u0085y"`},

		{name: "merge key written as a string", input: `{"<<":{"b":1}}`, want: "!!str \"<<\":\n  b: 1"},
		{name: "key ending in a merge key", input: `{"a<<":1,"x <<":2,"<<a":3}`, want: "\"a<<\": 1\n\"x <<\": 2\n\"<<a\": 3"},
		{name: "empty key", input: `{"":1}`, want: `"": 1`},
		{name: "longest implicit key", input: `{"` + longKey[1:] + `":1}`, want: longKey[1:] + ": 1"},
		{name: "explicit key for a scalar", input: `{"` + longKey + `":1}`, want: "? " + longKey + "\n: 1"},
		{name: "explicit key for a mapping", input: `[{"` + longKey + `":{"b":[1]}}]`, want: "- ? " + longKey + "\n  : b:\n    - 1"},
		{name: "explicit key quotes a multi-line string", input: `{"` + longKey + `":"\nw"}`, want: "? " + longKey + "\n: \"\\nw\""},
		{name: "merge key quotes a multi-line string", input: `{"<<":"v\nw"}`, want: "!!str \"<<\": \"v\\nw\""},
		{name: "literal block nested under a merge key", input: `{"<<":["v\nw"]}`, want: "!!str \"<<\":\n- |-\n  v\n  w"},

		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "invalid JSON with position", input: `{"a":}`, wantErr: "invalid JSON at line 1, column 6"},
		{name: "trailing garbage", input: `{} {}`, wantErr: "invalid JSON"},
		{name: "duplicate key", input: "{\"a\":1,\n \"a\":2}", wantErr: `duplicate key "a" at line 2, column 2`},
		{name: "nested duplicate key", input: `{"a":{"b":1,"b":2}}`, wantErr: `duplicate key "b" at line 1, column 13`},
		{name: "invalid UTF-8", input: "{\"a\":\"\xff\"}", wantErr: "invalid UTF-8 at line 1, column 7"},
		{name: "lone high surrogate", input: `{"a":"\ud800"}`, wantErr: `string at line 1, column 6 escapes a lone UTF-16 surrogate (\ud800)`},
		{name: "lone low surrogate", input: `{"a":"x\uDC00y"}`, wantErr: `(\uDC00)`},
		{name: "high surrogate before a letter", input: `{"a":"\ud83d\u0041"}`, wantErr: `(\ud83d)`},
		{name: "high surrogate after a pair", input: `{"a":"\ud83d\ude00\udbff"}`, wantErr: `(\udbff)`},
		{name: "lone surrogate in a key", input: `{"\ud83dz":1}`, wantErr: `string at line 1, column 2`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JSONToYAML(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("JSONToYAML() = %q, want an error", out.String())
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JSONToYAML() error = %q, want it to contain %q", err, tt.wantErr)
				}
				if out.Len() != 0 {
					t.Errorf("JSONToYAML() wrote %q before failing", out.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("JSONToYAML() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JSONToYAML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONToYAMLInvalidJSONIsMarked(t *testing.T) {
	err := JSONToYAML(&bytes.Buffer{}, strings.NewReader(`{"a":}`))
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("JSONToYAML() error = %v, want it to wrap ErrInvalidJSON", err)
	}
}

func TestJSONToYAMLRoundTrip(t *testing.T) {
	input := `{"zebra":1,"alpha":"yes","n":12345678901234567890,"nested":{"a":"123","list":[true,null,1.5]}}`
	var yamlOut, jsonOut, compact bytes.Buffer
	if err := JSONToYAML(&yamlOut, strings.NewReader(input)); err != nil {
		t.Fatalf("JSONToYAML() error = %v", err)
	}
	if err := YAMLToJSON(&jsonOut, &yamlOut); err != nil {
		t.Fatalf("YAMLToJSON() error = %v", err)
	}
	if err := JSONCompact(&compact, &jsonOut); err != nil {
		t.Fatalf("JSONCompact() error = %v", err)
	}
	if got := compact.String(); got != input {
		t.Errorf("round trip = %q, want %q", got, input)
	}
}

// TestJSONToYAMLReadsBack checks every value against YAMLToJSON, which reads
// it with go-yaml, both as a value and as a key.
func TestJSONToYAMLReadsBack(t *testing.T) {
	values := []string{
		`"yes"`, `"NO"`, `"null"`, `"~"`, `""`, `".inf"`, `".nan"`, `"1e3"`, `"0x1F"`, `"017"`, `"1_000"`,
		`"1:20"`, `"0.1.0"`, `"2001-12-14"`, `"<<"`, `"a<<"`, `"="`, `"-"`, `"..."`, `"---"`, `"? x"`,
		`"- x"`, `"x: y"`, `"x #y"`, `"x#y"`, `"a:b"`, `"http://x"`, `"$x"`, `"^x"`, `"(x"`, `"./x"`,
		`"../x"`, `".x"`, `"\u0000\u0007\u007f\u0085\u00a0\u2028\ufeff"`, `"\ud83d\ude00"`, `"\t"`,
		`"a\nb"`, `"a\nb\n"`, `"a\n\n"`, `"\nb"`, `"a\n\tb"`, `"\n b"`, `"\n\n\tb\n"`, `"a\r\nb"`, `"a \nb"`, `" a\nb"`,
		`1`, `-0`, `1.0`, `-0.0`, `1e3`, `1E-3`, `0.1000000000000000055511151231257827`, `9223372036854775808`,
		`true`, `false`, `null`, `{}`, `[]`, `{"a":[{"b":"c\nd"}]}`, `[[["x"]]]`,
	}
	longKey := `"` + strings.Repeat("k", maxImplicitKey+1) + `"`
	for _, v := range values {
		for _, doc := range []string{v, `{"k":` + v + `}`, `[` + v + `]`, `{` + keyOf(v) + `:1}`, `{"<<":` + v + `}`, `{` + longKey + `:` + v + `}`} {
			t.Run(doc, func(t *testing.T) { checkReadsBack(t, doc) })
		}
	}
}

// keyOf returns v as a JSON object key: v itself if it is a string, and v
// quoted otherwise.
func keyOf(v string) string {
	if strings.HasPrefix(v, `"`) {
		return v
	}
	return strconv.Quote(v)
}

func FuzzJSONToYAML(f *testing.F) {
	seeds := []string{
		`{"a":1,"b":[true,null,{"c":"d"}],"e":{}}`,
		`["yes","No","~","1e3","0.1.0","x: y","a #b","<<","a<<",".inf","./x","$x"]`,
		`{"<<":{"k":1},"m":"a\nb\n\n","n":" a\nb","o":"a \nb","p":"a\r\nb"}`,
		`[1e3,1E-3,-0,0.5,100000000000000000000,12345678901234567.5,1e400]`,
		`{"\u0000\u0085\u2028\ufeff":"\ud83d\ude00","\u00e9":"\t"}`,
		`[[[1]],[{"a":[{"b":[]}]}],"m\nn"]`,
		`"\ud800"`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		var out bytes.Buffer
		err := JSONToYAML(&out, strings.NewReader(input))
		if err != nil {
			if out.Len() != 0 {
				t.Fatalf("JSONToYAML() wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		checkReadsBack(t, input)
	})
}

// checkReadsBack converts the JSON document doc to YAML, adds the final line
// break as the CLI does, converts it back with YAMLToJSON, and checks that the
// result holds the same values in the same order, within YAMLToJSON's
// documented number limits.
func checkReadsBack(t *testing.T, doc string) {
	t.Helper()
	var yamlOut, jsonOut bytes.Buffer
	if err := JSONToYAML(&yamlOut, strings.NewReader(doc)); err != nil {
		t.Fatalf("JSONToYAML(%q) error = %v", doc, err)
	}
	yamlOut.WriteByte('\n')
	if err := YAMLToJSON(&jsonOut, bytes.NewReader(yamlOut.Bytes())); err != nil {
		t.Fatalf("YAMLToJSON(%q) error = %v", yamlOut.String(), err)
	}
	want, got := decodeOrdered(t, []byte(doc)), decodeOrdered(t, jsonOut.Bytes())
	if path, ok := sameJSON(want, got, "$"); !ok {
		t.Fatalf("%q -> YAML %q -> JSON %q: differs at %s", doc, yamlOut.String(), jsonOut.String(), path)
	}
}

type orderedMember struct {
	key   string
	value any
}

// decodeOrdered decodes a JSON document keeping object members in order, as
// []orderedMember, and numbers as json.Number.
func decodeOrdered(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value func() any
	value = func() any {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("decoding %q: %v", data, err)
		}
		switch tok {
		case json.Delim('{'):
			members := []orderedMember{}
			for dec.More() {
				key, _ := dec.Token()
				members = append(members, orderedMember{key.(string), value()})
			}
			dec.Token()
			return members
		case json.Delim('['):
			items := []any{}
			for dec.More() {
				items = append(items, value())
			}
			dec.Token()
			return items
		}
		return tok
	}
	return value()
}

// sameJSON reports whether got holds the values of want, and the path of the
// first difference. Numbers compare by value. YAMLToJSON reads integers past
// 64 bits and floats past float64 as strings, and floats only to float64
// precision, so those differences are allowed.
func sameJSON(want, got any, path string) (string, bool) {
	switch w := want.(type) {
	case []orderedMember:
		g, ok := got.([]orderedMember)
		if !ok || len(g) != len(w) {
			return path, false
		}
		for i := range w {
			if g[i].key != w[i].key {
				return path + "." + w[i].key, false
			}
			if p, ok := sameJSON(w[i].value, g[i].value, path+"."+w[i].key); !ok {
				return p, false
			}
		}
		return "", true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return path, false
		}
		for i := range w {
			if p, ok := sameJSON(w[i], g[i], path+"["+strconv.Itoa(i)+"]"); !ok {
				return p, false
			}
		}
		return "", true
	case json.Number:
		if s, ok := got.(string); ok {
			return path, s == yamlNumber(w.String()) && beyondYAMLToJSON(w.String())
		}
		g, ok := got.(json.Number)
		return path, ok && sameNumber(w.String(), g.String())
	}
	return path, want == got
}

func sameNumber(a, b string) bool {
	ai, aInt := new(big.Int).SetString(a, 10)
	bi, bInt := new(big.Int).SetString(b, 10)
	if aInt && bInt {
		return ai.Cmp(bi) == 0
	}
	af, _ := strconv.ParseFloat(a, 64)
	bf, _ := strconv.ParseFloat(b, 64)
	return af == bf
}

// beyondYAMLToJSON reports whether YAMLToJSON reads the number n as a string:
// an integer outside the 64-bit range, or a float too large for float64.
func beyondYAMLToJSON(n string) bool {
	if i, ok := new(big.Int).SetString(n, 10); ok {
		return i.Cmp(big.NewInt(math.MinInt64)) < 0 || i.Cmp(new(big.Int).SetUint64(math.MaxUint64)) > 0
	}
	f, _ := strconv.ParseFloat(n, 64)
	return math.IsInf(f, 0)
}
