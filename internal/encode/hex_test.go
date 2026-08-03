package encode

import (
	"bytes"
	"strings"
	"testing"
)

func TestHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple text", input: "hi", want: "6869"},
		{name: "empty input", input: "", want: ""},
		{name: "binary bytes", input: "\x00\xff", want: "00ff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Hex(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("Hex() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("Hex() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHexDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple text", input: "6869", want: "hi"},
		{name: "empty input", input: "", want: ""},
		{name: "trailing newline", input: "6869\n", want: "hi"},
		{name: "odd length", input: "686", wantErr: true},
		{name: "invalid characters", input: "zz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := HexDecode(&out, strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("HexDecode() expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("HexDecode() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("HexDecode() = %q, want %q", got, tt.want)
			}
		})
	}
}
