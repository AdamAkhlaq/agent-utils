package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stub returns a fixed digest and records the algorithms it was asked
// for, so these tests exercise only the CLI layer's flag handling, verify
// logic, and error classification, not any real hashing. The command probes
// the algorithm with empty input first, so the last recorded call is the one
// that hashed the real input.
func TestHashCommand(t *testing.T) {
	const digest = "deadbeef"
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantAlgo  string
		wantErr   string
		wantUsage bool
	}{
		{name: "default sha256", args: nil, stdin: "abc", want: digest + "\n", wantAlgo: "sha256"},
		{name: "explicit algorithm", args: []string{"-a", "md5"}, stdin: "abc", want: digest + "\n", wantAlgo: "md5"},
		{name: "empty input", args: nil, stdin: "", want: digest + "\n", wantAlgo: "sha256"},
		{name: "verify match prints nothing", args: []string{"-c", digest}, stdin: "abc", want: "", wantAlgo: "sha256"},
		{name: "verify is case-insensitive", args: []string{"-c", "DEADBEEF"}, stdin: "abc", want: "", wantAlgo: "sha256"},
		{name: "verify trims whitespace", args: []string{"-c", " deadbeef\n"}, stdin: "abc", want: "", wantAlgo: "sha256"},
		{name: "verify mismatch is runtime error", args: []string{"-c", "deadbee0"}, stdin: "abc", wantErr: "hash: checksum mismatch: expected deadbee0, got deadbeef", wantUsage: false},
		{name: "verify value not hex", args: []string{"-c", "zzzzzzzz"}, wantErr: "hash: -c value must be 8 hex characters", wantUsage: true},
		{name: "verify value wrong length", args: []string{"-c", "dead"}, wantErr: "hash: -c value must be 8 hex characters", wantUsage: true},
		{name: "verify value empty", args: []string{"-c", ""}, wantErr: "hash: -c value must be 8 hex characters", wantUsage: true},
		{name: "unknown algorithm", args: []string{"-a", "nope"}, wantErr: "hash: unknown algorithm", wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
		{name: "two file arguments", args: []string{"a", "b"}, wantErr: "at most one file argument", wantUsage: true},
		{name: "missing file", args: []string{"no-such-file"}, wantErr: "no such file", wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var algos []string
			sum := func(algo string, r io.Reader) (string, error) {
				if _, err := io.Copy(io.Discard, r); err != nil {
					return "", err
				}
				algos = append(algos, algo)
				if algo != "sha256" && algo != "sha1" && algo != "sha512" && algo != "md5" {
					return "", fmt.Errorf("unknown algorithm %q (valid: sha256, sha1, sha512, md5)", algo)
				}
				return digest, nil
			}
			cmd := HashCommand(sum)
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
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
			if got := algos[len(algos)-1]; got != tt.wantAlgo {
				t.Errorf("Run() hashed with %q, want %q", got, tt.wantAlgo)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHashCommandFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("file data"), 0o600); err != nil {
		t.Fatal(err)
	}
	var hashed string
	cmd := HashCommand(func(algo string, r io.Reader) (string, error) {
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		hashed = string(data)
		return "cafe", nil
	})
	var stdout, stderr bytes.Buffer
	if err := cmd.Run([]string{path}, strings.NewReader("not this"), &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if hashed != "file data" {
		t.Errorf("Run() hashed %q, want %q", hashed, "file data")
	}
	if got := stdout.String(); got != "cafe\n" {
		t.Errorf("Run() stdout = %q, want %q", got, "cafe\n")
	}
}

func TestHashCommandReadFailureIsRuntimeError(t *testing.T) {
	readFail := errors.New("read failed")
	cmd := HashCommand(func(algo string, r io.Reader) (string, error) {
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		// Succeed on the empty probe so the failure hits the real input read.
		if len(data) == 0 {
			return "deadbeef", nil
		}
		return "", readFail
	})
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader("abc"), &stdout, &stderr)
	if !errors.Is(err, readFail) {
		t.Fatalf("Run() error = %v, want it to wrap %v", err, readFail)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; a failed read must exit 1, not 2")
	}
	if stdout.Len() != 0 {
		t.Errorf("Run() stdout = %q, want empty", stdout.String())
	}
}
