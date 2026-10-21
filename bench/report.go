package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// metrics are the per-trial measures the report aggregates, in the order
// the tables show them.
var metrics = []struct {
	Key   string
	Label string
	Of    func(Trial) float64
}{
	{"cost_usd", "cost (USD)", func(t Trial) float64 { return t.TotalCostUSD }},
	{"tokens", "tokens", func(t Trial) float64 { return float64(t.TotalTokens()) }},
	{"turns", "turns", func(t Trial) float64 { return float64(t.NumTurns) }},
	{"duration_s", "duration (s)", func(t Trial) float64 { return float64(t.DurationMS) / 1000 }},
}

// summary is summary.json: every table of the report as data, so the
// Markdown is a rendering and nothing in it is unrecoverable.
type summary struct {
	RunID             string `json:"run_id"`
	Model             string `json:"model"`
	ClaudeCodeVersion string `json:"claude_code_version,omitempty"`
	AgentUtilsVersion string `json:"agent_utils_version,omitempty"`
	Trials            int    `json:"trials"`
	Tasks             int    `json:"tasks"`

	Arms   []armSummary   `json:"arms"`
	Ratios []ratioSummary `json:"ratios"`
	Cells  []cellSummary  `json:"cells"`
	// Excluded lists the trials that failed for infrastructure or harness
	// reasons: disclosed, never scored.
	Excluded []failedTask `json:"excluded_trials"`
}

// harnessReasons are terminal reasons that say nothing about the task: the
// API or the network failed, or the sandbox leaked. Design rule 10.
var harnessReasons = map[string]bool{"infra": true, "isolation_leak": true}

type armSummary struct {
	Arm             string             `json:"arm"`
	Trials          int                `json:"trials"`
	Tasks           int                `json:"tasks"`
	PassRate        float64            `json:"pass_rate"`
	PassAllTasks    int                `json:"pass_all_tasks"`
	CostPerSolved   *float64           `json:"cost_per_solved_task"`
	Median          map[string]float64 `json:"median"`
	Mean            map[string]float64 `json:"mean"`
	TokenShare      map[string]float64 `json:"token_share"`
	Failed          []failedTask       `json:"failed_tasks"`
	TerminalReasons map[string]int     `json:"terminal_reasons"`
}

type failedTask struct {
	Task    string `json:"task"`
	Arm     string `json:"arm,omitempty"`
	Trial   int    `json:"trial"`
	Reason  string `json:"reason"`
	Treason string `json:"terminal_reason"`
}

// ratioSummary is a paired comparison: for each task with both cells, the
// ratio of the two cells' medians; then the median of those ratios across
// tasks with a bootstrap interval over tasks.
type ratioSummary struct {
	Numerator   string              `json:"numerator"`
	Denominator string              `json:"denominator"`
	Tasks       int                 `json:"tasks"`
	Median      map[string]float64  `json:"median"`
	CI          map[string]interval `json:"ci95"`
}

type cellSummary struct {
	Task     string             `json:"task"`
	Arm      string             `json:"arm"`
	Trials   int                `json:"trials"`
	PassRate float64            `json:"pass_rate"`
	PassAll  bool               `json:"pass_all"`
	Median   map[string]float64 `json:"median"`
}

