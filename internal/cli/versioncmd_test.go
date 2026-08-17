package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := VersionCommand("v1.2.3")

	t.Run("prints the bare version", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := cmd.Run(nil, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "v1.2.3\n"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
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
