package img

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

func decodePNG(t *testing.T, out *bytes.Buffer) image.Image {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
	return decoded
}

func palettedFrame(w, h int, c color.RGBA) *image.Paletted {
	m := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{c})
	for i := range m.Pix {
		m.Pix[i] = 0
	}
	return m
}

func animatedGIF(t *testing.T, frames ...*image.Paletted) []byte {
	t.Helper()
	var buf bytes.Buffer
	g := &gif.GIF{}
	for _, f := range frames {
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 10)
	}
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encoding test GIF: %v", err)
	}
	return buf.Bytes()
}

func TestToPNGHappyPaths(t *testing.T) {
	singleTIFF := func(t *testing.T) []byte {
		t.Helper()
		var buf bytes.Buffer
		if err := tiff.Encode(&buf, solidNRGBA(8, 8, color.NRGBA{R: 20, G: 40, B: 220, A: 255}), nil); err != nil {
			t.Fatalf("encoding test TIFF: %v", err)
		}
		return buf.Bytes()
	}
	singleBMP := func(t *testing.T) []byte {
		t.Helper()
		var buf bytes.Buffer
		if err := bmp.Encode(&buf, solidNRGBA(8, 8, color.NRGBA{R: 200, G: 50, B: 50, A: 255})); err != nil {
			t.Fatalf("encoding test BMP: %v", err)
		}
		return buf.Bytes()
	}

	tests := []struct {
		name                string
		convert             func(io.Writer, io.Reader) error
		input               func(t *testing.T) []byte
		wantSize            image.Point
		wantR, wantG, wantB uint8
		tolerance           int
	}{
		{
			name:     "lossy WebP",
			convert:  WebPToPNG,
			input:    func(t *testing.T) []byte { return fixture(t, "lossy.webp") },
			wantSize: image.Pt(8, 8), wantR: 200, wantG: 50, wantB: 50, tolerance: 20,
		},
		{
			name:     "lossless WebP",
			convert:  WebPToPNG,
			input:    func(t *testing.T) []byte { return fixture(t, "lossless.webp") },
			wantSize: image.Pt(8, 8), wantR: 30, wantG: 180, wantB: 90,
		},
		{
			name:    "static GIF",
			convert: GIFToPNG,
			input: func(t *testing.T) []byte {
				return animatedGIF(t, palettedFrame(8, 8, color.RGBA{R: 200, G: 50, B: 50, A: 255}))
			},
			wantSize: image.Pt(8, 8), wantR: 200, wantG: 50, wantB: 50,
		},
		{
			// The stdlib gif.Decode returns the first frame of an animation;
			// verified empirically, and pinned here so a behavior change in a
			// future Go release fails loudly.
			name:    "animated GIF converts the first frame",
			convert: GIFToPNG,
			input: func(t *testing.T) []byte {
				return animatedGIF(t,
					palettedFrame(8, 8, color.RGBA{R: 200, G: 50, B: 50, A: 255}),
					palettedFrame(8, 8, color.RGBA{B: 255, A: 255}))
			},
			wantSize: image.Pt(8, 8), wantR: 200, wantG: 50, wantB: 50,
		},
		{
			name:     "BMP",
			convert:  BMPToPNG,
			input:    singleBMP,
			wantSize: image.Pt(8, 8), wantR: 200, wantG: 50, wantB: 50,
		},
		{
			name:     "single-page TIFF",
			convert:  TIFFToPNG,
			input:    singleTIFF,
			wantSize: image.Pt(8, 8), wantR: 20, wantG: 40, wantB: 220,
		},
		{
			// Fixture page 1 is 8x8 blue, page 2 is 4x4 yellow: x/image/tiff
			// decodes only the first IFD, verified empirically.
			name:     "multi-page TIFF converts the first page",
			convert:  TIFFToPNG,
			input:    func(t *testing.T) []byte { return fixture(t, "multipage.tiff") },
			wantSize: image.Pt(8, 8), wantR: 20, wantG: 40, wantB: 220,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tt.convert(&out, bytes.NewReader(tt.input(t))); err != nil {
				t.Fatalf("convert error = %v", err)
			}
			decoded := decodePNG(t, &out)
			if got := decoded.Bounds().Size(); got != tt.wantSize {
				t.Errorf("output size = %v, want %v", got, tt.wantSize)
			}
			assertCenterColor(t, decoded, tt.wantR, tt.wantG, tt.wantB, tt.tolerance)
		})
	}

	t.Run("lossless WebP preserves alpha as 8-bit PNG", func(t *testing.T) {
		var out bytes.Buffer
		if err := WebPToPNG(&out, bytes.NewReader(fixture(t, "lossless.webp"))); err != nil {
			t.Fatalf("WebPToPNG() error = %v", err)
		}
		decoded := decodePNG(t, &out)
		b := decoded.Bounds()
		c := color.NRGBAModel.Convert(decoded.At(b.Dx()/2, b.Dy()/2)).(color.NRGBA)
		if want := (color.NRGBA{R: 30, G: 180, B: 90, A: 128}); c != want {
			t.Errorf("center pixel = %v, want %v", c, want)
		}
	})

	t.Run("lossy WebP output stays 8-bit", func(t *testing.T) {
		// Lossy WebP decodes to YCbCr, which png.Encode would widen to a
		// 16-bit PNG without the NRGBA conversion. Offset 24 is the IHDR bit
		// depth (signature, chunk header, width, height precede it).
		var out bytes.Buffer
		if err := WebPToPNG(&out, bytes.NewReader(fixture(t, "lossy.webp"))); err != nil {
			t.Fatalf("WebPToPNG() error = %v", err)
		}
		if got := out.Bytes()[24]; got != 8 {
			t.Errorf("PNG bit depth = %d, want 8", got)
		}
	})
}

func TestToPNGErrors(t *testing.T) {
	converters := []struct {
		format  string
		convert func(io.Writer, io.Reader) error
		valid   func(t *testing.T) []byte
	}{
		{"WebP", WebPToPNG, func(t *testing.T) []byte { return fixture(t, "lossy.webp") }},
		{"GIF", GIFToPNG, func(t *testing.T) []byte {
			return animatedGIF(t, palettedFrame(4, 4, color.RGBA{A: 255}))
		}},
		{"BMP", BMPToPNG, func(t *testing.T) []byte {
			var buf bytes.Buffer
			if err := bmp.Encode(&buf, solidNRGBA(4, 4, color.NRGBA{A: 255})); err != nil {
				t.Fatal(err)
			}
			return buf.Bytes()
		}},
		{"TIFF", TIFFToPNG, func(t *testing.T) []byte {
			var buf bytes.Buffer
			if err := tiff.Encode(&buf, solidNRGBA(4, 4, color.NRGBA{A: 255}), nil); err != nil {
				t.Fatal(err)
			}
			return buf.Bytes()
		}},
	}
	for _, c := range converters {
		inputs := []struct {
			name  string
			input func(t *testing.T) []byte
		}{
			{"empty input", func(t *testing.T) []byte { return nil }},
			{"wrong format (PNG input)", func(t *testing.T) []byte {
				return pngBytes(t, solidNRGBA(4, 4, color.NRGBA{A: 255})).Bytes()
			}},
			{"corrupt (truncated) input", func(t *testing.T) []byte {
				valid := c.valid(t)
				return valid[:len(valid)/2]
			}},
		}
		for _, tt := range inputs {
			t.Run(c.format+" "+tt.name, func(t *testing.T) {
				err := c.convert(&bytes.Buffer{}, bytes.NewReader(tt.input(t)))
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if want := "decoding " + c.format; !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to mention %q", err, want)
				}
			})
		}
	}
}
