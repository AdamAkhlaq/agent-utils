package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnicodeCommand(t *testing.T) {
	inspectJSON := func(data []byte) (string, error) {
		if bytes.Contains(data, []byte{0xff}) {
			return "", errors.New("invalid UTF-8 at byte offset 0 (0xff)")
		}
		return `[inspect:` + string(data) + `]`, nil
	}
	checkJSON := func(data []byte) (string, error) {
		if bytes.Contains(data, []byte{0xff}) {
			return "", errors.New("invalid UTF-8 at byte offset 0 (0xff)")
		}
		return `{check:` + string(data) + `}`, nil
	}
	nfc := func(data []byte) ([]byte, error) {
		if bytes.Contains(data, []byte{0xff}) {
			return nil, errors.New("invalid UTF-8 at byte offset 0 (0xff)")
		}
		return append([]byte("nfc:"), data...), nil
	}
	nfd := func(data []byte) ([]byte, error) {
		if bytes.Contains(data, []byte{0xff}) {
			return nil, errors.New("invalid UTF-8 at byte offset 0 (0xff)")
		}
		return append([]byte("nfd:"), data...), nil
	}
	cmd := UnicodeCommand(inspectJSON, checkJSON, nfc, nfd)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   string
		wantUsage bool
	}{
		{name: "inspect from stdin", stdin: "ab", want: "[inspect:ab]\n"},
		{name: "inspect keeps trailing newline", stdin: "ab\n", want: "[inspect:ab\n]\n"},
		{name: "inspect empty input", stdin: "", want: "[inspect:]\n"},
		{name: "check mode", args: []string{"-check"}, stdin: "ab", want: "{check:ab}\n"},
		{name: "nfc mode writes bytes without newline", args: []string{"-nfc"}, stdin: "ab", want: "nfc:ab"},
		{name: "nfd mode writes bytes without newline", args: []string{"-nfd"}, stdin: "ab", want: "nfd:ab"},
		{name: "nfc and nfd together", args: []string{"-nfc", "-nfd"}, wantErr: "mutually exclusive", wantUsage: true},
		{name: "nfc and check together", args: []string{"-nfc", "-check"}, wantErr: "mutually exclusive", wantUsage: true},
		{name: "nfd and check together", args: []string{"-nfd", "-check"}, wantErr: "mutually exclusive", wantUsage: true},
		{name: "all three together", args: []string{"-nfc", "-nfd", "-check"}, wantErr: "mutually exclusive", wantUsage: true},
		{name: "unknown flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
		{name: "two file arguments", args: []string{"a.txt", "b.txt"}, wantErr: "at most one file argument", wantUsage: true},
		{name: "missing file is runtime error", args: []string{"does-not-exist.txt"}, wantErr: "no such file"},
		{name: "invalid utf-8 is runtime error", stdin: "\xff", wantErr: "invalid UTF-8", wantUsage: false},
		{name: "invalid utf-8 in nfc mode is runtime error", args: []string{"-nfc"}, stdin: "\xff", wantErr: "invalid UTF-8", wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want containing %q", err, tt.wantErr)
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

	t.Run("file argument", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "input.txt")
		if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{path}, strings.NewReader("ignored"), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "[inspect:from file]\n"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})
}
