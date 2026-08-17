package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCommandsCommand(t *testing.T) {
	commands := map[string]Command{
		"zeta":  {Name: "zeta", Summary: "does z"},
		"alpha": {Name: "alpha", Summary: "does a"},
	}
	cmd := CommandsCommand(commands)
	commands[cmd.Name] = cmd

	t.Run("emits sorted JSON including itself", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := cmd.Run(nil, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		var got []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
		}
		names := make([]string, len(got))
		for i, e := range got {
			names[i] = e.Name
		}
		want := []string{"alpha", "commands", "zeta"}
		if len(names) != len(want) {
			t.Fatalf("Run() listed %v, want %v", names, want)
		}
		for i := range want {
			if names[i] != want[i] {
				t.Fatalf("Run() listed %v, want %v (sorted)", names, want)
			}
		}
		if got[0].Summary != "does a" {
			t.Errorf("alpha summary = %q, want %q", got[0].Summary, "does a")
		}
		if !strings.HasSuffix(stdout.String(), "\n") {
			t.Error("Run() output has no trailing newline")
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
