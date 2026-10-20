package img

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

const limitMsg = "over the limit of 128000000 pixels"

func TestCheckPixels(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantErr       bool
	}{
		{name: "small", width: 640, height: 480},
		{name: "exactly at the limit", width: 16000, height: 8000},
		{name: "one row over the limit", width: 16000, height: 8001, wantErr: true},
		{name: "one pixel over the limit", width: MaxPixels + 1, height: 1, wantErr: true},
		{name: "a single row at the limit", width: MaxPixels, height: 1},
		{name: "a single column at the limit", width: 1, height: MaxPixels},
		{name: "a single column over the limit", width: 1, height: MaxPixels + 1, wantErr: true},
		{name: "largest square within the limit", width: 11313, height: 11313},
		{name: "smallest square over the limit", width: 11314, height: 11314, wantErr: true},
		{name: "zero width has no pixels", width: 0, height: math.MaxInt},
		{name: "zero height has no pixels", width: math.MaxInt, height: 0},
		{name: "product would overflow", width: math.MaxInt, height: math.MaxInt, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPixels("image", tt.width, tt.height)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkPixels(%d, %d) error = %v, wantErr %v", tt.width, tt.height, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), limitMsg) {
				t.Errorf("checkPixels() error = %q, want it to name the limit", err)
			}
		})
	}
}

// The crafted images below are a valid tiny image whose header is rewritten
// to declare other dimensions, so the pixel data cannot fill them. At the
// limit, the decoder runs and fails on the missing data; over it, the image
// must be rejected before the decoder allocates its pixels. They are one
// byte per pixel where the format allows, so the at-the-limit cases cost the
// test 128 MB rather than 512.

var onePixelGray = image.NewGray(image.Rect(0, 0, 1, 1))

func craftedPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, onePixelGray); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	// Signature (8), IHDR length (4), "IHDR" (4), width, height, ..., CRC.
	binary.BigEndian.PutUint32(data[16:], uint32(width))
	binary.BigEndian.PutUint32(data[20:], uint32(height))
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func craftedJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, onePixelGray, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	sof := bytes.Index(data, []byte{0xFF, 0xC0})
	if sof < 0 {
		t.Fatal("encoded JPEG has no SOF0 marker")
	}
	// Marker (2), length (2), precision (1), height, width.
	binary.BigEndian.PutUint16(data[sof+5:], uint16(height))
	binary.BigEndian.PutUint16(data[sof+7:], uint16(width))
	return data
}

// craftedGIF declares a logical screen and a first frame of separate sizes.
func craftedGIF(t *testing.T, screenW, screenH, frameW, frameH int) []byte {
	t.Helper()
	var buf bytes.Buffer
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White})
	if err := gif.Encode(&buf, frame, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	binary.LittleEndian.PutUint16(data[6:], uint16(screenW))
	binary.LittleEndian.PutUint16(data[8:], uint16(screenH))
	i := 13
	if data[10]&0x80 != 0 {
		i += 3 << (data[10]&7 + 1)
	}
	if data[i] != ',' {
		t.Fatalf("encoded GIF has no image descriptor at %d", i)
	}
	// Separator (1), left (2), top (2), width, height.
	binary.LittleEndian.PutUint16(data[i+5:], uint16(frameW))
	binary.LittleEndian.PutUint16(data[i+7:], uint16(frameH))
	return data
}

func craftedBMP(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black})); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	binary.LittleEndian.PutUint32(data[18:], uint32(width))
	binary.LittleEndian.PutUint32(data[22:], uint32(height))
	return data
}

// patchTIFF rewrites tag values in the page'th IFD (0-based) of a
// little-endian TIFF, as the x/image encoder writes.
func patchTIFF(t *testing.T, data []byte, page int, tags map[uint16]uint32) []byte {
	t.Helper()
	out := bytes.Clone(data)
	if string(out[:4]) != "II*\x00" {
		t.Fatal("want a little-endian TIFF")
	}
	le := binary.LittleEndian
	ifd := int(le.Uint32(out[4:]))
	for range page {
		ifd = int(le.Uint32(out[ifd+2+12*int(le.Uint16(out[ifd:])):]))
	}
	for e := range int(le.Uint16(out[ifd:])) {
		entry := out[ifd+2+12*e:]
		v, ok := tags[le.Uint16(entry)]
		if !ok {
			continue
		}
		switch typ := le.Uint16(entry[2:]); typ {
		case 3: // SHORT
			le.PutUint16(entry[8:], uint16(v))
		case 4: // LONG
			le.PutUint32(entry[8:], v)
		default:
			t.Fatalf("unexpected TIFF field type %d", typ)
		}
	}
	return out
}

