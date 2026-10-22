package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// manifest is manifest.json: everything that pins a run, written when the
// run starts and completed when it ends.
type manifest struct {
	RunID             string            `json:"run_id"`
	Model             string            `json:"model"`
	ClaudeCodeVersion string            `json:"claude_code_version"`
	AgentUtilsVersion string            `json:"agent_utils_version"`
	HarnessCommit     string            `json:"harness_commit"`
	Images            map[string]string `json:"images"`
	Arms              []string          `json:"arms"`
	Trials            int               `json:"trials"`
	Concurrency       int               `json:"concurrency"`
	Seed              uint64            `json:"seed"`
	Tasks             map[string]string `json:"tasks"`   // name -> SHA-256 of instruction.md
	Context           map[string]string `json:"context"` // arm context file -> SHA-256
	ContextTokens     map[string]int    `json:"context_tokens_estimate"`
	NamedSentence     string            `json:"named_sentence"`
	StartedAt         string            `json:"started_at"`
	FinishedAt        string            `json:"finished_at,omitempty"`
	InfraRetries      int               `json:"infra_retries"`
	ProxyRefusals     int               `json:"proxy_refusals"`
}

func readManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

func writeManifest(path string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// cell is one (task, arm, trial) to run.
type cell struct {
	Task  Task
	Arm   string
	Trial int
}

func (c cell) key() string { return fmt.Sprintf("%s|%s|%d", c.Task.Name, c.Arm, c.Trial) }

// plan lists every cell in execution order: tasks in name order, trials in
// order, and within each trial the task's arms in an order shuffled from the
// seed, so the arms of one trial run back to back and no arm is always
// first. Cells whose key is in done are left out (resume).
func plan(tasks []Task, selectedArms []string, trials int, seed uint64, done map[string]bool) []cell {
	rng := rand.New(rand.NewPCG(seed, seed^0xda3e39cb94b95bdb))
	var cells []cell
	for _, t := range tasks {
		var applicable []string
		for _, a := range taskArms(t) {
			if slices.Contains(selectedArms, a) {
				applicable = append(applicable, a)
			}
		}
		for n := 1; n <= trials; n++ {
			order := slices.Clone(applicable)
			rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
			for _, a := range order {
				c := cell{Task: t, Arm: a, Trial: n}
				if !done[c.key()] {
					cells = append(cells, c)
				}
			}
		}
	}
	return cells
}

// runConfig is everything "bench run" resolved before the first trial.
type runConfig struct {
	Root        string
	RunID       string
	RunDir      string
	Model       string
	Arms        []string
	Trials      int
	Concurrency int
	Version     string // agent-utils release in the tool image
	Tasks       []Task
	Seed        uint64
	Stderr      io.Writer
}

const (
	maxInfraRetries = 3
	trialGrace      = 30 * time.Second // beyond task.toml's timeout, for container start and teardown
)

// infraPatterns in Claude Code's stderr or result mark a failure of the
// infrastructure rather than of the task: retried, never scored.
var infraPatterns = []string{
	"rate limit", "rate_limit", "429", "overloaded", "529", "500", "502", "503", "504",
	"ECONNRESET", "ETIMEDOUT", "ENOTFOUND", "socket hang up", "fetch failed", "Connection error",
	"authentication", "Not logged in", "OAuth token",
}

// execute runs the plan: a worker pool pulls cells in planned order, so the
// interleaving holds up to the concurrency, and every finished trial is
// appended to trials.jsonl before the next starts. It returns the number of
// infrastructure retries.
func execute(cfg runConfig, cells []cell, net network, m *manifest) (int, error) {
	trialsPath := filepath.Join(cfg.RunDir, "trials.jsonl")
	var mu sync.Mutex
	retries := 0
	var firstErr error
	queue := make(chan cell)
	var wg sync.WaitGroup
	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range queue {
				t, n, err := runCell(cfg, c, net)
				mu.Lock()
				retries += n
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					fmt.Fprintf(cfg.Stderr, "ERROR %s %s #%d: %v\n", c.Task.Name, c.Arm, c.Trial, err)
				} else {
					if m.ClaudeCodeVersion == "" && t.claudeVersion != "" {
						m.ClaudeCodeVersion = t.claudeVersion
					}
					if err := appendTrial(trialsPath, t.Trial); err != nil && firstErr == nil {
						firstErr = err
					}
					verdict := "pass"
					if !t.Passed {
						verdict = "FAIL (" + t.TerminalReason + ")"
					}
					fmt.Fprintf(cfg.Stderr, "%-5s %s %s #%d  $%.4f  %d turns  %.0fs\n", verdict, c.Task.Name, c.Arm, c.Trial,
						t.TotalCostUSD, t.NumTurns, float64(t.DurationMS)/1000)
				}
				mu.Unlock()
			}
		}()
	}
	for _, c := range cells {
		mu.Lock()
		stop := firstErr != nil
		mu.Unlock()
		if stop {
			break
		}
		queue <- c
	}
	close(queue)
	wg.Wait()
	return retries, firstErr
}

