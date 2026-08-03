package encode

import (
	"bytes"
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "reserved characters", input: "a b&c", want: "a+b%26c"},
		{name: "empty input", input: "", want: ""},
		{name: "percent sign", input: "100%", want: "100%25"},
		{name: "unreserved characters", input: "abc-_.~", want: "abc-_.~"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := URL(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("URL() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestURLDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "reserved characters", input: "a+b%26c", want: "a b&c"},
		{name: "empty input", input: "", want: ""},
		{name: "trailing newline", input: "a+b%26c\n", want: "a b&c"},
		{name: "invalid escape", input: "%zz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := URLDecode(&out, strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("URLDecode() expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("URLDecode() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("URLDecode() = %q, want %q", got, tt.want)
			}
		})
	}
}