func craftedTIFF(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, onePixelGray, nil); err != nil {
		t.Fatal(err)
	}
	const imageWidth, imageLength, rowsPerStrip = 256, 257, 278
	return patchTIFF(t, buf.Bytes(), 0, map[uint16]uint32{
		imageWidth: uint32(width), imageLength: uint32(height), rowsPerStrip: uint32(height),
	})
}

func riffWebP(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := binary.LittleEndian.AppendUint32([]byte("RIFF"), uint32(len(body)))
	return append(out, body...)
}

func webpChunk(id string, payload []byte) []byte {
	out := binary.LittleEndian.AppendUint32([]byte(id), uint32(len(payload)))
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// vp8lHeader is a lossless WebP bitstream cut off right after its header.
func vp8lHeader(width, height int) []byte {
	bits := uint32(width-1) | uint32(height-1)<<14
	return binary.LittleEndian.AppendUint32([]byte{0x2f}, bits)
}

func craftedLosslessWebP(width, height int) []byte {
	return riffWebP(webpChunk("VP8L", vp8lHeader(width, height)))
}

// craftedLossyWebP patches the frame header of the lossy fixture: RIFF
// header (12), chunk header (8), frame tag (3), start code (3), width, height.
func craftedLossyWebP(t *testing.T, width, height int) []byte {
	t.Helper()
	data := bytes.Clone(fixture(t, "lossy.webp"))
	binary.LittleEndian.PutUint16(data[26:], uint16(width))
	binary.LittleEndian.PutUint16(data[28:], uint16(height))
	return data
}

// craftedCanvasWebP wraps a lossless frame in a VP8X canvas of another size.
func craftedCanvasWebP(canvasW, canvasH, frameW, frameH int) []byte {
	vp8x := make([]byte, 10)
	w, h := uint32(canvasW-1), uint32(canvasH-1)
	vp8x[4], vp8x[5], vp8x[6] = byte(w), byte(w>>8), byte(w>>16)
	vp8x[7], vp8x[8], vp8x[9] = byte(h), byte(h>>8), byte(h>>16)
	return riffWebP(webpChunk("VP8X", vp8x), webpChunk("VP8L", vp8lHeader(frameW, frameH)))
}

func resizeTo(opts ResizeOptions) func(io.Writer, io.Reader) error {
	return func(w io.Writer, r io.Reader) error { return Resize(w, r, opts) }
}

func qrDecodeTo(w io.Writer, r io.Reader) error {
	_, err := QRDecode(r)
	return err
}

func pngToJPEG(w io.Writer, r io.Reader) error { return PNGToJPEG(w, r, 85) }

// allocatedDuring reports the bytes the Go heap allocated while fn ran.
func allocatedDuring(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestPixelLimit(t *testing.T) {
	resize := resizeTo(ResizeOptions{Max: 100, Quality: 85})

	tests := []struct {
		name    string
		convert func(io.Writer, io.Reader) error
		input   func(t *testing.T) []byte
		// wantLimit means the limit rejects the image; otherwise the image
		// passes the check and the decoder fails on the missing pixel data.
		wantLimit bool
		wantMsg   string
	}{
		{name: "PNG at the limit", convert: pngToJPEG, input: func(t *testing.T) []byte { return craftedPNG(t, 16000, 8000) }},
		{name: "PNG over the limit", convert: pngToJPEG, input: func(t *testing.T) []byte { return craftedPNG(t, 16000, 8001) }, wantLimit: true},
		{name: "PNG far over the limit", convert: pngToJPEG, input: func(t *testing.T) []byte { return craftedPNG(t, 30000, 30000) }, wantLimit: true},
		{name: "JPEG at the limit", convert: JPEGToPNG, input: func(t *testing.T) []byte { return craftedJPEG(t, 16000, 8000) }},
		{name: "JPEG over the limit", convert: JPEGToPNG, input: func(t *testing.T) []byte { return craftedJPEG(t, 16000, 8001) }, wantLimit: true},
		{name: "JPEG at the format's maximum", convert: JPEGToPNG, input: func(t *testing.T) []byte { return craftedJPEG(t, 65535, 65535) }, wantLimit: true},
		{name: "GIF at the limit", convert: GIFToPNG, input: func(t *testing.T) []byte { return craftedGIF(t, 16000, 8000, 16000, 8000) }},
		{name: "GIF over the limit", convert: GIFToPNG, input: func(t *testing.T) []byte { return craftedGIF(t, 16000, 8001, 16000, 8001) }, wantLimit: true},
		// The logical screen is the declared size: a small frame on an
		// oversized screen is rejected, since later frames may fill it.
		{name: "GIF small frame on a screen over the limit", convert: GIFToPNG, input: func(t *testing.T) []byte { return craftedGIF(t, 65535, 65535, 1, 1) }, wantLimit: true},
		// The decoder rejects a frame larger than the screen on its own, so
		// the screen size checked here bounds what the decoder allocates.
		{name: "GIF frame larger than its screen", convert: GIFToPNG, input: func(t *testing.T) []byte { return craftedGIF(t, 1, 1, 16000, 8001) }, wantMsg: "frame bounds larger than image bounds"},
		{name: "BMP at the limit", convert: BMPToPNG, input: func(t *testing.T) []byte { return craftedBMP(t, 16000, 8000) }},
		{name: "BMP over the limit", convert: BMPToPNG, input: func(t *testing.T) []byte { return craftedBMP(t, 16000, 8001) }, wantLimit: true},
		{name: "TIFF at the limit", convert: TIFFToPNG, input: func(t *testing.T) []byte { return craftedTIFF(t, 16000, 8000) }},
		{name: "TIFF over the limit", convert: TIFFToPNG, input: func(t *testing.T) []byte { return craftedTIFF(t, 16000, 8001) }, wantLimit: true},
		{name: "lossless WebP at the limit", convert: WebPToPNG, input: func(t *testing.T) []byte { return craftedLosslessWebP(16000, 8000) }},
		{name: "lossless WebP over the limit", convert: WebPToPNG, input: func(t *testing.T) []byte { return craftedLosslessWebP(16000, 8001) }, wantLimit: true},
		{name: "lossy WebP over the limit", convert: WebPToPNG, input: func(t *testing.T) []byte { return craftedLossyWebP(t, 16000, 8001) }, wantLimit: true},
		{name: "WebP canvas over the limit", convert: WebPToPNG, input: func(t *testing.T) []byte { return craftedCanvasWebP(16000, 8001, 1, 1) }, wantLimit: true},
		// The decoder rejects a frame that disagrees with its canvas, so the
		// canvas size checked here bounds what the decoder allocates.
		{name: "WebP frame larger than its canvas", convert: WebPToPNG, input: func(t *testing.T) []byte { return craftedCanvasWebP(1, 1, 16000, 8001) }, wantMsg: "webp: invalid format"},
		{name: "resize PNG over the limit", convert: resize, input: func(t *testing.T) []byte { return craftedPNG(t, 16000, 8001) }, wantLimit: true},
		{name: "resize JPEG over the limit", convert: resize, input: func(t *testing.T) []byte { return craftedJPEG(t, 16000, 8001) }, wantLimit: true},
		// Resize accepts only PNG and JPEG, and says so before decoding.
		{name: "resize GIF over the limit", convert: resize, input: func(t *testing.T) []byte { return craftedGIF(t, 16000, 8001, 16000, 8001) }, wantMsg: `unsupported input format "gif"`},
		{name: "QR decode PNG over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedPNG(t, 16000, 8001) }, wantLimit: true},
		{name: "QR decode JPEG over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedJPEG(t, 16000, 8001) }, wantLimit: true},
		{name: "QR decode GIF over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedGIF(t, 16000, 8001, 16000, 8001) }, wantLimit: true},
		{name: "QR decode BMP over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedBMP(t, 16000, 8001) }, wantLimit: true},
		{name: "QR decode TIFF over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedTIFF(t, 16000, 8001) }, wantLimit: true},
		{name: "QR decode WebP over the limit", convert: qrDecodeTo, input: func(t *testing.T) []byte { return craftedLosslessWebP(16000, 8001) }, wantLimit: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input(t)
			var err error
			allocated := allocatedDuring(func() { err = tt.convert(io.Discard, bytes.NewReader(input)) })
			if err == nil {
				t.Fatal("convert succeeded on a crafted image, want an error")
			}
			if got := strings.Contains(err.Error(), limitMsg); got != tt.wantLimit {
				t.Fatalf("convert error = %q, limit error = %v, want %v", err, got, tt.wantLimit)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("convert error = %q, want it to mention %q", err, tt.wantMsg)
			}
			// Any rejected image above declares at least 128 MB of pixels.
			if (tt.wantLimit || tt.wantMsg != "") && allocated > 32<<20 {
				t.Errorf("rejecting the image allocated %d MB, want it rejected before its pixels are allocated", allocated>>20)
			}
		})
	}
}

// x/image/tiff decodes, and reports the size of, only the first page, so a
// later page cannot declare a size that the check never sees.
func TestPixelLimitMultiPageTIFF(t *testing.T) {
	const imageWidth, imageLength = 256, 257
	data := patchTIFF(t, fixture(t, "multipage.tiff"), 1, map[uint16]uint32{imageWidth: 60000, imageLength: 60000})
	if cfg, err := tiff.DecodeConfig(bytes.NewReader(data)); err != nil || cfg.Width != 8 || cfg.Height != 8 {
		t.Fatalf("DecodeConfig() = %dx%d, %v; want the first page's 8x8", cfg.Width, cfg.Height, err)
	}
	var out bytes.Buffer
	if err := TIFFToPNG(&out, bytes.NewReader(data)); err != nil {
		t.Fatalf("TIFFToPNG() error = %v", err)
	}
	if got := decodePNG(t, &out).Bounds().Size(); got != image.Pt(8, 8) {
		t.Errorf("output size = %v, want the first page's 8x8", got)
	}
}

func TestResizeOutputLimit(t *testing.T) {
	tests := []struct {
		name    string
		srcW    int
		srcH    int
		opts    ResizeOptions
		wantMsg string
	}{
		{name: "explicit size over the limit", srcW: 1, srcH: 1, opts: ResizeOptions{Width: 16000, Height: 8001, Quality: 85}, wantMsg: "resized image is 16000x8001 pixels, " + limitMsg},
		{name: "derived height over the limit", srcW: 1, srcH: 2, opts: ResizeOptions{Width: 12000, Quality: 85}, wantMsg: "resized image is 12000x24000 pixels, " + limitMsg},
		{name: "derived width over the limit", srcW: 2, srcH: 1, opts: ResizeOptions{Height: 12000, Quality: 85}, wantMsg: "resized image is 24000x12000 pixels, " + limitMsg},
		{name: "side alone over the limit", srcW: 1, srcH: 1, opts: ResizeOptions{Width: MaxPixels + 1, Quality: 85}, wantMsg: "at most 128000000 pixels"},
		{name: "side whose derived product would overflow", srcW: 1, srcH: 1000, opts: ResizeOptions{Width: math.MaxInt, Quality: 85}, wantMsg: "at most 128000000 pixels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := pngBytes(t, solidNRGBA(tt.srcW, tt.srcH, color.NRGBA{A: 255}))
			var err error
			allocated := allocatedDuring(func() { err = Resize(io.Discard, src, tt.opts) })
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Resize() error = %v, want it to mention %q", err, tt.wantMsg)
			}
			if allocated > 32<<20 {
				t.Errorf("rejecting the resize allocated %d MB, want it rejected before the output is allocated", allocated>>20)
			}
		})
	}
}

// Scale needs scratch memory of output width x input height, and past that
// Resize uses Transform, whose work is the wider width times the taller
// height. Only a resize that widens one side while shrinking the other can
// push that past MaxPixels when neither image is over it.
func TestResizeWorkingArea(t *testing.T) {
	const area, extreme = "working area", "too extreme"
	tests := []struct {
		name    string
		srcW    int
		srcH    int
		opts    ResizeOptions
		wantMsg string
	}{
		{name: "widening while shortening", srcW: 1000, srcH: 100000, opts: ResizeOptions{Width: 2000, Height: 1000, Quality: 85}, wantMsg: area},
		{name: "widening just past the limit", srcW: 1000, srcH: 128000, opts: ResizeOptions{Width: 1001, Height: 1000, Quality: 85}, wantMsg: area},
		{name: "narrowing while heightening fits Scale", srcW: 100000, srcH: 1000, opts: ResizeOptions{Width: 2000, Height: 2000, Quality: 85}},
		{name: "plain downscale of a tall image", srcW: 1000, srcH: 100000, opts: ResizeOptions{Width: 700, Quality: 85}},
		{name: "working area at the limit", srcW: 1000, srcH: 128000, opts: ResizeOptions{Width: 1000, Height: 1000, Quality: 85}},
		{name: "a very long line shrunk a little", srcW: 128000000, srcH: 1, opts: ResizeOptions{Max: 1000, Quality: 85}},
		{name: "a very long line shrunk to a point", srcW: 128000000, srcH: 1, opts: ResizeOptions{Max: 1, Quality: 85}, wantMsg: extreme},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			allocated := allocatedDuring(func() {
				err = Resize(io.Discard, bytes.NewReader(craftedPNG(t, tt.srcW, tt.srcH)), tt.opts)
			})
			if err == nil {
				t.Fatal("Resize() succeeded on a header without pixel data, want an error")
			}
			limited := strings.Contains(err.Error(), area) || strings.Contains(err.Error(), extreme)
			if tt.wantMsg == "" && limited || tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Resize() error = %v, want limit %q", err, tt.wantMsg)
			}
			if tt.wantMsg != "" && allocated > 32<<20 {
				t.Errorf("rejecting the resize allocated %d MB, want it rejected before decoding", allocated>>20)
			}
		})
	}
}

// Resize switches from Scale to Transform when Scale's scratch buffer would be
// too large; the switch must not change the picture.
func TestResampleMethodsAgree(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 301, 199))
	for i := range src.Pix {
		src.Pix[i] = uint8(i*7 + i/1013)
	}
	// A decoder may return an image whose bounds do not start at (0, 0).
	offset := src.SubImage(image.Rect(40, 30, 301, 199))
	for _, src := range []image.Image{src, offset} {
		for _, size := range []image.Point{{100, 66}, {37, 199}, {602, 398}, {150, 300}} {
			scaled := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
			transformed := image.NewRGBA(scaled.Bounds())
			resample(scaled, src, true)
			resample(transformed, src, false)
			for i := range scaled.Pix {
				if d := int(scaled.Pix[i]) - int(transformed.Pix[i]); d < -1 || d > 1 {
					t.Fatalf("%v from %v: byte %d differs by %d between Scale and Transform", size, src.Bounds(), i, d)
				}
			}
		}
	}
}

