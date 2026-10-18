package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
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
// output. Callers must Close the input and finish the output with Commit or
// Abort.
func openInOut(fs *flag.FlagSet, stdin io.Reader, stdout io.Writer) (io.ReadCloser, output, error) {
	if fs.NArg() > 2 {
		return nil, nil, &UsageError{Err: fmt.Errorf("%s: expected at most an input and an output file, got %d arguments", fs.Name(), fs.NArg())}
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

// output is a command's result sink. Commit makes the written result final;
// Abort discards it after a failure.
type output interface {
	io.Writer
	Commit() error
	Abort()
}

// openOutput returns stdout when path is empty, or an output for the named
// file. A regular file is written under a temporary name beside it and only
// renamed into place by Commit, so a failed command never truncates or
// half-writes an existing file, and an output may safely be the input file.
// Anything else (/dev/null, a FIFO) is written directly, since renaming over
// it would replace the special file itself.
func openOutput(path string, stdout io.Writer) (output, error) {
	if path == "" {
		return stdoutOutput{stdout}, nil
	}
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	perm := os.FileMode(0o666)
	info, err := os.Stat(target)
	switch {
	case err == nil && !info.Mode().IsRegular():
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return nil, fmt.Errorf("opening output file: %w", err)
		}
		return &directOutput{f}, nil
	case err == nil:
		perm = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("opening output file: %w", err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, fmt.Errorf("naming temporary output file: %w", err)
	}
	tmp := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".tmp-"+hex.EncodeToString(suffix))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	// OpenFile applies the umask; an existing file keeps its exact mode.
	if info != nil {
		if err := f.Chmod(perm); err != nil {
			f.Close()
			os.Remove(tmp)
			return nil, fmt.Errorf("creating output file: %w", err)
		}
	}
	return &fileOutput{f: f, tmp: tmp, target: target}, nil
}

// writeOutput runs write against the output named by path (stdout when
// empty), committing the result only if write succeeds.
func writeOutput(path string, stdout io.Writer, write func(w io.Writer) error) error {
	out, err := openOutput(path, stdout)
	if err != nil {
		return err
	}
	if err := write(out); err != nil {
		out.Abort()
		return err
	}
	return out.Commit()
}

type stdoutOutput struct{ io.Writer }

func (stdoutOutput) Commit() error { return nil }
func (stdoutOutput) Abort()        {}

type directOutput struct{ *os.File }

func (o *directOutput) Commit() error {
	if err := o.Close(); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}
	return nil
}

func (o *directOutput) Abort() { o.Close() }

type fileOutput struct {
	f           *os.File
	tmp, target string
}

func (o *fileOutput) Write(p []byte) (int, error) { return o.f.Write(p) }

// Commit closes the temporary file, where buffered write errors surface,
// before renaming it over the target.
func (o *fileOutput) Commit() error {
	if err := o.f.Close(); err != nil {
		os.Remove(o.tmp)
		return fmt.Errorf("writing output file: %w", err)
	}
	if err := os.Rename(o.tmp, o.target); err != nil {
		os.Remove(o.tmp)
		return fmt.Errorf("writing output file: %w", err)
	}
	return nil
}

func (o *fileOutput) Abort() {
	o.f.Close()
	os.Remove(o.tmp)
}
