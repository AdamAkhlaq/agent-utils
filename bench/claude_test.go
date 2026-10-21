package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeArgs(t *testing.T) {
	task := Task{MaxTurns: 40, MaxBudgetUSD: 1}
	tests := []struct {
		arm  string
		want string
	}{
		{"tool", "--tools Bash,Edit,Glob,Grep,Read,Write --permission-mode bypassPermissions"},
		{"shell", "--tools Bash,Edit,Glob,Grep,Read,Write --permission-mode bypassPermissions"},
		{"model", "--tools Read,Write --permission-mode bypassPermissions"},
	}
	for _, tt := range tests {
		t.Run(tt.arm, func(t *testing.T) {
			args := strings.Join(claudeArgs(tt.arm, "claude-sonnet-5-5", task), " ")
			for _, want := range []string{
				"--model claude-sonnet-5-5",
				`--setting-sources  --disable-slash-commands --strict-mcp-config --no-chrome --no-session-persistence`,
				"--append-system-prompt-file /context.md",
				tt.want,
				"--max-turns 40 --max-budget-usd 1 --output-format stream-json --verbose",
			} {
				if !strings.Contains(args, want) {
					t.Errorf("args %q\nmissing %q", args, want)
				}
			}
		})
	}
	// The empty --setting-sources value must survive as its own argument.
	args := claudeArgs("tool", "m", task)
	for i, a := range args {
		if a == "--setting-sources" && args[i+1] != "" {
			t.Errorf("--setting-sources followed by %q, want an empty argument", args[i+1])
		}
	}
}

func readFixture(t *testing.T, name string) transcript {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr, err := parseTranscript(f)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestParseTranscriptFixtures(t *testing.T) {
	tests := []struct {
		file         string
		arm          string
		tools        string
		turns        int
		toolCalls    int
		bashCalls    int
		firstCommand string
		cost         float64
		cacheRead    int64
		output       int64
		terminal     string
		isError      bool
		leak         string
	}{
		{"model.jsonl", "model", "Read,Write", 2, 1, 0, "", 0.00128675, 4375, 516, "completed", false, ""},
		{"shell.jsonl", "shell", "Task,Bash,...", 2, 1, 1, "cd /tmp && python3 -I -c", 0.00502539, 19839, 1284, "completed", false,
			"tools [Bash CronCreate CronDelete CronList Edit EnterWorktree ExitWorktree ListAgents NotebookEdit Read RemoteTrigger ReportFindings ScheduleWakeup SendMessage Task TaskStop ToolSearch WebFetch WebSearch Workflow Write], want [Bash Edit Glob Grep Read Write]"},
		{"tool.jsonl", "tool", "Bash,Edit,Glob,Grep,Read,Write", 4, 3, 3, "agent-utils cidr -h", 0.0017555, 0, 0, "completed", false, ""},
		{"maxturns.jsonl", "tool", "Bash,Read,Write", 2, 1, 1, "", 0.00097164, 0, 0, "max_turns", true,
			"tools [Bash Read Write], want [Bash Edit Glob Grep Read Write]"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			tr := readFixture(t, tt.file)
			if tr.Init == nil || tr.Result == nil {
				t.Fatalf("init = %v, result = %v", tr.Init != nil, tr.Result != nil)
			}
			if tr.Init.ClaudeCodeVersion != "2.1.304" || tr.Init.Model != "claude-haiku-5-5" {
				t.Errorf("init = %+v", tr.Init)
			}
			if tr.Result.NumTurns != tt.turns {
				t.Errorf("num_turns = %d, want %d", tr.Result.NumTurns, tt.turns)
			}
			if tr.ToolCalls != tt.toolCalls || tr.BashCalls != tt.bashCalls {
				t.Errorf("tool calls = %d, bash calls = %d, want %d and %d", tr.ToolCalls, tr.BashCalls, tt.toolCalls, tt.bashCalls)
			}
			if tt.firstCommand != "" && (len(tr.BashCommands) == 0 || !strings.HasPrefix(tr.BashCommands[0], tt.firstCommand)) {
				t.Errorf("bash commands = %q, want the first to start with %q", tr.BashCommands, tt.firstCommand)
			}
			if d := tr.Result.TotalCostUSD - tt.cost; d > 1e-9 || d < -1e-9 {
				t.Errorf("total_cost_usd = %v, want %v", tr.Result.TotalCostUSD, tt.cost)
			}
			if tt.cacheRead != 0 && tr.Result.Usage.CacheReadInputTokens != tt.cacheRead {
				t.Errorf("cache_read_input_tokens = %d, want %d", tr.Result.Usage.CacheReadInputTokens, tt.cacheRead)
			}
			if tt.output != 0 && tr.Result.Usage.OutputTokens != tt.output {
				t.Errorf("output_tokens = %d, want %d", tr.Result.Usage.OutputTokens, tt.output)
			}
			if tr.Result.TerminalReason != tt.terminal || tr.Result.IsError != tt.isError {
				t.Errorf("terminal_reason = %q, is_error = %v; want %q, %v", tr.Result.TerminalReason, tr.Result.IsError, tt.terminal, tt.isError)
			}
			if tr.Result.DurationMS <= 0 || tr.Result.DurationAPIMS <= 0 {
				t.Errorf("durations = %d, %d; want positive", tr.Result.DurationMS, tr.Result.DurationAPIMS)
			}
			if leak := isolationLeak(tr.Init, tt.arm); leak != tt.leak {
				t.Errorf("isolationLeak = %q, want %q", leak, tt.leak)
			}
		})
	}
}

