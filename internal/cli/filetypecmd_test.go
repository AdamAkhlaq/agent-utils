package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stubs record which renderer ran and echo the input they saw, so these
// tests exercise only the CLI layer's flag handling and routing.
func TestFiletypeCommand(t *testing.T) {
	file := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(file, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantMode  string
		wantErr   bool
		wantUsage bool
	}{
		{name: "default calls plain", args: nil, stdin: "X", want: "plain:X\n", wantMode: "plain"},
		{name: "json flag calls asJSON", args: []string{"-json"}, stdin: "X", want: "json:X\n", wantMode: "json"},
		{name: "file argument", args: []string{file}, want: "plain:from file\n", wantMode: "plain"},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMode string
			render := func(mode string) func(r io.Reader) (string, error) {
				return func(r io.Reader) (string, error) {
					gotMode = mode
					data, err := io.ReadAll(r)
					return mode + ":" + string(data), err
				}
			}
			cmd := FiletypeCommand(render("plain"), render("json"))
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
			if gotMode != tt.wantMode {
				t.Errorf("Run() mode = %q, want %q", gotMode, tt.wantMode)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFiletypeCommandDetectionFailureIsRuntimeError(t *testing.T) {
	empty := errors.New("empty input")
	fail := func(r io.Reader) (string, error) { return "", empty }
	cmd := FiletypeCommand(fail, fail)
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader(""), &stdout, &stderr)
	if !errors.Is(err, empty) {
		t.Fatalf("Run() error = %v, want %v", err, empty)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; a detection failure must exit 1, not 2")
	}
	if stdout.Len() != 0 {
		t.Errorf("Run() stdout = %q, want empty", stdout.String())
	}
}
