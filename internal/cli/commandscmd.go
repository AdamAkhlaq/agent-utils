package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
)

// CommandsCommand builds the commands meta-command: the machine-readable
// counterpart to Usage, emitting every registered command as JSON so agents
// and scripts can discover the tool surface without parsing help text. It
// takes the registry map it will itself be part of; map values added after
// construction are visible at run time.
func CommandsCommand(commands map[string]Command) Command {
	return Command{
		Name:    "commands",
		Summary: "list every command as JSON (machine-readable tool discovery)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("commands", flag.ContinueOnError)
			fs.SetOutput(stderr)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("commands: unexpected argument %q", fs.Arg(0))}
			}
			type entry struct {
				Name    string `json:"name"`
				Summary string `json:"summary"`
			}
			entries := make([]entry, 0, len(commands))
			for _, cmd := range commands {
				entries = append(entries, entry{Name: cmd.Name, Summary: cmd.Summary})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
			out, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("encoding command list: %w", err)
			}
			if _, err := stdout.Write(out); err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout)
			return err
		},
	}
}