func TestParseTranscriptRateLimit(t *testing.T) {
	tr := readFixture(t, "shell.jsonl")
	if tr.RateLimit == nil {
		t.Fatal("no rate_limit_event parsed")
	}
	if tr.RateLimit.Status != "allowed" || tr.RateLimit.Windows["five_hour"].Utilization != 0.25 {
		t.Errorf("rate limit = %+v", tr.RateLimit)
	}
}

func TestParseTranscriptErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // substring of the error, or "" for success
	}{
		{"empty", "", ""},
		{"blank lines", "\n\n", ""},
		{"not json", "claude: not logged in\n", "line 1 is not JSON"},
		{"json after prose", `{"type":"system","subtype":"init","tools":[]}` + "\nSegmentation fault\n", "line 2 is not JSON"},
		{"unknown type skipped", `{"type":"future_event","payload":1}` + "\n", ""},
		{"init with wrong shape", `{"type":"system","subtype":"init","tools":"Bash"}` + "\n", "init line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTranscript(strings.NewReader(tt.input))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestIsolationLeak(t *testing.T) {
	clean := func() *initLine {
		return &initLine{Tools: []string{"Write", "Read"}, Plugins: []plugin{{Name: "x", Source: "x@builtin"}}}
	}
	tests := []struct {
		name   string
		mutate func(*initLine)
		want   string
	}{
		{"clean, order-insensitive", func(*initLine) {}, ""},
		{"extra tool", func(i *initLine) { i.Tools = append(i.Tools, "Bash") }, "tools [Bash Read Write], want [Read Write]"},
		{"mcp server", func(i *initLine) { i.MCPServers = []any{map[string]any{"name": "x"}} }, "1 MCP servers"},
		{"slash command", func(i *initLine) { i.SlashCommands = []string{"/deploy"} }, "slash commands [/deploy]"},
		{"skill", func(i *initLine) { i.Skills = []any{"s"} }, "1 skills"},
		{"user plugin", func(i *initLine) { i.Plugins = append(i.Plugins, plugin{Name: "p", Source: "p@marketplace"}) }, "plugin p (p@marketplace)"},
		{"two leaks", func(i *initLine) { i.Skills = []any{"s"}; i.SlashCommands = []string{"/x"} }, "slash commands [/x]; 1 skills"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := clean()
			tt.mutate(init)
			if got := isolationLeak(init, "model"); got != tt.want {
				t.Errorf("isolationLeak = %q, want %q", got, tt.want)
			}
		})
	}
	if got := isolationLeak(nil, "model"); got != "no init line in the transcript" {
		t.Errorf("nil init = %q", got)
	}
}