// Go's TIFF encoder writes the header (IFD) after the pixel data, so checking
// the size means reading to the end of the file first. A file must be read in
// place, and a stream buffered at most once on top of what tiff.Decode itself
// allocates.
func TestTIFFBufferedOnce(t *testing.T) {
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, image.NewGray(image.Rect(0, 0, 2048, 2048)), nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	stream := func() io.Reader { return struct{ io.Reader }{bytes.NewReader(data)} }
	tests := []struct {
		name  string
		input func() io.Reader
		extra uint64
	}{
		{name: "file", input: func() io.Reader { return bytes.NewReader(data) }},
		{name: "stream", input: stream, extra: uint64(len(data)) + lazyChunk},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain := allocatedDuring(func() { tiff.Decode(tt.input()) })
			var err error
			got := allocatedDuring(func() { _, err = decodeTIFFLimited(tt.input()) })
			if err != nil {
				t.Fatal(err)
			}
			if limit := plain + plain/20 + tt.extra; got > limit {
				t.Errorf("decodeTIFFLimited allocated %d bytes, want at most %d (tiff.Decode alone: %d)", got, limit, plain)
			}
		})
	}
}

// A stream is buffered only as fast as data arrives, so a tiny TIFF whose
// header points gigabytes past its end costs no more than its own bytes.
func TestTIFFFarOffsetStream(t *testing.T) {
	data := []byte("II*\x00\xf0\xff\xff\x7f")
	var err error
	allocated := allocatedDuring(func() { _, err = decodeTIFFLimited(struct{ io.Reader }{bytes.NewReader(data)}) })
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("decodeTIFFLimited() error = %v, want %v for a header pointing past the end of the input", err, io.ErrUnexpectedEOF)
	}
	if allocated > 4<<20 {
		t.Errorf("decodeTIFFLimited allocated %d MB for a %d-byte input", allocated>>20, len(data))
	}
}

