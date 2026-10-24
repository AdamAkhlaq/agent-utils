package format

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf16"
)

const bom = "\xef\xbb\xbf"

// converters is every converter that reads its input as text, each with a
// valid input that holds the string "ada".
var converters = []struct {
	name    string
	convert func(io.Writer, io.Reader) error
	input   string
}{
	{"csv2json", func(w io.Writer, r io.Reader) error { return CSVToJSON(w, r, ',') }, "name,age\r\nada,36\r\n"},
	{"json2csv", func(w io.Writer, r io.Reader) error { return JSONToCSV(w, r, ',') }, `[{"name": "ada"}]`},
	{"json pretty", func(w io.Writer, r io.Reader) error { return JSON(w, r, "  ") }, `{"name": "ada"}`},
	{"json compact", JSONCompact, `{"name": "ada"}`},
	{"json2yaml", JSONToYAML, `{"name": "ada"}`},
	{"json2toml", JSONToTOML, `{"name": "ada"}`},
	{"toml2json", TOMLToJSON, "name = \"ada\"\n"},
	{"xml2json", XMLToJSON, `<?xml version="1.0" encoding="UTF-8"?><person><name>ada</name></person>`},
	{"yaml2json", YAMLToJSON, "name: ada\n"},
}

func TestLeadingBOMIsSkipped(t *testing.T) {
	for _, tt := range converters {
		t.Run(tt.name, func(t *testing.T) {
			var want, got bytes.Buffer
			if err := tt.convert(&want, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("without a BOM: error = %v", err)
			}
			if err := tt.convert(&got, strings.NewReader(bom+tt.input)); err != nil {
				t.Fatalf("with a BOM: error = %v", err)
			}
			if got.String() != want.String() {
				t.Errorf("with a BOM =\n%s\nwant the output without one:\n%s", got.String(), want.String())
			}
		})
		t.Run(tt.name+" BOM only is empty input", func(t *testing.T) {
			wantErr := tt.convert(io.Discard, strings.NewReader(""))
			gotErr := tt.convert(io.Discard, strings.NewReader(bom))
			if wantErr == nil || gotErr == nil || gotErr.Error() != wantErr.Error() {
				t.Errorf("BOM only: error = %v, want the empty-input error %v", gotErr, wantErr)
			}
		})
		t.Run(tt.name+" read failure", func(t *testing.T) {
			if err := tt.convert(io.Discard, iotest.ErrReader(io.ErrUnexpectedEOF)); err == nil {
				t.Error("expected the read error, got nil")
			}
		})
	}
}

