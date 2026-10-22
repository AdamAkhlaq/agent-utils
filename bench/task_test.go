package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodTOML = `command = "jwt-decode"
arms = ["tool", "shell", "model"]
tags = ["basic"]
timeout_seconds = 300
max_turns = 40
max_budget_usd = 1.00
`

const goodInstruction = "Write the payload of `/task/fixtures/token.jwt` as JSON to `/out/claims.json`.\n\nDone means the file parses. Do not ask questions; write the file and stop.\n"

var testCommands = map[string]bool{
	"jwt-decode": true, "time": true, "png2jpeg": true, "cidr": true, "case": true,
	"video": true, "commands": true, "version": true,
}

// writeTask lays out a valid task directory and returns it, so each test
// below changes one thing and checks the one problem it causes.
func writeTask(t *testing.T, name string) Task {
	t.Helper()
	dir := filepath.Join(t.TempDir(), filepath.FromSlash(name))
	for _, sub := range []string{"fixtures", "expected"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"task.toml":          goodTOML,
		"instruction.md":     goodInstruction,
		"fixtures/token.jwt": "a.b.c\n",
		"oracle.sh":          "#!/bin/sh\nagent-utils jwt-decode /task/fixtures/token.jwt > /out/claims.json\n",
		"check.sh":           "#!/bin/sh\ntest -s /out/claims.json\n",
	}
	for p, content := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	task, err := loadTask(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestParseTask(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{"valid", goodTOML, ""},
		{"unknown field", goodTOML + "trials = 3\n", "trials"},
		{"wrong type", strings.Replace(goodTOML, "max_turns = 40", `max_turns = "forty"`, 1), "max_turns"},
		{"malformed", "command = \n", "toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := parseTask(strings.NewReader(tt.toml))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if task.Command != "jwt-decode" || len(task.Arms) != 3 || task.MaxBudgetUSD != 1 {
					t.Errorf("parsed %+v", task)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want one mentioning %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTask(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Task)
		want   string // substring of the one expected problem; "" means none
	}{
		{"valid", func(*Task) {}, ""},
		{"case name uppercase", func(k *Task) { k.Name = "jwt-decode/Basic" }, "lowercase"},
		{"command missing", func(k *Task) { k.Command = "" }, "command is required"},
		{"command unknown", func(k *Task) { k.Command = "jwt-verify" }, "not an agent-utils command"},
		{"command mismatches directory", func(k *Task) { k.Command = "time" }, "does not match the directory"},
		{"command excluded", func(k *Task) { k.Name = "video/basic"; k.Command = "video" }, "excluded from the suite"},
		{"arms empty", func(k *Task) { k.Arms = nil }, "arms is required"},
		{"arm unknown", func(k *Task) { k.Arms = []string{"tool", "cli"} }, `unknown arm "cli"`},
		{"arm named listed", func(k *Task) { k.Arms = []string{"tool", "named"} }, `"named" is implied by "tool"`},
		{"arm repeated", func(k *Task) { k.Arms = []string{"tool", "tool"} }, `arm "tool" listed twice`},
		{"arms without tool", func(k *Task) { k.Arms = []string{"shell"} }, `must include "tool"`},
		{"tags empty", func(k *Task) { k.Tags = nil }, "tags is required"},
		{"tag unknown", func(k *Task) { k.Tags = []string{"easy"} }, `unknown tag "easy"`},
		{"tag repeated", func(k *Task) { k.Tags = []string{"basic", "basic"} }, `tag "basic" listed twice`},
		{"timeout zero", func(k *Task) { k.TimeoutSeconds = 0 }, "timeout_seconds must be positive"},
		{"turns negative", func(k *Task) { k.MaxTurns = -1 }, "max_turns must be positive"},
		{"budget zero", func(k *Task) { k.MaxBudgetUSD = 0 }, "max_budget_usd must be positive"},
		{"instruction names a missing fixture", func(k *Task) {
			k.Instruction = strings.Replace(k.Instruction, "token.jwt", "token.txt", 1)
		}, "not in fixtures/"},
		{"model arm with binary output", func(k *Task) {
			k.Instruction = strings.Replace(k.Instruction, "/out/claims.json", "/out/claims.png", 1)
		}, "the model arm cannot write /out/claims.png"},
		{"model arm with binary fixture", func(k *Task) { k.Fixtures = []string{"token.jwt", "photo.png"} }, "cannot read the fixture photo.png"},
		{"binary output without model arm", func(k *Task) {
			k.Arms = []string{"tool", "shell"}
			k.Instruction = strings.Replace(k.Instruction, "/out/claims.json", "/out/claims.png", 1)
		}, ""},
		{"oracle missing", func(k *Task) { os.Remove(filepath.Join(k.Dir, "oracle.sh")) }, "oracle.sh: missing"},
		{"checker not executable", func(k *Task) { os.Chmod(filepath.Join(k.Dir, "check.sh"), 0o644) }, "check.sh: is not executable"},
		{"checker without shebang", func(k *Task) {
			os.WriteFile(filepath.Join(k.Dir, "check.sh"), []byte("test -s /out/claims.json\n"), 0o755)
		}, "check.sh: must start with a #! line"},
		{"checker with CRLF", func(k *Task) {
			os.WriteFile(filepath.Join(k.Dir, "check.sh"), []byte("#!/bin/sh\r\ntest -s /out/claims.json\r\n"), 0o755)
		}, "check.sh: has CRLF line endings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := writeTask(t, "jwt-decode/basic")
			tt.mutate(&task)
			problems := validateTask(task, testCommands)
			if tt.want == "" {
				if len(problems) > 0 {
					t.Fatalf("unexpected problems: %v", problems)
				}
				return
			}
			if len(problems) != 1 || !strings.Contains(problems[0], tt.want) {
				t.Fatalf("problems = %v, want exactly one containing %q", problems, tt.want)
			}
			if !strings.HasPrefix(problems[0], task.Name+": ") {
				t.Errorf("problem %q is not prefixed with the task name", problems[0])
			}
		})
	}
}

func TestLintInstruction(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"clean", goodInstruction, nil},
		{"empty", "  \n", []string{"is empty"}},
		{"no output path", "Decode the token. Do not ask questions; write the file and stop.\n", []string{"must name an output path under /out/"}},
		{"no closing sentence", "Write it to /out/a.json.\n", []string{`must end with "Do not ask questions; write the file and stop."`}},
		{"closing sentence not last", "Write /out/a. Do not ask questions; write the file and stop. Then summarize.\n", []string{`must end with "Do not ask questions; write the file and stop."`}},
		{"plural closing sentence", "Write /out/a and /out/b. Do not ask questions; write the files and stop.\n", nil},
		{"names agent-utils", "Run agent-utils on it and write /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "agent-utils"`}},
		{"names a command in a code span", "Use `jwt-decode` and write /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "jwt-decode"`}},
		{"names a command in another case", "Use JQ and Python to write /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "jq"`, `names a tool: "python"`}},
		{"word containing a command name", "Write the timestamp and the lowercase result to /out/a. Do not ask questions; write the file and stop.\n", nil},
		{"command name as a plain word", "In that case write /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "case"`}},
		{"excluded command as a plain word", "Write the version to /out/a. Do not ask questions; write the file and stop.\n", nil},
		{"convert and magick", "Convert it with magick to /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "convert"`, `names a tool: "magick"`}},
		{"repeated tool reported once", "Use jq, then jq again, for /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "jq"`}},
		{"tool inside a hyphenated word", "A jq-style, python-based filter writes /out/a. Do not ask questions; write the file and stop.\n", []string{`names a tool: "jq"`, `names a tool: "python"`}},
		{"hyphenated word with no tool part", "Write the wall-clock, base64url-encoded value to /out/a. Do not ask questions; write the file and stop.\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lintInstruction(tt.text, testCommands)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("lintInstruction = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskArms(t *testing.T) {
	tests := []struct {
		arms []string
		want string
	}{
		{[]string{"tool", "shell", "model"}, "tool named shell model"},
		{[]string{"tool", "shell"}, "tool named shell"},
		{[]string{"model", "tool"}, "tool named model"},
		{[]string{"shell"}, "shell"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.arms, ","), func(t *testing.T) {
			if got := strings.Join(taskArms(Task{Arms: tt.arms}), " "); got != tt.want {
				t.Errorf("taskArms = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoverage(t *testing.T) {
	task := func(command string) Task { return Task{Name: command + "/basic", Command: command} }
	tests := []struct {
		name     string
		commands map[string]bool
		tasks    []Task
		want     []string
	}{
		{"complete", testCommands, []Task{task("jwt-decode"), task("time"), task("png2jpeg"), task("cidr"), task("case")}, nil},
		{"one gap", testCommands, []Task{task("jwt-decode"), task("time"), task("png2jpeg"), task("cidr")},
			[]string{`command "case" has no task under bench/tasks/case/ and no exclusion`}},
		{"exclusion of a command that no longer exists", map[string]bool{"jwt-decode": true, "video": true, "commands": true},
			[]Task{task("jwt-decode")}, []string{`excluded command "version" does not exist; remove the exclusion`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coverage(tt.commands, tt.tasks)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("coverage = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterTasks(t *testing.T) {
	tasks := []Task{{Name: "cidr/contains"}, {Name: "jwt-decode/basic"}, {Name: "jwt-decode/hard"}}
	tests := []struct {
		glob    string
		want    string
		wantErr bool
	}{
		{"", "cidr/contains jwt-decode/basic jwt-decode/hard", false},
		{"jwt-decode/*", "jwt-decode/basic jwt-decode/hard", false},
		{"*/basic", "jwt-decode/basic", false},
		{"nothing/*", "", false},
		{"[", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			got, err := filterTasks(tasks, tt.glob)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			var names []string
			for _, k := range got {
				names = append(names, k.Name)
			}
			if s := strings.Join(names, " "); s != tt.want {
				t.Errorf("filtered = %q, want %q", s, tt.want)
			}
		})
	}
}

func TestLoadTasks(t *testing.T) {
	root := t.TempDir()
	good := writeTask(t, "jwt-decode/basic")
	if err := os.Rename(filepath.Dir(good.Dir), filepath.Join(root, "jwt-decode")); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "time", "add-dst")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "task.toml"), []byte("command = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tasks, problems := loadTasks(root)
	if len(tasks) != 1 || tasks[0].Name != "jwt-decode/basic" || tasks[0].Instruction != goodInstruction {
		t.Errorf("tasks = %+v", tasks)
	}
	if len(tasks) == 1 && strings.Join(tasks[0].Fixtures, ",") != "token.jwt" {
		t.Errorf("fixtures = %v, want [token.jwt]", tasks[0].Fixtures)
	}
	if len(problems) != 1 || !strings.Contains(problems["time/add-dst"], "line 1: command:") {
		t.Errorf("problems = %v, want one for time/add-dst naming line 1", problems)
	}
}
