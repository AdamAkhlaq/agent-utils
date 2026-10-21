package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNetworkIsolation brings up a run's network and proxy and checks, from
// a container on that network, that the only way out is the proxy and the
// only host the proxy serves is the API. It needs the built images, so it
// runs with BENCH_DOCKER=1 only.
func TestNetworkIsolation(t *testing.T) {
	if os.Getenv("BENCH_DOCKER") != "1" {
		t.Skip("set BENCH_DOCKER=1 to test the network isolation in Docker")
	}
	if err := dockerAvailable(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "proxy.log")
	net, err := startNetwork("network-test", logPath)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = net.stop()
		}
	})

	// curl prints the HTTP status it got, or "000" with a non-zero exit when
	// the connection never got that far.
	probe := func(args ...string) (string, int) {
		cmd := exec.Command("docker", append([]string{"run", "--rm", "--network", net.Name, "--entrypoint", "curl", baseImage,
			"-sS", "-o", "/dev/null", "-m", "20", "-w", "%{http_code}"}, args...)...)
		out, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out)), code
	}

	t.Run("api through the proxy", func(t *testing.T) {
		out, code := probe("-x", net.ProxyURL(), "https://api.anthropic.com/v1/models")
		if code != 0 || !strings.HasPrefix(out, "4") {
			t.Errorf("want an HTTP 4xx from the API (no credential), got status %q, curl exit %d", out, code)
		}
	})
	t.Run("other host refused by the proxy", func(t *testing.T) {
		out, code := probe("-x", net.ProxyURL(), "https://example.com/")
		if code == 0 || !strings.Contains(out, "403") {
			t.Errorf("want curl to fail with the proxy's 403, got %q, exit %d", out, code)
		}
	})
	t.Run("no direct egress", func(t *testing.T) {
		out, code := probe("https://api.anthropic.com/v1/models")
		if code == 0 {
			t.Errorf("direct connection succeeded with status %q; the network is not internal", out)
		}
	})

	stopped = true
	if err := net.stop(); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), `refused CONNECT "example.com:443"`) {
		t.Errorf("proxy log lacks the refusal:\n%s", log)
	}
	if countRefusals(logPath) != 1 {
		t.Errorf("countRefusals = %d, want 1", countRefusals(logPath))
	}
}
