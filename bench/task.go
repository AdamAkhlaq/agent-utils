package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Task is one benchmark task directory: bench/tasks/<command>/<case>/.
type Task struct {
	// Name is "<command>/<case>", the task's identity in results.
	Name string `toml:"-"`
	// Dir is the task directory on disk.
	Dir string `toml:"-"`
	// Instruction is the content of instruction.md, the prompt every arm gets.
	Instruction string `toml:"-"`
	// Fixtures lists the files under fixtures/, relative to it, sorted.
	Fixtures []string `toml:"-"`

	Command        string   `toml:"command"`
	Arms           []string `toml:"arms"`
	Tags           []string `toml:"tags"`
	TimeoutSeconds int      `toml:"timeout_seconds"`
	MaxTurns       int      `toml:"max_turns"`
	MaxBudgetUSD   float64  `toml:"max_budget_usd"`
}

// The three experimental conditions. "tool" has agent-utils on PATH, "shell"
// has the same image without it, and "model" can only read and write files.
var arms = []string{"tool", "shell", "model"}

var tags = []string{"basic", "scale", "hard", "sweep"}

// excludedCommands lists the commands that deliberately have no task, with
// the reason the coverage check prints.
var excludedCommands = map[string]string{
	"video":    "reaches the network; the suite runs offline",
	"commands": "meta: prints the command manifest",
	"version":  "meta: prints the binary's version",
}

// forbiddenWords are the tool names an instruction must not contain, beyond
// the command names themselves: naming one would steer the agent and make the
// arms measure the prompt instead of the environment.
var forbiddenWords = []string{"agent-utils", "python", "python3", "jq", "yq", "openssl", "convert", "magick"}

// binaryExtensions marks output or fixture files the "model" arm cannot
// produce or read as text, so a task that uses one must not list that arm.
var binaryExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff", ".webp", ".pdf", ".bin", ".zip", ".gz"}

var (
	caseNamePattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	outPathPattern     = regexp.MustCompile(`/out/[A-Za-z0-9._/-]*[A-Za-z0-9_-]`)
	fixturePathPattern = regexp.MustCompile(`/task/fixtures/[A-Za-z0-9._/-]*[A-Za-z0-9_-]`)
	donePattern        = regexp.MustCompile(`Do not ask questions; write the files? and stop\.\s*$`)
	wordPattern        = regexp.MustCompile(`[A-Za-z0-9_-]+`)
)

// parseTask decodes a task.toml. Unknown keys are an error, so a misspelled
// field cannot silently fall back to a default. Errors name the key, since
// go-toml's own messages name the Go field or nothing at all.
func parseTask(r io.Reader) (Task, error) {
	var t Task
	dec := toml.NewDecoder(r)
	dec.DisallowUnknownFields()
	err := dec.Decode(&t)
	var missing *toml.StrictMissingError
	var decodeErr *toml.DecodeError
	switch {
	case errors.As(err, &missing):
		keys := make([]string, 0, len(missing.Errors))
		for _, e := range missing.Errors {
			keys = append(keys, strings.Join(e.Key(), "."))
		}
		return Task{}, fmt.Errorf("unknown field %s", strings.Join(keys, ", "))
	case errors.As(err, &decodeErr):
		row, _ := decodeErr.Position()
		if key := strings.Join(decodeErr.Key(), "."); key != "" {
			return Task{}, fmt.Errorf("line %d: %s: %s", row, key, decodeErr.Error())
		}
		return Task{}, fmt.Errorf("line %d: %s", row, decodeErr.Error())
	case err != nil:
		return Task{}, err
	}
	return t, nil
}

// loadTask reads one task directory. Problems with the files' content are
// reported by validateTask; this only fails when the directory cannot be read
// as a task at all.
func loadTask(dir, name string) (Task, error) {
	f, err := os.Open(filepath.Join(dir, "task.toml"))
	if err != nil {
		return Task{}, err
	}
	defer f.Close()
	t, err := parseTask(f)
	if err != nil {
		return Task{}, fmt.Errorf("%s: %w", filepath.Join(dir, "task.toml"), err)
	}
	t.Name = name
	t.Dir = dir
	instruction, err := os.ReadFile(filepath.Join(dir, "instruction.md"))
	if err != nil {
		return Task{}, err
	}
	t.Instruction = string(instruction)
	t.Fixtures, err = listFiles(filepath.Join(dir, "fixtures"))
	if err != nil {
		return Task{}, err
	}
	return t, nil
}

