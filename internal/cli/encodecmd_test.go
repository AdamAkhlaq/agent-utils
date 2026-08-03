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

// upper/lower stand in for a real encode/decode pair so these tests exercise
// only the CLI layer's routing, not any particular encoding.
func upper(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes.ToUpper(data))
	return err
}

func lower(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes.ToLower(data))
	return err
}

func TestEncodeCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "encode adds newline", args: nil, stdin: "hi", want: "HI\n"},
		{name: "decode is raw", args: []string{"-d"}, stdin: "HI", want: "hi"},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
	}
	cmd := EncodeCommand("shout", "test command", upper, lower)
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
}

func TestEncodeCommandReadsFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := EncodeCommand("shout", "test command", upper, lower)
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("stdin must be ignored when a file is given")
	if err := cmd.Run([]string{path}, stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "HI\n"; got != want {
		t.Errorf("Run() stdout = %q, want %q", got, want)
	}
	if int64(stdin.Len()) != stdin.Size() {
		t.Error("stdin was read even though a file argument was given")
	}
}
