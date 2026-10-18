// Package download holds commands that shell out to external tools.
package download

import (
	"fmt"
	"io"
	"os/exec"
)

// Video downloads the video at url into dir using yt-dlp, writing the path of
// each downloaded file to stdout, one per line, and yt-dlp's errors and
// warnings to stderr. audioOnly extracts just the audio track (yt-dlp -x,
// which needs ffmpeg).
func Video(stdout, stderr io.Writer, url, dir string, audioOnly bool) error {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return fmt.Errorf("yt-dlp not installed (see https://github.com/yt-dlp/yt-dlp#installation): %w", err)
	}
	cmd := exec.Command(path, videoArgs(url, dir, audioOnly)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}
	return nil
}

// videoArgs builds the yt-dlp argument list. --ignore-config stops yt-dlp
// loading a yt-dlp.conf from the output or current directory, which could
// otherwise add --exec commands or change what is printed. --print implies
// --quiet, so
// yt-dlp's stdout carries only the final paths (after_move is the stage after
// post-processing, so -x reports the extracted audio file) and its progress
// bar is suppressed. The URL goes after "--" so one starting with "-" can't
// be read as a flag; there is no shell involved, so no other quoting or
// injection concerns exist.
func videoArgs(url, dir string, audioOnly bool) []string {
	args := []string{"--ignore-config", "-P", dir, "--print", "after_move:filepath", "--color", "never"}
	if audioOnly {
		args = append(args, "-x")
	}
	return append(args, "--", url)
}
