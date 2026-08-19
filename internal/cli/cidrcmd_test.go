package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The stubs record what they were called with, so these tests exercise only
// the CLI layer's mode dispatch, argument counting, true/false exit
// convention, and error classification, not any real subnet math.
func newCIDRStubs() (cmd Command, calls *cidrCalls) {
	calls = &cidrCalls{}
	infoJSON := func(prefix string) (string, error) {
		calls.infoPrefix = prefix
		if prefix == "bad" {
			return "", fmt.Errorf("invalid prefix %q", prefix)
		}
		return `{"network": "stub"}`, nil
	}
	contains := func(prefix, ip string) (bool, error) {
		calls.containsArgs = [2]string{prefix, ip}
		if prefix == "bad" {
			return false, fmt.Errorf("invalid prefix %q", prefix)
		}
		return ip == "10.0.0.1", nil
	}
	overlaps := func(a, b string) (bool, error) {
		calls.overlapsArgs = [2]string{a, b}
		if a == "bad" {
			return false, fmt.Errorf("first prefix: invalid prefix %q", a)
		}
		return a == b, nil
	}
	split := func(prefix string, newLen int) ([]string, error) {
		calls.splitPrefix = prefix
		calls.splitLen = newLen
		if newLen > 30 {
			return nil, fmt.Errorf("would produce 131072 subnets; refusing to print more than 65536")
		}
		return []string{"10.0.0.0/25", "10.0.0.128/25"}, nil
	}
	return CIDRCommand(infoJSON, contains, overlaps, split), calls
}

type cidrCalls struct {
	infoPrefix   string
	containsArgs [2]string
	overlapsArgs [2]string
	splitPrefix  string
	splitLen     int
}

func TestCIDRCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantOut   string
		wantErr   string
		wantUsage bool
		wantCalls func(t *testing.T, c *cidrCalls)
	}{
		{
			name: "info prints JSON", args: []string{"info", "10.0.0.0/24"}, wantOut: `{"network": "stub"}` + "\n",
			wantCalls: func(t *testing.T, c *cidrCalls) {
				if c.infoPrefix != "10.0.0.0/24" {
					t.Errorf("info called with %q, want 10.0.0.0/24", c.infoPrefix)
				}
			},
		},
		{name: "info with no arguments", args: []string{"info"}, wantErr: "exactly one prefix argument, got 0", wantUsage: true},
		{name: "info with two arguments", args: []string{"info", "a", "b"}, wantErr: "exactly one prefix argument, got 2", wantUsage: true},
		{name: "info malformed prefix is usage error", args: []string{"info", "bad"}, wantErr: `cidr info: invalid prefix "bad"`, wantUsage: true},
		{
			name: "contains true", args: []string{"contains", "10.0.0.0/24", "10.0.0.1"}, wantOut: "true\n",
			wantCalls: func(t *testing.T, c *cidrCalls) {
				if c.containsArgs != [2]string{"10.0.0.0/24", "10.0.0.1"} {
					t.Errorf("contains called with %v", c.containsArgs)
				}
			},
		},
		{name: "contains false prints false and fails", args: []string{"contains", "10.0.0.0/24", "10.9.9.9"}, wantOut: "false\n", wantErr: "cidr: 10.0.0.0/24 does not contain 10.9.9.9", wantUsage: false},
		{name: "contains malformed prefix is usage error", args: []string{"contains", "bad", "10.0.0.1"}, wantErr: `cidr contains: invalid prefix "bad"`, wantUsage: true},
		{name: "contains with one argument", args: []string{"contains", "10.0.0.0/24"}, wantErr: "a prefix and an IP argument, got 1", wantUsage: true},
		{name: "contains with three arguments", args: []string{"contains", "a", "b", "c"}, wantErr: "a prefix and an IP argument, got 3", wantUsage: true},
		{
			name: "overlaps true", args: []string{"overlaps", "10.0.0.0/24", "10.0.0.0/24"}, wantOut: "true\n",
			wantCalls: func(t *testing.T, c *cidrCalls) {
				if c.overlapsArgs != [2]string{"10.0.0.0/24", "10.0.0.0/24"} {
					t.Errorf("overlaps called with %v", c.overlapsArgs)
				}
			},
		},
		{name: "overlaps false prints false and fails", args: []string{"overlaps", "10.0.0.0/24", "10.0.1.0/24"}, wantOut: "false\n", wantErr: "cidr: 10.0.0.0/24 and 10.0.1.0/24 do not overlap", wantUsage: false},
		{name: "overlaps malformed prefix is usage error", args: []string{"overlaps", "bad", "10.0.0.0/24"}, wantErr: `cidr overlaps: first prefix: invalid prefix "bad"`, wantUsage: true},
		{name: "overlaps with one argument", args: []string{"overlaps", "10.0.0.0/24"}, wantErr: "exactly two prefix arguments, got 1", wantUsage: true},
		{
			name: "split prints one subnet per line", args: []string{"split", "10.0.0.0/24", "25"}, wantOut: "10.0.0.0/25\n10.0.0.128/25\n",
			wantCalls: func(t *testing.T, c *cidrCalls) {
				if c.splitPrefix != "10.0.0.0/24" || c.splitLen != 25 {
					t.Errorf("split called with (%q, %d), want (10.0.0.0/24, 25)", c.splitPrefix, c.splitLen)
				}
			},
		},
		{name: "split refusal is usage error", args: []string{"split", "10.0.0.0/8", "31"}, wantErr: "refusing to print more than 65536", wantUsage: true},
		{name: "split non-integer length", args: []string{"split", "10.0.0.0/24", "abc"}, wantErr: `new prefix length "abc" is not an integer`, wantUsage: true},
		{name: "split with one argument", args: []string{"split", "10.0.0.0/24"}, wantErr: "a prefix and a new prefix length, got 1", wantUsage: true},
		{name: "split with three arguments", args: []string{"split", "a", "1", "2"}, wantErr: "a prefix and a new prefix length, got 3", wantUsage: true},
		{name: "no mode", args: nil, wantErr: "expected a mode: info, contains, overlaps, or split", wantUsage: true},
		{name: "unknown mode", args: []string{"summarize", "10.0.0.0/24"}, wantErr: `unknown mode "summarize"`, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, calls := newCIDRStubs()
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if got := stdout.String(); got != tt.wantOut {
				t.Errorf("Run() stdout = %q, want %q", got, tt.wantOut)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %q, want it to contain %q", err, tt.wantErr)
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
			if tt.wantCalls != nil {
				tt.wantCalls(t, calls)
			}
		})
	}
}
