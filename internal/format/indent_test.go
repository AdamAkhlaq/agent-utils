package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestIndentedSize(t *testing.T) {
	tests := []struct {
		name        string
		doc         string
		wantNesting int
	}{
		{name: "scalar", doc: `42`},
		{name: "string with brackets and escapes", doc: `"[{,:}] \" \\ \u005b"`},
		{name: "string ending in an escaped backslash", doc: `["a\\",{"k\\":"v"}]`, wantNesting: 2},
		{name: "empty object and array", doc: `{"a":{},"b":[]}`, wantNesting: 2},
		{name: "empty containers with whitespace inside", doc: `[ { } , [ ] ]`, wantNesting: 2},
		{name: "nested", doc: `{"a":[1,{"b":[true,null]}],"c":"x"}`, wantNesting: 4},
		{name: "leading whitespace dropped", doc: " \n\t[1]", wantNesting: 1},
		{name: "trailing whitespace kept", doc: "[1] \n\t", wantNesting: 1},
		{name: "trailing whitespace after a scalar kept", doc: "123 \n"},
		{name: "deep", doc: strings.Repeat("[", 500) + strings.Repeat("]", 500), wantNesting: 500},
	}
	for _, tt := range tests {
		for _, indent := range []string{"  ", "", "\t", "        "} {
			t.Run(tt.name+"/"+strings.ReplaceAll(indent, " ", "_"), func(t *testing.T) {
				var want bytes.Buffer
				if err := json.Indent(&want, []byte(tt.doc), "", indent); err != nil {
					t.Fatal(err)
				}
				size, nesting := indentedSize([]byte(tt.doc), len(indent))
				if size != int64(want.Len()) {
					t.Errorf("size = %d, json.Indent wrote %d", size, want.Len())
				}
				if nesting != tt.wantNesting {
					t.Errorf("nesting = %d, want %d", nesting, tt.wantNesting)
				}
				var got bytes.Buffer
				if err := IndentJSON(&got, []byte(tt.doc), indent, 0); err != nil {
					t.Fatal(err)
				}
				if got.String() != want.String() {
					t.Errorf("IndentJSON wrote %q, json.Indent %q", got.String(), want.String())
				}
			})
		}
	}
}

// deepJSON nests n arrays: its indented size grows with n squared.
func deepJSON(n int) []byte {
	return []byte(strings.Repeat("[", n) + strings.Repeat("]", n))
}

func TestIndentJSONLimit(t *testing.T) {
	doc := deepJSON(100) // 20,000 bytes indented by two spaces
	size, _ := indentedSize(doc, 2)
	tests := []struct {
		name      string
		floor     int
		inputSize int
		indent    string
		wantErr   string
	}{
		{name: "exactly at the floor", floor: int(size), inputSize: 1, indent: "  "},
		{name: "one byte over the floor", floor: int(size) - 1, inputSize: 1, indent: "  ",
			wantErr: "output size limit: pretty-printed output would exceed 19999 bytes (the larger of 19999 bytes and 64 times the input size): the JSON nests 100 levels deep, with 2 spaces of indentation per level"},
		{name: "64 times the input when that is larger", floor: 1, inputSize: int(size+63) / 64, indent: "  "},
		{name: "over 64 times the input", floor: 1, inputSize: int(size-1) / 64, indent: "  ", wantErr: "would exceed 19968 bytes"},
		{name: "one space per level", floor: 1, inputSize: 1, indent: " ", wantErr: "with 1 space of indentation per level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := indentJSON(&out, doc, tt.indent, tt.floor, tt.inputSize)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("indentJSON() error = %v", err)
				}
				if int64(out.Len()) != size {
					t.Errorf("wrote %d bytes, want %d", out.Len(), size)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("indentJSON() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if !errors.Is(err, ErrOutputLimit) {
				t.Errorf("indentJSON() error = %v, want it to wrap ErrOutputLimit", err)
			}
			if out.Len() != 0 {
				t.Errorf("wrote %d bytes before failing", out.Len())
			}
		})
	}
}

