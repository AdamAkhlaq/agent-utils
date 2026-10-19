package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuoteCommand(t *testing.T) {
	brackets := QuoteTarget{
		Name: "brackets",
		Quote: func(s string) (string, error) {
			if s == "bad" {
				return "", errors.New("cannot quote bad")
			}
			return "[" + s + "]", nil
		},
		Unquote: func(s string) (string, error) {
			if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
				return "", errors.New("not bracketed")
			}
			return s[1 : len(s)-1], nil
		},
	}
	oneWay := QuoteTarget{Name: "oneway", Quote: func(s string) (string, error) { return strings.ToUpper(s), nil }}
	cmd := QuoteCommand([]QuoteTarget{brackets, oneWay})

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   string
		wantUsage bool
	}{
		{name: "quote adds a newline", args: []string{"-as", "brackets"}, stdin: "a b", want: "[a b]\n"},
		{name: "quote keeps a trailing newline", args: []string{"-as", "brackets"}, stdin: "a\n", want: "[a\n]\n"},
		{name: "quote empty input", args: []string{"-as", "brackets"}, stdin: "", want: "[]\n"},
		{name: "second target", args: []string{"-as", "oneway"}, stdin: "ab", want: "AB\n"},
		{name: "decode is raw", args: []string{"-d", "-as", "brackets"}, stdin: "[a b]", want: "a b"},
		{name: "decode keeps a newline inside the literal", args: []string{"-d", "-as", "brackets"}, stdin: "[a\n]", want: "a\n"},
		{name: "decode passes a trailing newline to the core", args: []string{"-d", "-as", "brackets"}, stdin: "[a]\n", wantErr: "not bracketed"},
		{name: "quote failure is a runtime error", args: []string{"-as", "brackets"}, stdin: "bad", wantErr: "cannot quote bad"},
		{name: "decode failure is a runtime error", args: []string{"-d", "-as", "brackets"}, stdin: "a", wantErr: "not bracketed"},
		{name: "missing -as lists the targets", args: nil, wantErr: "quote: -as <target> is required (brackets, oneway)", wantUsage: true},
		{name: "missing -as with -d", args: []string{"-d"}, wantErr: "-as <target> is required", wantUsage: true},
		{name: "unknown target", args: []string{"-as", "sql"}, wantErr: `quote: unknown target "sql" (valid: brackets, oneway)`, wantUsage: true},
		{name: "decode of a one-way target", args: []string{"-d", "-as", "oneway"}, wantErr: "quote: -as oneway cannot be decoded, because its syntax does not stand for one raw string (-d works with brackets)", wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
		{name: "two file arguments", args: []string{"-as", "brackets", "a", "b"}, wantErr: "expected at most one file argument", wantUsage: true},
		{name: "missing file", args: []string{"-as", "brackets", "no-such-file"}, wantErr: "no-such-file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want it to contain %q", err, tt.wantErr)
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				if stdout.Len() != 0 {
					t.Errorf("Run() wrote %q to stdout on failure", stdout.String())
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

	t.Run("usage lines list only decodable targets for -d", func(t *testing.T) {
		want := "agent-utils quote -as <brackets|oneway> [file]\nagent-utils quote -d -as <brackets> [file]"
		if cmd.Usage != want {
			t.Errorf("Usage = %q, want %q", cmd.Usage, want)
		}
	})

	t.Run("reads file argument byte for byte", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "in.txt")
		if err := os.WriteFile(path, []byte("hi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{"-as", "brackets", path}, strings.NewReader("stdin"), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "[hi\n]\n"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})
}
