package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// maxTranscriptLine bounds one stream-json line. Claude Code's lines carry
// whole tool results, which can be large; 64 MiB is far above anything a
// trial produces and far below what would exhaust memory.
const maxTranscriptLine = 64 << 20

// armTools is the exact tool list each arm runs with. The tool and shell
// arms get every local tool and nothing that reaches the network (the
// default set under bypassPermissions includes WebSearch and WebFetch, which
// would be a second channel out). The model arm can only read and write.
var armTools = map[string][]string{
	"tool":  {"Bash", "Edit", "Glob", "Grep", "Read", "Write"},
	"shell": {"Bash", "Edit", "Glob", "Grep", "Read", "Write"},
	"model": {"Read", "Write"},
}

// claudeArgs is the argument list after "claude -p <instruction>" for one
// trial: the common isolation flags, then the arm's tools and permission
// mode, then the task's budgets.
func claudeArgs(arm, model string, t Task) []string {
	args := []string{
		"--model", model,
		"--setting-sources", "",
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--no-chrome",
		"--no-session-persistence",
		"--append-system-prompt-file", "/context.md",
		"--tools", strings.Join(armTools[arm], ","),
		// bypassPermissions in every arm: what an arm may do is fixed by its
		// tool list. acceptEdits would deny the model arm's Write to /out,
		// which lies outside the working directory (seen in the pilot).
		"--permission-mode", "bypassPermissions",
	}
	return append(args,
		"--max-turns", strconv.Itoa(t.MaxTurns),
		"--max-budget-usd", strconv.FormatFloat(t.MaxBudgetUSD, 'f', -1, 64),
		"--output-format", "stream-json",
		"--verbose",
	)
}

// transcript is what the harness keeps from one trial's stream-json: the
// init line's environment, every tool call, and the result line's totals.
type transcript struct {
	Init   *initLine
	Result *resultLine

	ToolCalls    int
	BashCalls    int
	BashCommands []string
	// RateLimit is the last rate_limit_event seen, if any.
	RateLimit *rateLimitInfo
	// Lines counts every JSON line read, including unknown types.
	Lines int
}

type initLine struct {
	Tools             []string `json:"tools"`
	MCPServers        []any    `json:"mcp_servers"`
	SlashCommands     []string `json:"slash_commands"`
	Skills            []any    `json:"skills"`
	Plugins           []plugin `json:"plugins"`
	Model             string   `json:"model"`
	PermissionMode    string   `json:"permissionMode"`
	ClaudeCodeVersion string   `json:"claude_code_version"`
}

type plugin struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type resultLine struct {
	Subtype           string `json:"subtype"`
	IsError           bool   `json:"is_error"`
	TerminalReason    string `json:"terminal_reason"`
	NumTurns          int    `json:"num_turns"`
	DurationMS        int64  `json:"duration_ms"`
	DurationAPIMS     int64  `json:"duration_api_ms"`
	TotalCostUSD      float64
	Usage             tokenUsage      `json:"usage"`
	PermissionDenials []any           `json:"permission_denials"`
	Result            json.RawMessage `json:"result"`
	APIErrorStatus    *int            `json:"api_error_status"`
}

type tokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
}

type rateLimitInfo struct {
	Status        string `json:"status"`
	RateLimitType string `json:"rateLimitType"`
	ResetsAt      int64  `json:"resetsAt"`
	Windows       map[string]struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    int64   `json:"resetsAt"`
	} `json:"unifiedWindows"`
}

// parseTranscript reads stream-json lines. Lines that are not JSON objects
// are an error (the stream is machine output; anything else means Claude
// Code did not run as expected), but unknown line types are skipped, since
// the stream gains types across versions.
func parseTranscript(r io.Reader) (transcript, error) {
	var tr transcript
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), maxTranscriptLine)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		tr.Lines++
		var head struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return tr, fmt.Errorf("line %d is not JSON: %w", tr.Lines, err)
		}
		switch {
		case head.Type == "system" && head.Subtype == "init":
			var init initLine
			if err := json.Unmarshal(line, &init); err != nil {
				return tr, fmt.Errorf("init line: %w", err)
			}
			tr.Init = &init
		case head.Type == "assistant":
			var msg struct {
				Message struct {
					Content []struct {
						Type  string `json:"type"`
						Name  string `json:"name"`
						Input struct {
							Command string `json:"command"`
						} `json:"input"`
					} `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				return tr, fmt.Errorf("assistant line %d: %w", tr.Lines, err)
			}
			for _, c := range msg.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				tr.ToolCalls++
				if c.Name == "Bash" {
					tr.BashCalls++
					tr.BashCommands = append(tr.BashCommands, c.Input.Command)
				}
			}
		case head.Type == "rate_limit_event":
			var ev struct {
				Info rateLimitInfo `json:"rate_limit_info"`
			}
			if err := json.Unmarshal(line, &ev); err != nil {
				return tr, fmt.Errorf("rate_limit_event line %d: %w", tr.Lines, err)
			}
			tr.RateLimit = &ev.Info
		case head.Type == "result":
			var res resultLine
			if err := json.Unmarshal(line, &res); err != nil {
				return tr, fmt.Errorf("result line: %w", err)
			}
			// total_cost_usd is a float in the stream; decode it apart from
			// the struct so a missing field is zero rather than a type error.
			var cost struct {
				TotalCostUSD float64 `json:"total_cost_usd"`
			}
			if err := json.Unmarshal(line, &cost); err != nil {
				return tr, fmt.Errorf("result line: %w", err)
			}
			res.TotalCostUSD = cost.TotalCostUSD
			tr.Result = &res
		}
	}
	if err := sc.Err(); err != nil {
		return tr, fmt.Errorf("reading transcript: %w", err)
	}
	return tr, nil
}

// isolationLeak reports why the init line shows a trial that was not
// isolated, or "" when it was: only the arm's tools, no MCP server, slash
// command, or skill, and no plugin beyond Claude Code's built-in ones.
func isolationLeak(init *initLine, arm string) string {
	if init == nil {
		return "no init line in the transcript"
	}
	var leaks []string
	want := armTools[arm]
	got := slices.Clone(init.Tools)
	sort.Strings(got)
	if !slices.Equal(got, want) {
		leaks = append(leaks, fmt.Sprintf("tools %v, want %v", got, want))
	}
	if len(init.MCPServers) > 0 {
		leaks = append(leaks, fmt.Sprintf("%d MCP servers", len(init.MCPServers)))
	}
	if len(init.SlashCommands) > 0 {
		leaks = append(leaks, fmt.Sprintf("slash commands %v", init.SlashCommands))
	}
	if len(init.Skills) > 0 {
		leaks = append(leaks, fmt.Sprintf("%d skills", len(init.Skills)))
	}
	for _, p := range init.Plugins {
		if !strings.HasSuffix(p.Source, "@builtin") {
			leaks = append(leaks, fmt.Sprintf("plugin %s (%s)", p.Name, p.Source))
		}
	}
	return strings.Join(leaks, "; ")
}
