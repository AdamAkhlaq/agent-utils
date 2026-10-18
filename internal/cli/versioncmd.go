package cli

import (
	"fmt"
	"io"
)

// VersionCommand builds the version command around a fixed version string;
// main resolves what that string is (ldflags, build info, or "dev"). Output
// is the bare version, one line, so scripts and agents can use it directly.
func VersionCommand(version string) Command {
	return Command{
		Name:    "version",
		Summary: "print the agent-utils version",
		Usage:   "agent-utils version",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("version")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("version: unexpected argument %q", fs.Arg(0))}
			}
			_, err := fmt.Fprintln(stdout, version)
			return err
		},
	}
}
