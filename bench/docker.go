package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Image tags. The base image is the "shell" arm and runs every checker; the
// tool image adds agent-utils from a release tag and is the "tool" arm; the
// proxy image is the only egress of a run's network.
const (
	baseImage  = "agent-utils-bench:base"
	toolImage  = "agent-utils-bench:tool"
	proxyImage = "agent-utils-bench:proxy"
)

// scriptTimeout bounds an oracle or checker run. Both are a few commands
// over small fixtures, so a minute means something is stuck, not slow.
const scriptTimeout = time.Minute

// buildImages builds the three images from bench/image/Dockerfile with the
// repository root as the context (the proxy stage compiles bench/proxy
// against go.mod), installing the agent-utils release named by version into
// the tool image. Docker's own progress goes to stderr.
func buildImages(root, version string) error {
	for _, target := range []struct{ name, tag string }{{"base", baseImage}, {"tool", toolImage}, {"proxy", proxyImage}} {
		cmd := exec.Command("docker", "build", "--file", filepath.Join(root, "bench", "image", "Dockerfile"),
			"--target", target.name, "--tag", target.tag,
			"--build-arg", "AGENT_UTILS_VERSION="+version, root)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker build --target %s: %w", target.name, err)
		}
	}
	return nil
}

// imageDigest returns the content-addressed ID of a built image, which the
// manifest records so a rerun can prove it used the same image.
func imageDigest(tag string) (string, error) {
	out, err := exec.Command("docker", "image", "inspect", "--format", "{{.Id}}", tag).Output()
	if err != nil {
		return "", fmt.Errorf("docker image inspect %s: %w", tag, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runOracle runs the task's oracle.sh in the tool image with /task read-only
// and the named volume mounted at /out, offline. It returns the script's
// combined output with any error.
func runOracle(t Task, volume string) (string, error) {
	return runScript(toolImage, t.Dir, volume, false, "/task/oracle.sh")
}

// runCheck runs the task's check.sh in the base image, where agent-utils is
// absent, with both /task and /out read-only. Exit 0 is a pass; the output
// carries the checker's reason otherwise.
func runCheck(t Task, volume string) (string, error) {
	return runScript(baseImage, t.Dir, volume, true, "/task/check.sh")
}

// runScript runs one script in a named container. Killing the docker client
// on timeout would leave the container running (and holding the volume), so
// the container is named and removed by force when the deadline passes.
func runScript(image, taskDir, volume string, outReadOnly bool, script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	out := volume + ":/out"
	if outReadOnly {
		out += ":ro"
	}
	name := volume + "-" + strings.TrimSuffix(path.Base(script), ".sh")
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none",
		"--volume", taskDir+":/task:ro", "--volume", out, "--entrypoint", script, image)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		_ = exec.Command("docker", "rm", "--force", name).Run()
		err = fmt.Errorf("%s did not finish within %s", script, scriptTimeout)
	}
	return strings.TrimSpace(buf.String()), err
}

// trialSpec is everything a trial container needs beyond the task.
type trialSpec struct {
	Name        string // container name
	Arm         string
	Image       string
	Task        Task
	ContextFile string // absolute path of the arm's context paragraph
	Volume      string // the /out volume
	Network     string // the run's internal network
	ProxyURL    string // HTTPS_PROXY inside that network
	Args        []string
	Timeout     time.Duration
	Stdout      *os.File // the transcript
	Stderr      *os.File
}

// errTrialTimeout marks a container killed at the task's timeout.
var errTrialTimeout = errors.New("trial timed out")

// runTrial runs one trial container to completion and returns Claude Code's
// exit code. The token is forwarded from the harness's environment by name,
// never placed on the command line. The context paragraph is bind-mounted
// at /context.md: a file cannot be mounted at a path that does not exist
// inside the read-only /task mount, and the task directory stays untouched.
func runTrial(spec trialSpec) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), spec.Timeout)
	defer cancel()
	args := []string{"run", "--rm", "--name", spec.Name, "--network", spec.Network,
		"--env", "CLAUDE_CODE_OAUTH_TOKEN",
		"--env", "HTTPS_PROXY=" + spec.ProxyURL,
		"--env", "HTTP_PROXY=" + spec.ProxyURL,
		"--volume", spec.Task.Dir + ":/task:ro",
		"--volume", spec.ContextFile + ":/context.md:ro",
		"--volume", spec.Volume + ":/out",
		spec.Image}
	args = append(args, spec.Args...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		_ = exec.Command("docker", "rm", "--force", spec.Name).Run()
		return -1, errTrialTimeout
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// 125 to 127 are the docker client's own codes (daemon error, command
		// not runnable, command not found): the harness failed, not the trial.
		if code := exit.ExitCode(); code >= 125 && code <= 127 {
			return code, fmt.Errorf("docker run failed with exit %d; see the trial's .stderr file", code)
		}
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, fmt.Errorf("docker run: %w", err)
	}
	return 0, nil
}

