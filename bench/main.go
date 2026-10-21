// Command bench is the agent-utils benchmark harness: it validates the task
// suite under bench/tasks and, in later phases, runs the trial matrix and
// reports on it. It is a development tool in this module, never part of the
// released binary. Run it from the repository with "go run ./bench".
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const usage = `usage: go run ./bench <command> [flags]

Commands:
  validate   check every task: static rules, coverage, and each oracle through its checker in Docker
  run        run the trial matrix in Docker and write a run directory (needs CLAUDE_CODE_OAUTH_TOKEN)
  report     compute summary.md and summary.json for one or more run directories

Run "go run ./bench <command> -h" for a command's flags.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "validate":
		return validateMain(args[1:], stdout, stderr)
	case "run":
		return runMain(args[1:], stdout, stderr)
	case "report":
		return reportMain(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "bench: unknown command %q\n\n%s", args[0], usage)
	return 2
}

func validateMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	static := fs.Bool("static", false, "skip Docker: run only the checks that need no daemon")
	coverageRequired := fs.Bool("coverage", false, "fail when a command has no task (otherwise gaps are warnings)")
	glob := fs.String("tasks", "", "validate only the tasks whose name matches this glob, such as 'jwt-decode/*'")
	root := fs.String("root", "", "repository root (default: found from the working directory)")
	version := fs.String("agent-utils-version", "", "release tag to install in the tool image (default: the latest v* tag)")
	printUsage := func(w io.Writer) {
		fmt.Fprintf(w, "usage: go run ./bench validate [-static] [-coverage] [-tasks glob] [-root dir] [-agent-utils-version vX.Y.Z]\n\n")
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "bench validate: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if _, err := path.Match(*glob, ""); err != nil {
		fmt.Fprintf(stderr, "bench validate: -tasks %q: %v\n", *glob, err)
		return 2
	}
	if *root == "" {
		r, err := findRoot()
		if err != nil {
			fmt.Fprintf(stderr, "bench validate: %v\n", err)
			return 1
		}
		*root = r
	}
	// Docker bind mounts need absolute paths, whatever -root was given.
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		return 1
	}
	*root = abs

	result, err := validate(*root, *glob, *coverageRequired)
	var noMatch errNoMatch
	if errors.As(err, &noMatch) {
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		return 1
	}
	for _, w := range result.warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}
	for _, p := range result.problems {
		fmt.Fprintf(stderr, "%s\n", p)
	}
	if len(result.problems) > 0 {
		fmt.Fprintf(stderr, "%s in %s\n", plural(len(result.problems), "problem"), plural(len(result.tasks), "task"))
		return 1
	}
	// In the full run this line is progress, not a result: the result is the
	// oracle verdict, so stdout stays empty until the Docker step speaks.
	if *static {
		fmt.Fprintf(stdout, "static checks: %s ok\n", plural(len(result.tasks), "task"))
		return 0
	}
	fmt.Fprintf(stderr, "static checks: %s ok\n", plural(len(result.tasks), "task"))

	if *version == "" {
		v, err := latestTag(*root)
		if err != nil {
			fmt.Fprintf(stderr, "bench validate: %v\n", err)
			return 1
		}
		*version = v
	}
	if err := dockerAvailable(); err != nil {
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "building images with agent-utils %s\n", *version)
	if err := buildImages(*root, *version); err != nil {
		fmt.Fprintf(stderr, "bench validate: %v\n", err)
		return 1
	}
	failures := 0
	for _, t := range result.tasks {
		if err := checkOracle(t); err != nil {
			failures++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", t.Name, err)
			continue
		}
		fmt.Fprintf(stdout, "ok   %s\n", t.Name)
	}
	if failures > 0 {
		fmt.Fprintf(stderr, "%d of %d tasks failed the oracle step\n", failures, len(result.tasks))
		return 1
	}
	fmt.Fprintf(stdout, "oracles: %s ok (agent-utils %s)\n", plural(len(result.tasks), "task"), *version)
	return 0
}

// errNoMatch is a -tasks glob that selects nothing: a flag value the user
// got wrong, so a usage error rather than a failed check.
type errNoMatch string

func (e errNoMatch) Error() string { return fmt.Sprintf("-tasks %q matches no task", string(e)) }

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

type validation struct {
	tasks    []Task
	problems []string
	warnings []string
}

// validate runs every static check: each task's rules, the context files'
// equality of length and the tool paragraph's match with the README, and the
// coverage of the command list. Coverage gaps are warnings unless required.
func validate(root, glob string, coverageRequired bool) (validation, error) {
	var v validation
	commands, err := commandNames(filepath.Join(root, "docs", "commands"))
	if err != nil {
		return v, err
	}
	all, loadProblems := loadTasks(filepath.Join(root, "bench", "tasks"))
	v.tasks, err = filterTasks(all, glob)
	if err != nil {
		return v, err
	}
	// A task that could not be loaded is reported only when the glob selects
	// it, like any other problem, so -tasks restricts everything.
	for _, name := range sortedKeys(loadProblems) {
		if ok, _ := path.Match(glob, name); glob == "" || ok {
			v.problems = append(v.problems, name+": "+loadProblems[name])
			v.tasks = append(v.tasks, Task{Name: name})
		}
	}
	if glob != "" && len(v.tasks) == 0 {
		return v, errNoMatch(glob)
	}
	for _, t := range v.tasks {
		if t.Dir != "" {
			v.problems = append(v.problems, validateTask(t, commands)...)
		}
	}

	toolText, err := os.ReadFile(filepath.Join(root, "bench", "context", "tool.md"))
	if err != nil {
		return v, err
	}
	neutralText, err := os.ReadFile(filepath.Join(root, "bench", "context", "neutral.md"))
	if err != nil {
		return v, err
	}
	if p := contextLengthProblem(string(toolText), string(neutralText)); p != "" {
		v.problems = append(v.problems, p)
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return v, err
	}
	snippet, err := agentSnippet(string(readme))
	if err != nil {
		return v, err
	}
	if string(toolText) != snippet {
		v.problems = append(v.problems, "context/tool.md is not the README's agent snippet, verbatim")
	}

	gaps := coverage(commands, all)
	switch {
	case coverageRequired:
		v.problems = append(v.problems, gaps...)
	case len(gaps) > 0:
		v.warnings = append(v.warnings, fmt.Sprintf("coverage: %d gaps (pass -coverage to fail on them)\n  %s",
			len(gaps), strings.Join(gaps, "\n  ")))
	}
	return v, nil
}

// checkOracle runs the task's oracle and then its checker on what the oracle
// wrote. A task whose oracle fails its own checker is broken, not hard. A
// volume that cannot be removed afterwards is an error too, so nothing is
// left behind silently.
func checkOracle(t Task) (err error) {
	volume := volumeName("agent-utils-bench-validate", t)
	if err := createVolume(volume); err != nil {
		return err
	}
	defer func() {
		if rmErr := removeVolume(volume); rmErr != nil && err == nil {
			err = rmErr
		}
	}()
	if out, err := runOracle(t, volume); err != nil {
		return fmt.Errorf("oracle: %w\n%s", err, out)
	}
	if out, err := runCheck(t, volume); err != nil {
		return fmt.Errorf("checker rejected the oracle's output: %w\n%s", err, out)
	}
	return nil
}

// findRoot walks up from the working directory to the directory holding
// go.mod, so the harness works from the repository root or from bench/.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside the repository: no go.mod found above the working directory")
		}
		dir = parent
	}
}

// latestTag returns the newest v* tag reachable from HEAD, the release the
// tool image installs by default.
func latestTag(root string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v*")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no release tag found (pass -agent-utils-version): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
