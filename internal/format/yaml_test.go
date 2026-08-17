package format

import (
	"bytes"
	"strings"
	"testing"
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
			name:  "nested structures",
			input: "b:\n  y: 1\n  x: [true, null]\na: str",
			want:  "{\n  \"b\": {\n    \"y\": 1,\n    \"x\": [\n      true,\n      null\n    ]\n  },\n  \"a\": \"str\"\n}",
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
			name:  "scalar document",
			input: "just a string",
			want:  "\"just a string\"",
		},
		{
			name:  "comment-only document is null",
			input: "# only a comment",
			want:  "null",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "multiple documents", input: "a: 1\n---\nb: 2", wantErr: "2 YAML documents"},
		{name: "invalid yaml", input: "a: [unclosed", wantErr: "invalid YAML"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := YAMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("YAMLToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("YAMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
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
