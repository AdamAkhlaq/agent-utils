package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// runMain is "bench run": resolve the configuration, pin the run in its
// manifest, bring up the network, execute the plan (resuming a run-id that
// already has trials), then report.
func runMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	model := fs.String("model", "", "full model ID, such as claude-sonnet-5-5 (required)")
	armList := fs.String("arms", "tool,named,shell,model", "arms to run, comma-separated")
	trials := fs.Int("trials", 0, "trials per task per arm (required)")
	glob := fs.String("tasks", "", "run only the tasks whose name matches this glob")
	tag := fs.String("tag", "", "run only the tasks carrying this tag (such as sweep)")
	runID := fs.String("run-id", "", "name of the run directory (default: <date>-<model>); an existing run is resumed")
	outDir := fs.String("out", "", "directory that holds run directories (default: bench/.runs)")
	concurrency := fs.Int("concurrency", 2, "trials in flight at once")
	version := fs.String("agent-utils-version", "", "release tag to install in the tool image (default: the latest v* tag)")
	root := fs.String("root", "", "repository root (default: found from the working directory)")
	printUsage := func(w io.Writer) {
		fmt.Fprintf(w, "usage: go run ./bench run -model <id> -trials N [-arms tool,named,shell,model] [-tasks glob] [-tag tag] [-run-id id] [-concurrency k]\n\n")
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "bench run: %v\n", err)
		printUsage(stderr)
		return 2
	}
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "bench run: "+format+"\n", a...)
		return 2
	}
	if fs.NArg() > 0 {
		return usageErr("unexpected argument %q", fs.Arg(0))
	}
	if *model == "" {
		return usageErr("-model is required")
	}
	if *trials <= 0 {
		return usageErr("-trials must be positive")
	}
	if *concurrency <= 0 {
		return usageErr("-concurrency must be positive")
	}
	if _, err := path.Match(*glob, ""); err != nil {
		return usageErr("-tasks %q: %v", *glob, err)
	}
	var selected []string
	for _, a := range strings.Split(*armList, ",") {
		a = strings.TrimSpace(a)
		if !slices.Contains(arms, a) {
			return usageErr("-arms: unknown arm %q (want a subset of %s)", a, strings.Join(arms, ","))
		}
		selected = append(selected, a)
	}
	if *tag != "" && !slices.Contains(tags, *tag) {
		return usageErr("-tag: unknown tag %q (want one of %s)", *tag, strings.Join(tags, ", "))
	}
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") == "" {
		fmt.Fprintf(stderr, "bench run: CLAUDE_CODE_OAUTH_TOKEN is not set; export the token from \"claude setup-token\" first\n")
		return 1
	}

	fail := func(err error) int {
		fmt.Fprintf(stderr, "bench run: %v\n", err)
		return 1
	}
	var err error
	if *root == "" {
		if *root, err = findRoot(); err != nil {
			return fail(err)
		}
	}
	if *root, err = filepath.Abs(*root); err != nil {
		return fail(err)
	}
	if *outDir == "" {
		*outDir = filepath.Join(*root, "bench", ".runs")
	}
	if *runID == "" {
		*runID = time.Now().UTC().Format("20060102-1504") + "-" + *model
	}
	if *version == "" {
		if *version, err = latestTag(*root); err != nil {
			return fail(err)
		}
	}

	result, err := validate(*root, *glob)
	if err != nil {
		return fail(err)
	}
	if len(result.problems) > 0 {
		for _, p := range result.problems {
			fmt.Fprintln(stderr, p)
		}
		return fail(errors.New("the task suite does not validate; fix the problems above first"))
	}
	tasks := result.tasks
	if *tag != "" {
		tasks = slices.DeleteFunc(slices.Clone(tasks), func(t Task) bool { return !slices.Contains(t.Tags, *tag) })
		if len(tasks) == 0 {
			return usageErr("-tag %q matches no task", *tag)
		}
	}

	cfg := runConfig{
		Root: *root, RunID: *runID, RunDir: filepath.Join(*outDir, *runID), Model: *model,
		Arms: sortedArms(selected), Trials: *trials, Concurrency: *concurrency, Version: *version,
		Tasks: tasks, Seed: seedFrom(*runID), Stderr: stderr,
	}
	if err := os.MkdirAll(cfg.RunDir, 0o755); err != nil {
		return fail(err)
	}
	existing, err := readTrials(filepath.Join(cfg.RunDir, "trials.jsonl"))
	if err != nil {
		return fail(err)
	}
	cells := plan(cfg.Tasks, cfg.Arms, cfg.Trials, cfg.Seed, doneKeys(existing))
	fmt.Fprintf(stderr, "run %s: %d tasks, arms %s, %d trials per cell: %d cells to run, %d already done\n",
		cfg.RunID, len(cfg.Tasks), strings.Join(cfg.Arms, ","), cfg.Trials, len(cells), len(existing))
	if len(cells) == 0 {
		fmt.Fprintf(stderr, "nothing to run; reporting\n")
		return reportMain([]string{cfg.RunDir}, stdout, stderr)
	}

	if err := dockerAvailable(); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stderr, "building images with agent-utils %s\n", cfg.Version)
	if err := buildImages(cfg.Root, cfg.Version); err != nil {
		return fail(err)
	}
	m, err := readManifest(filepath.Join(cfg.RunDir, "manifest.json"))
	if err != nil {
		return fail(err)
	}
	if m == nil {
		fresh, err := buildManifest(cfg)
		if err != nil {
			return fail(err)
		}
		m = &fresh
	} else if err := checkResume(*m, cfg); err != nil {
		return fail(err)
	}
	if err := writeManifest(filepath.Join(cfg.RunDir, "manifest.json"), *m); err != nil {
		return fail(err)
	}

	proxyLog := filepath.Join(cfg.RunDir, "proxy.log")
	net, err := startNetwork(cfg.RunID, proxyLog)
	if err != nil {
		return fail(err)
	}
	retries, execErr := execute(cfg, cells, net, m)
	if err := net.stop(); err != nil {
		fmt.Fprintf(stderr, "bench run: %v\n", err)
	}
	m.InfraRetries += retries
	m.ProxyRefusals = countRefusals(proxyLog)
	m.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(filepath.Join(cfg.RunDir, "manifest.json"), *m); err != nil {
		return fail(err)
	}
	if execErr != nil {
		return fail(fmt.Errorf("stopped: %w (rerun with the same -run-id to resume)", execErr))
	}
	return reportMain([]string{cfg.RunDir}, stdout, stderr)
}

