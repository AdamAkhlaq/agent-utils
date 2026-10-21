package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticTrials builds a small run: two tasks, three arms, two trials,
// with the tool arm cheaper and the model arm failing one task.
func syntheticTrials() []Trial {
	var ts []Trial
	add := func(task, arm string, n int, passed bool, cost float64, tokens int64, turns int, ms int64, reason string) {
		ts = append(ts, Trial{RunID: "r1", Task: task, Arm: arm, Trial: n, Model: "claude-sonnet-5-5", Passed: passed,
			CheckReason: reason, TerminalReason: "completed", TotalCostUSD: cost, CacheReadInputTokens: tokens * 3 / 4,
			InputTokens: tokens / 8, CacheCreationInputTokens: tokens / 8, OutputTokens: 0, NumTurns: turns, DurationMS: ms})
	}
	for n := 1; n <= 2; n++ {
		add("cidr/contains", "named", n, true, 0.008, 7000, 2, 8000, "")
		ts[len(ts)-1].UsedTool = true
		add("cidr/contains", "tool", n, true, 0.01, 8000, 3, 10000, "")
		add("cidr/contains", "shell", n, true, 0.02, 16000, 4, 20000, "")
		add("cidr/contains", "model", n, n == 1, 0.005, 4000, 2, 5000, map[bool]string{true: "", false: "want true, got \"false\""}[n == 1])
		add("yaml2json/scale-30k", "tool", n, true, 0.03, 24000, 3, 15000, "")
		add("yaml2json/scale-30k", "shell", n, true, 0.03, 24000, 5, 30000, "")
		add("yaml2json/scale-30k", "model", n, false, 0.20, 80000, 2, 60000, "documents differ at $.inventory.services[3].replicas")
	}
	return ts
}

func TestSummarizeExcludesHarnessFailures(t *testing.T) {
	trials := syntheticTrials()
	trials = append(trials,
		Trial{Task: "cidr/contains", Arm: "tool", Trial: 3, TerminalReason: "infra", CheckReason: "infrastructure failure after 3 retries: 529 overloaded", TotalCostUSD: 5},
		Trial{Task: "cidr/contains", Arm: "shell", Trial: 3, TerminalReason: "isolation_leak", CheckReason: "1 MCP servers"})
	s := summarize("r1", trials, nil)
	if len(s.Excluded) != 2 || s.Excluded[0].Treason != "infra" || s.Excluded[1].Arm != "shell" {
		t.Fatalf("excluded = %+v", s.Excluded)
	}
	for _, a := range s.Arms {
		if a.Arm == "tool" && (a.Trials != 4 || a.PassRate != 1 || math.Abs(*a.CostPerSolved-0.02) > 1e-9) {
			t.Errorf("tool arm counted the infra trial: %+v", a)
		}
	}
	var md bytes.Buffer
	writeMarkdown(&md, s)
	if !strings.Contains(md.String(), "- cidr/contains `tool` trial 3 (infra): infrastructure failure after 3 retries: 529 overloaded") {
		t.Errorf("excluded section missing:\n%s", md.String())
	}
	if !strings.Contains(md.String(), "16 trials over 2 tasks (2 excluded, see the last section)") {
		t.Errorf("header lacks the exclusion count:\n%s", md.String())
	}
}

