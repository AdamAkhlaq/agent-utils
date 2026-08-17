package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The stub generator counts its calls and returns predictable values, so
// these tests exercise only the CLI layer's flag handling and looping.
func TestUUIDCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      string
		wantCalls int
		wantErr   bool
		wantUsage bool
	}{
		{name: "default is one", args: nil, want: "id-1\n", wantCalls: 1},
		{name: "count of three", args: []string{"-n", "3"}, want: "id-1\nid-2\nid-3\n", wantCalls: 3},
		{name: "zero count", args: []string{"-n", "0"}, wantErr: true, wantUsage: true},
		{name: "negative count", args: []string{"-n", "-5"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "unexpected argument", args: []string{"stray"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			cmd := UUIDCommand(func() (string, error) {
				calls++
				return fmt.Sprintf("id-%d", calls), nil
			})
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
			if calls != tt.wantCalls {
				t.Errorf("Run() called the generator %d times, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestUUIDCommandGeneratorFailureIsRuntimeError(t *testing.T) {
	broken := errors.New("entropy source unavailable")
	cmd := UUIDCommand(func() (string, error) { return "", broken })
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader(""), &stdout, &stderr)
	if !errors.Is(err, broken) {
		t.Fatalf("Run() error = %v, want %v", err, broken)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; a generator failure must exit 1, not 2")
	}
}
