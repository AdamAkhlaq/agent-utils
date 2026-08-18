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

// The stubs record what they were called with, so these tests exercise only
// the CLI layer's mode selection, flag handling, and error classification,
// not any real SemVer logic.
func newSemverStubs() (cmd Command, calls *semverCalls) {
	calls = &semverCalls{}
	sortStream := func(w io.Writer, r io.Reader, descending bool) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		calls.sortInput = string(data)
		calls.sortDescending = descending
		calls.sorted = true
		_, err = fmt.Fprintln(w, "sorted")
		return err
	}
	compare := func(a, b string) (int, error) {
		calls.compared = [2]string{a, b}
		if a == "bad" {
			return 0, fmt.Errorf("first version: invalid version %q", a)
		}
		return -1, nil
	}
	compileCheck := func(constraint string) (func(version string) (bool, error), error) {
		if constraint == "" || strings.HasPrefix(constraint, "^") {
			return nil, fmt.Errorf("unsupported constraint %q", constraint)
		}
		calls.constraint = constraint
		return func(version string) (bool, error) {
			calls.checkedVersion = version
			switch version {
			case "bad":
				return false, fmt.Errorf("invalid version %q", version)
			case "0.1.0":
				return false, nil
			}
			return true, nil
		}, nil
	}
	return SemverCommand(sortStream, compare, compileCheck), calls
}

type semverCalls struct {
	sorted         bool
	sortInput      string
	sortDescending bool
	compared       [2]string
	constraint     string
	checkedVersion string
}

func TestSemverCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantOut   string
		wantErr   string
		wantUsage bool
		wantCalls func(t *testing.T, c *semverCalls)
	}{
		{
			name: "default mode sorts stdin", args: nil, stdin: "2.0.0\n1.0.0\n", wantOut: "sorted\n",
			wantCalls: func(t *testing.T, c *semverCalls) {
				if !c.sorted || c.sortInput != "2.0.0\n1.0.0\n" || c.sortDescending {
					t.Errorf("sort call = %+v, want ascending sort of the stdin data", c)
				}
			},
		},
		{
			name: "descending sort", args: []string{"-r"}, stdin: "1.0.0\n", wantOut: "sorted\n",
			wantCalls: func(t *testing.T, c *semverCalls) {
				if !c.sortDescending {
					t.Error("sort call was ascending, want descending")
				}
			},
		},
		{
			name: "compare prints result", args: []string{"-compare", "1.0.0", "2.0.0"}, wantOut: "-1\n",
			wantCalls: func(t *testing.T, c *semverCalls) {
				if c.compared != [2]string{"1.0.0", "2.0.0"} {
					t.Errorf("compare call = %v, want [1.0.0 2.0.0]", c.compared)
				}
			},
		},
		{name: "compare with no arguments", args: []string{"-compare"}, wantErr: "exactly two version arguments, got 0", wantUsage: true},
		{name: "compare with one argument", args: []string{"-compare", "1.0.0"}, wantErr: "exactly two version arguments, got 1", wantUsage: true},
		{name: "compare with three arguments", args: []string{"-compare", "1.0.0", "2.0.0", "3.0.0"}, wantErr: "exactly two version arguments, got 3", wantUsage: true},
		{name: "compare invalid version is runtime error", args: []string{"-compare", "bad", "2.0.0"}, wantErr: `semver: first version: invalid version "bad"`, wantUsage: false},
		{
			name: "check satisfied from argument", args: []string{"-check", ">=1.0.0", "1.2.3"}, wantOut: "true\n",
			wantCalls: func(t *testing.T, c *semverCalls) {
				if c.constraint != ">=1.0.0" || c.checkedVersion != "1.2.3" {
					t.Errorf("check call = %+v, want constraint >=1.0.0 against 1.2.3", c)
				}
			},
		},
		{
			name: "check reads and trims stdin", args: []string{"-check", ">=1.0.0"}, stdin: "  v1.2.3\n", wantOut: "true\n",
			wantCalls: func(t *testing.T, c *semverCalls) {
				if c.checkedVersion != "v1.2.3" {
					t.Errorf("checked version = %q, want %q", c.checkedVersion, "v1.2.3")
				}
			},
		},
		{name: "check unsatisfied prints false and fails", args: []string{"-check", ">=1.0.0", "0.1.0"}, wantOut: "false\n", wantErr: `semver: 0.1.0 does not satisfy ">=1.0.0"`, wantUsage: false},
		{name: "check invalid version is runtime error", args: []string{"-check", ">=1.0.0", "bad"}, wantErr: `semver: invalid version "bad"`, wantUsage: false},
		{name: "check bad constraint is usage error", args: []string{"-check", "^1.0.0"}, wantErr: `semver: unsupported constraint "^1.0.0"`, wantUsage: true},
		{name: "check empty constraint is usage error", args: []string{"-check", ""}, wantErr: `semver: unsupported constraint ""`, wantUsage: true},
		{name: "check with two arguments", args: []string{"-check", ">=1.0.0", "1.0.0", "2.0.0"}, wantErr: "at most one version argument, got 2", wantUsage: true},
		{name: "compare and check are exclusive", args: []string{"-compare", "-check", ">=1.0.0"}, wantErr: "-compare and -check are mutually exclusive", wantUsage: true},
		{name: "r with compare", args: []string{"-r", "-compare", "1.0.0", "2.0.0"}, wantErr: "-r only applies when sorting", wantUsage: true},
		{name: "r with check", args: []string{"-r", "-check", ">=1.0.0"}, wantErr: "-r only applies when sorting", wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
		{name: "sort with two file arguments", args: []string{"a", "b"}, wantErr: "at most one file argument", wantUsage: true},
		{name: "sort with missing file", args: []string{"no-such-file"}, wantErr: "no such file", wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, calls := newSemverStubs()
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
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

func TestSemverCommandFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versions.txt")
	if err := os.WriteFile(path, []byte("2.0.0\n1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, calls := newSemverStubs()
	var stdout, stderr bytes.Buffer
	if err := cmd.Run([]string{path}, strings.NewReader("not this"), &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calls.sortInput != "2.0.0\n1.0.0\n" {
		t.Errorf("Run() sorted %q, want the file contents", calls.sortInput)
	}
}

func TestSemverCommandSortErrorIsRuntimeError(t *testing.T) {
	lineErr := errors.New(`line 2: invalid version "1.2"`)
	cmd := SemverCommand(
		func(w io.Writer, r io.Reader, descending bool) error { return lineErr },
		nil,
		nil,
	)
	var stdout, stderr bytes.Buffer
	err := cmd.Run(nil, strings.NewReader("1.0.0\n1.2\n"), &stdout, &stderr)
	if !errors.Is(err, lineErr) {
		t.Fatalf("Run() error = %v, want it to wrap %v", err, lineErr)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; invalid input data must exit 1, not 2")
	}
}