// trialResult is a Trial plus what the manifest needs from it.
type trialResult struct {
	Trial
	claudeVersion string
}

// runCell runs one trial with infrastructure retries. Every attempt's
// transcript is kept (attempt suffixes), and the trial record points at the
// one that counted.
func runCell(cfg runConfig, c cell, net network) (trialResult, int, error) {
	var last trialResult
	for attempt := 0; ; attempt++ {
		t, infra, err := runOnce(cfg, c, net, attempt)
		if err != nil {
			return t, attempt, err
		}
		t.InfraRetries = attempt
		if !infra {
			return t, attempt, nil
		}
		last = t
		if attempt == maxInfraRetries {
			last.TerminalReason = "infra"
			last.CheckReason = "infrastructure failure after " + fmt.Sprint(maxInfraRetries) + " retries: " + last.CheckReason
			return last, attempt, nil
		}
		wait := time.Duration(1<<attempt) * time.Minute
		fmt.Fprintf(cfg.Stderr, "retry %s %s #%d in %s: %s\n", c.Task.Name, c.Arm, c.Trial, wait, firstLine(last.CheckReason))
		time.Sleep(wait)
	}
}

// runOnce is one attempt: container, transcript, isolation check, copy of
// /out, checker. It reports whether the attempt failed for infrastructure
// reasons (to be retried) and never scores such an attempt as a task result.
func runOnce(cfg runConfig, c cell, net network, attempt int) (trialResult, bool, error) {
	suffix := ""
	if attempt > 0 {
		suffix = fmt.Sprintf(".retry%d", attempt)
	}
	rel := filepath.Join("transcripts", filepath.FromSlash(c.Task.Name), c.Arm, fmt.Sprintf("%d%s.jsonl", c.Trial, suffix))
	transcriptPath := filepath.Join(cfg.RunDir, rel)
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o755); err != nil {
		return trialResult{}, false, err
	}
	stdout, err := os.Create(transcriptPath)
	if err != nil {
		return trialResult{}, false, err
	}
	stderr, err := os.Create(strings.TrimSuffix(transcriptPath, ".jsonl") + ".stderr")
	if err != nil {
		stdout.Close()
		return trialResult{}, false, err
	}

	image := baseImage
	if c.Arm == "tool" || c.Arm == "named" {
		image = toolImage
	}
	contextFile := filepath.Join(cfg.Root, "bench", "context", "neutral.md")
	switch c.Arm {
	case "tool":
		contextFile = filepath.Join(cfg.Root, "bench", "context", "tool.md")
	case "named":
		// Generated per task into the run directory, so the exact text every
		// named trial saw is kept with the run.
		toolText, err := os.ReadFile(filepath.Join(cfg.Root, "bench", "context", "tool.md"))
		if err != nil {
			return trialResult{}, false, err
		}
		contextFile = filepath.Join(cfg.RunDir, "context", dockerSafe(c.Task.Name)+"-named.md")
		if err := os.MkdirAll(filepath.Dir(contextFile), 0o755); err != nil {
			return trialResult{}, false, err
		}
		if err := os.WriteFile(contextFile, []byte(namedContext(string(toolText), c.Task.Command)), 0o644); err != nil {
			return trialResult{}, false, err
		}
	}
	name := fmt.Sprintf("agent-utils-bench-%s-%s-%s-%d", dockerSafe(cfg.RunID), dockerSafe(c.Task.Name), c.Arm, c.Trial)
	volume := name + "-out"
	t := trialResult{Trial: Trial{
		RunID: cfg.RunID, Task: c.Task.Name, Arm: c.Arm, Trial: c.Trial, Model: cfg.Model,
		StartedAt: time.Now().UTC().Format(time.RFC3339), Transcript: filepath.ToSlash(rel),
		BashCommands: []string{}, RateLimitUtilization: -1,
	}}

	if err := createVolume(volume); err != nil {
		stdout.Close()
		stderr.Close()
		return t, false, err
	}
	defer func() { _ = removeVolume(volume) }()

	exitCode, runErr := runTrial(trialSpec{
		Name: name, Arm: c.Arm, Image: image, Task: c.Task,
		ContextFile: contextFile,
		Volume:      volume, Network: net.Name, ProxyURL: net.ProxyURL(),
		Args:    claudeArgs(c.Arm, cfg.Model, c.Task),
		Timeout: time.Duration(c.Task.TimeoutSeconds)*time.Second + trialGrace,
		Stdout:  stdout, Stderr: stderr,
	})
	stdout.Close()
	stderr.Close()
	t.ExitCode = exitCode

	transcriptFile, err := os.Open(transcriptPath)
	if err != nil {
		return t, false, err
	}
	tr, parseErr := parseTranscript(transcriptFile)
	transcriptFile.Close()
	stderrText, _ := os.ReadFile(strings.TrimSuffix(transcriptPath, ".jsonl") + ".stderr")

	if tr.Init != nil {
		t.claudeVersion = tr.Init.ClaudeCodeVersion
	}
	if tr.RateLimit != nil {
		t.RateLimitUtilization = tr.RateLimit.Windows["five_hour"].Utilization
	}
	t.ToolCalls, t.BashCalls, t.BashCommands = tr.ToolCalls, tr.BashCalls, tr.BashCommands
	if t.BashCommands == nil {
		t.BashCommands = []string{}
	}
	t.UsedTool = usedTool(t.BashCommands, c.Task.Command)
	if tr.Result != nil {
		r := tr.Result
		t.InputTokens, t.CacheCreationInputTokens = r.Usage.InputTokens, r.Usage.CacheCreationInputTokens
		t.CacheReadInputTokens, t.OutputTokens = r.Usage.CacheReadInputTokens, r.Usage.OutputTokens
		t.TotalCostUSD, t.NumTurns = r.TotalCostUSD, r.NumTurns
		t.DurationMS, t.DurationAPIMS = r.DurationMS, r.DurationAPIMS
		t.TerminalReason = r.TerminalReason
	}

	switch {
	case errors.Is(runErr, errTrialTimeout):
		t.TerminalReason = "timeout"
		t.CheckReason = fmt.Sprintf("container killed after %d s", c.Task.TimeoutSeconds)
		return t, false, nil
	case runErr != nil:
		return t, false, runErr
	}
	if leak := isolationLeak(tr.Init, c.Arm); tr.Init != nil && leak != "" {
		t.TerminalReason = "isolation_leak"
		t.CheckReason = leak
		return t, false, nil
	}
	if tr.Result == nil || isInfraFailure(tr.Result, string(stderrText), parseErr) {
		reason := strings.TrimSpace(string(stderrText))
		if parseErr != nil {
			reason = parseErr.Error() + "\n" + reason
		}
		if tr.Result != nil {
			reason = string(tr.Result.Result) + "\n" + reason
		}
		if tr.Result == nil {
			t.TerminalReason = "no_result"
		}
		t.CheckReason = strings.TrimSpace(reason)
		return t, true, nil
	}

	outDir := filepath.Join(cfg.RunDir, "out", filepath.FromSlash(c.Task.Name), c.Arm, fmt.Sprint(c.Trial))
	if err := copyOut(volume, outDir); err != nil {
		return t, false, err
	}
	checkOut, checkErr := runCheck(c.Task, volume)
	if checkErr == nil {
		t.Passed = true
	} else {
		t.CheckReason = checkOut
		if checkOut == "" {
			t.CheckReason = checkErr.Error()
		}
	}
	return t, false, nil
}

