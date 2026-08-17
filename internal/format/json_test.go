package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		indent  string
		want    string
		wantErr string
	}{
		{
			name:   "object preserves key order",
			input:  `{"b":1,"a":[true,null]}`,
			indent: "  ",
			want:   "{\n  \"b\": 1,\n  \"a\": [\n    true,\n    null\n  ]\n}",
		},
		{
			name:   "big integer survives exactly",
			input:  `{"n":12345678901234567890}`,
			indent: "  ",
			want:   "{\n  \"n\": 12345678901234567890\n}",
		},
		{
			name:   "four space indent",
			input:  `{"a":1}`,
			indent: "    ",
			want:   "{\n    \"a\": 1\n}",
		},
		{
			name:   "empty indent gives newlines only",
			input:  `{"a":1}`,
			indent: "",
			want:   "{\n\"a\": 1\n}",
		},
		{
			name:   "scalar document",
			input:  `42`,
			indent: "  ",
			want:   "42",
		},
		{
			name:   "trailing newline in input dropped",
			input:  "{\"a\":1}\n",
			indent: "  ",
			want:   "{\n  \"a\": 1\n}",
		},
		{
			name:    "empty input",
			input:   "",
			indent:  "  ",
			wantErr: "line 1, column 1",
		},
		{
			name:    "invalid token with position",
			input:   "{\n  \"a\": nope\n}",
			indent:  "  ",
			wantErr: "line 2",
		},
		{
			name:    "trailing garbage",
			input:   `{} {}`,
			indent:  "  ",
			wantErr: "invalid JSON",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JSON(&out, strings.NewReader(tt.input), tt.indent)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("JSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("JSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONCompact(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "minifies pretty input", input: "{\n  \"b\": 1,\n  \"a\": 2\n}", want: `{"b":1,"a":2}`},
		{name: "big integer survives exactly", input: `{"n": 12345678901234567890}`, want: `{"n":12345678901234567890}`},
		{name: "already compact", input: `[1,2,3]`, want: `[1,2,3]`},
		{name: "empty input", input: "", wantErr: true},
		{name: "invalid input", input: `{"a":}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JSONCompact(&out, strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("JSONCompact() expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("JSONCompact() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JSONCompact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONValid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "valid object", input: `{"a": 1}`},
		{name: "valid scalar", input: `null`},
		{name: "empty input", input: "", wantErr: "unexpected end of JSON input"},
		{name: "invalid input", input: `{"a": }`, wantErr: "line 1, column 7"},
		{name: "error on later line", input: "[\n1,\n]", wantErr: "line 3, column 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := JSONValid(strings.NewReader(tt.input))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("JSONValid() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("JSONValid() expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("JSONValid() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	input := `{"b":1,"a":{"nested":[1,2,{"deep":true}]},"n":12345678901234567890}`
	var pretty, compact bytes.Buffer
	if err := JSON(&pretty, strings.NewReader(input), "  "); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if err := JSONCompact(&compact, &pretty); err != nil {
		t.Fatalf("JSONCompact() error = %v", err)
	}
	if got := compact.String(); got != input {
		t.Errorf("round trip = %q, want %q", got, input)
	}
}
