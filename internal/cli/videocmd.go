package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// VideoCommand builds the video command around a downloader function, keeping
// this package independent of the download package; main wires the layers
// together.
func VideoCommand(dl func(stdout, stderr io.Writer, url, dir string, audioOnly bool) error) Command {
	return Command{
		Name:    "video",
		Summary: "download a video with yt-dlp (-audio for audio only)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("video", flag.ContinueOnError)
			fs.SetOutput(stderr)
			dir := fs.String("o", ".", "output directory")
			audio := fs.Bool("audio", false, "download audio only (extracted with ffmpeg)")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() != 1 {
				return &UsageError{Err: fmt.Errorf("video: expected exactly one URL argument")}
			}
			return dl(stdout, stderr, fs.Arg(0), *dir, *audio)
		},
	}
}
