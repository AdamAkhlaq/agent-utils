package cli

import (
	"errors"
	"flag"
	"io"
)

// MarkdownTableCommand builds the md-table command around the text package's
// pure renderers; -csv switches the input format from JSON to CSV. No extra
// newline is appended: the renderer terminates every table row.
func MarkdownTableCommand(fromJSON, fromCSV func(w io.Writer, r io.Reader) error) Command {
	return Command{
		Name:    "md-table",
		Summary: "render a JSON array or CSV (-csv) as a GitHub-flavored Markdown table",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("md-table", flag.ContinueOnError)
			fs.SetOutput(stderr)
			csvFlag := fs.Bool("csv", false, "input is CSV with a header row instead of JSON")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			render := fromJSON
			if *csvFlag {
				render = fromCSV
			}
			return render(stdout, in)
		},
	}
}