// summarize computes every table from the trials. It is a pure function of
// the trials, the run-id (the bootstrap seed), and the versions passed in.
func summarize(runID string, trials []Trial, m *manifest) summary {
	s := summary{RunID: runID, Trials: len(trials), Excluded: []failedTask{}}
	if m != nil {
		s.Model = m.Model
		s.ClaudeCodeVersion = m.ClaudeCodeVersion
		s.AgentUtilsVersion = m.AgentUtilsVersion
	} else if len(trials) > 0 {
		s.Model = trials[0].Model
	}

	cells := map[string]map[string][]Trial{} // task -> arm -> trials
	taskSet := map[string]bool{}
	for _, t := range trials {
		if harnessReasons[t.TerminalReason] {
			s.Excluded = append(s.Excluded, failedTask{Task: t.Task, Trial: t.Trial, Reason: t.CheckReason, Treason: t.TerminalReason, Arm: t.Arm})
			continue
		}
		if cells[t.Task] == nil {
			cells[t.Task] = map[string][]Trial{}
		}
		cells[t.Task][t.Arm] = append(cells[t.Task][t.Arm], t)
		taskSet[t.Task] = true
	}
	tasks := sortedKeys(taskSet)
	s.Tasks = len(tasks)

	for _, arm := range arms {
		var armTrials []Trial
		a := armSummary{Arm: arm, Median: map[string]float64{}, Mean: map[string]float64{}, TokenShare: map[string]float64{}, TerminalReasons: map[string]int{}}
		for _, task := range tasks {
			cell := cells[task][arm]
			if len(cell) == 0 {
				continue
			}
			a.Tasks++
			if passAll(cell) {
				a.PassAllTasks++
			}
			armTrials = append(armTrials, cell...)
			c := cellSummary{Task: task, Arm: arm, Trials: len(cell), PassRate: passRate(cell), PassAll: passAll(cell), Median: map[string]float64{}}
			for _, met := range metrics {
				c.Median[met.Key] = median(values(cell, met.Of))
			}
			s.Cells = append(s.Cells, c)
			for _, t := range cell {
				if !t.Passed {
					a.Failed = append(a.Failed, failedTask{Task: task, Trial: t.Trial, Reason: t.CheckReason, Treason: t.TerminalReason})
				}
			}
		}
		if len(armTrials) == 0 {
			continue
		}
		a.Trials = len(armTrials)
		a.PassRate = passRate(armTrials)
		var cost, uncached, write, read, output float64
		passes := 0
		for _, t := range armTrials {
			cost += t.TotalCostUSD
			uncached += float64(t.InputTokens)
			write += float64(t.CacheCreationInputTokens)
			read += float64(t.CacheReadInputTokens)
			output += float64(t.OutputTokens)
			if t.Passed {
				passes++
			}
			a.TerminalReasons[t.TerminalReason]++
		}
		if passes > 0 {
			v := cost / float64(passes)
			a.CostPerSolved = &v
		}
		if total := uncached + write + read + output; total > 0 {
			a.TokenShare["uncached_input"] = uncached / total
			a.TokenShare["cache_write"] = write / total
			a.TokenShare["cache_read"] = read / total
			a.TokenShare["output"] = output / total
		}
		for _, met := range metrics {
			xs := values(armTrials, met.Of)
			a.Median[met.Key] = median(xs)
			a.Mean[met.Key] = mean(xs)
		}
		s.Arms = append(s.Arms, a)
	}

	for _, denom := range []string{"shell", "model"} {
		r := ratioSummary{Numerator: "tool", Denominator: denom, Median: map[string]float64{}, CI: map[string]interval{}}
		ratios := map[string][]float64{}
		for _, task := range tasks {
			num, den := cells[task]["tool"], cells[task][denom]
			if len(num) == 0 || len(den) == 0 {
				continue
			}
			r.Tasks++
			for _, met := range metrics {
				d := median(values(den, met.Of))
				if d == 0 {
					continue
				}
				ratios[met.Key] = append(ratios[met.Key], median(values(num, met.Of))/d)
			}
		}
		if r.Tasks == 0 {
			continue
		}
		for _, met := range metrics {
			xs := ratios[met.Key]
			r.Median[met.Key] = median(xs)
			r.CI[met.Key] = bootstrapCI(xs, median, seedFrom(runID+"/"+denom+"/"+met.Key))
		}
		s.Ratios = append(s.Ratios, r)
	}
	return s
}

func values(trials []Trial, of func(Trial) float64) []float64 {
	xs := make([]float64, len(trials))
	for i, t := range trials {
		xs[i] = of(t)
	}
	return xs
}

