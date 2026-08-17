package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func testCommands() map[string]Command {
	nop := func(args []string, stdin io.Reader, stdout, stderr io.Writer) error { return nil }
	return map[string]Command{
		"zeta":  {Name: "zeta", Summary: "last alphabetically", Run: nop},
		"alpha": {Name: "alpha", Summary: "first alphabetically", Run: nop},
	}
}

func TestDispatch(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantUsage bool
	}{
		{name: "no command", args: nil, wantUsage: true},
		{name: "unknown command", args: []string{"nonsense"}, wantUsage: true},
		{name: "known command", args: []string{"alpha"}, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Dispatch(testCommands(), tt.args, strings.NewReader(""), &stdout, &stderr)
			var usageErr *UsageError
			if got := errors.As(err, &usageErr); got != tt.wantUsage {
				t.Fatalf("Dispatch() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
			}
			if tt.wantUsage && !strings.Contains(stderr.String(), "usage: agent-utils") {
				t.Errorf("expected usage on stderr, got %q", stderr.String())
			}
		})
	}
}

func TestUsageSortsCommands(t *testing.T) {
	var out bytes.Buffer
	Usage(&out, testCommands())
	listing := out.String()
	alpha := strings.Index(listing, "alpha")
	zeta := strings.Index(listing, "zeta")
	if alpha == -1 || zeta == -1 {
		t.Fatalf("usage output missing commands: %q", listing)
	}
	if alpha > zeta {
		t.Errorf("commands not sorted: %q", listing)
	}
}