func TestSummarize(t *testing.T) {
	s := summarize("r1", syntheticTrials(), &manifest{Model: "claude-sonnet-5-5", ClaudeCodeVersion: "2.1.304", AgentUtilsVersion: "v2.1.0"})
	if s.Trials != 14 || s.Tasks != 2 || len(s.Arms) != 4 || len(s.Cells) != 7 {
		t.Fatalf("summary shape: trials %d tasks %d arms %d cells %d", s.Trials, s.Tasks, len(s.Arms), len(s.Cells))
	}
	byArm := map[string]armSummary{}
	for _, a := range s.Arms {
		byArm[a.Arm] = a
	}
	if r := byArm["tool"].PassRate; r != 1 {
		t.Errorf("tool pass rate = %v, want 1", r)
	}
	if r := byArm["model"].PassRate; r != 0.25 {
		t.Errorf("model pass rate = %v, want 0.25", r)
	}
	if n := byArm["model"].PassAllTasks; n != 0 {
		t.Errorf("model pass-all tasks = %d, want 0", n)
	}
	if n := byArm["tool"].PassAllTasks; n != 2 {
		t.Errorf("tool pass-all tasks = %d, want 2", n)
	}
	// Cost per solved task counts every trial's spend, including failures.
	if c := byArm["model"].CostPerSolved; c == nil || math.Abs(*c-0.41) > 1e-9 {
		t.Errorf("model cost per solved = %v, want 0.41 (all spend / 1 pass)", c)
	}
	if c := byArm["tool"].CostPerSolved; c == nil || math.Abs(*c-0.02) > 1e-9 {
		t.Errorf("tool cost per solved = %v, want 0.02", c)
	}
	if share := byArm["tool"].TokenShare["cache_read"]; math.Abs(share-0.75) > 1e-9 {
		t.Errorf("cache read share = %v, want 0.75", share)
	}
	if len(byArm["model"].Failed) != 3 || byArm["model"].Failed[0].Task != "cidr/contains" {
		t.Errorf("model failed list = %+v", byArm["model"].Failed)
	}

	if len(s.Ratios) != 4 {
		t.Fatalf("ratios = %+v", s.Ratios)
	}
	named := s.Ratios[0]
	if named.Numerator != "named" || named.Denominator != "shell" || named.Tasks != 1 || math.Abs(named.Median["cost_usd"]-0.4) > 1e-9 {
		t.Errorf("named/shell ratio = %+v", named)
	}
	shell := s.Ratios[2]
	if shell.Numerator != "tool" || shell.Denominator != "shell" || shell.Tasks != 2 {
		t.Errorf("third ratio = %+v", shell)
	}
	if byArm["named"].ToolUseRate != 1 || byArm["tool"].ToolUseRate != 0 {
		t.Errorf("tool use rates: named %v, tool %v", byArm["named"].ToolUseRate, byArm["tool"].ToolUseRate)
	}
	// Per-task cost ratios are 0.5 and 1.0; the median is 0.75.
	if math.Abs(shell.Median["cost_usd"]-0.75) > 1e-9 {
		t.Errorf("tool/shell cost ratio = %v, want 0.75", shell.Median["cost_usd"])
	}
	if math.Abs(shell.Median["turns"]-0.675) > 1e-9 {
		t.Errorf("tool/shell turns ratio = %v, want 0.675", shell.Median["turns"])
	}
	ci := shell.CI["cost_usd"]
	if ci.Low < 0.5 || ci.High > 1.0 || ci.Low > ci.High {
		t.Errorf("tool/shell cost CI = %+v, want within [0.5, 1.0]", ci)
	}
}

func TestReportIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, tr := range syntheticTrials() {
		if err := appendTrial(filepath.Join(dir, "trials.jsonl"), tr); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeManifest(filepath.Join(dir, "manifest.json"), manifest{RunID: "r1", Model: "claude-sonnet-5-5", ClaudeCodeVersion: "2.1.304", AgentUtilsVersion: "v2.1.0"}); err != nil {
		t.Fatal(err)
	}
	read := func() (string, string) {
		if _, err := reportRun(dir); err != nil {
			t.Fatal(err)
		}
		md, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
		js, _ := os.ReadFile(filepath.Join(dir, "summary.json"))
		return string(md), string(js)
	}
	md1, js1 := read()
	md2, js2 := read()
	if md1 != md2 || js1 != js2 {
		t.Fatal("two reports of the same trials.jsonl differ")
	}
	for _, want := range []string{
		"# Benchmark run r1",
		"Model `claude-sonnet-5-5`, Claude Code 2.1.304, agent-utils v2.1.0. 14 trials over 2 tasks.",
		"| `tool` | 4 | 2 | 100% | 2 of 2 | 0% | $0.0200 |",
		"| `named` | 2 | 1 | 100% | 1 of 1 | 100% | $0.0080 |",
		"| `model` | 4 | 2 | 25% | 0 of 2 | 0% | $0.4100 |",
		"| `named` / `shell` | 1 | 0.40 [",
		"| `tool` / `shell` | 2 | 0.75 [",
		"## Input-size scaling (median tokens)",
		"| yaml2json/scale-30k | 24000 | n/a | 24000 | 80000 |",
		"- yaml2json/scale-30k trial 1 (completed): documents differ at $.inventory.services[3].replicas",
	} {
		if !strings.Contains(md1, want) {
			t.Errorf("summary.md lacks %q\n%s", want, md1)
		}
	}
	var s summary
	if err := json.Unmarshal([]byte(js1), &s); err != nil {
		t.Fatal(err)
	}
	if s.RunID != "r1" || len(s.Cells) != 7 {
		t.Errorf("summary.json = %+v", s)
	}
}

func TestWriteSweep(t *testing.T) {
	a := summarize("haiku", syntheticTrials(), &manifest{Model: "claude-haiku-5-5"})
	b := summarize("sonnet", syntheticTrials(), &manifest{Model: "claude-sonnet-5-5"})
	var buf bytes.Buffer
	writeSweep(&buf, []summary{a, b})
	out := buf.String()
	if strings.Count(out, "| haiku |") != 4 || strings.Count(out, "| sonnet |") != 4 {
		t.Errorf("sweep rows:\n%s", out)
	}
}

func TestReadTrialsErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trials.jsonl")
	if ts, err := readTrials(path); err != nil || ts != nil {
		t.Errorf("missing file: %v, %v; want nil, nil", ts, err)
	}
	if err := os.WriteFile(path, []byte(`{"run_id":"r"}`+"\n"+`{"run_id":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTrials(path); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("truncated line: err = %v, want one naming line 2", err)
	}
}
