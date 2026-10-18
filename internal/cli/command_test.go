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
	const list = "usage: agent-utils"
	tests := []struct {
		name       string
		args       []string
		wantUsage  bool
		wantErr    string
		wantStdout string
		wantStderr string
	}{
		{name: "no command lists commands on stderr", args: nil, wantUsage: true, wantErr: "no command given", wantStderr: list},
		{name: "unknown command hints at help", args: []string{"nonsense"}, wantUsage: true, wantErr: `unknown command "nonsense" (run 'agent-utils help' for the command list)`},
		{name: "known command", args: []string{"alpha"}},
		{name: "help lists commands on stdout", args: []string{"help"}, wantStdout: list},
		{name: "-h lists commands", args: []string{"-h"}, wantStdout: list},
		{name: "-help lists commands", args: []string{"-help"}, wantStdout: list},
		{name: "--help lists commands", args: []string{"--help"}, wantStdout: list},
		{name: "help command shows its help", args: []string{"help", "demo"}, wantStdout: "agent-utils demo: demonstrate help rendering"},
		{name: "help unknown command", args: []string{"help", "nonsense"}, wantUsage: true, wantErr: `unknown command "nonsense"`},
		{name: "help with two names", args: []string{"help", "alpha", "zeta"}, wantUsage: true, wantErr: "help: takes at most one command name, got 2"},
		{name: "-h with two names names the word typed", args: []string{"-h", "alpha", "zeta"}, wantUsage: true, wantErr: "-h: takes at most one command name, got 2"},
		{name: "help help lists commands", args: []string{"help", "help"}, wantStdout: list},
		{name: "help -h lists commands", args: []string{"help", "-h"}, wantStdout: list},
		{name: "-h --help lists commands", args: []string{"-h", "--help"}, wantStdout: list},
		{name: "help --version shows version help", args: []string{"help", "--version"}, wantStdout: "agent-utils version:"},
		{name: "--version runs version", args: []string{"--version"}, wantStdout: "v9.9.9"},
		{name: "-version runs version", args: []string{"-version"}, wantStdout: "v9.9.9"},
		{name: "--version passes extra args on", args: []string{"--version", "x"}, wantUsage: true, wantErr: `version: unexpected argument "x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := testCommands()
			commands["demo"] = flagCommand()
			commands["version"] = VersionCommand("v9.9.9")
			var stdout, stderr bytes.Buffer
			err := Dispatch(commands, tt.args, strings.NewReader(""), &stdout, &stderr)
			var usageErr *UsageError
			if got := errors.As(err, &usageErr); got != tt.wantUsage {
				t.Fatalf("Dispatch() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Dispatch() error = %v, want nil", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("Dispatch() error = %v, want containing %q", err, tt.wantErr)
			}
			if tt.wantStdout == "" && stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want containing %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr == "" && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want containing %q", stderr.String(), tt.wantStderr)
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
		Name:     "demo",
		Summary:  "demonstrate help rendering",
		Usage:    "agent-utils demo [-n count] [file]",
		Examples: []string{"agent-utils demo -n 2   # twice"},
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
			want := "agent-utils demo: demonstrate help rendering\n\nUsage:\n  agent-utils demo [-n count] [file]\n\nFlags:\n  -n int\n    \thow many (default 1)\n\nExamples:\n  agent-utils demo -n 2   # twice\n"
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

func TestUsageAlignsSummaries(t *testing.T) {
	var out bytes.Buffer
	Usage(&out, map[string]Command{
		"a":               {Name: "a", Summary: "short name"},
		"much-longer-one": {Name: "much-longer-one", Summary: "long name"},
	})
	for _, want := range []string{
		"  a                short name\n",
		"  much-longer-one  long name\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("usage missing aligned line %q in:\n%s", want, out.String())
		}
	}
}
