package cli

import (
	"fmt"
	"io"
)

// VideoCommand builds the video command around a downloader function, keeping
// this package independent of the download package; main wires the layers
// together.
func VideoCommand(dl func(stdout, stderr io.Writer, url, dir string, audioOnly bool) error) Command {
	return Command{
		Name:    "video",
		Summary: "download a video with yt-dlp, printing the saved file path (-audio for audio only)",
		Usage:   "agent-utils video [-o dir] [-audio] <url>",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("video")
			dir := fs.String("o", ".", "output directory")
			audio := fs.Bool("audio", false, "download audio only (extracted with ffmpeg)")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return &UsageError{Err: fmt.Errorf("video: expected exactly one URL argument")}
			}
			return dl(stdout, stderr, fs.Arg(0), *dir, *audio)
		},
	}
}
