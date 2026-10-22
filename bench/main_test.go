package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunWithoutToken covers the precondition "bench run" checks after its
// flags: no credential, no run. The token variable is cleared for the test.
func TestRunWithoutToken(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "-model", "claude-sonnet-5-5", "-trials", "1"}, &stdout, &stderr); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "CLAUDE_CODE_OAUTH_TOKEN is not set") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRun(t *testing.T) {
	// The suite grows with every category PR, so the static check's count comes
	// from the task directories on disk rather than a literal.
	taskDirs, err := filepath.Glob(filepath.Join("..", "bench", "tasks", "*", "*", "task.toml"))
	if err != nil || len(taskDirs) == 0 {
		t.Fatalf("no task directories found: %v", err)
	}
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no command", nil, 2, "", "usage:"},
		{"unknown command", []string{"frobnicate"}, 2, "", `unknown command "frobnicate"`},
		{"help", []string{"help"}, 0, "usage:", ""},
		{"validate help", []string{"validate", "-h"}, 0, "usage: go run ./bench validate", ""},
		{"validate unknown flag", []string{"validate", "-trials", "3"}, 2, "", "bench validate: flag provided but not defined: -trials"},
		{"validate stray argument", []string{"validate", "-static", "jwt-decode"}, 2, "", `unexpected argument "jwt-decode"`},
		{"validate bad glob", []string{"validate", "-static", "-root", "..", "-tasks", "["}, 2, "", "-tasks \"[\": syntax error in pattern"},
		{"validate no match", []string{"validate", "-static", "-root", "..", "-tasks", "nothing/*"}, 2, "", `-tasks "nothing/*" matches no task`},
		{"validate static", []string{"validate", "-static", "-root", ".."}, 0, fmt.Sprintf("static checks: %d tasks ok", len(taskDirs)), "coverage:"},
		{"validate one task", []string{"validate", "-static", "-root", "..", "-tasks", "jwt-decode/basic"}, 0, "static checks: 1 task ok", ""},
		{"validate outside a repository", []string{"validate", "-static", "-root", t.TempDir()}, 1, "", "docs/commands: no such file or directory"},
		{"run help", []string{"run", "-h"}, 0, "usage: go run ./bench run", ""},
		{"run without model", []string{"run", "-trials", "1"}, 2, "", "-model is required"},
		{"run without trials", []string{"run", "-model", "claude-sonnet-5-5"}, 2, "", "-trials must be positive"},
		{"run unknown arm", []string{"run", "-model", "m", "-trials", "1", "-arms", "tool,cli"}, 2, "", `unknown arm "cli"`},
		{"run unknown tag", []string{"run", "-model", "m", "-trials", "1", "-tag", "easy"}, 2, "", `unknown tag "easy"`},
		{"run bad glob", []string{"run", "-model", "m", "-trials", "1", "-tasks", "["}, 2, "", "syntax error in pattern"},
		{"run stray argument", []string{"run", "-model", "m", "-trials", "1", "extra"}, 2, "", `unexpected argument "extra"`},
		{"report help", []string{"report", "-h"}, 0, "usage: go run ./bench report", ""},
		{"report without a directory", []string{"report"}, 2, "", "at least one run directory is required"},
		{"report missing directory", []string{"report", filepath.Join(t.TempDir(), "none")}, 1, "", "no trials in trials.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
