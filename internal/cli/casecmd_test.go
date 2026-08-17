package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaseCommand(t *testing.T) {
	stub := func(s, target string) (string, error) {
		if target == "bad" {
			return "", errors.New("unknown case target \"bad\" (valid: snake, camel)")
		}
		return fmt.Sprintf("%s:%s", target, s), nil
	}
	cmd := CaseCommand(stub)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "target plumbed to core", args: []string{"-to", "snake"}, stdin: "hi", want: "snake:hi\n"},
		{name: "one trailing newline stripped", args: []string{"-to", "camel"}, stdin: "hi\n", want: "camel:hi\n"},
		{name: "empty input", args: []string{"-to", "snake"}, stdin: "", want: "snake:\n"},
		{name: "missing -to", args: nil, stdin: "hi", wantErr: true, wantUsage: true},
		{name: "unknown target is usage error", args: []string{"-to", "bad"}, stdin: "hi", wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"-to", "snake", "a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"-to", "snake", "no-such-file"}, wantErr: true, wantUsage: false},
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

	t.Run("missing -to lists the targets", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := cmd.Run(nil, strings.NewReader("hi"), &stdout, &stderr)
		if err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		for _, target := range []string{"snake", "camel", "pascal", "kebab", "screaming"} {
			if !strings.Contains(err.Error(), target) {
				t.Errorf("Run() error %q does not list target %q", err, target)
			}
		}
	})

	t.Run("reads file argument", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "in.txt")
		if err := os.WriteFile(path, []byte("hi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{"-to", "kebab", path}, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "kebab:hi\n"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})
}
