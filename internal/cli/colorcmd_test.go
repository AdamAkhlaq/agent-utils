package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestColorCommand(t *testing.T) {
	convert := func(input, to string) (string, error) {
		if input == "bad" {
			return "", errors.New("unrecognized color \"bad\"")
		}
		return fmt.Sprintf("%s:%s", to, input), nil
	}
	jsonRepr := func(input string) (string, error) {
		if input == "bad" {
			return "", errors.New("unrecognized color \"bad\"")
		}
		return fmt.Sprintf("json:%s", input), nil
	}
	cmd := ColorCommand(convert, jsonRepr)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "value argument", args: []string{"-to", "hex", "#f80"}, want: "hex:#f80\n"},
		{name: "stdin input", args: []string{"-to", "rgb"}, stdin: "#f80", want: "rgb:#f80\n"},
		{name: "one trailing newline stripped", args: []string{"-to", "hsl"}, stdin: "#f80\n", want: "hsl:#f80\n"},
		{name: "json from argument", args: []string{"-json", "#f80"}, want: "json:#f80\n"},
		{name: "json from stdin", args: []string{"-json"}, stdin: "#f80\n", want: "json:#f80\n"},
		{name: "empty stdin plumbed through", args: []string{"-to", "hex"}, stdin: "", want: "hex:\n"},
		{name: "missing -to", args: []string{"#f80"}, wantErr: true, wantUsage: true},
		{name: "unknown -to form", args: []string{"-to", "cmyk", "#f80"}, wantErr: true, wantUsage: true},
		{name: "-to and -json together", args: []string{"-to", "hex", "-json", "#f80"}, wantErr: true, wantUsage: true},
		{name: "two color arguments", args: []string{"-to", "hex", "#f80", "#fff"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "core error is runtime not usage", args: []string{"-to", "hex", "bad"}, wantErr: true, wantUsage: false},
		{name: "json core error is runtime not usage", args: []string{"-json", "bad"}, wantErr: true, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("missing -to names the forms", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := cmd.Run([]string{"#f80"}, strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		for _, form := range []string{"hex", "rgb", "hsl", "-json"} {
			if !strings.Contains(err.Error(), form) {
				t.Errorf("Run() error %q does not mention %q", err, form)
			}
		}
	})
}
