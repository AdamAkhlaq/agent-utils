package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCommandsCommand(t *testing.T) {
	noFlags := func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return parseFlags(newFlagSet("zeta"), args)
	}
	commands := map[string]Command{
		"zeta": {Name: "zeta", Summary: "does z", Run: noFlags},
		"demo": flagCommand(),
	}
	cmd := CommandsCommand(commands)
	commands[cmd.Name] = cmd

	t.Run("emits sorted JSON including itself", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := cmd.Run(nil, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		var got []manifestEntry
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
		}
		names := make([]string, len(got))
		for i, e := range got {
			names[i] = e.Name
		}
		if want := []string{"commands", "demo", "zeta"}; !slices.Equal(names, want) {
			t.Fatalf("Run() listed %v, want %v (sorted)", names, want)
		}
		wantDemo := manifestEntry{
			Name:    "demo",
			Summary: "demonstrate help rendering",
			Usage:   []string{"agent-utils demo [-n count] [file]"},
			Flags:   []manifestFlag{{Name: "-n", Type: "integer", Default: "1", Description: "how many"}},
			Examples: []Example{
				{Command: "agent-utils demo -n 2", Output: "2"},
				{Command: "agent-utils demo file.txt"},
				{Command: "agent-utils demo -n 10", Output: "10"},
			},
		}
		if !reflect.DeepEqual(got[1], wantDemo) {
			t.Errorf("entry = %+v, want %+v", got[1], wantDemo)
		}
		if !strings.HasSuffix(stdout.String(), "\n") {
			t.Error("Run() output has no trailing newline")
		}
	})

	t.Run("absent fields are empty arrays, not null", func(t *testing.T) {
		var stdout bytes.Buffer
		if err := cmd.Run(nil, strings.NewReader(""), &stdout, io.Discard); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if strings.Contains(stdout.String(), "null") {
			t.Errorf("manifest contains null:\n%s", stdout.String())
		}
	})

	t.Run("command that does work before parsing flags is reported", func(t *testing.T) {
		eager := map[string]Command{"eager": {Name: "eager", Summary: "reads first", Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			if _, err := io.ReadAll(stdin); err != nil {
				return err
			}
			return parseFlags(newFlagSet("eager"), args)
		}}}
		err := CommandsCommand(eager).Run(nil, strings.NewReader(""), io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "eager did not answer -h") {
			t.Errorf("Run() error = %v, want it to name the eager command", err)
		}
	})

	t.Run("command that does not answer -h is reported", func(t *testing.T) {
		broken := map[string]Command{"broken": {Name: "broken", Summary: "ignores flags", Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return nil
		}}}
		err := CommandsCommand(broken).Run(nil, strings.NewReader(""), io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "broken did not answer -h") {
			t.Errorf("Run() error = %v, want it to name the broken command", err)
		}
	})

	t.Run("unexpected argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := cmd.Run([]string{"stray"}, strings.NewReader(""), &stdout, &stderr)
		if err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		var usageErr *UsageError
		if !errors.As(err, &usageErr) {
			t.Errorf("Run() error = %v, want a usage error", err)
		}
	})
}
