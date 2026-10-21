package main

import (
	"strings"
	"testing"
)

func testTasks() []Task {
	return []Task{
		{Name: "cidr/contains", Arms: []string{"tool", "shell", "model"}},
		{Name: "png2jpeg/basic", Arms: []string{"tool", "shell"}},
	}
}

func TestPlan(t *testing.T) {
	cells := plan(testTasks(), arms, 2, seedFrom("run-1"), nil)
	if len(cells) != 10 {
		t.Fatalf("got %d cells, want 10 (3+2 arms x 2 trials)", len(cells))
	}
	// Tasks in order, trials in order, every arm of a trial adjacent.
	var order []string
	for _, c := range cells {
		order = append(order, c.Task.Name+"#"+string(rune('0'+c.Trial)))
	}
	want := "cidr/contains#1 cidr/contains#1 cidr/contains#1 cidr/contains#2 cidr/contains#2 cidr/contains#2 png2jpeg/basic#1 png2jpeg/basic#1 png2jpeg/basic#2 png2jpeg/basic#2"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %s\nwant %s", got, want)
	}
	seen := map[string]int{}
	for _, c := range cells {
		seen[c.Arm]++
		if c.Task.Name == "png2jpeg/basic" && c.Arm == "model" {
			t.Errorf("model arm planned for a task that excludes it")
		}
	}
	if seen["tool"] != 4 || seen["shell"] != 4 || seen["model"] != 2 {
		t.Errorf("arm counts = %v", seen)
	}
	// The same seed gives the same order; a different seed shuffles arms.
	again := plan(testTasks(), arms, 2, seedFrom("run-1"), nil)
	for i := range cells {
		if cells[i].key() != again[i].key() {
			t.Fatalf("plan is not deterministic at cell %d: %s vs %s", i, cells[i].key(), again[i].key())
		}
	}
	first := func(cells []cell) string {
		var s []string
		for _, c := range cells {
			s = append(s, c.Arm)
		}
		return strings.Join(s, ",")
	}
	if first(plan(testTasks(), arms, 6, seedFrom("a"), nil)) == first(plan(testTasks(), arms, 6, seedFrom("b"), nil)) {
		t.Errorf("two seeds gave the same arm order over 6 trials")
	}
}

func TestPlanResumeAndArmSelection(t *testing.T) {
	done := map[string]bool{"cidr/contains|tool|1": true, "cidr/contains|shell|1": true, "png2jpeg/basic|tool|2": true}
	cells := plan(testTasks(), []string{"tool", "shell"}, 2, seedFrom("x"), done)
	var keys []string
	for _, c := range cells {
		keys = append(keys, c.key())
	}
	got := strings.Join(keys, " ")
	for _, k := range []string{"cidr/contains|tool|1", "cidr/contains|shell|1", "png2jpeg/basic|tool|2"} {
		if strings.Contains(got, k) {
			t.Errorf("done cell %s was planned again", k)
		}
	}
	if strings.Contains(got, "model") {
		t.Errorf("unselected arm planned: %s", got)
	}
	if len(cells) != 5 {
		t.Errorf("got %d cells, want 5: %s", len(cells), got)
	}
}

func TestIsInfraFailure(t *testing.T) {
	status := 529
	tests := []struct {
		name   string
		result *resultLine
		stderr string
		parse  error
		want   bool
	}{
		{"no result line", nil, "", nil, true},
		{"success", &resultLine{Subtype: "success", TerminalReason: "completed"}, "", nil, false},
		{"max turns", &resultLine{Subtype: "error_max_turns", IsError: true, TerminalReason: "max_turns"}, "", nil, false},
		{"budget", &resultLine{Subtype: "error_max_budget_usd", IsError: true, TerminalReason: "max_budget_usd"}, "", nil, false},
		{"api error status", &resultLine{IsError: true, APIErrorStatus: &status}, "", nil, true},
		{"rate limit in result", &resultLine{IsError: true, Result: []byte(`"Rate limit reached"`)}, "", nil, true},
		{"auth failure in stderr", &resultLine{IsError: true}, "Not logged in", nil, true},
		{"other error", &resultLine{IsError: true, Result: []byte(`"something unexpected"`)}, "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInfraFailure(tt.result, tt.stderr, tt.parse); got != tt.want {
				t.Errorf("isInfraFailure = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckResume(t *testing.T) {
	cfg := runConfig{RunID: "r", Model: "claude-sonnet-5-5", Version: "v2.1.0"}
	tests := []struct {
		name string
		m    manifest
		want string
	}{
		{"same", manifest{Model: "claude-sonnet-5-5", AgentUtilsVersion: "v2.1.0"}, ""},
		{"other model", manifest{Model: "claude-haiku-5-5", AgentUtilsVersion: "v2.1.0"}, "model claude-haiku-5-5, not claude-sonnet-5-5"},
		{"other release", manifest{Model: "claude-sonnet-5-5", AgentUtilsVersion: "v2.0.0"}, "agent-utils v2.0.0, not v2.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkResume(tt.m, cfg)
			if tt.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("err = %v, want one containing %q", err, tt.want)
			}
		})
	}
}
