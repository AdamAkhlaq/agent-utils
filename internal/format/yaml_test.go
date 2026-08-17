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

func TestJSONToYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "key order preserved",
			input: `{"zebra":1,"alpha":2}`,
			want:  "zebra: 1\nalpha: 2",
		},
		{
			name:  "nested structures",
			input: `{"b":{"y":1,"x":[true,null]},"a":"str"}`,
			want:  "b:\n  \"y\": 1\n  x:\n  - true\n  - null\na: str",
		},
		{
			name:  "array document",
			input: `[1,"two",3.5]`,
			want:  "- 1\n- two\n- 3.5",
		},
		{
			name:  "big integer survives exactly",
			input: `{"n":12345678901234567890}`,
			want:  "\"n\": 12345678901234567890",
		},
		{
			name:  "boolean-looking string stays quoted",
			input: `{"a":"yes"}`,
			want:  "a: \"yes\"",
		},
		{
			name:  "number-looking string stays quoted",
			input: `{"a":"123"}`,
			want:  "a: \"123\"",
		},
		{
			name:  "scalar document",
			input: `42`,
			want:  "42",
		},
		{
			name:  "pretty-printed input",
			input: "{\n  \"a\": 1\n}",
			want:  "a: 1",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "invalid JSON with position", input: `{"a":}`, wantErr: "line 1, column"},
		{name: "trailing garbage", input: `{} {}`, wantErr: "invalid JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JSONToYAML(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("JSONToYAML() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JSONToYAML() error = %q, want it to contain %q", err, tt.wantErr)
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
