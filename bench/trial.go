package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// Trial is one line of trials.jsonl: everything the report needs from one
// run of one task in one arm. Field names are the published contract.
type Trial struct {
	RunID string `json:"run_id"`
	Task  string `json:"task"`
	Arm   string `json:"arm"`
	Trial int    `json:"trial"`
	Model string `json:"model"`

	Passed      bool   `json:"passed"`
	CheckReason string `json:"check_reason"`
	// TerminalReason is Claude Code's terminal_reason ("completed",
	// "max_turns", "max_budget_usd", ...), or one of the harness's own:
	// "timeout" (the container was killed), "isolation_leak", "no_result"
	// (Claude Code exited without a result line), "infra" (retries exhausted).
	TerminalReason string `json:"terminal_reason"`
	ExitCode       int    `json:"exit_code"`

	InputTokens              int64   `json:"input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	TotalCostUSD             float64 `json:"total_cost_usd"`

	NumTurns     int      `json:"num_turns"`
	ToolCalls    int      `json:"tool_calls"`
	BashCalls    int      `json:"bash_calls"`
	BashCommands []string `json:"bash_commands"`
	// UsedTool is whether a shell command invoked agent-utils with the
	// task's command (always false in the shell and model arms).
	UsedTool bool `json:"used_tool"`

	DurationMS    int64  `json:"duration_ms"`
	DurationAPIMS int64  `json:"duration_api_ms"`
	StartedAt     string `json:"started_at"`
	Transcript    string `json:"transcript"`
	InfraRetries  int    `json:"infra_retries"`
	// RateLimitUtilization is the five-hour window's utilization reported
	// during the trial (0 to 1), or -1 when no rate_limit_event was seen.
	RateLimitUtilization float64 `json:"rate_limit_utilization"`
}

// TotalTokens is every token the trial consumed, across the four buckets.
func (t Trial) TotalTokens() int64 {
	return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens + t.OutputTokens
}

// Key identifies a cell's trial for resume: the same triple is never run
// twice within a run.
func (t Trial) Key() string {
	return fmt.Sprintf("%s|%s|%d", t.Task, t.Arm, t.Trial)
}

// readTrials loads trials.jsonl. A missing file is an empty run. A malformed
// line is an error naming it, so a half-written line from a crash is caught
// rather than silently dropped.
func readTrials(path string) ([]Trial, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeTrials(f)
}

func decodeTrials(r io.Reader) ([]Trial, error) {
	var trials []Trial
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), maxTranscriptLine)
	n := 0
	for sc.Scan() {
		n++
		if len(sc.Bytes()) == 0 {
			continue
		}
		var t Trial
		if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
			return nil, fmt.Errorf("trials.jsonl line %d: %w", n, err)
		}
		if t.BashCommands == nil {
			t.BashCommands = []string{}
		}
		trials = append(trials, t)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("trials.jsonl: %w", err)
	}
	return trials, nil
}

// appendTrial writes one line and syncs it, so an interrupted run loses at
// most the trial in flight.
func appendTrial(path string, t Trial) error {
	if t.BashCommands == nil {
		t.BashCommands = []string{}
	}
	line, err := json.Marshal(t)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
