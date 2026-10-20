package img

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"math"

	"golang.org/x/image/tiff"
)

// MaxPixels caps the size (width x height) of every image the image commands
// decode or create. Decoders allocate from the dimensions an image declares,
// not from its file size, so without a cap a few-kilobyte file declaring
// 30000x30000 pixels takes gigabytes of memory. 128 megapixels admits the
// largest common photos (100 MP medium format, 108 MP phone sensors), and
// converting an image at the limit peaks near 1 GB for 8-bit color and 1.5 GB
// for 16-bit. Some decoders buffer more than the pixels, so a file within the
// limit can still reach 2.3 to 3.6 GB: a TIFF strip may inflate to 8 bytes per
// pixel whatever its bit depth, and a progressive CMYK JPEG keeps every DCT
// coefficient as well as both color forms.
const MaxPixels = 128_000_000

// MaxQRSize is the largest QR code image QR generates: the largest size whose
// size x size image fits within MaxPixels.
const MaxQRSize = 11313

// checkPixels rejects a width x height image larger than MaxPixels. It
// divides instead of multiplying, so huge dimensions cannot overflow.
func checkPixels(what string, width, height int) error {
	if width > 0 && height > MaxPixels/width {
		return fmt.Errorf("%s is %dx%d pixels, over the limit of %d pixels (width x height)", what, width, height, MaxPixels)
	}
	return nil
}

// peekConfig reads the image header at the start of r with decodeConfig and
// returns a reader that replays the bytes it consumed before continuing with
// the rest of r, so a decoder can then read the whole image. For every format
// but TIFF (see decodeTIFFLimited) only the header is held in memory, so input
// that is not an image fails as soon as its first bytes are read.
func peekConfig(r io.Reader, decodeConfig func(io.Reader) (image.Config, error)) (image.Config, io.Reader, error) {
	var header bytes.Buffer
	cfg, err := decodeConfig(io.TeeReader(r, &header))
	return cfg, io.MultiReader(&header, r), err
}

// decodeLimited checks the dimensions an image declares in its header before
// decoding it, so an image over MaxPixels is rejected before its pixels are
// allocated. Each decoder used here allocates in proportion to the size its
// header declares (see MaxPixels): a GIF frame must fit the logical screen, a
// WebP frame must match its canvas, and a multi-page TIFF reports and decodes
// its first page.
func decodeLimited(r io.Reader, decodeConfig func(io.Reader) (image.Config, error), decode func(io.Reader) (image.Image, error)) (image.Image, error) {
	cfg, r, err := peekConfig(r, decodeConfig)
	if err != nil {
		return nil, err
	}
	if err := checkPixels("image", cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	return decode(r)
}

// limited returns decode guarded by decodeLimited.
func limited(decodeConfig func(io.Reader) (image.Config, error), decode func(io.Reader) (image.Image, error)) func(io.Reader) (image.Image, error) {
	return func(r io.Reader) (image.Image, error) {
		return decodeLimited(r, decodeConfig, decode)
	}
}

// decodeTIFFLimited is decodeLimited for TIFF. The header that DecodeConfig
// needs may follow the pixels, at the end of the file, so peeking it from a
// stream as decodeLimited does would buffer the file twice. A file is read in
// place instead. Any other stream is buffered as far as DecodeConfig reads,
// and that buffer is handed on to Decode, which keeps its own copy, releasing
// each part as Decode consumes it.
func decodeTIFFLimited(r io.Reader) (image.Image, error) {
	m, err := decodeTIFF(r)
	// The TIFF decoder reports a file cut short as a bare io.EOF, which reads
	// like a clean end of input; the other decoders say "unexpected EOF".
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return m, err
}

func decodeTIFF(r io.Reader) (image.Image, error) {
	if src, ok := section(r); ok {
		if err := checkTIFF(src); err != nil {
			return nil, err
		}
		return tiff.Decode(src)
	}
	in := bufio.NewReader(r)
	header, err := in.Peek(4)
	if err != nil {
		return nil, err
	}
	// Checked before buffering, so input that is not a TIFF fails on its first
	// bytes instead of being read to the offset its header would name.
	if !isTIFF(header) {
		return nil, tiff.FormatError("malformed header")
	}
	buf := &lazyReaderAt{r: in}
	if err := checkTIFF(io.NewSectionReader(buf, 0, math.MaxInt64)); err != nil {
		return nil, err
	}
	return tiff.Decode(buf.drain())
}

func checkTIFF(src *io.SectionReader) error {
	cfg, err := tiff.DecodeConfig(src)
	if err != nil {
		return err
	}
	return checkPixels("image", cfg.Width, cfg.Height)
}

// section returns r from its current position as an io.SectionReader, which
// the TIFF decoder reads in place, if r allows random access. A pipe is an
// *os.File too, but cannot seek.
func section(r io.Reader) (*io.SectionReader, bool) {
	rs, ok := r.(interface {
		io.ReaderAt
		io.Seeker
	})
	if !ok {
		return nil, false
	}
	off, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, false
	}
	return io.NewSectionReader(rs, off, math.MaxInt64-off), true
}

