package cli

import (
	"fmt"
	"io"
	"os"
)

// JSONSchemaCommand builds the json-schema command around a schema compiler,
// keeping this package independent of the format package; main wires the
// layers together. compile reads the schema file and returns a validator,
// which reads one document and returns the JSON report and how many
// violations it lists. A document that fails its schema prints the report
// and exits 3; a broken schema or unreadable input exits 1.
func JSONSchemaCommand(compile func(schemaPath string) (func(r io.Reader) (report string, violations int, err error), error)) Command {
	return Command{
		Name:    "json-schema",
		Summary: "validate JSON against a JSON Schema: JSON report of each violation, exit 3 if invalid",
		Usage:   "agent-utils json-schema <schema-file> [file]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("json-schema")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if fs.NArg() < 1 || fs.NArg() > 2 {
				return &UsageError{Err: fmt.Errorf("json-schema: expected a schema file and at most one input file, got %d argument(s)", fs.NArg())}
			}
			schemaPath := fs.Arg(0)
			validate, err := compile(schemaPath)
			if err != nil {
				return fmt.Errorf("json-schema: %w", err)
			}

			in := io.NopCloser(stdin)
			inputName := "stdin"
			if fs.NArg() == 2 {
				inputName = fs.Arg(1)
				f, err := os.Open(inputName)
				if err != nil {
					return fmt.Errorf("json-schema: %w", err)
				}
				in = f
			}
			defer in.Close()
			report, violations, err := validate(in)
			if err != nil {
				return fmt.Errorf("json-schema: %s: %w", inputName, err)
			}
			if _, err := fmt.Fprintln(stdout, report); err != nil {
				return err
			}
			if violations > 0 {
				noun := "errors"
				if violations == 1 {
					noun = "error"
				}
				return &FalseError{Err: fmt.Errorf("json-schema: %s does not match %s: %d %s", inputName, schemaPath, violations, noun)}
			}
			return nil
		},
	}
}
