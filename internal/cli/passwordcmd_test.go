package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The stub generator records its arguments and returns predictable values,
// so these tests exercise only the CLI layer's flag handling and looping.
func TestPasswordCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		want        string
		wantLength  int
		wantSymbols bool
		wantErr     bool
		wantUsage   bool
	}{
		{name: "defaults", args: nil, want: "pw-1\n", wantLength: 20, wantSymbols: true},
		{name: "custom length", args: []string{"-l", "32"}, want: "pw-1\n", wantLength: 32, wantSymbols: true},
		{name: "no symbols", args: []string{"-no-symbols"}, want: "pw-1\n", wantLength: 20, wantSymbols: false},
		{name: "count of three", args: []string{"-n", "3"}, want: "pw-1\npw-2\npw-3\n", wantLength: 20, wantSymbols: true},
		{name: "zero length", args: []string{"-l", "0"}, wantErr: true, wantUsage: true},
		{name: "zero count", args: []string{"-n", "0"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "unexpected argument", args: []string{"stray"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			var gotLength int
			var gotSymbols bool
			cmd := PasswordCommand(func(length int, symbols bool) (string, error) {
				calls++
				gotLength, gotSymbols = length, symbols
				return fmt.Sprintf("pw-%d", calls), nil
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
			if gotLength != tt.wantLength || gotSymbols != tt.wantSymbols {
				t.Errorf("Run() called generator with (length=%d, symbols=%v), want (%d, %v)",
					gotLength, gotSymbols, tt.wantLength, tt.wantSymbols)
			}
		})
	}
}

func TestPasswordCommandGeneratorFailureIsRuntimeError(t *testing.T) {
	broken := errors.New("entropy source unavailable")
	cmd := PasswordCommand(func(length int, symbols bool) (string, error) { return "", broken })
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
