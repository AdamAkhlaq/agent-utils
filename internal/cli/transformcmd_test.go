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

func TestTransformCommand(t *testing.T) {
	cmd := TransformCommand("shout", "test transform", upper)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "stdin with newline added", args: nil, stdin: "hi", want: "HI\n"},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
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

	t.Run("reads file argument", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "in.txt")
		if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{path}, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "HI\n"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})

	t.Run("transform failure is a runtime error", func(t *testing.T) {
		broken := errors.New("bad input")
		failing := TransformCommand("fail", "test transform", func(w io.Writer, r io.Reader) error {
			return broken
		})
		var stdout, stderr bytes.Buffer
		err := failing.Run(nil, strings.NewReader("x"), &stdout, &stderr)
		if !errors.Is(err, broken) {
			t.Fatalf("Run() error = %v, want %v", err, broken)
		}
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			t.Error("Run() returned a usage error; a transform failure must exit 1, not 2")
		}
	})
}