// scaleMemory and transformMemory model x/image/draw's allocations so Resize
// can choose a resampler before decoding; they must never be below what the
// resamplers actually allocate, and not so far above it that the choice is
// meaningless.
func TestResampleMemoryEstimates(t *testing.T) {
	for _, tt := range []struct{ sw, sh, dw, dh int }{
		{sw: 300, sh: 200, dw: 100, dh: 66},
		{sw: 100, sh: 66, dw: 300, dh: 200},
		{sw: 20000, sh: 3, dw: 7, dh: 3},
		{sw: 3, sh: 20000, dw: 3, dh: 7},
		{sw: 20000, sh: 2, dw: 1, dh: 1},
		{sw: 7, sh: 3, dw: 20000, dh: 3},
		{sw: 1, sh: 1, dw: 50, dh: 50},
	} {
		src := image.NewGray(image.Rect(0, 0, tt.sw, tt.sh))
		for _, method := range []struct {
			name     string
			useScale bool
			estimate int64
		}{
			{name: "Scale", useScale: true, estimate: scaleMemory(tt.sw, tt.sh, tt.dw, tt.dh)},
			{name: "Transform", useScale: false, estimate: transformMemory(tt.sw, tt.sh, tt.dw, tt.dh)},
		} {
			t.Run(fmt.Sprintf("%s %dx%d to %dx%d", method.name, tt.sw, tt.sh, tt.dw, tt.dh), func(t *testing.T) {
				dst := image.NewRGBA(image.Rect(0, 0, tt.dw, tt.dh))
				got := int64(allocatedDuring(func() { resample(dst, src, method.useScale) }))
				const slack = 4 << 10
				if got > method.estimate+slack {
					t.Errorf("allocated %d bytes, estimate %d is too low", got, method.estimate)
				}
				if method.estimate > 4*got+64<<10 {
					t.Errorf("allocated %d bytes, estimate %d is more than 4 times too high", got, method.estimate)
				}
			})
		}
	}
}

