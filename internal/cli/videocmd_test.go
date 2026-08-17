package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// The stub downloader records its arguments, so these tests exercise only the
// CLI layer's flag handling, not any real downloading.
func TestVideoCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantURL   string
		wantDir   string
		wantAudio bool
		wantErr   bool
		wantUsage bool
	}{
		{name: "defaults", args: []string{"https://example.com/v"}, wantURL: "https://example.com/v", wantDir: "."},
		{name: "output dir and audio", args: []string{"-o", "/tmp/music", "-audio", "u"}, wantURL: "u", wantDir: "/tmp/music", wantAudio: true},
		{name: "no url", args: nil, wantErr: true, wantUsage: true},
		{name: "two urls", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotURL, gotDir string
			var gotAudio bool
			cmd := VideoCommand(func(stdout, stderr io.Writer, url, dir string, audioOnly bool) error {
				gotURL, gotDir, gotAudio = url, dir, audioOnly
				fmt.Fprintln(stdout, "progress")
				return nil
			})
			var stdout, stderrBuf bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderrBuf)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if gotURL != tt.wantURL || gotDir != tt.wantDir || gotAudio != tt.wantAudio {
				t.Errorf("Run() called downloader with (%q, %q, %v), want (%q, %q, %v)",
					gotURL, gotDir, gotAudio, tt.wantURL, tt.wantDir, tt.wantAudio)
			}
			if got, want := stdout.String(), "progress\n"; got != want {
				t.Errorf("Run() stdout = %q, want %q", got, want)
			}
		})
	}
}

func TestVideoCommandDownloaderFailureIsRuntimeError(t *testing.T) {
	broken := errors.New("yt-dlp: exit status 1")
	cmd := VideoCommand(func(stdout, stderr io.Writer, url, dir string, audioOnly bool) error {
		return broken
	})
	var stdout, stderrBuf bytes.Buffer
	err := cmd.Run([]string{"u"}, strings.NewReader(""), &stdout, &stderrBuf)
	if !errors.Is(err, broken) {
		t.Fatalf("Run() error = %v, want %v", err, broken)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; a download failure must exit 1, not 2")
	}
}
