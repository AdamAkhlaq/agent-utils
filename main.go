package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	// The time command resolves IANA zones; Windows has no OS tz database,
	// so embed Go's (the binary must behave identically on every platform).
	_ "time/tzdata"

	"github.com/adamakhlaq/agent-utils/v2/internal/chars"
	"github.com/adamakhlaq/agent-utils/v2/internal/cli"
	"github.com/adamakhlaq/agent-utils/v2/internal/clock"
	"github.com/adamakhlaq/agent-utils/v2/internal/digest"
	"github.com/adamakhlaq/agent-utils/v2/internal/download"
	"github.com/adamakhlaq/agent-utils/v2/internal/encode"
	"github.com/adamakhlaq/agent-utils/v2/internal/format"
	"github.com/adamakhlaq/agent-utils/v2/internal/generate"
	"github.com/adamakhlaq/agent-utils/v2/internal/hue"
	"github.com/adamakhlaq/agent-utils/v2/internal/img"
	"github.com/adamakhlaq/agent-utils/v2/internal/inspect"
	"github.com/adamakhlaq/agent-utils/v2/internal/netcalc"
	"github.com/adamakhlaq/agent-utils/v2/internal/semver"
	"github.com/adamakhlaq/agent-utils/v2/internal/text"
)

// version is stamped by GoReleaser via ldflags for release binaries. Other
// builds report their build info instead: the module version for `go install
// ...@vX.Y.Z`, a VCS pseudo-version for `go build` in a git checkout, and
// "dev" only when the toolchain recorded neither.
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
	// A signal (an agent's tool timeout, Ctrl-C) would otherwise leave the
	// temporary file of an unfinished output file behind.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signals
		cli.RemovePendingOutputs()
		code := 1
		if s, ok := sig.(syscall.Signal); ok {
			code = 128 + int(s)
		}
		os.Exit(code)
	}()

	err := recoverPanic(func() error {
		return run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	})
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "agent-utils:", err)
	os.Exit(exitCode(err))
}