// copyOut copies the /out volume's contents to a host directory, through a
// helper container, since a named volume has no host path. The directory
// must be somewhere Docker can bind-mount (under the home directory on
// Colima and Docker Desktop). Ownership and times are not preserved: the
// bind mount's root refuses them on Colima, and only the contents matter.
func copyOut(volume, hostDir string) error {
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("docker", "run", "--rm", "--network", "none",
		"--volume", volume+":/out:ro", "--volume", hostDir+":/dst",
		"--entrypoint", "sh", baseImage, "-c", "cp -R /out/. /dst/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copying /out: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// createVolume makes an empty named volume for /out. Named volumes rather
// than bind mounts, so the container's non-root user can write it on every
// Docker setup (Colima, Docker Desktop, Linux) and the checker container sees
// exactly what the first container wrote. "docker volume create" silently
// reuses an existing volume, whose stale contents could pass a checker, so a
// volume that already exists is an error naming it.
func createVolume(name string) error {
	if err := exec.Command("docker", "volume", "inspect", name).Run(); err == nil {
		return fmt.Errorf("volume %s already exists (left by an interrupted run?); remove it with: docker volume rm %s", name, name)
	}
	if out, err := exec.Command("docker", "volume", "create", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeVolume(name string) error {
	if out, err := exec.Command("docker", "volume", "rm", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume rm: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// volumeName is a Docker-safe name for a task's /out volume.
func volumeName(prefix string, t Task) string {
	return prefix + "-" + dockerSafe(t.Name)
}

func dockerSafe(s string) string {
	return strings.NewReplacer("/", "-", "_", "-", ".", "-", ":", "-").Replace(s)
}

// network is a run's internal Docker network with its proxy sidecar. Trial
// containers on it can reach the proxy by name and nothing else; the proxy
// is also on the default bridge, which is its route out.
type network struct {
	Name  string
	Proxy string // container name
}

const proxyPort = "3128"

func (n network) ProxyURL() string {
	return "http://" + n.Proxy + ":" + proxyPort
}

// startNetwork creates the internal network and the proxy sidecar, whose
// log (every refused host) is written to logPath.
func startNetwork(runID, logPath string) (network, error) {
	n := network{Name: "agent-utils-bench-" + dockerSafe(runID), Proxy: "agent-utils-bench-" + dockerSafe(runID) + "-proxy"}
	if out, err := exec.Command("docker", "network", "create", "--internal", n.Name).CombinedOutput(); err != nil {
		return n, fmt.Errorf("docker network create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = n.stop()
		return n, err
	}
	defer logFile.Close()
	// Memory and process caps are defense in depth: the proxy bounds its own
	// buffers, and these keep a bug in it from costing the host.
	cmd := exec.Command("docker", "run", "--detach", "--name", n.Proxy, "--network", n.Name,
		"--memory", "64m", "--pids-limit", "64", "--read-only",
		proxyImage, "-listen", ":"+proxyPort, "-allow", "api.anthropic.com:443")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = n.stop()
		return n, fmt.Errorf("starting the proxy: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("docker", "network", "connect", "bridge", n.Proxy).CombinedOutput(); err != nil {
		_ = n.stop()
		return n, fmt.Errorf("connecting the proxy to the bridge: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Stream the proxy's log to the file for the life of the run.
	logs := exec.Command("docker", "logs", "--follow", n.Proxy)
	logs.Stdout = logFile
	logs.Stderr = logFile
	if err := logs.Start(); err != nil {
		_ = n.stop()
		return n, fmt.Errorf("docker logs: %w", err)
	}
	go func() { _ = logs.Wait() }()
	return n, nil
}

// stop removes the proxy and the network. Errors are returned joined, so a
// leftover is reported rather than lost.
func (n network) stop() error {
	var errs []error
	if out, err := exec.Command("docker", "rm", "--force", n.Proxy).CombinedOutput(); err != nil {
		errs = append(errs, fmt.Errorf("removing the proxy: %w: %s", err, strings.TrimSpace(string(out))))
	}
	if out, err := exec.Command("docker", "network", "rm", n.Name).CombinedOutput(); err != nil {
		errs = append(errs, fmt.Errorf("removing the network: %w: %s", err, strings.TrimSpace(string(out))))
	}
	return errors.Join(errs...)
}

// dockerAvailable reports whether a Docker daemon answers.
func dockerAvailable() error {
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil {
		return fmt.Errorf("docker is not available: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
