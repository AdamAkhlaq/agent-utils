package cli

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// HashCommand builds the hash command around the digest package's pure Sum
// function, keeping this package independent of it; main wires the layers
// together.
func HashCommand(sum func(algo string, r io.Reader) (string, error)) Command {
	return Command{
		Name:    "hash",
		Summary: "hash input with sha256/sha1/sha512/md5 (-a algo, -c verify)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("hash", flag.ContinueOnError)
			fs.SetOutput(stderr)
			algo := fs.String("a", "sha256", "hash algorithm: sha256, sha1, sha512, or md5")
			check := fs.String("c", "", "verify: compare the digest to this hex checksum, print nothing on match")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}

			// Hashing empty input both validates -a and yields the digest
			// length -c must match, so every bad flag value exits 2 here,
			// before any input is read or consumed.
			probe, err := sum(*algo, strings.NewReader(""))
			if err != nil {
				return &UsageError{Err: fmt.Errorf("hash: %w", err)}
			}
			verify := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "c" {
					verify = true
				}
			})
			expected := strings.ToLower(strings.TrimSpace(*check))
			if verify {
				if _, err := hex.DecodeString(expected); err != nil || len(expected) != len(probe) {
					return &UsageError{Err: fmt.Errorf("hash: -c value must be %d hex characters for %s, got %q", len(probe), *algo, expected)}
				}
			}

			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			got, err := sum(*algo, in)
			if err != nil {
				return fmt.Errorf("hash: %w", err)
			}
			if verify {
				if got != expected {
					return fmt.Errorf("hash: checksum mismatch: expected %s, got %s", expected, got)
				}
				return nil
			}
			_, err = fmt.Fprintln(stdout, got)
			return err
		},
	}
}
