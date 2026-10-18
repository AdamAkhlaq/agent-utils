package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Command is one registered subcommand. Run receives the arguments after the
// command name plus the process streams, so commands never touch os.Stdin or
// os.Stdout directly and stay testable.
type Command struct {
	Name    string
	Summary string
	// Usage is the synopsis shown by -h, one invocation form per line,
	// optionally followed by a description column.
	Usage string
	Run   func(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

// UsageError marks misuse of the CLI (unknown command, bad flags) so main can
// exit 2 instead of 1. Detect it with errors.As.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string { return e.Err.Error() }

func (e *UsageError) Unwrap() error { return e.Err }

// Dispatch looks up args[0] in commands and runs it, rendering the command's
// help to stdout when it was asked for. On a missing or unknown command it
// prints usage to stderr and returns a *UsageError.
func Dispatch(commands map[string]Command, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		Usage(stderr, commands)
		return &UsageError{Err: errors.New("no command given")}
	}
	cmd, ok := commands[args[0]]
	if !ok {
		Usage(stderr, commands)
		return &UsageError{Err: fmt.Errorf("unknown command %q", args[0])}
	}
	err := cmd.Run(args[1:], stdin, stdout, stderr)
	var help *helpRequest
	if errors.As(err, &help) {
		return writeHelp(stdout, cmd, help.flags)
	}
	return err
}

// writeHelp renders a command's -h output: summary, synopsis, and flags.
func writeHelp(w io.Writer, cmd Command, fs *flag.FlagSet) error {
	var b strings.Builder
	fmt.Fprintf(&b, "agent-utils %s: %s\n", cmd.Name, cmd.Summary)
	if cmd.Usage != "" {
		b.WriteString("\nUsage:\n")
		for _, line := range strings.Split(strings.TrimRight(cmd.Usage, "\n"), "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	hasFlags := false
	fs.VisitAll(func(*flag.Flag) { hasFlags = true })
	if hasFlags {
		b.WriteString("\nFlags:\n")
		fs.SetOutput(&b)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// Usage writes the command list to w, sorted by name because map iteration
// order is randomized.
func Usage(w io.Writer, commands map[string]Command) {
	fmt.Fprintln(w, "usage: agent-utils <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "  %-10s %s\n", name, commands[name].Summary)
	}
}
