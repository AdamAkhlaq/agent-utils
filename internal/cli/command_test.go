package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func testCommands() map[string]Command {
	nop := func(args []string, stdin io.Reader, stdout, stderr io.Writer) error { return nil }
	return map[string]Command{
		"zeta":  {Name: "zeta", Summary: "last alphabetically", Run: nop},
		"alpha": {Name: "alpha", Summary: "first alphabetically", Run: nop},
	}
}

func TestDispatch(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantUsage bool
	}{
		{name: "no command", args: nil, wantUsage: true},
		{name: "unknown command", args: []string{"nonsense"}, wantUsage: true},
		{name: "known command", args: []string{"alpha"}, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Dispatch(testCommands(), tt.args, strings.NewReader(""), &stdout, &stderr)
			var usageErr *UsageError
			if got := errors.As(err, &usageErr); got != tt.wantUsage {
				t.Fatalf("Dispatch() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
			}
			if tt.wantUsage && !strings.Contains(stderr.String(), "usage: dev-utils") {
				t.Errorf("expected usage on stderr, got %q", stderr.String())
			}
		})
	}
}

func TestUsageSortsCommands(t *testing.T) {
	var out bytes.Buffer
	Usage(&out, testCommands())
	listing := out.String()
	alpha := strings.Index(listing, "alpha")
	zeta := strings.Index(listing, "zeta")
	if alpha == -1 || zeta == -1 {
		t.Fatalf("usage output missing commands: %q", listing)
	}
	if alpha > zeta {
		t.Errorf("commands not sorted: %q", listing)
	}
}

func TestBase64Cmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "encode", args: nil, stdin: "hi", want: "aGk=\n"},
		{name: "decode", args: []string{"-d"}, stdin: "aGk=\n", want: "hi"},
		{name: "decode invalid", args: []string{"-d"}, stdin: "!!!", wantErr: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "unexpected argument", args: []string{"file.txt"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Base64Cmd(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Base64Cmd() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Base64Cmd() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Base64Cmd() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Base64Cmd() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}
