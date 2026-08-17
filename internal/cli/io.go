package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// openInOut returns a conversion command's streams from its positionals:
// none means stdin to stdout, one names the input file, two name input and
// output. Callers must Close both and check the output's Close error.
func openInOut(fs *flag.FlagSet, stdin io.Reader, stdout io.Writer) (io.ReadCloser, io.WriteCloser, error) {
	if fs.NArg() > 2 {
		return nil, nil, &UsageError{Err: fmt.Errorf("%s: expected at most an input and an output file, got %d arguments", fs.Name(), fs.NArg())}
	}
	if fs.NArg() == 2 && filepath.Clean(fs.Arg(0)) == filepath.Clean(fs.Arg(1)) {
		// os.Create truncates the output before the input is read, so writing
		// a file onto itself would destroy it.
		return nil, nil, &UsageError{Err: fmt.Errorf("%s: input and output are the same file", fs.Name())}
	}
	var in io.ReadCloser = io.NopCloser(stdin)
	if fs.NArg() >= 1 {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", fs.Name(), err)
		}
		in = f
	}
	outPath := ""
	if fs.NArg() == 2 {
		outPath = fs.Arg(1)
	}
	out, err := openOutput(outPath, stdout)
	if err != nil {
		in.Close()
		return nil, nil, err
	}
	return in, out, nil
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