// checkResume refuses to continue a run whose pins differ from the current
// configuration: a run directory holds one model and one release.
func checkResume(m manifest, cfg runConfig) error {
	if m.Model != cfg.Model {
		return fmt.Errorf("run %s was started with model %s, not %s; use a new -run-id", cfg.RunID, m.Model, cfg.Model)
	}
	if m.AgentUtilsVersion != cfg.Version {
		return fmt.Errorf("run %s was started with agent-utils %s, not %s; use a new -run-id", cfg.RunID, m.AgentUtilsVersion, cfg.Version)
	}
	for _, t := range cfg.Tasks {
		h, err := sha256File(filepath.Join(t.Dir, "instruction.md"))
		if err != nil {
			return err
		}
		if prev, ok := m.Tasks[t.Name]; ok && prev != h {
			return fmt.Errorf("run %s: %s/instruction.md changed since the run started; use a new -run-id", cfg.RunID, t.Name)
		}
	}
	return nil
}

// reportMain is "bench report": summaries for each run directory, and the
// sweep table on stdout when there are several.
func reportMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	printUsage := func(w io.Writer) {
		fmt.Fprintf(w, "usage: go run ./bench report <run-dir> [<run-dir>...]\n\nWrites summary.md and summary.json into each run directory and prints the summary (or, for several runs, the model sweep table).\n")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "bench report: %v\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintf(stderr, "bench report: at least one run directory is required\n")
		printUsage(stderr)
		return 2
	}
	var summaries []summary
	for _, dir := range fs.Args() {
		s, err := reportRun(dir)
		if err != nil {
			fmt.Fprintf(stderr, "bench report: %v\n", err)
			return 1
		}
		summaries = append(summaries, s)
	}
	if len(summaries) == 1 {
		writeMarkdown(stdout, summaries[0])
	} else {
		writeSweep(stdout, summaries)
	}
	return 0
}