func TestBOMIsContentAfterTheStart(t *testing.T) {
	tests := []struct {
		name    string
		convert func(io.Writer, io.Reader) error
		input   string
		want    string
	}{
		{
			name:    "csv2json second row",
			convert: func(w io.Writer, r io.Reader) error { return CSVToJSON(w, r, ',') },
			input:   "a\n" + bom + "x\n",
			want:    "[\n  {\n    \"a\": \"" + bom + "x\"\n  }\n]",
		},
		{
			name:    "json string value",
			convert: JSONCompact,
			input:   `{"k": "` + bom + `v"}`,
			want:    `{"k":"` + bom + `v"}`,
		},
		{
			name:    "yaml second BOM",
			convert: YAMLToJSON,
			input:   bom + bom + "a: 1\n",
			want:    "{\n  \"\\ufeffa\": 1\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tt.convert(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("got\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestJSONSchemaSkipsLeadingBOM(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"schema.json": bom + `{"$ref": "defs.json"}`,
		"defs.json":   bom + `{"type": "object", "required": ["name"]}`,
	})
	schema, err := CompileJSONSchema(filepath.Join(dir, "schema.json"))
	if err != nil {
		t.Fatalf("CompileJSONSchema() error = %v", err)
	}
	report, err := schema.Validate(strings.NewReader(bom + `{"name": "ada"}`))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !report.Valid {
		t.Errorf("Validate() violations = %q, want none", violations(report))
	}
	report, err = schema.Validate(strings.NewReader(bom + `{}`))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Valid {
		t.Error("Validate() = valid, want the missing name reported")
	}
}

func TestReadText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "empty", input: "", want: ""},
		{name: "ASCII", input: "a: 1\n", want: "a: 1\n"},
		{name: "multi-byte", input: "café ☕ 👩‍💻", want: "café ☕ 👩‍💻"},
		{name: "replacement character as written", input: "\ufffd", want: "\ufffd"},
		{name: "leading BOM is dropped", input: bom + "a", want: "a"},
		{name: "invalid byte", input: "a\xffb", wantErr: "invalid UTF-8 at line 1, column 2 (0xff)"},
		{name: "position on a later line", input: "ab\ncd\xff", wantErr: "invalid UTF-8 at line 2, column 3 (0xff)"},
		{name: "position does not count a leading BOM", input: bom + "\xff", wantErr: "invalid UTF-8 at line 1, column 1 (0xff)"},
		{name: "truncated sequence at the end", input: "caf\xc3", wantErr: "invalid UTF-8 at line 1, column 4 (0xc3)"},
		{name: "overlong encoding", input: "\xc0\xaf", wantErr: "invalid UTF-8 at line 1, column 1 (0xc0)"},
		{name: "surrogate", input: "\xed\xa0\x80", wantErr: "invalid UTF-8 at line 1, column 1 (0xed)"},
		{name: "above U+10FFFF", input: "\xf4\x90\x80\x80", wantErr: "invalid UTF-8 at line 1, column 1 (0xf4)"},
		{
			name:    "UTF-16LE",
			input:   "\xff\xfea\x00",
			wantErr: "invalid UTF-8: this is UTF-16LE text (it starts with the byte order mark FF FE); convert it to UTF-8 first, for example with iconv -f UTF-16LE -t UTF-8",
		},
		{
			name:    "UTF-16BE",
			input:   "\xfe\xff\x00a",
			wantErr: "invalid UTF-8: this is UTF-16BE text (it starts with the byte order mark FE FF); convert it to UTF-8 first, for example with iconv -f UTF-16BE -t UTF-8",
		},
		{
			name:    "UTF-32LE",
			input:   "\xff\xfe\x00\x00a\x00\x00\x00",
			wantErr: "invalid UTF-8: this is UTF-32LE text (it starts with the byte order mark FF FE 00 00); convert it to UTF-8 first, for example with iconv -f UTF-32LE -t UTF-8",
		},
		{
			name:    "UTF-32BE",
			input:   "\x00\x00\xfe\xff\x00\x00\x00a",
			wantErr: "invalid UTF-8: this is UTF-32BE text (it starts with the byte order mark 00 00 FE FF); convert it to UTF-8 first, for example with iconv -f UTF-32BE -t UTF-8",
		},
		{name: "UTF-16 mark after the start is only invalid UTF-8", input: "a\xff\xfe", wantErr: "invalid UTF-8 at line 1, column 2 (0xff)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadText(strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("ReadText() error = %v, want %q", err, tt.wantErr)
				}
				if !errors.Is(err, ErrInvalidUTF8) {
					t.Errorf("ReadText() error = %v, want it to wrap ErrInvalidUTF8", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadText() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("ReadText() = %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("read failure is not invalid UTF-8", func(t *testing.T) {
		_, err := ReadText(iotest.ErrReader(io.ErrUnexpectedEOF))
		if !errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrInvalidUTF8) {
			t.Errorf("ReadText() error = %v, want it to wrap only the read error", err)
		}
	})
}

func TestNonUTF8InputIsRejected(t *testing.T) {
	for _, tt := range converters {
		inputs := map[string]string{
			"invalid byte": strings.Replace(tt.input, "ada", "a\xffda", 1),
			"UTF-16LE":     utf16LE(tt.input),
		}
		for kind, input := range inputs {
			t.Run(tt.name+" "+kind, func(t *testing.T) {
				var out bytes.Buffer
				err := tt.convert(&out, strings.NewReader(input))
				if !errors.Is(err, ErrInvalidUTF8) {
					t.Fatalf("error = %v, want it to wrap ErrInvalidUTF8", err)
				}
				if out.Len() != 0 {
					t.Errorf("wrote %q, want no output", out.String())
				}
			})
		}
	}
}

// utf16LE encodes s the way PowerShell 5 writes text files by default.
func utf16LE(s string) string {
	b := []byte{0xff, 0xfe}
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return string(b)
}
