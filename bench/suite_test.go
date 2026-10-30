package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSuiteStatic is the check CI runs on every task directory: the static
// rules, the context files' equality of length, the tool paragraph's match
// with the README, and a task for every command that is not excluded. It
// needs no Docker and no credential.
func TestSuiteStatic(t *testing.T) {
	result, err := validate("..", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.tasks) == 0 {
		t.Fatal("no tasks found under bench/tasks")
	}
	for _, p := range result.problems {
		t.Error(p)
	}
}

// TestValidateCoverage runs "bench validate" on this repository with one
// extra command page, so the suite has a gap: the whole suite fails on it,
// and a -tasks glob, which restricts every check to its tasks, does not.
func TestValidateCoverage(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"README.md", "bench"} {
		if err := os.Symlink(filepath.Join(repo, name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	commands, err := commandNames(filepath.Join(repo, "docs", "commands"))
	if err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(root, "docs", "commands")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range append(sortedKeys(commands), "uncovered") {
		if err := os.WriteFile(filepath.Join(docs, name+".md"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tasks, _ := loadTasks(filepath.Join(repo, "bench", "tasks"))

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"whole suite", nil, 1, "",
			fmt.Sprintf("command \"uncovered\" has no task under bench/tasks/uncovered/ and no exclusion\n1 problem (%d tasks checked)\n", len(tasks))},
		{"one command's tasks", []string{"-tasks", "jwt-decode/*"}, 0, "static checks: 2 tasks ok\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append([]string{"validate", "-static", "-root", root}, tt.args...), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if stderr.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// TestSuiteOracles builds the images and runs every oracle through its
// checker. It needs a Docker daemon and network access to build, so it runs
// only with BENCH_DOCKER=1; "go run ./bench validate" does the same.
func TestSuiteOracles(t *testing.T) {
	if os.Getenv("BENCH_DOCKER") != "1" {
		t.Skip("set BENCH_DOCKER=1 to run every oracle through its checker in Docker")
	}
	if err := dockerAvailable(); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	version, err := latestTag(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := buildImages(root, version); err != nil {
		t.Fatal(err)
	}
	result, err := validate(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range result.tasks {
		t.Run(task.Name, func(t *testing.T) {
			if err := checkOracle(task); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestExcludedCommandsHaveReasons keeps the exclusion list honest: every
// entry names a real command, with a reason the coverage output can print.
func TestExcludedCommandsHaveReasons(t *testing.T) {
	commands, err := commandNames(filepath.Join("..", "docs", "commands"))
	if err != nil {
		t.Fatal(err)
	}
	for name, reason := range excludedCommands {
		if !commands[name] {
			t.Errorf("excluded command %q does not exist", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("excluded command %q has no reason", name)
		}
	}
}
