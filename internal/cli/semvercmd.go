package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// SemverCommand builds the semver command around the semver package's pure
// functions, keeping this package independent of it; main wires the layers
// together. compileCheck turns a constraint into a matcher, so a bad
// constraint (a flag value) is reported as a usage error while a bad version
// (data) stays a runtime error.
func SemverCommand(
	sortStream func(w io.Writer, r io.Reader, descending bool) error,
	compare func(a, b string) (int, error),
	compileCheck func(constraint string) (func(version string) (bool, error), error),
) Command {
	return Command{
		Name:    "semver",
		Summary: "sort SemVer versions (-r desc), compare two (-compare), or test a constraint (-check)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("semver", flag.ContinueOnError)
			fs.SetOutput(stderr)
			desc := fs.Bool("r", false, "sort in descending order")
			compareMode := fs.Bool("compare", false, "compare two version arguments: print -1, 0, or 1")
			constraint := fs.String("check", "", `test a version against a constraint like ">=1.2.0 <2.0.0": print true/false, exit 0/1`)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			// An empty -check value must still select check mode (and then
			// fail as an empty constraint), so mode detection has to look at
			// whether the flag was passed, not at its value.
			checkMode := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "check" {
					checkMode = true
				}
			})
			if *compareMode && checkMode {
				return &UsageError{Err: fmt.Errorf("semver: -compare and -check are mutually exclusive")}
			}
			if *desc && (*compareMode || checkMode) {
				return &UsageError{Err: fmt.Errorf("semver: -r only applies when sorting")}
			}

			switch {
			case *compareMode:
				if fs.NArg() != 2 {
					return &UsageError{Err: fmt.Errorf("semver: -compare takes exactly two version arguments, got %d", fs.NArg())}
				}
				result, err := compare(fs.Arg(0), fs.Arg(1))
				if err != nil {
					return fmt.Errorf("semver: %w", err)
				}
				_, err = fmt.Fprintln(stdout, result)
				return err
			case checkMode:
				match, err := compileCheck(*constraint)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("semver: %w", err)}
				}
				var version string
				switch fs.NArg() {
				case 0:
					data, err := io.ReadAll(stdin)
					if err != nil {
						return fmt.Errorf("reading input: %w", err)
					}
					version = strings.TrimSpace(string(data))
				case 1:
					version = fs.Arg(0)
				default:
					return &UsageError{Err: fmt.Errorf("semver: -check takes at most one version argument, got %d", fs.NArg())}
				}
				ok, err := match(version)
				if err != nil {
					return fmt.Errorf("semver: %w", err)
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "false"); err != nil {
						return err
					}
					return fmt.Errorf("semver: %s does not satisfy %q", version, *constraint)
				}
				_, err = fmt.Fprintln(stdout, "true")
				return err
			default:
				in, err := openInput(fs, stdin)
				if err != nil {
					return err
				}
				defer in.Close()
				if err := sortStream(stdout, in, *desc); err != nil {
					return fmt.Errorf("semver: %w", err)
				}
				return nil
			}
		},
	}
}
