package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// newFlagSet returns a FlagSet whose own output is silenced. Left alone, the
// flag package prints parse errors and usage itself, which duplicates the
// error main reports and sends requested help to stderr; parseFlags and
// Dispatch own that output instead.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseFlags parses args into fs. A -h or -help flag becomes a *helpRequest
// for Dispatch to render; any other parse failure is a usage error.
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, flag.ErrHelp):
		return &helpRequest{flags: fs}
	default:
		return &UsageError{Err: fmt.Errorf("%s: %w (run 'agent-utils %s -h' for usage)", fs.Name(), err, fs.Name())}
	}
}

// helpRequest carries a command's FlagSet out of its Run function, because
// only the command knows its flags and only Dispatch knows its metadata.
type helpRequest struct {
	flags *flag.FlagSet
}

func (h *helpRequest) Error() string { return "help requested" }
