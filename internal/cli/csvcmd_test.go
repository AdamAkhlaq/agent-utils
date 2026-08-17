package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// The stub echoes input and records the separator it received, so these tests
// exercise only the CLI layer's flag handling, not any real CSV conversion.
func TestCSVToJSONCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantSep   rune
		wantErr   bool
		wantUsage bool
	}{
		{name: "default comma", args: nil, stdin: "X", want: "X\n", wantSep: ','},
		{name: "semicolon separator", args: []string{"-sep", ";"}, stdin: "X", want: "X\n", wantSep: ';'},
		{name: "backslash t means tab", args: []string{"-sep", `\t`}, stdin: "X", want: "X\n", wantSep: '\t'},
		{name: "multi-character separator", args: []string{"-sep", "ab"}, wantErr: true, wantUsage: true},
		{name: "empty separator", args: []string{"-sep", ""}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSep rune
			cmd := CSVToJSONCommand(func(w io.Writer, r io.Reader, sep rune) error {
				gotSep = sep
				_, err := io.Copy(w, r)
				return err
			})
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
			if gotSep != tt.wantSep {
				t.Errorf("Run() sep = %q, want %q", gotSep, tt.wantSep)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCSVToJSONCommandConversionFailureIsRuntimeError(t *testing.T) {
	broken := errors.New("reading CSV: record on line 2: wrong number of fields")
	cmd := CSVToJSONCommand(func(w io.Writer, r io.Reader, sep rune) error { return broken })
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader("a,b\n1"), &stdout, &stderr)
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
