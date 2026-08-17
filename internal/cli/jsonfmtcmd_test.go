package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// The stubs echo input and record which mode ran with what indent, so these
// tests exercise only the CLI layer's flag handling and routing, not any real
// JSON formatting.
func TestJSONFmtCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		want       string
		wantMode   string
		wantIndent string
		wantErr    bool
		wantUsage  bool
	}{
		{name: "default pretty", args: nil, stdin: "X", want: "X\n", wantMode: "pretty", wantIndent: "  "},
		{name: "custom indent", args: []string{"-indent", "4"}, stdin: "X", want: "X\n", wantMode: "pretty", wantIndent: "    "},
		{name: "zero indent", args: []string{"-indent", "0"}, stdin: "X", want: "X\n", wantMode: "pretty", wantIndent: ""},
		{name: "compact", args: []string{"-c"}, stdin: "X", want: "X\n", wantMode: "compact"},
		{name: "check prints nothing", args: []string{"-check"}, stdin: "X", want: "", wantMode: "check"},
		{name: "compact and check conflict", args: []string{"-c", "-check"}, wantErr: true, wantUsage: true},
		{name: "indent with compact conflicts", args: []string{"-indent", "4", "-c"}, wantErr: true, wantUsage: true},
		{name: "explicit default indent with check conflicts", args: []string{"-indent", "2", "-check"}, wantErr: true, wantUsage: true},
		{name: "negative indent", args: []string{"-indent", "-1"}, wantErr: true, wantUsage: true},
		{name: "oversized indent", args: []string{"-indent", "9"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: true, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMode, gotIndent string
			echo := func(w io.Writer, r io.Reader) error {
				_, err := io.Copy(w, r)
				return err
			}
			cmd := JSONFmtCommand(
				func(w io.Writer, r io.Reader, indent string) error {
					gotMode, gotIndent = "pretty", indent
					return echo(w, r)
				},
				func(w io.Writer, r io.Reader) error {
					gotMode = "compact"
					return echo(w, r)
				},
				func(r io.Reader) error {
					gotMode = "check"
					return echo(io.Discard, r)
				},
			)
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
			if gotIndent != tt.wantIndent {
				t.Errorf("Run() indent = %q, want %q", gotIndent, tt.wantIndent)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONFmtCommandCheckFailureIsRuntimeError(t *testing.T) {
	invalid := errors.New("invalid JSON at line 1, column 1")
	cmd := JSONFmtCommand(
		func(w io.Writer, r io.Reader, indent string) error { return nil },
		func(w io.Writer, r io.Reader) error { return nil },
		func(r io.Reader) error { return invalid },
	)
	var stdout, stderr bytes.Buffer
	err := cmd.Run([]string{"-check"}, strings.NewReader("{"), &stdout, &stderr)
	if !errors.Is(err, invalid) {
		t.Fatalf("Run() error = %v, want %v", err, invalid)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; an invalid document must exit 1, not 2")
	}
	if stdout.Len() != 0 {
		t.Errorf("Run() stdout = %q, want empty", stdout.String())
	}
}
