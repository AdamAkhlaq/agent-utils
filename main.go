package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/adamakhlaq/dev-utils/internal/cli"
	"github.com/adamakhlaq/dev-utils/internal/encode"
	"github.com/adamakhlaq/dev-utils/internal/img"
)

func main() {
	err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "dev-utils:", err)
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
		cli.EncodeCommand("hex", "hex-encode or -decode input (-d to decode)", encode.Hex, encode.HexDecode),
		cli.EncodeCommand("url", "URL-encode or -decode input (-d to decode)", encode.URL, encode.URLDecode),
		cli.QRCommand(img.QR, img.QRDecode),
	} {
		commands[cmd.Name] = cmd
	}
	return cli.Dispatch(commands, args, stdin, stdout, stderr)
}