// writeMarkdown renders the summary as the tables a reader sees.
func writeMarkdown(w io.Writer, s summary) {
	fmt.Fprintf(w, "# Benchmark run %s\n\n", s.RunID)
	fmt.Fprintf(w, "Model `%s`", s.Model)
	if s.ClaudeCodeVersion != "" {
		fmt.Fprintf(w, ", Claude Code %s", s.ClaudeCodeVersion)
	}
	if s.AgentUtilsVersion != "" {
		fmt.Fprintf(w, ", agent-utils %s", s.AgentUtilsVersion)
	}
	fmt.Fprintf(w, ". %d trials over %d tasks", s.Trials, s.Tasks)
	if len(s.Excluded) > 0 {
		fmt.Fprintf(w, " (%d excluded, see the last section)", len(s.Excluded))
	}
	fmt.Fprintf(w, ". Costs are Claude Code's list-price estimates, the same basis in every arm.\n\n")

	fmt.Fprintf(w, "## Per arm\n\n")
	fmt.Fprintf(w, "| Arm | Trials | Tasks | Pass rate | Tasks with every trial passing | Cost per solved task | Median cost | Median tokens | Median turns | Median duration |\n")
	fmt.Fprintf(w, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, a := range s.Arms {
		cps := "n/a (no passes)"
		if a.CostPerSolved != nil {
			cps = fmt.Sprintf("$%.4f", *a.CostPerSolved)
		}
		fmt.Fprintf(w, "| `%s` | %d | %d | %s | %d of %d | %s | $%.4f | %s | %s | %s s |\n",
			a.Arm, a.Trials, a.Tasks, percent(a.PassRate), a.PassAllTasks, a.Tasks, cps,
			a.Median["cost_usd"], integer(a.Median["tokens"]), number(a.Median["turns"]), number(a.Median["duration_s"]))
	}

	if len(s.Ratios) > 0 {
		fmt.Fprintf(w, "\n## Paired ratios (tool over the other arm)\n\n")
		fmt.Fprintf(w, "The ratio of the two arms' medians, per task, then the median across tasks with a 95%% bootstrap interval over tasks. Below 1 means the tool arm used less.\n\n")
		fmt.Fprintf(w, "| Comparison | Tasks | Cost | Tokens | Turns | Duration |\n")
		fmt.Fprintf(w, "| --- | --- | --- | --- | --- | --- |\n")
		for _, r := range s.Ratios {
			fmt.Fprintf(w, "| `tool` / `%s` | %d |", r.Denominator, r.Tasks)
			for _, met := range metrics {
				ci := r.CI[met.Key]
				fmt.Fprintf(w, " %.2f [%.2f, %.2f] |", r.Median[met.Key], ci.Low, ci.High)
			}
			fmt.Fprintf(w, "\n")
		}
	}

	fmt.Fprintf(w, "\n## Token composition per arm\n\n")
	fmt.Fprintf(w, "| Arm | Uncached input | Cache write | Cache read | Output |\n")
	fmt.Fprintf(w, "| --- | --- | --- | --- | --- |\n")
	for _, a := range s.Arms {
		fmt.Fprintf(w, "| `%s` | %s | %s | %s | %s |\n", a.Arm,
			percent(a.TokenShare["uncached_input"]), percent(a.TokenShare["cache_write"]),
			percent(a.TokenShare["cache_read"]), percent(a.TokenShare["output"]))
	}

	fmt.Fprintf(w, "\n## Per task\n\n")
	fmt.Fprintf(w, "| Task | Arm | Trials | Pass rate | Median cost | Median tokens | Median turns | Median duration |\n")
	fmt.Fprintf(w, "| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, c := range s.Cells {
		fmt.Fprintf(w, "| %s | `%s` | %d | %s | $%.4f | %s | %s | %s s |\n", c.Task, c.Arm, c.Trials, percent(c.PassRate),
			c.Median["cost_usd"], integer(c.Median["tokens"]), number(c.Median["turns"]), number(c.Median["duration_s"]))
	}

	scale := scaleRows(s.Cells)
	if len(scale) > 0 {
		fmt.Fprintf(w, "\n## Input-size scaling (median tokens)\n\n")
		fmt.Fprintf(w, "| Task | `tool` | `shell` | `model` |\n")
		fmt.Fprintf(w, "| --- | --- | --- | --- |\n")
		for _, row := range scale {
			fmt.Fprintf(w, "| %s |", row.task)
			for _, arm := range arms {
				if v, ok := row.tokens[arm]; ok {
					fmt.Fprintf(w, " %s |", integer(v))
				} else {
					fmt.Fprintf(w, " n/a |")
				}
			}
			fmt.Fprintf(w, "\n")
		}
	}

	fmt.Fprintf(w, "\n## Failed trials\n\n")
	any := false
	for _, a := range s.Arms {
		if len(a.Failed) == 0 {
			continue
		}
		any = true
		fmt.Fprintf(w, "`%s`:\n\n", a.Arm)
		for _, f := range a.Failed {
			reason := f.Reason
			if reason == "" {
				reason = "(no checker reason)"
			}
			fmt.Fprintf(w, "- %s trial %d (%s): %s\n", f.Task, f.Trial, f.Treason, firstLine(reason))
		}
		fmt.Fprintf(w, "\n")
	}
	if !any {
		fmt.Fprintf(w, "None.\n")
	}

	fmt.Fprintf(w, "\n## Excluded trials (infrastructure or harness failures, not scored)\n\n")
	if len(s.Excluded) == 0 {
		fmt.Fprintf(w, "None.\n")
	}
	for _, f := range s.Excluded {
		fmt.Fprintf(w, "- %s `%s` trial %d (%s): %s\n", f.Task, f.Arm, f.Trial, f.Treason, firstLine(f.Reason))
	}
}

type scaleRow struct {
	task   string
	tokens map[string]float64
}

// scaleRows picks the size-scaling tasks (case names starting with "scale-")
// and their median tokens per arm.
func scaleRows(cells []cellSummary) []scaleRow {
	byTask := map[string]map[string]float64{}
	for _, c := range cells {
		if !strings.HasPrefix(c.Task[strings.Index(c.Task, "/")+1:], "scale-") {
			continue
		}
		if byTask[c.Task] == nil {
			byTask[c.Task] = map[string]float64{}
		}
		byTask[c.Task][c.Arm] = c.Median["tokens"]
	}
	var rows []scaleRow
	for _, task := range sortedKeys(byTask) {
		rows = append(rows, scaleRow{task, byTask[task]})
	}
	return rows
}

// writeSweep renders the model-sweep table from several runs' summaries,
// one row per run and arm.
func writeSweep(w io.Writer, summaries []summary) {
	fmt.Fprintf(w, "# Model sweep\n\n")
	fmt.Fprintf(w, "| Run | Model | Arm | Trials | Pass rate | Cost per solved task | Median cost | Median tokens | Median turns |\n")
	fmt.Fprintf(w, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, s := range summaries {
		for _, a := range s.Arms {
			cps := "n/a"
			if a.CostPerSolved != nil {
				cps = fmt.Sprintf("$%.4f", *a.CostPerSolved)
			}
			fmt.Fprintf(w, "| %s | `%s` | `%s` | %d | %s | %s | $%.4f | %s | %s |\n", s.RunID, s.Model, a.Arm, a.Trials,
				percent(a.PassRate), cps, a.Median["cost_usd"], integer(a.Median["tokens"]), number(a.Median["turns"]))
		}
	}
}

func percent(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }
func integer(f float64) string { return fmt.Sprintf("%d", int64(math.Round(f))) }
func number(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f", f)
}
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// reportRun reads one run directory, writes summary.md and summary.json
// into it, and returns the summary.
func reportRun(dir string) (summary, error) {
	trials, err := readTrials(filepath.Join(dir, "trials.jsonl"))
	if err != nil {
		return summary{}, err
	}
	if len(trials) == 0 {
		return summary{}, fmt.Errorf("%s: no trials in trials.jsonl", dir)
	}
	m, err := readManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return summary{}, err
	}
	runID := filepath.Base(dir)
	if m != nil {
		runID = m.RunID
	}
	sort.SliceStable(trials, func(i, j int) bool {
		if trials[i].Task != trials[j].Task {
			return trials[i].Task < trials[j].Task
		}
		if trials[i].Arm != trials[j].Arm {
			return trials[i].Arm < trials[j].Arm
		}
		return trials[i].Trial < trials[j].Trial
	})
	s := summarize(runID, trials, m)
	var md strings.Builder
	writeMarkdown(&md, s)
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(md.String()), 0o644); err != nil {
		return s, err
	}
	js, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return s, err
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(js, '\n'), 0o644); err != nil {
		return s, err
	}
	return s, nil
}
