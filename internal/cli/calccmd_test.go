package cli

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestCalcCommand(t *testing.T) {
	core := func(prefix string) func(expr string, scale int) (string, error) {
		return func(expr string, scale int) (string, error) {
			if expr == "" {
				return "", errors.New("empty expression")
			}
			if expr == "bad" {
				return "", errors.New("syntax error at column 1")
			}
			return fmt.Sprintf("%s%q@%d", prefix, expr, scale), nil
		}
	}
	cmd := CalcCommand(core("text:"), core("json:"))

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "expression argument", args: []string{"1 + 2"}, want: "text:\"1 + 2\"@-1\n"},
		{name: "stdin input", stdin: "1 + 2", want: "text:\"1 + 2\"@-1\n"},
		{name: "one trailing newline stripped", stdin: "1 + 2\n", want: "text:\"1 + 2\"@-1\n"},
		{name: "only one trailing newline stripped", stdin: "1 + 2\n\n", want: "text:\"1 + 2\\n\"@-1\n"},
		{name: "scale passed through", args: []string{"-scale", "2", "1/3"}, want: "text:\"1/3\"@2\n"},
		{name: "explicit scale zero", args: []string{"-scale=0", "1/3"}, want: "text:\"1/3\"@0\n"},
		{name: "scale at the maximum", args: []string{"-scale", "1000", "1/3"}, want: "text:\"1/3\"@1000\n"},
		{name: "json", args: []string{"-json", "1/3"}, want: "json:\"1/3\"@-1\n"},
		{name: "json with scale from stdin", args: []string{"-json", "-scale", "4"}, stdin: "1/3\n", want: "json:\"1/3\"@4\n"},
		{name: "leading minus is the expression", args: []string{"-2 * 3"}, want: "text:\"-2 * 3\"@-1\n"},
		{name: "leading minus decimal", args: []string{"-.5"}, want: "text:\"-.5\"@-1\n"},
		{name: "leading minus parenthesis", args: []string{"-(1)"}, want: "text:\"-(1)\"@-1\n"},
		{name: "leading minus after flags", args: []string{"-json", "-scale", "2", "-1/3"}, want: "json:\"-1/3\"@2\n"},
		{name: "explicit end of flags", args: []string{"--", "-1"}, want: "text:\"-1\"@-1\n"},
		{name: "leading minus and space", args: []string{"- 2"}, want: "text:\"- 2\"@-1\n"},
		{name: "double negation", args: []string{"--5"}, want: "text:\"--5\"@-1\n"},
		{name: "minus plus", args: []string{"-+1"}, want: "text:\"-+1\"@-1\n"},
		{name: "double-dash flag forms", args: []string{"--json", "--scale", "3", "-1"}, want: "json:\"-1\"@3\n"},
		{name: "inline scale value", args: []string{"-scale=3", "-1"}, want: "text:\"-1\"@3\n"},
		{name: "lone minus is an expression", args: []string{"-"}, want: "text:\"-\"@-1\n"},
		{name: "empty stdin is a runtime error", stdin: "", wantErr: true, wantUsage: false},
		{name: "core error is runtime not usage", args: []string{"bad"}, wantErr: true, wantUsage: false},
		{name: "unquoted expression", args: []string{"1", "+", "2"}, wantErr: true, wantUsage: true},
		{name: "negative scale", args: []string{"-scale", "-1", "1/3"}, wantErr: true, wantUsage: true},
		{name: "absurd scale", args: []string{"-scale", "1001", "1/3"}, wantErr: true, wantUsage: true},
		{name: "non-integer scale", args: []string{"-scale", "two", "1/3"}, wantErr: true, wantUsage: true},
		{name: "fractional scale", args: []string{"-scale", "2.5", "1/3"}, wantErr: true, wantUsage: true},
		{name: "hex scale", args: []string{"-scale", "0x10", "1/3"}, wantErr: true, wantUsage: true},
		{name: "unknown flag", args: []string{"-jsn", "1/3"}, wantErr: true, wantUsage: true},
		{name: "overflowing scale", args: []string{"-scale", "99999999999999999999", "1/3"}, wantErr: true, wantUsage: true},
		{name: "scale missing its value", args: []string{"-scale"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x", "1"}, wantErr: true, wantUsage: true},
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

	t.Run("manifest states no default scale", func(t *testing.T) {
		entry, err := newManifestEntry(cmd)
		if err != nil {
			t.Fatal(err)
		}
		want := manifestFlag{
			Name:        "-scale",
			Type:        "integer",
			Default:     "",
			Description: "round the result to exactly n decimal places, padded with zeros (default: exact, or 20 places if the decimal does not terminate)",
		}
		if !slices.Contains(entry.Flags, want) {
			t.Errorf("manifest flags = %+v, want %+v among them", entry.Flags, want)
		}
	})

	t.Run("help shows the scale value name without a default", func(t *testing.T) {
		var stdout bytes.Buffer
		if err := Dispatch(map[string]Command{"calc": cmd}, []string{"calc", "-h"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout.String(), "  -scale n\n") || strings.Contains(stdout.String(), "(default \"") {
			t.Errorf("help = %s, want \"-scale n\" and no stated default", stdout.String())
		}
	})

	t.Run("unquoted expression hints to quote it", func(t *testing.T) {
		err := cmd.Run([]string{"2", "*", "3"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "quote the whole expression") {
			t.Fatalf("Run() error = %v, want a hint to quote the expression", err)
		}
	})
}
