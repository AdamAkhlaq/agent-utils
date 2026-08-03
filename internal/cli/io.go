package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// openInput returns the command's input source: the file named by the single
// positional argument, or stdin when there is none. A positional is always a
// filename, never literal data.
func openInput(fs *flag.FlagSet, stdin io.Reader) (io.ReadCloser, error) {
	switch fs.NArg() {
	case 0:
		return io.NopCloser(stdin), nil
	case 1:
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fs.Name(), err)
		}
		return f, nil
	default:
		return nil, &UsageError{Err: fmt.Errorf("%s: expected at most one file argument, got %d", fs.Name(), fs.NArg())}
	}
}

// openOutput returns the command's output sink: the named file, or stdout when
// path is empty. Callers must Close it and check the error, because closing a
// written file is where write failures can surface.
func openOutput(path string, stdout io.Writer) (io.WriteCloser, error) {
	if path == "" {
		return nopWriteCloser{stdout}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	return f, nil
}

// nopWriteCloser is the io.NopCloser counterpart for writers, which the stdlib
// doesn't provide.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
