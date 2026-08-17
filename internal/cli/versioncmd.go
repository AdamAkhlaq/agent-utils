package cli

import (
	"errors"
	"flag"
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
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("version", flag.ContinueOnError)
			fs.SetOutput(stderr)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("version: unexpected argument %q", fs.Arg(0))}
			}
			_, err := fmt.Fprintln(stdout, version)
			return err
		},
	}
}