func TestIndentJSONWriteError(t *testing.T) {
	err := IndentJSON(failWriter{}, []byte(`[1,2]`), "  ", 0)
	if err == nil || !strings.Contains(err.Error(), "writing output") {
		t.Fatalf("IndentJSON() error = %v, want the write error", err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// TestPrettyOutputLimit runs the issue's inputs through every pretty-printer:
// 20 KB nested 9,999 deep would be 200 MB of output, so each fails at once
// and writes nothing.
func TestPrettyOutputLimit(t *testing.T) {
	brackets := string(deepJSON(9999))
	tests := []struct {
		name    string
		convert func(w *bytes.Buffer, input string) error
		input   string
		wantErr string
	}{
		{name: "json-fmt", input: brackets, wantErr: "nests 9999 levels deep, with 2 spaces of indentation per level; use -c for compact output",
			convert: func(w *bytes.Buffer, in string) error { return JSON(w, strings.NewReader(in), "  ") }},
		{name: "json-fmt at eight spaces", input: brackets, wantErr: "with 8 spaces of indentation per level",
			convert: func(w *bytes.Buffer, in string) error { return JSON(w, strings.NewReader(in), "        ") }},
		{name: "toml2json", input: "a = " + brackets, wantErr: "nests 10000 levels deep",
			convert: func(w *bytes.Buffer, in string) error { return TOMLToJSON(w, strings.NewReader(in)) }},
		{name: "xml2json", input: strings.Repeat("<a>", 9999) + "x" + strings.Repeat("</a>", 9999), wantErr: "nests 9999 levels deep",
			convert: func(w *bytes.Buffer, in string) error { return XMLToJSON(w, strings.NewReader(in)) }},
		{name: "yaml2json", input: brackets, wantErr: "nests 9999 levels deep",
			convert: func(w *bytes.Buffer, in string) error { return YAMLToJSON(w, strings.NewReader(in)) }},
		{name: "yaml2json flow mappings", input: strings.Repeat("{a: ", 9999) + "1" + strings.Repeat("}", 9999), wantErr: "nests 9999 levels deep",
			convert: func(w *bytes.Buffer, in string) error { return YAMLToJSON(w, strings.NewReader(in)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := tt.convert(&out, tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), "pretty-printed output would exceed 64 MiB (the larger of 64 MiB and 64 times the input size)") {
				t.Errorf("error = %v, want it to name the limit", err)
			}
			if out.Len() != 0 {
				t.Errorf("wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// TestJSONValidatesFirst checks that malformed input nested too deep to
// pretty-print fails on its syntax, before any output is built.
func TestJSONValidatesFirst(t *testing.T) {
	var out bytes.Buffer
	err := JSON(&out, strings.NewReader(strings.Repeat("[", 9999)+strings.Repeat("]", 9998)), "  ")
	if err == nil || !errors.Is(err, ErrInvalidJSON) || !strings.Contains(err.Error(), "line 1, column 19997") {
		t.Fatalf("JSON() error = %v, want the syntax error at line 1, column 19997", err)
	}
}

// FuzzIndentedSize checks indentedSize and IndentJSON against json.Indent
// itself on every valid document the fuzzer finds.
func FuzzIndentedSize(f *testing.F) {
	for _, seed := range []string{`{"a":[1,{"b":[]}],"c":"x\"]"}`, " [ {} , [ ] ] \n", `"\\"`, "-1.5e3 ", `[[[[]]]]`, `["\\\"",{"\\":1}]`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		if !json.Valid([]byte(doc)) {
			return
		}
		for _, indent := range []string{"  ", "\t", ""} {
			var want, got bytes.Buffer
			if err := json.Indent(&want, []byte(doc), "", indent); err != nil {
				t.Fatal(err)
			}
			if size, _ := indentedSize([]byte(doc), len(indent)); size != int64(want.Len()) {
				t.Fatalf("indentedSize(%q, %q) = %d, json.Indent wrote %d", doc, indent, size, want.Len())
			}
			if err := IndentJSON(&got, []byte(doc), indent, len(doc)); err != nil {
				t.Fatal(err)
			}
			if got.String() != want.String() {
				t.Fatalf("IndentJSON(%q, %q) = %q, json.Indent wrote %q", doc, indent, got.String(), want.String())
			}
		}
	})
}
