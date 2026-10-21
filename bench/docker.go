package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Image tags. The base image is the "shell" arm and runs every checker; the
// tool image adds agent-utils from a release tag and is the "tool" arm.
const (
	baseImage = "agent-utils-bench:base"
	toolImage = "agent-utils-bench:tool"
)

// scriptTimeout bounds an oracle or checker run. Both are a few commands
// over small fixtures, so a minute means something is stuck, not slow.
const scriptTimeout = time.Minute

// buildImages builds the base and tool images from bench/image, installing
// the agent-utils release named by version into the tool image. Docker's
// own progress goes to stderr.
func buildImages(root, version string) error {
	dir := filepath.Join(root, "bench", "image")
	for _, target := range []struct{ name, tag string }{{"base", baseImage}, {"tool", toolImage}} {
		cmd := exec.Command("docker", "build", "--target", target.name, "--tag", target.tag,
			"--build-arg", "AGENT_UTILS_VERSION="+version, dir)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker build --target %s: %w", target.name, err)
		}
	}
	return nil
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
	return prefix + "-" + strings.NewReplacer("/", "-", "_", "-").Replace(t.Name)
}

// dockerAvailable reports whether a Docker daemon answers.
func dockerAvailable() error {
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil {
		return fmt.Errorf("docker is not available: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