// isInfraFailure decides whether an attempt failed because of the API, the
// network, or the credential rather than the task. A result line with
// is_error and an api_error_status, or an error result whose text matches
// the patterns, is infrastructure; so is a missing result line whose stderr
// matches. A max-turns or budget stop is a task failure, not infrastructure.
func isInfraFailure(r *resultLine, stderr string, parseErr error) bool {
	if r == nil {
		return true
	}
	if r.APIErrorStatus != nil {
		return true
	}
	if !r.IsError {
		return false
	}
	if r.TerminalReason == "max_turns" || r.TerminalReason == "max_budget_usd" || strings.HasPrefix(r.Subtype, "error_max") {
		return false
	}
	text := string(r.Result) + "\n" + stderr
	lower := strings.ToLower(text)
	for _, p := range infraPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return parseErr != nil
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func gitHead(root string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	head := strings.TrimSpace(string(out))
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = root
	if out, err := status.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		head += "-dirty"
	}
	return head
}

// buildManifest pins the run before the first trial.
func buildManifest(cfg runConfig) (manifest, error) {
	m := manifest{
		RunID: cfg.RunID, Model: cfg.Model, AgentUtilsVersion: cfg.Version, HarnessCommit: gitHead(cfg.Root),
		Images: map[string]string{}, Arms: cfg.Arms, Trials: cfg.Trials, Concurrency: cfg.Concurrency, Seed: cfg.Seed,
		Tasks: map[string]string{}, Context: map[string]string{}, ContextTokens: map[string]int{},
		StartedAt: time.Now().UTC().Format(time.RFC3339), NamedSentence: namedSentence,
	}
	for _, tag := range []string{baseImage, toolImage, proxyImage} {
		d, err := imageDigest(tag)
		if err != nil {
			return m, err
		}
		m.Images[strings.TrimPrefix(tag, "agent-utils-bench:")] = d
	}
	for _, t := range cfg.Tasks {
		h, err := sha256File(filepath.Join(t.Dir, "instruction.md"))
		if err != nil {
			return m, err
		}
		m.Tasks[t.Name] = h
	}
	for _, name := range []string{"tool.md", "neutral.md"} {
		p := filepath.Join(cfg.Root, "bench", "context", name)
		h, err := sha256File(p)
		if err != nil {
			return m, err
		}
		m.Context[name] = h
		text, err := os.ReadFile(p)
		if err != nil {
			return m, err
		}
		m.ContextTokens[name] = estimateTokens(string(text))
	}
	return m, nil
}

// countRefusals counts the proxy's refused connections in its log: evidence
// for the manifest that nothing but the API was reached (or that something
// tried).
func countRefusals(logPath string) int {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "refused ")
}

// doneKeys reads the finished cells of a run for resume.
func doneKeys(trials []Trial) map[string]bool {
	done := map[string]bool{}
	for _, t := range trials {
		done[t.Key()] = true
	}
	return done
}

func sortedArms(selected []string) []string {
	out := slices.Clone(selected)
	sort.Slice(out, func(i, j int) bool { return slices.Index(arms, out[i]) < slices.Index(arms, out[j]) })
	return out
}
