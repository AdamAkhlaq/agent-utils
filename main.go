package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	// The time command resolves IANA zones; Windows has no OS tz database,
	// so embed Go's (the binary must behave identically on every platform).
	_ "time/tzdata"

	"github.com/adamakhlaq/agent-utils/internal/cli"
	"github.com/adamakhlaq/agent-utils/internal/clock"
	"github.com/adamakhlaq/agent-utils/internal/download"
	"github.com/adamakhlaq/agent-utils/internal/encode"
	"github.com/adamakhlaq/agent-utils/internal/format"
	"github.com/adamakhlaq/agent-utils/internal/generate"
	"github.com/adamakhlaq/agent-utils/internal/img"
	"github.com/adamakhlaq/agent-utils/internal/text"
)

// version is stamped by GoReleaser via ldflags for release binaries; builds
// installed with `go install ...@vX.Y.Z` get their version from build info
// instead, and local `go build` reports "dev".
var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "agent-utils:", err)
	var usageErr *cli.UsageError
	if errors.As(err, &usageErr) {
		os.Exit(2)
	}
	os.Exit(1)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	commands := make(map[string]cli.Command)
	for _, cmd := range []cli.Command{
		cli.EncodeCommand("base64", "base64-encode or -decode input (-d to decode)", encode.Base64, encode.Base64Decode),
		cli.CSVToJSONCommand(format.CSVToJSON),
		cli.EncodeCommand("hex", "hex-encode or -decode input (-d to decode)", encode.Hex, encode.HexDecode),
		cli.EncodeCommand("url", "URL-encode or -decode input (-d to decode)", encode.URL, encode.URLDecode),
		cli.JSONFmtCommand(format.JSON, format.JSONCompact, format.JSONValid),
		cli.TransformCommand("jwt-decode", "decode a JWT's header and payload (does not verify the signature)", encode.JWTDecode),
		cli.ConvertCommand("jpeg2png", "convert a JPEG image to PNG", img.JPEGToPNG),
		cli.PNGToJPEGCommand(img.PNGToJPEG),
		cli.LoremCommand(generate.LoremWords, generate.LoremParagraphs),
		cli.PasswordCommand(generate.Password),
		cli.QRCommand(img.QR, img.QRDecode),
		cli.StringTransformCommand("slugify", "turn text into a lowercase hyphenated slug", text.Slugify),
		cli.TimeCommand(time.Now, clock.Parse, clock.Format, clock.JSON),
		cli.UUIDCommand(generate.UUID),
		cli.VersionCommand(resolveVersion()),
		cli.VideoCommand(download.Video),
		cli.TransformCommand("yaml2json", "convert a YAML document to pretty-printed JSON", format.YAMLToJSON),
	} {
		commands[cmd.Name] = cmd
	}
	meta := cli.CommandsCommand(commands)
	commands[meta.Name] = meta
	return cli.Dispatch(commands, args, stdin, stdout, stderr)
}