// failingReader returns its data and then err instead of io.EOF.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

// A read error from a TIFF stream must surface as itself, not as a format
// error or a clean end of input, whether the decoder is still looking for the
// header or already reading pixels.
func TestTIFFStreamReadError(t *testing.T) {
	var goTIFF bytes.Buffer
	if err := tiff.Encode(&goTIFF, image.NewGray(image.Rect(0, 0, 256, 256)), nil); err != nil {
		t.Fatal(err)
	}
	headerFirst, err := os.ReadFile(filepath.Join("testdata", "multipage.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	errRead := errors.New("read failed")
	tests := []struct {
		name string
		data []byte
		cut  int
	}{
		{name: "header after the pixels", data: goTIFF.Bytes(), cut: goTIFF.Len() / 2},
		{name: "header before the pixels", data: headerFirst, cut: len(headerFirst) / 2},
		// Past the first chunk, so the error arrives after the decoder has
		// taken over the stream from the buffer.
		{name: "header before the pixels, failing after the buffer", data: headerFirstTIFF(1024, 2048), cut: lazyChunk + lazyChunk/2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeTIFFLimited(struct{ io.Reader }{bytes.NewReader(tt.data)}); err != nil {
				t.Fatalf("the whole input failed to decode: %v", err)
			}
			_, err := decodeTIFFLimited(&failingReader{data: tt.data[:tt.cut], err: errRead})
			if !errors.Is(err, errRead) {
				t.Errorf("decodeTIFFLimited() error = %v, want %v", err, errRead)
			}
		})
	}
	t.Run("input shorter than a header", func(t *testing.T) {
		for _, input := range []string{"", "II*"} {
			_, fromFile := decodeTIFFLimited(strings.NewReader(input))
			_, fromStream := decodeTIFFLimited(struct{ io.Reader }{strings.NewReader(input)})
			if !errors.Is(fromFile, io.ErrUnexpectedEOF) || !errors.Is(fromStream, io.ErrUnexpectedEOF) {
				t.Errorf("%q: file error %v, stream error %v, want %v from both", input, fromFile, fromStream, io.ErrUnexpectedEOF)
			}
		}
	})
}

// headerFirstTIFF returns an uncompressed 8-bit gray TIFF whose header comes
// before its pixels, unlike what Go's encoder writes.
func headerFirstTIFF(width, height int) []byte {
	const entries = 9
	pixels := 8 + 2 + entries*12 + 4
	le := binary.LittleEndian
	b := []byte("II*\x00")
	b = le.AppendUint32(b, 8)
	b = le.AppendUint16(b, entries)
	for _, e := range [][2]uint32{
		{256, uint32(width)}, {257, uint32(height)}, {258, 8}, {259, 1}, {262, 1},
		{273, uint32(pixels)}, {277, 1}, {278, uint32(height)}, {279, uint32(width * height)},
	} {
		b = le.AppendUint16(b, uint16(e[0]))
		b = le.AppendUint16(b, 4) // LONG
		b = le.AppendUint32(b, 1)
		b = le.AppendUint32(b, e[1])
	}
	b = le.AppendUint32(b, 0)
	return append(b, make([]byte, width*height)...)
}

// A reader that is already partway through its data is read from where it
// is, whatever format the bytes before that point look like.
func TestDecodeAnyFromCurrentPosition(t *testing.T) {
	var tiffBuf, pngBuf bytes.Buffer
	if err := tiff.Encode(&tiffBuf, image.NewGray(image.Rect(0, 0, 3, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngBuf, image.NewGray(image.Rect(0, 0, 5, 4))); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		prefix []byte
		image  []byte
		format string
	}{
		{name: "PNG after a TIFF", prefix: tiffBuf.Bytes(), image: pngBuf.Bytes(), format: "png"},
		{name: "TIFF after a PNG", prefix: pngBuf.Bytes(), image: tiffBuf.Bytes(), format: "tiff"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(append(append([]byte{}, tt.prefix...), tt.image...))
			if _, err := r.Seek(int64(len(tt.prefix)), io.SeekStart); err != nil {
				t.Fatal(err)
			}
			_, format, err := decodeAnyLimited(r)
			if err != nil || format != tt.format {
				t.Fatalf("decodeAnyLimited() = %q, %v, want %q", format, err, tt.format)
			}
		})
	}
}

func FuzzDecodeLimited(f *testing.F) {
	small := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for _, encode := range []func(io.Writer, image.Image) error{
		png.Encode,
		func(w io.Writer, m image.Image) error { return jpeg.Encode(w, m, nil) },
		func(w io.Writer, m image.Image) error { return gif.Encode(w, m, nil) },
		bmp.Encode,
		func(w io.Writer, m image.Image) error { return tiff.Encode(w, m, nil) },
	} {
		var buf bytes.Buffer
		if err := encode(&buf, small); err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())
	}
	for _, name := range []string{"lossless.webp", "lossy.webp", "multipage.tiff"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	decoders := []func(io.Reader) (image.Image, error){
		limited(png.DecodeConfig, png.Decode),
		limited(jpeg.DecodeConfig, jpeg.Decode),
		limited(gif.DecodeConfig, gif.Decode),
		limited(bmp.DecodeConfig, bmp.Decode),
		limited(webp.DecodeConfig, webp.Decode),
		decodeTIFFLimited,
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Images within MaxPixels may still take hundreds of megabytes, which
		// would make the fuzzer crawl; the limit itself is tested above.
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width > 0 && cfg.Height > (1<<20)/cfg.Width {
			t.Skip()
		}
		for _, decode := range decoders {
			decode(bytes.NewReader(data))
		}
		// A file is read in place and a stream is buffered; both must decode
		// the same image or fail alike.
		fromFile, fileFormat, fileErr := decodeAnyLimited(bytes.NewReader(data))
		fromStream, streamFormat, streamErr := decodeAnyLimited(struct{ io.Reader }{bytes.NewReader(data)})
		if (fileErr == nil) != (streamErr == nil) {
			t.Fatalf("file error %v, stream error %v", fileErr, streamErr)
		}
		if fileErr == nil && (fileFormat != streamFormat || fromFile.Bounds() != fromStream.Bounds()) {
			t.Fatalf("file decoded %s %v, stream decoded %s %v", fileFormat, fromFile.Bounds(), streamFormat, fromStream.Bounds())
		}
	})
}