// listFiles returns every regular file under root, relative to it with
// forward slashes, sorted. A missing root is an empty list.
func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// loadTasks reads every <command>/<case>/task.toml under tasksDir, sorted by
// name. A directory that cannot be loaded is returned as a problem keyed by
// task name rather than stopping the walk, so validate shows every broken
// task at once and can restrict the report to a glob.
func loadTasks(tasksDir string) ([]Task, map[string]string) {
	var tasks []Task
	problems := map[string]string{}
	commands, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, map[string]string{filepath.Base(tasksDir): err.Error()}
	}
	for _, c := range commands {
		if !c.IsDir() {
			continue
		}
		cases, err := os.ReadDir(filepath.Join(tasksDir, c.Name()))
		if err != nil {
			problems[c.Name()] = err.Error()
			continue
		}
		for _, k := range cases {
			if !k.IsDir() {
				continue
			}
			name := c.Name() + "/" + k.Name()
			t, err := loadTask(filepath.Join(tasksDir, c.Name(), k.Name()), name)
			if err != nil {
				problems[name] = err.Error()
				continue
			}
			tasks = append(tasks, t)
		}
	}
	return tasks, problems
}

// validateTask returns every rule the task breaks, as messages prefixed with
// the task name. commands is the set of command names the binary has.
func validateTask(t Task, commands map[string]bool) []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, t.Name+": "+fmt.Sprintf(format, args...))
	}

	command, caseName, ok := strings.Cut(t.Name, "/")
	if !ok || !caseNamePattern.MatchString(caseName) {
		add("case directory must be lowercase words joined by hyphens")
	}
	if t.Command == "" {
		add("task.toml: command is required")
	} else if !commands[t.Command] {
		add("task.toml: command %q is not an agent-utils command", t.Command)
	} else if t.Command != command {
		add("task.toml: command %q does not match the directory %q", t.Command, command)
	}
	if reason, excluded := excludedCommands[t.Command]; excluded {
		add("task.toml: command %q is excluded from the suite (%s)", t.Command, reason)
	}

	if len(t.Arms) == 0 {
		add("task.toml: arms is required")
	}
	for i, a := range t.Arms {
		if !slices.Contains(arms, a) {
			add("task.toml: unknown arm %q (want one of %s)", a, strings.Join(arms, ", "))
		} else if slices.Index(t.Arms, a) != i {
			add("task.toml: arm %q listed twice", a)
		}
	}
	if len(t.Arms) > 0 && !slices.Contains(t.Arms, "tool") {
		add("task.toml: arms must include \"tool\", the arm the oracle runs in")
	}
	if len(t.Tags) == 0 {
		add("task.toml: tags is required")
	}
	for i, tag := range t.Tags {
		if !slices.Contains(tags, tag) {
			add("task.toml: unknown tag %q (want one of %s)", tag, strings.Join(tags, ", "))
		} else if slices.Index(t.Tags, tag) != i {
			add("task.toml: tag %q listed twice", tag)
		}
	}
	if t.TimeoutSeconds <= 0 {
		add("task.toml: timeout_seconds must be positive")
	}
	if t.MaxTurns <= 0 {
		add("task.toml: max_turns must be positive")
	}
	if t.MaxBudgetUSD <= 0 {
		add("task.toml: max_budget_usd must be positive")
	}

	for _, p := range lintInstruction(t.Instruction, commands) {
		add("instruction.md: %s", p)
	}
	for _, ref := range fixturePathPattern.FindAllString(t.Instruction, -1) {
		rel := strings.TrimPrefix(ref, "/task/fixtures/")
		if !slices.Contains(t.Fixtures, rel) {
			add("instruction.md: names %s, which is not in fixtures/", ref)
		}
	}
	if slices.Contains(t.Arms, "model") {
		for _, p := range outPathPattern.FindAllString(t.Instruction, -1) {
			if isBinary(p) {
				add("task.toml: the model arm cannot write %s; drop \"model\" from arms", p)
			}
		}
		for _, f := range t.Fixtures {
			if isBinary(f) {
				add("task.toml: the model arm cannot read the fixture %s as text; drop \"model\" from arms", f)
			}
		}
	}

	for _, script := range []string{"oracle.sh", "check.sh"} {
		for _, p := range checkScript(filepath.Join(t.Dir, script)) {
			add("%s: %s", script, p)
		}
	}
	return problems
}

