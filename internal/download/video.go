// Package download holds commands that shell out to external tools.
package download

import (
	"fmt"
	"io"
	"os/exec"
)

// Video downloads the video at url into dir using yt-dlp, streaming the
// tool's progress through stdout and stderr. audioOnly extracts just the
// audio track (yt-dlp -x, which needs ffmpeg).
func Video(stdout, stderr io.Writer, url, dir string, audioOnly bool) error {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return fmt.Errorf("yt-dlp not installed (brew install yt-dlp): %w", err)
	}
	cmd := exec.Command(path, videoArgs(url, dir, audioOnly)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}
	return nil
}

// videoArgs builds the yt-dlp argument list. The URL goes after "--" so one
// starting with "-" can't be read as a flag; there is no shell involved, so
// no other quoting or injection concerns exist.
func videoArgs(url, dir string, audioOnly bool) []string {
	args := []string{"-P", dir}
	if audioOnly {
		args = append(args, "-x")
	}
	return append(args, "--", url)
}