// recoverPanic turns a panic in fn into an error carrying the stack. An
// unrecovered panic exits 2, which an agent would read as its own usage
// mistake; as an error it exits 1 like any other runtime failure.
func recoverPanic(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error (please report it at https://github.com/AdamAkhlaq/agent-utils/issues): %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

// exitCode maps an error to the exit-code contract: 1 runtime failure, 2
// usage error, 3 a predicate that answered false.
func exitCode(err error) int {
	var usageErr *cli.UsageError
	var falseErr *cli.FalseError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &usageErr):
		return 2
	case errors.As(err, &falseErr):
		return 3
	default:
		return 1
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return cli.Dispatch(registry(), args, stdin, stdout, stderr)
}

// registry builds every command, wiring each CLI layer to its pure core.
func registry() map[string]cli.Command {
	filetypePlain := func(r io.Reader) (string, error) {
		info, err := inspect.Detect(r)
		return info.MIME, err
	}
	filetypeJSON := func(r io.Reader) (string, error) {
		info, err := inspect.Detect(r)
		if err != nil {
			return "", err
		}
		return inspect.JSON(info)
	}
	cidrInfoJSON := func(prefix string) (string, error) {
		d, err := netcalc.Info(prefix)
		if err != nil {
			return "", err
		}
		return netcalc.JSON(d)
	}
	urlparseJSON := func(raw string) (string, error) {
		info, err := inspect.ParseURL(raw)
		if err != nil {
			return "", err
		}
		return inspect.URLJSON(info)
	}
	imgResize := func(w io.Writer, r io.Reader, width, height, max, quality int) error {
		return img.Resize(w, r, img.ResizeOptions{Width: width, Height: height, Max: max, Quality: quality})
	}
	certDecode := func(w io.Writer, r io.Reader) error {
		certs, err := inspect.Certificates(r, time.Now())
		if err != nil {
			return err
		}
		out, err := inspect.CertJSON(certs)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, out)
		return err
	}
	semverCheck := func(constraint string) (func(version string) (bool, error), error) {
		c, err := semver.ParseConstraint(constraint)
		if err != nil {
			return nil, err
		}
		return c.MatchString, nil
	}
	cidrSplit := func(prefix string, newLen int) ([]string, error) {
		subnets, err := netcalc.Split(prefix, newLen)
		if errors.Is(err, netcalc.ErrSplitLength) {
			return nil, &cli.UsageError{Err: err}
		}
		return subnets, err
	}
	jsonValid := func(r io.Reader) error {
		err := format.JSONValid(r)
		if errors.Is(err, format.ErrInvalidJSON) {
			return &cli.FalseError{Err: err}
		}
		return err
	}
	commands := make(map[string]cli.Command)
	for _, cmd := range []cli.Command{
		cli.EncodeCommand("base64", "base64-encode or -decode input (-d to decode)", encode.Base64, encode.Base64Decode),
		cli.ConvertCommand("bmp2png", "convert a BMP image to PNG", img.BMPToPNG),
		cli.CaseCommand(text.Case),
		cli.TransformCommand("cert-decode", "decode X.509 certificates (PEM or DER) to a JSON array", certDecode),
		cli.CIDRCommand(cidrInfoJSON, netcalc.Contains, netcalc.Overlaps, cidrSplit),
		cli.ColorCommand(hue.Convert, hue.JSON),
		cli.CSVToJSONCommand(format.CSVToJSON),
		cli.FiletypeCommand(filetypePlain, filetypeJSON),
		cli.ConvertCommand("gif2png", "convert a GIF image to PNG (first frame of an animation)", img.GIFToPNG),
		cli.HashCommand(digest.Sum),
		cli.EncodeCommand("hex", "hex-encode or -decode input (-d to decode)", encode.Hex, encode.HexDecode),
		cli.ImgResizeCommand(imgResize),
		cli.EncodeCommand("url", "URL-encode or -decode input (-d to decode)", encode.URL, encode.URLDecode),
		cli.URLParseCommand(urlparseJSON),
		cli.ConvertCommand("jpeg2png", "convert a JPEG image to PNG", img.JPEGToPNG),
		cli.JSONFmtCommand(format.JSON, format.JSONCompact, jsonValid),
		cli.JSONToCSVCommand(format.JSONToCSV),
		cli.TransformCommand("json2toml", "convert a JSON document to TOML", format.JSONToTOML),
		cli.TransformCommand("json2yaml", "convert a JSON document to YAML", format.JSONToYAML),
		cli.TransformCommand("jwt-decode", "decode a JWT's header and payload (does not verify the signature)", encode.JWTDecode),
		cli.PNGToJPEGCommand(img.PNGToJPEG),
		cli.LoremCommand(generate.LoremWords, generate.LoremParagraphs),
		cli.MarkdownTableCommand(text.MarkdownTable, text.MarkdownTableCSV),
		cli.PasswordCommand(generate.Password),
		cli.QRCommand(img.QR, img.QRDecode),
		cli.SemverCommand(semver.Sort, semver.CompareStrings, semverCheck),
		cli.StringTransformCommand("slugify", "turn text into a lowercase hyphenated slug", text.Slugify),
		cli.ConvertCommand("strip-ansi", "remove ANSI escape sequences (colors, cursor movement, OSC) from text", text.StripANSI),
		cli.TimeCommand(time.Now, clock.Parse, clock.Format, clock.JSON),
		cli.ConvertCommand("tiff2png", "convert a TIFF image to PNG (first page of a multi-page file)", img.TIFFToPNG),
		cli.TransformCommand("toml2json", "convert a TOML document to pretty-printed JSON", format.TOMLToJSON),
		cli.UnicodeCommand(chars.InspectJSON, chars.CheckJSON, chars.NFC, chars.NFD),
		cli.UUIDCommand(generate.UUID),
		cli.VersionCommand(resolveVersion()),
		cli.VideoCommand(download.Video),
		cli.ConvertCommand("webp2png", "convert a WebP image to PNG", img.WebPToPNG),
		cli.TransformCommand("xml2json", "convert an XML document to pretty-printed JSON", format.XMLToJSON),
		cli.TransformCommand("yaml2json", "convert a YAML document to pretty-printed JSON", format.YAMLToJSON),
	} {
		cmd.Examples = examples[cmd.Name]
		commands[cmd.Name] = cmd
	}
	meta := cli.CommandsCommand(commands)
	meta.Examples = examples[meta.Name]
	commands[meta.Name] = meta
	return commands
}
