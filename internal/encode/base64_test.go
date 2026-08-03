package encode

import (
	"bytes"
	"strings"
	"testing"
)

func TestBase64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple text", input: "hi", want: "aGk="},
		{name: "empty input", input: "", want: ""},
		{name: "binary bytes", input: "\x00\x01\xff", want: "AAH/"},
		{name: "multiple blocks", input: "hello, world", want: "aGVsbG8sIHdvcmxk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Base64(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("Base64() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("Base64() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBase64Decode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple text", input: "aGk=", want: "hi"},
		{name: "empty input", input: "", want: ""},
		{name: "trailing newline", input: "aGk=\n", want: "hi"},
		{name: "invalid input", input: "!!!", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Base64Decode(&out, strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("Base64Decode() expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Base64Decode() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("Base64Decode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBase64RoundTrip(t *testing.T) {
	input := "any bytes at all: \x00\x7f\xff"
	var encoded, decoded bytes.Buffer
	if err := Base64(&encoded, strings.NewReader(input)); err != nil {
		t.Fatalf("Base64() error = %v", err)
	}
	if err := Base64Decode(&decoded, &encoded); err != nil {
		t.Fatalf("Base64Decode() error = %v", err)
	}
	if got := decoded.String(); got != input {
		t.Errorf("round trip = %q, want %q", got, input)
	}
}
