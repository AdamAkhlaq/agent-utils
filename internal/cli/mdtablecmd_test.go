package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// The stubs tag their output so these tests verify only the CLI layer's
// dispatch between the JSON and CSV renderers, not any real rendering.
func TestMarkdownTableCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "default is JSON", args: nil, stdin: "X", want: "json:X"},
		{name: "-csv selects CSV", args: []string{"-csv"}, stdin: "X", want: "csv:X"},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagged := func(tag string) func(w io.Writer, r io.Reader) error {
				return func(w io.Writer, r io.Reader) error {
					if _, err := io.WriteString(w, tag+":"); err != nil {
						return err
					}
					_, err := io.Copy(w, r)
					return err
				}
			}
			cmd := MarkdownTableCommand(tagged("json"), tagged("csv"))
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

func TestMarkdownTableCommandRenderFailureIsRuntimeError(t *testing.T) {
	broken := errors.New("empty JSON array: a table with no rows has no meaning")
	fail := func(w io.Writer, r io.Reader) error { return broken }
	cmd := MarkdownTableCommand(fail, fail)
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader("[]"), &stdout, &stderr)
	if !errors.Is(err, broken) {
		t.Fatalf("Run() error = %v, want %v", err, broken)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; bad input data must exit 1, not 2")
	}
	if stdout.Len() != 0 {
		t.Errorf("Run() stdout = %q, want empty", stdout.String())
	}
}
