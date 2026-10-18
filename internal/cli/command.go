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
// help to stdout when it was asked for. It also answers the conventional entry
// points (help, -h, --help, help <command>, --version), since they are the
// first thing a human or agent tries. A missing command prints the command
// list to stderr; every usage failure returns a *UsageError.
func Dispatch(commands map[string]Command, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		Usage(stderr, commands)
		return &UsageError{Err: errors.New("no command given")}
	}
	name, rest := args[0], args[1:]
	if isHelpWord(name) {
		switch len(rest) {
		case 0:
			Usage(stdout, commands)
			return nil
		case 1:
			name, rest = rest[0], []string{"-h"}
		default:
			return &UsageError{Err: fmt.Errorf("%s: takes at most one command name, got %d", name, len(rest))}
		}
		if isHelpWord(name) {
			Usage(stdout, commands)
			return nil
		}
	}
	if name == "-version" || name == "--version" {
		name = "version"
	}
	cmd, ok := commands[name]
	if !ok {
		return &UsageError{Err: fmt.Errorf("unknown command %q (run 'agent-utils help' for the command list)", name)}
	}
	err := cmd.Run(rest, stdin, stdout, stderr)
	var help *helpRequest
	if errors.As(err, &help) {
		return writeHelp(stdout, cmd, help.flags)
	}
	return err
}

func isHelpWord(arg string) bool {
	switch arg {
	case "help", "-h", "-help", "--help":
		return true
	}
	return false
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
// order is randomized, with summaries aligned past the longest name.
func Usage(w io.Writer, commands map[string]Command) {
	fmt.Fprintln(w, "usage: agent-utils <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	names := make([]string, 0, len(commands))
	width := 0
	for name := range commands {
		names = append(names, name)
		width = max(width, len(name))
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "  %-*s  %s\n", width, name, commands[name].Summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'agent-utils <command> -h' for a command's usage, flags, and examples,")
	fmt.Fprintln(w, "or 'agent-utils commands' for all of it as JSON.")
}
