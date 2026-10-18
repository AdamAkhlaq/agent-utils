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
// counterpart to Usage, emitting every registered command with its synopsis,
// flags, and examples as JSON so agents and scripts can learn the whole tool
// surface in one call. It takes the registry map it will itself be part of;
// map values added after construction are visible at run time.
func CommandsCommand(commands map[string]Command) Command {
	return Command{
		Name:    "commands",
		Summary: "list every command with its usage, flags, and examples as JSON",
		Usage:   "agent-utils commands",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("commands")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("commands: unexpected argument %q", fs.Arg(0))}
			}
			entries := make([]manifestEntry, 0, len(commands))
			for _, cmd := range commands {
				entry, err := newManifestEntry(cmd)
				if err != nil {
					return err
				}
				entries = append(entries, entry)
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

type manifestEntry struct {
	Name     string         `json:"name"`
	Summary  string         `json:"summary"`
	Usage    []string       `json:"usage"`
	Flags    []manifestFlag `json:"flags"`
	Examples []Example      `json:"examples"`
}

type manifestFlag struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

// newManifestEntry describes cmd. Its flags are only known inside its Run
// function, so they are collected by asking it for -h, which every command
// must answer by parsing flags before anything else; the streams fail if
// touched, so one that does work first is reported instead of misdescribed.
func newManifestEntry(cmd Command) (manifestEntry, error) {
	entry := manifestEntry{
		Name:     cmd.Name,
		Summary:  cmd.Summary,
		Usage:    cmd.usageLines(),
		Flags:    []manifestFlag{},
		Examples: append([]Example{}, cmd.Examples...),
	}
	var help *helpRequest
	if err := cmd.Run([]string{"-h"}, untouchable{}, untouchable{}, untouchable{}); !errors.As(err, &help) {
		return manifestEntry{}, fmt.Errorf("commands: %s did not answer -h with its flags (got %v)", cmd.Name, err)
	}
	help.flags.VisitAll(func(f *flag.Flag) {
		entry.Flags = append(entry.Flags, manifestFlag{
			Name:        "-" + f.Name,
			Type:        flagType(f),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})
	return entry, nil
}

// flagType names a flag's value type in JSON Schema terms, the vocabulary
// tool-definition formats for agents use.
func flagType(f *flag.Flag) string {
	getter, ok := f.Value.(flag.Getter)
	if !ok {
		return "string"
	}
	switch getter.Get().(type) {
	case bool:
		return "boolean"
	case int, int64, uint, uint64:
		return "integer"
	case float64:
		return "number"
	default:
		return "string"
	}
}

// untouchable is a stream that fails on any use.
type untouchable struct{}

var errTouched = errors.New("stream used while answering -h")

func (untouchable) Read([]byte) (int, error)  { return 0, errTouched }
func (untouchable) Write([]byte) (int, error) { return 0, errTouched }