// The JPEG decoder accepts a zero height, which used to reach a division by
// zero when the width was derived.
func TestResizeZeroHeightJPEG(t *testing.T) {
	err := Resize(io.Discard, bytes.NewReader(craftedJPEG(t, 16, 0)), ResizeOptions{Height: 10, Quality: 85})
	if err == nil || !strings.Contains(err.Error(), "16x0 pixels, so there is nothing to resize") {
		t.Fatalf("Resize() error = %v, want a zero-size image error", err)
	}
}

func TestMaxQRSize(t *testing.T) {
	if err := checkPixels("QR code image", MaxQRSize, MaxQRSize); err != nil {
		t.Errorf("MaxQRSize %d does not fit within MaxPixels: %v", MaxQRSize, err)
	}
	if checkPixels("QR code image", MaxQRSize+1, MaxQRSize+1) == nil {
		t.Errorf("MaxQRSize %d is not the largest size within MaxPixels", MaxQRSize)
	}
}

func TestQRSizeLimit(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantMsg string
	}{
		{name: "just over the limit", size: MaxQRSize + 1, wantMsg: limitMsg},
		{name: "square would overflow", size: math.MaxInt, wantMsg: limitMsg},
		{name: "zero", size: 0, wantMsg: "size must be positive"},
		// go-qrcode reads a negative size as a multiple of the module count.
		{name: "negative", size: -1000, wantMsg: "size must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			allocated := allocatedDuring(func() { err = QR(io.Discard, "hello", tt.size) })
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("QR() error = %v, want it to mention %q", err, tt.wantMsg)
			}
			if allocated > 32<<20 {
				t.Errorf("rejecting the size allocated %d MB, want it rejected before the image is allocated", allocated>>20)
			}
		})
	}
}

