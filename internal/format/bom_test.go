package format

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

const bom = "\xef\xbb\xbf"

func TestLeadingBOMIsSkipped(t *testing.T) {
	tests := []struct {
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
	for _, tt := range tests {
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
