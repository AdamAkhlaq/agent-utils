package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: supply an argument")
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading input: %v\n", err)
		os.Exit(1)
	}

	if os.Args[1] == "base64" {
		if len(os.Args) > 2 && os.Args[2] == "-d" {
			decoded, err := base64.StdEncoding.DecodeString(string(input))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while decoding input: %v\n", err)
				os.Exit(1)
			}
			os.Stdout.Write(decoded)
			return

		} else {
			fmt.Println(base64.StdEncoding.EncodeToString(input))
		}
	}
}
