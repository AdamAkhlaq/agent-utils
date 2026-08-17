package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestCSVToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sep     rune
		want    string
		wantErr string
	}{
		{
			name:  "column order preserved",
			input: "zebra,alpha\n1,2\n3,4",
			want:  "[\n  {\n    \"zebra\": \"1\",\n    \"alpha\": \"2\"\n  },\n  {\n    \"zebra\": \"3\",\n    \"alpha\": \"4\"\n  }\n]",
		},
		{
			name:  "values stay strings",
			input: "zip,count\n01234,42",
			want:  "[\n  {\n    \"zip\": \"01234\",\n    \"count\": \"42\"\n  }\n]",
		},
		{
			name:  "quoted field with comma newline and quote",
			input: "name,note\n\"Doe, Jane\",\"line \"\"one\"\"\nline two\"",
			want:  "[\n  {\n    \"name\": \"Doe, Jane\",\n    \"note\": \"line \\\"one\\\"\\nline two\"\n  }\n]",
		},
		{
			name:  "crlf line endings",
			input: "a,b\r\n1,2\r\n",
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "semicolon separator",
			input: "a;b\n1;2",
			sep:   ';',
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "tab separator",
			input: "a\tb\n1\t2",
			sep:   '\t',
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "header row only",
			input: "a,b\n",
			want:  "[]",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "duplicate headers", input: "a,a\n1,2", wantErr: `duplicate header "a"`},
		{name: "ragged row", input: "a,b\n1", wantErr: "wrong number of fields"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := tt.sep
			if sep == 0 {
				sep = ','
			}
			var out bytes.Buffer
			err := CSVToJSON(&out, strings.NewReader(tt.input), sep)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("CSVToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CSVToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CSVToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("CSVToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCSVToJSONOutputIsValidJSON(t *testing.T) {
	input := "name,city\n\"Doe, Jane\",Oslo\nBob,\"New\nYork\""
	var out bytes.Buffer
	if err := CSVToJSON(&out, strings.NewReader(input), ','); err != nil {
		t.Fatalf("CSVToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("CSVToJSON() output is not valid JSON: %v", err)
	}
}
