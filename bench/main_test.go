package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
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
		{"validate static", []string{"validate", "-static", "-root", ".."}, 0, "static checks: 5 tasks ok", "coverage:"},
		{"validate one task", []string{"validate", "-static", "-root", "..", "-tasks", "jwt-decode/*"}, 0, "static checks: 1 task ok", ""},
		{"validate outside a repository", []string{"validate", "-static", "-root", t.TempDir()}, 1, "", "docs/commands: no such file or directory"},
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