// lazyReaderAt is an io.ReaderAt over a stream. It reads the stream in chunks
// of lazyChunk bytes, only up to the chunk a ReadAt reaches, and only as fast
// as data arrives, so an offset far past the end of a small input costs no
// more than the input and one chunk. Chunks are never copied or regrown.
type lazyReaderAt struct {
	r      io.Reader
	chunks [][]byte
	size   int64
	err    error
}

const lazyChunk = 1 << 20

func (l *lazyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	for end := off + int64(len(p)); l.err == nil && l.size < end; {
		chunk := make([]byte, lazyChunk)
		n := 0
		for n < len(chunk) && l.err == nil {
			var m int
			m, l.err = l.r.Read(chunk[n:])
			n += m
		}
		if n > 0 {
			l.chunks = append(l.chunks, chunk[:n])
			l.size += int64(n)
		}
	}
	read := 0
	for read < len(p) && off < l.size {
		n := copy(p[read:], l.chunks[off/lazyChunk][off%lazyChunk:])
		read += n
		off += int64(n)
	}
	if read < len(p) {
		return read, l.err
	}
	return read, nil
}

// drain returns a reader over what l has buffered followed by the rest of the
// stream, releasing each chunk once it has been read. l must not be used
// after drain is called.
func (l *lazyReaderAt) drain() io.Reader {
	return &drainReader{l}
}

type drainReader struct{ l *lazyReaderAt }

func (d *drainReader) Read(p []byte) (int, error) {
	l := d.l
	if len(l.chunks) == 0 {
		if l.err == nil {
			return l.r.Read(p)
		}
		return 0, l.err
	}
	n := copy(p, l.chunks[0])
	if l.chunks[0] = l.chunks[0][n:]; len(l.chunks[0]) == 0 {
		l.chunks[0] = nil
		l.chunks = l.chunks[1:]
	}
	return n, nil
}

// decodeAnyLimited decodes an image in any format registered with
// image.Decode, guarded like decodeLimited, and returns the format's name. A
// TIFF goes through decodeTIFFLimited, so a file is still read in place.
func decodeAnyLimited(r io.Reader) (image.Image, string, error) {
	if src, ok := section(r); ok {
		header := make([]byte, 4)
		if n, _ := src.ReadAt(header, 0); isTIFF(header[:n]) {
			m, err := decodeTIFFLimited(r)
			return m, "tiff", err
		}
	}
	in := bufio.NewReader(r)
	header, err := in.Peek(4)
	if err != nil && err != io.EOF {
		return nil, "", err
	}
	if isTIFF(header) {
		m, err := decodeTIFFLimited(in)
		return m, "tiff", err
	}
	cfg, format, rest, err := peekConfigAny(in)
	if err != nil {
		return nil, format, err
	}
	if err := checkPixels("image", cfg.Width, cfg.Height); err != nil {
		return nil, format, err
	}
	m, _, err := image.Decode(rest)
	return m, format, err
}

// peekConfigAny is peekConfig for any format registered with image.Decode,
// and also returns the format's name.
func peekConfigAny(r io.Reader) (image.Config, string, io.Reader, error) {
	var format string
	cfg, rest, err := peekConfig(r, func(r io.Reader) (image.Config, error) {
		cfg, f, err := image.DecodeConfig(r)
		format = f
		return cfg, err
	})
	return cfg, format, rest, err
}

// isTIFF reports whether header starts with a TIFF byte-order mark and magic
// number.
func isTIFF(header []byte) bool {
	return bytes.HasPrefix(header, []byte("II*\x00")) || bytes.HasPrefix(header, []byte("MM\x00*"))
}
