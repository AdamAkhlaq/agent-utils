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
			if tt.wantUsage && !strings.Contains(stderr.String(), "usage: agent-utils") {
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

func flagCommand() Command {
	return Command{
		Name:    "demo",
		Summary: "demonstrate help rendering",
		Usage:   "agent-utils demo [-n count] [file]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("demo")
			fs.Int("n", 1, "how many")
			return parseFlags(fs, args)
		},
	}
}

func TestDispatchCommandHelp(t *testing.T) {
	for _, flag := range []string{"-h", "-help", "--help"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			commands := map[string]Command{"demo": flagCommand()}
			if err := Dispatch(commands, []string{"demo", flag}, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("Dispatch() error = %v", err)
			}
			want := "agent-utils demo: demonstrate help rendering\n\nUsage:\n  agent-utils demo [-n count] [file]\n\nFlags:\n  -n int\n    \thow many (default 1)\n"
			if stdout.String() != want {
				t.Errorf("help = %q, want %q", stdout.String(), want)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestDispatchCommandHelpWithoutFlags(t *testing.T) {
	var stdout bytes.Buffer
	cmd := Command{Name: "bare", Summary: "no flags", Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return parseFlags(newFlagSet("bare"), args)
	}}
	if err := Dispatch(map[string]Command{"bare": cmd}, []string{"bare", "-h"}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if want := "agent-utils bare: no flags\n"; stdout.String() != want {
		t.Errorf("help = %q, want %q", stdout.String(), want)
	}
}

func TestDispatchBadFlagReportsOnce(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Dispatch(map[string]Command{"demo": flagCommand()}, []string{"demo", "-x"}, strings.NewReader(""), &stdout, &stderr)
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("Dispatch() error = %v, want *UsageError", err)
	}
	if want := "demo: flag provided but not defined: -x (run 'agent-utils demo -h' for usage)"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("flag package wrote output itself: stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}