// endlessZeros counts what is read from it. It stands for an endless stream
// but fails after 1 MB, so code that reads it whole fails the test instead of
// hanging it.
type endlessZeros struct{ n int }

func (z *endlessZeros) Read(p []byte) (int, error) {
	if z.n >= 1<<20 {
		return 0, errors.New("read past 1 MB of an endless stream")
	}
	clear(p)
	z.n += len(p)
	return len(p), nil
}

// Only the header is buffered before the size check, so input that is not an
// image fails on its first bytes rather than being read into memory whole.
func TestNonImageFailsOnItsHeader(t *testing.T) {
	tests := []struct {
		name    string
		convert func(io.Writer, io.Reader) error
	}{
		{name: "PNGToJPEG", convert: pngToJPEG},
		{name: "JPEGToPNG", convert: JPEGToPNG},
		{name: "GIFToPNG", convert: GIFToPNG},
		{name: "BMPToPNG", convert: BMPToPNG},
		{name: "TIFFToPNG", convert: TIFFToPNG},
		{name: "WebPToPNG", convert: WebPToPNG},
		{name: "Resize", convert: resizeTo(ResizeOptions{Max: 100, Quality: 85})},
		{name: "QRDecode", convert: qrDecodeTo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in endlessZeros
			if err := tt.convert(io.Discard, &in); err == nil {
				t.Fatal("convert succeeded on endless zeros, want an error")
			}
			if in.n > 64<<10 {
				t.Errorf("convert read %d bytes of non-image input before failing, want only a header's worth", in.n)
			}
		})
	}
}
