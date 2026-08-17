package download

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVideoArgs(t *testing.T) {
	tests := []struct {
		name      string
		url, dir  string
		audioOnly bool
		want      []string
	}{
		{name: "video into current dir", url: "https://example.com/v", dir: ".", want: []string{"-P", ".", "--", "https://example.com/v"}},
		{name: "audio into named dir", url: "u", dir: "/tmp/music", audioOnly: true, want: []string{"-P", "/tmp/music", "-x", "--", "u"}},
		{name: "url starting with dash stays an argument", url: "-rf", dir: ".", want: []string{"-P", ".", "--", "-rf"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := videoArgs(tt.url, tt.dir, tt.audioOnly); !slices.Equal(got, tt.want) {
				t.Errorf("videoArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeYTDLP puts a shell script named yt-dlp on PATH, so these tests exercise
// the real lookup, spawn, and streaming paths without yt-dlp installed.
func fakeYTDLP(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestVideo(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		err := Video(io.Discard, io.Discard, "https://example.com/v", ".", false)
		if err == nil {
			t.Fatal("Video() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "yt-dlp not installed") {
			t.Errorf("Video() error = %q, want it to mention yt-dlp not installed", err)
		}
	})

	t.Run("passes arguments and streams stdout", func(t *testing.T) {
		fakeYTDLP(t, `printf '%s\n' "$@"`)
		var stdout, stderr bytes.Buffer
		if err := Video(&stdout, &stderr, "https://example.com/v", "/tmp/out", true); err != nil {
			t.Fatalf("Video() error = %v", err)
		}
		want := "-P\n/tmp/out\n-x\n--\nhttps://example.com/v\n"
		if got := stdout.String(); got != want {
			t.Errorf("Video() streamed %q, want %q", got, want)
		}
	})

	t.Run("failure propagates with stderr streamed", func(t *testing.T) {
		fakeYTDLP(t, `echo "boom" >&2; exit 3`)
		var stdout, stderr bytes.Buffer
		err := Video(&stdout, &stderr, "https://example.com/v", ".", false)
		if err == nil {
			t.Fatal("Video() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "exit status 3") {
			t.Errorf("Video() error = %q, want it to contain the exit status", err)
		}
		if got, want := stderr.String(), "boom\n"; got != want {
			t.Errorf("Video() stderr = %q, want %q", got, want)
		}
	})
}
