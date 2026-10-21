package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSuiteStatic is the check CI runs on every task directory: the static
// rules, the context files' equality of length, and the tool paragraph's
// match with the README. It needs no Docker and no credential.
func TestSuiteStatic(t *testing.T) {
	result, err := validate("..", "", false)
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

// TestSuiteCoverage fails when a command has no task and no exclusion. It
// is skipped unless BENCH_COVERAGE=1 until the task suite covers every
// command; that phase removes the skip.
func TestSuiteCoverage(t *testing.T) {
	if os.Getenv("BENCH_COVERAGE") != "1" {
		t.Skip("set BENCH_COVERAGE=1 to require a task for every command (the suite is still being filled in)")
	}
	result, err := validate("..", "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range result.problems {
		t.Error(p)
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
	result, err := validate(root, "", false)
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