// lintInstruction checks the prompt text: it must name an output under /out/,
// end with the fixed closing sentence, and name no tool. Tool names are
// matched as whole words and as the parts of hyphenated words, so
// "timestamp" does not trip on the "time" command, but "jq" in a code span
// and "jq-style" do. Excluded commands ("version", "video") are ordinary
// words that steer nothing, so they are allowed.
func lintInstruction(text string, commands map[string]bool) []string {
	var problems []string
	if strings.TrimSpace(text) == "" {
		return []string{"is empty"}
	}
	if !outPathPattern.MatchString(text) {
		problems = append(problems, "must name an output path under /out/")
	}
	if !donePattern.MatchString(text) {
		problems = append(problems, `must end with "Do not ask questions; write the file and stop."`)
	}
	seen := map[string]bool{}
	for _, word := range wordPattern.FindAllString(text, -1) {
		word = strings.ToLower(word)
		candidates := []string{word}
		if strings.Contains(word, "-") {
			candidates = append(candidates, strings.Split(word, "-")...)
		}
		for _, w := range candidates {
			if seen[w] {
				continue
			}
			if _, excluded := excludedCommands[w]; (commands[w] && !excluded) || slices.Contains(forbiddenWords, w) {
				seen[w] = true
				problems = append(problems, fmt.Sprintf("names a tool: %q", w))
			}
		}
	}
	return problems
}

func isBinary(p string) bool {
	return slices.Contains(binaryExtensions, strings.ToLower(path.Ext(p)))
}

// checkScript requires the oracle and checker to be executable shell scripts,
// since Docker runs them directly.
func checkScript(p string) []string {
	info, err := os.Stat(p)
	if err != nil {
		return []string{"missing"}
	}
	var problems []string
	if info.Mode()&0o111 == 0 {
		problems = append(problems, "is not executable (chmod +x)")
	}
	f, err := os.Open(p)
	if err != nil {
		return append(problems, err.Error())
	}
	defer f.Close()
	line, _ := bufio.NewReader(f).ReadString('\n')
	if !strings.HasPrefix(line, "#!") {
		problems = append(problems, "must start with a #! line")
	}
	// A CRLF script fails inside the container with a misleading "no such
	// file or directory", because the kernel looks for an interpreter named
	// "/bin/sh\r".
	if strings.HasSuffix(line, "\r\n") {
		problems = append(problems, "has CRLF line endings; the interpreter line must end with LF")
	}
	return problems
}

// commandNames reads the command list from docs/commands, which the root
// package's tests keep equal to the registry. This avoids importing or
// running the binary from the harness.
func commandNames(docsDir string) (map[string]bool, error) {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".md"); ok && !e.IsDir() {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: no command pages found", docsDir)
	}
	return names, nil
}

// coverage reports every command without a task and every exclusion that is
// stale: a name that is not a command, or one that has a task after all.
func coverage(commands map[string]bool, tasks []Task) []string {
	covered := map[string]bool{}
	for _, t := range tasks {
		covered[t.Command] = true
	}
	var problems []string
	for _, c := range sortedKeys(commands) {
		if _, excluded := excludedCommands[c]; excluded {
			continue
		}
		if !covered[c] {
			problems = append(problems, fmt.Sprintf("command %q has no task under bench/tasks/%s/ and no exclusion", c, c))
		}
	}
	for _, c := range sortedKeys(excludedCommands) {
		if !commands[c] {
			problems = append(problems, fmt.Sprintf("excluded command %q does not exist; remove the exclusion", c))
		}
	}
	return problems
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// filterTasks keeps the tasks whose name matches the glob (path.Match
// syntax, so "jwt-decode/*" selects one command's tasks). An empty glob keeps
// every task.
func filterTasks(tasks []Task, glob string) ([]Task, error) {
	if glob == "" {
		return tasks, nil
	}
	if _, err := path.Match(glob, ""); err != nil {
		return nil, fmt.Errorf("-tasks %q: %w", glob, err)
	}
	var kept []Task
	for _, t := range tasks {
		if ok, _ := path.Match(glob, t.Name); ok {
			kept = append(kept, t)
		}
	}
	return kept, nil
}
